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

		instance, err := a.createBeaconInstance(ctx, address, traceID, logCtx)
		if err != nil {
			return fmt.Errorf("failed to create beacon instance: %w", err)
		}

		a.beaconNodes[traceID] = instance
	}

	return nil
}

// createBeaconInstance creates a single beacon node instance with all its components.
func (a *Application) createBeaconInstance(ctx context.Context, address, traceID string, log logrus.FieldLogger) (*BeaconNodeInstance, error) {
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

	sinks, err := a.initSinks(ctx, log, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init sinks: %w", err)
	}

	node, err := a.initBeacon(ctx, log, address, traceID, sinks, cache, summary, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to init beacon: %w", err)
	}

	return &BeaconNodeInstance{
		Node:    node,
		Cache:   cache,
		Sinks:   sinks,
		Metrics: metrics,
		Summary: summary,
		Address: address,
	}, nil
}

// initBeacon creates a new beacon node connection.
func (a *Application) initBeacon(
	ctx context.Context,
	log logrus.FieldLogger,
	address, traceID string,
	sinks []sinks.ContributoorSink,
	cache *events.DuplicateCache,
	summary *events.Summary,
	metrics *events.Metrics,
) (ethereum.BeaconNodeAPI, error) {
	// Get the network from config if set
	var networkOverride string
	if a.config.NetworkName != "" {
		networkOverride = a.config.NetworkName
	}

	// Start with default config
	config := ethereum.NewDefaultConfig()
	config.BeaconNodeAddress = address
	config.NetworkOverride = networkOverride

	return ethereum.NewBeaconWrapper(
		ctx,
		log,
		traceID,
		config,
		sinks,
		a.clockDrift,
		cache,
		summary,
		metrics,
	)
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
