package main

import (
	"context"
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

type contributoor struct {
	log               logrus.FieldLogger
	config            *config.Config
	beaconNode        *ethereum.BeaconNode
	clockDrift        clockdrift.ClockDrift
	sinks             []sinks.ContributoorSink
	cache             *events.DuplicateCache
	summary           *events.Summary
	metrics           *events.Metrics
	metricsServer     *http.Server
	pprofServer       *http.Server
	healthCheckServer *http.Server
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
				Usage:    "address of the beacon node api (e.g. http://localhost:5052)",
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

			if err := s.initSinks(ctx, c.Bool("debug")); err != nil {
				return err
			}

			if err := s.initCache(); err != nil {
				return err
			}

			if err := s.initMetrics(); err != nil {
				return err
			}

			if err := s.initSummary(); err != nil {
				return err
			}

			if err := s.initBeaconNode(); err != nil {
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

	s.cache.Start()
	go s.summary.Start(ctx)

	return s.beaconNode.Start(ctx)
}

func (s *contributoor) stop(ctx context.Context) error {
	s.log.Info("Context cancelled, starting shutdown")

	// Create a fresh context for shutdown
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.beaconNode.Stop(stopCtx); err != nil {
		s.log.WithError(err).Error("Failed to stop beacon node")
	}

	for _, sink := range s.sinks {
		if err := sink.Stop(stopCtx); err != nil {
			s.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to stop sink")
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

	s.log.Infof("Serving metrics at %s", addr)

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

	s.log.Infof("Serving pprof at %s", addr)

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

	s.log.Infof("Serving health check at %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.log.Fatal(err)
		}
	}()

	return nil
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

func (s *contributoor) initSinks(ctx context.Context, debug bool) error {
	if debug {
		stdoutSink, err := sinks.NewStdoutSink(log, s.config, s.config.NetworkName.DisplayName())
		if err != nil {
			return err
		}

		s.sinks = append(s.sinks, stdoutSink)
	}

	xatuSink, err := sinks.NewXatuSink(log, s.config, s.config.NetworkName.DisplayName())
	if err != nil {
		return err
	}

	s.sinks = append(s.sinks, xatuSink)

	for _, sink := range s.sinks {
		if err := sink.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *contributoor) initCache() error {
	s.cache = events.NewDuplicateCache()

	return nil
}

func (s *contributoor) initMetrics() error {
	s.metrics = events.NewMetrics("contributoor")

	return nil
}

func (s *contributoor) initSummary() error {
	s.summary = events.NewSummary(s.log, 10*time.Second)

	return nil
}

func (s *contributoor) initBeaconNode() error {
	b, err := ethereum.NewBeaconNode(
		s.log,
		&ethereum.Config{
			BeaconNodeAddress:   s.config.BeaconNodeAddress,
			OverrideNetworkName: strings.ToLower(s.config.NetworkName.DisplayName()),
		},
		s.sinks,
		s.clockDrift,
		s.cache,
		s.summary,
		s.metrics,
		&ethereum.Options{},
	)
	if err != nil {
		return err
	}

	s.beaconNode = b

	return nil
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
