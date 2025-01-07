package v1

import (
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FinalizedCheckpointEvent represents a beacon chain finalization event.
type FinalizedCheckpointEvent struct {
	events.BaseEvent
	data     *eth2v1.FinalizedCheckpointEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewFinalizedCheckpointEvent(beacon events.BeaconDataProvider, meta *xatu.Meta, data *eth2v1.FinalizedCheckpointEvent, recvTime time.Time) *FinalizedCheckpointEvent {
	return &FinalizedCheckpointEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
	}
}

func (e *FinalizedCheckpointEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_FINALIZED_CHECKPOINT_V2.String()
}

func (e *FinalizedCheckpointEvent) Data() interface{} {
	return e.data
}

func (e *FinalizedCheckpointEvent) Decorated() *xatu.DecoratedEvent {
	//TODO(@matty): Populate event data.
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_FINALIZED_CHECKPOINT_V2,
			DateTime: timestamppb.New(e.Time()),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsFinalizedCheckpointV2{},
	}

	if e.beacon == nil {
		return decorated
	}

	//TODO(@matty): Populate additional data.
	extra := &xatu.ClientMeta_AdditionalEthV1EventsFinalizedCheckpointV2Data{}
	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsFinalizedCheckpointV2{
		EthV1EventsFinalizedCheckpointV2: extra,
	}

	return decorated
}
