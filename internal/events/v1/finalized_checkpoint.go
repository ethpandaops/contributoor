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

// FinalizedCheckpointEvent represents a beacon chain finalization event.
type FinalizedCheckpointEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.FinalizedCheckpointEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewFinalizedCheckpointEvent(log logrus.FieldLogger, beacon events.BeaconDataProvider, meta *xatu.Meta, data *eth2v1.FinalizedCheckpointEvent, recvTime time.Time) *FinalizedCheckpointEvent {
	return &FinalizedCheckpointEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_FINALIZED_CHECKPOINT_V2.String()),
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
		Data: &xatu.DecoratedEvent_EthV1EventsFinalizedCheckpointV2{
			EthV1EventsFinalizedCheckpointV2: &xatuethv1.EventFinalizedCheckpointV2{
				Epoch: &wrapperspb.UInt64Value{Value: uint64(e.data.Epoch)},
				State: xatuethv1.RootAsString(e.data.State),
				Block: xatuethv1.RootAsString(e.data.Block),
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	epoch := e.beacon.GetEpoch(uint64(e.data.Epoch))

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsFinalizedCheckpointV2{
		EthV1EventsFinalizedCheckpointV2: &xatu.ClientMeta_AdditionalEthV1EventsFinalizedCheckpointV2Data{
			Epoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
		},
	}

	return decorated
}
