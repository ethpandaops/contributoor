package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// beaconComponents holds all the components needed for a beacon instance.
type beaconComponents struct {
	cache   *events.DuplicateCache
	metrics *events.Metrics
	summary *events.Summary
	sinks   []sinks.ContributoorSink
}

// initBeacons initializes all beacon node instances from the configuration.
func (a *Application) initBeacons(ctx context.Context) error {
	addresses := strings.Split(a.config.BeaconNodeAddress, ",")

	traceIDs, err := generateBeaconTraceIDs(addresses)
	if err != nil {
		return fmt.Errorf("failed to generate trace IDs: %w", err)
	}

	a.beaconNodes = make(map[string]*BeaconNodeInstance)

	a.log.WithFields(logrus.Fields{
		"count":     len(addresses),
		"trace_ids": traceIDs,
		"addresses": addresses,
	}).Info("Initializing beacons")

	for i, address := range addresses {
		address = strings.TrimSpace(address)
		traceID := traceIDs[i]

		logCtx := a.log.WithField("trace_id", traceID)

		instance, err := a.createBeaconInstance(ctx, logCtx, address, traceID, nil)
		if err != nil {
			return fmt.Errorf("failed to create beacon instance: %w", err)
		}

		a.beaconNodes[traceID] = instance
	}

	return nil
}

// createBeaconInstance creates a single beacon node instance with all its components.
func (a *Application) createBeaconInstance(
	ctx context.Context,
	log logrus.FieldLogger,
	address, traceID string,
	excludedTopics []string,
) (*BeaconNodeInstance, error) {
	// Create components.,
	components, err := a.createBeaconComponents(ctx, log, traceID)
	if err != nil {
		return nil, err
	}

	// Create beacon configuration.
	config := a.createBeaconConfig(address)

	// Create and configure topic manager.
	topicManager, err := a.createTopicManager(ctx, log, config)
	if err != nil {
		return nil, err
	}

	// Ensure beacon factory exists
	if a.beaconFactory == nil {
		a.beaconFactory = ethereum.NewBeaconFactory(log, a.clockDrift)
	}

	// Create beacon using factory
	beaconOpts := &ethereum.BeaconOptions{
		TraceID:       traceID,
		Config:        config,
		Sinks:         components.sinks,
		Cache:         components.cache,
		Summary:       components.summary,
		Metrics:       components.metrics,
		TopicManager:  topicManager,
		ExcludeTopics: excludedTopics,
	}

	node, err := a.beaconFactory.CreateBeacon(ctx, beaconOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create beacon: %w", err)
	}

	return &BeaconNodeInstance{
		Node:          node,
		Cache:         components.cache,
		Sinks:         components.sinks,
		Metrics:       components.metrics,
		Summary:       components.summary,
		Address:       address,
		TopicManager:  topicManager,
		log:           log,
		traceID:       traceID,
		app:           a,
		stopMonitor:   make(chan struct{}),
		summaryCancel: nil, // Will be set when the summary starts.
	}, nil
}

// createBeaconComponents creates all the necessary parts for a beacon instance.
func (a *Application) createBeaconComponents(ctx context.Context, log logrus.FieldLogger, traceID string) (*beaconComponents, error) {
	cache, err := a.initCache()
	if err != nil {
		return nil, fmt.Errorf("failed to init cache: %w", err)
	}

	metrics, err := a.initMetrics(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init metrics: %w", err)
	}

	summary, err := a.initSummary(log, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init summary: %w", err)
	}

	allSinks, err := a.initSinks(ctx, log, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init sinks: %w", err)
	}

	return &beaconComponents{
		cache:   cache,
		metrics: metrics,
		summary: summary,
		sinks:   allSinks,
	}, nil
}

// createBeaconConfig creates the configuration for a beacon node.
func (a *Application) createBeaconConfig(address string) *ethereum.Config {
	config := ethereum.NewDefaultConfig()
	config.BeaconNodeAddress = address

	if a.config.NetworkName != "" {
		config.NetworkOverride = a.config.NetworkName
	}

	// Apply attestation subnet configuration if present.
	if a.config.AttestationSubnetCheck != nil {
		config.AttestationSubnetConfig.Enabled = a.config.AttestationSubnetCheck.Enabled
		config.AttestationSubnetConfig.MaxSubnets = 2

		if int(a.config.AttestationSubnetCheck.MaxSubnets) != 0 {
			config.AttestationSubnetConfig.MaxSubnets = int(a.config.AttestationSubnetCheck.MaxSubnets)
		}
	}

	return config
}

// createTopicManager creates and configures a topic manager for the beacon node.
func (a *Application) createTopicManager(ctx context.Context, log logrus.FieldLogger, config *ethereum.Config) (ethereum.TopicManager, error) {
	topicManager := ethereum.NewTopicManager(log, &ethereum.TopicConfig{
		AllTopics:               ethereum.GetDefaultAllTopics(),
		OptInTopics:             ethereum.GetOptInTopics(),
		AttestationEnabled:      config.AttestationSubnetConfig.Enabled,
		AttestationMaxSubnets:   config.AttestationSubnetConfig.MaxSubnets,
		MismatchDetectionWindow: config.AttestationSubnetConfig.MismatchDetectionWindow,
		MismatchThreshold:       config.AttestationSubnetConfig.MismatchThreshold,
		MismatchCooldown:        time.Duration(config.AttestationSubnetConfig.MismatchCooldownSeconds) * time.Second,
		SubnetHighWaterMark:     config.AttestationSubnetConfig.SubnetHighWaterMark,
	})

	// Check for attestation subnet participation if enabled
	if config.AttestationSubnetConfig.Enabled {
		identity := ethereum.NewNodeIdentity(log, config.BeaconNodeAddress, config.BeaconNodeHeaders)
		if err := identity.Start(ctx); err != nil {
			log.WithError(err).Warn("Failed to fetch node identity")
		} else {
			activeSubnets := identity.GetAttnets()
			topicManager.RegisterCondition(
				ethereum.TopicSingleAttestation,
				ethereum.CreateAttestationSubnetCondition(len(activeSubnets), config.AttestationSubnetConfig.MaxSubnets),
			)
			topicManager.SetAdvertisedSubnets(activeSubnets)
		}
	}

	return topicManager, nil
}

// initCache creates a new duplicate event cache.
func (a *Application) initCache() (*events.DuplicateCache, error) {
	return events.NewDuplicateCache(), nil
}

// initMetrics creates a new metrics instance for a beacon node.
func (a *Application) initMetrics(traceID string) (*events.Metrics, error) {
	return events.NewMetrics(
		strings.ReplaceAll(fmt.Sprintf("contributoor_%s", traceID), "-", "_"),
	), nil
}

// initSummary creates a new summary logger for a beacon node.
func (a *Application) initSummary(log logrus.FieldLogger, traceID string) (*events.Summary, error) {
	return events.NewSummary(log, traceID, 10*time.Second), nil
}

// RestartWithoutSingleAttestation restarts the beacon node without the single_attestation topic.
// This method ensures proper cleanup of the old beacon instance before creating a new one.
// Resources that are cleaned up:
// - Summary goroutine (cancelled via summaryCancel).
// - Monitoring goroutines (signaled via stopMonitor channel).
// - Beacon node connection (stopped via Node.Stop()).
// - Old Metrics and Summary instances (replaced with new ones).
// Resources that are reused:
// - Cache (shared across restarts).
// - Sinks (shared across restarts).
// - Log instance.
func (b *BeaconNodeInstance) RestartWithoutSingleAttestation(ctx context.Context) error {
	b.reconnectMutex.Lock()
	defer b.reconnectMutex.Unlock()

	// Check cooldown period from TopicManager configuration
	cooldownPeriod := 5 * time.Minute // default fallback
	if b.TopicManager != nil {
		cooldownPeriod = b.TopicManager.GetCooldownPeriod()
	}

	if time.Since(b.lastReconnect) < cooldownPeriod {
		b.log.Debug("Skipping reconnection due to cooldown period")

		return nil
	}

	b.log.Warn("Restarting beacon")

	// Cancel the summary goroutine if it's running
	if b.summaryCancel != nil {
		b.summaryCancel()
		b.log.Debug("Cancelled old summary goroutine")
	}

	// Signal monitoring goroutines to stop
	close(b.stopMonitor)

	// Stop the current beacon node (this will also stop all sinks via BeaconWrapper.Stop())
	if err := b.Node.Stop(ctx); err != nil {
		b.log.WithError(err).Error("Failed to stop beacon node")
	}

	// Important: Set old node to nil to help GC and prevent accidental reuse
	oldNode := b.Node
	oldMetrics := b.Metrics
	oldSummary := b.Summary
	b.Node = nil
	b.Metrics = nil
	b.Summary = nil

	// Create a new beacon node without single_attestation.
	// Use a modified traceID to avoid metrics collision.
	newTraceID := fmt.Sprintf("%s-nosub", b.traceID)
	b.log = b.log.WithField("trace_id", newTraceID)

	// Exclude single_attestation topic when creating new beacon.
	excludedTopics := []string{ethereum.TopicSingleAttestation}

	// Create new beacon instance with excluded topics.
	newInstance, err := b.app.createBeaconInstance(ctx, b.log, b.Address, newTraceID, excludedTopics)
	if err != nil {
		b.log.WithError(err).Error("Failed to create new beacon instance")

		return fmt.Errorf("failed to create new beacon instance: %w", err)
	}

	// Start the new beacon node.
	if err := newInstance.Node.Start(ctx); err != nil {
		b.log.WithError(err).Error("Failed to start new beacon node")

		return fmt.Errorf("failed to start new beacon node: %w", err)
	}

	// Replace the node reference and update components.
	b.Node = newInstance.Node
	b.Metrics = newInstance.Metrics
	b.Summary = newInstance.Summary
	b.TopicManager = newInstance.TopicManager
	b.Sinks = newInstance.Sinks
	b.Cache = newInstance.Cache
	b.traceID = newTraceID
	b.lastReconnect = time.Now()

	// Create new stopMonitor channel for the new instance.
	b.stopMonitor = make(chan struct{})
	b.summaryCancel = nil // Will be set when summary starts.

	// Clean up old components that are no longer needed.
	// The old Node, Metrics, and Summary have been replaced.
	// The old Sinks have been stopped and replaced with new ones.
	_ = oldNode
	_ = oldMetrics
	_ = oldSummary

	// Restart monitoring goroutine for the new instance.
	go b.app.monitorBeaconInstance(ctx, b)

	b.log.Info("Restarted beacon node successfully")

	return nil
}

// generateBeaconTraceIDs generates unique trace IDs for beacon nodes based on their addresses.
func generateBeaconTraceIDs(addresses []string) ([]string, error) {
	if len(addresses) == 0 {
		return nil, errors.New("no addresses provided")
	}

	traceIDs := make([]string, len(addresses))
	uniqueIDs := make(map[string]bool)

	for i, address := range addresses {
		// Generate a hash of the address
		hash := sha256.Sum256([]byte(address))
		baseID := base64.URLEncoding.EncodeToString(hash[:])[:8]

		// Ensure uniqueness
		id := baseID
		counter := 1

		for uniqueIDs[id] {
			id = fmt.Sprintf("%s-%d", baseID, counter)
			counter++
		}

		uniqueIDs[id] = true
		traceIDs[i] = id
	}

	return traceIDs, nil
}
