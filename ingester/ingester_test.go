package ingester

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

const apiKey = "testApiKey"

func TestNoEntityProvided(t *testing.T) {
	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"}, 0)
	ingester.Activate()
	ingester.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: 1,
		SDKIngestionMaxItems: 500,
	})
	ingester.PublishExposure(&core.Exposure{
		MethodCalled: "isEnabled",
		Entity:       nil,
		Codename:     "test",
		HashKey:      "",
		Variation:    "enabled",
		Timestamp:    time.Now(),
	}, true)

	ingester.Track(&core.Event{
		Name:            "test",
		EventProperties: core.Attributes{"test": true},
	})

	assert.Zero(t, ingester.strategy.callCount)
	assert.Empty(t, ingester.strategy.accumulator)
}

func TestIngestionSendAfter500Calls(t *testing.T) {
	gock.New("https://ingestion.airdeploy.io").
		Post("/collector").
		MatchParam("envKey", apiKey).
		Reply(200)

	var wg sync.WaitGroup
	wg.Add(1)
	defer func() {
		wg.Wait()
		gock.OffAll() // WARNING: gock.OffAll() must executed after wg.Wait()
	}()

	const maxItems = 500
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		// catch ingestion
		if request.Method == http.MethodPost {

			buf, err := ioutil.ReadAll(request.Body)
			assert.NoError(t, err)

			unzipData, err := gUnzipData(buf)
			assert.Nil(t, err)

			var data *IngestionDataRequest
			err = json.Unmarshal(unzipData, &data)
			assert.NoError(t, err)

			if assert.Len(t, data.Entities, 1) {
				assert.EqualValues(t, &core.Entity{ID: "1"}, data.Entities[0])
			}
			assert.Len(t, data.Events, 0)
			assert.Len(t, data.DetectedFlags, 0)
			assert.Len(t, data.Exposures, maxItems)

			wg.Done()
			gock.Observe(nil)
		}
	})

	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"}, 0)
	ingester.Activate()
	ingester.SetURL("https://ingestion.airdeploy.io/collector?envKey=" + apiKey)
	ingester.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: 1,
		SDKIngestionMaxItems: maxItems,
	})

	for i := 0; i < maxItems; i++ {
		ingester.publish(
			&IngestionDataRequest{
				Entities: []*core.Entity{{ID: "1"}},
				Exposures: []*core.Exposure{{
					Codename:     "test",
					HashKey:      "",
					Variation:    "enabled",
					Entity:       &core.Entity{ID: "1"},
					MethodCalled: "isEnabled",
					Timestamp:    time.Now(),
				}},
			})
	}
}

func TestIngestionDeduped25Entities(t *testing.T) {
	gock.New("https://ingestion.airdeploy.io").
		Post("/collector").
		MatchParam("envKey", apiKey).
		Reply(200)

	var wg sync.WaitGroup
	wg.Add(1)
	defer func() {
		wg.Wait()
		gock.OffAll() // WARNING: gock.OffAll() must executed after wg.Wait()
	}()

	const (
		oneSendEntityCount = 25
		sendRepeat         = 20
		maxItems           = oneSendEntityCount * sendRepeat
	)
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		// catch ingestion
		if request.Method == http.MethodPost {

			buf, err := ioutil.ReadAll(request.Body)
			assert.NoError(t, err)

			unzipData, err := gUnzipData(buf)
			assert.Nil(t, err)

			var data *IngestionDataRequest
			err = json.Unmarshal(unzipData, &data)
			assert.NoError(t, err)

			assert.Len(t, data.Entities, oneSendEntityCount)
			assert.Len(t, data.Events, 0)
			assert.Len(t, data.DetectedFlags, 0)
			assert.Len(t, data.Exposures, maxItems)

			wg.Done()
			gock.Observe(nil)
		}
	})

	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"}, 0)
	ingester.Activate()
	ingester.SetURL("https://ingestion.airdeploy.io/collector?envKey=" + apiKey)
	ingester.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: 1,
		SDKIngestionMaxItems: maxItems,
	})

	var entities []*core.Entity
	for i := 0; i < oneSendEntityCount; i++ {
		entities = append(entities, &core.Entity{ID: strconv.Itoa(i)})
	}

	for i := 0; i < sendRepeat; i++ {
		for _, entity := range entities {
			ingester.publish(&IngestionDataRequest{
				Entities: []*core.Entity{entity},
				Exposures: []*core.Exposure{{
					Codename:     "test",
					HashKey:      "",
					Variation:    "enabled",
					Entity:       entity,
					MethodCalled: "isEnabled",
					Timestamp:    time.Now(),
				}},
			})
		}
	}
}

func TestDetectedFlagShouldImmediatelyIngest(t *testing.T) {
	gock.New("https://ingestion.com").
		Post("/v3/ingest/12345678").
		Reply(200)

	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"}, 10)
	ingester.Activate()
	ingester.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: 60,
		SDKIngestionMaxItems: 500,
	})
	ingester.SetURL("https://ingestion.com/v3/ingest/12345678")

	ingester.PublishExposure(&core.Exposure{
		MethodCalled: "isEnabled",
		Entity: &core.Entity{
			ID: "1",
		},
		Codename:  "test",
		HashKey:   "",
		Variation: "enabled",
		Timestamp: time.Now(),
	}, true)

	count := 0
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		// catch ingestion
		gock.Observe(nil)
		count++
	})
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, count)

	timeout := ingester.Shutdown(1 * time.Second)
	assert.False(t, timeout)
}

func TestFirst10ExposureIngest(t *testing.T) {

	gock.New("https://ingestion.com").
		Post("/v3/ingest/12345678").
		Times(11).
		Reply(200)

	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"}, 10)
	ingester.Activate()
	ingester.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: 60,
		SDKIngestionMaxItems: 500,
	})
	ingester.SetURL("https://ingestion.com/v3/ingest/12345678")
	time.Sleep(100 * time.Millisecond)

	callCounter := 0
	gock.Observe(func(request *http.Request, mock gock.Mock) {
		// catch ingestion
		callCounter++
		if callCounter == 11 {
			gock.Observe(nil)
		}
	})

	for i := 0; i < 12; i++ {
		ingester.PublishExposure(&core.Exposure{
			MethodCalled: "isEnabled",
			Entity: &core.Entity{
				ID: "1",
			},
			Codename:  "test",
			HashKey:   "",
			Variation: "enabled",
			Timestamp: time.Now(),
		}, false)
	}

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 10, callCounter)

	timeout := ingester.Shutdown(1 * time.Second)
	assert.False(t, timeout)
	assert.Equal(t, 11, callCounter)
}

func TestIngester_PublishExposure(t *testing.T) {
	t.Run("exposure has no entity, ingester's entity is used", func(t *testing.T) {
		ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"}, 10)
		ingester.Activate()
		ingester.SetConfig(&core.SDKConfig{
			SDKIngestionInterval: 60,
			SDKIngestionMaxItems: 500,
		})

		ingester.SetURL("https://ingestion.com/v3/ingest/12345678")

		entity := &core.Entity{
			ID:         "1",
			Type:       "User",
			Attributes: core.Attributes{"id": "1"},
		}
		ingester.SetEntity(entity)

		gock.New("https://ingestion.com").
			Post("/v3/ingest/12345678").
			Times(1).
			Reply(200)

		gock.Observe(func(request *http.Request, mock gock.Mock) {
			buf, err := ioutil.ReadAll(request.Body)
			assert.NoError(t, err)

			var data *IngestionDataRequest
			err = json.Unmarshal(buf, &data)
			assert.NoError(t, err)

			assert.Zero(t, len(data.DetectedFlags))
			assert.Equal(t, 1, len(data.Entities))
			assert.Equal(t, 1, len(data.Exposures))

			gock.Observe(nil)
		})
		ingester.PublishExposure(&core.Exposure{
			Codename:     "example-flag",
			HashKey:      "12345",
			Variation:    "on",
			Entity:       nil,
			MethodCalled: "getVariation",
			Timestamp:    time.Now(),
		}, false)

		timeout := ingester.Shutdown(1 * time.Second)
		assert.False(t, timeout)
	})
}
