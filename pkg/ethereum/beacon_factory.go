package ethereum

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/internal/sinks"
	ethcore "github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/sirupsen/logrus"
)

// BeaconFactory creates beacon instances with consistent configuration.
type BeaconFactory struct {
	log        logrus.FieldLogger
	clockDrift clockdrift.ClockDrift
}

// BeaconOptions contains all parameters for beacon creation.
type BeaconOptions struct {
	TraceID       string
	Config        *Config
	Sinks         []sinks.ContributoorSink
	Cache         *events.DuplicateCache
	Summary       *events.Summary
	Metrics       *events.Metrics
	TopicManager  TopicManager
	ExcludeTopics []string
}

// NewBeaconFactory creates a new beacon factory.
func NewBeaconFactory(log logrus.FieldLogger, clockDrift clockdrift.ClockDrift) *BeaconFactory {
	return &BeaconFactory{
		log:        log,
		clockDrift: clockDrift,
	}
}

// CreateBeacon creates a new beacon instance with the given options.
func (bf *BeaconFactory) CreateBeacon(ctx context.Context, opts *BeaconOptions) (*BeaconWrapper, error) {
	// Prepare ethcore config
	ethcoreConfig := &ethcore.Config{
		BeaconNodeAddress: opts.Config.BeaconNodeAddress,
		BeaconNodeHeaders: opts.Config.BeaconNodeHeaders,
		NetworkOverride:   opts.Config.NetworkOverride,
	}

	// Apply excluded topics to topic manager
	for _, topic := range opts.ExcludeTopics {
		opts.TopicManager.ExcludeTopic(topic)
	}

	// Create ethcore beacon node
	beaconOpts := &ethcore.Options{Options: beacon.DefaultOptions()}
	beaconOpts.BeaconSubscription.Enabled = true
	beaconOpts.BeaconSubscription.Topics = opts.TopicManager.GetEnabledTopics(ctx)

	ethcoreBeacon, err := ethcore.NewBeaconNode(bf.log, opts.TraceID, ethcoreConfig, beaconOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create ethcore beacon: %w", err)
	}

	// Create wrapper
	wrapper := &BeaconWrapper{
		BeaconNode:   ethcoreBeacon,
		log:          bf.log,
		traceID:      opts.TraceID,
		clockDrift:   bf.clockDrift,
		config:       opts.Config,
		sinks:        opts.Sinks,
		cache:        opts.Cache,
		summary:      opts.Summary,
		metrics:      opts.Metrics,
		topicManager: opts.TopicManager,
	}

	// Initialize as unhealthy (false) since the node starts disconnected
	wrapper.isHealthy.Store(false)

	// Log subnet mismatch detection status
	if opts.Config.SubnetMismatchDetection != nil && opts.Config.SubnetMismatchDetection.Enabled {
		bf.log.WithFields(logrus.Fields{
			"detection_window":   opts.Config.SubnetMismatchDetection.DetectionWindow,
			"mismatch_threshold": opts.Config.SubnetMismatchDetection.MismatchThreshold,
			"cooldown_period":    time.Duration(opts.Config.SubnetMismatchDetection.CooldownSeconds) * time.Second,
		}).Info("Subnet mismatch detection enabled")
	} else {
		bf.log.Info("Subnet mismatch detection disabled")
	}

	// Setup event subscriptions on ready
	ethcoreBeacon.OnReady(wrapper.setupEventSubscriptions)

	return wrapper, nil
}
