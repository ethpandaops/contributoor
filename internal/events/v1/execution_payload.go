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

// ExecutionPayloadEvent represents an execution_payload or
// execution_payload_gossip SSE event (Gloas, EIP-7732). Both topics carry the
// same payload; gossip fires when the envelope passes gossip validation,
// execution_payload once it has been imported.
type ExecutionPayloadEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.ExecutionPayloadEvent
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
	gossip   bool
}

// NewExecutionPayloadEvent creates an event for the execution_payload topic.
func NewExecutionPayloadEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.ExecutionPayloadEvent,
	recvTime time.Time,
) *ExecutionPayloadEvent {
	return newExecutionPayloadEvent(log, beacon, cache, meta, data, recvTime, false)
}

// NewExecutionPayloadGossipEvent creates an event for the execution_payload_gossip topic.
func NewExecutionPayloadGossipEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.ExecutionPayloadEvent,
	recvTime time.Time,
) *ExecutionPayloadEvent {
	return newExecutionPayloadEvent(log, beacon, cache, meta, data, recvTime, true)
}

func newExecutionPayloadEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.ExecutionPayloadEvent,
	recvTime time.Time,
	gossip bool,
) *ExecutionPayloadEvent {
	e := &ExecutionPayloadEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		gossip:    gossip,
	}

	e.log = log.WithField("event", e.Type())

	return e
}

func (e *ExecutionPayloadEvent) name() xatu.Event_Name {
	if e.gossip {
		return xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_GOSSIP
	}

	return xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD
}

func (e *ExecutionPayloadEvent) Type() string {
	return e.name().String()
}

func (e *ExecutionPayloadEvent) Data() any {
	return e.data
}

func (e *ExecutionPayloadEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     e.name(),
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
	}

	payload := xatuethv1.NewExecutionPayloadEventFromAPIV1(e.data)

	if e.gossip {
		decorated.Data = &xatu.DecoratedEvent_EthV1EventsExecutionPayloadGossip{
			EthV1EventsExecutionPayloadGossip: payload,
		}
	} else {
		decorated.Data = &xatu.DecoratedEvent_EthV1EventsExecutionPayload{
			EthV1EventsExecutionPayload: payload,
		}
	}

	if e.beacon == nil {
		return decorated
	}

	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Slot), e.recvTime)

	if e.gossip {
		decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsExecutionPayloadGossip{
			EthV1EventsExecutionPayloadGossip: &xatu.ClientMeta_AdditionalEthV1EventsExecutionPayloadGossipData{
				Slot:        slot,
				Epoch:       epoch,
				Propagation: propagation,
			},
		}
	} else {
		decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsExecutionPayload{
			EthV1EventsExecutionPayload: &xatu.ClientMeta_AdditionalEthV1EventsExecutionPayloadData{
				Slot:        slot,
				Epoch:       epoch,
				Propagation: propagation,
			},
		}
	}

	return decorated
}

func (e *ExecutionPayloadEvent) Ignore(ctx context.Context) (bool, error) {
	if e.data == nil {
		return true, nil
	}

	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Slot), e.data, e.recvTime)
}
