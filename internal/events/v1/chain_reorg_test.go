package v1

import (
	"testing"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestChainReorgEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		mockSlot   = ethwallclock.NewSlot(slot, now.Add(-10*time.Second), now)
		mockEpoch  = ethwallclock.NewEpoch(3, now.Add(-5*time.Minute), now)
		oldRoot    = phase0.Root{0x1} // Simple root for testing
		newRoot    = phase0.Root{0x2} // Simple root for testing
		depth      = uint64(2)
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewChainReorgEvent(
		logrus.New(),
		mockBeacon,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&eth2v1.ChainReorgEvent{
			Slot:         phase0.Slot(slot),
			Depth:        depth,
			OldHeadBlock: oldRoot,
			NewHeadBlock: newRoot,
			OldHeadState: oldRoot,
			NewHeadState: newRoot,
			Epoch:        phase0.Epoch(3),
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsChainReorgV2()
		additionalData = decorated.GetEthV1EventsChainReorgV2()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_CHAIN_REORG_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, slot, additionalData.Slot.Value)
	require.Equal(t, depth, additionalData.Depth.Value)
	require.Equal(t, oldRoot.String(), additionalData.OldHeadBlock)
	require.Equal(t, newRoot.String(), additionalData.NewHeadBlock)
	require.Equal(t, oldRoot.String(), additionalData.OldHeadState)
	require.Equal(t, newRoot.String(), additionalData.NewHeadState)
	require.Equal(t, uint64(3), additionalData.Epoch.Value)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, slot, metaData.Slot.Number.Value)
	require.Equal(t, uint64(3), metaData.Epoch.Number.Value)
}
