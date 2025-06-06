package v1

import (
	"context"
	"fmt"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// BlockGossipEvent represents a beacon chain block gossip event.
type BlockGossipEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.BlockGossipEvent
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewBlockGossipEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.BlockGossipEvent,
	recvTime time.Time,
) *BlockGossipEvent {
	return &BlockGossipEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
		cache:     cache,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_GOSSIP.String()),
	}
}

func (e *BlockGossipEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_GOSSIP.String()
}

func (e *BlockGossipEvent) Data() interface{} {
	return e.data
}

func (e *BlockGossipEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_GOSSIP,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsBlockGossip{
			EthV1EventsBlockGossip: &xatuethv1.EventBlockGossip{
				Slot:  &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
				Block: xatuethv1.RootAsString(e.data.Block),
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

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsBlockGossip{
		EthV1EventsBlockGossip: &xatu.ClientMeta_AdditionalEthV1EventsBlockGossipData{
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

func (e *BlockGossipEvent) Ignore(ctx context.Context) (bool, error) {
	if err := e.beacon.Synced(ctx); err != nil {
		return true, err
	}

	// Check if event is from an unexpected network based on slot
	if e.beacon.IsSlotFromUnexpectedNetwork(uint64(e.data.Slot)) {
		e.log.WithField("slot", e.data.Slot).Warn("Ignoring block gossip event from unexpected network")

		return true, nil
	}

	hash, err := hashstructure.Hash(e.data, hashstructure.FormatV2, nil)
	if err != nil {
		return true, err
	}

	item, retrieved := e.cache.GetOrSet(fmt.Sprint(hash), e.recvTime, ttlcache.WithTTL[string, time.Time](ttlcache.DefaultTTL))
	if retrieved {
		e.log.WithFields(logrus.Fields{
			"hash":                  hash,
			"time_since_first_item": time.Since(item.Value()),
			"slot":                  e.data.Slot,
		}).Debug("Duplicate block gossip event received")

		return true, nil
	}

	return false, nil
}
