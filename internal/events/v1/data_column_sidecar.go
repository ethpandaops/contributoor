package v1

import (
	"context"
	"fmt"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// DataColumnSidecarEvent represents a data column sidecar event.
type DataColumnSidecarEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.DataColumnSidecarEvent
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewDataColumnSidecarEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.DataColumnSidecarEvent,
	recvTime time.Time,
) *DataColumnSidecarEvent {
	return &DataColumnSidecarEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR.String()),
	}
}

func (e *DataColumnSidecarEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR.String()
}

func (e *DataColumnSidecarEvent) Data() interface{} {
	return e.data
}

func (e *DataColumnSidecarEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_DATA_COLUMN_SIDECAR,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsDataColumnSidecar{
			EthV1EventsDataColumnSidecar: &xatuethv1.EventDataColumnSidecar{
				BlockRoot: xatuethv1.RootAsString(e.data.BlockRoot),
				Slot:      &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
				Index:     &wrapperspb.UInt64Value{Value: e.data.Index},
				//nolint:gosec // KZG commitments count is bounded by protocol limits
				KzgCommitmentsCount: &wrapperspb.UInt32Value{Value: uint32(len(e.data.KZGCommitments))},
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		columnSlot = e.beacon.GetSlot(uint64(e.data.Slot))
		epoch      = e.beacon.GetEpochFromSlot(uint64(e.data.Slot))
	)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsDataColumnSidecar{
		EthV1EventsDataColumnSidecar: &xatu.ClientMeta_AdditionalEthV1EventsDataColumnSidecarData{
			Slot: &xatu.SlotV2{
				Number:        &wrapperspb.UInt64Value{Value: columnSlot.Number()},
				StartDateTime: timestamppb.New(columnSlot.TimeWindow().Start()),
			},
			Epoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
			Propagation: &xatu.PropagationV2{
				SlotStartDiff: &wrapperspb.UInt64Value{
					//nolint:gosec // safe.
					Value: uint64(e.recvTime.Sub(columnSlot.TimeWindow().Start()).Milliseconds()),
				},
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
	if e.beacon.IsSlotFromUnexpectedNetwork(uint64(e.data.Slot)) {
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
			"column_index":          e.data.Index,
		}).Debug("Duplicate data column sidecar event received")

		return true, nil
	}

	return false, nil
}
