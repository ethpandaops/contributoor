package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/internal/events/mock"
	eth2v1 "github.com/ethpandaops/go-eth2-client/api/v1"
	"github.com/ethpandaops/go-eth2-client/spec/deneb"
	"github.com/ethpandaops/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataColumnSidecarEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(123)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		blockRoot  = phase0.Root{0x1}
		cache      = ttlcache.New[string, time.Time]()
	)

	column := &eth2v1.DataColumnSidecarEvent{
		Slot:           phase0.Slot(slot),
		BlockRoot:      blockRoot,
		Index:          1,
		KZGCommitments: []deneb.KZGCommitment{{}},
	}

	t.Run("cache miss", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewDataColumnSidecarEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, column, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(true)

		event := NewDataColumnSidecarEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, column, now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(slot).Return(false).Times(2)

		event := NewDataColumnSidecarEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, column, now,
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
		rollbackColumn := &eth2v1.DataColumnSidecarEvent{
			Slot:           phase0.Slot(rollbackSlot),
			BlockRoot:      blockRoot,
			Index:          1,
			KZGCommitments: []deneb.KZGCommitment{{}},
		}

		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(rollbackSlot).Return(false).Times(2)

		first := NewDataColumnSidecarEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, rollbackColumn, now,
		)

		ignore, err := first.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore, "first delivery must be processed")

		first.Rollback()

		second := NewDataColumnSidecarEvent(
			logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}}, rollbackColumn, now,
		)

		ignore, err = second.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore,
			"after Rollback, a re-delivery of the same event must not be treated as a duplicate")
	})
}
