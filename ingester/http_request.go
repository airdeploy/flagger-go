package ingester

import (
	"bytes"
	"compress/gzip"
	"github.com/airdeploy/flagger-go/v3/log"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func httpRequest(data []byte, URL string) error {
	var req *http.Request
	if len(data) > 1024 {
		var compressed bytes.Buffer
		w := gzip.NewWriter(&compressed)
		if _, err := w.Write(data); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}

		r, err := http.NewRequest(http.MethodPost, URL, &compressed)
		if err != nil {
			return err
		}
		req = r
		req.Header.Set("Content-Encoding", "gzip")
	} else {
		r, err := http.NewRequest(http.MethodPost, URL, bytes.NewBuffer(data))
		if err != nil {
			return err
		}
		req = r
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("%d: %s", resp.StatusCode, resp.Status)
	}

	if _, err = ioutil.ReadAll(resp.Body); err == nil {
		log.Debugf("Http Call Executed, url: %+v, data: %+v", URL, string(data))
	}
	return errors.Wrap(err, "ioutil.ReadAll")
}
