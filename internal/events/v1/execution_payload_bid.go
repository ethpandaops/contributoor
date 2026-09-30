package v1

import (
	"context"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/go-eth2-client/spec/gloas"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ExecutionPayloadBidEvent represents an execution_payload_bid SSE event
// (Gloas, EIP-7732).
type ExecutionPayloadBidEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *gloas.SignedExecutionPayloadBid
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewExecutionPayloadBidEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *gloas.SignedExecutionPayloadBid,
	recvTime time.Time,
) *ExecutionPayloadBidEvent {
	return &ExecutionPayloadBidEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_BID.String()),
	}
}

func (e *ExecutionPayloadBidEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_BID.String()
}

func (e *ExecutionPayloadBidEvent) Data() any {
	return e.data
}

func (e *ExecutionPayloadBidEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_BID,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsExecutionPayloadBid{
			EthV1EventsExecutionPayloadBid: xatuethv1.NewSignedExecutionPayloadBidFromGloas(e.data),
		},
	}

	if e.beacon == nil || e.data.Message == nil {
		return decorated
	}

	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Message.Slot), e.recvTime)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsExecutionPayloadBid{
		EthV1EventsExecutionPayloadBid: &xatu.ClientMeta_AdditionalEthV1EventsExecutionPayloadBidData{
			Slot:        slot,
			Epoch:       epoch,
			Propagation: propagation,
		},
	}

	return decorated
}

func (e *ExecutionPayloadBidEvent) Ignore(ctx context.Context) (bool, error) {
	if e.data == nil || e.data.Message == nil {
		return true, nil
	}

	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Message.Slot), e.data, e.recvTime)
}
