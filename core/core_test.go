package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCore_EvaluateFlag(t *testing.T) {
	t.Run("empty codename", func(t *testing.T) {
		core := &Core{}

		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename2", HashKey: "hashkey2"},
				{Codename: "codename3", HashKey: "hashkey3"},
			},
		})

		entity := &Entity{ID: "ID_2"}
		r := core.EvaluateFlag("", entity)

		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     false,
			Reason:    CodenameIsEmpty,
		}, r)
	})

	t.Run("empty entity id", func(t *testing.T) {
		core := &Core{}

		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename2", HashKey: "hashkey2"},
				{Codename: "codename3", HashKey: "hashkey3"},
			},
		})

		entity := &Entity{ID: ""}
		r := core.EvaluateFlag("test", entity)

		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     false,
			Reason:    IdIsEmpty,
		}, r)
	})
	t.Run("empty config", func(t *testing.T) {
		core := &Core{}
		r := core.EvaluateFlag("codename", nil)
		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    nil,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     true,
			Reason:    FlaggerIsNotInitialized,
		}, r)

		core.SetConfig(&Configuration{})
		r = core.EvaluateFlag("codename", nil)
		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    nil,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     true,
			Reason:    ConfigIsEmpty,
		}, r)
	})

	t.Run("empty entity", func(t *testing.T) {
		core := &Core{}
		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename2", HashKey: "hashkey2"},
				{Codename: "codename3", HashKey: "hashkey3"},
			},
		})

		r := core.EvaluateFlag("codename2", nil)
		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    nil,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    NoEntityProvided,
		}, r)

		// internal entity
		core.SetEntity(&Entity{ID: "ID_1"})
		r = core.EvaluateFlag("codename3", nil)
		assert.Equal(t, &FlagResult{
			Entity:    &Entity{ID: "ID_1"},
			Hashkey:   "hashkey3",
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    Default,
		}, r)

		// external entity
		r = core.EvaluateFlag("codename2", &Entity{ID: "ID_2"})
		assert.Equal(t, &FlagResult{
			Entity:    &Entity{ID: "ID_2"},
			Hashkey:   "hashkey2",
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    Default,
		}, r)
	})

	t.Run("have no flag", func(t *testing.T) {
		core := &Core{}
		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename2", HashKey: "hashkey2"},
				{Codename: "codename3", HashKey: "hashkey3"},
			},
		})
		r := core.EvaluateFlag("codename5", &Entity{ID: "3315"})
		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    &Entity{ID: "3315"},
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     true,
			Reason:    FlagNotInConfig,
		}, r)
	})

	t.Run("change config", func(t *testing.T) {
		core := &Core{}

		// first config
		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename2", HashKey: "hashkey2"},
				{Codename: "codename3", HashKey: "hashkey3"},
			},
		})
		assert.Equal(t, &FlagResult{
			Hashkey:   "hashkey2",
			Entity:    &Entity{ID: "3315"},
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    Default,
		}, core.EvaluateFlag("codename2", &Entity{ID: "3315"}))

		// second config
		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename5", HashKey: "hashkey5"},
				{Codename: "codename6", HashKey: "hashkey6"},
				{Codename: "codename7", HashKey: "hashkey7"},
			},
		})

		assert.Equal(t, &FlagResult{
			Hashkey:   "",
			Entity:    &Entity{ID: "3312"},
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     true,
			Reason:    FlagNotInConfig,
		}, core.EvaluateFlag("codename2", &Entity{ID: "3312"}))

		assert.Equal(t, &FlagResult{
			Hashkey:   "hashkey6",
			Entity:    &Entity{ID: "32"},
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    Default,
		}, core.EvaluateFlag("codename6", &Entity{ID: "32"}))
	})

	t.Run("change entity", func(t *testing.T) {
		core := &Core{}
		core.SetConfig(&Configuration{
			Flags: []*FlagConfig{
				{Codename: "codename1", HashKey: "hashkey1"},
				{Codename: "codename2", HashKey: "hashkey2"},
				{Codename: "codename3", HashKey: "hashkey3"},
			},
		})

		// first entity
		core.SetEntity(&Entity{ID: "1"})
		assert.Equal(t, &FlagResult{
			Hashkey:   "hashkey2",
			Entity:    &Entity{ID: "1"},
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    Default,
		}, core.EvaluateFlag("codename2", nil))

		// second entity
		core.SetEntity(&Entity{ID: "2"})
		assert.Equal(t, &FlagResult{
			Hashkey:   "hashkey3",
			Entity:    &Entity{ID: "2"},
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			Reason:    Default,
		}, core.EvaluateFlag("codename3", nil))
	})
}
