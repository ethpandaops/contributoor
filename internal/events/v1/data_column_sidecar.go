package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DataColumnSidecarEventData is a placeholder for the actual data column sidecar event type.
// This will be replaced with eth2v1.DataColumnSidecarEvent once upstream support is available.
type DataColumnSidecarEventData struct {
	BlockRoot   [32]byte
	Slot        uint64
	ColumnIndex uint64
}

// DataColumnSidecarEvent represents a data column sidecar event.
// NOTE: This implementation is prepared for upstream support. Once available:
// - Replace DataColumnSidecarEventData with eth2v1.DataColumnSidecarEvent
// - Update the Decorated() method to use proper xatu types
// - Update Type() method to use xatu.Event_BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR.
type DataColumnSidecarEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *DataColumnSidecarEventData // Will be replaced with *eth2v1.DataColumnSidecarEvent
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewDataColumnSidecarEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *DataColumnSidecarEventData, // Will be replaced with *eth2v1.DataColumnSidecarEvent
	recvTime time.Time,
) *DataColumnSidecarEvent {
	return &DataColumnSidecarEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", "BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR"),
	}
}

func (e *DataColumnSidecarEvent) Type() string {
	// Placeholder until xatu.Event_BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR is available
	return "BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR"
}

func (e *DataColumnSidecarEvent) Data() interface{} {
	return e.data
}

func (e *DataColumnSidecarEvent) Decorated() *xatu.DecoratedEvent {
	// For now, return a minimal decorated event
	// This will be properly implemented once upstream support is available
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_UNKNOWN, // Placeholder
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		// Data field will be populated once xatu supports data column sidecars
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		columnSlot = e.beacon.GetSlot(e.data.Slot)
		epoch      = e.beacon.GetEpochFromSlot(e.data.Slot)
	)

	// Add basic metadata that's compatible with current xatu types
	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsHead{
		EthV1EventsHead: &xatu.ClientMeta_AdditionalEthV1EventsHeadData{
			Slot: &xatu.Slot{
				Number:        columnSlot.Number(),
				StartDateTime: timestamppb.New(columnSlot.TimeWindow().Start()),
			},
			Epoch: &xatu.Epoch{
				Number:        epoch.Number(),
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
			Propagation: &xatu.Propagation{
				//nolint:gosec // not concerned in reality
				SlotStartDiff: uint64(e.recvTime.Sub(columnSlot.TimeWindow().Start()).Milliseconds()),
			},
		},
	}

	return decorated
}

// Ignore determines if the event should be ignored.
// Following the blob sidecar pattern - NO subnet filtering, only sync/network/duplicate checks.
func (e *DataColumnSidecarEvent) Ignore(ctx context.Context) (bool, error) {
	// Check if beacon node is synced
	if err := e.beacon.Synced(ctx); err != nil {
		return true, err
	}

	// Check if event is from an unexpected network based on slot
	if e.beacon.IsSlotFromUnexpectedNetwork(e.data.Slot) {
		e.log.WithField("slot", e.data.Slot).Warn("Ignoring data column sidecar event from unexpected network")

		return true, nil
	}

	// Duplicate detection
	hash, err := hashstructure.Hash(e.data, hashstructure.FormatV2, nil)
	if err != nil {
		return true, err
	}

	item, retrieved := e.cache.GetOrSet(fmt.Sprint(hash), e.recvTime, ttlcache.WithTTL[string, time.Time](ttlcache.DefaultTTL))
	if retrieved {
		e.log.WithFields(logrus.Fields{
			"hash":                  hash,
			"time_since_first_item": time.Since(item.Value()),
			"slot":                  e.data.Slot,
			"column_index":          e.data.ColumnIndex,
		}).Debug("Duplicate data column sidecar event received")

		return true, nil
	}

	// IMPORTANT: NO subnet filtering logic here
	// Unlike single_attestation, we process ALL data column sidecar events
	// No calls to RecordSeenSubnet() or IsActiveSubnet()

	return false, nil
}
