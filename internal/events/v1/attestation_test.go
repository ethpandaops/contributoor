package v1

import (
	"testing"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAttestationEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now         = time.Now()
		slot        = uint64(123)
		sourceEpoch = uint64(2)
		targetEpoch = uint64(3)
		mockBeacon  = mock.NewMockBeaconDataProvider(ctrl)
		mockSlot    = ethwallclock.NewSlot(slot, now.Add(-10*time.Second), now)
		mockEpoch   = ethwallclock.NewEpoch(targetEpoch, now.Add(-5*time.Minute), now)
		mockSource  = ethwallclock.NewEpoch(sourceEpoch, now.Add(-10*time.Minute), now)
		mockTarget  = ethwallclock.NewEpoch(targetEpoch, now.Add(-5*time.Minute), now)
		blockRoot   = phase0.Root{0x1}                     // Simple root for testing
		sourceRoot  = phase0.Root{0x2}                     // Simple root for testing
		targetRoot  = phase0.Root{0x3}                     // Simple root for testing
		signature   = phase0.BLSSignature{}                // Zero signature
		aggrBits    = bitfield.Bitlist([]byte{0x01, 0x01}) // Single bit set
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)
	mockBeacon.EXPECT().GetEpoch(targetEpoch).Return(mockTarget)
	mockBeacon.EXPECT().GetEpochFromSlot(sourceEpoch).Return(mockSource)
	mockBeacon.EXPECT().GetValidatorIndex(
		phase0.Epoch(targetEpoch),
		phase0.Slot(slot),
		phase0.CommitteeIndex(1),
		uint64(0),
	).Return(phase0.ValidatorIndex(789), nil)

	event := NewAttestationEvent(
		logrus.New(),
		mockBeacon,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&phase0.Attestation{
			AggregationBits: aggrBits,
			Data: &phase0.AttestationData{
				Slot:            phase0.Slot(slot),
				Index:           phase0.CommitteeIndex(1),
				BeaconBlockRoot: blockRoot,
				Source: &phase0.Checkpoint{
					Epoch: phase0.Epoch(sourceEpoch),
					Root:  sourceRoot,
				},
				Target: &phase0.Checkpoint{
					Epoch: phase0.Epoch(targetEpoch),
					Root:  targetRoot,
				},
			},
			Signature: signature,
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsAttestationV2()
		additionalData = decorated.GetEthV1EventsAttestationV2()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, "0x0101", additionalData.AggregationBits)
	require.Equal(t, xatuethv1.TrimmedString(signature.String()), additionalData.Signature)

	// Assert attestation data.
	data := additionalData.Data
	require.NotNil(t, data)
	require.Equal(t, slot, data.Slot.Value)
	require.Equal(t, uint64(1), data.Index.Value)
	require.Equal(t, blockRoot.String(), data.BeaconBlockRoot)

	// Assert source checkpoint.
	source := data.Source
	require.NotNil(t, source)
	require.Equal(t, sourceEpoch, source.Epoch.Value)
	require.Equal(t, sourceRoot.String(), source.Root)

	// Assert target checkpoint.
	target := data.Target
	require.NotNil(t, target)
	require.Equal(t, targetEpoch, target.Epoch.Value)
	require.Equal(t, targetRoot.String(), target.Root)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, slot, metaData.Slot.Number.Value)
	require.Equal(t, targetEpoch, metaData.Epoch.Number.Value)

	// Assert source metadata.
	require.NotNil(t, metaData.Source)
	require.Equal(t, sourceEpoch, metaData.Source.Epoch.Number.Value)

	// Assert target metadata.
	require.NotNil(t, metaData.Target)
	require.Equal(t, targetEpoch, metaData.Target.Epoch.Number.Value)

	// Assert attesting validator (since we have a single bit set).
	require.NotNil(t, metaData.AttestingValidator)
	require.Equal(t, uint64(0), metaData.AttestingValidator.CommitteeIndex.Value)
	require.Equal(t, uint64(789), metaData.AttestingValidator.Index.Value)
}
