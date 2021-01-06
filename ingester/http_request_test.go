package ingester

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	serverURL = "http://localhost:8000"
	postPath  = "/v3/ingest/9f5859owpnhxszee"
	url       = serverURL + postPath
)

func TestHttpRequest(t *testing.T) {

	t.Run("data is big enough, send gzip data", func(t *testing.T) {
		data := generateIngestionDataRequest(4, 3)
		dataStr, err := json.Marshal(data)
		assert.Nil(t, err)
		assert.True(t, len(dataStr) > 1024)

		var wg sync.WaitGroup
		wg.Add(1)
		defer func() {
			wg.Wait()
			gock.OffAll()
		}()

		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {

				buf, err := ioutil.ReadAll(request.Body)
				assert.NoError(t, err)

				unzipData, err := gUnzipData(buf)
				assert.Nil(t, err)

				var data *IngestionDataRequest
				err = json.Unmarshal(unzipData, &data)
				assert.Nil(t, err)

				assert.Len(t, data.Entities, 3)
				assert.Len(t, data.Events, 0)
				assert.Len(t, data.DetectedFlags, 0)
				assert.Len(t, data.Exposures, 4)

				gock.Observe(nil)

				wg.Done()
			}
		})

		gock.New(serverURL).
			Post(postPath).
			MatchType("json").
			Compression("gzip").
			Reply(200)

		err = httpRequest(dataStr, url)
		assert.Nil(t, err)
	})

	t.Run("data is small, sent as is", func(t *testing.T) {
		data := generateIngestionDataRequest(1, 1)
		dataStr, err := json.Marshal(data)
		assert.True(t, len(dataStr) < 1024)

		var wg sync.WaitGroup
		wg.Add(1)
		defer func() {
			wg.Wait()
			gock.OffAll()
		}()

		gock.Observe(func(request *http.Request, mock gock.Mock) {
			// catch ingestion
			if request.Method == http.MethodPost {

				buf, err := ioutil.ReadAll(request.Body)
				assert.NoError(t, err)

				var data *IngestionDataRequest
				err = json.Unmarshal(buf, &data)
				assert.Nil(t, err)

				assert.Len(t, data.Entities, 1)
				assert.Len(t, data.Events, 0)
				assert.Len(t, data.DetectedFlags, 0)
				assert.Len(t, data.Exposures, 1)

				gock.Observe(nil)

				wg.Done()
			}
		})

		gock.New(serverURL).
			Post(postPath).
			MatchType("json").
			Reply(200)

		err = httpRequest(dataStr, url)
		assert.Nil(t, err)
	})

	t.Run("status code != 200", func(t *testing.T) {
		data := generateIngestionDataRequest(3, 2)
		dataStr, err := json.Marshal(data)
		assert.Nil(t, err)
		gock.New(serverURL).
			Post(postPath).
			MatchType("json").
			Reply(500)

		err = httpRequest(dataStr, url)
		assert.NotNil(t, err)
		gock.OffAll()
	})

	t.Run("wrong URL", func(t *testing.T) {
		data := generateIngestionDataRequest(1, 1)
		dataStr, err := json.Marshal(data)
		assert.Nil(t, err)

		err = httpRequest(dataStr, "https://(&TGR(&#$G$#&($:1234/dada/dasdsa/dasda")
		log.Printf("%+v", err)
		assert.NotNil(t, err)
	})
}

func gUnzipData(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}

func generateIngestionDataRequest(exposuresCount, entitiesCount int) IngestionDataRequest {
	var entities []*core.Entity
	for i := 0; i < entitiesCount; i++ {
		entities = append(entities,
			&core.Entity{ID: strconv.Itoa(i), Attributes: core.Attributes{"id": strconv.Itoa(i), "type": "User"}},
		)
	}
	data := IngestionDataRequest{
		ID:        uuid.New().String(),
		Entities:  entities,
		Exposures: []*core.Exposure{},
		Events:    []*core.Event{},
		SDKInfo: &core.SDKInfo{
			Name:    "golang",
			Version: "3.0.0",
		},
		DetectedFlags: []string{},
	}
	for i := 0; i < exposuresCount; i++ {
		data.Exposures = append(data.Exposures, &core.Exposure{
			Codename:     "example-flag",
			HashKey:      "123",
			Variation:    "on",
			Entity:       entities[rand.Intn(len(entities))],
			MethodCalled: "getVariation",
			Timestamp:    time.Now(),
		})
	}

	return data
}
