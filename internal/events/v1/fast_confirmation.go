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

// FastConfirmationEvent represents a fast_confirmation SSE event.
type FastConfirmationEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.FastConfirmationEvent
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

// fastConfirmationKey is the duplicate key for fast_confirmation. The event
// fires on every run of the algorithm, so current_slot is left out: only the
// first run that confirms a block is forwarded, and its current_slot gives the
// confirmation delay. This matches the xatu sentry.
type fastConfirmationKey struct {
	Slot  uint64
	Block string
}

func NewFastConfirmationEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.FastConfirmationEvent,
	recvTime time.Time,
) *FastConfirmationEvent {
	return &FastConfirmationEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_FAST_CONFIRMATION.String()),
	}
}

func (e *FastConfirmationEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_FAST_CONFIRMATION.String()
}

func (e *FastConfirmationEvent) Data() any {
	return e.data
}

func (e *FastConfirmationEvent) Decorated() *xatu.DecoratedEvent {
	payload := &xatuethv1.EventFastConfirmation{
		Slot:  &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
		Block: xatuethv1.RootAsString(e.data.Block),
	}

	// current_slot is only sent by beacon nodes that implement beacon-APIs #616.
	if e.data.CurrentSlot != nil {
		payload.CurrentSlot = &wrapperspb.UInt64Value{Value: uint64(*e.data.CurrentSlot)}
	}

	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_FAST_CONFIRMATION,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsFastConfirmation{
			EthV1EventsFastConfirmation: payload,
		},
	}

	if e.beacon == nil {
		return decorated
	}

	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Slot), e.recvTime)

	additional := &xatu.ClientMeta_AdditionalEthV1EventsFastConfirmationData{
		Slot:        slot,
		Epoch:       epoch,
		Propagation: propagation,
	}

	// Mirror the xatu sentry, which also records the wall clock at receipt.
	if wallclockSlot, wallclockEpoch, err := e.beacon.GetWallclock().FromTime(e.recvTime); err == nil {
		additional.WallclockSlot = &xatu.SlotV2{
			Number:        &wrapperspb.UInt64Value{Value: wallclockSlot.Number()},
			StartDateTime: timestamppb.New(wallclockSlot.TimeWindow().Start()),
		}
		additional.WallclockEpoch = &xatu.EpochV2{
			Number:        &wrapperspb.UInt64Value{Value: wallclockEpoch.Number()},
			StartDateTime: timestamppb.New(wallclockEpoch.TimeWindow().Start()),
		}
	}

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsFastConfirmation{
		EthV1EventsFastConfirmation: additional,
	}

	return decorated
}

func (e *FastConfirmationEvent) Ignore(ctx context.Context) (bool, error) {
	key := fastConfirmationKey{
		Slot:  uint64(e.data.Slot),
		Block: xatuethv1.RootAsString(e.data.Block),
	}

	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Slot), key, e.recvTime)
}
