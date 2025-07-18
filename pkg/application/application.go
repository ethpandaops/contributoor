// Package application provides the core Contributoor functionality as a reusable library.
// It encapsulates beacon node connections, event processing, and metric collection.
package application

import (
	"net/http"
	"time"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/sirupsen/logrus"
)

// Application represents a Contributoor instance with all its components.
// It manages beacon node connections, event processing, metrics collection,
// and provides HTTP endpoints for health checks and metrics.
type Application struct {
	config                 *config.Config
	log                    logrus.FieldLogger
	clockDrift             clockdrift.ClockDrift
	beaconNodes            map[string]*BeaconNodeInstance
	servers                *ServerManager
	debug                  bool
	ntpServer              string
	clockDriftSyncInterval time.Duration
}

// BeaconNodeInstance holds all components related to a single beacon node connection.
type BeaconNodeInstance struct {
	Node    ethereum.BeaconNodeAPI
	Cache   *events.DuplicateCache
	Sinks   []sinks.ContributoorSink
	Metrics *events.Metrics
	Summary *events.Summary
	Address string // The beacon node's address
}

// ServerManager handles HTTP server lifecycle for metrics, pprof, and health checks.
type ServerManager struct {
	metricsServer     *http.Server
	pprofServer       *http.Server
	healthCheckServer *http.Server
}

// New creates a new Application instance with the provided options.
// It validates the configuration and sets up logging but does not start any services.
// Use Start() to begin processing.
func New(opts Options) (*Application, error) {
	// Validate required options
	if opts.Config == nil {
		return nil, ErrConfigRequired
	}

	// Set defaults
	if opts.Logger == nil {
		opts.Logger = logrus.New().WithField("module", "contributoor")
	}

	// Set NTP configuration with defaults
	ntpServer := opts.NTPServer
	if ntpServer == "" {
		ntpServer = "pool.ntp.org"
	}

	clockDriftSyncInterval := opts.ClockDriftSyncInterval
	if clockDriftSyncInterval == 0 {
		clockDriftSyncInterval = 5 * time.Minute
	}

	app := &Application{
		config:                 opts.Config,
		log:                    opts.Logger,
		debug:                  opts.Debug,
		beaconNodes:            make(map[string]*BeaconNodeInstance),
		servers:                &ServerManager{},
		ntpServer:              ntpServer,
		clockDriftSyncInterval: clockDriftSyncInterval,
	}

	// If clock drift service is provided, use it. Otherwise we'll create one during Start()
	if opts.ClockDrift != nil {
		app.clockDrift = opts.ClockDrift
	}

	return app, nil
}

// Config returns the application configuration.
func (a *Application) Config() *config.Config {
	return a.config
}

// BeaconNodes returns the map of beacon node instances.
// This is useful for testing and monitoring.
func (a *Application) BeaconNodes() map[string]*BeaconNodeInstance {
	return a.beaconNodes
}

// Metrics returns the metrics for a specific beacon node by trace ID.
// Returns nil if the trace ID is not found.
func (a *Application) Metrics(traceID string) *events.Metrics {
	if instance, ok := a.beaconNodes[traceID]; ok {
		return instance.Metrics
	}

	return nil
}

// IsHealthy returns true if at least one beacon node is healthy and connected.
func (a *Application) IsHealthy() bool {
	for _, instance := range a.beaconNodes {
		if node, ok := instance.Node.(*ethereum.BeaconWrapper); ok && node.IsHealthy() {
			return true
		}
	}

	return false
}

// Logger returns the application logger.
func (a *Application) Logger() logrus.FieldLogger {
	return a.log
}
