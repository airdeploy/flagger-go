package ingester

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test2FailsAndSuccessfullySend(t *testing.T) {
	policy := NewRetryPolicy()

	// send will fail and must be put in queue
	policy.ingest(&RetryPolicyRequest{
		data:         []byte("first"),
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})
	assert.Equal(t, 1, len(policy.queue))

	policy.ingest(&RetryPolicyRequest{
		data:         []byte("second"),
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})
	assert.Equal(t, 2, len(policy.queue))

	// send will complete and queue must be drained until empty
	policy.ingest(&RetryPolicyRequest{
		data:         []byte("third"),
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return nil
		},
		callback: func(err error) {},
	})
	assert.Zero(t, len(policy.queue))
	assert.Zero(t, policy.currentMemorySize)
}

func TestWontPutInQueueBecauseMaxMemoryExceeds(t *testing.T) {
	policy := NewRetryPolicy()
	policy.SetMaxSize(0)
	// send will fail and won't be put in queue because it exceeds maxMemorySizeInBytes
	policy.ingest(&RetryPolicyRequest{
		data:         []byte("test"),
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})

	assert.Zero(t, len(policy.queue))
	assert.Zero(t, policy.currentMemorySize)
}

func TestQueueIsFullReplaceFirstElement(t *testing.T) {
	policy := NewRetryPolicy()
	policy.SetMaxSize(0)

	bytes := []byte("test")
	policy.SetMaxSize(size(bytes) + 1)

	// send will fail and must be put in queue
	policy.ingest(&RetryPolicyRequest{
		data:         bytes,
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})
	assert.Equal(t, len(policy.queue), 1)

	// send will fail and must replace the first element
	policy.ingest(&RetryPolicyRequest{
		data:         []byte("tes"),
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})

	assert.Equal(t, len(policy.queue), 1)
	assert.Equal(t, policy.queue[0].data, []byte("tes"))
}

func TestIngestionIsBiggerThanAMaxSize(t *testing.T) {
	policy := NewRetryPolicy()
	big := []byte("verybigingestion")
	small := []byte("small")

	policy.SetMaxSize(size(small) + 1)

	// send will fail and must be put in queue
	policy.ingest(&RetryPolicyRequest{
		data:         small,
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})

	// send will fail but won't be put in queue because it's too big
	policy.ingest(&RetryPolicyRequest{
		data:         big,
		ingestionURL: "",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})

	assert.Equal(t, len(policy.queue), 1)
	assert.Equal(t, policy.queue[0].data, small)
}

func TestURLHasChanged(t *testing.T) {
	policy := NewRetryPolicy()

	policy.ingest(&RetryPolicyRequest{
		data:         []byte("tes"),
		ingestionURL: "http://google.com",
		httpRequest: func(data []byte, ingestionURL string) error {
			return errors.New("some connection problem")
		},
		callback: func(err error) {},
	})

	otherURL := "http://some-server.com"
	calls := 0
	policy.ingest(&RetryPolicyRequest{
		data:         []byte("tes"),
		ingestionURL: otherURL,
		httpRequest: func(data []byte, ingestionURL string) error {
			assert.Equal(t, ingestionURL, otherURL)
			calls++
			return nil
		},
		callback: func(err error) {},
	})

	assert.Equal(t, calls, 2)
}

func TestCallback(t *testing.T) {
	policy := NewRetryPolicy()

	called := false
	policy.ingest(&RetryPolicyRequest{
		data:         []byte("tes"),
		ingestionURL: "http://google.com",
		httpRequest: func(data []byte, ingestionURL string) error {
			return nil
		},
		callback: func(err error) {
			called = true
		},
	})

	time.Sleep(100)
	assert.True(t, called)
}
