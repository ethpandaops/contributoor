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
	rawAddresses := strings.Split(a.config.BeaconNodeAddress, ",")

	addresses := make([]string, len(rawAddresses))
	seen := make(map[string]bool, len(rawAddresses))

	for i, address := range rawAddresses {
		address = strings.TrimSpace(address)
		addresses[i] = address

		if seen[address] {
			return fmt.Errorf("duplicate beacon node address: %s", address)
		}

		seen[address] = true
	}

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

		// Apply mismatch detection settings if provided
		if a.config.AttestationSubnetCheck.MismatchDetectionWindow != 0 {
			config.AttestationSubnetConfig.MismatchDetectionWindow = int(a.config.AttestationSubnetCheck.MismatchDetectionWindow)
		}

		if a.config.AttestationSubnetCheck.MismatchThreshold != 0 {
			config.AttestationSubnetConfig.MismatchThreshold = int(a.config.AttestationSubnetCheck.MismatchThreshold)
		}

		if a.config.AttestationSubnetCheck.MismatchCooldownSeconds != 0 {
			config.AttestationSubnetConfig.MismatchCooldownSeconds = int(a.config.AttestationSubnetCheck.MismatchCooldownSeconds)
		}

		if a.config.AttestationSubnetCheck.SubnetHighWaterMark != 0 {
			config.AttestationSubnetConfig.SubnetHighWaterMark = int(a.config.AttestationSubnetCheck.SubnetHighWaterMark)
		}
	}

	return config
}

// createTopicManager creates and configures a topic manager for the beacon node.
func (a *Application) createTopicManager(ctx context.Context, log logrus.FieldLogger, config *ethereum.Config) (ethereum.TopicManager, error) {
	// Log attestation subnet configuration
	if config.AttestationSubnetConfig.Enabled {
		log.WithFields(logrus.Fields{
			"max_subnets":               config.AttestationSubnetConfig.MaxSubnets,
			"mismatch_detection_window": config.AttestationSubnetConfig.MismatchDetectionWindow,
			"mismatch_threshold":        config.AttestationSubnetConfig.MismatchThreshold,
			"mismatch_cooldown_seconds": config.AttestationSubnetConfig.MismatchCooldownSeconds,
			"subnet_high_water_mark":    config.AttestationSubnetConfig.SubnetHighWaterMark,
		}).Info("Attestation subnet checking enabled")
	} else {
		log.Info("Attestation subnet checking disabled")
	}

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

	// Check for attestation subnet participation if enabled. The SSE topic list is computed once,
	// right after this returns (see BeaconFactory.CreateBeacon), so this is the only chance to
	// register single_attestation for the life of this connection - worth a bounded retry rather
	// than giving up on the first transient failure.
	if config.AttestationSubnetConfig.Enabled {
		activeSubnets, err := fetchNodeIdentityAttnetsWithRetry(ctx, log, config.BeaconNodeAddress, config.BeaconNodeHeaders)
		if err != nil {
			log.WithError(err).Error(
				"Failed to fetch node identity after retries; single_attestation will not be available for this connection",
			)
		} else {
			topicManager.RegisterCondition(
				ethereum.TopicSingleAttestation,
				ethereum.CreateAttestationSubnetCondition(len(activeSubnets), config.AttestationSubnetConfig.MaxSubnets),
			)
			topicManager.SetAdvertisedSubnets(activeSubnets)
		}
	}

	return topicManager, nil
}

// fetchNodeIdentityAttnetsWithRetry fetches the node's identity and attnets, retrying a bounded
// number of times on either the fetch or the attnets parse failing, since a beacon node that's
// still warming up alongside contributoor at startup is a normal, not exceptional, occurrence.
func fetchNodeIdentityAttnetsWithRetry(
	ctx context.Context,
	log logrus.FieldLogger,
	address string,
	headers map[string]string,
) ([]int, error) {
	const (
		maxAttempts = 3
		retryDelay  = 2 * time.Second
	)

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		identity := ethereum.NewNodeIdentity(log, address, headers)

		if err := identity.Start(ctx); err != nil {
			lastErr = err
		} else if subnets, attnetsErr := identity.GetAttnets(); attnetsErr != nil {
			lastErr = attnetsErr
		} else {
			return subnets, nil
		}

		if attempt == maxAttempts {
			break
		}

		log.WithError(lastErr).WithField("attempt", attempt).Warn("Failed to fetch node identity, retrying")

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
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
// The replacement instance (new node, cache, sinks, metrics, summary) is built and started fully
// before anything about the current instance is touched. If building or starting the replacement
// fails, the current instance is left completely untouched and keeps running - a failed restart
// no longer orphans the beacon. Only once the replacement is confirmed live does this swap it in
// and tear down everything the old instance owned (node, sinks, cache janitors, Prometheus
// collector).
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

	// Record the attempt regardless of outcome, so a failed restart still respects the cooldown
	// before the next attempt instead of retrying in a tight loop.
	b.lastReconnect = time.Now()

	b.log.Warn("Restarting beacon")

	// Use a modified traceID to avoid metrics collision with the instance being replaced.
	newTraceID := fmt.Sprintf("%s-nosub", b.traceID)
	newLog := b.log.WithField("trace_id", newTraceID)

	// Exclude single_attestation topic when creating new beacon.
	excludedTopics := []string{ethereum.TopicSingleAttestation}

	// Build and start the replacement before touching the current instance.
	newInstance, err := b.app.createBeaconInstance(ctx, newLog, b.Address, newTraceID, excludedTopics)
	if err != nil {
		return fmt.Errorf("failed to create new beacon instance: %w", err)
	}

	newInstance.Cache.Start()

	if err := newInstance.Node.Start(ctx); err != nil {
		return fmt.Errorf("failed to start new beacon node: %w", err)
	}

	// The replacement is live. Retire the current instance's monitoring and summary goroutines
	// before swapping the fields they read.
	if b.summaryCancel != nil {
		b.summaryCancel()
		b.log.Debug("Cancelled old summary goroutine")
	}

	close(b.stopMonitor)

	oldNode := b.Node
	oldSinks := b.Sinks
	oldCache := b.Cache
	oldMetrics := b.Metrics

	b.Node = newInstance.Node
	b.Metrics = newInstance.Metrics
	b.Summary = newInstance.Summary
	b.TopicManager = newInstance.TopicManager
	b.Sinks = newInstance.Sinks
	b.Cache = newInstance.Cache
	b.traceID = newTraceID
	b.log = newLog
	b.stopMonitor = make(chan struct{})
	b.summaryCancel = nil // Will be set when summary starts.

	// Restart monitoring goroutine for the new instance.
	go b.app.monitorBeaconInstance(ctx, b)

	b.log.Info("Restarted beacon node successfully")

	// Tear down everything the retired instance owned, now that the replacement has taken over.
	if err := oldNode.Stop(ctx); err != nil {
		b.log.WithError(err).Error("Failed to stop old beacon node")
	}

	for _, sink := range oldSinks {
		if err := sink.Stop(ctx); err != nil {
			b.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to stop old sink")
		}
	}

	oldCache.Stop()
	oldMetrics.Unregister()

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
