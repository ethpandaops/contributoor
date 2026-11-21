package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/go-eth2-client/spec/electra"
	"github.com/ethpandaops/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newSingleAttestation(slot uint64, committeeIndex phase0.CommitteeIndex) *electra.SingleAttestation {
	return &electra.SingleAttestation{
		CommitteeIndex: committeeIndex,
		AttesterIndex:  1,
		Data: &phase0.AttestationData{
			Slot:            phase0.Slot(slot),
			Index:           0,
			BeaconBlockRoot: phase0.Root{0xaa},
			Source:          &phase0.Checkpoint{Epoch: 1, Root: phase0.Root{0xbb}},
			Target:          &phase0.Checkpoint{Epoch: 2, Root: phase0.Root{0xcc}},
		},
	}
}

func TestSingleAttestationEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now         = time.Now()
		slot        = uint64(123)
		cache       = ttlcache.New[string, time.Time]()
		mockBeacon  = mock.NewMockBeaconDataProvider(ctrl)
		attestation = newSingleAttestation(slot, 10)
	)

	t.Run("cache miss (not synced)", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, attestation, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(true)

		event := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, attestation, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("dropped when committees-per-slot is not yet known", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false)
		mockBeacon.EXPECT().GetAttestationSubnetID(slot, uint64(10)).Return(uint64(0), false)

		event := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, attestation, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore,
			"an event whose subnet can't be computed yet must be dropped, not exported with a "+
				"guessed or wrong subnet")
	})

	t.Run("uses the real spec formula's result, not committee_index%64", func(t *testing.T) {
		// committees_per_slot=45, slot=100, committee_index=10 - the exact vector used to
		// PoC-confirm this bug during the nemesis triage. Spec formula gives 62; the old
		// committee_index%64 code gave 10.
		vectorSlot := uint64(100)
		vectorAttestation := newSingleAttestation(vectorSlot, 10)

		const specCorrectSubnet = uint64(62)

		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(vectorSlot).Return(false)
		mockBeacon.EXPECT().GetAttestationSubnetID(vectorSlot, uint64(10)).Return(specCorrectSubnet, true)

		var capturedSubnetID uint64

		mockBeacon.EXPECT().
			RecordSeenSubnet(gomock.Any(), vectorSlot).
			DoAndReturn(func(subnetID uint64, _ uint64) { capturedSubnetID = subnetID })
		mockBeacon.EXPECT().IsActiveSubnet(specCorrectSubnet).Return(true)

		event := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, vectorAttestation, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore)
		require.Equal(t, specCorrectSubnet, capturedSubnetID,
			"the subnet passed to RecordSeenSubnet/IsActiveSubnet must be GetAttestationSubnetID's "+
				"result (62), not the old committee_index%%64 value (10)")
	})

	t.Run("inactive subnet", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false)
		mockBeacon.EXPECT().GetAttestationSubnetID(slot, uint64(10)).Return(uint64(5), true)
		mockBeacon.EXPECT().RecordSeenSubnet(uint64(5), slot)
		mockBeacon.EXPECT().IsActiveSubnet(uint64(5)).Return(false)

		event := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, attestation, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false).Times(2)
		mockBeacon.EXPECT().GetAttestationSubnetID(slot, uint64(10)).Return(uint64(7), true).Times(2)
		mockBeacon.EXPECT().RecordSeenSubnet(uint64(7), slot).Times(2)
		mockBeacon.EXPECT().IsActiveSubnet(uint64(7)).Return(true).Times(2)

		event := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, attestation, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore)

		ignore, err = event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})
}
