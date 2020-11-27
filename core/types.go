package core

import (
	"strings"
	"time"

	"github.com/airdeploy/flagger-go/v3/log"
)

// Configuration represent flagger configuration
type Configuration struct {
	HashKey   string        `json:"hashKey"`
	Flags     []*FlagConfig `json:"flags"`
	SdkConfig SDKConfig     `json:"sdkConfig,omitempty"`
}

// Escape represent method for escaping configuration
func (c *Configuration) Escape() {
	for _, f := range c.Flags {
		f.escape()
	}
}

// SDKConfig represent flagger SDK configuration
type SDKConfig struct {
	SDKIngestionInterval int `json:"SDK_INGESTION_INTERVAL,omitempty"`
	SDKIngestionMaxItems int `json:"SDK_INGESTION_MAX_CALLS,omitempty"`
}

// IngestionIntervalDuration converts seconds to time.Duration. Prevents interval less than 1
func (s *SDKConfig) IngestionIntervalDuration() time.Duration {
	// to prevent timer go crazy
	if s.SDKIngestionInterval < 1 {
		s.SDKIngestionInterval = 1
	}
	return time.Duration(s.SDKIngestionInterval) * time.Second
}

// Copy return copy instance SDKConfig
func (s *SDKConfig) Copy() *SDKConfig {
	return &SDKConfig{
		SDKIngestionInterval: s.SDKIngestionInterval,
		SDKIngestionMaxItems: s.SDKIngestionMaxItems,
	}
}

// SDKInfo represent flagger SDK information
type SDKInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Copy creates a copy of SDKInfo
func (s *SDKInfo) Copy() *SDKInfo {
	return &SDKInfo{
		Name:    s.Name,
		Version: s.Version,
	}
}

// FlagConfig represent flagger flag configuration
type FlagConfig struct {
	Codename           string               `json:"codename"`
	KillSwitchEngaged  bool                 `json:"killSwitchEngaged,omitempty"`
	HashKey            string               `json:"hashkey,omitempty"`
	Variations         []*FlagVariation     `json:"variations,omitempty"`
	FlagSubPopulations []*FlagSubpopulation `json:"subpopulations,omitempty"`
	Blacklist          []*Entity            `json:"blacklist,omitempty"`
	Whitelist          []*Entity            `json:"whitelist,omitempty"`
}

func (fc *FlagConfig) escape() {
	for _, fs := range fc.FlagSubPopulations {
		fs.escape()
	}
}

// Entity represent flagger entity
type Entity struct {
	ID         string     `json:"id"`
	Type       string     `json:"type,omitempty"`
	Name       string     `json:"name,omitempty"`
	Variation  string     `json:"variation,omitempty"` // used only in whitelist
	Group      *Group     `json:"group,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// EscapeEntity creates a new entity with all fields escaped
func EscapeEntity(e *Entity) *Entity {
	if e == nil {
		return nil
	}
	res := Entity{
		ID:         e.ID,
		Type:       e.Type,
		Name:       e.Name,
		Variation:  e.Variation,
		Group:      nil,
		Attributes: nil,
	}

	//first lowercase all attribute keys
	if res.Attributes == nil {
		res.Attributes = Attributes{}
	}

	if e.Attributes != nil {
		res.Attributes = escapeAttributes(e.Attributes)
	}

	// propagate "name" and "id" to attributes if not exists
	if _, ok := res.Attributes["name"]; !ok && res.Name != "" {
		res.Attributes["name"] = res.Name
	}
	if _, ok := res.Attributes["id"]; !ok {
		res.Attributes["id"] = res.ID
	}

	// propagate default type for Entity
	if res.Type == "" {
		res.Type = "User"
	}

	if e.Group != nil {
		res.Group = escapeGroup(e.Group)
	}

	return &res
}

// lower casing attributes keys and propagate name and id to attributes
// returns pointer to new group
func escapeGroup(g *Group) *Group {
	res := Group{
		ID:         g.ID,
		Type:       g.Type,
		Name:       g.Name,
		Attributes: nil,
	}

	res.Attributes = Attributes{}
	for k, v := range g.Attributes {
		res.Attributes[k] = v
	}

	// lowercase all attribute keys
	res.Attributes = escapeAttributes(res.Attributes)

	// propagate "name" and "id" to attributes if not exists
	if _, ok := res.Attributes["name"]; !ok && res.Name != "" {
		res.Attributes["name"] = res.Name
	}
	if _, ok := res.Attributes["id"]; !ok {
		res.Attributes["id"] = res.ID
	}
	return &res
}

func (e *Entity) equals(entity *Entity) bool {
	return e.ID == entity.ID && strings.EqualFold(e.Type, entity.Type)
}

func (e *Entity) equalsGroup(group *Group) bool {
	return e.ID == group.ID && strings.EqualFold(e.Type, group.Type)
}

// Group represent flagger group from entities
type Group struct {
	ID         string     `json:"id"`
	Type       string     `json:"type,omitempty"`
	Name       string     `json:"name,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// Attributes must be a flat map with values one of these values: string, int, float or bool.
// escapeAttributes function satisfies this invariant
type Attributes map[string]interface{}

// sets all keys to lowercase and filters out keys-value pairs with invalid value
func escapeAttributes(attributes Attributes) Attributes {
	var res = make(Attributes)
	for key, value := range attributes {
		switch v := value.(type) {
		case int:
			res[strings.ToLower(key)] = float64(v)
		case float32:
			res[strings.ToLower(key)] = float64(v)
		case bool, string, float64:
			res[strings.ToLower(key)] = v
		}
	}
	return res
}

// FlagVariation represent variation entity of Flag
type FlagVariation struct {
	Codename    string  `json:"codename"`
	Probability float64 `json:"probability"`
	Payload     Payload `json:"payload"`
}

// Payload represent Flag payload
type Payload map[string]interface{}

// FlagSubpopulation represent subpopulation entity of Flag
type FlagSubpopulation struct {
	EntityType         string        `json:"entityType"`
	SamplingPercentage float64       `json:"samplingPercentage"`
	Filters            []*FlagFilter `json:"filters"`
}

func (fs *FlagSubpopulation) escape() {
	var result = make([]*FlagFilter, 0, len(fs.Filters))

	// filter out empty Operators and EscapeEntity Filter
	for _, filter := range fs.Filters {
		if filter.Operator.isValid() {
			filter.escape()
			result = append(result, filter)
		}
	}
	fs.Filters = result
}

// Operator represent filter operator
type Operator string

const (
	is    Operator = "IS"
	isNot Operator = "IS_NOT"
	lt    Operator = "LT"
	lte   Operator = "LTE"
	gt    Operator = "GT"
	gte   Operator = "GTE"
	in    Operator = "IN"
	notIn Operator = "NOT_IN"
)

var supportedOperators = []Operator{is, isNot, lt, lte, gt, gte, in, notIn}

func (o Operator) isValid() bool {
	for _, valid := range supportedOperators {
		if valid == o {
			return true
		}
	}
	return false
}

// FlagFilter represent one flag filter entity
type FlagFilter struct {
	AttributeName string      `json:"attributeName"`
	Operator      Operator    `json:"operator"`
	Value         FilterValue `json:"value"`
	FilterType    string      `json:"type"`
}

func (ff *FlagFilter) escape() {
	ff.AttributeName = strings.ToLower(ff.AttributeName)
	if ff.FilterType == filterTypeDate {

		if ss, ok := ff.Value.(string); ok {
			ts, err := time.Parse(time.RFC3339, ss)
			if err != nil {
				log.Warnf("Cannot parse value \"%+v\" for attribute \"%+v\" as RFC3339(\"%+v\")", ff.Value, ff.AttributeName, time.RFC3339)
				return
			}
			ff.Value = ts
		}

		if ss, ok := ff.Value.([]string); ok {
			tss := make([]time.Time, 0, len(ss))
			for _, s := range ss {
				ts, err := time.Parse(time.RFC3339, s)
				if err != nil {
					log.Warnf("Cannot parse value \"%+v\" for attribute \"%+v\" as RFC3339(\"%+v\")", ff.Value, ff.AttributeName, time.RFC3339)
					continue
				}
				tss = append(tss, ts)
			}
			ff.Value = tss
		}
	}

	// fix json unmarshal array of strings into []interface{}
	if ff.Operator == in || ff.Operator == notIn {
		switch values := ff.Value.(type) {
		case []interface{}:
			if ff.FilterType == filterTypeString {
				var outPutValue []string
				for _, val := range values {
					stringValue, ok := val.(string)
					if ok {
						outPutValue = append(outPutValue, stringValue)
					}
				}
				ff.Value = outPutValue
			}

			if ff.FilterType == filterTypeNumber {
				var outPutValue []float64
				for _, val := range values {
					floatValue, ok := val.(float64)
					if ok {
						outPutValue = append(outPutValue, floatValue)
					}
				}
				ff.Value = outPutValue
			}

			if ff.FilterType == filterTypeBool {
				var outPutValue []bool
				for _, val := range values {
					boolValue, ok := val.(bool)
					if ok {
						outPutValue = append(outPutValue, boolValue)
					}
				}
				ff.Value = outPutValue
			}

			if ff.FilterType == filterTypeDate {
				var outPutValue []time.Time
				for _, val := range values {
					stringValue, ok := val.(string) // converting interface to string
					if ok {
						ts, err := time.Parse(time.RFC3339, stringValue)
						if err == nil {
							outPutValue = append(outPutValue, ts)
						}
					}
				}
				ff.Value = outPutValue
			}
		}
	}
}

// FilterValue placeholder for one of these values: [ int | float | string | bool | []int, []float | []string | []bool ]
type FilterValue interface{}

// Event represent flagger event
type Event struct {
	Name            string     `json:"name"`
	EventProperties Attributes `json:"eventProperties"`
	Entity          *Entity    `json:"entity,omitempty"`
}

// EscapeEvent represent method for escaping event
func EscapeEvent(event *Event) *Event {
	return &Event{
		Name:            event.Name,
		EventProperties: escapeAttributes(event.EventProperties),
		Entity:          EscapeEntity(event.Entity),
	}
}

// Exposure represent flagger exposure
type Exposure struct {
	Codename     string    `json:"codename"`
	HashKey      string    `json:"hashkey,omitempty"`
	Variation    string    `json:"variation"`
	Entity       *Entity   `json:"entity"`
	MethodCalled string    `json:"methodCalled"`
	Timestamp    time.Time `json:"timestamp"`
}
