package flagger

import (
	"context"
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"testing"
	"time"
)

func TestInternal(t *testing.T) {
	t.Run("return err if unable to get the data", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		gock.New(flagsURL).
			Get(flagsPath + apiKey).
			Times(3).
			Reply(500)

		var configuration *core.Configuration
		err := getConfiguration(ctx, http.DefaultTransport, flagsURL+flagsPath+apiKey, defaultAttemptsConnection, &configuration)
		assert.NotNil(t, err)
		assert.Equal(t, "500: 500 Internal Server Error", err.Error())
	})

	t.Run("retry mechanism", func(t *testing.T) {
		t.Run("available from first attempt", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var configFromServer *core.Configuration
			mustJSONOFile("configuration.json", &configFromServer)

			gock.New(flagsURL).
				Get(flagsPath + apiKey).
				Reply(200).
				JSON(configFromServer)

			var configuration *core.Configuration
			err := getConfiguration(ctx, http.DefaultTransport, flagsURL+flagsPath+apiKey, 3, &configuration)
			assert.Nil(t, err)

			assert.Equal(t, "2779", configuration.HashKey)
		})

		t.Run("two attempt failed, then success", func(t *testing.T) {

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var configFromServer *core.Configuration
			mustJSONOFile("configuration.json", &configFromServer)

			gock.New(flagsURL).
				Get(flagsPath + apiKey).
				Times(2).
				Reply(500)

			gock.New(flagsURL).
				Get(flagsPath + apiKey).
				Reply(200).
				JSON(configFromServer)

			var configuration *core.Configuration
			err := getConfiguration(ctx, http.DefaultTransport, flagsURL+flagsPath+apiKey, 3, &configuration)
			assert.Nil(t, err)

			assert.Equal(t, "2779", configuration.HashKey)
		})
	})
}
