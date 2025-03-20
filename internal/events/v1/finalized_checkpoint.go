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

// FinalizedCheckpointEvent represents a beacon chain finalization event.
type FinalizedCheckpointEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.FinalizedCheckpointEvent
	cache    *ttlcache.Cache[string, time.Time]
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewFinalizedCheckpointEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *eth2v1.FinalizedCheckpointEvent,
	recvTime time.Time,
) *FinalizedCheckpointEvent {
	return &FinalizedCheckpointEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
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

func (e *FinalizedCheckpointEvent) Ignore(ctx context.Context) (bool, error) {
	if err := e.beacon.Synced(ctx); err != nil {
		return true, err
	}

	// For finalized checkpoint, we need to convert epoch to slot
	// Assuming 32 slots per epoch
	epochSlot := uint64(e.data.Epoch) * 32

	// Check if event is from an unexpected network based on converted slot
	if e.beacon.IsSlotFromUnexpectedNetwork(epochSlot) {
		e.log.WithFields(logrus.Fields{
			"epoch":      e.data.Epoch,
			"epoch_slot": epochSlot,
		}).Warn("Ignoring finalized checkpoint event from unexpected network")

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
		}).Debug("Duplicate contribution and proof event received")

		return true, nil
	}

	return false, nil
}
