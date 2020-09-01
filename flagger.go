package flagger

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/ingester"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/airdeploy/flagger-go/v3/sse"
)

// check implementation public interface on compile time
var _ interface {
	Init(ctx context.Context, args *InitArgs) error
	Publish(ctx context.Context, entity *core.Entity)
	Track(ctx context.Context, event *core.Event)
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
func (flagger *Flagger) Init(ctx context.Context, args *InitArgs) error {
	args, sdkInfo, err := prepareInitArgs(args, SDKInfo) // this method return copy of *InitArgs and *core.SDKInfo
	if err != nil {
		return err
	}

	flagger.mux.Lock()
	defer flagger.mux.Unlock()

	// Ingester
	if flagger.ingester == nil {
		flagger.ingester = ingester.NewIngester(sdkInfo)
	} else {
		flagger.ingester.Shutdown(1000 * time.Millisecond)
		flagger.ingester = ingester.NewIngester(sdkInfo)
	}
	flagger.ingester.SetURL(args.IngestionURL)

	// get configuration from SourceURL/BackupSourceURL
	var configuration *core.Configuration
	err = getConfiguration(ctx, flagger.rt, args.SourceURL, defaultAttemptsConnection, &configuration)
	if err != nil {
		log.Warnf("Unable to fetch FlaggerConfiguration from SourceURL")
		err := getConfiguration(ctx, flagger.rt, args.BackupSourceURL, defaultAttemptsConnection, &configuration)
		if err != nil {
			log.Warnf("Unable to fetch FlaggerConfiguration from BackupSourceURL")
		} else {
			bytes, _ := json.Marshal(configuration)
			log.Debugf("init flagger from BackupSourceURL was success: %+v", string(bytes))
		}
	} else {
		bytes, _ := json.Marshal(configuration)
		log.Debugf("init flagger from SourceURL was success: %+v", string(bytes))
	}

	if configuration != nil {
		flagger.core.SetConfig(configuration)
		flagger.ingester.SetConfig(&configuration.SdkConfig)
	} else {
		// have no configuration
		flagger.core.SetConfig(nil)
		flagger.ingester.SetConfig(defaultSDKConfig)
	}

	// SSE
	if flagger.sse != nil {
		flagger.sse.Shutdown()
	}
	flagger.sse = sse.NewClient(func(v *core.Configuration) {
		log.Debugf("set new configuration")
		flagger.core.SetConfig(v)
		flagger.ingester.SetConfig(&v.SdkConfig)
	})
	flagger.sse.SetURL(args.SSEURL)
	return nil
}

// Shutdown ingests data(if any), stop ingester and closes SSE connection.
// Shutdown waits to finish current ingestion request, but no longer than a timeout.
//
// returns true if closed by timeout
func (flagger *Flagger) Shutdown(timeout time.Duration) bool {
	flagger.core.SetConfig(nil)
	flagger.sse.Shutdown()
	if flagger.ingester != nil {
		return flagger.ingester.Shutdown(timeout)
	}
	return false
}

// Explicitly notify Airship about an Entity
func (flagger *Flagger) Publish(ctx context.Context, entity *core.Entity) {
	if entity == nil {
		log.Warnf("Could not publish because entity is empty")
		return
	}

	if entity.ID == "" {
		log.Warnf("Could not publish because entity.id is empty")
		return
	}

	entity = core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagger.ingester.Publish(entity)
	flagger.mux.RUnlock()
}

// Simple event tracking API.
// Entity is an optional parameter if it was set before.
func (flagger *Flagger) Track(ctx context.Context, event *core.Event) {
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

	event = core.EscapeEvent(event)

	flagger.mux.RLock()
	flagger.ingester.Track(event)
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

	flagger.mux.RLock()
	flagger.core.SetEntity(core.EscapeEntity(entity))
	if flagger.ingester != nil {
		flagger.ingester.SetEntity(entity)
	}

	bytes, _ := json.Marshal(entity)
	log.Debugf("New entity is set to Flagger, entity: %+v", string(bytes))
	flagger.mux.RUnlock()
}

// Determines if flag is enabled for entity.
func (flagger *Flagger) IsEnabled(codename string, entity *core.Entity) bool {
	entity = core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, entity)
	flagger.ingestExposure("isEnabled", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("IsEnabled: %+v", string(bytes))
	return flagResult.Enabled
}

// Determines if entity is within the targeted subpopulations
func (flagger *Flagger) IsSampled(codename string, entity *core.Entity) bool {
	core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, entity)
	flagger.ingestExposure("isSampled", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("IsSampled: %+v", string(bytes))
	return flagResult.Sampled
}

// Returns the variation assigned to the entity in a multivariate flag
func (flagger *Flagger) GetVariation(codename string, entity *core.Entity) string {
	core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, entity)
	flagger.ingestExposure("getVariation", codename, flagResult)
	flagger.mux.RUnlock()

	bytes, _ := json.Marshal(flagResult)
	log.Debugf("GetVariation: %+v", string(bytes))
	return flagResult.Variation.Codename
}

// Returns the payload associated with the treatment assigned to the entity
func (flagger *Flagger) GetPayload(codename string, entity *core.Entity) core.Payload {
	core.EscapeEntity(entity)

	flagger.mux.RLock()
	flagResult := flagger.core.EvaluateFlag(codename, entity)
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
		result.Reason != core.IdIsEmpty {
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
