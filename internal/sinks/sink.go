package sinks

import (
	"context"

	"github.com/ethpandaops/contributoor/internal/events"
)

// ContributoorSink defines the interface for event sinks.
type ContributoorSink interface {
	// Start initializes and starts the sink.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the sink.
	Stop(ctx context.Context) error
	// HandleEvent processes an event and forwards it to the appropriate destination.
	HandleEvent(ctx context.Context, event events.Event) error
	// Name returns the name of the sink for logging purposes.
	Name() string
}
