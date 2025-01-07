package sinks

import (
	"time"

	"github.com/ethpandaops/xatu/pkg/proto/xatu"
)

// mockEvent implements events.Event interface for testing.
type mockEvent struct {
	eventType string
	time      time.Time
	decorated *xatu.DecoratedEvent
}

func (e *mockEvent) Type() string                    { return e.eventType }
func (e *mockEvent) Time() time.Time                 { return e.time }
func (e *mockEvent) Data() interface{}               { return e.decorated }
func (e *mockEvent) Decorated() *xatu.DecoratedEvent { return e.decorated }
func (e *mockEvent) Meta() *xatu.Meta                { return e.decorated.Meta }
