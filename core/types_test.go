package core

import (
	"github.com/airdeploy/flagger-go/v3/json"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntityEscape(t *testing.T) {
	assert.Nil(t, EscapeEntity(nil))
	t.Run("EscapeEntity is idempotent and doesn't mutate given entity", func(t *testing.T) {
		entity := &Entity{
			ID: "123",
		}
		escapedEntity := EscapeEntity(entity)
		escapedEntity = EscapeEntity(escapedEntity)

		assert.Empty(t, entity.Type)
		assert.Nil(t, entity.Attributes)

		assert.Nil(t, escapedEntity.Attributes["name"])
		assert.Equal(t, "User", escapedEntity.Type)
		assert.Equal(t, "123", escapedEntity.Attributes["id"])
	})

	t.Run("name is propagated to attributes", func(t *testing.T) {
		entity := &Entity{
			ID:   "321",
			Name: "test",
		}
		escapedEntity := EscapeEntity(entity)

		assert.Equal(t, escapedEntity.Type, "User")
		assert.Equal(t, escapedEntity.Attributes["id"], "321")
		assert.Equal(t, escapedEntity.Attributes["name"], "test")
	})

	t.Run("type User doesn't propagated if specified", func(t *testing.T) {
		entity := &Entity{
			ID:   "1233432",
			Type: "Company",
		}
		escapedEntity := EscapeEntity(entity)
		assert.Equal(t, "Company", escapedEntity.Type)
		assert.Equal(t, "1233432", escapedEntity.Attributes["id"])
		assert.Nil(t, escapedEntity.Attributes["name"])
	})

	t.Run("EscapeEntity doesn't override id and name in attributes", func(t *testing.T) {
		entity := &Entity{
			ID:   "1233432",
			Name: "Mike",
			Attributes: Attributes{
				"ID":   "889808980",
				"NaMe": "John",
			},
		}
		escapedEntity := EscapeEntity(entity)
		assert.Equal(t, "User", escapedEntity.Type)
		assert.Equal(t, "889808980", escapedEntity.Attributes["id"])
		assert.Equal(t, "John", escapedEntity.Attributes["name"])
	})

	t.Run("group is escaped", func(t *testing.T) {
		entity := Entity{
			ID: "1",
			Group: &Group{
				ID:   "1",
				Type: "Company",
				Name: "Company Name",
				Attributes: Attributes{
					"AGE": 42,
				},
			},
		}
		escapedEntity := EscapeEntity(&entity)

		assert.Equal(t, 42., escapedEntity.Group.Attributes["age"])
		assert.Equal(t, "1", escapedEntity.Group.Attributes["id"])
		assert.Equal(t, "Company Name", escapedEntity.Group.Attributes["name"])
	})

	t.Run("nil check", func(t *testing.T) {
		entity := Entity{
			ID:         "1",
			Attributes: nil,
		}
		escapedEntity := EscapeEntity(&entity)
		assert.Equal(t, Attributes{"id": "1"}, escapedEntity.Attributes)
	})
}

func TestEscapeGroup(t *testing.T) {
	companyName := "Company Name"
	companyID := "1"
	group := Group{
		ID:   companyID,
		Type: "Company",
		Name: companyName,
		Attributes: Attributes{
			"AGE": 42,
		},
	}
	escapedGroup := escapeGroup(&group)
	t.Run("Escape is idempotent", func(t *testing.T) {
		escapedGroup = escapeGroup(&group)

		assert.Equal(t, 42, group.Attributes["AGE"])
		assert.Equal(t, 42., escapedGroup.Attributes["age"])
		group.Attributes["AGE"] = 24
		assert.Equal(t, 42., escapedGroup.Attributes["age"])
	})

	t.Run("name and id propagated to attributes", func(t *testing.T) {
		assert.Equal(t, companyID, escapedGroup.Attributes["id"])
		assert.Equal(t, companyName, escapedGroup.Attributes["name"])

		assert.Nil(t, group.Attributes["id"])
		assert.Nil(t, group.Attributes["name"])
	})
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

func TestFlagFilterEscape(t *testing.T) {
	t.Run("escape function doesn't do anything for invalid value", func(t *testing.T) {
		value := "corrupted value"
		filter := FlagFilter{
			AttributeName: "date",
			Operator:      is,
			Value:         value,
			FilterType:    filterTypeDate,
		}
		filter.escape()

		assert.Equal(t, value, filter.Value)
	})

	t.Run("escape function doesn't do anything for invalid array of values", func(t *testing.T) {
		value := []string{"corrupted value", "2016-03-16T05:44:23Z"}
		filter := FlagFilter{
			AttributeName: "date",
			Operator:      is,
			Value:         value,
			FilterType:    filterTypeDate,
		}
		filter.escape()

		assert.Equal(t, []time.Time{
			time.Date(2016, 3, 16, 5, 44, 23, 0, time.UTC),
		}, filter.Value)
	})

	t.Run("escape converts string to time", func(t *testing.T) {
		createdAt := "2016-03-16T05:44:23Z"
		filter := FlagFilter{
			AttributeName: "date",
			Operator:      is,
			Value:         createdAt,
			FilterType:    filterTypeDate,
		}
		filter.escape()

		assert.Equal(t, time.Date(2016, 03, 16, 5, 44, 23, 0, time.UTC), filter.Value)
	})

	t.Run("escape converts []string to []time", func(t *testing.T) {
		createdAt := []string{"2016-03-16T05:44:23Z"}
		filter := FlagFilter{
			AttributeName: "date",
			Operator:      is,
			Value:         createdAt,
			FilterType:    filterTypeDate,
		}
		filter.escape()

		assert.Equal(t, []time.Time{
			time.Date(2016, 3, 16, 5, 44, 23, 0, time.UTC),
		}, filter.Value)
	})

	t.Run("escape array", func(t *testing.T) {
		t.Run("parsed to []string, ignoring invalid values", func(t *testing.T) {
			filterStr := "{" +
				"\"attributeName\":\"values\"," +
				"\"operator\":\"IN\"," +
				"\"value\":[21, 42, \"FR\", \"AT\"]," +
				"\"type\":\"STRING\"" +
				"}"

			var filter FlagFilter
			err := json.Unmarshal([]byte(filterStr), &filter)
			if err != nil {
				assert.Fail(t, err.Error())
			}
			filter.escape()
			assert.Equal(t, []string{"FR", "AT"}, filter.Value)
		})

		t.Run("parsed to []float64, ignoring invalid values", func(t *testing.T) {
			filterStr := "{" +
				"\"attributeName\":\"values\"," +
				"\"operator\":\"IN\"," +
				"\"value\":[\"not a number\", 21, 42, 52.1]," +
				"\"type\":\"NUMBER\"" +
				"}"

			var filter FlagFilter
			err := json.Unmarshal([]byte(filterStr), &filter)
			if err != nil {
				assert.Fail(t, err.Error())
			}
			filter.escape()
			assert.Equal(t, []float64{21, 42, 52.1}, filter.Value)
		})

		t.Run("parsed to []bool, ignoring invalid values", func(t *testing.T) {
			filterStr := "{" +
				"\"attributeName\":\"values\"," +
				"\"operator\":\"IN\"," +
				"\"value\":[true, 42, false, \"AT\"]," +
				"\"type\":\"BOOLEAN\"" +
				"}"

			var filter FlagFilter
			err := json.Unmarshal([]byte(filterStr), &filter)
			if err != nil {
				assert.Fail(t, err.Error())
			}
			filter.escape()
			assert.Equal(t, []bool{true, false}, filter.Value)
		})

		t.Run("parsed to []date, ignoring invalid values", func(t *testing.T) {
			filterStr := "{" +
				"\"attributeName\":\"values\"," +
				"\"operator\":\"IN\"," +
				"\"value\":[true, 42, false, \"AT\", \"2016-03-16T05:44:23Z\"]," +
				"\"type\":\"DATE\"" +
				"}"

			var filter FlagFilter
			err := json.Unmarshal([]byte(filterStr), &filter)
			if err != nil {
				assert.Fail(t, err.Error())
			}
			filter.escape()
			assert.Equal(t, []time.Time{time.Date(2016, 3, 16, 5, 44, 23, 0, time.UTC)}, filter.Value)
		})
	})
}

func TestSDKConfig(t *testing.T) {
	t.Run("IngestionIntervalDuration convert interval to time.Duration", func(t *testing.T) {
		config := &SDKConfig{
			SDKIngestionInterval: 60,
		}
		assert.Equal(t, 60*time.Second, config.IngestionIntervalDuration())
		config = &SDKConfig{
			SDKIngestionInterval: 0,
		}
		assert.Equal(t, 1*time.Second, config.IngestionIntervalDuration())
	})

	t.Run("Copy creates deep copy", func(t *testing.T) {
		config := SDKConfig{
			SDKIngestionInterval: 60,
			SDKIngestionMaxItems: 500,
		}
		configCopy := config.Copy()

		assert.Equal(t, config.SDKIngestionInterval, configCopy.SDKIngestionInterval)
		assert.Equal(t, config.SDKIngestionMaxItems, configCopy.SDKIngestionMaxItems)

		config.SDKIngestionMaxItems = 100

		assert.Equal(t, 500, configCopy.SDKIngestionMaxItems)
	})
}

func TestSDKInfo_Copy(t *testing.T) {
	t.Run("Copy creates deep copy", func(t *testing.T) {
		sdkInfo := SDKInfo{
			Name:    "golang",
			Version: "3.0.0",
		}

		sdkInfoCopy := sdkInfo.Copy()

		assert.Equal(t, sdkInfo.Name, sdkInfoCopy.Name)
		assert.Equal(t, sdkInfo.Version, sdkInfoCopy.Version)

		sdkInfo.Version = "3.0.1"
		assert.Equal(t, "3.0.0", sdkInfoCopy.Version)
	})
}

func TestEscapeEvent(t *testing.T) {
	t.Run("EscapeEvent escapes both entity and attributes", func(t *testing.T) {
		event := EscapeEvent(&Event{
			Name: "test",
			EventProperties: Attributes{
				"KEY":            "SomeStringValue",
				"wrongValueTYpe": uint(1),
			},
			Entity: &Entity{
				ID: "1",
			},
		})

		assert.Equal(t, "test", event.Name)
		assert.Equal(t, "SomeStringValue", event.EventProperties["key"])
		assert.Nil(t, event.EventProperties["wrongvaluetype"])
		assert.Equal(t, "1", event.Entity.ID)
		assert.Equal(t, "1", event.Entity.Attributes["id"])
	})
}
