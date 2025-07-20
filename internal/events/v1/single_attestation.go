package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/attestantio/go-eth2-client/spec/electra"
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

// SingleAttestationEvent represents a single attestation event.
type SingleAttestationEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *electra.SingleAttestation
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewSingleAttestationEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *electra.SingleAttestation,
	recvTime time.Time,
) *SingleAttestationEvent {
	return &SingleAttestationEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String()),
	}
}

func (e *SingleAttestationEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String()
}

func (e *SingleAttestationEvent) Data() interface{} {
	return e.data
}

func (e *SingleAttestationEvent) Decorated() *xatu.DecoratedEvent {
	var (
		singleAttest     = e.data
		attestData       = singleAttest.Data
		targetCheckpoint = attestData.Target
		sourceCheckpoint = attestData.Source
	)

	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsAttestationV2{
			EthV1EventsAttestationV2: &xatuethv1.AttestationV2{
				AggregationBits: "",
				Data: &xatuethv1.AttestationDataV2{
					Slot:            &wrapperspb.UInt64Value{Value: uint64(attestData.Slot)},
					Index:           &wrapperspb.UInt64Value{Value: uint64(attestData.Index)},
					BeaconBlockRoot: xatuethv1.RootAsString(attestData.BeaconBlockRoot),
					Source: &xatuethv1.CheckpointV2{
						Epoch: &wrapperspb.UInt64Value{Value: uint64(attestData.Source.Epoch)},
						Root:  xatuethv1.RootAsString(attestData.Source.Root),
					},
					Target: &xatuethv1.CheckpointV2{
						Epoch: &wrapperspb.UInt64Value{Value: uint64(attestData.Target.Epoch)},
						Root:  xatuethv1.RootAsString(attestData.Target.Root),
					},
				},
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		attestingSlot = e.beacon.GetWallclock().Slots().FromNumber(uint64(attestData.Slot))
		epoch         = e.beacon.GetWallclock().Epochs().FromSlot(uint64(attestData.Slot))
		targetEpoch   = e.beacon.GetWallclock().Epochs().FromNumber(uint64(targetCheckpoint.Epoch))
		sourceEpoch   = e.beacon.GetWallclock().Epochs().FromNumber(uint64(sourceCheckpoint.Epoch))
	)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsAttestationV2{
		EthV1EventsAttestationV2: &xatu.ClientMeta_AdditionalEthV1EventsAttestationV2Data{
			Slot: &xatu.SlotV2{
				Number:        &wrapperspb.UInt64Value{Value: attestingSlot.Number()},
				StartDateTime: timestamppb.New(attestingSlot.TimeWindow().Start()),
			},
			Epoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
			Propagation: &xatu.PropagationV2{
				SlotStartDiff: &wrapperspb.UInt64Value{
					Value: uint64(e.recvTime.Sub(attestingSlot.TimeWindow().Start()).Milliseconds()), //nolint:gosec // conversion ok.
				},
			},
			Target: &xatu.ClientMeta_AdditionalEthV1AttestationTargetV2Data{
				Epoch: &xatu.EpochV2{
					Number:        &wrapperspb.UInt64Value{Value: targetEpoch.Number()},
					StartDateTime: timestamppb.New(targetEpoch.TimeWindow().Start()),
				},
			},
			Source: &xatu.ClientMeta_AdditionalEthV1AttestationSourceV2Data{
				Epoch: &xatu.EpochV2{
					Number:        &wrapperspb.UInt64Value{Value: sourceEpoch.Number()},
					StartDateTime: timestamppb.New(sourceEpoch.TimeWindow().Start()),
				},
			},
			AttestingValidator: &xatu.AttestingValidatorV2{
				CommitteeIndex: &wrapperspb.UInt64Value{Value: uint64(singleAttest.CommitteeIndex)},
				Index:          &wrapperspb.UInt64Value{Value: uint64(singleAttest.AttesterIndex)},
			},
		},
	}

	return decorated
}

func (e *SingleAttestationEvent) Ignore(ctx context.Context) (bool, error) {
	if err := e.beacon.Synced(ctx); err != nil {
		return true, err
	}

	attestData := e.data.Data

	// Check if event is from an unexpected network based on slot
	if e.beacon.IsSlotFromUnexpectedNetwork(uint64(attestData.Slot)) {
		e.log.WithField("slot", attestData.Slot).Warn("Ignoring single attestation event from unexpected network")

		return true, nil
	}

	// Filter by subnet ID - committee index modulo 64 gives us the subnet ID
	subnetID := uint64(e.data.CommitteeIndex) % 64
	if !e.beacon.IsActiveSubnet(subnetID) {
		e.log.WithFields(logrus.Fields{
			"committee_index": e.data.CommitteeIndex,
			"subnet_id":       subnetID,
		}).Debug("Ignoring attestation from inactive subnet")

		return true, nil
	} else {
		e.log.WithFields(logrus.Fields{
			"committee_index": e.data.CommitteeIndex,
			"subnet_id":       subnetID,
		}).Info("Decorating attestation from active subnet")
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
			"slot":                  attestData.Slot,
			"index":                 attestData.Index,
		}).Debug("Duplicate single attestation event received")

		return true, nil
	}

	return false, nil
}
