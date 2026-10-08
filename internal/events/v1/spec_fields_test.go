package v1

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	eth2v1 "github.com/ethpandaops/go-eth2-client/api/v1"
	"github.com/ethpandaops/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBlockEvent_GloasFields(t *testing.T) {
	builderIndex := uint64(42)
	blockHash := phase0.Hash32{0xab}

	gloas := NewBlockEvent(logrus.New(), nil, ttlcache.New[string, time.Time](), &xatu.Meta{Client: &xatu.ClientMeta{}},
		&eth2v1.BlockEvent{Slot: 10, Block: phase0.Root{0x1}, BuilderIndex: &builderIndex, BlockHash: &blockHash}, time.Now(),
	).Decorated().GetEthV1EventsBlockV2()

	require.NotNil(t, gloas.GetBuilderIndex())
	require.Equal(t, builderIndex, gloas.GetBuilderIndex().GetValue())
	require.Equal(t, blockHash.String(), gloas.GetBlockHash())

	// Pre-Gloas blocks carry neither field; they must stay unset rather than 0.
	preGloas := NewBlockEvent(logrus.New(), nil, ttlcache.New[string, time.Time](), &xatu.Meta{Client: &xatu.ClientMeta{}},
		&eth2v1.BlockEvent{Slot: 10, Block: phase0.Root{0x1}}, time.Now(),
	).Decorated().GetEthV1EventsBlockV2()

	require.Nil(t, preGloas.GetBuilderIndex())
	require.Empty(t, preGloas.GetBlockHash())
}

func TestExecutionOptimistic_Forwarded(t *testing.T) {
	meta := func() *xatu.Meta { return &xatu.Meta{Client: &xatu.ClientMeta{}} }
	cache := ttlcache.New[string, time.Time]()

	head := NewHeadEvent(logrus.New(), nil, cache, meta(), &eth2v1.HeadEvent{Slot: 10, ExecutionOptimistic: true}, time.Now())
	require.True(t, head.Decorated().GetEthV1EventsHeadV2().GetExecutionOptimistic())

	checkpoint := NewFinalizedCheckpointEvent(logrus.New(), nil, cache, meta(), &eth2v1.FinalizedCheckpointEvent{Epoch: 2, ExecutionOptimistic: true}, time.Now())
	require.True(t, checkpoint.Decorated().GetEthV1EventsFinalizedCheckpointV2().GetExecutionOptimistic())

	reorg := NewChainReorgEvent(logrus.New(), nil, cache, meta(), &eth2v1.ChainReorgEvent{Slot: 10, Epoch: 0, ExecutionOptimistic: true}, time.Now())
	require.True(t, reorg.Decorated().GetEthV1EventsChainReorgV2().GetExecutionOptimistic())
}

func TestChainReorgEvent_EpochDerivedFromSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		slot       = uint64(343871)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(ethwallclock.NewSlot(slot, now.Add(-time.Second), now)).AnyTimes()
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(ethwallclock.NewEpoch(slot/32, now.Add(-time.Minute), now)).AnyTimes()

	// Nimbus omits epoch, which decodes as 0.
	nimbus := NewChainReorgEvent(logrus.New(), mockBeacon, ttlcache.New[string, time.Time](), &xatu.Meta{Client: &xatu.ClientMeta{}},
		&eth2v1.ChainReorgEvent{Slot: phase0.Slot(slot), Depth: 2}, now)
	require.Equal(t, slot/32, nimbus.Decorated().GetEthV1EventsChainReorgV2().GetEpoch().GetValue())

	// A reported epoch is kept as is.
	other := NewChainReorgEvent(logrus.New(), mockBeacon, ttlcache.New[string, time.Time](), &xatu.Meta{Client: &xatu.ClientMeta{}},
		&eth2v1.ChainReorgEvent{Slot: phase0.Slot(slot), Depth: 2, Epoch: 10745}, now)
	require.Equal(t, uint64(10745), other.Decorated().GetEthV1EventsChainReorgV2().GetEpoch().GetValue())
}

func TestFastConfirmationEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now         = time.Now()
		slot        = uint64(100)
		currentSlot = phase0.Slot(101)
		block       = phase0.Root{0x2}
		mockBeacon  = mock.NewMockBeaconDataProvider(ctrl)
		wallclock   = ethwallclock.NewEthereumBeaconChain(now.Add(-time.Hour), 12*time.Second, 32)
	)

	mockBeacon.EXPECT().GetSlot(slot).Return(ethwallclock.NewSlot(slot, now.Add(-time.Second), now)).AnyTimes()
	mockBeacon.EXPECT().GetEpochFromSlot(slot).Return(ethwallclock.NewEpoch(3, now.Add(-time.Minute), now)).AnyTimes()
	mockBeacon.EXPECT().GetWallclock().Return(wallclock).AnyTimes()

	event := NewFastConfirmationEvent(logrus.New(), mockBeacon, ttlcache.New[string, time.Time](), &xatu.Meta{Client: &xatu.ClientMeta{}},
		&eth2v1.FastConfirmationEvent{Slot: phase0.Slot(slot), Block: block, CurrentSlot: &currentSlot}, now)

	decorated := event.Decorated()
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_FAST_CONFIRMATION.String(), event.Type())

	payload := decorated.GetEthV1EventsFastConfirmation()
	require.Equal(t, slot, payload.GetSlot().GetValue())
	require.Equal(t, block.String(), payload.GetBlock())
	require.Equal(t, uint64(currentSlot), payload.GetCurrentSlot().GetValue())

	additional := decorated.GetMeta().GetClient().GetEthV1EventsFastConfirmation()
	require.Equal(t, slot, additional.GetSlot().GetNumber().GetValue())
	require.Equal(t, uint64(3), additional.GetEpoch().GetNumber().GetValue())
	require.NotNil(t, additional.GetWallclockSlot())

	// Beacon nodes without current_slot leave it unset.
	missing := NewFastConfirmationEvent(logrus.New(), mockBeacon, ttlcache.New[string, time.Time](), &xatu.Meta{Client: &xatu.ClientMeta{}},
		&eth2v1.FastConfirmationEvent{Slot: phase0.Slot(slot), Block: block}, now)
	require.Nil(t, missing.Decorated().GetEthV1EventsFastConfirmation().GetCurrentSlot())
}

// fast_confirmation fires on every algorithm run; repeated runs confirming the
// same block are duplicates regardless of current_slot.
func TestFastConfirmationEvent_IgnoreRepeatedRuns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		cache      = ttlcache.New[string, time.Time]()
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		block      = phase0.Root{0x2}
		first      = phase0.Slot(101)
		second     = phase0.Slot(102)
	)

	mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).AnyTimes()
	mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(gomock.Any()).Return(false).AnyTimes()

	newEvent := func(slot phase0.Slot, block phase0.Root, current *phase0.Slot) *FastConfirmationEvent {
		return NewFastConfirmationEvent(logrus.New(), mockBeacon, cache, &xatu.Meta{Client: &xatu.ClientMeta{}},
			&eth2v1.FastConfirmationEvent{Slot: slot, Block: block, CurrentSlot: current}, now)
	}

	ignore, err := newEvent(100, block, &first).Ignore(context.Background())
	require.NoError(t, err)
	require.False(t, ignore)

	ignore, err = newEvent(100, block, &second).Ignore(context.Background())
	require.NoError(t, err)
	require.True(t, ignore, "repeat run for the same confirmed block should be a duplicate")

	ignore, err = newEvent(101, phase0.Root{0x3}, &second).Ignore(context.Background())
	require.NoError(t, err)
	require.False(t, ignore, "a newly confirmed block is not a duplicate")
}
