package ethereum

import (
	"context"
	"fmt"
	"runtime"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/contributoor/internal/clockdrift"
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

	log        logrus.FieldLogger
	traceID    string
	clockDrift clockdrift.ClockDrift
	sinks      []sinks.ContributoorSink
	cache      *events.DuplicateCache
	summary    *events.Summary
	metrics    *events.Metrics
}

// NewBeaconWrapper creates a wrapped beacon node.
func NewBeaconWrapper(
	log logrus.FieldLogger,
	traceID string,
	config *Config,
	sinks []sinks.ContributoorSink,
	clockDrift clockdrift.ClockDrift,
	cache *events.DuplicateCache,
	summary *events.Summary,
	metrics *events.Metrics,
) (*BeaconWrapper, error) {
	// Prepare ethcore config.
	ethcoreConfig := &ethcore.Config{
		BeaconNodeAddress: config.BeaconNodeAddress,
		BeaconNodeHeaders: config.BeaconNodeHeaders,
		NetworkOverride:   config.NetworkOverride,
	}

	beaconOpts := &ethcore.Options{Options: beacon.DefaultOptions()}
	beaconOpts.BeaconSubscription.Enabled = true
	beaconOpts.BeaconSubscription.Topics = []string{
		"block",
		"block_gossip",
		"head",
		"finalized_checkpoint",
		"blob_sidecar",
		"chain_reorg",
	}

	// Create the beacon node.
	ethcoreBeacon, err := ethcore.NewBeaconNode(log, traceID, ethcoreConfig, beaconOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create ethcore beacon: %w", err)
	}

	wrapper := &BeaconWrapper{
		BeaconNode: ethcoreBeacon,
		log:        log,
		traceID:    traceID,
		clockDrift: clockDrift,
		sinks:      sinks,
		cache:      cache,
		summary:    summary,
		metrics:    metrics,
	}

	// Register OnReady callback to setup event subscriptions.
	ethcoreBeacon.OnReady(wrapper.setupEventSubscriptions)

	return wrapper, nil
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
	return w.BeaconNode.Start(ctx)
}

// Stop to handle sink lifecycle.
func (w *BeaconWrapper) Stop(ctx context.Context) error {
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
func (w *BeaconWrapper) setupEventSubscriptions(ctx context.Context) error {
	node := w.Node()

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

	w.log.Info("Event subscriptions setup successfully")

	return nil
}

// createEventMeta creates Xatu metadata for events.
func (w *BeaconWrapper) createEventMeta(ctx context.Context) (*xatu.Meta, error) {
	metadata := w.Metadata()
	network := metadata.GetNetwork()

	return &xatu.Meta{
		Client: &xatu.ClientMeta{
			Name:           metadata.GetClient(ctx),
			Version:        metadata.GetNodeVersion(ctx),
			Id:             w.traceID,
			Implementation: metadata.GetClient(ctx),
			Os:             runtime.GOOS,
			ClockDrift:     uint64(w.clockDrift.GetDrift().Milliseconds()), //nolint:gosec // ok.
			Ethereum: &xatu.ClientMeta_Ethereum{
				Network: &xatu.ClientMeta_Ethereum_Network{
					Name: string(network.Name),
					Id:   network.ID,
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

	// Send to all sinks
	failure := false

	for _, sink := range w.sinks {
		if err := sink.HandleEvent(ctx, event); err != nil {
			failure = true

			continue
		}
	}

	// Update metrics and summary
	w.metrics.AddDecoratedEvent(1, event.Type(), string(w.Metadata().GetNetwork().Name))
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
