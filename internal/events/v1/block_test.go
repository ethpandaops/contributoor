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

func TestBlockEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		mockSlot   = ethwallclock.NewSlot(slot, now.Add(-10*time.Second), now)
		mockEpoch  = ethwallclock.NewEpoch(3, now.Add(-5*time.Minute), now)
		block      = phase0.Root{0x1}                  // Simple root for testing
		cache      = ttlcache.New[string, time.Time]() // Create a new cache for testing
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewBlockEvent(
		logrus.New(),
		mockBeacon,
		cache,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&eth2v1.BlockEvent{
			Slot:                phase0.Slot(slot),
			Block:               block,
			ExecutionOptimistic: true,
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsBlockV2()
		additionalData = decorated.GetEthV1EventsBlockV2()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOCK_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, slot, additionalData.Slot.Value)
	require.Equal(t, block.String(), additionalData.Block)
	require.True(t, additionalData.ExecutionOptimistic)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, slot, metaData.Slot.Number.Value)
	require.Equal(t, uint64(3), metaData.Epoch.Number.Value)
}

func TestBlockEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		blockRoot  = phase0.Root{0x1}
		cache      = ttlcache.New[string, time.Time]()
	)

	block := &eth2v1.BlockEvent{
		Slot:                phase0.Slot(slot),
		Block:               blockRoot,
		ExecutionOptimistic: true,
	}

	t.Run("cache miss", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewBlockEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			block,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(true)

		event := NewBlockEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			block,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false).Times(2)

		event := NewBlockEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			block,
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
