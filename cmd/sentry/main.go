package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof only enabled if pprofAddr config is set.
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
	contr "github.com/ethpandaops/contributoor/internal/contributoor"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var log = logrus.New()

type beaconNodeInstance struct {
	node    ethereum.BeaconNodeAPI
	cache   *events.DuplicateCache
	sinks   []sinks.ContributoorSink
	metrics *events.Metrics
	summary *events.Summary
}

type contributoor struct {
	log               logrus.FieldLogger
	config            *config.Config
	clockDrift        clockdrift.ClockDrift
	beaconNodes       map[string]*beaconNodeInstance // traceID -> instance
	metricsServer     *http.Server
	pprofServer       *http.Server
	healthCheckServer *http.Server
	debug             bool
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal")
		cancel()
	}()

	app := &cli.App{
		Name:  "contributoor",
		Usage: "Contributoor node",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Usage:    "config file path",
				Required: false,
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "debug mode",
				Value: false,
			},
			&cli.StringFlag{
				Name:     "network",
				Usage:    "ethereum network name (mainnet, sepolia, holesky)",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "beacon-node-address",
				Usage:    "comma-separated addresses of beacon node apis (e.g. http://localhost:5052,http://localhost:5053)",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "metrics-address",
				Usage:    "address of the metrics server",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "health-check-address",
				Usage:    "address of the health check server",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "log-level",
				Usage:    "log level (debug, info, warn, error)",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "output-server-address",
				Usage:    "address of the output server",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "username",
				Usage:    "username for the output server",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "password",
				Usage:    "password for the output server",
				Value:    "",
				Required: false,
			},
			// Can't use bool flag here because it doesn't have a "nil" value.
			&cli.StringFlag{
				Name:     "output-server-tls",
				Usage:    "enable TLS for the output server",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "contributoor-directory",
				Usage:    "directory where contributoor stores configuration and data",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "release",
				Usage:    "print release and exit",
				Required: false,
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("release") {
				fmt.Printf("%s\n", contr.Release)

				os.Exit(0)
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			s, err := newContributoor(c)
			if err != nil {
				return err
			}

			s.log.WithFields(logrus.Fields{
				"config_path": s.config.ContributoorDirectory,
				"version":     s.config.Version,
				"commit":      contr.GitCommit,
				"release":     contr.Release,
			}).Info("Starting contributoor")

			if err := s.initClockDrift(ctx); err != nil {
				return err
			}

			if err := s.initBeacons(ctx); err != nil {
				return err
			}

			// Publish on done channel when we're finished cleaning up.
			done := make(chan struct{})

			go func() {
				<-ctx.Done()

				if err := s.stop(ctx); err != nil {
					s.log.WithError(err).Error("Failed to stop contributoor")
				}

				close(done)
			}()

			if err := s.start(ctx); err != nil {
				// Cancel context to trigger cleanup.
				cancel()

				// Wait for cleanup to complete.
				<-done

				// Only return the error if it's not due to context cancellation.
				if err != context.Canceled {
					return err
				}

				return nil
			}

			// Wait for shutdown to complete.
			<-done

			return nil
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func newContributoor(c *cli.Context) (*contributoor, error) {
	cfg := config.NewDefaultConfig()

	configLocation := c.String("config")
	if configLocation != "" {
		configFromFile, err := config.NewConfigFromPath(configLocation)
		if err != nil {
			return nil, err
		}

		cfg = configFromFile
	}

	if err := applyConfigOverridesFromFlags(cfg, c); err != nil {
		return nil, errors.Wrap(err, "failed to apply config overrides from cli flags")
	}

	return &contributoor{
		log:    log.WithField("module", "contributoor"),
		config: cfg,
		debug:  c.Bool("debug"),
	}, nil
}

func (s *contributoor) start(ctx context.Context) error {
	if err := s.startMetricsServer(); err != nil {
		return err
	}

	if err := s.startPProfServer(); err != nil {
		return err
	}

	if err := s.startHealthCheckServer(); err != nil {
		return err
	}

	for _, instance := range s.beaconNodes {
		// Start the cache.
		instance.cache.Start()

		// We don't really want to output/log a summary until the node is healthy.
		// Wait for node to become healthy before starting summary.
		go func(instance *beaconNodeInstance) {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if node, ok := instance.node.(*ethereum.BeaconNode); ok && node.IsHealthy() {
						instance.summary.Start(ctx)

						return
					}
				}
			}
		}(instance)
	}

	return s.connectBeacons(ctx)
}

func (s *contributoor) stop(ctx context.Context) error {
	s.log.Info("Context cancelled, starting shutdown")

	// Create a fresh context for shutdown
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for traceID, instance := range s.beaconNodes {
		// Stop the beacon + any sinks we have.
		if err := instance.node.Stop(stopCtx); err != nil {
			s.log.WithError(err).WithField("trace_id", traceID).Error("Failed to stop beacon")
		}

		for _, sink := range instance.sinks {
			if err := sink.Stop(stopCtx); err != nil {
				s.log.WithError(err).WithFields(logrus.Fields{
					"trace_id": traceID,
					"sink":     sink.Name(),
				}).Error("Failed to stop sink")
			}
		}
	}

	// Shutdown HTTP servers
	if s.metricsServer != nil {
		if err := s.metricsServer.Shutdown(stopCtx); err != nil {
			s.log.WithError(err).Error("Failed to stop metrics server")
		}
	}

	if s.pprofServer != nil {
		if err := s.pprofServer.Shutdown(stopCtx); err != nil {
			s.log.WithError(err).Error("Failed to stop pprof server")
		}
	}

	if s.healthCheckServer != nil {
		if err := s.healthCheckServer.Shutdown(stopCtx); err != nil {
			s.log.WithError(err).Error("Failed to stop health check server")
		}
	}

	s.log.Info("Shutdown complete")

	return nil
}

func (s *contributoor) startMetricsServer() error {
	metricsHost, metricsPort := s.config.GetMetricsHostPort()
	if metricsHost == "" {
		return nil
	}

	var addr = fmt.Sprintf(":%s", metricsPort)

	// Start listening before creating the server to catch invalid addresses.
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	sm := http.NewServeMux()
	sm.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 15 * time.Second,
		Handler:           sm,
	}

	s.metricsServer = server

	s.log.WithField("endpoint", "/metrics").Infof("Serving metrics at %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.log.Fatal(err)
		}
	}()

	return nil
}

func (s *contributoor) startPProfServer() error {
	pprofHost, pprofPort := s.config.GetPprofHostPort()
	if pprofHost == "" {
		return nil
	}

	var addr = fmt.Sprintf(":%s", pprofPort)

	// Start listening before creating the server to catch invalid addresses.
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start pprof server: %w", err)
	}

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 120 * time.Second,
	}

	s.pprofServer = server

	s.log.WithField("endpoint", "/debug/pprof").Infof("Serving pprof at %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.log.Fatal(err)
		}
	}()

	return nil
}

func (s *contributoor) startHealthCheckServer() error {
	healthCheckHost, healthCheckPort := s.config.GetHealthCheckHostPort()
	if healthCheckHost == "" {
		return nil
	}

	var addr = fmt.Sprintf(":%s", healthCheckPort)

	// Start listening before creating the server to catch invalid addresses.
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	sm := http.NewServeMux()
	sm.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 15 * time.Second,
		Handler:           sm,
	}

	s.healthCheckServer = server

	s.log.WithField("endpoint", "/healthz").Infof("Serving health check at %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.log.Fatal(err)
		}
	}()

	return nil
}

func (s *contributoor) connectBeacons(ctx context.Context) error {
	var (
		errChan      = make(chan error, len(s.beaconNodes))
		doneChan     = make(chan string, len(s.beaconNodes))
		timeout      = time.After(30 * time.Second)
		healthyNodes = make(map[string]struct{})
	)

	// Connect to all beacon nodes concurrently.
	for traceID, instance := range s.beaconNodes {
		go func() {
			readyChan, err := instance.node.Start(ctx)
			if err != nil {
				errChan <- fmt.Errorf("failed to connect to beacon %s: %w", traceID, err)

				return
			}

			// Wait for the node to become healthy.
			select {
			case <-readyChan:
				doneChan <- traceID
			case <-ctx.Done():
				errChan <- fmt.Errorf("context cancelled while waiting to connect to beacon %s", traceID)
			}
		}()
	}

	// Wait for at least one node to connect successfully or all to fail.
	remainingNodes := len(s.beaconNodes)
	for remainingNodes > 0 {
		select {
		case err := <-errChan:
			s.log.WithError(err).Error("Failed to connect to beacon")

			remainingNodes--

			// If we've failed on all nodes, return the last error.
			if remainingNodes == 0 && len(healthyNodes) == 0 {
				return fmt.Errorf("all beacons failed to connect: %w", err)
			}
		case traceID := <-doneChan:
			s.log.WithField("trace_id", traceID).Info("Beacon connected successfully")

			healthyNodes[traceID] = struct{}{}
			remainingNodes--

			// If we have at least one healthy node, we're good to grab metrics from that.
			// Process any remaining nodes in background.
			if len(healthyNodes) == 1 && remainingNodes > 0 {
				s.connectRemainingBeaconNodesInBackground(ctx, &remainingNodes, errChan, doneChan, healthyNodes)

				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			// Only timeout if we have no healthy nodes.
			if len(healthyNodes) == 0 {
				return fmt.Errorf("timeout waiting to connect to any beacon")
			}

			// If we have healthy nodes, continue waiting on the others in the background.
			s.connectRemainingBeaconNodesInBackground(ctx, &remainingNodes, errChan, doneChan, healthyNodes)

			return nil
		}
	}

	// If we get here with no healthy nodes, all nodes failed.
	if len(healthyNodes) == 0 {
		return fmt.Errorf("all beacons failed to connect")
	}

	return nil
}

func (s *contributoor) connectRemainingBeaconNodesInBackground(
	ctx context.Context,
	remainingNodes *int,
	errChan chan error,
	doneChan chan string,
	healthyNodes map[string]struct{},
) {
	s.log.WithFields(logrus.Fields{
		"healthy_nodes":   len(healthyNodes),
		"remaining_nodes": *remainingNodes,
	}).Info("Continuing beacon connection in background")

	go func() {
		for *remainingNodes > 0 {
			select {
			case err := <-errChan:
				s.log.WithError(err).Error("Failed to connect to beacon in background")

				*remainingNodes--
			case traceID := <-doneChan:
				s.log.WithField("trace_id", traceID).Info("Additional beacon connected successfully")

				healthyNodes[traceID] = struct{}{}
				*remainingNodes--
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *contributoor) initClockDrift(ctx context.Context) error {
	clockDriftService := clockdrift.NewService(s.log, &clockdrift.ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	})

	if err := clockDriftService.Start(ctx); err != nil {
		return err
	}

	s.clockDrift = clockDriftService

	return nil
}

func (s *contributoor) initBeacons(ctx context.Context) error {
	addresses := strings.Split(s.config.BeaconNodeAddress, ",")

	traceIDs, err := generateBeaconTraceIDs(addresses)
	if err != nil {
		return fmt.Errorf("failed to generate trace IDs: %w", err)
	}

	s.beaconNodes = make(map[string]*beaconNodeInstance)

	s.log.WithFields(logrus.Fields{
		"count":     len(addresses),
		"trace_ids": traceIDs,
		"addresses": addresses,
	}).Info("Initializing beacons")

	for i, address := range addresses {
		address = strings.TrimSpace(address)
		traceID := traceIDs[i]

		logCtx := s.log.WithField("trace_id", traceID)

		instance, err := s.createBeaconInstance(ctx, address, traceID, logCtx)
		if err != nil {
			return fmt.Errorf("failed to create beacon instance: %w", err)
		}

		s.beaconNodes[traceID] = instance
	}

	return nil
}

func (s *contributoor) initBeacon(
	log logrus.FieldLogger,
	address, traceID string,
	sinks []sinks.ContributoorSink,
	cache *events.DuplicateCache,
	summary *events.Summary,
	metrics *events.Metrics,
) (ethereum.BeaconNodeAPI, error) {
	return ethereum.NewBeaconNode(
		log,
		traceID,
		&ethereum.Config{
			BeaconNodeAddress: address,
		},
		sinks,
		s.clockDrift,
		cache,
		summary,
		metrics,
		&ethereum.Options{},
	)
}

func (s *contributoor) initCache() (*events.DuplicateCache, error) {
	return events.NewDuplicateCache(), nil
}

func (s *contributoor) initMetrics(traceID string) (*events.Metrics, error) {
	return events.NewMetrics(
		strings.ReplaceAll(fmt.Sprintf("contributoor_%s", traceID), "-", "_"),
	), nil
}

func (s *contributoor) initSummary(log logrus.FieldLogger, traceID string) (*events.Summary, error) {
	return events.NewSummary(log, traceID, 10*time.Second), nil
}

func (s *contributoor) initSinks(ctx context.Context, log logrus.FieldLogger, traceID string) ([]sinks.ContributoorSink, error) {
	eventSinks := make([]sinks.ContributoorSink, 0)

	if s.debug {
		stdoutSink, err := sinks.NewStdoutSink(log, s.config, traceID)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout sink: %w", err)
		}

		eventSinks = append(eventSinks, stdoutSink)
	}

	xatuSink, err := sinks.NewXatuSink(log, s.config, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create xatu sink: %w", err)
	}

	eventSinks = append(eventSinks, xatuSink)

	// Start all the sinks.
	for _, sink := range eventSinks {
		if err := sink.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start sink %s: %w", sink.Name(), err)
		}
	}

	return eventSinks, nil
}

func (s *contributoor) createBeaconInstance(ctx context.Context, address, traceID string, log logrus.FieldLogger) (*beaconNodeInstance, error) {
	cache, err := s.initCache()
	if err != nil {
		return nil, fmt.Errorf("failed to init cache: %w", err)
	}

	metrics, err := s.initMetrics(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init metrics: %w", err)
	}

	summary, err := s.initSummary(log, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init summary: %w", err)
	}

	sinks, err := s.initSinks(ctx, log, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to init sinks: %w", err)
	}

	node, err := s.initBeacon(log, address, traceID, sinks, cache, summary, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to init beacon: %w", err)
	}

	return &beaconNodeInstance{
		node:    node,
		cache:   cache,
		sinks:   sinks,
		metrics: metrics,
		summary: summary,
	}, nil
}

// applyConfigOverridesFromFlags applies CLI flags to the config if they are set.
func applyConfigOverridesFromFlags(cfg *config.Config, c *cli.Context) error {
	// Apply environment variables first, then override with CLI flags if set
	if network := os.Getenv("CONTRIBUTOOR_NETWORK"); network != "" {
		log.Infof("Setting network from env to %s", network)

		if err := cfg.SetNetwork(network); err != nil {
			return errors.Wrap(err, "failed to set network from env")
		}
	}

	if c.String("network") != "" {
		log.Infof("Overriding network from CLI to %s", c.String("network"))

		if err := cfg.SetNetwork(c.String("network")); err != nil {
			return errors.Wrap(err, "failed to set network from cli")
		}
	}

	if addr := os.Getenv("CONTRIBUTOOR_BEACON_NODE_ADDRESS"); addr != "" {
		log.Infof("Setting beacon node address from env")
		cfg.SetBeaconNodeAddress(addr)
	}

	if c.String("beacon-node-address") != "" {
		log.Infof("Overriding beacon node address from CLI")
		cfg.SetBeaconNodeAddress(c.String("beacon-node-address"))
	}

	if addr := os.Getenv("CONTRIBUTOOR_METRICS_ADDRESS"); addr != "" {
		log.Infof("Setting metrics address from env to %s", addr)
		cfg.SetMetricsAddress(addr)
	}

	if c.String("metrics-address") != "" {
		log.Infof("Overriding metrics address from CLI to %s", c.String("metrics-address"))
		cfg.SetMetricsAddress(c.String("metrics-address"))
	}

	if addr := os.Getenv("CONTRIBUTOOR_HEALTH_CHECK_ADDRESS"); addr != "" {
		log.Infof("Setting health check address from env to %s", addr)
		cfg.SetHealthCheckAddress(addr)
	}

	if c.String("health-check-address") != "" {
		log.Infof("Overriding health check address from CLI to %s", c.String("health-check-address"))
		cfg.SetHealthCheckAddress(c.String("health-check-address"))
	}

	if level := os.Getenv("CONTRIBUTOOR_LOG_LEVEL"); level != "" {
		log.Infof("Setting log level from env to %s", level)
		cfg.SetLogLevel(level)
	}

	if c.String("log-level") != "" {
		log.Infof("Overriding log level from CLI to %s", c.String("log-level"))
		cfg.SetLogLevel(c.String("log-level"))
	}

	if addr := os.Getenv("CONTRIBUTOOR_OUTPUT_SERVER_ADDRESS"); addr != "" {
		log.Infof("Setting output server address from env")
		cfg.SetOutputServerAddress(addr)
	}

	if c.String("output-server-address") != "" {
		log.Infof("Overriding output server address from CLI")
		cfg.SetOutputServerAddress(c.String("output-server-address"))
	}

	// Handle credentials from env
	var (
		username = os.Getenv("CONTRIBUTOOR_USERNAME")
		password = os.Getenv("CONTRIBUTOOR_PASSWORD")
	)

	if username != "" || password != "" {
		log.Infof("Setting output server credentials from env")
		cfg.SetOutputServerCredentials(
			base64.StdEncoding.EncodeToString(
				[]byte(fmt.Sprintf("%s:%s", username, password)),
			),
		)
	}

	// CLI flags override env vars for credentials
	if c.String("username") != "" || c.String("password") != "" {
		log.Infof("Overriding output server credentials from CLI")
		cfg.SetOutputServerCredentials(
			base64.StdEncoding.EncodeToString(
				[]byte(fmt.Sprintf("%s:%s", c.String("username"), c.String("password"))),
			),
		)
	}

	if tls := os.Getenv("CONTRIBUTOOR_OUTPUT_SERVER_TLS"); tls != "" {
		log.Infof("Setting output server tls from env to %s", tls)

		tlsBool, err := strconv.ParseBool(tls)
		if err != nil {
			return errors.Wrap(err, "failed to parse output server tls env var")
		}

		cfg.SetOutputServerTLS(tlsBool)
	}

	if c.String("output-server-tls") != "" {
		log.Infof("Overriding output server tls from CLI to %s", c.String("output-server-tls"))

		tls, err := strconv.ParseBool(c.String("output-server-tls"))
		if err != nil {
			return errors.Wrap(err, "failed to parse output server tls flag")
		}

		cfg.SetOutputServerTLS(tls)
	}

	return nil
}

// generateBeaconTraceIDs generates a short identifier for the beacon node, allows us to distinguish
// between multiple nodes in logs and across summaries.
func generateBeaconTraceIDs(addresses []string) ([]string, error) {
	traceIDs := make([]string, len(addresses))

	for i, address := range addresses {
		if len(address) > 30 {
			address = address[len(address)-30:]
		}

		traceIDs[i] = fmt.Sprintf("bn_%x", sha256.Sum256([]byte(address)))[0:8]
	}

	return traceIDs, nil
}
