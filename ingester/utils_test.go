package ingester

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestUtils(t *testing.T) {

	timeout := 100
	t.Run("timeout is reached", func(t *testing.T) {
		start := time.Now()
		wg := sync.WaitGroup{}
		wg.Add(1)

		isTimeoutReached := waitTimeout(&wg, time.Duration(timeout)*time.Millisecond)
		assert.True(t, isTimeoutReached)
		duration := int(time.Since(start) / time.Millisecond)
		assert.GreaterOrEqual(t, duration, timeout)
	})

	t.Run("wg is finished, timeout is not reached", func(t *testing.T) {
		start := time.Now()
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			time.Sleep(10 * time.Millisecond)
			wg.Done()
		}()

		isTimeoutReached := waitTimeout(&wg, time.Duration(timeout)*time.Millisecond)
		assert.False(t, isTimeoutReached)
		duration := int(time.Since(start) / time.Millisecond)
		assert.Less(t, duration, timeout)
	})

}
