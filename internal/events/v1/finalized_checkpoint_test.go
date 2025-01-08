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

func TestFinalizedCheckpointEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now        = time.Now()
		epoch      = uint64(3)
		mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
		mockEpoch  = ethwallclock.NewEpoch(epoch, now.Add(-5*time.Minute), now)
		block      = phase0.Root{0x1} // Simple root for testing
		state      = phase0.Root{0x2} // Simple root for testing
	)

	mockBeacon.EXPECT().GetEpoch(epoch).Return(mockEpoch)

	event := NewFinalizedCheckpointEvent(
		logrus.New(),
		mockBeacon,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&eth2v1.FinalizedCheckpointEvent{
			Epoch: phase0.Epoch(epoch),
			Block: block,
			State: state,
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsFinalizedCheckpointV2()
		additionalData = decorated.GetEthV1EventsFinalizedCheckpointV2()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_FINALIZED_CHECKPOINT_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, epoch, additionalData.Epoch.Value)
	require.Equal(t, block.String(), additionalData.Block)
	require.Equal(t, state.String(), additionalData.State)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, epoch, metaData.Epoch.Number.Value)
}
