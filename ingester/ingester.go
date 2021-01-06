package ingester

import (
	"github.com/airdeploy/flagger-go/v3/core"
	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/google/uuid"
	"time"
)

// check public interface on compile time
var _ interface {
	Publish(entity *core.Entity)
	Track(event *core.Event)
	PublishExposure(exposure *core.Exposure, isNewFlag bool)
	SetEntity(entity *core.Entity)
	Activate(ingestionURL string, config *core.SDKConfig)
} = new(Ingester)

// NewIngester creates new instance of ingester
func NewIngester(sdkInfo *core.SDKInfo, firstExposuresIngestThreshold int) *Ingester {
	return &Ingester{
		strategy: newGroupStrategy(sdkInfo, httpRequest, firstExposuresIngestThreshold),
	}
}

// Shutdown shutdowns the ingester
// return true if existed because of timeout
func (i *Ingester) Shutdown(timeout time.Duration) bool {
	return i.strategy.ShutdownWithTimeout(timeout)
}

// Publish publishes new entity
func (i *Ingester) Publish(entity *core.Entity) {
	i.publish(&IngestionDataRequest{
		Entities: []*core.Entity{entity},
	})
}

// Track adds new event to the ingester
func (i *Ingester) Track(event *core.Event) {
	i.mux.RLock()

	// do not publish if entity is not provided to the ingester
	if event.Entity == nil && i.entity == nil {
		log.Warnf("No entity provided to the flagger. Event will not be recorded, %+v", event)
		i.mux.RUnlock()
		return
	}

	var entities []*core.Entity

	if event.Entity != nil {
		entities = append(entities, event.Entity)
	} else if i.entity != nil {
		entities = append(entities, i.entity)
	}

	request := &IngestionDataRequest{
		Entities: entities,
		Events:   []*core.Event{event},
	}
	i.mux.RUnlock()

	i.publish(request)
}

// PublishExposure adds new exposure to the ingester
func (i *Ingester) PublishExposure(exposure *core.Exposure, isNewFlag bool) {
	i.mux.Lock()
	defer i.mux.Unlock()

	if exposure.Entity == nil && i.entity == nil {
		return // have no entity

	} else if exposure.Entity == nil {
		exposure.Entity = i.entity
	}

	ingestionData := &IngestionDataRequest{
		Exposures: []*core.Exposure{exposure},
	}

	if isNewFlag {
		ingestionData.DetectedFlags = []string{exposure.Codename}
	}

	ingestionData.Entities = []*core.Entity{exposure.Entity}
	i.publish(ingestionData)
}

// SendEmptyIngestion sends empty ingestion to inform a server about sdk usage
func (i *Ingester) SendEmptyIngestion() {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return
	}
	i.publish(&IngestionDataRequest{
		ID: newUUID.String(),
	})
}

func (i *Ingester) publish(data *IngestionDataRequest) {
	i.strategy.Publish(data)
}

// SetEntity sets the default entity to the ingester
func (i *Ingester) SetEntity(entity *core.Entity) {
	i.mux.Lock()
	i.entity = entity
	i.mux.Unlock()
}

// Activate activates ingester strategy. Must be the first method called after NewIngester
func (i *Ingester) Activate(ingestionURL string, config *core.SDKConfig) {
	i.strategy.Activate(ingestionURL, config)
}
