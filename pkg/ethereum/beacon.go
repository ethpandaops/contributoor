// Package ethereum provides Ethereum beacon node functionality
package ethereum

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/altair"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/contributoor/pkg/ethereum/services"
	"github.com/ethpandaops/contributoor/pkg/events"
	"github.com/ethpandaops/contributoor/pkg/sinks"
	"github.com/ethpandaops/ethwallclock"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// BeaconNode represents a connection to an Ethereum beacon node and manages any associated services (eg: metadata, etc).
type BeaconNode struct {
	config      *Config
	log         logrus.FieldLogger
	name        string
	beacon      beacon.Node
	services    []services.Service
	metadataSvc *services.MetadataService
	dutiesSvc   *services.DutiesService
	sinks       []sinks.ContributoorSink
}

// NewBeaconNode creates a new beacon node instance with the given configuration. It initializes any services and
// configures the beacon subscriptions.
func NewBeaconNode(
	log logrus.FieldLogger,
	config *Config,
	name string,
	sinks []sinks.ContributoorSink,
	opt *Options,
) (*BeaconNode, error) {
	// Set default options and disable prometheus metrics.
	opts := *beacon.DefaultOptions().DisablePrometheusMetrics()

	// Configure beacon subscriptions if provided, otherwise use defaults.
	if config.BeaconSubscriptions != nil {
		opts.BeaconSubscription = beacon.BeaconSubscriptionOptions{
			Enabled: true,
			Topics:  *config.BeaconSubscriptions,
		}
	} else {
		opts.EnableDefaultBeaconSubscription()
	}

	// Configure health check parameters.
	opts.HealthCheck.Interval.Duration = time.Second * 3
	opts.HealthCheck.SuccessfulResponses = 1

	// Create the beacon node.
	node := beacon.NewNode(log, &beacon.Config{
		Name:    name,
		Addr:    config.BeaconNodeAddress,
		Headers: config.BeaconNodeHeaders,
	}, "contributoor", opts)

	// Initialize services.
	metadata := services.NewMetadataService(log, node, config.OverrideNetworkName)
	duties := services.NewDutiesService(log, node, &metadata, true, true)

	return &BeaconNode{
		log:         log.WithField("module", "contributoor/ethereum/beacon"),
		config:      config,
		name:        name,
		beacon:      node,
		metadataSvc: &metadata,
		dutiesSvc:   &duties,
		sinks:       sinks,
		services:    []services.Service{&metadata, &duties},
	}, nil
}

// Start begins the beacon node operation and its services
// It waits for the node to become healthy before starting services.
func (b *BeaconNode) Start(ctx context.Context) error {
	var (
		s           = gocron.NewScheduler(time.Local)
		startupErrs = make(chan error, 1)
		beaconReady = make(chan struct{})
	)

	// Register callback for when the node becomes healthy.
	b.beacon.OnFirstTimeHealthy(ctx, func(ctx context.Context, event *beacon.FirstTimeHealthyEvent) error {
		b.log.Info("Upstream beacon node is healthy")

		close(beaconReady)

		if err := b.startServices(ctx, startupErrs); err != nil {
			return err
		}

		b.log.Info("All services are ready")

		b.log.Info("Setting up beacon node event subscriptions")

		// Set up event subscriptions.
		if err := b.setupSubscriptions(ctx); err != nil {
			startupErrs <- fmt.Errorf("failed to setup subscriptions: %w", err)
		}

		return nil
	})

	s.StartAsync()
	b.beacon.StartAsync(ctx)

	// Wait for the node to become healthy or timeout.
	select {
	case err := <-startupErrs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-beaconReady:
		// Beacon node is healthy, continue with normal operation.
	case <-time.After(10 * time.Minute):
		return errors.New("upstream beacon node is not healthy. check your configuration.")
	}

	// Continue monitoring for errors.
	select {
	case err := <-startupErrs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully shuts down the beacon node and its services.
func (b *BeaconNode) Stop(ctx context.Context) error {
	b.log.Info("Stopping beacon node")

	for _, service := range b.services {
		b.log.WithField("service", service.Name()).Info("Stopping service")

		if err := service.Stop(ctx); err != nil {
			b.log.WithError(err).WithField("service", service.Name()).Error("Failed to stop service")
		}
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

	syncState := status.SyncState()
	if syncState == nil {
		return errors.New("missing beacon node status sync state")
	}

	if syncState.SyncDistance > 3 {
		return errors.New("beacon node is not synced")
	}

	wallclock := b.metadataSvc.Wallclock()
	if wallclock == nil {
		return errors.New("missing wallclock")
	}

	currentSlot := wallclock.Slots().Current()

	if currentSlot.Number()-uint64(syncState.HeadSlot) > 32 {
		return fmt.Errorf("beacon node is too far behind head, head slot is %d, current slot is %d", syncState.HeadSlot, currentSlot.Number())
	}

	// Check if all services are ready
	for _, service := range b.services {
		if err := service.Ready(ctx); err != nil {
			return errors.Wrapf(err, "service %s is not ready", service.Name())
		}
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

// GetValidatorIndex returns the validator index for a given position in a committee.
func (b *BeaconNode) GetValidatorIndex(epoch phase0.Epoch, slot phase0.Slot, committeeIndex phase0.CommitteeIndex, position uint64) (phase0.ValidatorIndex, error) {
	return b.dutiesSvc.GetValidatorIndex(epoch, slot, committeeIndex, position)
}

func (b *BeaconNode) startServices(ctx context.Context, errs chan error) error {
	wg := sync.WaitGroup{}

	for _, service := range b.services {
		wg.Add(1)

		service.OnReady(ctx, func(ctx context.Context) error {
			b.log.WithField("service", service.Name()).Info("Service is ready")

			wg.Done()

			return nil
		})

		b.log.WithField("service", service.Name()).Info("Starting service")

		if err := service.Start(ctx); err != nil {
			errs <- fmt.Errorf("failed to start service: %w", err)
		}

		b.log.WithField("service", service.Name()).Info("Waiting for service to be ready")

		wg.Wait()
	}

	return nil
}

func (b *BeaconNode) setupSubscriptions(ctx context.Context) error {
	// Subscribe to attestations.
	b.beacon.OnAttestation(ctx, func(ctx context.Context, attestation *phase0.Attestation) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewAttestationEvent(b, meta, attestation, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to blocks.
	b.beacon.OnBlock(ctx, func(ctx context.Context, block *eth2v1.BlockEvent) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewBlockEvent(b, meta, block, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to chain reorgs.
	b.beacon.OnChainReOrg(ctx, func(ctx context.Context, chainReorg *eth2v1.ChainReorgEvent) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewChainReorgEvent(b, meta, chainReorg, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to head events.
	b.beacon.OnHead(ctx, func(ctx context.Context, head *eth2v1.HeadEvent) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewHeadEvent(b, meta, head, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to voluntary exits.
	b.beacon.OnVoluntaryExit(ctx, func(ctx context.Context, voluntaryExit *phase0.SignedVoluntaryExit) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewVoluntaryExitEvent(b, meta, voluntaryExit, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to contribution and proofs.
	b.beacon.OnContributionAndProof(ctx, func(ctx context.Context, contributionAndProof *altair.SignedContributionAndProof) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewContributionAndProofEvent(b, meta, contributionAndProof, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to finalized checkpoints.
	b.beacon.OnFinalizedCheckpoint(ctx, func(ctx context.Context, finalizedCheckpoint *eth2v1.FinalizedCheckpointEvent) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewFinalizedCheckpointEvent(b, meta, finalizedCheckpoint, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	// Subscribe to blob sidecars.
	b.beacon.OnBlobSidecar(ctx, func(ctx context.Context, blobSidecar *eth2v1.BlobSidecarEvent) error {
		meta, err := b.createEventMeta(ctx)
		if err != nil {
			return err
		}

		event := events.NewBlobSidecarEvent(b, meta, blobSidecar, time.Now())

		// Send directly to sinks.
		for _, sink := range b.sinks {
			if err := sink.HandleEvent(ctx, event); err != nil {
				b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to handle event")
			}
		}

		return nil
	})

	return nil
}

func (b *BeaconNode) createEventMeta(ctx context.Context) (*xatu.Meta, error) {
	var networkMeta *xatu.ClientMeta_Ethereum_Network

	network := b.metadataSvc.Network
	if network != nil {
		networkMeta = &xatu.ClientMeta_Ethereum_Network{
			Name: string(network.Name),
			Id:   network.ID,
		}

		if b.config.OverrideNetworkName != "" {
			networkMeta.Name = b.config.OverrideNetworkName
		}
	}

	clientName := b.name
	if clientName == "" {
		hashed, err := b.metadataSvc.NodeIDHash()
		if err != nil {
			return nil, err
		}

		clientName = hashed
	}

	// TODO(@matty):
	// - Handle Presets
	// - Handle Labels
	// - Handle Clock Drift

	return &xatu.Meta{
		Client: &xatu.ClientMeta{
			Name:           clientName,
			Version:        xatu.Short(),
			Id:             uuid.New().String(),
			Implementation: xatu.Implementation,
			ModuleName:     xatu.ModuleName_SENTRY,
			Os:             runtime.GOOS,
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
