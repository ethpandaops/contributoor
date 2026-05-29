package ethereum

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/electra"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/internal/contributoor"
	"github.com/ethpandaops/contributoor/internal/events"
	v1 "github.com/ethpandaops/contributoor/internal/events/v1"
	"github.com/ethpandaops/contributoor/internal/sinks"
	ethcore "github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -package mock -destination mock/beacon_node.mock.go github.com/ethpandaops/contributoor/pkg/ethereum BeaconNodeAPI

// MaxReasonableSlotDifference is the maximum number of slots that can be
// between the event slot and the current slot before we consider the event
// to be from a different network.
const MaxReasonableSlotDifference uint64 = 10000

// BeaconNodeAPI is the interface for the BeaconNode.
type BeaconNodeAPI interface {
	// Start starts the beacon node and blocks until it is healthy.
	Start(ctx context.Context) error
	// Stop stops the beacon node.
	Stop(ctx context.Context) error
	// Synced checks if the beacon node is synced and ready.
	Synced(ctx context.Context) error
}

// BeaconWrapper wraps ethcore beacon with additional contributoor functionality.
type BeaconWrapper struct {
	*ethcore.BeaconNode // Embed ethcore beacon

	log          logrus.FieldLogger
	traceID      string
	clockDrift   clockdrift.ClockDrift
	sinks        []sinks.ContributoorSink
	cache        *events.DuplicateCache
	config       *Config
	summary      *events.Summary
	metrics      *events.Metrics
	isHealthy    atomic.Bool // Track connection state for transition logging
	topicManager TopicManager
}

// Start to handle sink lifecycle.
// This method now blocks until the beacon node is ready or an error occurs.
func (w *BeaconWrapper) Start(ctx context.Context) error {
	// Start sinks first.
	for _, sink := range w.sinks {
		if err := sink.Start(ctx); err != nil {
			return fmt.Errorf("failed to start sink %s: %w", sink.Name(), err)
		}
	}

	// The upstream Start() is now blocking and waits until the node is ready.
	if err := w.BeaconNode.Start(ctx); err != nil {
		return err
	}

	// Start subnet refresh if attestation subnet tracking is enabled
	if w.config.AttestationSubnetConfig.Enabled && w.topicManager != nil {
		// Create a fetcher function that gets the current attnets
		fetcher := func() []int {
			identity := NewNodeIdentity(w.log, w.config.BeaconNodeAddress, w.config.BeaconNodeHeaders)
			if err := identity.Start(ctx); err != nil {
				w.log.WithError(err).Debug("Failed to fetch node identity during refresh")

				return nil
			}

			return identity.GetAttnets()
		}

		// Start refreshing every 30 seconds
		w.topicManager.StartSubnetRefresh(ctx, 30*time.Second, fetcher)
	}

	return nil
}

// Stop to handle sink lifecycle.
func (w *BeaconWrapper) Stop(ctx context.Context) error {
	// Mark as unhealthy to prevent health check logs
	w.isHealthy.Store(false)

	// Stop subnet refresh if it's running
	if w.topicManager != nil {
		w.topicManager.StopSubnetRefresh()
	}

	// Stop upstream first.
	if err := w.BeaconNode.Stop(ctx); err != nil {
		w.log.WithError(err).Error("Failed to stop beacon node")
	}

	// Stop contributoor sinks.
	for _, sink := range w.sinks {
		if err := sink.Stop(ctx); err != nil {
			w.log.WithError(err).Errorf("Failed to stop sink %s", sink.Name())
		}
	}

	return nil
}

// setupEventSubscriptions is fired when beacon becomes healthy.
//
//nolint:gocyclo // splitting would add further complexity.
func (w *BeaconWrapper) setupEventSubscriptions(ctx context.Context) error {
	node := w.Node()

	// Set initial state to true.
	w.isHealthy.Store(true)

	node.OnHealthCheckFailed(ctx, func(ctx context.Context, event *beacon.HealthCheckFailedEvent) error {
		// Only log if we're still healthy (this filters out messages from stopped beacons)
		if w.isHealthy.Load() {
			w.log.WithField("trace_id", w.traceID).Warn("Beacon node connection lost")
		}

		// Update state
		w.isHealthy.Store(false)

		return nil
	})

	node.OnHealthCheckSucceeded(ctx, func(ctx context.Context, event *beacon.HealthCheckSucceededEvent) error {
		// Only log when transitioning from unhealthy to healthy.
		if w.isHealthy.CompareAndSwap(false, true) {
			w.log.WithField("trace_id", w.traceID).Info("Beacon node connection restored")

			// Upon reconnection, check if the node's attestation subnets have changed
			if w.config.AttestationSubnetConfig.Enabled && w.topicManager != nil {
				identity := NewNodeIdentity(w.log, w.config.BeaconNodeAddress, w.config.BeaconNodeHeaders)
				if err := identity.Start(ctx); err != nil {
					w.log.WithError(err).Warn("Failed to fetch node identity on reconnection")
				} else {
					newSubnets := identity.GetAttnets()

					// Update the topic manager with the current subnets
					// If these differ from what the beacon was previously advertising,
					// the mismatch detection will catch it when we receive attestations
					// from the old subnets that are no longer advertised
					w.topicManager.SetAdvertisedSubnets(newSubnets)
				}
			}
		}

		return nil
	})

	node.OnEvent(ctx, func(ctx context.Context, event *eth2v1.Event) error {
		w.summary.AddEventStreamEvents(event.Topic, 1)

		return nil
	})

	node.OnBlock(ctx, func(ctx context.Context, block *eth2v1.BlockEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewBlockEvent(w.log, w, w.cache.BeaconETHV1EventsBlock, meta, block, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnBlockGossip(ctx, func(ctx context.Context, blockGossip *eth2v1.BlockGossipEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewBlockGossipEvent(w.log, w, w.cache.BeaconETHV1EventsBlockGossip, meta, blockGossip, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnHead(ctx, func(ctx context.Context, head *eth2v1.HeadEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewHeadEvent(w.log, w, w.cache.BeaconETHV1EventsHead, meta, head, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnFinalizedCheckpoint(ctx, func(ctx context.Context, checkpoint *eth2v1.FinalizedCheckpointEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewFinalizedCheckpointEvent(w.log, w, w.cache.BeaconETHV1EventsFinalizedCheckpoint, meta, checkpoint, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnChainReOrg(ctx, func(ctx context.Context, reorg *eth2v1.ChainReorgEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewChainReorgEvent(w.log, w, w.cache.BeaconETHV1EventsChainReorg, meta, reorg, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnBlobSidecar(ctx, func(ctx context.Context, blob *eth2v1.BlobSidecarEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewBlobSidecarEvent(w.log, w, w.cache.BeaconETHV1EventsBlobSidecar, meta, blob, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnDataColumnSidecar(ctx, func(ctx context.Context, dataColumn *eth2v1.DataColumnSidecarEvent) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewDataColumnSidecarEvent(w.log, w, w.cache.BeaconETHV1EventsDataColumnSidecar, meta, dataColumn, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnSingleAttestation(ctx, func(ctx context.Context, attestation *electra.SingleAttestation) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewSingleAttestationEvent(w.log, w, w.cache.BeaconETHV1EventsAttestationV2, meta, attestation, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	node.OnAttestation(ctx, func(ctx context.Context, attestation *spec.VersionedAttestation) error {
		now := w.clockDrift.Now()

		meta, err := w.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewAggregateAttestationEvent(w.log, w, w.cache.BeaconETHV1EventsAttestationV2, meta, attestation, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return w.handleDecoratedEvent(ctx, event)
	})

	w.log.Info("Event subscriptions setup successfully")

	return nil
}

// createEventMeta creates Xatu metadata for events.
func (w *BeaconWrapper) createEventMeta(ctx context.Context) (*xatu.Meta, error) {
	var (
		metadata = w.Metadata()
		network  = metadata.GetNetwork()
	)

	hashedNodeID, err := metadata.GetNodeIDHash()
	if err != nil {
		return nil, err
	}

	return &xatu.Meta{
		Client: &xatu.ClientMeta{
			Name:           hashedNodeID,
			Version:        contributoor.Short(),
			Id:             w.traceID,
			Implementation: contributoor.Implementation,
			ModuleName:     contributoor.Module,
			Os:             runtime.GOOS,
			ClockDrift:     uint64(w.clockDrift.GetDrift().Milliseconds()), //nolint:gosec // ok.
			Ethereum: &xatu.ClientMeta_Ethereum{
				Network: &xatu.ClientMeta_Ethereum_Network{
					Name: string(network.Name),
					Id:   network.ID,
				},
				Execution: &xatu.ClientMeta_Ethereum_Execution{},
				Consensus: &xatu.ClientMeta_Ethereum_Consensus{
					Implementation: metadata.GetClient(ctx),
					Version:        metadata.GetNodeVersion(ctx),
				},
			},
		},
	}, nil
}

// handleDecoratedEvent processes events through sinks.
func (w *BeaconWrapper) handleDecoratedEvent(ctx context.Context, event events.Event) error {
	// Final sync check
	if err := w.Synced(ctx); err != nil {
		return err
	}

	var (
		failure   = false
		eventType = "unknown"
	)

	if event.Type() != "" {
		eventType = event.Type()
	}

	for _, sink := range w.sinks {
		if err := sink.HandleEvent(ctx, event); err != nil {
			w.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")

			failure = true

			continue
		}
	}

	// Update metrics and summary
	w.metrics.AddDecoratedEvent(1, eventType, string(w.Metadata().GetNetwork().Name))
	w.summary.AddEventsExported(1)

	if failure {
		w.summary.AddFailedEvents(1)
	}

	return nil
}

// GetSlot returns the wallclock slot for a given slot number.
func (w *BeaconWrapper) GetSlot(slot uint64) ethwallclock.Slot {
	return w.BeaconNode.GetSlot(slot)
}

// GetEpoch returns the wallclock epoch for a given epoch number.
func (w *BeaconWrapper) GetEpoch(epoch uint64) ethwallclock.Epoch {
	return w.BeaconNode.GetEpoch(epoch)
}

// GetEpochFromSlot returns the wallclock epoch for a given slot number.
func (w *BeaconWrapper) GetEpochFromSlot(slot uint64) ethwallclock.Epoch {
	return w.BeaconNode.GetEpochFromSlot(slot)
}

// IsSlotFromUnexpectedNetwork checks if a slot appears to be from an unexpected network
// by comparing it with the current wallclock slot.
func (b *BeaconWrapper) IsSlotFromUnexpectedNetwork(eventSlot uint64) bool {
	wallclock := b.GetWallclock()
	if wallclock == nil {
		// Can't verify without wallclock.
		return false
	}

	// Get current slot from wallclock.
	currentSlot, _, err := wallclock.Now()
	if err != nil {
		return false
	}

	return isSlotDifferenceTooLarge(eventSlot, currentSlot.Number())
}

// IsActiveSubnet checks if the given subnet ID is in the node's active subnets.
func (w *BeaconWrapper) IsActiveSubnet(subnetID uint64) bool {
	if w.topicManager == nil {
		return false
	}

	return w.topicManager.IsActiveSubnet(subnetID)
}

// RecordSeenSubnet records that we've seen an attestation from a specific subnet.
func (w *BeaconWrapper) RecordSeenSubnet(subnetID uint64, slot uint64) {
	if w.topicManager == nil {
		return
	}

	w.topicManager.RecordAttestation(subnetID, phase0.Slot(slot))
}

// NeedsReconnection returns a channel that signals when reconnection is needed.
func (w *BeaconWrapper) NeedsReconnection() <-chan struct{} {
	if w.topicManager == nil {
		return nil
	}

	return w.topicManager.NeedsReconnection()
}

// GetTopicManager returns the topic manager for this beacon wrapper.
func (w *BeaconWrapper) GetTopicManager() TopicManager {
	return w.topicManager
}

// isSlotDifferenceTooLarge checks if the difference between two slots exceeds the
// maximum reasonable difference threshold, indicating they might be from different networks.
// This helper function is extracted for better testability.
func isSlotDifferenceTooLarge(slotA, slotB uint64) bool {
	// Calculate absolute difference.
	var slotDiff uint64
	if slotA > slotB {
		slotDiff = slotA - slotB
	} else {
		slotDiff = slotB - slotA
	}

	// If slot difference is greater than MaxReasonableSlotDifference,
	// it's likely from a different network.
	return slotDiff > MaxReasonableSlotDifference
}
