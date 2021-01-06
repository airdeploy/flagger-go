package flagger_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/airdeploy/flagger-go/v3"
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/ingester"
	"github.com/airdeploy/flagger-go/v3/internal"
	"github.com/airdeploy/flagger-go/v3/internal/utils"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/stretchr/testify/assert"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/h2non/gock.v1"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const (
	ingestionSchemaFile = "ingestion.schema.json"
	defaultConfig       = "./testdata/configuration.json"
	ingestionConfig     = "./testdata/configuration_ingestion.json"
	sseConfig           = "./testdata/configuration_sse.json"
)

var errAPIKeyNotFound = errors.New("API keys not found")

func Test_validateIngestionSchema(t *testing.T) {
	catchIngestion(2)
	defer gock.OffAll()
	defer gock.Observe(nil)

	count := 0

	ingestionSchemaBuf, err := ioutil.ReadFile(ingestionSchemaFile)
	if err != nil {
		panic(fmt.Sprintf("bad file: %+v", err))
	}
	schemaLoader := gojsonschema.NewBytesLoader(ingestionSchemaBuf)
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		if request.Method == http.MethodPost {
			count++
			// read data from request
			buf, err := ioutil.ReadAll(request.Body)
			assert.NoError(t, err)

			// convert to IngestionDataRequest
			var data *ingester.IngestionDataRequest
			err = json.Unmarshal(buf, &data)
			assert.Nil(t, err)

			// skip init ingestion
			if isEmpty(data) {
				return
			}

			// additionally validates against schema
			documentLoader := gojsonschema.NewBytesLoader(buf)
			result, err := gojsonschema.Validate(schemaLoader, documentLoader)
			assert.NoError(t, err)
			if result.Valid() {
				fmt.Printf("The document is valid\n")
			} else {
				fmt.Printf("The document is not valid. see errors :\n")
				for _, desc := range result.Errors() {
					assert.Fail(t, "- %s\n", desc)
				}
			}
		}
	})

	f, err := initFlaggerInstance(ingestionConfig)
	assert.NoError(t, err)

	f.IsEnabled("test", &core.Entity{
		ID:   "1234",
		Type: "User",
		Name: "John",
		Group: &core.Group{
			ID:   "5678",
			Type: "Company",
			Name: "Stark Int",
			Attributes: map[string]interface{}{
				"active": true,
			},
		},
		Attributes: map[string]interface{}{
			"lastName": "Travolta",
		},
	})

	timeout := f.Shutdown(1 * time.Second)
	assert.False(t, timeout)

	assert.Equal(t, 2, count)
}

func TestFlagger_Init(t *testing.T) {

	var configuration *core.Configuration
	utils.MustJSONFile(ingestionConfig, &configuration)

	t.Run("empty APIKey", func(t *testing.T) {
		f := flagger.NewFlagger()
		assert.NotNil(t, f)

		err := f.Init(&flagger.InitArgs{APIKey: "", SSEURL: utils.SseURL})

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		assert.Equal(t, flagger.ErrBadInitArgs, err)
	})

	t.Run("positive", func(t *testing.T) {
		defer gock.OffAll()

		catchIngestion(2)

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		ok := f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("second call with same arguments triggers ingestion", func(t *testing.T) {
		defer gock.OffAll()
		// 2 init ingestion + 2 * one exposure per ingestion
		catchIngestion(4)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {
				count++
			}
		})

		f := flagger.NewFlagger()
		assert.NotNil(t, f)

		var configuration *core.Configuration
		utils.MustJSONFile(ingestionConfig, &configuration)

		gock.New(utils.FlagsURL).
			Get(utils.FlagsPath + utils.APIKey).
			Times(2).
			Reply(http.StatusOK).
			JSON(configuration)

		err := f.Init(&flagger.InitArgs{APIKey: utils.APIKey, SSEURL: utils.SseURL})
		assert.NoError(t, err)

		ok := f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		err = f.Init(&flagger.InitArgs{APIKey: utils.APIKey, SSEURL: utils.SseURL})
		assert.NoError(t, err)

		ok = f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		assert.Equal(t, 4, count)
		gock.Observe(nil)
	})

	t.Run("wrong apiKey results in flag functions false", func(t *testing.T) {

		// flagger tries to get config from source 3 times
		gock.New(utils.FlagsURL).
			Get(utils.FlagsPath + utils.APIKey).
			Times(3).
			ReplyError(errAPIKeyNotFound)

		// flagger tries to get config from backup-source 3 times
		gock.New(utils.BackupFlagsURL).
			Get(utils.FlagsPath + utils.APIKey).
			Times(3).
			ReplyError(errAPIKeyNotFound)

		defer gock.OffAll()

		f := flagger.NewFlagger()
		assert.NotNil(t, f)

		err := f.Init(&flagger.InitArgs{APIKey: utils.APIKey, SSEURL: utils.SseURL})
		assert.NotNil(t, err)
		assert.Error(t, err, errAPIKeyNotFound)

		assert.False(t, f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))
		assert.False(t, f.IsSampled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))
		assert.Equal(t, core.Payload{}, f.GetPayload("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))
		assert.Equal(t, "off", f.GetVariation("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("init from backup source", func(t *testing.T) {
		catchIngestion(2)
		// flagger fails to get config from source 3 times
		gock.New(utils.FlagsURL).
			Get(utils.FlagsPath + utils.APIKey).
			Times(3).
			ReplyError(errAPIKeyNotFound)

		var configuration *core.Configuration
		utils.MustJSONFile(ingestionConfig, &configuration)

		gock.New(utils.BackupFlagsURL).
			Get(utils.FlagsPath + utils.APIKey).
			Reply(http.StatusOK).
			JSON(configuration)

		defer gock.OffAll()

		f := flagger.NewFlagger()
		assert.NotNil(t, f)

		err := f.Init(&flagger.InitArgs{APIKey: utils.APIKey, SSEURL: utils.SseURL})
		assert.Nil(t, err)

		assert.True(t, f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}

func TestSetEntity(t *testing.T) {
	t.Run("set then reset", func(t *testing.T) {
		catchIngestion(4)

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		f.SetEntity(&core.Entity{ID: utils.WhitelistedUserID})
		assert.True(t, f.IsEnabled("new-signup-flow", nil))
		assert.Equal(t, "enabled", f.GetVariation("new-signup-flow", nil))

		f.SetEntity(nil)
		disabled := f.IsEnabled("test", nil)
		off := f.GetVariation("test", nil)
		assert.False(t, disabled)
		assert.Equal(t, "off", off)

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("cannot set entity with empty id", func(t *testing.T) {
		catchIngestion(2)

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		f.SetEntity(&core.Entity{ID: utils.WhitelistedUserID})
		f.SetEntity(&core.Entity{ID: ""})
		enabled := f.IsEnabled("new-signup-flow", nil)
		nonEmptyVariation := f.GetVariation("new-signup-flow", nil)
		assert.True(t, enabled)
		assert.Equal(t, "enabled", nonEmptyVariation)

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}

func TestFlagger_Track(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		catchIngestion(2)
		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			if request.Method == http.MethodPost {
				count++
				data, err := utils.ParseIngestionBody(request.Body)
				assert.NoError(t, err)
				// skip init ingestion
				if isEmpty(data) {
					return
				}
				if assert.Len(t, data.Entities, 1) {
					assert.Equal(t, "1", data.Entities[0].ID)
					assert.Equal(t, "User", data.Entities[0].Type)
				}
				if assert.Len(t, data.Events, 1) {
					assert.Equal(t, "test", data.Events[0].Name)
				}
			}
		})

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		f.Track(&core.Event{
			Name: "test",
			EventProperties: core.Attributes{
				"plan":       "Bronze",
				"referrer":   "www.Google.com",
				"shirt_size": "medium",
			},
			Entity: &core.Entity{ID: "1"},
		})

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		gock.Observe(nil)

		assert.Equal(t, 2, count)
	})

	t.Run("with default Entity", func(t *testing.T) {

		catchIngestion(2)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			if request.Method == http.MethodPost {
				count++
				data, err := utils.ParseIngestionBody(request.Body)
				assert.NoError(t, err)
				// skip init ingestion
				if isEmpty(data) {
					return
				}
				if assert.Len(t, data.Entities, 1) {
					assert.Equal(t, "1", data.Entities[0].ID)
					assert.Equal(t, "User", data.Entities[0].Type)
					assert.Equal(t, "test", data.Events[0].Name)
				}
				assert.Len(t, data.Events, 1)
			}
		})

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		f.SetEntity(&core.Entity{ID: "1"})
		f.Track(&core.Event{
			Name: "test",
			EventProperties: core.Attributes{
				"plan":       "Bronze",
				"referrer":   "www.Google.com",
				"shirt_size": "medium",
			},
		})

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		gock.Observe(nil)

		assert.Equal(t, 2, count)
	})

	t.Run("doesn't add invalid events to the ingester", func(t *testing.T) {
		catchIngestion(2)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			if request.Method == http.MethodPost {
				count++
				data, err := utils.ParseIngestionBody(request.Body)
				assert.NoError(t, err)
				// skip init ingestion
				if isEmpty(data) {
					return
				}
				if assert.Len(t, data.Entities, 1) {
					assert.Equal(t, "1", data.Entities[0].ID)
					assert.Equal(t, "User", data.Entities[0].Type)
				}
				if assert.Len(t, data.Events, 1) {
					assert.Equal(t, "test", data.Events[0].Name)
				}
			}
		})

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		// valid
		f.Track(&core.Event{
			Name: "test",
			EventProperties: core.Attributes{
				"plan":       "Bronze",
				"referrer":   "www.Google.com",
				"shirt_size": "medium",
			},
			Entity: &core.Entity{ID: "1"},
		})

		// invalid
		f.Track(nil)
		f.Track(&core.Event{Name: "", EventProperties: nil, Entity: nil})
		f.Track(&core.Event{Name: "test", EventProperties: nil, Entity: &core.Entity{ID: ""}})

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		gock.Observe(nil)

		assert.Equal(t, 2, count)
	})
}

func TestFlagger_Publish(t *testing.T) {

	t.Run("publish adds entity to ingester", func(t *testing.T) {
		catchIngestion(2)
		defer gock.OffAll()

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			if request.Method == http.MethodPost {
				count++
				data, err := utils.ParseIngestionBody(request.Body)
				assert.NoError(t, err)
				// skip init ingestion
				if isEmpty(data) {
					return
				}

				if assert.Len(t, data.Entities, 1) {
					assert.Equal(t, "54", data.Entities[0].ID)
					assert.Equal(t, "User", data.Entities[0].Type)
				}

			}
		})

		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		f.Publish(&core.Entity{ID: "54"})

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		gock.Observe(nil)
		assert.Equal(t, 2, count)
	})

	t.Run("invalid entities are not added to the ingester", func(t *testing.T) {
		f, err := initFlaggerInstance(ingestionConfig)
		assert.Nil(t, err)

		catchIngestion(2)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			if request.Method == http.MethodPost {
				count++
				data, err := utils.ParseIngestionBody(request.Body)
				assert.NoError(t, err)
				// skip init ingestion
				if isEmpty(data) {
					return
				}

				if assert.Len(t, data.Entities, 1) {
					assert.Equal(t, "1", data.Entities[0].ID)
					assert.Equal(t, "User", data.Entities[0].Type)
				}

			}
		})

		f.Publish(&core.Entity{ID: ""})
		f.Publish(nil)
		f.Publish(&core.Entity{ID: "1"})
		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		gock.Observe(nil)
		assert.Equal(t, 2, count)
	})
}

func TestFlagFunctions(t *testing.T) {
	t.Run("IsEnabled", func(t *testing.T) {
		catchIngestion(3)

		f, err := initFlaggerInstance(ingestionConfig)
		assert.NoError(t, err)

		codename := "new-signup-flow"
		assert.True(t, f.IsEnabled(codename, &core.Entity{
			ID: "1",
			Attributes: map[string]interface{}{
				"country":  "France",
				"bday":     "2016-03-16T05:44:23.000Z",
				"age":      42,
				"booleans": false,
			},
		}))

		assert.False(t, f.IsEnabled(codename, &core.Entity{
			ID: "2",
			Attributes: map[string]interface{}{
				"country": "USA",
			},
		}))

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)

	})

	t.Run("IsSampled", func(t *testing.T) {
		defer gock.OffAll()

		catchIngestion(3)

		f, err := initFlaggerInstance(ingestionConfig)
		assert.NoError(t, err)

		entity := &core.Entity{
			ID:         "kfjvv3",
			Attributes: core.Attributes{"admin": true},
		}

		sampled := f.IsSampled("premium-support", entity)
		assert.True(t, sampled)

		//group
		assert.True(t, f.IsSampled("org-chart", &core.Entity{
			ID:   "14",
			Type: "User",
			Group: &core.Group{
				ID:   "15",
				Type: "Company",
			},
		}))

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("GetPayload", func(t *testing.T) {
		defer gock.OffAll()

		catchIngestion(2)

		f, err := initFlaggerInstance(ingestionConfig)
		assert.NoError(t, err)

		payload := f.GetPayload("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.Equal(t, "on", payload["newFeature"])

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("GetVariation", func(t *testing.T) {

		catchIngestion(2)

		defer gock.OffAll()

		f, err := initFlaggerInstance(ingestionConfig)
		assert.NoError(t, err)

		variation := f.GetVariation("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.Equal(t, "enabled", variation)

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}

func TestFilters(t *testing.T) {
	defer gock.OffAll()
	gock.New(utils.IngestionURL).
		Post(utils.IngestionPath + utils.APIKey).
		Times(6).
		Reply(http.StatusOK)

	f, err := initFlaggerInstance(defaultConfig)
	assert.NoError(t, err)

	t.Run("positive test", func(t *testing.T) {

		t.Run("LTE, equal test", func(t *testing.T) {
			isEnabled := f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2014-09-20T00:00:00Z",
				}})
			assert.True(t, isEnabled)
		})

		t.Run("LTE, less test", func(t *testing.T) {
			isEnabled := f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2014-08-20T00:00:00Z",
				}})
			assert.True(t, isEnabled)
		})

		t.Run("GTE(equal) and IS test", func(t *testing.T) {
			isEnabled := f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2016-03-16T05:44:23Z",
					"country":   "USA",
				}})
			assert.True(t, isEnabled)
		})

	})

	t.Run("negative test", func(t *testing.T) {
		t.Run("date is out of range", func(t *testing.T) {
			isEnabled := f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2015-09-20T00:00:00Z",
				}})
			assert.False(t, isEnabled)
		})

		t.Run("date is right, country is absent", func(t *testing.T) {
			isEnabled := f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2016-03-16T05:44:23Z",
				}})
			assert.False(t, isEnabled)
		})

		t.Run("date is right, but wrong country", func(t *testing.T) {
			isEnabled := f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2016-03-16T05:44:23Z",
					"country":   "UK",
				}})
			assert.False(t, isEnabled)
		})
	})

	timeout := f.Shutdown(1 * time.Second)
	assert.False(t, timeout)
}

// dynamic-pricing flag killSwitch is on at source(configuration.json) but off at sse
func TestFlagger_SSE(t *testing.T) {
	defer gock.OffAll()
	catchIngestion(2)
	flaggerConfigMessage := getConfigMessage()
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)

	broker := internal.NewSSEServer(ctx, flaggerConfigMessage)

	ssePort := "3101"
	go func() {
		log.Fatal("HTTP server error: ", http.ListenAndServe("localhost:"+ssePort, broker))
	}()

	time.Sleep(10 * time.Millisecond)
	var configuration *core.Configuration

	utils.MustJSONFile(defaultConfig, &configuration)
	gock.New(utils.FlagsURL).
		Get(utils.FlagsPath + utils.APIKey).
		Reply(http.StatusOK).
		JSON(configuration)

	f := flagger.NewFlagger()
	err := f.Init(&flagger.InitArgs{APIKey: utils.APIKey, SSEURL: "http://localhost:" + ssePort + "/"})
	assert.NoError(t, err)

	time.Sleep(1000 * time.Millisecond)

	entity := &core.Entity{
		ID:         "kfjvv3",
		Attributes: core.Attributes{"admin": true},
	}

	// dynamic-pricing isKill
	assert.True(t, f.IsSampled("dynamic-pricing", entity))
	done()

	timeout := f.Shutdown(5 * time.Second)
	assert.False(t, timeout)
}

func TestCustomURLS(t *testing.T) {
	var configuration *core.Configuration
	utils.MustJSONFile(defaultConfig, &configuration)
	customURL := "https://mycustomurl-somewebsite.com"
	gock.New(customURL).
		Get("/" + utils.APIKey).
		Reply(http.StatusOK).
		JSON(configuration)

	gock.New(customURL).
		Post("/ingest/" + utils.APIKey).
		Reply(http.StatusOK)
	defer gock.OffAll()

	f := flagger.NewFlagger()
	err := f.Init(&flagger.InitArgs{
		APIKey:       utils.APIKey,
		SourceURL:    customURL + "/" + utils.APIKey,
		IngestionURL: customURL + "/ingest/" + utils.APIKey,
		SSEURL:       utils.SseURL,
	})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	entity := &core.Entity{
		ID:         "kfjvv3",
		Attributes: core.Attributes{"admin": true},
	}

	// dynamic-pricing isKill
	sampled := f.IsSampled("premium-support", entity)
	assert.True(t, sampled)

	shutdown := f.Shutdown(2 * time.Second)
	assert.False(t, shutdown)
}

func TestShutdown(t *testing.T) {
	t.Run("Shutdown makes all flag functions return default variation", func(t *testing.T) {
		defer gock.OffAll()
		catchIngestion(1)
		f, err := initFlaggerInstance(defaultConfig)
		assert.NoError(t, err)

		ok := f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		f.Shutdown(1 * time.Second)

		notOk := f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.False(t, notOk)

	})

	t.Run("Init, Shutdown, Init recover initial state", func(t *testing.T) {
		catchIngestion(3)

		defer gock.OffAll()
		f, err := initFlaggerInstance(defaultConfig)
		assert.NoError(t, err)

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		f, err = initFlaggerInstance(defaultConfig)
		assert.NoError(t, err)

		ok := f.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		timeout = f.Shutdown(1 * time.Second)
		assert.False(t, timeout)

	})

	t.Run("Shutdown called without init", func(t *testing.T) {
		f := flagger.NewFlagger()

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}

func TestIngestion(t *testing.T) {

	t.Run("First 10 exposures are always ingested", func(t *testing.T) {
		defer gock.OffAll()
		f, err := initFlaggerInstance(defaultConfig)
		assert.NoError(t, err)

		gock.New(utils.IngestionURL).
			Post(utils.IngestionPath + utils.APIKey).
			// init (1) + first 10 + last ingestion with 2 exposures
			Times(12).
			Reply(http.StatusOK)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			if request.Method == http.MethodPost {
				count++
			}
		})

		for i := 0; i < 12; i++ {
			f.IsEnabled("new-signup-flow", &core.Entity{
				ID: "1",
			})
		}

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, 11, count)

		timeout := f.Shutdown(1 * time.Second)

		assert.False(t, timeout)
		assert.Equal(t, 12, count)
		gock.Observe(nil)

	})

	t.Run("Detected flags always ingested", func(t *testing.T) {
		defer gock.OffAll()
		f, err := initFlaggerInstance(defaultConfig)
		assert.NoError(t, err)

		firstExposuresIngestThreshold := 10
		detectedFlagCount := 7

		totalIngestedFlags := firstExposuresIngestThreshold + detectedFlagCount

		gock.New(utils.IngestionURL).
			Post(utils.IngestionPath + utils.APIKey).
			Times(totalIngestedFlags).
			Reply(http.StatusOK)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {
				count++
				if count == totalIngestedFlags {
					gock.Observe(nil)
				}
			}
		})

		// ingesting first 10 exposures
		for i := 0; i < firstExposuresIngestThreshold; i++ {
			f.IsEnabled("new-signup-flow", &core.Entity{
				ID: "1",
			})
		}

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, firstExposuresIngestThreshold, count)

		for i := 0; i < detectedFlagCount; i++ {
			f.IsEnabled("new-flag-"+strconv.Itoa(i), &core.Entity{
				ID: "1",
			})
		}

		timeout := f.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		assert.Equal(t, totalIngestedFlags, count)

	})
}

func TestFlaggerMustNotMutatePassedEntity(t *testing.T) {
	catchIngestion(5)
	entityID := "123"
	entityType := "Entity Type"
	entityName := "Entity Name"
	groupID := "321"
	groupType := "Group Type"
	groupName := "Group Name"
	groupAttributes := core.Attributes{
		"UPPERCASE":  "VALUE",
		"uint_value": uint8(1),
		"time":       time.Now(),
	}
	entityAttributes := core.Attributes{
		"correct_attribute": float64(42),
		"other":             time.Now(),
	}
	entity := &core.Entity{
		ID:   entityID,
		Type: entityType,
		Name: entityName,
		Group: &core.Group{
			ID:         groupID,
			Type:       groupType,
			Name:       groupName,
			Attributes: groupAttributes,
		},
		Attributes: entityAttributes,
	}

	f, err := initFlaggerInstance(defaultConfig)
	assert.Nil(t, err)

	f.IsEnabled("test", entity)
	f.IsSampled("test", entity)
	f.GetVariation("test", entity)
	f.GetPayload("test", entity)
	f.Publish(entity)
	f.Track(&core.Event{
		Name:            "test",
		EventProperties: nil,
		Entity:          entity,
	})
	f.SetEntity(entity)

	assert.Equal(t, entityID, entity.ID)
	assert.Equal(t, entityType, entity.Type)
	assert.Equal(t, entityName, entity.Name)
	assert.Equal(t, groupID, entity.Group.ID)
	assert.Equal(t, groupName, entity.Group.Name)
	assert.Equal(t, groupAttributes, entity.Group.Attributes)
	assert.Equal(t, entityAttributes, entity.Attributes)

	timeout := f.Shutdown(1 * time.Second)
	assert.False(t, timeout)

}

func BenchmarkFlagger_flagFunctions(b *testing.B) {
	f, err := initFlaggerInstance(defaultConfig)
	assert.Nil(b, err)

	for i := 0; i < b.N; i++ {
		f.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
			Attributes: map[string]interface{}{
				"createdAt": "2014-09-20T00:00:00Z",
			}})
		f.IsSampled("color-theme", &core.Entity{ID: "31404847", Type: "User",
			Attributes: map[string]interface{}{
				"createdAt": "2014-09-20T00:00:00Z",
			}})

		f.GetPayload("color-theme", &core.Entity{ID: "31404847", Type: "User",
			Attributes: map[string]interface{}{
				"createdAt": "2014-09-20T00:00:00Z",
			}})

		f.GetVariation("color-theme", &core.Entity{ID: "31404847", Type: "User",
			Attributes: map[string]interface{}{
				"createdAt": "2014-09-20T00:00:00Z",
			}})
	}
}

func initFlaggerInstance(configFileName string) (*flagger.Flagger, error) {

	var configuration *core.Configuration
	utils.MustJSONFile(configFileName, &configuration)

	gock.New(utils.FlagsURL).
		Get(utils.FlagsPath + utils.APIKey).
		Reply(http.StatusOK).
		JSON(configuration)

	f := flagger.NewFlagger()
	err := f.Init(&flagger.InitArgs{APIKey: utils.APIKey, SSEURL: utils.SseURL})
	return f, err
}

func getConfigMessage() []byte {
	flaggerConfigMessage := []byte("id: 76c58618-b75c-4872-a107-986998601fe4\n" +
		"event: flagConfigUpdate\n" +
		"data: ")
	config, _ := ioutil.ReadFile(sseConfig)

	config = bytes.Replace(config, []byte(" "), []byte(""), -1)
	config = bytes.Replace(config, []byte("\n"), []byte(""), -1)
	flaggerConfigMessage = append(flaggerConfigMessage, config...)
	flaggerConfigMessage = append(flaggerConfigMessage, []byte("\n\n")...)
	return flaggerConfigMessage
}

func catchIngestion(times int) {
	gock.New(utils.IngestionURL).
		Post(utils.IngestionPath + utils.APIKey).
		Times(times).
		Reply(http.StatusOK)
}

func isEmpty(dr *ingester.IngestionDataRequest) bool {
	return len(dr.Entities) == 0 &&
		len(dr.Exposures) == 0 &&
		len(dr.DetectedFlags) == 0 &&
		len(dr.Events) == 0
}
