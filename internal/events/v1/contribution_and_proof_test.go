package v1

import (
	"testing"
	"time"

	"github.com/attestantio/go-eth2-client/spec/altair"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestContributionAndProofEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		mockSlot   = ethwallclock.NewSlot(slot, now.Add(-10*time.Second), now)
		mockEpoch  = ethwallclock.NewEpoch(3, now.Add(-5*time.Minute), now)
		signature  = phase0.BLSSignature{} // 96-byte zero signature
		blockRoot  = phase0.Root{0x1}      // Simple root for testing
		aggrBits   = []byte{0x1}           // Simple aggregation bits
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewContributionAndProofEvent(
		logrus.New(),
		mockBeacon,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&altair.SignedContributionAndProof{
			Message: &altair.ContributionAndProof{
				AggregatorIndex: 456,
				SelectionProof:  signature,
				Contribution: &altair.SyncCommitteeContribution{
					Slot:              phase0.Slot(slot),
					SubcommitteeIndex: 1,
					BeaconBlockRoot:   blockRoot,
					AggregationBits:   aggrBits,
					Signature:         signature,
				},
			},
			Signature: signature,
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsContributionAndProofV2()
		additionalData = decorated.GetEthV1EventsContributionAndProofV2()
		trimmedSig     = xatuethv1.TrimmedString(signature.String())
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_CONTRIBUTION_AND_PROOF_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, uint64(456), additionalData.Message.AggregatorIndex.Value)
	require.Equal(t, trimmedSig, additionalData.Message.SelectionProof)
	require.Equal(t, trimmedSig, additionalData.Signature)

	// Assert contribution data.
	contribution := additionalData.Message.Contribution
	require.NotNil(t, contribution)
	require.Equal(t, slot, contribution.Slot.Value)
	require.Equal(t, uint64(1), contribution.SubcommitteeIndex.Value)
	require.Equal(t, blockRoot.String(), contribution.BeaconBlockRoot)
	require.Equal(t, "0x01", contribution.AggregationBits)
	require.Equal(t, trimmedSig, contribution.Signature)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.NotNil(t, metaData.Contribution)
	require.Equal(t, slot, metaData.Contribution.Slot.Number.Value)
	require.Equal(t, uint64(3), metaData.Contribution.Epoch.Number.Value)
}
