package v1

import (
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/pkg/events"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// HeadEvent represents a beacon chain head event.
type HeadEvent struct {
	events.BaseEvent
	data     *eth2v1.HeadEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewHeadEvent(beacon events.BeaconDataProvider, meta *xatu.Meta, data *eth2v1.HeadEvent, recvTime time.Time) *HeadEvent {
	return &HeadEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
	}
}

func (e *HeadEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V2.String()
}

func (e *HeadEvent) Data() interface{} {
	return e.data
}

func (e *HeadEvent) Decorated() *xatu.DecoratedEvent {
	//TODO(@matty): Populate event data.
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsHeadV2{},
	}

	if e.beacon == nil {
		return decorated
	}

	//TODO(@matty): Populate additional data.
	extra := &xatu.ClientMeta_AdditionalEthV1EventsHeadV2Data{}
	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsHeadV2{
		EthV1EventsHeadV2: extra,
	}

	return decorated
}
