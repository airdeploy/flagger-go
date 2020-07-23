package ingester

import (
	"context"
	"github.com/airdeploy/flagger-go/core"
	"sync"
)

type Ingester struct {
	entity   *core.Entity
	strategy *GroupStrategy
	mux      sync.Mutex
}

type GroupStrategy struct {
	// the responsibility of the WaitGroup here is to ensure that all ingestion data
	// is sent when ShutdownWithTimeout is called
	wg          sync.WaitGroup
	ingestionWg sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	lock     sync.RWMutex
	isActive bool

	httpRequest HttpRequest   // readonly
	sdkInfo     *core.SDKInfo // readonly
	retryPolicy *RetryPolicy  // readonly

	// channels to update ingester config
	updateSDKConfigChannel chan *core.SDKConfig
	ingestionURLChannel    chan string

	// inner channels to prevent synchronization
	ingestionDataChannel chan *IngestionDataRequest
	retryPolicyChannel   chan *RetryPolicyRequest

	// ingestion data
	callCount   int
	accumulator []*IngestionDataRequest
}

type RetryPolicy struct {
	maxMemorySizeInBytes int64
	queue                []*QueueElement
	currentMemorySize    int64
}

type QueueElement struct {
	data     []byte
	callback RetryPolicyCallback
}

type HttpRequest func(data []byte, ingestionURL string) error

type RetryPolicyRequest struct {
	data         []byte              // data to be sent to the server
	ingestionURL string              // URL
	httpRequest  HttpRequest         // function to be executed to send data
	callback     RetryPolicyCallback // callback
}

// This callback is called when retry policy finishes the processing of the ingestion data httpRequest
// There are 2 possible scenarios:
// 1) ingestion is successfully sent to the server
// 2) new ingestion arrive, so the current ingestion is shift from the queue(not enough memory)
type RetryPolicyCallback func(err error)

type IngestionDataRequest struct {
	Id            string           `json:"id"`
	Entities      []*core.Entity   `json:"entities"`
	Exposures     []*core.Exposure `json:"exposures"` // the output of every single API call
	Events        []*core.Event    `json:"events"`    // user generated event
	SDKInfo       *core.SDKInfo    `json:"sdkInfo"`   // Dictionary holding info about the Flagger version that's sending data back
	DetectedFlags []string         `json:"detectedFlags"`
}
