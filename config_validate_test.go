package flagger

import (
	"github.com/airdeploy/flagger-go/v3/internal/utils"
	"testing"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/stretchr/testify/assert"
)

func Test_prepareInitArgs(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		args2, sdkInfo, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, sdkInfo == SDKInfo)
		assert.Equal(t, sdkInfo, SDKInfo)
		assert.False(t, args1 == args2)
		assert.NoError(t, err)
	})

	t.Run("positive2", func(t *testing.T) {
		args1 := &InitArgs{APIKey: utils.APIKey}
		args2, sdkInfo, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, sdkInfo == SDKInfo)
		assert.Equal(t, sdkInfo, SDKInfo)
		assert.False(t, args1 == args2)
		assert.EqualValues(t,
			&InitArgs{
				APIKey:          utils.APIKey,
				SourceURL:       defaultSourceURL + utils.APIKey,
				BackupSourceURL: defaultBackupSourceURL + utils.APIKey,
				IngestionURL:    defaultIngestionURL + utils.APIKey,
				SSEURL:          defaultSSEURL + utils.APIKey,
			},
			args2)
		assert.NoError(t, err)
	})

	t.Run("empty APIKey", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          "",
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		_, _, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("empty SDKInfo.Name", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		_, _, err := prepareInitArgs(args1, &core.SDKInfo{Name: "", Version: "3.0.0"})
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("empty SDKInfo.Version", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		_, _, err := prepareInitArgs(args, &core.SDKInfo{Name: "golang", Version: ""})
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("empty SourceURL", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		args2, sdkInfo, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, sdkInfo == SDKInfo)
		assert.Equal(t, sdkInfo, SDKInfo)
		assert.False(t, args1 == args2)
		assert.NoError(t, err)
		assert.Equal(t, defaultSourceURL+utils.APIKey, args2.SourceURL)
	})

	t.Run("empty BackupSourceURL", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		args2, sdkInfo, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, sdkInfo == SDKInfo)
		assert.Equal(t, sdkInfo, SDKInfo)
		assert.False(t, args1 == args2)
		assert.NoError(t, err)
		assert.Equal(t, defaultBackupSourceURL+utils.APIKey, args2.BackupSourceURL)
	})

	t.Run("empty IngestionURL", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "",
			SSEURL:          "https://sse.airdeploy.io",
		}
		args2, sdkInfo, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, sdkInfo == SDKInfo)
		assert.Equal(t, sdkInfo, SDKInfo)
		assert.False(t, args1 == args2)
		assert.NoError(t, err)
		assert.Equal(t, defaultIngestionURL+utils.APIKey, args2.IngestionURL)
	})

	t.Run("empty SSEURL", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "",
		}
		args2, sdkInfo, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, sdkInfo == SDKInfo)
		assert.Equal(t, sdkInfo, SDKInfo)
		assert.False(t, args1 == args2)
		assert.NoError(t, err)
		assert.Equal(t, defaultSSEURL+utils.APIKey, args2.SSEURL)
	})

	t.Run("bad SourceURL", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "bad url",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		_, _, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("bad BackupSourceURL", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "bad url",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		_, _, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("bad IngestionURL", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "bad url",
			SSEURL:          "https://sse.airdeploy.io",
		}
		_, _, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("bad SSEURL", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "bad url",
		}
		_, _, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("Custom URLs", func(t *testing.T) {
		sourceURL := "https://flags.airdeploy.io/v3/config/"
		backupSourceURL := "https://backup-api.airshiphq.com/v3/config/"
		ingestionURL := "https://ingestion.airdeploy.io/v3/ingest/"
		sseURL := "https://sse.airdeploy.io/v3/sse/"
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       sourceURL,
			BackupSourceURL: backupSourceURL,
			IngestionURL:    ingestionURL,
			SSEURL:          sseURL,
		}
		args, _, _ = prepareInitArgs(args, SDKInfo)
		assert.Equal(t, args.SourceURL, sourceURL+utils.APIKey)
		assert.Equal(t, args.BackupSourceURL, backupSourceURL+utils.APIKey)
		assert.Equal(t, args.IngestionURL, ingestionURL+utils.APIKey)
		assert.Equal(t, args.SSEURL, sseURL+utils.APIKey)
	})
}

func TestInitArgs_copy(t *testing.T) {
	args := &InitArgs{
		APIKey:          utils.APIKey,
		SourceURL:       "SourceURL",
		BackupSourceURL: "BackupSourceURL",
		IngestionURL:    "IngestionURL",
		SSEURL:          "SSEURL",
	}
	assert.EqualValues(t, args, args.copy())
}
