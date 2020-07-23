package core

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntityEscape(t *testing.T) {
	idOnly := &Entity{
		ID: "123",
	}
	idOnly = EscapeEntity(idOnly)
	assert.Equal(t, "User", idOnly.Type)
	assert.Equal(t, "123", idOnly.Attributes["id"])
	assert.Nil(t, idOnly.Attributes["name"])

	idAndName := &Entity{
		ID:   "321",
		Name: "test",
	}
	idAndName = EscapeEntity(idAndName)

	assert.Equal(t, idAndName.Type, "User")
	assert.Equal(t, idAndName.Attributes["id"], "321")
	assert.Equal(t, idAndName.Attributes["name"], "test")

	idAndType := &Entity{
		ID:   "1233432",
		Type: "Company",
	}
	idAndType = EscapeEntity(idAndType)
	assert.Equal(t, "Company", idAndType.Type)
	assert.Equal(t, "1233432", idAndType.Attributes["id"])
	assert.Nil(t, idAndType.Attributes["name"])

	idWithAttributes := &Entity{
		ID:   "1233432",
		Name: "Mike",
		Attributes: Attributes{
			"ID":   "889808980",
			"NaMe": "John",
		},
	}
	idWithAttributes = EscapeEntity(idWithAttributes)
	assert.Equal(t, "User", idWithAttributes.Type)
	assert.Equal(t, "889808980", idWithAttributes.Attributes["id"])
	assert.Equal(t, "John", idWithAttributes.Attributes["name"])

}

func TestFilterOperator(t *testing.T) {
	assert.Equal(t, false, Operator("invalidOperator").isValid())
	assert.Equal(t, true, Operator("IS").isValid())
	assert.Equal(t, true, Operator("IN").isValid())

	flagSubPopulations := FlagSubpopulation{
		EntityType:         "User",
		SamplingPercentage: 0.3,
		Filters: []*FlagFilter{{
			AttributeName: "name",
			Operator:      "invalid",
			Value:         nil,
		},
			{
				AttributeName: "name",
				Operator:      "IS",
				Value:         nil,
			}},
	}
	flagSubPopulations.escape()

	assert.Equal(t, 1, len(flagSubPopulations.Filters))
	assert.Equal(t, Operator("IS"), flagSubPopulations.Filters[0].Operator)
}

func TestFlaggerConfigurationEscape(t *testing.T) {
	now1 := time.Now().Add(time.Duration(rand.Intn(500)+1) * time.Second).Truncate(time.Second)
	now2 := now1.Add(time.Duration(rand.Intn(500)+1) * time.Second).Truncate(time.Second)
	now3 := now2.Add(time.Duration(rand.Intn(500)+1) * time.Second).Truncate(time.Second)

	configuration := Configuration{
		HashKey: "123",
		Flags: []*FlagConfig{{
			Codename: "test",
			FlagSubPopulations: []*FlagSubpopulation{{
				EntityType:         "User",
				SamplingPercentage: 0.3,
				Filters: []*FlagFilter{
					{
						AttributeName: "TesTATTRiBUTe",
						Operator:      "IS",
						Value:         nil,
					},
					{
						AttributeName: "data",
						Operator:      "invalid",
						Value:         nil,
					},
					{
						AttributeName: "createdAt",
						Operator:      "LTE",
						Value:         now1.Format(time.RFC3339),
						FilterType:    filterTypeDate,
					},
					{
						AttributeName: "createdAt",
						Operator:      "IN",
						Value: []string{
							now2.Format(time.RFC3339),
							now1.Format(time.RFC3339),
							now3.Format(time.RFC3339),
						},
						FilterType: filterTypeDate,
					},
				},
			}},
		}},
		SdkConfig: SDKConfig{},
	}

	configuration.Escape()
	configuration.Escape()
	configuration.Escape()

	assert.Equal(t, "testattribute", configuration.Flags[0].FlagSubPopulations[0].Filters[0].AttributeName)
	assert.Equal(t, 3, len(configuration.Flags[0].FlagSubPopulations[0].Filters))

	filters := configuration.Flags[0].FlagSubPopulations[0].Filters
	for i := 0; i < len(filters); i++ {
		if filters[i].AttributeName == "createdAt" && filters[i].FilterType == "LTE" {
			assert.Equal(t, now1, filters[i].Value)
		}
	}
}

func TestEscapeAttributes(t *testing.T) {
	t.Run("test key is in lowercase", func(t *testing.T) {
		assert.Equal(t, escapeAttributes(Attributes{"KEY": "CorrectStringValue"}), Attributes{"key": "CorrectStringValue"})
	})

	t.Run("test type of the value is string", func(t *testing.T) {
		assert.Equal(t, escapeAttributes(Attributes{"KEY": "CorrectStringValue"}), Attributes{"key": "CorrectStringValue"})
	})

	t.Run("test type of the value is bool", func(t *testing.T) {
		assert.Equal(t, escapeAttributes(Attributes{"KEY": true}), Attributes{"key": true})
	})

	t.Run("int is converted to float64 because of json unmarshalling", func(t *testing.T) {
		assert.Equal(t, escapeAttributes(Attributes{"KEY": 123456789}), Attributes{"key": 123456789.})
	})

	t.Run("test type of the value is float", func(t *testing.T) {
		assert.Equal(t, escapeAttributes(Attributes{"KEY": 23.0}), Attributes{"key": 23.0})
	})

	t.Run("test type of the value is incorrect", func(t *testing.T) {
		assert.Equal(t, escapeAttributes(Attributes{"KEY": map[string]string{"key": "value"}}), Attributes{})
	})
}
