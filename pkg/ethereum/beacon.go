// Package ethereum provides Ethereum beacon node functionality
package ethereum

import (
	"context"
	"fmt"
	"runtime"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/internal/contributoor"
	"github.com/ethpandaops/contributoor/internal/events"
	v1 "github.com/ethpandaops/contributoor/internal/events/v1"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/ethereum/services"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -package mock -destination mock/beacon_node.mock.go github.com/ethpandaops/contributoor/pkg/ethereum BeaconNodeAPI

// MaxReasonableSlotDifference is the maximum number of slots that can be
// between the event slot and the current slot before we consider the event
// to be from a different network.
const MaxReasonableSlotDifference uint64 = 10000

// BeaconNodeAPI is the interface for the BeaconNode.
type BeaconNodeAPI interface {
	// Start starts the beacon node. Returns a channel that will be closed when the node is healthy.
	Start(ctx context.Context) (chan struct{}, error)
	// Stop stops the beacon node.
	Stop(ctx context.Context) error
	// Synced checks if the beacon node is synced and ready.
	Synced(ctx context.Context) error
}

// BeaconNode represents a connection to an Ethereum beacon node and manages any associated services (eg: metadata, etc).
type BeaconNode struct {
	config      *Config
	log         logrus.FieldLogger
	beacon      beacon.Node
	clockDrift  clockdrift.ClockDrift
	metadataSvc *services.MetadataService
	sinks       []sinks.ContributoorSink
	cache       *events.DuplicateCache
	summary     *events.Summary
	metrics     *events.Metrics
	traceID     string
	healthy     bool
}

// NewBeaconNode creates a new beacon node instance with the given configuration. It initializes any services and
// configures the beacon subscriptions.
func NewBeaconNode(
	log logrus.FieldLogger,
	traceID string,
	config *Config,
	sinks []sinks.ContributoorSink,
	clockDrift clockdrift.ClockDrift,
	cache *events.DuplicateCache,
	summary *events.Summary,
	metrics *events.Metrics,
	opt *Options,
) (*BeaconNode, error) {
	// Give the ethpandaops/beacon a logger that is suppressed to warn level.
	// We'll handle any beacon specific info-level logging ourselves.
	beaconLogger := logrus.New()
	beaconLogger.SetLevel(logrus.WarnLevel)

	log = log.WithField("trace_id", traceID)

	// Set default options and disable prometheus metrics.
	opts := *beacon.DefaultOptions().DisablePrometheusMetrics()

	opts.BeaconSubscription = beacon.BeaconSubscriptionOptions{
		Enabled: true,
		Topics: []string{
			"block",
			"head",
			"finalized_checkpoint",
			"blob_sidecar",
			"chain_reorg",
		},
	}

	// Configure beacon subscriptions if provided, otherwise use defaults.
	if config.BeaconSubscriptions != nil {
		opts.BeaconSubscription = beacon.BeaconSubscriptionOptions{
			Enabled: true,
			Topics:  *config.BeaconSubscriptions,
		}
	}

	// Configure health check parameters.
	opts.HealthCheck.Interval.Duration = time.Second * 3
	opts.HealthCheck.SuccessfulResponses = 1

	// Create the beacon node.
	node := beacon.NewNode(beaconLogger, &beacon.Config{
		Addr:    config.BeaconNodeAddress,
		Headers: config.BeaconNodeHeaders,
	}, "contributoor", opts)

	// Initialize services.
	metadata := services.NewMetadataService(log, node, config.NetworkOverride)

	return &BeaconNode{
		log:         log,
		config:      config,
		beacon:      node,
		clockDrift:  clockDrift,
		metadataSvc: &metadata,
		sinks:       sinks,
		cache:       cache,
		summary:     summary,
		metrics:     metrics,
		traceID:     traceID,
	}, nil
}

// Start begins the beacon node operation and its services
// It returns a channel that will be closed when the node is healthy.
func (b *BeaconNode) Start(ctx context.Context) (chan struct{}, error) {
	var (
		startupErrs = make(chan error, 1)
		beaconReady = make(chan struct{})
	)

	// Register callback for when the node becomes healthy.
	b.beacon.OnFirstTimeHealthy(ctx, func(ctx context.Context, event *beacon.FirstTimeHealthyEvent) error {
		b.log.Debug("Upstream beacon node is healthy")

		b.healthy = true

		close(beaconReady)

		if err := b.startServices(ctx, startupErrs); err != nil {
			return err
		}

		// Set up event subscriptions.
		if err := b.setupSubscriptions(ctx); err != nil {
			startupErrs <- fmt.Errorf("failed to setup subscriptions: %w", err)
		}

		return nil
	})

	b.beacon.StartAsync(ctx)

	// Return the ready channel immediately for consumers, upon closing the channel
	// this indicates that the node is healthy and ready to process events.
	return beaconReady, nil
}

// Stop gracefully shuts down the beacon node and its services.
func (b *BeaconNode) Stop(ctx context.Context) error {
	b.log.Info("Stopping beacon node")
	b.healthy = false

	b.log.WithField("service", b.metadataSvc.Name()).Info("Stopping service")

	if err := b.metadataSvc.Stop(ctx); err != nil {
		b.log.WithError(err).WithField("service", b.metadataSvc.Name()).Error("Failed to stop service")
	}

	if err := b.beacon.Stop(ctx); err != nil {
		b.log.WithError(err).Error("Failed to stop beacon node")

		return fmt.Errorf("failed to stop beacon node: %w", err)
	}

	return nil
}

// Synced checks if the beacon node is synced and ready
// It verifies sync state, wallclock, and service readiness.
func (b *BeaconNode) Synced(ctx context.Context) error {
	status := b.beacon.Status()
	if status == nil {
		return errors.New("missing beacon status")
	}

	wallclock := b.metadataSvc.Wallclock()
	if wallclock == nil {
		return errors.New("missing wallclock")
	}

	if err := b.metadataSvc.Ready(ctx); err != nil {
		return errors.Wrapf(err, "service %s is not ready", b.metadataSvc.Name())
	}

	return nil
}

// Node returns the underlying beacon node instance.
func (b *BeaconNode) Node() beacon.Node {
	return b.beacon
}

// Metadata returns the metadata service instance.
func (b *BeaconNode) Metadata() *services.MetadataService {
	return b.metadataSvc
}

// GetWallclock returns the wallclock for the beacon chain.
func (b *BeaconNode) GetWallclock() *ethwallclock.EthereumBeaconChain {
	return b.metadataSvc.Wallclock()
}

// GetSlot returns the wallclock slot for a given slot number.
func (b *BeaconNode) GetSlot(slot uint64) ethwallclock.Slot {
	return b.metadataSvc.Wallclock().Slots().FromNumber(slot)
}

// GetEpoch returns the wallclock epoch for a given slot number.
func (b *BeaconNode) GetEpoch(epoch uint64) ethwallclock.Epoch {
	return b.metadataSvc.Wallclock().Epochs().FromNumber(epoch)
}

// GetEpochFromSlot returns the wallclock epoch for a given slot.
func (b *BeaconNode) GetEpochFromSlot(slot uint64) ethwallclock.Epoch {
	return b.metadataSvc.Wallclock().Epochs().FromSlot(slot)
}

// IsHealthy returns whether the node is healthy.
func (b *BeaconNode) IsHealthy() bool {
	return b.healthy
}

// IsSlotFromUnexpectedNetwork checks if a slot appears to be from an unexpected network
// by comparing it with the current wallclock slot.
func (b *BeaconNode) IsSlotFromUnexpectedNetwork(eventSlot uint64) bool {
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

func (b *BeaconNode) startServices(ctx context.Context, errs chan error) error {
	b.metadataSvc.OnReady(ctx, func(ctx context.Context) error {
		b.log.WithField("service", b.metadataSvc.Name()).Debug("Service is ready")

		hashed, err := b.metadataSvc.NodeIDHash()
		if err != nil {
			return err
		}

		b.log.WithFields(logrus.Fields{
			"node_id":    hashed,
			"network_id": b.metadataSvc.Network.ID,
			"network":    b.metadataSvc.Network.Name,
			"hash":       hashed,
		}).Info("Detected network and node ID hash")

		return nil
	})

	b.log.WithField("service", b.metadataSvc.Name()).Debug("Starting service")

	if err := b.metadataSvc.Start(ctx); err != nil {
		errs <- fmt.Errorf("failed to start service: %w", err)
	}

	b.log.WithField("service", b.metadataSvc.Name()).Debug("Waiting for service to be ready")

	return nil
}

func (b *BeaconNode) setupSubscriptions(ctx context.Context) error {
	b.log.WithField("topics", b.beacon.Options().BeaconSubscription.Topics).Info("Subscribing to events upstream")

	// Track events received.
	b.beacon.OnEvent(ctx, func(ctx context.Context, event *eth2v1.Event) error {
		b.summary.AddEventStreamEvents(event.Topic, 1)

		return nil
	})

	// Subscribe to blocks.
	b.beacon.OnBlock(ctx, func(ctx context.Context, block *eth2v1.BlockEvent) error {
		now := b.clockDrift.Now()

		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewBlockEvent(b.log, b, b.cache.BeaconETHV1EventsBlock, meta, block, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return b.handleDecoratedEvent(ctx, event)
	})

	// Subscribe to chain reorgs.
	b.beacon.OnChainReOrg(ctx, func(ctx context.Context, chainReorg *eth2v1.ChainReorgEvent) error {
		now := b.clockDrift.Now()

		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewChainReorgEvent(b.log, b, b.cache.BeaconETHV1EventsChainReorg, meta, chainReorg, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return b.handleDecoratedEvent(ctx, event)
	})

	// Subscribe to head events.
	b.beacon.OnHead(ctx, func(ctx context.Context, head *eth2v1.HeadEvent) error {
		now := b.clockDrift.Now()

		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewHeadEvent(b.log, b, b.cache.BeaconETHV1EventsHead, meta, head, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return b.handleDecoratedEvent(ctx, event)
	})

	// Subscribe to finalized checkpoints.
	b.beacon.OnFinalizedCheckpoint(ctx, func(ctx context.Context, finalizedCheckpoint *eth2v1.FinalizedCheckpointEvent) error {
		now := b.clockDrift.Now()

		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewFinalizedCheckpointEvent(b.log, b, b.cache.BeaconETHV1EventsFinalizedCheckpoint, meta, finalizedCheckpoint, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return b.handleDecoratedEvent(ctx, event)
	})

	// Subscribe to blob sidecars.
	b.beacon.OnBlobSidecar(ctx, func(ctx context.Context, blobSidecar *eth2v1.BlobSidecarEvent) error {
		now := b.clockDrift.Now()

		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := v1.NewBlobSidecarEvent(b.log, b, b.cache.BeaconETHV1EventsBlobSidecar, meta, blobSidecar, now)

		ignore, err := event.Ignore(ctx)
		if err != nil || ignore {
			if err != nil {
				return err
			}

			return nil
		}

		return b.handleDecoratedEvent(ctx, event)
	})

	return nil
}

func (b *BeaconNode) createEventMeta(ctx context.Context) (*xatu.Meta, error) {
	var networkMeta *xatu.ClientMeta_Ethereum_Network

	network := b.metadataSvc.Network
	if network == nil {
		return nil, errors.New("network is unknown")
	}

	networkMeta = &xatu.ClientMeta_Ethereum_Network{
		Name: string(network.Name),
		Id:   network.ID,
	}

	hashed, err := b.metadataSvc.NodeIDHash()
	if err != nil {
		return nil, err
	}

	// TODO(@matty):
	// - Handle Labels

	//nolint:gosec // fine for clock drift.
	return &xatu.Meta{
		Client: &xatu.ClientMeta{
			Name:           hashed,
			Version:        contributoor.Short(),
			Id:             uuid.New().String(),
			Implementation: contributoor.Implementation,
			ModuleName:     contributoor.Module,
			Os:             runtime.GOOS,
			ClockDrift:     uint64(b.clockDrift.GetDrift().Milliseconds()),
			Ethereum: &xatu.ClientMeta_Ethereum{
				Network:   networkMeta,
				Execution: &xatu.ClientMeta_Ethereum_Execution{},
				Consensus: &xatu.ClientMeta_Ethereum_Consensus{
					Implementation: b.metadataSvc.Client(ctx),
					Version:        b.metadataSvc.NodeVersion(ctx),
				},
			},
		},
	}, nil
}

func (b *BeaconNode) handleDecoratedEvent(ctx context.Context, event events.Event) error {
	if err := b.Synced(ctx); err != nil {
		return err
	}

	var (
		unknown    = "unknown"
		network    = event.Meta().GetClient().GetEthereum().GetNetwork().GetId()
		networkStr = fmt.Sprintf("%d", network)
		eventType  = event.Type()
		failure    = false
	)

	if networkStr == "" || networkStr == "0" {
		networkStr = unknown
	}

	if eventType == "" {
		eventType = unknown
	}

	b.metrics.AddDecoratedEvent(1, eventType, networkStr)
	b.summary.AddEventsExported(1)

	// Send to all sinks.
	for _, sink := range b.sinks {
		if err := sink.HandleEvent(ctx, event); err != nil {
			b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")

			failure = true

			continue
		}
	}

	if failure {
		b.summary.AddFailedEvents(1)
	}

	return nil
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
