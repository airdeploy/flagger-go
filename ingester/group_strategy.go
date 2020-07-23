package ingester

import (
	"context"
	"github.com/airdeploy/flagger-go/core"
	"github.com/airdeploy/flagger-go/json"
	"github.com/airdeploy/flagger-go/log"
	"github.com/google/uuid"
	"sync"
	"time"
)

func NewGroupStrategy(sdkInfo *core.SDKInfo, httpRequest HttpRequest) *GroupStrategy {
	ctx, cancel := context.WithCancel(context.Background())
	gs := &GroupStrategy{
		wg:          sync.WaitGroup{},
		ingestionWg: sync.WaitGroup{},
		ctx:         ctx,
		cancel:      cancel,

		isActive: true,

		httpRequest: httpRequest,
		sdkInfo:     sdkInfo,
		retryPolicy: NewRetryPolicy(),

		// 16 is a magic number. 2 should work just fine, but 16>2, so 16
		updateSDKConfigChannel: make(chan *core.SDKConfig, 16),
		ingestionURLChannel:    make(chan string, 16),

		// size must be really big to prevent synchronization
		ingestionDataChannel: make(chan *IngestionDataRequest, 4000),
		// to prevent stutter of the Publish function
		retryPolicyChannel: make(chan *RetryPolicyRequest, 1000),

		callCount:   0,
		accumulator: make([]*IngestionDataRequest, 0, 100),
	}
	gs.wg.Add(1) // wait for ingester to initialize
	gs.startWorker()
	return gs
}

func (gs *GroupStrategy) startWorker() {
	requestPolicyCtx, cancelRequestPolicy := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case request := <-gs.retryPolicyChannel:
				gs.retryPolicy.ingest(request)
			case <-requestPolicyCtx.Done():
				return
			}
		}
	}()

	go func() {
		sdkConfig := <-gs.updateSDKConfigChannel
		ingestionInterval := sdkConfig.IngestionIntervalDuration() // use 50*time.Milliseconds instead of 0
		ingestionMaxCalls := sdkConfig.SDKIngestionMaxItems
		ingestionTimer := time.NewTimer(ingestionInterval)
		ingestionURL := <-gs.ingestionURLChannel
		gs.wg.Done() // initialized

		for {
			select {
			case data := <-gs.ingestionDataChannel:
				// Adding data to accumulator
				gs.accumulator = append(gs.accumulator, data)
				gs.callCount++
				gs.ingestionWg.Done()

				if gs.callCount >= ingestionMaxCalls {
					gs.wg.Add(1)
					gs.ingest(ingestionURL, func(err error) {
						gs.wg.Done()
					})
				}
			case <-ingestionTimer.C:
				// Ingestion timer expires
				if gs.callCount > 0 {
					gs.wg.Add(1)
					gs.ingest(ingestionURL, func(err error) {
						gs.wg.Done()
					})
				}
				ingestionTimer.Reset(ingestionInterval)

			case URL := <-gs.ingestionURLChannel:
				log.Debugf("URL has changed to %s", URL)
				ingestionURL = URL

			case sdkConfig = <-gs.updateSDKConfigChannel:
				log.Debugf("New sdkConfig %+v", sdkConfig)
				ingestionInterval = sdkConfig.IngestionIntervalDuration()
				ingestionTimer.Reset(ingestionInterval)
				ingestionMaxCalls = sdkConfig.SDKIngestionMaxItems
				if gs.callCount >= ingestionMaxCalls {
					gs.wg.Add(1)
					gs.ingest(ingestionURL, func(err error) {
						gs.wg.Done()
					})
				}

			case <-gs.ctx.Done():
				// this case is triggered by ShutdownWithTimeout
				// gs.wg delta is 1
				if gs.callCount > 0 {
					gs.ingest(ingestionURL, func(err error) {
						gs.wg.Done()
					})
				} else {
					gs.wg.Done() // release waiting for the shutdown function
				}
				cancelRequestPolicy()
				return
			}
		}
	}()

}

// side effects notice: it clears callCount and accumulator
func (gs *GroupStrategy) ingest(ingestionURL string, callback RetryPolicyCallback) {
	bytes, err := transformToBytes(gs.accumulator, gs.sdkInfo)
	if err == nil {
		rpr := &RetryPolicyRequest{
			data:         bytes,
			ingestionURL: ingestionURL,
			httpRequest:  gs.httpRequest,
			callback:     callback,
		}
		gs.retryPolicyChannel <- rpr
	}
	gs.callCount = 0
	gs.accumulator = []*IngestionDataRequest{}
}

func (gs *GroupStrategy) Publish(data *IngestionDataRequest) {
	gs.lock.RLock()
	defer gs.lock.RUnlock()
	if gs.isActive {
		gs.ingestionWg.Add(1)
		gs.ingestionDataChannel <- data
	}
}

func (gs *GroupStrategy) SetConfig(sdkConfig *core.SDKConfig) {
	gs.lock.RLock()
	defer gs.lock.RUnlock()
	if gs.isActive {
		gs.updateSDKConfigChannel <- sdkConfig
	}
}

func (gs *GroupStrategy) SetURL(ingestionURL string) {
	gs.lock.RLock()
	defer gs.lock.RUnlock()
	if gs.isActive {
		gs.ingestionURLChannel <- ingestionURL
	}
}

// Important notes, this function
// - waits for group strategy to be initialized(URL and sdkConfig are set)
// - waits for current ingestion to finish
// - waits for the ingestionDataChannel to read all the data
// - adds all the data in the accumulator, ingest it and waits for the httpRequest to finish
// returns false if shutdown terminates before the timeout
func (gs *GroupStrategy) ShutdownWithTimeout(timeout time.Duration) bool {
	gs.lock.Lock()
	if !gs.isActive {
		defer gs.lock.Unlock()
		return false // todo: refactor this, it's not beautiful solution to multiple ShutdownWithTimeout calls
	}
	gs.isActive = false // stops new data to be published
	gs.lock.Unlock()

	waitTimeout(&gs.ingestionWg, timeout) // wait to read from ingestionDataChannel

	gs.wg.Add(1) // this is required because it could be that ingester has some data to send
	gs.cancel()  // triggers gs.ctx.Done() async, that's why we add one more to waitGroup

	timer := time.NewTimer(timeout)
	c := make(chan struct{})
	go func() {
		defer close(c)
		gs.wg.Wait()
	}()
	select {
	case <-c:
		timer.Stop()
		log.Debugf("All requests are finished")
		return false // completed normally
	case <-timer.C:
		timer.Stop()

		log.Warnf("Some httpRequest are not finished, exited with a timeout")
		return true // timed out
	}
}

func transformToBytes(acc []*IngestionDataRequest, sdkInfo *core.SDKInfo) ([]byte, error) {
	var entitiesMap = make(map[string]*core.Entity, 4)
	var events = make([]*core.Event, 0, 4)
	var exposures = make([]*core.Exposure, 0, 4)
	var detectedFlags = make(map[string]struct{}, 4) // map used as set

	for _, data := range acc {
		for _, entity := range data.Entities {
			entitiesMap[entity.ID+entity.Type] = entity
		}

		// append ensures len(data.events) != 0
		events = append(events, data.Events...)
		exposures = append(exposures, data.Exposures...)

		for _, flag := range data.DetectedFlags {
			detectedFlags[flag] = struct{}{}
		}
	}

	id, err := uuid.NewRandom()
	if err != nil {
		log.Errorf("Error while generating UUID: %+v", err)
	}

	return json.Marshal(&IngestionDataRequest{
		Id:            id.String(),
		Entities:      entityMapToSlice(entitiesMap),
		Exposures:     exposures,
		Events:        events,
		SDKInfo:       sdkInfo,
		DetectedFlags: detectedFlagsMapToSlice(detectedFlags),
	})
}

func entityMapToSlice(entityMap map[string]*core.Entity) []*core.Entity {
	slice := make([]*core.Entity, 0, len(entityMap))
	for _, v := range entityMap {
		slice = append(slice, v)
	}
	return slice
}

func detectedFlagsMapToSlice(detectedFlags map[string]struct{}) []string {
	slice := make([]string, 0, len(detectedFlags))
	for k := range detectedFlags {
		slice = append(slice, k)
	}
	return slice
}