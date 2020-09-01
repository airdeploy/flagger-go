package flagger

import (
	"context"
	"github.com/airdeploy/flagger-go/v3/core"
	"time"
)

var stdFlagger = NewFlagger()

// Init represent function for initialize Flagger
func Init(ctx context.Context, args *InitArgs) error {
	return stdFlagger.Init(ctx, args)
}

// Publish represent function for publishing Entity into Ingestion URL
func Publish(ctx context.Context, entity *core.Entity) {
	stdFlagger.Publish(ctx, entity)
}

// Track
func Track(ctx context.Context, event *core.Event) {
	stdFlagger.Track(ctx, event)
}

// SetEntity represent function to storing Entity(default like), that will be use instead if any method have no entity
func SetEntity(entity *core.Entity) {
	stdFlagger.SetEntity(entity)
}

// IsEnabled represent function for check if the flag enabled for Entity by codename
func IsEnabled(codename string, entity *core.Entity) bool {
	return stdFlagger.IsEnabled(codename, entity)
}

// IsEnabled represent function for check if the flag sampled for Entity by codename
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

func Shutdown(timeout time.Duration) bool {
	return stdFlagger.Shutdown(timeout)
}
