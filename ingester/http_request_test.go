package ingester

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
)

const (
	serverUrl = "http://localhost:8000"
	postPath  = "/v3/ingest/9f5859owpnhxszee"
	url       = serverUrl + postPath
)

func TestHttpRequest(t *testing.T) {

	t.Run("data is big enough, send gzip data", func(t *testing.T) {
		data := "{\n    \"id\": \"59689e2f-9274-49e6-85fd-c6c6750c461a\",\n    \"entities\": [\n        {\n            \"id\": \"rptd6i\",\n            \"attributes\": {\n                \"id\": \"rptd6i\"\n            },\n            \"type\": \"User\"\n        },\n        {\n            \"id\": \"2mz3s\",\n            \"attributes\": {\n                \"id\": \"2mz3s\"\n            },\n            \"type\": \"User\"\n        },\n        {\n            \"id\": \"jz0r8s\",\n            \"attributes\": {\n                \"id\": \"jz0r8s\"\n            },\n            \"type\": \"User\"\n        }\n    ],\n    \"exposures\": [\n        {\n            \"hashkey\": \"1962742\",\n            \"codename\": \"example-flag\",\n            \"variation\": \"on\",\n            \"entity\": {\n                \"id\": \"rptd6i\",\n                \"attributes\": {\n                    \"id\": \"rptd6i\"\n                },\n                \"type\": \"User\"\n            },\n            \"methodCalled\": \"isEnabled\",\n            \"timestamp\": \"2020-07-16T18:38:56.639Z\"\n        },\n        {\n            \"hashkey\": \"1962742\",\n            \"codename\": \"example-flag\",\n            \"variation\": \"on\",\n            \"entity\": {\n                \"id\": \"2mz3s\",\n                \"attributes\": {\n                    \"id\": \"2mz3s\"\n                },\n                \"type\": \"User\"\n            },\n            \"methodCalled\": \"isSampled\",\n            \"timestamp\": \"2020-07-16T18:38:56.641Z\"\n        },\n        {\n            \"hashkey\": \"1962742\",\n            \"codename\": \"example-flag\",\n            \"variation\": \"on\",\n            \"entity\": {\n                \"id\": \"jz0r8s\",\n                \"attributes\": {\n                    \"id\": \"jz0r8s\"\n                },\n                \"type\": \"User\"\n            },\n            \"methodCalled\": \"getVariation\",\n            \"timestamp\": \"2020-07-16T18:38:56.642Z\"\n        }\n    ],\n    \"events\": [],\n    \"sdkInfo\": {\n        \"name\": \"nodejs\",\n        \"version\": \"3.0.0\"\n    },\n    \"detectedFlags\": []\n}"
		assert.True(t, len(data) > 1024)

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
				assert.Len(t, data.Exposures, 3)

				gock.Observe(nil)

				wg.Done()
			}
		})

		gock.New(serverUrl).
			Post(postPath).
			MatchType("json").
			Compression("gzip").
			Reply(200)

		err := httpRequest([]byte(data), url)
		assert.Nil(t, err)
	})

	t.Run("data is small, sent as is", func(t *testing.T) {
		data := "{\n    \"id\": \"59689e2f-9274-49e6-85fd-c6c6750c461a\",\n    \"entities\": [\n        {\n            \"id\": \"jz0r8s\",\n            \"attributes\": {\n                \"id\": \"jz0r8s\"\n            },\n            \"type\": \"User\"\n        }\n    ],\n    \"exposures\": [\n        {\n            \"hashkey\": \"1962742\",\n            \"codename\": \"example-flag\",\n            \"variation\": \"on\",\n            \"entity\": {\n                \"id\": \"jz0r8s\",\n                \"attributes\": {\n                    \"id\": \"jz0r8s\"\n                },\n                \"type\": \"User\"\n            },\n            \"methodCalled\": \"getVariation\",\n            \"timestamp\": \"2020-07-16T18:38:56.642Z\"\n        }\n    ],\n    \"events\": [],\n    \"sdkInfo\": {\n        \"name\": \"nodejs\",\n        \"version\": \"3.0.0\"\n    },\n    \"detectedFlags\": []\n}"

		assert.True(t, len(data) < 1024)

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

		gock.New(serverUrl).
			Post(postPath).
			MatchType("json").
			Reply(200)

		err := httpRequest([]byte(data), url)
		assert.Nil(t, err)
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
