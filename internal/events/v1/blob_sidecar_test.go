package v1

import (
	"testing"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/deneb"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
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
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewBlobSidecarEvent(
		logrus.New(),
		mockBeacon,
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
