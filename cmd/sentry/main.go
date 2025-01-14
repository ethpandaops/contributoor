package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof only enabled if pprofAddr config is set.
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethpandaops/bamboo/pkg/clockdrift"
	config "github.com/ethpandaops/bamboo/proto/contributoor/config/v1"
	contr "github.com/ethpandaops/contributoor/internal/contributoor"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var log = logrus.New()

type contributoor struct {
	log           logrus.FieldLogger
	config        *config.Config
	name          string
	beaconNode    *ethereum.BeaconNode
	clockDrift    clockdrift.ClockDrift
	sinks         []sinks.ContributoorSink
	cache         *events.DuplicateCache
	summary       *events.Summary
	metrics       *events.Metrics
	metricsServer *http.Server
	pprofServer   *http.Server
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
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "debug mode",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			s, err := newContributoor(c)
			if err != nil {
				return err
			}

			s.log.WithFields(logrus.Fields{
				"config_path": s.config.ContributoorDirectory,
				"name":        s.name,
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
	cfg, err := config.NewConfigFromPath(c.String("config"))
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s_contributoor", strings.ToLower(cfg.NetworkName.String()))

	return &contributoor{
		log:    log.WithField("module", "contributoor"),
		config: cfg,
		name:   name,
	}, nil
}

func (s *contributoor) start(ctx context.Context) error {
	if err := s.startMetricsServer(); err != nil {
		return err
	}

	if err := s.startPProfServer(); err != nil {
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
		stdoutSink, err := sinks.NewStdoutSink(log, s.config, s.name)
		if err != nil {
			return err
		}

		s.sinks = append(s.sinks, stdoutSink)
	}

	xatuSink, err := sinks.NewXatuSink(log, s.config, s.name)
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
			BeaconNodeAddress:   s.config.NodeAddress(),
			OverrideNetworkName: s.name,
		},
		s.name,
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
