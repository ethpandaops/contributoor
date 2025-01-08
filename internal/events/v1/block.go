package v1

import (
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// BlockEvent represents a beacon chain block event.
type BlockEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.BlockEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewBlockEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	meta *xatu.Meta,
	data *eth2v1.BlockEvent,
	recvTime time.Time,
) *BlockEvent {
	return &BlockEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_V2.String()),
	}
}

func (e *BlockEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_V2.String()
}

func (e *BlockEvent) Data() interface{} {
	return e.data
}

func (e *BlockEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsBlockV2{
			EthV1EventsBlockV2: &xatuethv1.EventBlockV2{
				Slot:                &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
				Block:               xatuethv1.RootAsString(e.data.Block),
				ExecutionOptimistic: e.data.ExecutionOptimistic,
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		slot  = e.beacon.GetSlot(uint64(e.data.Slot))
		epoch = e.beacon.GetEpochFromSlot(uint64(e.data.Slot))
	)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsBlockV2{
		EthV1EventsBlockV2: &xatu.ClientMeta_AdditionalEthV1EventsBlockV2Data{
			Slot: &xatu.SlotV2{
				StartDateTime: timestamppb.New(slot.TimeWindow().Start()),
				Number:        &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
			},
			Epoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
			Propagation: &xatu.PropagationV2{
				SlotStartDiff: &wrapperspb.UInt64Value{
					//nolint:gosec // not concerned in reality
					Value: uint64(e.recvTime.Sub(slot.TimeWindow().Start()).Milliseconds()),
				},
			},
		},
	}

	return decorated
}
