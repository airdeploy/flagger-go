package flagger

import (
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/internal/httputils"
)

var (
	// SDKInfo represent meta information
	SDKInfo = &core.SDKInfo{
		Name:    "golang",
		Version: "3.1.0",
	}

	defaultAttemptsConnection = 2
	defaultSourceURL          = httputils.MustURL("https://flags.airdeploy.io/v3/config/")
	defaultBackupSourceURL    = httputils.MustURL("https://backup-api.airshiphq.com/v3/config/")
	defaultSSEURL             = httputils.MustURL("https://sse.airdeploy.io/v3/sse/")
	defaultIngestionURL       = httputils.MustURL("https://ingestion.airdeploy.io/v3/ingest/")
)
