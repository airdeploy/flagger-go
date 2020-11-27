package httputils_test

import (
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/internal/httputils"
	"github.com/airdeploy/flagger-go/v3/internal/utils"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"testing"
)

func TestInternal(t *testing.T) {
	t.Run("return err if unable to get the data", func(t *testing.T) {

		gock.New(utils.FlagsURL).
			Get(utils.FlagsPath + utils.APIKey).
			Times(3).
			Reply(500)

		var configuration *core.Configuration
		err := httputils.GetConfiguration(http.DefaultTransport, utils.FlagsURL+utils.FlagsPath+utils.APIKey, 2, &configuration)
		assert.NotNil(t, err)
		assert.Equal(t, "500: 500 Internal Server Error", err.Error())

		gock.OffAll()
		err = httputils.GetConfiguration(http.DefaultTransport, "http://localhost:3423/randompath", 1, &configuration)
		assert.NotNil(t, err)
	})

	t.Run("retry mechanism", func(t *testing.T) {
		t.Run("available from first attempt", func(t *testing.T) {
			var configFromServer *core.Configuration
			utils.MustJSONFile("../../testdata/configuration.json", &configFromServer)

			gock.New(utils.FlagsURL).
				Get(utils.FlagsPath + utils.APIKey).
				Reply(200).
				JSON(configFromServer)

			var configuration *core.Configuration
			err := httputils.GetConfiguration(http.DefaultTransport, utils.FlagsURL+utils.FlagsPath+utils.APIKey, 3, &configuration)
			assert.Nil(t, err)

			assert.Equal(t, "2779", configuration.HashKey)
		})

		t.Run("two attempt failed, then success", func(t *testing.T) {

			var configFromServer *core.Configuration
			utils.MustJSONFile("../../testdata/configuration.json", &configFromServer)

			gock.New(utils.FlagsURL).
				Get(utils.FlagsPath + utils.APIKey).
				Times(2).
				Reply(500)

			gock.New(utils.FlagsURL).
				Get(utils.FlagsPath + utils.APIKey).
				Reply(200).
				JSON(configFromServer)

			var configuration *core.Configuration
			err := httputils.GetConfiguration(http.DefaultTransport, utils.FlagsURL+utils.FlagsPath+utils.APIKey, 3, &configuration)
			assert.Nil(t, err)

			assert.Equal(t, "2779", configuration.HashKey)
		})
	})
}

func TestMustURL(t *testing.T) {
	t.Run("successfully parse a string", func(t *testing.T) {
		unparsedURL := "http://localhost:3000/path"
		url := httputils.MustURL(unparsedURL)
		assert.Equal(t, unparsedURL, url)
	})

	t.Run("panic is thrown", func(t *testing.T) {
		badURL := "htttttps://schemeurl.#$%^com"

		defer func() {
			if r := recover(); r != nil {
				println("Yay", r)
				assert.Equal(t, "bad url: "+badURL, r)
			}
		}()
		_ = httputils.MustURL(badURL)
		assert.Fail(t, "Must not be reached")
	})
}
