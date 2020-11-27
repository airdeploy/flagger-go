package utils

import (
	"encoding/json"
	"fmt"
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
