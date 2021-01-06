package flagger

import (
	"github.com/airdeploy/flagger-go/v3/internal/httputils"
	"net/http"
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
}

// InitArgs represent init arguments for Flagger
type InitArgs struct {
	APIKey          string
	SourceURL       string
	BackupSourceURL string
	IngestionURL    string
	SSEURL          string
}

// Init gets FlaggerConfiguration, establishes and maintains SSE connections and initialize Ingester
func (flagger *Flagger) Init(args *InitArgs) error {
	args, sdkInfo, err := prepareInitArgs(args, SDKInfo) // this method return copy of *InitArgs and *core.SDKInfo
	if err != nil {
		return err
	}

	flagger.mux.Lock()
	defer flagger.mux.Unlock()

	flagger.Shutdown(1 * time.Second)

	// Ingester
	flagger.ingester = ingester.NewIngester(sdkInfo, firstExposuresIngestThreshold)

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

// Shutdown ingests data(if any), stops ingester and closes SSE connection.
// Shutdown waits to finish current ingestion request, but no longer than a timeout.
//
// returns true if closed by timeout
func (flagger *Flagger) Shutdown(timeout time.Duration) bool {
	flagger.core.SetConfig(nil)
	flagger.core.SetEntity(nil)

	// this could happen if Shutdown is called before init
	if flagger.sse != nil {
		flagger.sse.Shutdown()
		flagger.sse = nil
	}
	if flagger.ingester != nil {
		defer func() {
			flagger.ingester = nil
		}()
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

	flagger.mux.RLock()
	flagger.ingester.Publish(escapedEntity)
	flagger.mux.RUnlock()
}

// Track is simple event tracking API.
// Entity is an optional parameter if it was set before.
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

	flagger.mux.RLock()
	flagger.ingester.Track(escapedEvent)
	flagger.mux.RUnlock()
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
	flagger.mux.RLock()
	flagger.core.SetEntity(escapedEntity)
	if flagger.ingester != nil {
		flagger.ingester.SetEntity(escapedEntity)
	}
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(escapedEntity)
	log.Debugf("New entity is set to Flagger, entity: %+v", string(bytes))
}

// IsEnabled checks whether a flag is enabled for an entity
func (flagger *Flagger) IsEnabled(codename string, entity *core.Entity) bool {
	escapedEntity := core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, escapedEntity)
	flagger.ingestExposure("isEnabled", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("IsEnabled: %+v", string(bytes))
	return flagResult.Enabled
}

// IsSampled returns whether or not an entity is within one of the targeted populations.
// However, the entity may or may not be "sampled".
// A sampled entity may someday receive this feature, but this function only determines whether entity is sampled.
func (flagger *Flagger) IsSampled(codename string, entity *core.Entity) bool {
	escapedEntity := core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, escapedEntity)
	flagger.ingestExposure("isSampled", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("IsSampled: %+v", string(bytes))
	return flagResult.Sampled
}

// GetVariation returns the variation that the entity will receive (after resolving all Flagging Rules).
// This is a more general flag function that is useful for multivariate flags.
func (flagger *Flagger) GetVariation(codename string, entity *core.Entity) string {
	escapedEntity := core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, escapedEntity)
	flagger.ingestExposure("getVariation", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("GetVariation: %+v", string(bytes))
	return flagResult.Variation.Codename
}

// GetPayload returns the payload associated with the treatment assigned to the entity
func (flagger *Flagger) GetPayload(codename string, entity *core.Entity) core.Payload {
	escapedEntity := core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, escapedEntity)
	flagger.ingestExposure("getPayload", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("GetPayload: %+v", string(bytes))
	return flagResult.Payload
}

func (flagger *Flagger) ingestExposure(methodName, codename string, result *core.FlagResult) {
	if flagger.ingester != nil &&
		// do not ingest if data is corrupted or there is nothing to ingest
		result.Reason != core.CodenameIsEmpty &&
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
