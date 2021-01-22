package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_evaluateFlag(t *testing.T) {
	t.Run("kill switch", func(t *testing.T) {
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "11"},
				Hashkey:   "hashkey",
				Enabled:   false,
				Sampled:   false,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    KillSwitchEngaged,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: true,
				},
				&Entity{ID: "11"}))
	})

	t.Run("individual blacklist", func(t *testing.T) {
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "12", Type: "User"},
				Hashkey:   "hashkey",
				Enabled:   false,
				Sampled:   false,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    IndividualBlacklist,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: false,
					Blacklist: []*Entity{
						{ID: "12", Type: "User"},
					},
				},
				&Entity{ID: "12", Type: "User"}))
	})

	t.Run("individual whitelist", func(t *testing.T) {
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "12", Type: "User"},
				Hashkey:   "hashkey",
				Enabled:   true,
				Sampled:   false,
				Variation: &FlagVariation{Codename: "data", Probability: 1.0, Payload: Payload{"payload": 1}},
				Payload:   Payload{"payload": 1},
				Reason:    IndividualWhitelist,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: false,
					Blacklist: []*Entity{
						{ID: "15", Type: "User"},
						{ID: "12", Type: "Agents"},
					},
					Whitelist: []*Entity{
						{ID: "12", Type: "User", Variation: "data"},
					},
					Variations: []*FlagVariation{
						{
							Codename:    "data",
							Probability: 1.0,
							Payload:     Payload{"payload": 1},
						},
					},
				},
				&Entity{ID: "12", Type: "User"}))
	})

	t.Run("group blacklist", func(t *testing.T) {
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}},
				Hashkey:   "hashkey",
				Enabled:   false,
				Sampled:   false,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    GroupBlacklist,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: false,
					Blacklist: []*Entity{
						{ID: "15", Type: "User"},
						{ID: "37", Type: "Group"},
					},
					Whitelist: []*Entity{
						{ID: "12", Type: "User"},
						{ID: "97", Type: "Group"},
					},
				},
				&Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}}))
	})

	t.Run("group whitelist", func(t *testing.T) {
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}},
				Hashkey:   "hashkey",
				Enabled:   true,
				Sampled:   false,
				Variation: &FlagVariation{Codename: "data2", Probability: 0.8, Payload: Payload{"payload": 2}},
				Payload:   Payload{"payload": 2},
				Reason:    GroupWhitelist,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: false,
					Blacklist: []*Entity{
						{ID: "15", Type: "User"},
						{ID: "43", Type: "User"},
					},
					Whitelist: []*Entity{
						{ID: "12", Type: "User"},
						{ID: "37", Type: "Group", Variation: "data2"},
					},
					Variations: []*FlagVariation{
						{
							Codename:    "data1",
							Probability: 0.2,
							Payload:     Payload{"payload": 1},
						},
						{
							Codename:    "data2",
							Probability: 0.8,
							Payload:     Payload{"payload": 2},
						},
					},
				},
				&Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}}))
	})

	t.Run("individual policy always beats group policy", func(t *testing.T) {
		// Whitelist entity + Blacklist group => isEnabled true
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}},
				Hashkey:   "hashkey",
				Enabled:   true,
				Sampled:   false,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    IndividualWhitelist,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: false,
					Blacklist: []*Entity{
						{ID: "37", Type: "Group"},
					},
					Whitelist: []*Entity{
						{ID: "31", Type: "User"},
					},
				},
				&Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}}))

		// whitelist group + blacklist entity => flag is off
		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}},
				Hashkey:   "hashkey",
				Enabled:   false,
				Sampled:   false,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    IndividualBlacklist,
			},
			evaluateFlag(
				"",
				&FlagConfig{
					HashKey:           "hashkey",
					KillSwitchEngaged: false,
					Blacklist: []*Entity{
						{ID: "31", Type: "User"},
					},
					Whitelist: []*Entity{
						{ID: "37", Type: "Group"},
					},
				},
				&Entity{ID: "31", Type: "User", Group: &Group{ID: "37", Type: "Group"}}))
	})

	t.Run("individual sampling", func(t *testing.T) {
		assert.Equal(t, 0.6221520481720589, samplingHash("envKey", "hashKey1", "27", "User"))
		assert.Equal(t, 0.8797622552514648, variationHash("color", "27", "User"))

		assert.Equal(t,
			&FlagResult{
				Entity: &Entity{
					ID:         "27",
					Type:       "User",
					Attributes: Attributes{"country": "FR", "fire": true},
				},
				Hashkey:   "hashKey1",
				Enabled:   true,
				Sampled:   true,
				Variation: &FlagVariation{Codename: "data1", Probability: 0.9, Payload: Payload{"payload": 1}},
				Payload:   Payload{"payload": 1},
				Reason:    IsSampled,
			},
			evaluateFlag(
				"envKey",
				&FlagConfig{
					Codename:          "color",
					KillSwitchEngaged: false,
					HashKey:           "hashKey1",
					Variations: []*FlagVariation{
						{Codename: "data1", Probability: 0.9, Payload: Payload{"payload": 1}},
						{Codename: "data2", Probability: 0.4, Payload: Payload{"payload": 2}},
					},
					FlagSubPopulations: []*FlagSubpopulation{
						{ // don`t match by country filter
							EntityType:         "User",
							SamplingPercentage: 0.3,
							Filters: []*FlagFilter{
								{AttributeName: "country", Operator: "in", Value: []string{"RU", "JP"}, FilterType: "string"},
								{AttributeName: "fire", Operator: "is", Value: true, FilterType: "boolean"},
							},
						},
						{
							EntityType:         "User",
							SamplingPercentage: 0.7,
							Filters: []*FlagFilter{
								{AttributeName: "country", Operator: "IN", Value: []string{"FR", "UA"}, FilterType: "string"},
								{AttributeName: "fire", Operator: "IS", Value: true, FilterType: "boolean"},
							},
						},
					},
				},
				&Entity{
					ID:         "27",
					Type:       "User",
					Attributes: Attributes{"country": "FR", "fire": true},
				}))
	})

	t.Run("group sampling", func(t *testing.T) {
		assert.Equal(t, 0.18881596249145105, samplingHash("envKey3", "hashKey1", "41", "User"))
		assert.Equal(t, 0.2713666612754982, variationHash("btc", "41", "User"))

		assert.Equal(t, 0.6319170796034438, samplingHash("envKey3", "hashKey1", "78", "Mob"))
		assert.Equal(t, 0.3486789963884541, variationHash("btc", "78", "Mob"))

		assert.Equal(t,
			&FlagResult{
				Entity: &Entity{
					ID:    "41",
					Type:  "User",
					Group: &Group{ID: "78", Type: "Mob"},
				},
				Hashkey:   "hashKey1",
				Enabled:   true,
				Sampled:   true,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    IsSampledByGroup,
			},
			evaluateFlag(
				"envKey3",
				&FlagConfig{
					Codename:          "btc",
					KillSwitchEngaged: false,
					HashKey:           "hashKey1",
					Variations:        []*FlagVariation{},
					FlagSubPopulations: []*FlagSubpopulation{
						{ // don`t match by EntityType
							EntityType:         "Web",
							SamplingPercentage: 0.4,
						},
						{
							EntityType:         "Mob",
							SamplingPercentage: 0.7,
						},
					},
				},
				&Entity{
					ID:    "41",
					Type:  "User",
					Group: &Group{ID: "78", Type: "Mob"},
				}))

		assert.Equal(t,
			&FlagResult{
				Entity: &Entity{
					ID:    "41",
					Type:  "User",
					Group: &Group{ID: "78", Type: "Company"},
				},
				Hashkey: "",
				Enabled: true,
				Sampled: true,
				Variation: &FlagVariation{
					Codename:    "tree",
					Probability: 0.5,
					Payload:     Payload{},
				},
				Payload: defaultPayload(),
				Reason:  IsSampledByGroup,
			},
			evaluateFlag(
				"1",
				&FlagConfig{
					Codename:          "org-chart",
					KillSwitchEngaged: false,
					HashKey:           "",
					Variations: []*FlagVariation{{
						Codename:    "expanding",
						Probability: 0.5,
						Payload:     Payload{},
					},
						{
							Codename:    "tree",
							Probability: 0.5,
							Payload:     Payload{},
						},
					},
					FlagSubPopulations: []*FlagSubpopulation{
						{
							EntityType:         "Company",
							SamplingPercentage: 1,
						},
					},
				},
				&Entity{
					ID:    "41",
					Type:  "User",
					Group: &Group{ID: "78", Type: "Company"},
				}))
	})

	t.Run("default", func(t *testing.T) {
		assert.Equal(t, 0.8453842981876637, samplingHash("envKey5", "hashKey2", "65", "Web"))
		assert.Equal(t, 0.4718345265441504, variationHash("ETH", "65", "Web"))

		assert.Equal(t, 0.7895209201024632, samplingHash("envKey5", "hashKey2", "33", "Mob"))
		assert.Equal(t, 0.21944960234667907, variationHash("ETH", "33", "Mob"))

		assert.Equal(t,
			&FlagResult{
				Entity:    &Entity{ID: "65", Type: "Web", Group: &Group{ID: "33", Type: "Mob"}},
				Hashkey:   "hashKey5",
				Enabled:   false,
				Sampled:   false,
				Variation: DefaultVariation(),
				Payload:   defaultPayload(),
				Reason:    Default,
			},
			evaluateFlag(
				"envKey5",
				&FlagConfig{
					Codename:          "ETH",
					KillSwitchEngaged: false,
					HashKey:           "hashKey5",
					Variations:        []*FlagVariation{
						//{Codename: "data1", Probability: 0.2, Payload: "payload1"},
						//{Codename: "data2", Probability: 0.4, Payload: "payload2"},
						//{Codename: "data3", Probability: 0.4, Payload: "payload3"},
					},
					FlagSubPopulations: []*FlagSubpopulation{
						{ // don`t match by country filter and fire
							EntityType:         "Web",
							SamplingPercentage: 0.1,
						},
						{
							EntityType:         "Mob",
							SamplingPercentage: 0.1,
						},
					},
					Blacklist: []*Entity{
						{ID: "66", Type: "Mob"},
						{ID: "12", Type: "User"},
					},
					Whitelist: []*Entity{
						{ID: "67", Type: "User"},
						{ID: "39", Type: "Mob"},
						{ID: "36", Type: "Mob"},
					},
				},
				&Entity{
					ID:    "65",
					Type:  "Web",
					Group: &Group{ID: "33", Type: "Mob"},
				}))
	})
}

func Test_extractVariation(t *testing.T) {
	// positive
	assert.Equal(t,
		&FlagVariation{Codename: "codename4", Probability: 0.1},
		extractVariation(
			&FlagConfig{
				Variations: []*FlagVariation{
					{Codename: "codename1", Probability: 0.1},
					{Codename: "codename2", Probability: 0.1},
					{Codename: "codename3", Probability: 0.1},
					{Codename: "codename4", Probability: 0.1},
					{Codename: "codename5", Probability: 0.1},
				},
			},
			"codename4"))

	// have no Variation by codename
	assert.Equal(t,
		DefaultVariation(),
		extractVariation(
			&FlagConfig{
				Variations: []*FlagVariation{
					{Codename: "codename1", Probability: 0.1},
					{Codename: "codename2", Probability: 0.1},
					{Codename: "codename3", Probability: 0.1},
					{Codename: "codename4", Probability: 0.1},
					{Codename: "codename5", Probability: 0.1},
				},
			},
			"codename7"))
}

func Test_chooseVariation(t *testing.T) {
	// empty variations
	assert.Equal(t,
		DefaultVariation(),
		chooseVariation(
			0.3,
			[]*FlagVariation{}))

	// 0.0 hash
	assert.Equal(t,
		&FlagVariation{Codename: "F1", Probability: 0.3},
		chooseVariation(
			0.0,
			[]*FlagVariation{
				{Codename: "F1", Probability: 0.3},
				{Codename: "F2", Probability: 0.7},
			}))

	// simple
	assert.Equal(t,
		&FlagVariation{Codename: "F1", Probability: 0.3},
		chooseVariation(
			0.2,
			[]*FlagVariation{
				{Codename: "F1", Probability: 0.3},
				{Codename: "F2", Probability: 0.7},
			}))

	// 1.0 hash
	assert.Equal(t,
		&FlagVariation{Codename: "F2", Probability: 0.7},
		chooseVariation(
			1.0,
			[]*FlagVariation{
				{Codename: "F1", Probability: 0.3},
				{Codename: "F2", Probability: 0.7},
			}))

	// real life case
	assert.Equal(t,
		&FlagVariation{Codename: "F4", Probability: 0.3},
		chooseVariation(
			0.7,
			[]*FlagVariation{
				{Codename: "F1", Probability: 0.2},
				{Codename: "F2", Probability: 0.1},
				{Codename: "F3", Probability: 0.2},
				{Codename: "F4", Probability: 0.3},
				{Codename: "F5", Probability: 0.2},
			}))
}

func Test_sampleSubpopulation(t *testing.T) {
	// without filters
	assert.Equal(t,
		&FlagSubpopulation{
			EntityType:         "User",
			SamplingPercentage: 0.4,
			Filters:            nil,
		},
		sampleSubpopulation(
			0.3,
			[]*FlagSubpopulation{
				{
					EntityType:         "User",
					SamplingPercentage: 0.2,
					Filters:            nil,
				},
				{
					EntityType:         "User",
					SamplingPercentage: 0.4,
					Filters:            nil,
				},
				{
					EntityType:         "User",
					SamplingPercentage: 0.5,
					Filters:            nil,
				},
			},
			"User",
			Attributes{}))

	// with filters
	assert.Equal(t,
		&FlagSubpopulation{
			EntityType:         "User",
			SamplingPercentage: 0.4,
			Filters: []*FlagFilter{
				{
					AttributeName: "country",
					Operator:      "IN",
					Value:         "JP",
					FilterType:    "string",
				},
			},
		},
		sampleSubpopulation(
			0.3,
			[]*FlagSubpopulation{
				{
					EntityType:         "User",
					SamplingPercentage: 0.2,
					Filters:            nil,
				},
				{
					EntityType:         "User",
					SamplingPercentage: 0.4,
					Filters: []*FlagFilter{
						{
							AttributeName: "country",
							Operator:      "IN",
							Value:         "UA",
							FilterType:    "string",
						},
					},
				},
				{
					EntityType:         "User",
					SamplingPercentage: 0.4,
					Filters: []*FlagFilter{
						{
							AttributeName: "country",
							Operator:      "IN",
							Value:         "JP",
							FilterType:    "string",
						},
					},
				},
				{
					EntityType:         "User",
					SamplingPercentage: 0.5,
					Filters:            nil,
				},
			},
			"User",
			Attributes{
				"country": "JP",
			}))
}

func TestEntity_equals(t *testing.T) {
	for _, tt := range []struct {
		e1    *Entity
		e2    *Entity
		equal bool
	}{
		{
			e1:    &Entity{ID: "1122", Type: "User"},
			e2:    &Entity{ID: "1122", Type: "User"},
			equal: true,
		},
		{
			e1:    &Entity{ID: "1122", Type: "User"},
			e2:    &Entity{ID: "1122", Type: "user"},
			equal: true,
		},
		{
			e1:    &Entity{ID: "1133", Type: "User"},
			e2:    &Entity{ID: "1122", Type: "User"},
			equal: false,
		},
		{
			e1:    &Entity{ID: "1122", Type: "Group"},
			e2:    &Entity{ID: "1122", Type: "User"},
			equal: false,
		},
	} {
		assert.Equal(t, tt.equal, tt.e1.equals(tt.e2))
	}
}

func TestEntity_equalsGroup(t *testing.T) {
	for _, tt := range []struct {
		entity *Entity
		group  *Group
		equal  bool
	}{
		{
			entity: &Entity{ID: "1122", Type: "User"},
			group:  &Group{ID: "1122", Type: "User"},
			equal:  true,
		},
		{
			entity: &Entity{ID: "1122", Type: "User"},
			group:  &Group{ID: "1122", Type: "user"},
			equal:  true,
		},
		{
			entity: &Entity{ID: "1133", Type: "User"},
			group:  &Group{ID: "1122", Type: "User"},
			equal:  false,
		},
		{
			entity: &Entity{ID: "1122", Type: "Group"},
			group:  &Group{ID: "1122", Type: "User"},
			equal:  false,
		},
	} {
		assert.Equal(t, tt.equal, tt.entity.equalsGroup(tt.group))
	}
}
