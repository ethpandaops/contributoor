package v1

import (
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/contributoor/internal/events"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// BlobSidecarEvent represents a blob sidecar event.
type BlobSidecarEvent struct {
	events.BaseEvent
	log      logrus.FieldLogger
	data     *eth2v1.BlobSidecarEvent
	beacon   events.BeaconDataProvider
	recvTime time.Time
}

func NewBlobSidecarEvent(
	log logrus.FieldLogger,
	beacon events.BeaconDataProvider,
	meta *xatu.Meta,
	data *eth2v1.BlobSidecarEvent,
	recvTime time.Time,
) *BlobSidecarEvent {
	return &BlobSidecarEvent{
		BaseEvent: events.NewBaseEvent(meta),
		data:      data,
		beacon:    beacon,
		recvTime:  recvTime,
		log:       log.WithField("event", xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOB_SIDECAR.String()),
	}
}

func (e *BlobSidecarEvent) Type() string {
	return xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOB_SIDECAR.String()
}

func (e *BlobSidecarEvent) Data() interface{} {
	return e.data
}

func (e *BlobSidecarEvent) Decorated() *xatu.DecoratedEvent {
	decorated := &xatu.DecoratedEvent{
		Meta: e.Meta(),
		Event: &xatu.Event{
			Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_BLOB_SIDECAR,
			DateTime: timestamppb.New(e.recvTime),
			Id:       uuid.New().String(),
		},
		Data: &xatu.DecoratedEvent_EthV1EventsBlobSidecar{
			EthV1EventsBlobSidecar: &xatuethv1.EventBlobSidecar{
				BlockRoot:     xatuethv1.RootAsString(e.data.BlockRoot),
				Slot:          &wrapperspb.UInt64Value{Value: uint64(e.data.Slot)},
				Index:         &wrapperspb.UInt64Value{Value: uint64(e.data.Index)},
				KzgCommitment: xatuethv1.KzgCommitmentToString(e.data.KZGCommitment),
				VersionedHash: xatuethv1.VersionedHashToString(e.data.VersionedHash),
			},
		},
	}

	if e.beacon == nil {
		return decorated
	}

	var (
		blobSlot = e.beacon.GetSlot(uint64(e.data.Slot))
		epoch    = e.beacon.GetEpochFromSlot(uint64(e.data.Slot))
	)

	decorated.Meta.Client.AdditionalData = &xatu.ClientMeta_EthV1EventsBlobSidecar{
		EthV1EventsBlobSidecar: &xatu.ClientMeta_AdditionalEthV1EventsBlobSidecarData{
			Slot: &xatu.SlotV2{
				Number:        &wrapperspb.UInt64Value{Value: blobSlot.Number()},
				StartDateTime: timestamppb.New(blobSlot.TimeWindow().Start()),
			},
			Epoch: &xatu.EpochV2{
				Number:        &wrapperspb.UInt64Value{Value: epoch.Number()},
				StartDateTime: timestamppb.New(epoch.TimeWindow().Start()),
			},
			Propagation: &xatu.PropagationV2{
				SlotStartDiff: &wrapperspb.UInt64Value{
					//nolint:gosec // not concerned in reality
					Value: uint64(e.recvTime.Sub(blobSlot.TimeWindow().Start()).Milliseconds()),
				},
			},
		},
	}

	return decorated
}
