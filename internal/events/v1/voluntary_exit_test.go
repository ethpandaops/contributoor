package v1

import (
	"testing"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestVoluntaryExitEvent_Decorated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now           = time.Now()
		slot          = uint64(123)
		epoch         = uint64(3)
		slotDuration  = 12 * time.Second
		genesis       = now.Add(-time.Duration(slot) * slotDuration)
		mockBeacon    = mock.NewMockBeaconDataProvider(ctrl)
		mockEpoch     = ethwallclock.NewEpoch(epoch, now.Add(-5*time.Minute), now)
		mockWallclock = ethwallclock.NewEthereumBeaconChain(genesis, slotDuration, 32)
		signature     = phase0.BLSSignature{} // 96-byte zero signature
	)

	mockBeacon.EXPECT().GetEpoch(epoch).Return(mockEpoch)
	mockBeacon.EXPECT().GetWallclock().Return(mockWallclock)

	event := NewVoluntaryExitEvent(
		logrus.New(),
		mockBeacon,
		&xatu.Meta{
			Client: &xatu.ClientMeta{},
		},
		&phase0.SignedVoluntaryExit{
			Message: &phase0.VoluntaryExit{
				Epoch:          phase0.Epoch(epoch),
				ValidatorIndex: phase0.ValidatorIndex(456),
			},
			Signature: signature,
		},
		now,
	)

	var (
		decorated      = event.Decorated()
		metaData       = decorated.Meta.Client.GetEthV1EventsVoluntaryExitV2()
		additionalData = decorated.GetEthV1EventsVoluntaryExitV2()
	)

	// Assert event.
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_VOLUNTARY_EXIT_V2.String(), event.Type())

	// Assert additional data.
	require.NotNil(t, additionalData)
	require.Equal(t, uint64(456), additionalData.Message.ValidatorIndex.Value)
	require.Equal(t, epoch, additionalData.Message.Epoch.Value)
	require.Equal(t, signature.String(), additionalData.Signature)

	// Assert metadata.
	require.NotNil(t, metaData)
	require.Equal(t, epoch, metaData.Epoch.Number.Value)
	require.Equal(t, slot, metaData.WallclockSlot.Number.Value)
	require.Equal(t, epoch, metaData.WallclockEpoch.Number.Value)
}
