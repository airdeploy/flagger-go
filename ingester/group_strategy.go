package ingester

import (
	"context"
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/json"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/google/uuid"
	"sync"
	"time"
)

func newGroupStrategy(sdkInfo *core.SDKInfo, httpRequest httpRequestType, firstExposuresIngestThreshold int) *groupStrategy {
	gs := &groupStrategy{
		wg: sync.WaitGroup{},

		isActive: false,

		httpRequest: httpRequest,
		sdkInfo:     sdkInfo,
		sdkConfig:   defaultSDKConfig,

		retryPolicy: newRetryPolicy(),

		callCount:                     0,
		firstExposuresIngestThreshold: firstExposuresIngestThreshold,
		accumulator:                   make([]*IngestionDataRequest, 0, 100),
	}
	return gs
}

func (gs *groupStrategy) shouldSendIngestionData(ingestionMaxCalls int, data *IngestionDataRequest) bool {
	return (gs.callCount >= ingestionMaxCalls) ||
		(len(data.DetectedFlags) > 0) ||
		(len(data.Exposures) > 0 && gs.exposuresCount <= gs.firstExposuresIngestThreshold)
}

func (gs *groupStrategy) startWorker() {

	go func() {
		gs.lock.RLock()
		sdkConfig := gs.sdkConfig
		ingestionURL := gs.url
		gs.lock.RUnlock()
		ingestionInterval := sdkConfig.IngestionIntervalDuration() // use 50*time.Milliseconds instead of 0
		ingestionTimer := time.NewTimer(ingestionInterval)
		for {
			select {
			case <-ingestionTimer.C:
				//Ingestion timer expires
				gs.lock.RLock()
				count := gs.callCount
				gs.lock.RUnlock()

				gs.wg.Add(1)
				if count > 0 {
					gs.ingest(ingestionURL, func(err error) {
						gs.wg.Done()
					})
				} else {
					gs.wg.Done()
				}
				ingestionTimer.Reset(ingestionInterval)

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
				return
			}
		}
	}()

}

// side effects notice: it clears callCount and accumulator
func (gs *groupStrategy) ingest(ingestionURL string, callback RetryPolicyCallback) {
	gs.lock.Lock()
	defer gs.lock.Unlock()
	bytes, err := transformToBytes(gs.accumulator, gs.sdkInfo)
	if err == nil {
		rpr := &retryPolicyRequest{
			data:         bytes,
			ingestionURL: ingestionURL,
			httpRequest:  gs.httpRequest,
			callback:     callback,
		}
		go gs.retryPolicy.ingest(rpr)
	}
	gs.callCount = 0
	gs.accumulator = []*IngestionDataRequest{}
}

func (gs *groupStrategy) Publish(data *IngestionDataRequest) {
	gs.lock.Lock()
	url := gs.url
	if !gs.isActive {
		gs.lock.Unlock()
		return
	}

	gs.accumulator = append(gs.accumulator, data)
	gs.callCount++

	if exposuresCount := len(data.Exposures); exposuresCount > 0 && (gs.exposuresCount <= gs.firstExposuresIngestThreshold) {
		gs.exposuresCount += exposuresCount
	}
	maxItems := gs.sdkConfig.SDKIngestionMaxItems

	if gs.shouldSendIngestionData(maxItems, data) {
		gs.lock.Unlock()
		gs.wg.Add(1)
		gs.ingest(url, func(err error) {
			gs.wg.Done()
		})
	} else {
		gs.lock.Unlock()
	}
}

func (gs *groupStrategy) Activate(ingestionURL string, config *core.SDKConfig) {
	ctx, cancel := context.WithCancel(context.Background())

	gs.lock.Lock()
	gs.isActive = true
	gs.ctx = ctx
	gs.cancel = cancel
	gs.url = ingestionURL
	if config != nil {
		gs.sdkConfig = config
	}
	gs.lock.Unlock()

	gs.startWorker()
}

// ShutdownWithTimeout features:
// - waits for current ingestion to finish
// - adds all the data in the accumulator, ingest it and waits for the httpRequest to finish
// returns false if shutdown terminates before the timeout
func (gs *groupStrategy) ShutdownWithTimeout(timeout time.Duration) bool {
	gs.lock.Lock()
	if !gs.isActive {
		gs.lock.Unlock()
		return false
	}
	gs.isActive = false // stops new data to be published
	gs.lock.Unlock()

	gs.wg.Add(1)
	gs.cancel() // triggers gs.ctx.Done() async, that's why we add one more to waitGroup

	timer := time.NewTimer(timeout)
	c := make(chan struct{})
	go func() {
		defer close(c)
		gs.wg.Wait()
	}()
	select {
	case <-c:
		timer.Stop()
		log.Debugf("ShutdownWithTimeout is finished by sending all requests")
		return false // completed normally
	case <-timer.C:
		timer.Stop()

		log.Warnf("ShutdownWithTimeout exited with a timeout, some requests are not finished")
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
		ID:            id.String(),
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
