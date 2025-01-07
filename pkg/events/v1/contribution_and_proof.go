package v1

import (
	"time"

	"github.com/attestantio/go-eth2-client/spec/altair"
	"github.com/ethpandaops/contributoor/pkg/events"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ContributionAndProofEvent represents a sync committee contribution and proof event.
type ContributionAndProofEvent struct {
	events.BaseEvent
	data     *altair.SignedContributionAndProof
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewContributionAndProofEvent(beacon events.BeaconDataProvider, meta *xatu.Meta, data *altair.SignedContributionAndProof, recvTime time.Time) *ContributionAndProofEvent {
	return &ContributionAndProofEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
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
		Data: &xatu.DecoratedEvent_EthV1EventsContributionAndProofV2{},
	}

	if e.beacon == nil {
		return decorated
	}

	//TODO(@matty): Populate additional data.
	extra := &xatu.ClientMeta_AdditionalEthV1EventsContributionAndProofV2Data{}
	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsContributionAndProofV2{
		EthV1EventsContributionAndProofV2: extra,
	}

	return decorated
}
