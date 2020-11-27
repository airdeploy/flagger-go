package httputils

import (
	"fmt"
	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"
)

// GetConfiguration gets json from URL and parse it as recv interface.
// If it fails it tries the amount of times defined in the attempts. If fails after retries returns with an error.
// Returns nil on success
func GetConfiguration(rt http.RoundTripper, URL string, attempts int, recv interface{}) error {
	err := retry.Retry(func(attempt uint) error {
		req, err := http.NewRequest(http.MethodGet, URL, http.NoBody)
		if err != nil {
			return err
		}

		req.Header.Set("content-type", "application/json")

		resp, err := rt.RoundTrip(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("%d: %s", resp.StatusCode, resp.Status)
		}

		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		err = json.Unmarshal(buf, recv)
		return err
	},
		strategy.Limit(uint(attempts)))

	return err
}

// MustURL validates string to be URL. Panics if it's not a valid URL
func MustURL(u string) string {
	_, err := url.Parse(u)
	if err != nil {
		panic(fmt.Sprintf("bad url: %s", u))
	}
	return u
}
