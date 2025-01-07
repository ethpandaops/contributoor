package v1

import (
	"fmt"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// VoluntaryExitEvent represents a validator voluntary exit event.
type VoluntaryExitEvent struct {
	events.BaseEvent
	data     *phase0.SignedVoluntaryExit
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewVoluntaryExitEvent(beacon events.BeaconDataProvider, meta *xatu.Meta, data *phase0.SignedVoluntaryExit, recvTime time.Time) *VoluntaryExitEvent {
	return &VoluntaryExitEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
	}
}

func (e *VoluntaryExitEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_VOLUNTARY_EXIT_V2.String()
}

func (e *VoluntaryExitEvent) Data() interface{} {
	return e.data
}

func (e *VoluntaryExitEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_VOLUNTARY_EXIT_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsVoluntaryExitV2{
			EthV1EventsVoluntaryExitV2: &xatuethv1.EventVoluntaryExitV2{
				Message: &xatuethv1.EventVoluntaryExitMessageV2{
					Epoch:          &wrapperspb.UInt64Value{Value: uint64(e.data.Message.Epoch)},
					ValidatorIndex: &wrapperspb.UInt64Value{Value: uint64(e.data.Message.ValidatorIndex)},
				},
				Signature: fmt.Sprintf("%#x", e.data.Signature),
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	//TODO(@matty): Populate additional data.
	extra := &xatu.ClientMeta_AdditionalEthV1EventsVoluntaryExitV2Data{}
	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsVoluntaryExitV2{
		EthV1EventsVoluntaryExitV2: extra,
	}

	return decorated
}
