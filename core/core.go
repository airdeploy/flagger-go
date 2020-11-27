package core

import (
	"github.com/airdeploy/flagger-go/v3/log"
	"sync"
)

// check implementation on compile time
var _ interface {
	SetConfig(v *Configuration)
	SetEntity(entity *Entity)
	EvaluateFlag(codename string, entity *Entity) *FlagResult
} = new(Core)

// NewCore return the new instance Core
func NewCore() *Core {
	return &Core{}
}

// Core represent things for encapsulate business logic for flags calculation
type Core struct {
	configuration *Configuration
	entity        *Entity
	mux           sync.Mutex
}

// SetConfig represent callback function for insert incoming configuration
func (core *Core) SetConfig(v *Configuration) {
	if v != nil {
		v.Escape()
	}
	core.mux.Lock()
	core.configuration = v
	core.mux.Unlock()
}

// SetEntity represent function from main Flagger interface
func (core *Core) SetEntity(v *Entity) {
	core.mux.Lock()
	core.entity = v
	core.mux.Unlock()
}

// GetEntity represent method for return stored Entity
func (core *Core) GetEntity() *Entity {
	core.mux.Lock()
	defer core.mux.Unlock()
	return core.entity
}

// EvaluateFlag represent method for calculation Flag for Entity by codename
func (core *Core) EvaluateFlag(codename string, entity *Entity) *FlagResult {
	core.mux.Lock()
	configuration := core.configuration
	core.mux.Unlock()

	if codename == "" {
		log.Warnf("Codename is empty, returning \"off\" variation for entity:  %+v", entity)
		return &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     false,
			Reason:    CodenameIsEmpty,
		}
	}

	if configuration == nil {
		log.Warnf("Flagger is not initialized")
		return &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     true,
			Reason:    FlaggerIsNotInitialized,
		}
	}

	if len(configuration.Flags) == 0 {
		return &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     true,
			Reason:    ConfigIsEmpty,
		}
	}

	if entity == nil {
		core.mux.Lock()
		entity = core.entity
		core.mux.Unlock()
	}
	if entity == nil {
		return &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     false,
			Reason:    NoEntityProvided,
		}
	}

	if entity.ID == "" {
		log.Warnf("id is empty, returning \"off\" variation for codename \"%+v\" and entity %+v", codename, entity)
		return &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     false,
			Reason:    IDIsEmpty,
		}
	}

	for _, flagConfig := range configuration.Flags {
		if flagConfig.Codename == codename {
			return evaluateFlag(core.configuration.HashKey, flagConfig, entity) // success
		}
	}

	return &FlagResult{
		Hashkey:   "",
		Entity:    entity,
		Enabled:   false,
		Sampled:   false,
		Variation: newEmptyVariation(),
		Payload:   newEmptyPayload(),
		IsNew:     true,
		Reason:    FlagNotInConfig,
	}
}
