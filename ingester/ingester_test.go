package ingester

import (
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/airdeploy/flagger-go/core"
	"github.com/airdeploy/flagger-go/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func TestNoEntityProvided(t *testing.T) {
	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"})
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
	apiKey := randomString(8)
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

	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"})
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
	apiKey := randomString(8)
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

	ingester := NewIngester(&core.SDKInfo{Name: "golang", Version: "3.0.0"})
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

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}
