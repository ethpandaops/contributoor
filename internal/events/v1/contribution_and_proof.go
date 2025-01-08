package v1

import (
	"time"

	"github.com/attestantio/go-eth2-client/spec/altair"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ContributionAndProofEvent represents a sync committee contribution and proof event.
type ContributionAndProofEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *altair.SignedContributionAndProof
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewContributionAndProofEvent(log logrus.FieldLogger, beacon events.BeaconDataProvider, meta *xatu.Meta, data *altair.SignedContributionAndProof, recvTime time.Time) *ContributionAndProofEvent {
	return &ContributionAndProofEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_CONTRIBUTION_AND_PROOF_V2.String()),
	}
}

func (e *ContributionAndProofEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_CONTRIBUTION_AND_PROOF_V2.String()
}

func (e *ContributionAndProofEvent) Data() interface{} {
	return e.data
}

func (e *ContributionAndProofEvent) Decorated() *xatu.DecoratedEvent {
	//TODO(@matty): Populate event data.
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_CONTRIBUTION_AND_PROOF_V2,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsContributionAndProofV2{
			EthV1EventsContributionAndProofV2: &xatuethv1.EventContributionAndProofV2{
				Signature: xatuethv1.TrimmedString(xatuethv1.BLSSignatureToString(&e.data.Signature)),
				Message: &xatuethv1.ContributionAndProofV2{
					AggregatorIndex: &wrapperspb.UInt64Value{Value: uint64(e.data.Message.AggregatorIndex)},
					SelectionProof:  xatuethv1.TrimmedString(xatuethv1.BLSSignatureToString(&e.data.Message.SelectionProof)),
					Contribution: &xatuethv1.SyncCommitteeContributionV2{
						Slot:              &wrapperspb.UInt64Value{Value: uint64(e.data.Message.Contribution.Slot)},
						SubcommitteeIndex: &wrapperspb.UInt64Value{Value: e.data.Message.Contribution.SubcommitteeIndex},
						AggregationBits:   xatuethv1.BytesToString(e.data.Message.Contribution.AggregationBits.Bytes()),
						Signature:         xatuethv1.TrimmedString(xatuethv1.BLSSignatureToString(&e.data.Message.Contribution.Signature)),
						BeaconBlockRoot:   xatuethv1.RootAsString(e.data.Message.Contribution.BeaconBlockRoot),
					},
				},
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		slot  = e.beacon.GetSlot(uint64(e.data.Message.Contribution.Slot))
		epoch = e.beacon.GetEpochFromSlot(uint64(e.data.Message.Contribution.Slot))
	)

	extra := &xatu.ClientMeta_AdditionalEthV1EventsContributionAndProofV2Data{
		Contribution: &xatu.ClientMeta_AdditionalEthV1EventsContributionAndProofContributionV2Data{
			Slot: &xatu.SlotV2{
				Number:        &wrapperspb.UInt64Value{Value: slot.Number()},
				StartDateTime: timestamppb.New(slot.TimeWindow().Start()),
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

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsContributionAndProofV2{
		EthV1EventsContributionAndProofV2: extra,
	}

	return decorated
}
