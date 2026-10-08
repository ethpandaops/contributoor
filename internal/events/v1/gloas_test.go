package v1

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/internal/events/mock"
	"github.com/ethpandaops/ethwallclock"
	eth2v1 "github.com/ethpandaops/go-eth2-client/api/v1"
	"github.com/ethpandaops/go-eth2-client/spec/gloas"
	"github.com/ethpandaops/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type gloasEventCase struct {
	name     string
	slot     uint64
	wantType xatu.Event_Name
	build    func(beacon events.BeaconDataProvider, cache *ttlcache.Cache[string, time.Time], now time.Time) events.Event
	// check asserts the event payload and returns the slot/epoch metadata.
	check func(t *testing.T, decorated *xatu.DecoratedEvent) (slot *xatu.SlotV2, epoch *xatu.EpochV2, propagation *xatu.PropagationV2)
}

func gloasEventCases() []gloasEventCase {
	const slot = uint64(353024 * 32)

	var (
		blockRoot = phase0.Root{0x1}
		blockHash = phase0.Hash32{0x2}
		meta      = func() *xatu.Meta { return &xatu.Meta{Client: &xatu.ClientMeta{}} }
		log       = logrus.New()
	)

	payload := &eth2v1.ExecutionPayloadEvent{
		Slot:         phase0.Slot(slot),
		BuilderIndex: 7,
		BlockHash:    blockHash,
		BlockRoot:    blockRoot,
	}

	return []gloasEventCase{
		{
			name:     "head_v2",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_HEAD_V3,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewHeadV2Event(log, b, c, meta(), &eth2v1.HeadEventV2{
					Slot:                      phase0.Slot(slot),
					Block:                     blockRoot,
					State:                     phase0.Root{0x3},
					PayloadStatus:             "full",
					EpochTransition:           true,
					CurrentEpochDependentRoot: phase0.Root{0x4},
					NextEpochDependentRoot:    phase0.Root{0x5},
				}, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsHeadV3()
				require.NotNil(t, data)
				require.Equal(t, slot, data.GetSlot().GetValue())
				require.Equal(t, blockRoot.String(), data.GetBlock())
				require.Equal(t, "full", data.GetPayloadStatus())
				require.True(t, data.GetEpochTransition())
				require.Equal(t, phase0.Root{0x5}.String(), data.GetNextEpochDependentRoot())

				extra := d.GetMeta().GetClient().GetEthV1EventsHeadV3()
				require.NotNil(t, extra)
				// Mirrors the xatu sentry: slot number lives on the event only.
				require.Nil(t, extra.GetSlot().GetNumber())

				return &xatu.SlotV2{Number: data.GetSlot(), StartDateTime: extra.GetSlot().GetStartDateTime()}, extra.GetEpoch(), extra.GetPropagation()
			},
		},
		{
			name:     "execution_payload",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewExecutionPayloadEvent(log, b, c, meta(), payload, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsExecutionPayload()
				require.NotNil(t, data)
				require.Nil(t, d.GetEthV1EventsExecutionPayloadGossip())
				require.Equal(t, blockRoot.String(), data.GetBlockRoot())
				require.Equal(t, blockHash.String(), data.GetBlockHash())
				require.Equal(t, uint64(7), data.GetBuilderIndex().GetValue())

				extra := d.GetMeta().GetClient().GetEthV1EventsExecutionPayload()
				require.NotNil(t, extra)

				return extra.GetSlot(), extra.GetEpoch(), extra.GetPropagation()
			},
		},
		{
			name:     "execution_payload_gossip",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_GOSSIP,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewExecutionPayloadGossipEvent(log, b, c, meta(), payload, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsExecutionPayloadGossip()
				require.NotNil(t, data)
				require.Nil(t, d.GetEthV1EventsExecutionPayload())
				require.Equal(t, blockRoot.String(), data.GetBlockRoot())

				extra := d.GetMeta().GetClient().GetEthV1EventsExecutionPayloadGossip()
				require.NotNil(t, extra)

				return extra.GetSlot(), extra.GetEpoch(), extra.GetPropagation()
			},
		},
		{
			name:     "execution_payload_available",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_AVAILABLE,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewExecutionPayloadAvailableEvent(log, b, c, meta(), &eth2v1.ExecutionPayloadAvailableEvent{
					Slot:      phase0.Slot(slot),
					BlockRoot: blockRoot,
				}, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsExecutionPayloadAvailable()
				require.NotNil(t, data)
				require.Equal(t, blockRoot.String(), data.GetBlockRoot())

				extra := d.GetMeta().GetClient().GetEthV1EventsExecutionPayloadAvailable()
				require.NotNil(t, extra)

				return extra.GetSlot(), extra.GetEpoch(), extra.GetPropagation()
			},
		},
		{
			name:     "execution_payload_bid",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_EXECUTION_PAYLOAD_BID,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewExecutionPayloadBidEvent(log, b, c, meta(), &gloas.SignedExecutionPayloadBid{
					Message: &gloas.ExecutionPayloadBid{
						Slot:         phase0.Slot(slot),
						BuilderIndex: 9,
						BlockHash:    blockHash,
						Value:        1000,
					},
				}, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsExecutionPayloadBid()
				require.NotNil(t, data)
				require.Equal(t, slot, data.GetMessage().GetSlot().GetValue())
				require.Equal(t, uint64(9), data.GetMessage().GetBuilderIndex().GetValue())

				extra := d.GetMeta().GetClient().GetEthV1EventsExecutionPayloadBid()
				require.NotNil(t, extra)

				return extra.GetSlot(), extra.GetEpoch(), extra.GetPropagation()
			},
		},
		{
			name:     "payload_attestation_message",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_PAYLOAD_ATTESTATION,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewPayloadAttestationEvent(log, b, c, meta(), &gloas.PayloadAttestationMessage{
					ValidatorIndex: 42,
					Data: &gloas.PayloadAttestationData{
						BeaconBlockRoot: blockRoot,
						Slot:            phase0.Slot(slot),
						PayloadPresent:  true,
					},
				}, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsPayloadAttestation()
				require.NotNil(t, data)
				require.Equal(t, uint64(42), data.GetValidatorIndex().GetValue())
				require.True(t, data.GetData().GetPayloadPresent())

				extra := d.GetMeta().GetClient().GetEthV1EventsPayloadAttestation()
				require.NotNil(t, extra)

				return extra.GetSlot(), extra.GetEpoch(), extra.GetPropagation()
			},
		},
		{
			name:     "proposer_preferences",
			slot:     slot,
			wantType: xatu.Event_BEACON_API_ETH_V1_EVENTS_PROPOSER_PREFERENCES,
			build: func(b events.BeaconDataProvider, c *ttlcache.Cache[string, time.Time], now time.Time) events.Event {
				return NewProposerPreferencesEvent(log, b, c, meta(), &gloas.SignedProposerPreferences{
					Message: &gloas.ProposerPreferences{
						ProposalSlot:   phase0.Slot(slot),
						ValidatorIndex: 11,
						TargetGasLimit: 60_000_000,
					},
				}, now)
			},
			check: func(t *testing.T, d *xatu.DecoratedEvent) (*xatu.SlotV2, *xatu.EpochV2, *xatu.PropagationV2) {
				t.Helper()

				data := d.GetEthV1EventsProposerPreferences()
				require.NotNil(t, data)
				require.Equal(t, uint64(11), data.GetMessage().GetValidatorIndex().GetValue())

				extra := d.GetMeta().GetClient().GetEthV1EventsProposerPreferences()
				require.NotNil(t, extra)

				return extra.GetSlot(), extra.GetEpoch(), extra.GetPropagation()
			},
		},
	}
}

func TestGloasEvents_Decorated(t *testing.T) {
	for _, tc := range gloasEventCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var (
				mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
				now        = time.Now()
				slotStart  = now.Add(-1500 * time.Millisecond)
				epoch      = tc.slot / 32
				mockSlot   = ethwallclock.NewSlot(tc.slot, slotStart, slotStart.Add(12*time.Second))
				mockEpoch  = ethwallclock.NewEpoch(epoch, slotStart, slotStart.Add(384*time.Second))
			)

			mockBeacon.EXPECT().GetSlot(tc.slot).Return(mockSlot)
			mockBeacon.EXPECT().GetEpochFromSlot(tc.slot).Return(mockEpoch)

			event := tc.build(mockBeacon, ttlcache.New[string, time.Time](), now)
			require.Equal(t, tc.wantType.String(), event.Type())

			decorated := event.Decorated()
			require.NotNil(t, decorated)
			require.Equal(t, tc.wantType, decorated.GetEvent().GetName())
			require.NotEmpty(t, decorated.GetEvent().GetId())

			slot, ep, propagation := tc.check(t, decorated)
			require.Equal(t, tc.slot, slot.GetNumber().GetValue())
			require.Equal(t, slotStart.Unix(), slot.GetStartDateTime().AsTime().Unix())
			require.Equal(t, epoch, ep.GetNumber().GetValue())
			require.Equal(t, uint64(1500), propagation.GetSlotStartDiff().GetValue())
		})
	}
}

func TestGloasEvents_Ignore(t *testing.T) {
	for _, tc := range gloasEventCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var (
				mockBeacon = mock.NewMockBeaconDataProvider(ctrl)
				cache      = ttlcache.New[string, time.Time]()
				now        = time.Now()
			)

			// Not synced.
			mockBeacon.EXPECT().Synced(gomock.Any()).Return(errors.New("not synced"))

			ignore, err := tc.build(mockBeacon, cache, now).Ignore(context.Background())
			require.Error(t, err)
			require.True(t, ignore)

			// Unexpected network.
			mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil)
			mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(tc.slot).Return(true)

			ignore, err = tc.build(mockBeacon, cache, now).Ignore(context.Background())
			require.NoError(t, err)
			require.True(t, ignore)

			// First sighting forwarded, duplicate dropped.
			mockBeacon.EXPECT().Synced(gomock.Any()).Return(nil).Times(2)
			mockBeacon.EXPECT().IsSlotFromUnexpectedNetwork(tc.slot).Return(false).Times(2)

			ignore, err = tc.build(mockBeacon, cache, now).Ignore(context.Background())
			require.NoError(t, err)
			require.False(t, ignore)

			ignore, err = tc.build(mockBeacon, cache, now).Ignore(context.Background())
			require.NoError(t, err)
			require.True(t, ignore)
		})
	}
}

func TestGloasEvents_IgnoreIncomplete(t *testing.T) {
	var (
		log   = logrus.New()
		meta  = &xatu.Meta{Client: &xatu.ClientMeta{}}
		cache = ttlcache.New[string, time.Time]()
		now   = time.Now()
	)

	// No beacon calls expected: incomplete payloads are dropped up front.
	mockBeacon := mock.NewMockBeaconDataProvider(gomock.NewController(t))

	for name, event := range map[string]events.Event{
		"execution_payload nil":        NewExecutionPayloadEvent(log, mockBeacon, cache, meta, nil, now),
		"execution_payload_available":  NewExecutionPayloadAvailableEvent(log, mockBeacon, cache, meta, nil, now),
		"execution_payload_bid":        NewExecutionPayloadBidEvent(log, mockBeacon, cache, meta, &gloas.SignedExecutionPayloadBid{}, now),
		"payload_attestation_message":  NewPayloadAttestationEvent(log, mockBeacon, cache, meta, &gloas.PayloadAttestationMessage{}, now),
		"proposer_preferences message": NewProposerPreferencesEvent(log, mockBeacon, cache, meta, &gloas.SignedProposerPreferences{}, now),
	} {
		t.Run(name, func(t *testing.T) {
			ignore, err := event.Ignore(context.Background())
			require.NoError(t, err)
			require.True(t, ignore)
		})
	}
}

// TestDataColumnSidecarEvent_GloasShape guards the go-eth2-client pin: from
// Gloas (beacon-APIs #583) the data_column_sidecar event carries no
// kzg_commitments. go-eth2-client <= v0.1.5 rejected that shape, silently
// dropping the event for every lighthouse/lodestar/teku/grandine node.
func TestDataColumnSidecarEvent_GloasShape(t *testing.T) {
	for name, raw := range map[string]string{
		"field omitted": `{"block_root":"0x0100000000000000000000000000000000000000000000000000000000000000","index":"5","slot":"11296768"}`,
		"empty list":    `{"block_root":"0x0100000000000000000000000000000000000000000000000000000000000000","index":"5","slot":"11296768","kzg_commitments":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var ev eth2v1.DataColumnSidecarEvent
			require.NoError(t, json.Unmarshal([]byte(raw), &ev))

			decorated := NewDataColumnSidecarEvent(logrus.New(), nil, nil, &xatu.Meta{Client: &xatu.ClientMeta{}}, &ev, time.Now()).Decorated()

			data := decorated.GetEthV1EventsDataColumnSidecar()
			require.NotNil(t, data)
			require.Equal(t, uint64(11296768), data.GetSlot().GetValue())
			require.Equal(t, uint64(5), data.GetIndex().GetValue())
			require.Equal(t, uint32(0), data.GetKzgCommitmentsCount().GetValue())
		})
	}
}
