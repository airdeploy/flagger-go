package ingester

import (
	"context"
	"github.com/airdeploy/flagger-go/v3/core"
	"sync"
)

// An Ingester is a mechanism to summarize and send data to server with retry policy
type Ingester struct {
	entity   *core.Entity
	strategy *groupStrategy
	mux      sync.Mutex
}

type groupStrategy struct {
	// the responsibility of the WaitGroup here is to ensure that all ingestion data
	// is sent when ShutdownWithTimeout is called
	wg            sync.WaitGroup
	ingestionWg   sync.WaitGroup
	retryPolicyWg sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	lock     sync.RWMutex
	isActive bool

	httpRequest httpRequestType // readonly
	sdkInfo     *core.SDKInfo   // readonly
	retryPolicy *retryPolicy    // readonly

	// channels to update ingester config
	updateSDKConfigChannel chan *core.SDKConfig
	ingestionURLChannel    chan string

	// inner channels to prevent synchronization
	ingestionDataChannel chan *IngestionDataRequest
	retryPolicyChannel   chan *retryPolicyRequest

	// ingestion data
	callCount                     int
	exposuresCount                int
	firstExposuresIngestThreshold int
	accumulator                   []*IngestionDataRequest
}

type retryPolicy struct {
	maxMemorySizeInBytes int64
	queue                []*queueElement
	currentMemorySize    int64
}

type queueElement struct {
	data     []byte
	callback RetryPolicyCallback
}

type httpRequestType func(data []byte, ingestionURL string) error

type retryPolicyRequest struct {
	data         []byte              // data to be sent to the server
	ingestionURL string              // URL
	httpRequest  httpRequestType     // function to be executed to send data
	callback     RetryPolicyCallback // callback
}

// RetryPolicyCallback is called when retry policy finishes the processing of the ingestion data httpRequest
// There are 2 possible scenarios:
// 1) ingestion is successfully sent to the server
// 2) new ingestion arrive, so the current ingestion is shift from the queue(not enough memory)
type RetryPolicyCallback func(err error)

// An IngestionDataRequest is a data object for the ingestion request
type IngestionDataRequest struct {
	ID            string           `json:"id"`
	Entities      []*core.Entity   `json:"entities"`
	Exposures     []*core.Exposure `json:"exposures"` // the output of every Flag Function call
	Events        []*core.Event    `json:"events"`    // user generated event
	SDKInfo       *core.SDKInfo    `json:"sdkInfo"`   // Dictionary holding info about the Flagger
	DetectedFlags []string         `json:"detectedFlags"`
}
