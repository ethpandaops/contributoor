package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/attestantio/go-eth2-client/spec"
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

// AggregateAttestationEvent represents an aggregate attestation event.
type AggregateAttestationEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *spec.VersionedAttestation
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

// NewAggregateAttestationEvent creates a new aggregate attestation event.
func NewAggregateAttestationEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *spec.VersionedAttestation,
	recvTime time.Time,
) *AggregateAttestationEvent {
	return &AggregateAttestationEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String()),
	}
}

// Type returns the event type.
func (e *AggregateAttestationEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String()
}

// Data returns the raw event data.
func (e *AggregateAttestationEvent) Data() any {
	return e.data
}

// Decorated returns the decorated event for export.
func (e *AggregateAttestationEvent) Decorated() *xatu.DecoratedEvent {
	attestData, err := e.data.Data()
	if err != nil {
		e.log.WithError(err).Error("Failed to get attestation data")

		return nil
	}

	aggregationBits, err := e.data.AggregationBits()
	if err != nil {
		e.log.WithError(err).Error("Failed to get aggregation bits")

		return nil
	}

	var (
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
				AggregationBits: xatuethv1.BytesToString(aggregationBits),
				Data: &xatuethv1.AttestationDataV2{
					Slot:            &wrapperspb.UInt64Value{Value: uint64(attestData.Slot)},
					Index:           &wrapperspb.UInt64Value{Value: uint64(attestData.Index)},
					BeaconBlockRoot: xatuethv1.RootAsString(attestData.BeaconBlockRoot),
					Source: &xatuethv1.CheckpointV2{
						Epoch: &wrapperspb.UInt64Value{Value: uint64(sourceCheckpoint.Epoch)},
						Root:  xatuethv1.RootAsString(sourceCheckpoint.Root),
					},
					Target: &xatuethv1.CheckpointV2{
						Epoch: &wrapperspb.UInt64Value{Value: uint64(targetCheckpoint.Epoch)},
						Root:  xatuethv1.RootAsString(targetCheckpoint.Root),
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
			// Note: AttestingValidator is not set for aggregate attestations
			// since they represent multiple validators' attestations combined.
		},
	}

	return decorated
}

// Ignore determines if the event should be ignored.
func (e *AggregateAttestationEvent) Ignore(ctx context.Context) (bool, error) {
	if err := e.beacon.Synced(ctx); err != nil {
		return true, err
	}

	attestData, err := e.data.Data()
	if err != nil {
		return true, fmt.Errorf("failed to get attestation data: %w", err)
	}

	// Check if event is from an unexpected network based on slot.
	if e.beacon.IsSlotFromUnexpectedNetwork(uint64(attestData.Slot)) {
		e.log.WithField("slot", attestData.Slot).Warn("Ignoring aggregate attestation event from unexpected network")

		return true, nil
	}

	// Note: No subnet filtering for aggregate attestations - they are broadcast globally.

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
		}).Debug("Duplicate aggregate attestation event received")

		return true, nil
	}

	return false, nil
}
