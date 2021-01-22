package flagger

import (
	"github.com/airdeploy/flagger-go/v3/internal/httputils"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/ingester"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/airdeploy/flagger-go/v3/sse"
)

const firstExposuresIngestThreshold = 11

// check implementation public interface on compile time
var _ interface {
	Init(args *InitArgs) error
	Publish(entity *core.Entity)
	Track(event *core.Event)
	SetEntity(entity *core.Entity)
	IsEnabled(codename string, entity *core.Entity) bool
	IsSampled(codename string, entity *core.Entity) bool
	GetVariation(codename string, entity *core.Entity) string
	GetPayload(codename string, entity *core.Entity) core.Payload
} = new(Flagger)

// NewFlagger return the new instance Flagger
func NewFlagger() *Flagger {
	return &Flagger{
		rt:   http.DefaultTransport,
		core: core.NewCore(),
	}
}

// Flagger represent flagger client implementation
type Flagger struct {
	rt       http.RoundTripper
	core     *core.Core
	ingester *ingester.Ingester
	sse      *sse.Client
	mux      sync.RWMutex
	enabled  bool
}

// InitArgs represent init arguments for Flagger
type InitArgs struct {
	APIKey          string
	SourceURL       string
	BackupSourceURL string
	IngestionURL    string
	SSEURL          string
	LogLevel        string
}

// Init gets FlaggerConfiguration, establishes and maintains SSE connections and initialize Ingester
func (flagger *Flagger) Init(args *InitArgs) error {
	args, err := prepareInitArgs(args, SDKInfo)
	if err != nil {
		return err
	}

	flagger.Shutdown(1 * time.Second)

	flagger.mux.Lock()
	defer flagger.mux.Unlock()

	// Ingester
	flagger.ingester = ingester.NewIngester(SDKInfo, firstExposuresIngestThreshold)

	// get configuration from SourceURL/BackupSourceURL
	var configuration *core.Configuration
	err = httputils.GetConfiguration(flagger.rt, args.SourceURL, defaultAttemptsConnection, &configuration)
	if err != nil {
		log.Warnf("Unable to fetch FlaggerConfiguration from SourceURL")
		err := httputils.GetConfiguration(flagger.rt, args.BackupSourceURL, defaultAttemptsConnection, &configuration)
		if err != nil {
			log.Warnf("Unable to fetch FlaggerConfiguration from BackupSourceURL")
			return err
		}
		bytes, _ := json.Marshal(configuration)
		log.Debugf("init flagger from BackupSourceURL was success: %+v", string(bytes))
	} else {
		bytes, _ := json.Marshal(configuration)
		log.Debugf("init flagger from SourceURL was success: %+v", string(bytes))
	}

	flagger.enabled = true

	// init returns err if flagger fails to get the configuration
	flagger.core.SetConfig(configuration)

	flagger.ingester.Activate(args.IngestionURL, &configuration.SdkConfig)
	flagger.ingester.SendEmptyIngestion()

	// SSE
	flagger.sse = sse.NewClient(func(v *core.Configuration) {
		flagger.core.SetConfig(v)
		flagger.ingester.Shutdown(time.Second)
		flagger.ingester.Activate(args.IngestionURL, &v.SdkConfig)
	})
	flagger.sse.SetURL(args.SSEURL)
	return nil
}

func (flagger *Flagger) silentInit() (res bool) {
	apiKey := os.Getenv(FlaggerAPIKey)
	sourceURL := os.Getenv(FlaggerSourceURL)
	backupSourceURL := os.Getenv(FlaggerBackupSourceURL)
	ingestionURL := os.Getenv(FlaggerIngestionURL)
	sseURL := os.Getenv(FlaggerSSEUrl)
	logLevel := os.Getenv(FlaggerLogLevel)
	log.Debugf("Trying to initialise flagger using environment variables, "+
		"FLAGGER_API_KEY: '%s', "+
		"FLAGGER_SOURCE_URL: '%s', "+
		"FLAGGER_BACKUP_SOURCE_URL: '%s', "+
		"FLAGGER_INGESTION_URL: '%s', "+
		"FLAGGER_SSE_URL: '%s', "+
		"FLAGGER_LOG_LEVEL: '%s'", apiKey, sourceURL, backupSourceURL, ingestionURL, sseURL, logLevel)
	err := flagger.Init(nil)
	res = err == nil
	if !res {
		log.Errorf("Could not initialize flagger using environment variables, set LogLevel to debug to see environment variables values, for more info see: https://docs.airdeploy.io/flagger-sdk/quick-start")
	}
	return res
}

// Shutdown ingests data(if any), stops ingester and closes SSE connection.
// Shutdown waits to finish current ingestion request, but no longer than a timeout.
//
// returns true if closed by timeout
func (flagger *Flagger) Shutdown(timeout time.Duration) bool {
	flagger.mux.Lock()
	defer flagger.mux.Unlock()

	flagger.enabled = false

	flagger.core.SetConfig(nil)
	flagger.core.SetEntity(nil)

	// this could happen if Shutdown is called before init
	if flagger.sse != nil {
		flagger.sse.Shutdown()
		flagger.sse = nil
	}
	if flagger.ingester != nil {
		return flagger.ingester.Shutdown(timeout)
	}
	return false
}

// Publish explicitly notifies Airship about an Entity
func (flagger *Flagger) Publish(entity *core.Entity) {
	if entity == nil {
		log.Warnf("Could not publish because entity is empty")
		return
	}

	if entity.ID == "" {
		log.Warnf("Could not publish because entity.id is empty")
		return
	}

	escapedEntity := core.EscapeEntity(entity)

	flagger.checkFlaggerInitialized(func() {
		flagger.ingester.Publish(escapedEntity)
	})
}

// Track is simple event tracking API.
// Entity could be omitted if it has already been set before.
func (flagger *Flagger) Track(event *core.Event) {
	if event == nil {
		log.Warnf("Could not track because event is empty")
		return
	}

	if event.Name == "" {
		log.Warnf("Could not track because event.name is empty")
		return
	}

	if event.Entity != nil && event.Entity.ID == "" {
		log.Warnf("Could not track because event.entity.id is empty")
		return
	}

	escapedEvent := core.EscapeEvent(event)

	flagger.checkFlaggerInitialized(func() {
		flagger.ingester.Track(escapedEvent)
	})
}

func (flagger *Flagger) checkFlaggerInitialized(callback func()) {
	flagger.mux.RLock()
	if !flagger.enabled {
		// unlocking before calling to prevent synchronization
		flagger.mux.RUnlock()
		if flagger.silentInit() {
			callback()
		}
	} else {
		defer flagger.mux.RUnlock()
		callback()
	}
}

// SetEntity stores an entity in Flagger, which allows omission of entity in other API methods.
//
// If you don't provide __any__ entity to Flagger:
// - flag functions always resolve with the default variation
// - Track method doesn't record an event
//
// Rule of thumb: make sure you provided an entity to the Flagger
func (flagger *Flagger) SetEntity(entity *core.Entity) {
	// entity could be nil in case user wants to reset Flagger's scope entity to nil
	if entity != nil && entity.ID == "" {
		bytes, _ := json.Marshal(entity)
		log.Warnf("Could not setEntity because id is empty, entity: %+v", string(bytes))
		return
	}
	escapedEntity := core.EscapeEntity(entity)

	flagger.mux.Lock()
	flagger.core.SetEntity(escapedEntity)

	if flagger.ingester == nil {
		flagger.ingester = ingester.NewIngester(SDKInfo, firstExposuresIngestThreshold)
	}
	flagger.ingester.SetEntity(escapedEntity)
	flagger.mux.Unlock()

	bytes, _ := json.Marshal(escapedEntity)
	log.Debugf("New entity is set to Flagger, entity: %+v", string(bytes))
}

// IsEnabled checks whether a flag is enabled for an entity
func (flagger *Flagger) IsEnabled(codename string, entity *core.Entity) bool {
	escapedEntity := core.EscapeEntity(entity)

	var flagResult *core.FlagResult
	flagger.checkFlaggerInitialized(func() {
		flagResult = flagger.core.EvaluateFlag(codename, escapedEntity)
		flagger.ingestExposure("isEnabled", codename, flagResult)
	})

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("IsEnabled: %+v", string(bytes))
	if flagResult == nil {
		return false
	}
	return flagResult.Enabled
}

// IsSampled returns whether or not an entity is within one of the targeted populations.
// However, the entity may or may not be "sampled".
// A sampled entity may someday receive this feature, but this function only determines whether entity is sampled.
func (flagger *Flagger) IsSampled(codename string, entity *core.Entity) bool {
	escapedEntity := core.EscapeEntity(entity)

	var flagResult *core.FlagResult
	flagger.checkFlaggerInitialized(func() {
		flagResult = flagger.core.EvaluateFlag(codename, escapedEntity)
		flagger.ingestExposure("isSampled", codename, flagResult)
	})

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("IsSampled: %+v", string(bytes))
	if flagResult == nil {
		return false
	}
	return flagResult.Sampled
}

// GetVariation returns the variation that the entity will receive (after resolving all Flagging Rules).
// This is a more general flag function that is useful for multivariate flags.
func (flagger *Flagger) GetVariation(codename string, entity *core.Entity) string {
	escapedEntity := core.EscapeEntity(entity)

	var flagResult *core.FlagResult
	flagger.checkFlaggerInitialized(func() {
		flagResult = flagger.core.EvaluateFlag(codename, escapedEntity)
		flagger.ingestExposure("getVariation", codename, flagResult)
	})

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("GetVariation: %+v", string(bytes))
	if flagResult == nil {
		return core.DefaultVariation().Codename
	}
	return flagResult.Variation.Codename
}

// GetPayload returns the payload associated with the treatment assigned to the entity
func (flagger *Flagger) GetPayload(codename string, entity *core.Entity) core.Payload {
	escapedEntity := core.EscapeEntity(entity)

	var flagResult *core.FlagResult
	flagger.checkFlaggerInitialized(func() {
		flagResult = flagger.core.EvaluateFlag(codename, escapedEntity)
		flagger.ingestExposure("getPayload", codename, flagResult)
	})

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("GetPayload: %+v", string(bytes))
	if flagResult == nil {
		return core.DefaultVariation().Payload
	}
	return flagResult.Payload
}

// flagger must be initialized
// not thread safe
func (flagger *Flagger) ingestExposure(methodName, codename string, result *core.FlagResult) {
	// do not ingest if data is corrupted or there is nothing to ingest
	if result.Reason != core.CodenameIsEmpty &&
		result.Reason != core.NoEntityProvided &&
		result.Reason != core.FlaggerIsNotInitialized &&
		result.Reason != core.IDIsEmpty {
		exposure := &core.Exposure{
			Codename:     codename,
			HashKey:      result.Hashkey,
			Variation:    result.Variation.Codename,
			Entity:       result.Entity,
			MethodCalled: methodName,
			Timestamp:    time.Now(),
		}

		flagger.ingester.PublishExposure(exposure, result.IsNew)
	}
}
