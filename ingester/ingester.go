package ingester

import (
	"github.com/airdeploy/flagger-go/core"
	"github.com/airdeploy/flagger-go/log"
	"time"
)

// check public interface on compile time
var _ interface {
	Publish(entity *core.Entity)
	Track(event *core.Event)
	PublishExposure(exposure *core.Exposure, isNewFlag bool)
	SetEntity(entity *core.Entity)
	SetConfig(v *core.SDKConfig)
	SetURL(ingestionURL string)
} = new(Ingester)

// NewIngester
func NewIngester(sdkInfo *core.SDKInfo) *Ingester {
	return &Ingester{
		strategy: NewGroupStrategy(sdkInfo, httpRequest),
	}
}

// Shutdown
func (i *Ingester) Shutdown(timeout time.Duration) bool {
	return i.strategy.ShutdownWithTimeout(timeout)
}

// Publish
func (i *Ingester) Publish(entity *core.Entity) {
	i.publish(&IngestionDataRequest{
		Entities: []*core.Entity{entity},
	})
}

// Track
func (i *Ingester) Track(event *core.Event) {
	i.mux.Lock()
	defer i.mux.Unlock()

	// do not publish if entity is not provided to the ingester
	if event.Entity == nil && i.entity == nil {
		log.Warnf("No entity provided to the flagger. Event will not be recorded, %+v", event)
		return
	}

	var entities []*core.Entity

	if event.Entity != nil {
		entities = append(entities, event.Entity)
	} else if i.entity != nil {
		entities = append(entities, i.entity)
	} else {
		entities = []*core.Entity{}
	}

	i.publish(&IngestionDataRequest{
		Entities: entities,
		Events:   []*core.Event{event},
	})
}

// PublishExposure
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

func (i *Ingester) publish(data *IngestionDataRequest) {
	i.strategy.Publish(data)
}

// SetEntity
func (i *Ingester) SetEntity(entity *core.Entity) {
	i.mux.Lock()
	i.entity = entity
	i.mux.Unlock()
}

// SetConfig
func (i *Ingester) SetConfig(v *core.SDKConfig) {
	i.strategy.SetConfig(v.Copy())
}

// SetURL
func (i *Ingester) SetURL(ingestionURL string) {
	i.strategy.SetURL(ingestionURL)
}
