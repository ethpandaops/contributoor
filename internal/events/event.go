package events

import (
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
)

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
	// GetValidatorIndex returns the validator index for a given position in a committee.
	GetValidatorIndex(epoch phase0.Epoch, slot phase0.Slot, committeeIndex phase0.CommitteeIndex, position uint64) (phase0.ValidatorIndex, error)
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
