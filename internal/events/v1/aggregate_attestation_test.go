package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAggregateAttestationEvent_Type(t *testing.T) {
	event := &AggregateAttestationEvent{}
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2.String(), event.Type())
}

func TestAggregateAttestationEvent_Decorated_NilBeacon(t *testing.T) {
	// Test Decorated() when beacon is nil (skips wallclock logic)
	var (
		now   = time.Now()
		slot  = phase0.Slot(123)
		index = phase0.CommitteeIndex(1)
		cache = ttlcache.New[string, time.Time]()
	)

	// Create aggregation bits with multiple bits set (aggregate)
	aggregationBits := bitfield.NewBitlist(128)
	aggregationBits.SetBitAt(0, true)
	aggregationBits.SetBitAt(5, true)
	aggregationBits.SetBitAt(10, true)

	attestation := &spec.VersionedAttestation{
		Version: spec.DataVersionPhase0,
		Phase0: &phase0.Attestation{
			AggregationBits: aggregationBits,
			Data: &phase0.AttestationData{
				Slot:            slot,
				Index:           index,
				BeaconBlockRoot: phase0.Root{0x1},
				Source: &phase0.Checkpoint{
					Epoch: phase0.Epoch(2),
					Root:  phase0.Root{0x2},
				},
				Target: &phase0.Checkpoint{
					Epoch: phase0.Epoch(3),
					Root:  phase0.Root{0x3},
				},
			},
			Signature: phase0.BLSSignature{},
		},
	}

	event := NewAggregateAttestationEvent(
		logrus.New(),
		nil, // nil beacon to skip wallclock logic
		cache,
		&xatu.Meta{Client: &xatu.ClientMeta{}},
		attestation,
		now,
	)

	decorated := event.Decorated()
	require.NotNil(t, decorated)
	require.Equal(t, xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2, decorated.Event.Name)

	// Verify attestation data
	attestData := decorated.GetEthV1EventsAttestationV2()
	require.NotNil(t, attestData)
	require.Equal(t, uint64(slot), attestData.Data.Slot.Value)
	require.Equal(t, uint64(index), attestData.Data.Index.Value)

	// Verify aggregation bits are populated (non-empty for aggregate)
	require.NotEmpty(t, attestData.AggregationBits)
	require.True(t, len(attestData.AggregationBits) > 2) // "0x" + hex data
}

func TestAggregateAttestationEvent_Ignore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		now  = time.Now()
		slot = phase0.Slot(123)
	)

	aggregationBits := bitfield.NewBitlist(128)
	aggregationBits.SetBitAt(0, true)

	attestation := &spec.VersionedAttestation{
		Version: spec.DataVersionPhase0,
		Phase0: &phase0.Attestation{
			AggregationBits: aggregationBits,
			Data: &phase0.AttestationData{
				Slot:            slot,
				Index:           phase0.CommitteeIndex(1),
				BeaconBlockRoot: phase0.Root{0x1},
				Source: &phase0.Checkpoint{
					Epoch: phase0.Epoch(2),
					Root:  phase0.Root{0x2},
				},
				Target: &phase0.Checkpoint{
					Epoch: phase0.Epoch(3),
					Root:  phase0.Root{0x3},
				},
			},
			Signature: phase0.BLSSignature{},
		},
	}

	t.Run("not synced", func(t *testing.T) {
		cache := ttlcache.New[string, time.Time]()
		mockBeacon := mock.NewMockBeaconDataProvider(ctrl)
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(fmt.Errorf("not synced"))

		event := NewAggregateAttestationEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			attestation,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.Error(t, err)
		require.True(t, ignore)
	})

	t.Run("unexpected network", func(t *testing.T) {
		cache := ttlcache.New[string, time.Time]()
		mockBeacon := mock.NewMockBeaconDataProvider(ctrl)
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(uint64(slot)).Return(true)

		event := NewAggregateAttestationEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			attestation,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("duplicate detection", func(t *testing.T) {
		cache := ttlcache.New[string, time.Time]()
		mockBeacon := mock.NewMockBeaconDataProvider(ctrl)
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(uint64(slot)).Return(false).Times(2)

		event := NewAggregateAttestationEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			attestation,
			now,
		)

		// First call should not be ignored
		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore)

		// Second call with same data should be ignored (duplicate)
		ignore, err = event.Ignore(context.Background())
		require.NoError(t, err)
		require.True(t, ignore)
	})

	t.Run("no subnet filtering for aggregates", func(t *testing.T) {
		// Unlike single attestations, aggregate attestations should NOT
		// check subnets since they're broadcast globally
		cache := ttlcache.New[string, time.Time]()
		mockBeacon := mock.NewMockBeaconDataProvider(ctrl)
		mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
		mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(uint64(slot)).Return(false)
		// Note: NO calls to IsActiveSubnet or RecordSeenSubnet expected

		event := NewAggregateAttestationEvent(
			logrus.New(),
			mockBeacon,
			cache,
			&xatu.Meta{Client: &xatu.ClientMeta{}},
			attestation,
			now,
		)

		ignore, err := event.Ignore(context.Background())
		require.NoError(t, err)
		require.False(t, ignore)
	})
}
