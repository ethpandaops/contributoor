package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// slotTiming builds the slot, epoch and propagation metadata shared by the
// Gloas (EIP-7732) event types. It mirrors the xatu sentry decoration so
// rows from contributoor and xatu sentries are interchangeable.
func slotTiming(
	beacon events.BeaconDataProvider,
	slotNumber uint64,
	recvTime time.Time,
) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
	var (
		slot      = beacon.GetSlot(slotNumber)
		epoch     = beacon.GetEpochFromSlot(slotNumber)
		slotStart = slot.TimeWindow().Start()
	)

	slotMeta := &xatu.SlotV2{
		Number:        &wrapperspb.UInt64Value{Value: slotNumber},
		StartDateTime: timestamppb.New(slotStart),
	}

	epochMeta := &xatu.EpochV2{
		Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
		StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
	}

	propagation := &xatu.PropagationV2{
		SlotStartDiff: &wrapperspb.UInt64Value{
			//nolint:gosec // matches xatu sentry behaviour.
			Value: uint64(recvTime.Sub(slotStart).Milliseconds()),
		},
	}

	return slotMeta, epochMeta, propagation
}

// shouldIgnore runs the checks every event goes through before being
// forwarded: beacon node sync state, network sanity (slot distance from
// wallclock) and duplicate detection.
func shouldIgnore(
	ctx context.Context,
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	slot uint64,
	data any,
	recvTime time.Time,
) (bool, error) {
	if err := beacon.Synced(ctx); err != nil {
		return true, err
	}

	if beacon.IsSlotFromUnexpectedNetwork(slot) {
		log.WithField(logFieldSlot, slot).Warn("Ignoring event from unexpected network")

		return true, nil
	}

	hash, err := hashstructure.Hash(data, hashstructure.FormatV2, nil)
	if err != nil {
		return true, err
	}

	item, retrieved := cache.GetOrSet(fmt.Sprint(hash), recvTime, ttlcache.WithTTL[string, time.Time](ttlcache.DefaultTTL))
	if retrieved {
		log.WithFields(logrus.Fields{
			logFieldHash:               hash,
			logFieldTimeSinceFirstItem: time.Since(item.Value()),
			logFieldSlot:               slot,
		}).Debug("Duplicate event received")

		return true, nil
	}

	return false, nil
}
