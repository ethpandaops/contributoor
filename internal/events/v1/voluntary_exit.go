package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
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

// VoluntaryExitEvent represents a validator voluntary exit event.
type VoluntaryExitEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *phase0.SignedVoluntaryExit
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewVoluntaryExitEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *phase0.SignedVoluntaryExit,
	recvTime time.Time,
) *VoluntaryExitEvent {
	return &VoluntaryExitEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_VOLUNTARY_EXIT_V2.String()),
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
				Signature: e.data.Signature.String(),
				Message: &xatuethv1.EventVoluntaryExitMessageV2{
					Epoch:          &wrapperspb.UInt64Value{Value: uint64(e.data.Message.Epoch)},
					ValidatorIndex: &wrapperspb.UInt64Value{Value: uint64(e.data.Message.ValidatorIndex)},
				},
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		epoch     = e.beacon.GetEpoch(uint64(e.data.Message.Epoch))
		wallclock = e.beacon.GetWallclock()
	)

	wallclockSlot, wallclockEpoch, err := wallclock.Now()
	if err != nil {
		e.log.WithError(err).Error("Failed to get extra voluntary exit data")

		return decorated
	}

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsVoluntaryExitV2{
		EthV1EventsVoluntaryExitV2: &xatu.ClientMeta_AdditionalEthV1EventsVoluntaryExitV2Data{
			Epoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
			WallclockSlot: &xatu.SlotV2{
				Number:        &wrapperspb.UInt64Value{Value: wallclockSlot.Number()},
				StartDateTime: timestamppb.New(wallclockSlot.TimeWindow().Start()),
			},
			WallclockEpoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: wallclockEpoch.Number()},
				StartDateTime: timestamppb.New(wallclockEpoch.TimeWindow().Start()),
			},
		},
	}

	return decorated
}

func (e *VoluntaryExitEvent) Ignore(ctx context.Context) (bool, error) {
	if err := e.beacon.Synced(ctx); err != nil {
		//nolint:nilerr // Returning nil is intentional.
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
			"epoch":                 e.data.Message.Epoch,
		}).Debug("Duplicate voluntary exit event received")

		return true, nil
	}

	return false, nil
}
