package v1

import (
	"context"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	eth2v1 "github.com/ethpandaops/go-eth2-client/api/v1"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// HeadV2Event represents a head_v2 SSE event (Gloas). It is recorded as
// xatu's BEACON_API_ETH_V1_EVENTS_HEAD_V3, xatu's naming for the head_v2 topic.
type HeadV2Event struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.HeadEventV2
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewHeadV2Event(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.HeadEventV2,
	recvTime time.Time,
) *HeadV2Event {
	return &HeadV2Event{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V3.String()),
	}
}

func (e *HeadV2Event) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V3.String()
}

func (e *HeadV2Event) Data() any {
	return e.data
}

func (e *HeadV2Event) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V3,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsHeadV3{
			EthV1EventsHeadV3: &xatuethv1.EventHeadV3{
				Slot:                      &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
				Block:                     xatuethv1.RootAsString(e.data.Block),
				State:                     xatuethv1.RootAsString(e.data.State),
				PayloadStatus:             e.data.PayloadStatus,
				EpochTransition:           e.data.EpochTransition,
				CurrentEpochDependentRoot: xatuethv1.RootAsString(e.data.CurrentEpochDependentRoot),
				NextEpochDependentRoot:    xatuethv1.RootAsString(e.data.NextEpochDependentRoot),
				ExecutionOptimistic:       e.data.ExecutionOptimistic,
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Slot), e.recvTime)

	// The xatu sentry omits the slot number on head_v2 metadata (it's on the
	// event itself); mirror that.
	slot.Number = nil

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsHeadV3{
		EthV1EventsHeadV3: &xatu.ClientMeta_AdditionalEthV1EventsHeadV3Data{
			Slot:        slot,
			Epoch:       epoch,
			Propagation: propagation,
		},
	}

	return decorated
}

func (e *HeadV2Event) Ignore(ctx context.Context) (bool, error) {
	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Slot), e.data, e.recvTime)
}
