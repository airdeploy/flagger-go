package ingester

import (
	"github.com/airdeploy/flagger-go/core"
	"github.com/airdeploy/flagger-go/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewGroupStrategy(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	t.Run("Timer exceeds and ingestion httpRequest is sent", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(1, 500, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			count++
			return nil
		})

		gs.Publish(ingestionDataRequest())
		time.Sleep(1010 * time.Millisecond) // wait for the timer to exceeds
		assert.Equal(t, 1, count)
	})

	t.Run("Change url", func(t *testing.T) {
		newUrl := "http://some-new-url.com"
		gs := initGroupStrategy(60, 4, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			assert.Equal(t, newUrl, ingestionURL)
			return nil
		})

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest())
		}

		gs.SetURL(newUrl)
		gs.Publish(ingestionDataRequest())

		time.Sleep(100) // wait for the ingestion to happened
	})

	t.Run("Change interval", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(1, 500, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			count++
			return nil
		})
		gs.Publish(ingestionDataRequest())
		time.Sleep(1010 * time.Millisecond) // wait for the timer to exceeds
		assert.Equal(t, 1, count)

		gs.SetConfig(&core.SDKConfig{
			SDKIngestionInterval: 2,
			SDKIngestionMaxItems: 500,
		})

		gs.Publish(ingestionDataRequest())
		time.Sleep(2010 * time.Millisecond) // wait for the new timer to exceeds
		assert.Equal(t, 2, count)

	})

	t.Run("Change maxItems", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(60, 50, func(data []byte, ingestionURL string) error {
			assert.NotNil(t, data)
			count++
			return nil
		})

		for i := 0; i < 90; i++ {
			gs.Publish(ingestionDataRequest())
		}

		time.Sleep(10 * time.Millisecond) // wait for the ingestion to happened
		assert.Equal(t, 1, count)

		// 40 ingestions are currently waiting
		// setting SDKIngestionMaxItems == 10 must trigger an ingestion
		gs.SetConfig(&core.SDKConfig{
			SDKIngestionInterval: 60,
			SDKIngestionMaxItems: 10,
		})

		time.Sleep(10 * time.Millisecond) // wait for the ingestion to happened
		assert.Equal(t, 2, count)

		for i := 0; i < 10; i++ {
			gs.Publish(ingestionDataRequest())
		}

		time.Sleep(10 * time.Millisecond) // wait for the ingestion to happened
		assert.Equal(t, 3, count)

	})

	t.Run("Ingestion is not sent if there is no data", func(t *testing.T) {
		initGroupStrategy(1, 500, func(data []byte, ingestionURL string) error {
			assert.Fail(t, "No data to publish, must not be called")
			return nil
		})

		time.Sleep(1010 * time.Millisecond) // wait for timer exceeds
	})

	t.Run("If there is no URL and ShutdownWithTimeout called => ingestion does not happen", func(t *testing.T) {
		count := 0
		gs := NewGroupStrategy(&core.SDKInfo{Name: "go", Version: "3.0.0"}, func(data []byte, ingestionURL string) error {
			count++
			return nil
		})

		gs.Publish(ingestionDataRequest())

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.True(t, timeout)
		assert.Zero(t, count)
	})

	t.Run("ShutdownWithTimeout check current state and sends all current data", func(t *testing.T) {
		gs := initGroupStrategy(60, 3, func(data []byte, ingestionURL string) error {
			time.Sleep(100 * time.Millisecond)
			log.Debugf("send is finished")
			return nil
		})

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest())
		}

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("ShutdownWithTimeout doesn't do anything with an empty Ingester", func(t *testing.T) {
		gs := initGroupStrategy(60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		assert.Zero(t, gs.callCount)

		timeout := gs.ShutdownWithTimeout(1 * time.Second)

		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("ShutdownWithTimeout waits for current ingestion to finish", func(t *testing.T) {
		gs := initGroupStrategy(60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest())
		}
		time.Sleep(10 * time.Millisecond) // wait 10 milliseconds so ingester triggers ingest function

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("ShutdownWithTimeout ingest current data", func(t *testing.T) {
		gs := initGroupStrategy(60, 5, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest())
		}

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.Zero(t, gs.callCount)
		assert.False(t, timeout)
	})

	t.Run("Ingestions are not added after ShutdownWithTimeout false", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			count++
			return nil
		})

		assert.True(t, gs.isActive)

		timeout := gs.ShutdownWithTimeout(1 * time.Second)

		assert.Zero(t, gs.callCount)
		assert.Zero(t, count)
		assert.False(t, timeout)
		assert.False(t, gs.isActive)

		// try to publish a lot after shutdown
		for i := 0; i < 10000; i++ {
			gs.Publish(ingestionDataRequest())
		}

		time.Sleep(100 * time.Millisecond) // wait for the possible ingest httpRequest

		assert.Zero(t, gs.callCount)
		assert.Zero(t, count)
		assert.False(t, gs.isActive)
	})

	t.Run("Ingestions are not added after ShutdownWithTimeout true", func(t *testing.T) {
		count := 0
		gs := initGroupStrategy(60, 3, func(data []byte, ingestionURL string) error {
			time.Sleep(1 * time.Second)
			count++
			return nil
		})

		assert.True(t, gs.isActive)

		gs.Publish(ingestionDataRequest())

		timeout := gs.ShutdownWithTimeout(100 * time.Millisecond)

		assert.Zero(t, count)
		assert.True(t, timeout)
		assert.False(t, gs.isActive)

		// try to publish a lot after shutdown
		for i := 0; i < 10000; i++ {
			gs.Publish(ingestionDataRequest())
		}

		time.Sleep(100 * time.Millisecond) // wait for the possible ingest httpRequest

		assert.Zero(t, count)
		assert.False(t, gs.isActive)
	})

	t.Run("Ingestions takes too much time, timeout is reached", func(t *testing.T) {
		count := 0

		gs := initGroupStrategy(60, 3, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(1 * time.Second)
			count++
			return nil
		})

		for i := 0; i < 3; i++ {
			gs.Publish(ingestionDataRequest())
		}

		timeout := gs.ShutdownWithTimeout(100 * time.Millisecond)

		assert.Zero(t, count)
		assert.True(t, timeout)
		assert.False(t, gs.isActive)
	})

	t.Run("publish exposure before ingester is initiated and ShutdownWithTimeout publish the accumulated data", func(t *testing.T) {
		count := 0
		gs := NewGroupStrategy(&core.SDKInfo{Name: "go", Version: "3.0.0"}, func(data []byte, ingestionURL string) error {
			count++
			return nil
		})

		for i := 0; i < 5; i++ {
			gs.Publish(ingestionDataRequest())
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
		logrus.SetLevel(logrus.ErrorLevel)
		gs := initGroupStrategy(60, 500, func(data []byte, ingestionURL string) error {
			log.Debugf("ingest is triggered")
			time.Sleep(100 * time.Second)
			return nil
		})

		for i := 0; i < 100000; i++ {
			gs.Publish(ingestionDataRequest())
		}
	})

	t.Run("data is publishing in the separate thread, ShutdownWithTimeout must stop publishing", func(t *testing.T) {
		logrus.SetLevel(logrus.ErrorLevel)
		gs := initGroupStrategy(60, 500, func(data []byte, ingestionURL string) error {
			return nil
		})

		go func() {
			for {
				gs.Publish(ingestionDataRequest())
				time.Sleep(1 * time.Nanosecond)
			}
		}()

		time.Sleep(10 * time.Millisecond)

		timeout := gs.ShutdownWithTimeout(1 * time.Second)
		assert.False(t, timeout)
	})

	t.Run("Multiple shutdown doesn't break anything", func(t *testing.T) {
		gs := initGroupStrategy(60, 500, func(data []byte, ingestionURL string) error {
			return nil
		})

		timeout := gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)

		timeout = gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
		timeout = gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
		timeout = gs.ShutdownWithTimeout(1000 * time.Millisecond)
		assert.False(t, timeout)
	})

}

func initGroupStrategy(interval, maxItems int, callback HttpRequest) *GroupStrategy {
	gs := NewGroupStrategy(&core.SDKInfo{Name: "go", Version: "3.0.0"}, callback)

	gs.SetURL("https://test.ingestion.com")
	gs.SetConfig(&core.SDKConfig{
		SDKIngestionInterval: interval,
		SDKIngestionMaxItems: maxItems,
	})
	return gs
}

func ingestionDataRequest() *IngestionDataRequest {
	return &IngestionDataRequest{
		Entities: []*core.Entity{
			{ID: "111"},
			{ID: "222"},
			{ID: "333"},
		},

		DetectedFlags: []string{"sound"},
		Exposures: []*core.Exposure{
			{
				Codename:     "sound",
				Variation:    "enabled",
				Entity:       &core.Entity{ID: "222"},
				MethodCalled: "isEnabled",
			},
			{
				Codename:     "wallet",
				Variation:    "bitcoin",
				Entity:       &core.Entity{ID: "333"},
				MethodCalled: "isSampled",
			},
		},
		Events: []*core.Event{
			{Name: "event1"},
			{Name: "event2"},
		},
	}
}
