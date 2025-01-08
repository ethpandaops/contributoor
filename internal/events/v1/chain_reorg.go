package v1

import (
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ChainReorgEvent represents a beacon chain reorganization event.
type ChainReorgEvent struct {
	events.BaseEvent
	data     *eth2v1.ChainReorgEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewChainReorgEvent(beacon events.BeaconDataProvider, meta *xatu.Meta, data *eth2v1.ChainReorgEvent, recvTime time.Time) *ChainReorgEvent {
	return &ChainReorgEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
	}
}

func (e *ChainReorgEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_CHAIN_REORG_V2.String()
}

func (e *ChainReorgEvent) Data() interface{} {
	return e.data
}

func (e *ChainReorgEvent) Decorated() *xatu.DecoratedEvent {
	//TODO(@matty): Populate event data.
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_CHAIN_REORG_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsChainReorgV2{
			EthV1EventsChainReorgV2: xatuethv1.NewReorgEventV2FromGoEth2ClientEvent(e.data),
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		slot  = e.beacon.GetSlot(uint64(e.data.Slot))
		epoch = e.beacon.GetEpochFromSlot(uint64(e.data.Slot))
	)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsChainReorgV2{
		EthV1EventsChainReorgV2: &xatu.ClientMeta_AdditionalEthV1EventsChainReorgV2Data{
			Slot: &xatu.SlotV2{
				Number:        &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
				StartDateTime: timestamppb.New(slot.TimeWindow().Start()),
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
