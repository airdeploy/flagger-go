package ingester

import (
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewGroupStrategy(t *testing.T) {
	t.Run("Timer exceeds and ingestion httpRequest is sent", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(0, 1, 500, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			count++
			return nil
		})
		gs.Activate()

		gs.Publish(ingestionDataRequest(false))

		time.Sleep(1010 * time.Millisecond)
		assert.Equal(t, 1, count)
		finishedWithTimeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, finishedWithTimeout)
	})

	t.Run("URL is changed, gs sends data to the new URL", func(t *testing.T) {
		newURL := "http://some-new-url.com"
		gs := initGroupStrategy(0, 60, 4, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			assert.Equal(t, newURL, ingestionURL)
			return nil
		})
		gs.Activate()

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		gs.SetURL(newURL)
		gs.Publish(ingestionDataRequest(false))

		finishedWithTimeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, finishedWithTimeout)
	})

	t.Run("Change interval", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(0, 1, 500, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			count++
			return nil
		})
		gs.Activate()
		gs.Publish(ingestionDataRequest(false))
		// wait for ingestion interval to exceeds
		time.Sleep(1010 * time.Millisecond)
		assert.Equal(t, 1, count)

		gs.SetConfig(&core.SDKConfig{
			SDKIngestionInterval: 2,
			SDKIngestionMaxItems: 500,
		})

		gs.Publish(ingestionDataRequest(false))
		// wait for ingestion interval to exceeds
		time.Sleep(2010 * time.Millisecond)
		assert.Equal(t, 2, count)
		finishedWithTimeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, finishedWithTimeout)
	})

	t.Run("Change maxItems", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(0, 60, 50, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			var ingestionDataRequest IngestionDataRequest
			err := json.Unmarshal(data, &ingestionDataRequest)
			assert.Nil(t, err)
			count++
			if count == 1 {
				assert.Equal(t, 50, len(ingestionDataRequest.Exposures))
			}
			if count == 2 {
				assert.Equal(t, 40, len(ingestionDataRequest.Exposures))
			}

			if count == 3 {
				assert.Equal(t, 10, len(ingestionDataRequest.Exposures))
			}
			return nil
		})
		gs.Activate()

		for i := 0; i < 90; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		// maxItems(50) should be reached, wait for the ingestion to happen
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, count)

		// 40 ingestions are currently waiting
		// setting SDKIngestionMaxItems == 10 must trigger an ingestion
		gs.SetConfig(&core.SDKConfig{
			SDKIngestionInterval: 60,
			SDKIngestionMaxItems: 10,
		})

		// new maxItems(10) should be reached, wait for the ingestion to happen
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 2, count)

		for i := 0; i < 10; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		time.Sleep(10 * time.Millisecond) // wait for the ingestion to happened
		assert.Equal(t, 3, count)
		finishedWithTimeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, finishedWithTimeout)
	})

	t.Run("Ingestion is not sent if there is no data", func(t *testing.T) {
		gs := initGroupStrategy(0, 1, 500, func(data []byte, ingestionURL string) error {
			assert.Fail(t, "No data to publish, must not be called")
			return nil
		})
		gs.Activate()

		time.Sleep(1010 * time.Millisecond) // wait for timer exceeds
		finishedWithTimeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, finishedWithTimeout)
	})

	t.Run("If there is no URL and ShutdownWithTimeout called => ingestion does not happen", func(t *testing.T) {
		count := 0
		gs := newGroupStrategy(&core.SDKInfo{Name: "go", Version: "3.0.0"}, func(data []byte, ingestionURL string) error {
			count++
			return nil
		}, 0)
		gs.Activate()

		gs.Publish(ingestionDataRequest(true))

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.True(t, timeout)
		assert.Zero(t, count)
	})

	t.Run("ShutdownWithTimeout check current state and sends all current data", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 3, func(data []byte, ingestionURL string) error {
			time.Sleep(100 * time.Millisecond)
			log.Debugf("send is finished")
			return nil
		})
		gs.Activate()

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("ShutdownWithTimeout doesn't do anything with an empty Ingester", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		gs.Activate()
		assert.Zero(t, gs.callCount)

		timeout := gs.ShutdownWithTimeout(1 * time.Second)

		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("ShutdownWithTimeout waits for current ingestion to finish", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		gs.Activate()

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest(false))
		}
		time.Sleep(10 * time.Millisecond) // wait 10 milliseconds so ingester triggers ingest function

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("ShutdownWithTimeout ingest current data", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 5, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		gs.Activate()

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("Ingestions are not added after ShutdownWithTimeout false", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(0, 60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			count++
			return nil
		})
		gs.Activate()

		assert.True(t, gs.isActive)

		timeout := gs.ShutdownWithTimeout(1 * time.Second)

		assert.Zero(t, gs.callCount)
		assert.Zero(t, count)
		assert.False(t, timeout)
		assert.False(t, gs.isActive)

		// try to publish a lot after shutdown
		for i := 0; i < 10000; i++ {
			gs.Publish(ingestionDataRequest(true))
		}

		time.Sleep(100 * time.Millisecond) // wait for the possible ingest httpRequest

		assert.Zero(t, gs.callCount)
		assert.Zero(t, count)
		assert.False(t, gs.isActive)
	})

	t.Run("Ingestions are not added after ShutdownWithTimeout true", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(0, 60, 3, func(data []byte, ingestionURL string) error {
			time.Sleep(1 * time.Second)
			count++
			return nil
		})
		gs.Activate()

		assert.True(t, gs.isActive)

		gs.Publish(ingestionDataRequest(false))

		timeout := gs.ShutdownWithTimeout(100 * time.Millisecond)

		assert.Zero(t, count)
		assert.True(t, timeout)
		assert.False(t, gs.isActive)

		// try to publish a lot after shutdown
		for i := 0; i < 10000; i++ {
			gs.Publish(ingestionDataRequest(true))
		}

		time.Sleep(100 * time.Millisecond) // wait for the possible ingest httpRequest

		assert.Zero(t, count)
		assert.False(t, gs.isActive)
	})

	t.Run("Ingestions takes too much time, timeout is reached", func(t *testing.T) {
		count := 0

		gs := initGroupStrategy(0, 60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(1 * time.Second)
			count++
			return nil
		})
		gs.Activate()

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		timeout := gs.ShutdownWithTimeout(100 * time.Millisecond)

		assert.Zero(t, count)
		assert.True(t, timeout)
		assert.False(t, gs.isActive)
	})

	t.Run("publish exposure before ingester is initiated and ShutdownWithTimeout publish the accumulated data", func(t *testing.T) {
		count := 0
		gs := newGroupStrategy(&core.SDKInfo{Name: "go", Version: "3.0.0"}, func(data []byte, ingestionURL string) error {
			count++
			return nil
		}, 0)
		gs.Activate()

		for i := 0; i < 5; i++ {
			gs.Publish(ingestionDataRequest(false))
		}
		gs.SetURL("http://ingestion-url.google.com")
		gs.SetConfig(&core.SDKConfig{
			SDKIngestionInterval: 60,
			SDKIngestionMaxItems: 5,
		})

		timeout := gs.ShutdownWithTimeout(10000 * time.Millisecond)
		assert.False(t, timeout)
		assert.Equal(t, 1, count)

	})

	t.Run("data is still ingesting even if retryPolicy httpRequest is froze", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 500, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Second)
			return nil
		})
		gs.Activate()

		for i := 0; i < 100000; i++ {
			gs.Publish(ingestionDataRequest(false))
		}
	})

	t.Run("data is publishing in the separate thread, ShutdownWithTimeout must stop publishing", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 500, func(data []byte, ingestionURL string) error {
			return nil
		})
		gs.Activate()

		go func() {
			for {
				gs.Publish(ingestionDataRequest(false))
				time.Sleep(1 * time.Nanosecond)
			}
		}()

		time.Sleep(10 * time.Millisecond)

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("Multiple shutdown doesn't break anything", func(t *testing.T) {
		gs := initGroupStrategy(0, 60, 500, func(data []byte, ingestionURL string) error {
			return nil
		})
		gs.Activate()

		timeout := gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)

		timeout = gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
		timeout = gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
		timeout = gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
	})

	t.Run("Detected Flag immediately ingested", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(0, 60, 500, func(data []byte, ingestionURL string) error {
			count++
			return nil
		})
		gs.Activate()

		for i := 0; i < 5; i++ {
			gs.Publish(ingestionDataRequest(true))
		}

		timeout := gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
		assert.Zero(t, gs.callCount)
		assert.Equal(t, 5, count)
	})

	t.Run("First 10 flags immediately ingested", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(10, 60, 500, func(data []byte, ingestionURL string) error {
			count++
			return nil
		})
		gs.Activate()

		for i := 0; i < 200; i++ {
			gs.Publish(ingestionDataRequest(false))
		}

		time.Sleep(1000 * time.Millisecond)
		assert.Equal(t, 10, count)

		timeout := gs.ShutdownWithTimeout(100000 * time.Millisecond)
		assert.False(t, timeout)
		assert.Zero(t, gs.callCount)
		assert.Equal(t, 11, count)

	})

}

func initGroupStrategy(firstExposuresIngestThreshold int, interval, maxItems int, callback httpRequestType) *groupStrategy {
	gs := newGroupStrategy(&core.SDKInfo{Name: "go", Version: "3.0.0"}, callback, firstExposuresIngestThreshold)
	gs.Activate()

	gs.SetURL("https://test.ingestion.com")
	gs.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: interval,
		SDKIngestionMaxItems: maxItems,
	})
	return gs
}

func ingestionDataRequest(addDetected bool) *IngestionDataRequest {
	req := &IngestionDataRequest{
		Entities: []*core.Entity{
			{ID: "111"},
			{ID: "222"},
			{ID: "333"},
		},
		Exposures: []*core.Exposure{
			{
				Codename:     "sound",
				Variation:    "enabled",
				Entity:       &core.Entity{ID: "222"},
				MethodCalled: "isEnabled",
			},
		},
		Events: []*core.Event{
			{Name: "event1"},
			{Name: "event2"},
		},
	}
	if addDetected {
		req.DetectedFlags = []string{"sound"}
	}

	return req
}
