package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/deneb"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBlobSidecarEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now           = time.Now()
		slot          = uint64(123)
		mockBeacon    = mock.NewMockBeaconDataProvider(ctrl)
		mockSlot      = ethwallclock.NewSlot(slot, now.Add(-10*time.Second), now)
		mockEpoch     = ethwallclock.NewEpoch(3, now.Add(-5*time.Minute), now)
		blockRoot     = phase0.Root{0x1}      // Simple root for testing
		kzgCommitment = deneb.KZGCommitment{} // Zero commitment
		versionedHash = deneb.VersionedHash{} // Zero hash
		index         = deneb.BlobIndex(1)
		cache         = ttlcache.New[string, time.Time]() // Create a new cache for testing
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewBlobSidecarEvent(
		logrus.New(),
		mockBeacon,
		cache,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&eth2v1.BlobSidecarEvent{
			Slot:          phase0.Slot(slot),
			BlockRoot:     blockRoot,
			Index:         index,
			KZGCommitment: kzgCommitment,
			VersionedHash: versionedHash,
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsBlobSidecar()
		additionalData = decorated.GetEthV1EventsBlobSidecar()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOB_SIDECAR.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, slot, additionalData.Slot.Value)
	require.Equal(t, uint64(1), additionalData.Index.Value)
	require.Equal(t, blockRoot.String(), additionalData.BlockRoot)
	require.Equal(t, xatuethv1.KzgCommitmentToString(kzgCommitment), additionalData.KzgCommitment)
	require.Equal(t, xatuethv1.VersionedHashToString(versionedHash), additionalData.VersionedHash)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, slot, metaData.Slot.Number.Value)
	require.Equal(t, uint64(3), metaData.Epoch.Number.Value)
}

func TestBlobSidecarEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now           = time.Now()
		slot          = uint64(123)
		mockBeacon    = mock.NewMockBeaconDataProvider(ctrl)
		blockRoot     = phase0.Root{0x1}
		kzgCommitment = deneb.KZGCommitment{}
		versionedHash = deneb.VersionedHash{}
		index         = deneb.BlobIndex(1)
		cache         = ttlcache.New[string, time.Time]()
	)

	blobSidecar := &eth2v1.BlobSidecarEvent{
		Slot:          phase0.Slot(slot),
		BlockRoot:     blockRoot,
		Index:         index,
		KZGCommitment: kzgCommitment,
		VersionedHash: versionedHash,
	}

	t.Run("cache miss", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewBlobSidecarEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			blobSidecar,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(true)

		event := NewBlobSidecarEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			blobSidecar,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false).Times(2)

		event := NewBlobSidecarEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			blobSidecar,
			now,
		)

		// First call should not be ignored
		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore)

		// Second call with same data should be ignored
		ignore, err = event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})
}
