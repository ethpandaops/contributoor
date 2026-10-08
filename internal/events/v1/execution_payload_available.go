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
)

// ExecutionPayloadAvailableEvent represents an execution_payload_available SSE
// event (Gloas, EIP-7732).
type ExecutionPayloadAvailableEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.ExecutionPayloadAvailableEvent
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewExecutionPayloadAvailableEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.ExecutionPayloadAvailableEvent,
	recvTime time.Time,
) *ExecutionPayloadAvailableEvent {
	return &ExecutionPayloadAvailableEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_AVAILABLE.String()),
	}
}

func (e *ExecutionPayloadAvailableEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_AVAILABLE.String()
}

func (e *ExecutionPayloadAvailableEvent) Data() any {
	return e.data
}

func (e *ExecutionPayloadAvailableEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_AVAILABLE,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsExecutionPayloadAvailable{
			EthV1EventsExecutionPayloadAvailable: xatuethv1.NewExecutionPayloadAvailableFromAPIV1(e.data),
		},
	}

	if e.beacon == nil {
		return decorated
	}

	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Slot), e.recvTime)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsExecutionPayloadAvailable{
		EthV1EventsExecutionPayloadAvailable: &xatu.ClientMeta_AdditionalEthV1EventsExecutionPayloadAvailableData{
			Slot:        slot,
			Epoch:       epoch,
			Propagation: propagation,
		},
	}

	return decorated
}

func (e *ExecutionPayloadAvailableEvent) Ignore(ctx context.Context) (bool, error) {
	if e.data == nil {
		return true, nil
	}

	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Slot), e.data, e.recvTime)
}
