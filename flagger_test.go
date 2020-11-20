package flagger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/airdeploy/flagger-go/v3/sse"
	"github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/ingester"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

const (
	flagsURL       = "https://flags.airdeploy.io"
	backupFlagsURL = "https://backup-api.airshiphq.com"
	flagsPath      = "/v3/config/"
	ingestionURL   = "https://ingestion.airdeploy.io"
	ingestionPath  = "/v3/ingest/"
	apiKey         = "8CBohB03MC0zHlDj"
)

func Test_validateIngestionSchema(t *testing.T) {
	catchIngestion(1)
	defer gock.OffAll()

	count := 0

	ingestionSchemaBuf, err := ioutil.ReadFile("ingestion.schema.json")
	if err != nil {
		panic(fmt.Sprintf("bad file: %+v", err))
	}
	schemaLoader := gojsonschema.NewBytesLoader(ingestionSchemaBuf)
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		// catch ingestion
		if request.Method == http.MethodPost {
			// read data from request
			buf, err := ioutil.ReadAll(request.Body)
			assert.NoError(t, err)

			// convert to IngestionDataRequest
			var data *ingester.IngestionDataRequest
			err = json.Unmarshal(buf, &data)
			assert.Nil(t, err)

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

			count++
			gock.Observe(nil)
		}
	})

	flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
	assert.NoError(t, err)

	flagger.IsEnabled("test", &core.Entity{
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

	timeout := flagger.Shutdown(1 * time.Second)
	assert.False(t, timeout)

	assert.Equal(t, 1, count)
}

func TestFlagger_Init(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var configuration *core.Configuration
	mustJSONOFile("configuration_ingestion.json", &configuration)

	t.Run("empty APIKey", func(t *testing.T) {
		flagger := NewFlagger()
		assert.NotNil(t, flagger)

		err := flagger.Init(ctx, &InitArgs{APIKey: "", SSEURL: "http://localhost:8000/v3/sse"})
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("positive", func(t *testing.T) {
		defer gock.OffAll()

		catchIngestion(1)

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.Nil(t, err)

		ok := flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("second call with same arguments triggers ingestion", func(t *testing.T) {
		defer gock.OffAll()
		catchIngestion(2)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {
				count++
				if count == 2 {
					gock.Observe(nil)
				}
			}
		})

		flagger := NewFlagger()
		assert.NotNil(t, flagger)

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.NoError(t, err)

		ok := flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		flagger, err = initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.NoError(t, err)

		ok = flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		assert.Equal(t, 2, count)
	})

	t.Run("wrong apiKey results in flag functions false", func(t *testing.T) {

		apiKeyNotFound := errors.New("API keys not found")
		wrongApiKey := "wrongApiKey"

		// flagger tries to get config from source 3 times
		gock.New(flagsURL).
			Get(flagsPath + wrongApiKey).
			Times(3).
			ReplyError(apiKeyNotFound)

		// flagger tries to get config from backup-source 3 times
		gock.New(backupFlagsURL).
			Get(flagsPath + wrongApiKey).
			Times(3).
			ReplyError(apiKeyNotFound)

		defer gock.OffAll()

		flagger := NewFlagger()
		assert.NotNil(t, flagger)

		err := flagger.Init(ctx, &InitArgs{APIKey: wrongApiKey, SSEURL: "http://localhost:8000/v3/sse"})
		assert.NotNil(t, err)
		assert.Error(t, err, apiKeyNotFound)

		assert.False(t, flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))
		assert.False(t, flagger.IsSampled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))
		assert.Equal(t, core.Payload{}, flagger.GetPayload("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))
		assert.Equal(t, "off", flagger.GetVariation("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"}))

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}

func TestSetEntity(t *testing.T) {
	defer gock.OffAll()

	catchIngestion(4)

	flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
	assert.Nil(t, err)

	flagger.SetEntity(&core.Entity{ID: "90843823"})
	enabled := flagger.IsEnabled("new-signup-flow", nil)
	nonEmptyVariation := flagger.GetVariation("new-signup-flow", nil)
	assert.True(t, enabled)
	assert.Equal(t, "enabled", nonEmptyVariation)

	flagger.SetEntity(nil)
	disabled := flagger.IsEnabled("test", nil)
	off := flagger.GetVariation("test", nil)
	assert.False(t, disabled)
	assert.Equal(t, "off", off)

	timeout := flagger.Shutdown(1 * time.Second)
	assert.False(t, timeout)
}

func TestFlagger_Track(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		catchIngestion(1)
		defer gock.OffAll()

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {
				buf, err := ioutil.ReadAll(request.Body)
				assert.NoError(t, err)

				var data ingester.IngestionDataRequest
				err = json.Unmarshal(buf, &data)
				assert.NoError(t, err)

				if assert.Len(t, data.Entities, 1) {
					assert.Equal(t, "1", data.Entities[0].ID)
					assert.Equal(t, "User", data.Entities[0].Type)
				}
				if assert.Len(t, data.Events, 1) {
					assert.Equal(t, "test", data.Events[0].Name)
				}

				count++
				gock.Observe(nil)
			}
		})

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.Nil(t, err)

		flagger.Track(ctx, &core.Event{
			Name: "test",
			EventProperties: core.Attributes{
				"plan":       "Bronze",
				"referrer":   "www.Google.com",
				"shirt_size": "medium",
			},
			Entity: &core.Entity{ID: "1"},
		})

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		assert.Equal(t, 1, count)
	})

	t.Run("with default Entity", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		catchIngestion(1)

		defer gock.OffAll()

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {
				buf, err := ioutil.ReadAll(request.Body)
				assert.NoError(t, err)

				var r ingester.IngestionDataRequest
				err = json.Unmarshal(buf, &r)
				assert.NoError(t, err)

				if assert.Len(t, r.Entities, 1) {
					assert.Equal(t, "1", r.Entities[0].ID)
					assert.Equal(t, "User", r.Entities[0].Type)
					assert.Equal(t, "test", r.Events[0].Name)
				}
				assert.Len(t, r.Events, 1)

				count++
				gock.Observe(nil)
			}
		})

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.Nil(t, err)

		flagger.SetEntity(&core.Entity{ID: "1"})
		flagger.Track(ctx, &core.Event{
			Name: "test",
			EventProperties: core.Attributes{
				"plan":       "Bronze",
				"referrer":   "www.Google.com",
				"shirt_size": "medium",
			},
		})

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		assert.Equal(t, 1, count)
	})
}

func TestFlagger_Publish(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	catchIngestion(1)
	defer gock.OffAll()

	count := 0
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		// catch ingestion
		if request.Method == http.MethodPost {

			buf, err := ioutil.ReadAll(request.Body)
			assert.NoError(t, err)

			var r ingester.IngestionDataRequest
			err = json.Unmarshal(buf, &r)
			assert.NoError(t, err)

			if assert.Len(t, r.Entities, 1) {
				assert.Equal(t, "54", r.Entities[0].ID)
				assert.Equal(t, "User", r.Entities[0].Type)
			}

			count++
			gock.Observe(nil)
		}
	})

	flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
	assert.Nil(t, err)

	flagger.Publish(ctx, &core.Entity{ID: "54"})

	timeout := flagger.Shutdown(1 * time.Second)
	assert.False(t, timeout)

	assert.Equal(t, 1, count)
}

func TestFlagFunctions(t *testing.T) {
	t.Run("IsEnabled", func(t *testing.T) {
		defer gock.OffAll()
		catchIngestion(2)

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.NoError(t, err)

		codename := "new-signup-flow"
		assert.True(t, flagger.IsEnabled(codename, &core.Entity{
			ID: "1",
			Attributes: map[string]interface{}{
				"country":  "France",
				"bday":     "2016-03-16T05:44:23.000Z",
				"age":      42,
				"booleans": false,
			},
		}))

		assert.False(t, flagger.IsEnabled(codename, &core.Entity{
			ID: "2",
			Attributes: map[string]interface{}{
				"country": "USA",
			},
		}))

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)

	})

	t.Run("IsSampled", func(t *testing.T) {
		defer gock.OffAll()

		catchIngestion(2)

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.NoError(t, err)

		entity := &core.Entity{
			ID:         "kfjvv3",
			Attributes: core.Attributes{"admin": true},
		}

		sampled := flagger.IsSampled("premium-support", entity)
		assert.True(t, sampled)

		//group
		assert.True(t, flagger.IsSampled("org-chart", &core.Entity{
			ID:   "14",
			Type: "User",
			Group: &core.Group{
				ID:   "15",
				Type: "Company",
			},
		}))

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("GetPayload", func(t *testing.T) {
		defer gock.OffAll()

		catchIngestion(1)

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.NoError(t, err)

		payload := flagger.GetPayload("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.Equal(t, "on", payload["newFeature"])

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("GetVariation", func(t *testing.T) {

		gock.New(ingestionURL).
			Post(ingestionPath + apiKey).
			Reply(200)

		defer gock.OffAll()

		flagger, err := initFlaggerInstance(apiKey, "configuration_ingestion.json")
		assert.NoError(t, err)

		variation := flagger.GetVariation("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.Equal(t, "enabled", variation)

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}

func TestFilters(t *testing.T) {
	defer gock.OffAll()
	gock.New(ingestionURL).
		Post(ingestionPath + apiKey).
		Times(6).
		Reply(200)

	flagger, err := initFlaggerInstance(apiKey, "configuration.json")
	assert.NoError(t, err)

	t.Run("positive test", func(t *testing.T) {

		t.Run("LTE, equal test", func(t *testing.T) {
			isEnabled := flagger.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2014-09-20T00:00:00Z",
				}})
			assert.True(t, isEnabled)
		})

		t.Run("LTE, less test", func(t *testing.T) {
			isEnabled := flagger.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2014-08-20T00:00:00Z",
				}})
			assert.True(t, isEnabled)
		})

		t.Run("GTE(equal) and IS test", func(t *testing.T) {
			isEnabled := flagger.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2016-03-16T05:44:23Z",
					"country":   "USA",
				}})
			assert.True(t, isEnabled)
		})

	})

	t.Run("negative test", func(t *testing.T) {
		t.Run("date is out of range", func(t *testing.T) {
			isEnabled := flagger.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2015-09-20T00:00:00Z",
				}})
			assert.False(t, isEnabled)
		})

		t.Run("date is right, country is absent", func(t *testing.T) {
			isEnabled := flagger.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2016-03-16T05:44:23Z",
				}})
			assert.False(t, isEnabled)
		})

		t.Run("date is right, but wrong country", func(t *testing.T) {
			isEnabled := flagger.IsEnabled("color-theme", &core.Entity{ID: "31404847", Type: "User",
				Attributes: map[string]interface{}{
					"createdAt": "2016-03-16T05:44:23Z",
					"country":   "UK",
				}})
			assert.False(t, isEnabled)
		})
	})

	timeout := flagger.Shutdown(1 * time.Second)
	assert.False(t, timeout)
}

// dynamic-pricing flag killSwitch is on at source(configuration.json) but off at sse
func TestFlagger_SSE(t *testing.T) {
	defer gock.OffAll()
	catchIngestion(1)
	flaggerConfigMessage := getConfigMessage()
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)

	broker := sse.NewSSEServer(ctx, flaggerConfigMessage)

	ssePort := "3101"
	go func() {
		log.Fatal("HTTP server error: ", http.ListenAndServe("localhost:"+ssePort, broker))
	}()

	time.Sleep(10 * time.Millisecond)
	var configuration *core.Configuration

	mustJSONOFile("configuration.json", &configuration)
	gock.New(flagsURL).
		Get(flagsPath + apiKey).
		Reply(200).
		JSON(configuration)

	flagger := NewFlagger()
	err := flagger.Init(ctx, &InitArgs{APIKey: apiKey, SSEURL: "http://localhost:" + ssePort + "/"})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	entity := &core.Entity{
		ID:         "kfjvv3",
		Attributes: core.Attributes{"admin": true},
	}

	// dynamic-pricing isKill
	assert.True(t, flagger.IsSampled("dynamic-pricing", entity))
	done()

	timeout := flagger.Shutdown(1 * time.Second)
	assert.False(t, timeout)
}

func TestCustomURLS(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	var configuration *core.Configuration
	mustJSONOFile("configuration.json", &configuration)
	customURL := "https://mycustomurl-somewebsite.com"
	gock.New(customURL).
		Get("/" + apiKey).
		Reply(200).
		JSON(configuration)

	gock.New(customURL).
		Post("/ingest/" + apiKey).
		Reply(200)
	defer gock.OffAll()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	flagger := NewFlagger()
	err := flagger.Init(ctx, &InitArgs{
		APIKey:       apiKey,
		SourceURL:    customURL + "/" + apiKey,
		IngestionURL: customURL + "/ingest/" + apiKey,
		SSEURL:       "http://localhost:8000/v3/sse",
	})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	entity := &core.Entity{
		ID:         "kfjvv3",
		Attributes: core.Attributes{"admin": true},
	}

	// dynamic-pricing isKill
	sampled := flagger.IsSampled("premium-support", entity)
	assert.True(t, sampled)

	shutdown := flagger.Shutdown(2 * time.Second)
	assert.False(t, shutdown)
}

func TestShutdown(t *testing.T) {
	t.Run("Shutdown makes all flag functions return default variation", func(t *testing.T) {
		defer gock.OffAll()
		catchIngestion(1)
		flagger, err := initFlaggerInstance("fdsfsdf3ofsf", "configuration.json")
		assert.NoError(t, err)

		ok := flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		flagger.Shutdown(1 * time.Second)

		notOk := flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.False(t, notOk)

	})

	t.Run("Init, Shutdown, Init recover initial state", func(t *testing.T) {

		defer gock.OffAll()
		flagger, err := initFlaggerInstance(apiKey, "configuration.json")
		assert.NoError(t, err)

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)

		catchIngestion(1)

		flagger, err = initFlaggerInstance(apiKey, "configuration.json")
		assert.NoError(t, err)

		ok := flagger.IsEnabled("enterprise-dashboard", &core.Entity{ID: "31404847", Type: "Company"})
		assert.True(t, ok)

		timeout = flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)

	})
}

func TestIngestion(t *testing.T) {

	t.Run("First 10 exposures are always ingested", func(t *testing.T) {
		defer gock.OffAll()
		flagger, err := initFlaggerInstance(apiKey, "configuration.json")
		assert.NoError(t, err)

		gock.New(ingestionURL).
			Post(ingestionPath + apiKey).
			Times(11).
			Reply(200)

		count := 0
		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {
				count++
				if count == 11 {
					gock.Observe(nil)
				}
			}
		})

		for i := 0; i < 12; i++ {
			flagger.IsEnabled("new-signup-flow", &core.Entity{
				ID: "1",
			})
		}

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, 10, count)

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		assert.Equal(t, 11, count)

	})

	t.Run("Detected flags always ingested", func(t *testing.T) {
		defer gock.OffAll()
		flagger, err := initFlaggerInstance(apiKey, "configuration.json")
		assert.NoError(t, err)

		firstExposuresIngestThreshold := 10
		detectedFlagCount := 7

		totalIngestedFlags := firstExposuresIngestThreshold + detectedFlagCount

		gock.New(ingestionURL).
			Post(ingestionPath + apiKey).
			Times(totalIngestedFlags).
			Reply(200)

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
			flagger.IsEnabled("new-signup-flow", &core.Entity{
				ID: "1",
			})
		}

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, firstExposuresIngestThreshold, count)

		for i := 0; i < detectedFlagCount; i++ {
			flagger.IsEnabled("new-flag-"+randomString(10), &core.Entity{
				ID: "1",
			})
		}

		timeout := flagger.Shutdown(1 * time.Second)
		assert.False(t, timeout)
		assert.Equal(t, totalIngestedFlags, count)

	})
}

func initFlaggerInstance(apiKey, configFileName string) (*Flagger, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancel()
	var configuration *core.Configuration
	mustJSONOFile(configFileName, &configuration)

	gock.New(flagsURL).
		Get(flagsPath + apiKey).
		Reply(200).
		JSON(configuration)

	flagger := NewFlagger()
	err := flagger.Init(ctx, &InitArgs{APIKey: apiKey, SSEURL: "http://localhost:8000/v3/sse"})
	return flagger, err
}

func mustJSONOFile(filename string, v interface{}) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("bad file: %+v", err))
	}

	err = json.Unmarshal(buf, v)
	if err != nil {
		panic(fmt.Sprintf("bad json: %+v", err))
	}
}

func getConfigMessage() []byte {
	flaggerConfigMessage := []byte("id: 76c58618-b75c-4872-a107-986998601fe4\n" +
		"event: flagConfigUpdate\n" +
		"data: ")
	config, _ := ioutil.ReadFile("configuration_sse.json")

	config = bytes.Replace(config, []byte(" "), []byte(""), -1)
	config = bytes.Replace(config, []byte("\n"), []byte(""), -1)
	flaggerConfigMessage = append(flaggerConfigMessage, config...)
	flaggerConfigMessage = append(flaggerConfigMessage, []byte("\n\n")...)
	return flaggerConfigMessage
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func catchIngestion(times int) {
	gock.New(ingestionURL).
		Post(ingestionPath + apiKey).
		Times(times).
		Reply(200)
}
