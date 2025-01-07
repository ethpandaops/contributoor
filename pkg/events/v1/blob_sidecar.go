package v1

import (
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/pkg/events"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BlobSidecarEvent represents a blob sidecar event.
type BlobSidecarEvent struct {
	events.BaseEvent
	data     *eth2v1.BlobSidecarEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewBlobSidecarEvent(beacon events.BeaconDataProvider, meta *xatu.Meta, data *eth2v1.BlobSidecarEvent, recvTime time.Time) *BlobSidecarEvent {
	return &BlobSidecarEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
	}
}

func (e *BlobSidecarEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOB_SIDECAR.String()
}

func (e *BlobSidecarEvent) Data() interface{} {
	return e.data
}

func (e *BlobSidecarEvent) Decorated() *xatu.DecoratedEvent {
	//TODO(@matty): Populate event data.
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOB_SIDECAR,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsBlobSidecar{},
	}

	if e.beacon == nil {
		return decorated
	}

	//TODO(@matty): Populate additional data.
	extra := &xatu.ClientMeta_AdditionalEthV1EventsBlobSidecarData{}
	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsBlobSidecar{
		EthV1EventsBlobSidecar: extra,
	}

	return decorated
}
