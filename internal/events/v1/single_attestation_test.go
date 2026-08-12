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
		now            = time.Now()
		slot           = uint64(123)
		committeeIndex = phase0.CommitteeIndex(10) // subnetID = 10 % 64 = 10
		mockBeacon     = mock.NewMockBeaconDataProvider(ctrl)
		cache          = ttlcache.New[string, time.Time]()
		attestation    = newSingleAttestation(slot, committeeIndex)
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

	t.Run("inactive subnet", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false)
		mockBeacon.EXPECT().RecordSeenSubnet(uint64(10), slot)
		mockBeacon.EXPECT().IsActiveSubnet(uint64(10)).Return(false)

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
		mockBeacon.EXPECT().RecordSeenSubnet(uint64(10), slot).Times(2)
		mockBeacon.EXPECT().IsActiveSubnet(uint64(10)).Return(true).Times(2)

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

	t.Run("rollback allows re-delivery", func(t *testing.T) {
		rollbackSlot := slot + 1
		rollbackAttestation := newSingleAttestation(rollbackSlot, committeeIndex)

		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(rollbackSlot).Return(false).Times(2)
		mockBeacon.EXPECT().RecordSeenSubnet(uint64(10), rollbackSlot).Times(2)
		mockBeacon.EXPECT().IsActiveSubnet(uint64(10)).Return(true).Times(2)

		first := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, rollbackAttestation, now,
		)

		ignore, err := first.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore, "first delivery must be processed")

		first.Rollback()

		second := NewSingleAttestationEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, rollbackAttestation, now,
		)

		ignore, err = second.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore,
			"after Rollback, a re-delivery of the same event must not be treated as a duplicate")
	})
}
