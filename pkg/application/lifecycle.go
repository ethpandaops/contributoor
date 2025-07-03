package application

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/sirupsen/logrus"
)

// Start initializes and starts all components of the application.
// It starts HTTP servers, connects to beacon nodes, and begins event processing.
func (a *Application) Start(ctx context.Context) error {
	a.log.Info("Starting application")

	// Initialize clock drift if not provided
	if a.clockDrift == nil {
		if err := a.initClockDrift(ctx); err != nil {
			return fmt.Errorf("failed to initialize clock drift: %w", err)
		}
	}

	// Initialize beacon nodes
	if err := a.initBeacons(ctx); err != nil {
		return fmt.Errorf("failed to initialize beacons: %w", err)
	}

	// Start HTTP servers
	if err := a.startMetricsServer(); err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	if err := a.startPProfServer(); err != nil {
		return fmt.Errorf("failed to start pprof server: %w", err)
	}

	if err := a.startHealthCheckServer(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	// Start beacon node components
	for _, instance := range a.beaconNodes {
		// Start the cache
		instance.Cache.Start()
	}

	// Connect to beacon nodes
	if err := a.connectBeacons(ctx); err != nil {
		return fmt.Errorf("failed to connect to beacons: %w", err)
	}

	// Start beacon node components
	for _, instance := range a.beaconNodes {
		// Start summary after node is healthy
		go a.startSummaryWhenHealthy(ctx, instance)
	}

	a.log.Info("Application started successfully")

	return nil
}

// Stop gracefully shuts down all components of the application.
func (a *Application) Stop(ctx context.Context) error {
	// If no beacon nodes are configured, the application was never started
	if len(a.beaconNodes) == 0 {
		a.log.Debug("Application was not started, nothing to stop")

		return nil
	}

	a.log.Info("Stopping application")

	// Create a timeout context if one wasn't provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
	}

	// Stop beacon nodes and sinks
	for traceID, instance := range a.beaconNodes {
		if err := instance.Node.Stop(ctx); err != nil {
			a.log.WithError(err).WithField("trace_id", traceID).Error("Failed to stop beacon")
		}

		for _, sink := range instance.Sinks {
			if err := sink.Stop(ctx); err != nil {
				a.log.WithError(err).WithFields(logrus.Fields{
					"trace_id": traceID,
					"sink":     sink.Name(),
				}).Error("Failed to stop sink")
			}
		}
	}

	// Stop HTTP servers
	a.stopServers(ctx)

	a.log.Info("Application stopped")

	return nil
}

// startSummaryWhenHealthy starts the summary logger once the beacon node becomes healthy.
func (a *Application) startSummaryWhenHealthy(ctx context.Context, instance *BeaconNodeInstance) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if node, ok := instance.Node.(*ethereum.BeaconWrapper); ok && node.IsHealthy() {
				instance.Summary.Start(ctx)

				return
			}
		}
	}
}

// connectBeacons connects to all configured beacon nodes.
// It ensures at least one beacon connects successfully before returning.
func (a *Application) connectBeacons(ctx context.Context) error {
	if len(a.beaconNodes) == 0 {
		return ErrNoBeaconNodes
	}

	// Create channels for coordination
	errChan := make(chan error, len(a.beaconNodes))
	doneChan := make(chan string, len(a.beaconNodes))
	healthyNodes := make(map[string]struct{})

	// Create a timeout for initial connection
	timeout := time.After(60 * time.Second)

	// Connect to all beacon nodes concurrently
	for traceID, instance := range a.beaconNodes {
		go func(traceID string, instance *BeaconNodeInstance) {
			err := instance.Node.Start(ctx)
			if err != nil {
				errChan <- fmt.Errorf("failed to connect to beacon %s: %w", traceID, err)

				return
			}

			// Node is now healthy
			doneChan <- traceID
		}(traceID, instance)
	}

	// Wait for at least one node to connect successfully or all to fail
	remainingNodes := len(a.beaconNodes)
	for remainingNodes > 0 {
		select {
		case err := <-errChan:
			a.log.WithError(err).Error("Failed to connect to beacon")

			remainingNodes--

			// If we've failed on all nodes, return the last error
			if remainingNodes == 0 && len(healthyNodes) == 0 {
				return fmt.Errorf("%w: %v", ErrAllBeaconsFailed, err)
			}
		case traceID := <-doneChan:
			a.log.WithField("trace_id", traceID).Info("Beacon connected successfully")

			healthyNodes[traceID] = struct{}{}
			remainingNodes--

			// If we have at least one healthy node, we're good
			if len(healthyNodes) == 1 && remainingNodes > 0 {
				a.connectRemainingBeaconNodesInBackground(ctx, &remainingNodes, errChan, doneChan, healthyNodes)

				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			// Only timeout if we have no healthy nodes
			if len(healthyNodes) == 0 {
				return fmt.Errorf("timeout waiting to connect to any beacon")
			}

			// If we have healthy nodes, continue waiting on the others in the background
			a.connectRemainingBeaconNodesInBackground(ctx, &remainingNodes, errChan, doneChan, healthyNodes)

			return nil
		}
	}

	// If we get here with no healthy nodes, all nodes failed
	if len(healthyNodes) == 0 {
		return ErrAllBeaconsFailed
	}

	return nil
}

// connectRemainingBeaconNodesInBackground continues connecting to remaining nodes in the background.
func (a *Application) connectRemainingBeaconNodesInBackground(
	ctx context.Context,
	remainingNodes *int,
	errChan chan error,
	doneChan chan string,
	healthyNodes map[string]struct{},
) {
	a.log.WithFields(logrus.Fields{
		"healthy_nodes":   len(healthyNodes),
		"remaining_nodes": *remainingNodes,
	}).Info("Continuing beacon connection in background")

	go func() {
		for *remainingNodes > 0 {
			select {
			case err := <-errChan:
				a.log.WithError(err).Error("Failed to connect to beacon in background")

				*remainingNodes--
			case traceID := <-doneChan:
				a.log.WithField("trace_id", traceID).Info("Additional beacon connected successfully")

				healthyNodes[traceID] = struct{}{}
				*remainingNodes--
			case <-ctx.Done():
				return
			}
		}
	}()
}
