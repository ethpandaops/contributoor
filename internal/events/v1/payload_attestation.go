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

// PayloadAttestationEvent represents a payload_attestation_message SSE event
// (Gloas, EIP-7732 PTC vote).
type PayloadAttestationEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *gloas.PayloadAttestationMessage
	beacon   events.BeaconDataProvider
	cache    *ttlcache.Cache[string, time.Time]
	recvTime time.Time
}

func NewPayloadAttestationEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	cache *ttlcache.Cache[string, time.Time],
	meta *xatu.Meta,
	data *gloas.PayloadAttestationMessage,
	recvTime time.Time,
) *PayloadAttestationEvent {
	return &PayloadAttestationEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		cache:     cache,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_PAYLOAD_ATTESTATION.String()),
	}
}

func (e *PayloadAttestationEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_PAYLOAD_ATTESTATION.String()
}

func (e *PayloadAttestationEvent) Data() any {
	return e.data
}

func (e *PayloadAttestationEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_PAYLOAD_ATTESTATION,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsPayloadAttestation{
			EthV1EventsPayloadAttestation: xatuethv1.NewPayloadAttestationMessageFromGloas(e.data),
		},
	}

	if e.beacon == nil || e.data.Data == nil {
		return decorated
	}

	slot, epoch, propagation := slotTiming(e.beacon, uint64(e.data.Data.Slot), e.recvTime)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsPayloadAttestation{
		EthV1EventsPayloadAttestation: &xatu.ClientMeta_AdditionalEthV1EventsPayloadAttestationData{
			Slot:        slot,
			Epoch:       epoch,
			Propagation: propagation,
		},
	}

	return decorated
}

func (e *PayloadAttestationEvent) Ignore(ctx context.Context) (bool, error) {
	if e.data == nil || e.data.Data == nil {
		return true, nil
	}

	return shouldIgnore(ctx, e.log, e.beacon, e.cache, uint64(e.data.Data.Slot), e.data, e.recvTime)
}
