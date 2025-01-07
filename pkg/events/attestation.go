package events

import (
	"fmt"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// AttestationEvent represents a beacon chain attestation event.
type AttestationEvent struct {
	BaseEvent
	data     *phase0.Attestation
	beacon   BeaconDataProvider
	recvTime time.Time
}

func NewAttestationEvent(beacon BeaconDataProvider, meta *xatu.Meta, data *phase0.Attestation, recvTime time.Time) *AttestationEvent {
	return &AttestationEvent{
		BaseEvent: NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
	}
}

func (e *AttestationEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String()
}
func (e *AttestationEvent) Data() interface{} { return e.data }

func (e *AttestationEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsAttestationV2{
			EthV1EventsAttestationV2: &xatuethv1.AttestationV2{
				AggregationBits: xatuethv1.BytesToString(e.data.AggregationBits),
				Data: &xatuethv1.AttestationDataV2{
					Slot:            &wrapperspb.UInt64Value{Value: uint64(e.data.Data.Slot)},
					Index:           &wrapperspb.UInt64Value{Value: uint64(e.data.Data.Index)},
					BeaconBlockRoot: xatuethv1.RootAsString(e.data.Data.BeaconBlockRoot),
					Source: &xatuethv1.CheckpointV2{
						Epoch: &wrapperspb.UInt64Value{Value: uint64(e.data.Data.Source.Epoch)},
						Root:  xatuethv1.RootAsString(e.data.Data.Source.Root),
					},
					Target: &xatuethv1.CheckpointV2{
						Epoch: &wrapperspb.UInt64Value{Value: uint64(e.data.Data.Target.Epoch)},
						Root:  xatuethv1.RootAsString(e.data.Data.Target.Root),
					},
				},
				Signature: xatuethv1.TrimmedString(fmt.Sprintf("%#x", e.data.Signature)),
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		attestionSlot = e.beacon.GetSlot(uint64(e.data.Data.Slot))
		epoch         = e.beacon.GetEpochFromSlot(uint64(e.data.Data.Slot))
		targetEpoch   = e.beacon.GetEpoch(uint64(e.data.Data.Target.Epoch))
		sourceEpoch   = e.beacon.GetEpochFromSlot(uint64(e.data.Data.Source.Epoch))
	)

	extra := &xatu.ClientMeta_AdditionalEthV1EventsAttestationV2Data{
		Slot: &xatu.SlotV2{
			Number:        &wrapperspb.UInt64Value{Value: uint64(e.data.Data.Slot)},
			StartDateTime: timestamppb.New(attestionSlot.TimeWindow().Start()),
		},
		Epoch: &xatu.EpochV2{
			Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
			StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
		},
		Propagation: &xatu.PropagationV2{
			SlotStartDiff: &wrapperspb.UInt64Value{
				//nolint:gosec // not concerned in reality.
				Value: uint64(e.recvTime.Sub(attestionSlot.TimeWindow().Start()).Milliseconds()),
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
	}

	// If the attestation is unaggreated, we can append the validator position within the committee.
	if e.data.AggregationBits.Count() == 1 {
		//nolint:gosec // not concerned in reality.
		position := uint64(e.data.AggregationBits.BitIndices()[0])

		validatorIndex, err := e.beacon.GetValidatorIndex(
			phase0.Epoch(epoch.Number()),
			e.data.Data.Slot,
			e.data.Data.Index,
			position,
		)
		if err == nil {
			extra.AttestingValidator = &xatu.AttestingValidatorV2{
				CommitteeIndex: &wrapperspb.UInt64Value{Value: position},
				Index:          &wrapperspb.UInt64Value{Value: uint64(validatorIndex)},
			}
		}
	}

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsAttestationV2{
		EthV1EventsAttestationV2: extra,
	}

	return decorated
}
