package flagger

import (
	"context"
	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
)

func getConfiguration(ctx context.Context, rt http.RoundTripper, URL string, attempts int, recv interface{}) error {
	err := retry.Retry(func(attempt uint) error {
		req, err := http.NewRequest(http.MethodGet, URL, http.NoBody)
		if err != nil {
			return err
		}

		req = req.WithContext(ctx)
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
		return errors.WithStack(err)
	},
		strategy.Limit(uint(attempts)))

	return err
}
