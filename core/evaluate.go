package core

// FlagResult represent calculated flag result
type FlagResult struct {
	Hashkey   string
	Entity    *Entity
	Enabled   bool
	Sampled   bool
	Variation *FlagVariation
	Payload   Payload
	IsNew     bool
	Reason    Reason
}

// Reason represent type of flag reason
type Reason string

const (
	// NoEntityProvided represent flag reason for witch have no entity for calculating
	NoEntityProvided Reason = "No entity provided to Flagger"

	// ConfigIsEmpty
	FlaggerIsNotInitialized Reason = "Flagger is not initialized"

	// ConfigIsEmpty
	ConfigIsEmpty Reason = "No flags in the current config"

	// CodenameIsEmpty
	CodenameIsEmpty Reason = "Flag codename is empty"

	// IdIsEmpty
	IdIsEmpty Reason = "Id is empty"

	// FlagNotInConfig
	FlagNotInConfig Reason = "Flag is not in the current config"

	// KillSwitchEngaged
	KillSwitchEngaged Reason = "Kill switch engaged"

	// IndividualBlacklist
	IndividualBlacklist Reason = "Entity is individually blacklisted"

	// IndividualWhitelist
	IndividualWhitelist Reason = "Entity is individually whitelisted"

	// GroupBlacklist
	GroupBlacklist Reason = "Entity's group is blacklisted"

	// GroupWhitelist
	GroupWhitelist Reason = "Entity's group is whitelisted"

	// IsSampled
	IsSampled Reason = "Entity is sampled in the individual subpopulation"

	// IsSampledByGroup
	IsSampledByGroup Reason = "Entity is sampled in the group subpopulation"

	// Default
	Default Reason = "Default (off) treatment reached"
)

func newEmptyPayload() Payload {
	return make(Payload)
}

func newEmptyVariation() *FlagVariation {
	return &FlagVariation{
		Codename:    "off",
		Probability: 1.0,
		Payload:     newEmptyPayload(),
	}
}

func evaluateFlag(confHashKey string, flagConfig *FlagConfig, entity *Entity) *FlagResult {

	// kill switch
	if flagConfig.KillSwitchEngaged {
		return &FlagResult{
			Hashkey:   flagConfig.HashKey,
			Entity:    entity,
			Enabled:   false,
			Sampled:   false,
			Variation: newEmptyVariation(),
			Payload:   newEmptyPayload(),
			IsNew:     false,
			Reason:    KillSwitchEngaged,
		}
	}

	// individual blacklist
	for _, v := range flagConfig.Blacklist {
		if v.equals(entity) {
			return &FlagResult{
				Hashkey:   flagConfig.HashKey,
				Entity:    entity,
				Enabled:   false,
				Sampled:   false,
				Variation: newEmptyVariation(),
				Payload:   newEmptyPayload(),
				IsNew:     false,
				Reason:    IndividualBlacklist,
			}
		}
	}

	// individual whitelist
	for _, v := range flagConfig.Whitelist {
		if v.equals(entity) {
			variation := extractVariation(flagConfig, v.Variation)
			return &FlagResult{
				Hashkey:   flagConfig.HashKey,
				Entity:    entity,
				Enabled:   true,
				Sampled:   false,
				Variation: variation,
				Payload:   variation.Payload,
				IsNew:     false,
				Reason:    IndividualWhitelist,
			}
		}
	}

	// if entity belong to a group
	if group := entity.Group; group != nil {

		// group blacklist
		for _, v := range flagConfig.Blacklist {
			if v.equalsGroup(group) {
				return &FlagResult{
					Hashkey:   flagConfig.HashKey,
					Entity:    entity,
					Enabled:   false,
					Sampled:   false,
					Variation: newEmptyVariation(),
					Payload:   newEmptyPayload(),
					IsNew:     false,
					Reason:    GroupBlacklist,
				}
			}
		}

		// group whitelist
		for _, v := range flagConfig.Whitelist {
			if v.equalsGroup(group) {
				variation := extractVariation(flagConfig, v.Variation)
				return &FlagResult{
					Hashkey:   flagConfig.HashKey,
					Entity:    entity,
					Enabled:   true,
					Sampled:   false,
					Variation: variation,
					Payload:   variation.Payload,
					IsNew:     false,
					Reason:    GroupWhitelist,
				}
			}
		}
	}

	// individual sampling
	hash := samplingHash(confHashKey, flagConfig.HashKey, entity.ID, entity.Type)
	sp := sampleSubpopulation(hash, flagConfig.FlagSubPopulations, entity.Type, entity.Attributes)
	if sp != nil {
		hash := variationHash(flagConfig.Codename, entity.ID, entity.Type)
		variation := chooseVariation(hash, flagConfig.Variations)
		return &FlagResult{
			Hashkey:   flagConfig.HashKey,
			Entity:    entity,
			Enabled:   true,
			Sampled:   true,
			Variation: variation,
			Payload:   variation.Payload,
			IsNew:     false,
			Reason:    IsSampled,
		}
	}

	// group sampling
	if group := entity.Group; group != nil {
		hash := samplingHash(confHashKey, flagConfig.HashKey, group.ID, group.Type)
		sp := sampleSubpopulation(hash, flagConfig.FlagSubPopulations, group.Type, group.Attributes)
		if sp != nil {
			hash := variationHash(flagConfig.Codename, group.ID, group.Type)
			variation := chooseVariation(hash, flagConfig.Variations)
			return &FlagResult{
				Hashkey:   flagConfig.HashKey,
				Entity:    entity,
				Enabled:   true,
				Sampled:   true,
				Variation: variation,
				Payload:   variation.Payload,
				IsNew:     false,
				Reason:    IsSampledByGroup,
			}
		}
	}

	// default
	return &FlagResult{
		Hashkey:   flagConfig.HashKey,
		Entity:    entity,
		Enabled:   false,
		Sampled:   false,
		Variation: newEmptyVariation(),
		Payload:   newEmptyPayload(),
		IsNew:     false,
		Reason:    Default,
	}
}

func extractVariation(flagConfig *FlagConfig, codename string) *FlagVariation {
	for _, v := range flagConfig.Variations {
		if v.Codename == codename {
			return v
		}
	}
	return newEmptyVariation()
}

func variationHash(codename, id, Type string) float64 {
	// never change this key!!!
	key := codename + id + Type
	return HashMD5(key)
}

func chooseVariation(hash float64, variations []*FlagVariation) *FlagVariation {
	cumulativeSum := 0.0
	for _, v := range variations {
		cumulativeSum += v.Probability
		if hash <= cumulativeSum {
			return v
		}
	}
	return newEmptyVariation()
}

func samplingHash(envKey, hashKey, id, Type string) float64 {
	// never change this key!!!
	key := envKey + hashKey + id + Type
	return HashMD5(key)
}

func sampleSubpopulation(hash float64, subpopulations []*FlagSubpopulation, Type string, attr Attributes) *FlagSubpopulation {
	for _, v := range subpopulations {
		if v.EntityType == Type && hash < v.SamplingPercentage && matchByFilters(v.Filters, attr) {
			return v
		}
	}
	return nil
}