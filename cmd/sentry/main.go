package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof only enabled if pprofAddr config is set.
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var log = logrus.New()

const defaultMetricsAddr = ":9090"

type contributoor struct {
	log        logrus.FieldLogger
	config     *config.Config
	name       string
	beaconNode *ethereum.BeaconNode
	clockDrift clockdrift.ClockDrift
	sinks      []sinks.ContributoorSink
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
			}).Info("Starting contributoor")

			if err := s.initClockDrift(ctx); err != nil {
				return err
			}

			if err := s.initSinks(ctx, c.Bool("debug")); err != nil {
				return err
			}

			if err := s.initBeaconNode(); err != nil {
				return err
			}

			// Publish on done channel when we're finished cleaning up.
			done := make(chan struct{})

			go func() {
				<-c.Context.Done()

				if err := s.stop(context.Background()); err != nil {
					s.log.WithError(err).Error("Failed to stop contributoor")
				}

				close(done)
			}()

			if err := s.start(c.Context); err != nil {
				if err == context.Canceled {
					// Wait for cleanup to complete.
					<-done

					return nil
				}

				return err
			}

			// Wait for shutdown to complete.
			<-done

			return nil
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil && err != context.Canceled {
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

	return s.beaconNode.Start(ctx)
}

func (s *contributoor) stop(ctx context.Context) error {
	s.log.Info("Context cancelled, starting shutdown")

	if err := s.beaconNode.Stop(ctx); err != nil {
		s.log.WithError(err).Error("Failed to stop beacon node")
	}

	for _, sink := range s.sinks {
		if err := sink.Stop(ctx); err != nil {
			s.log.WithError(err).WithField("sink", sink.Name()).Error("Failed to stop sink")
		}
	}

	s.log.Info("Shutdown complete")

	return nil
}

func (s *contributoor) startMetricsServer() error {
	sm := http.NewServeMux()
	sm.Handle("/metrics", promhttp.Handler())

	if s.config.MetricsAddress == "" {
		s.config.MetricsAddress = defaultMetricsAddr
	}

	server := &http.Server{
		Addr:              s.config.MetricsAddress,
		ReadHeaderTimeout: 15 * time.Second,
		Handler:           sm,
	}

	s.log.Infof("Serving metrics at %s", s.config.MetricsAddress)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Fatal(err)
		}
	}()

	return nil
}

func (s *contributoor) startPProfServer() error {
	if s.config.PprofAddress == "" {
		return nil
	}

	pprofServer := &http.Server{
		Addr:              s.config.PprofAddress,
		ReadHeaderTimeout: 120 * time.Second,
	}

	s.log.Infof("Serving pprof at %s", s.config.PprofAddress)

	go func() {
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

func (s *contributoor) initBeaconNode() error {
	b, err := ethereum.NewBeaconNode(
		s.log,
		&ethereum.Config{
			BeaconNodeAddress:   s.config.BeaconNodeAddress,
			OverrideNetworkName: s.name,
		},
		s.name,
		s.sinks,
		s.clockDrift,
		&ethereum.Options{},
	)
	if err != nil {
		return err
	}

	s.beaconNode = b

	return nil
}
