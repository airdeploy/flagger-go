package flagger

import (
	"github.com/airdeploy/flagger-go/v3/core"
	"time"
)

var stdFlagger = NewFlagger()

// Init represent function for initialize Flagger
func Init(args *InitArgs) error {
	return stdFlagger.Init(args)
}

// Publish represent function for publishing Entity into Ingestion URL
func Publish(entity *core.Entity) {
	stdFlagger.Publish(entity)
}

// Track is simple event tracking API.
// Entity is an optional parameter if it was set before.
func Track(event *core.Event) {
	stdFlagger.Track(event)
}

// SetEntity represent function to storing Entity(default like), that will be use instead if any method have no entity
func SetEntity(entity *core.Entity) {
	stdFlagger.SetEntity(entity)
}

// IsEnabled represent function for check if the flag enabled for Entity by codename
func IsEnabled(codename string, entity *core.Entity) bool {
	return stdFlagger.IsEnabled(codename, entity)
}

// IsSampled returns whether or not an entity is within one of the targeted populations.
// However, the entity may or may not be "sampled".
//	A sampled entity may someday receive this feature, but this function only determines whether entity is sampled.
func IsSampled(codename string, entity *core.Entity) bool {
	return stdFlagger.IsSampled(codename, entity)
}

// GetVariation return variation for Entity by codename
func GetVariation(codename string, entity *core.Entity) string {
	return stdFlagger.GetVariation(codename, entity)
}

// GetPayload return payload for Entity by codename
func GetPayload(codename string, entity *core.Entity) core.Payload {
	return stdFlagger.GetPayload(codename, entity)
}

// Shutdown ingests data(if any), stops ingester and closes SSE connection.
// Shutdown waits to finish current ingestion request, but no longer than a timeout.
//
// returns true if closed by timeout
func Shutdown(timeout time.Duration) bool {
	return stdFlagger.Shutdown(timeout)
}
