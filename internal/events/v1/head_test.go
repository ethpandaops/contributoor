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

func TestHeadEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		now        = time.Now()
		slot       = uint64(123)
		epoch      = uint64(3)
		mockSlot   = ethwallclock.NewSlot(slot, now.Add(-10*time.Second), now)
		mockEpoch  = ethwallclock.NewEpoch(epoch, now.Add(-5*time.Minute), now)
		cache      = ttlcache.New[string, time.Time]() // Create a new cache for testing
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(mockSlot)
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(mockEpoch)

	event := NewHeadEvent(
		logrus.New(),
		mockBeacon,
		cache,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&eth2v1.HeadEvent{
			Slot:                      phase0.Slot(slot),
			Block:                     phase0.Root{0x1},
			State:                     phase0.Root{0x2},
			EpochTransition:           true,
			PreviousDutyDependentRoot: phase0.Root{0x3},
			CurrentDutyDependentRoot:  phase0.Root{0x4},
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsHeadV2()
		additionalData = decorated.GetEthV1EventsHeadV2()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, slot, additionalData.Slot.Value)
	require.Equal(t, "0x0100000000000000000000000000000000000000000000000000000000000000", additionalData.Block)
	require.Equal(t, "0x0200000000000000000000000000000000000000000000000000000000000000", additionalData.State)
	require.True(t, additionalData.EpochTransition)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, epoch, metaData.Epoch.Number.Value)
}

func TestHeadEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		blockRoot  = phase0.Root{0x1}
		cache      = ttlcache.New[string, time.Time]()
	)

	head := &eth2v1.HeadEvent{
		Slot:  phase0.Slot(slot),
		Block: blockRoot,
	}

	t.Run("cache miss", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewHeadEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			head,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(true)

		event := NewHeadEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			head,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false).Times(2)

		event := NewHeadEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			head,
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
