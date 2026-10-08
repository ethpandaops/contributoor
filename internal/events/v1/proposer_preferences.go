package v1

import (
	"context"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/go-eth2-client/spec/gloas"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProposerPreferencesEvent represents a proposer_preferences SSE event
// (Gloas, EIP-7732).
type ProposerPreferencesEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *gloas.SignedProposerPreferences
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewProposerPreferencesEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *gloas.SignedProposerPreferences,
	recvTime time.Time,
) *ProposerPreferencesEvent {
	return &ProposerPreferencesEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_PROPOSER_PREFERENCES.String()),
	}
}

func (e *ProposerPreferencesEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_PROPOSER_PREFERENCES.String()
}

func (e *ProposerPreferencesEvent) Data() any {
	return e.data
}

func (e *ProposerPreferencesEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_PROPOSER_PREFERENCES,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsProposerPreferences{
			EthV1EventsProposerPreferences: xatuethv1.NewSignedProposerPreferencesFromGloas(e.data),
		},
	}

	if e.beacon == nil || e.data.Message == nil {
		return decorated
	}

	// Timing is relative to the proposal slot, which is usually in the future
	// when the preferences are gossiped (mirrors the xatu sentry).
	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Message.ProposalSlot), e.recvTime)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsProposerPreferences{
		EthV1EventsProposerPreferences: &xatu.ClientMeta_AdditionalEthV1EventsProposerPreferencesData{
			Slot:        slot,
			Epoch:       epoch,
			Propagation: propagation,
		},
	}

	return decorated
}

func (e *ProposerPreferencesEvent) Ignore(ctx context.Context) (bool, error) {
	if e.data == nil || e.data.Message == nil {
		return true, nil
	}

	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Message.ProposalSlot), e.data, e.recvTime)
}
