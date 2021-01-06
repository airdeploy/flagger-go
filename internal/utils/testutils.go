package utils

import (
	"encoding/json"
	"fmt"
	"github.com/airdeploy/flagger-go/v3/ingester"
	"io"
	"io/ioutil"
)

const (
	FlagsURL          = "https://flags.airdeploy.io"
	BackupFlagsURL    = "https://backup-api.airshiphq.com"
	FlagsPath         = "/v3/config/"
	IngestionURL      = "https://ingestion.airdeploy.io"
	IngestionPath     = "/v3/ingest/"
	APIKey            = "testApiKey"
	WhitelistedUserID = "90843823"
	SseURL            = "http://localhost:8000/v3/sse"
)

// MustJSONFile reads a file from filename and parse in as JSON to provided interface
func MustJSONFile(filename string, v interface{}) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("bad file: %+v", err))
	}

	err = json.Unmarshal(buf, v)
	if err != nil {
		panic(fmt.Sprintf("bad json: %+v", err))
	}
}

// ParseIngestionBody returns parse body and error
func ParseIngestionBody(body io.ReadCloser) (*ingester.IngestionDataRequest, error) {
	buf, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var data *ingester.IngestionDataRequest
	err = json.Unmarshal(buf, &data)
	return data, err
}
