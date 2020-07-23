package flagger

import (
	"fmt"
	"net/url"

	"github.com/airdeploy/flagger-go/core"
)

var (
	// SDKInfo represent meta information. do not modify this!
	SDKInfo = &core.SDKInfo{
		Name:    "golang",
		Version: "3.0.0",
	}

	defaultSDKConfig = &core.SDKConfig{
		SDKIngestionMaxItems: 500,
		SDKIngestionInterval: 60,
	}

	defaultAttemptsConnection = 2
	defaultSourceURL          = mustURL("https://flags.airdeploy.io/v3/config/")
	defaultBackupSourceURL    = mustURL("https://backup-api.airshiphq.com/v3/config/")
	defaultSSEURL             = mustURL("https://sse.airdeploy.io/v3/sse/")
	defaultIngestionURL       = mustURL("https://ingestion.airdeploy.io/v3/ingest/")
)

func mustURL(u string) string {
	_, err := url.Parse(u)
	if err != nil {
		panic(fmt.Sprintf("bad url: %s", u))
	}
	return u
}
