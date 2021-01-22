package flagger

import (
	"github.com/airdeploy/flagger-go/v3/internal/utils"
	"os"
	"testing"

	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/stretchr/testify/assert"
)

func Test_prepareInitArgs(t *testing.T) {
	sourceURL := "https://flags.airdeploy.io/v3/config/"
	backupSourceURL := "https://backup-api.airshiphq.com/v3/config/"
	ingestionURL := "https://ingestion.airdeploy.io/v3/ingest/"
	sseURL := "https://sse.airdeploy.io/v3/sse/"

	t.Run("positive", func(t *testing.T) {
		args1 := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
		}
		args2, err := prepareInitArgs(args1, SDKInfo)

		assert.False(t, args1 == args2)
		assert.NoError(t, err)
	})

	t.Run("positive2", func(t *testing.T) {
		args1 := &InitArgs{APIKey: utils.APIKey}
		args2, err := prepareInitArgs(args1, SDKInfo)
		assert.False(t, args1 == args2)
		assert.EqualValues(t,
			&InitArgs{
				APIKey:          utils.APIKey,
				SourceURL:       defaultSourceURL + utils.APIKey,
				BackupSourceURL: defaultBackupSourceURL + utils.APIKey,
				IngestionURL:    defaultIngestionURL + utils.APIKey,
				SSEURL:          defaultSSEURL + utils.APIKey,
				LogLevel:        "error",
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
		_, err := prepareInitArgs(args, SDKInfo)
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
		_, err := prepareInitArgs(args1, &core.SDKInfo{Name: "", Version: "3.0.0"})
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
		_, err := prepareInitArgs(args, &core.SDKInfo{Name: "golang", Version: ""})
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
		args2, err := prepareInitArgs(args1, SDKInfo)
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
		args2, err := prepareInitArgs(args1, SDKInfo)
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
		args2, err := prepareInitArgs(args1, SDKInfo)
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
		args2, err := prepareInitArgs(args1, SDKInfo)
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
		_, err := prepareInitArgs(args, SDKInfo)
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
		_, err := prepareInitArgs(args, SDKInfo)
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
		_, err := prepareInitArgs(args, SDKInfo)
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
		_, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("bad LogLevel", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       "https://source.airdeploy.io",
			BackupSourceURL: "https://backup.airdeploy.io",
			IngestionURL:    "https://ingestion.airdeploy.io",
			SSEURL:          "https://sse.airdeploy.io",
			LogLevel:        "notValid",
		}
		_, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
	})

	t.Run("Custom URLs", func(t *testing.T) {
		args := &InitArgs{
			APIKey:          utils.APIKey,
			SourceURL:       sourceURL,
			BackupSourceURL: backupSourceURL,
			IngestionURL:    ingestionURL,
			SSEURL:          sseURL,
		}
		args, _ = prepareInitArgs(args, SDKInfo)
		assert.Equal(t, args.SourceURL, sourceURL+utils.APIKey)
		assert.Equal(t, args.BackupSourceURL, backupSourceURL+utils.APIKey)
		assert.Equal(t, args.IngestionURL, ingestionURL+utils.APIKey)
		assert.Equal(t, args.SSEURL, sseURL+utils.APIKey)
	})

	t.Run("Custom URLs from env", func(t *testing.T) {
		_ = os.Setenv(FlaggerAPIKey, utils.APIKey)
		_ = os.Setenv(FlaggerSourceURL, sourceURL)
		_ = os.Setenv(FlaggerBackupSourceURL, backupSourceURL)
		_ = os.Setenv(FlaggerIngestionURL, ingestionURL)
		_ = os.Setenv(FlaggerSSEUrl, sseURL)

		args := &InitArgs{
			APIKey:          "",
			SourceURL:       "",
			BackupSourceURL: "",
			IngestionURL:    "",
			SSEURL:          "",
		}
		args, _ = prepareInitArgs(args, SDKInfo)

		assert.Equal(t, args.SourceURL, sourceURL+utils.APIKey)
		assert.Equal(t, args.BackupSourceURL, backupSourceURL+utils.APIKey)
		assert.Equal(t, args.IngestionURL, ingestionURL+utils.APIKey)
		assert.Equal(t, args.SSEURL, sseURL+utils.APIKey)

		_ = os.Unsetenv(FlaggerAPIKey)
		_ = os.Unsetenv(FlaggerSourceURL)
		_ = os.Unsetenv(FlaggerBackupSourceURL)
		_ = os.Unsetenv(FlaggerIngestionURL)
		_ = os.Unsetenv(FlaggerSSEUrl)
	})

	t.Run("Malformed url in env var", func(t *testing.T) {
		_ = os.Setenv(FlaggerSourceURL, "fdfsdfsdf")
		args := &InitArgs{
			APIKey: utils.APIKey,
		}
		args, err := prepareInitArgs(args, SDKInfo)
		assert.Equal(t, ErrBadInitArgs, err)
		_ = os.Unsetenv(FlaggerSourceURL)
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
