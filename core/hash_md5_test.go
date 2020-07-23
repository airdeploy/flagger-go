package core

import (
	"log"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHashLower(t *testing.T) {
	for _, tt := range []struct {
		id   string
		hash float64
	}{
		{
			id:   "1434",
			hash: 0.47103858437236173,
		},
		{
			id:   "4310",
			hash: 0.7868047339684145,
		},
		{
			id:   "1434300",
			hash: 0.11996106696333557,
		},
	} {
		t.Run(tt.id, func(t *testing.T) {
			hash := HashMD5(tt.id)
			assert.Equal(t, tt.hash, hash)
		})
	}
}

func TestSampling(t *testing.T) {
	var first, second []float64
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	iterations := 1000 * (r.Intn(10) + 1)
	hashingError := 0.1
	threshold := float64(iterations) * hashingError

	for i := 0; i < iterations; i++ {
		id := strconv.FormatInt(r.Int63(), 10)
		md5 := HashMD5(id)
		if md5 < 0.5 {
			first = append(first, md5)
		} else {
			second = append(second, md5)
		}
	}

	deviation := math.Abs(float64(len(first) - len(second)))
	log.Printf("Iterations: %+v", iterations)
	log.Printf("Deviation: %+v", deviation)
	log.Printf("Error %+v", deviation/float64(iterations))

	if deviation > threshold {
		t.Fail()
	}
}
