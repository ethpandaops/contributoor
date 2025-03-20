package events

import (
	"context"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
)

//go:generate mockgen -package mock -destination mock/event.mock.go github.com/ethpandaops/contributoor/internal/events Event
//go:generate mockgen -package mock -destination mock/beacon_data_provider.mock.go github.com/ethpandaops/contributoor/internal/events BeaconDataProvider

// BeaconDataProvider defines the interface for getting beacon chain data needed by events.
type BeaconDataProvider interface {
	// GetWallclock returns the wallclock for the beacon chain.
	GetWallclock() *ethwallclock.EthereumBeaconChain
	// GetSlot returns the wallclock slot for a given slot number.
	GetSlot(slot uint64) ethwallclock.Slot
	// GetEpoch returns the wallclock epoch for a given epoch number.
	GetEpoch(epoch uint64) ethwallclock.Epoch
	// GetEpochFromSlot returns the wallclock epoch for a given slot number.
	GetEpochFromSlot(slot uint64) ethwallclock.Epoch
	// Synced returns true if the beacon node is synced.
	Synced(ctx context.Context) error
	// Node returns the underlying beacon node instance.
	Node() beacon.Node
	// IsSlotFromUnexpectedNetwork checks if a slot appears to be from an unexpected network
	// by comparing it with the current wallclock slot.
	IsSlotFromUnexpectedNetwork(eventSlot uint64) bool
}

// Event is the interface that all events must implement.
type Event interface {
	// Meta returns the metadata of the event.
	Meta() *xatu.Meta
	// Type returns the type of the event.
	Type() string
	// Time returns the time of the event.
	Time() time.Time
	// Data returns the data of the event.
	Data() interface{}
	// Decorated returns the decorated event.
	Decorated() *xatu.DecoratedEvent
	// Ignore returns true if the event should be ignored.
	Ignore(ctx context.Context) (bool, error)
}

// BaseEvent provides common functionality for all events.
type BaseEvent struct {
	meta *xatu.Meta
	time time.Time
}

func NewBaseEvent(meta *xatu.Meta) BaseEvent {
	return BaseEvent{
		meta: meta,
		time: time.Now(),
	}
}

func (e *BaseEvent) Meta() *xatu.Meta { return e.meta }
func (e *BaseEvent) Time() time.Time  { return e.time }
