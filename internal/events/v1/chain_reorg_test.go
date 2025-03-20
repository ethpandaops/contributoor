package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
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
		cache      = ttlcache.New[string, time.Time]() // Create a new cache for testing
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewChainReorgEvent(
		logrus.New(),
		mockBeacon,
		cache,
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

func TestChainReorgEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		oldRoot    = phase0.Root{0x1}
		newRoot    = phase0.Root{0x2}
		cache      = ttlcache.New[string, time.Time]()
	)

	reorg := &eth2v1.ChainReorgEvent{
		Slot:         phase0.Slot(slot),
		Depth:        1,
		OldHeadBlock: oldRoot,
		NewHeadBlock: newRoot,
	}

	t.Run("cache miss", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewChainReorgEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			reorg,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(true)

		event := NewChainReorgEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			reorg,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false).Times(2)

		event := NewChainReorgEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			reorg,
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
