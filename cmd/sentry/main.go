package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var log = logrus.New()

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
		Name:  "sentry",
		Usage: "Contributoor sentry node",
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
			cfg, err := config.NewConfigFromPath(c.String("config"))
			if err != nil {
				return err
			}

			name := fmt.Sprintf("%s_contributoor", strings.ToLower(cfg.NetworkName.String()))

			log.WithFields(logrus.Fields{
				"config_path": cfg.ContributoorDirectory,
				"name":        name,
				"version":     cfg.Version,
			}).Info("Starting sentry")

			// Start clock drift service.
			clockDriftService := clockdrift.NewService(log, &clockdrift.ClockDriftConfig{
				NTPServer:    "pool.ntp.org",
				SyncInterval: 5 * time.Minute,
			})

			if cerr := clockDriftService.Start(ctx); cerr != nil {
				return cerr
			}

			var activeSinks []sinks.ContributoorSink

			// Always create stdout sink in debug mode.
			if c.Bool("debug") {
				stdoutSink, serr := sinks.NewStdoutSink(log, cfg, name)
				if serr != nil {
					return serr
				}

				activeSinks = append(activeSinks, stdoutSink)
			}

			xatuSink, err := sinks.NewXatuSink(log, cfg, name)
			if err != nil {
				return err
			}

			activeSinks = append(activeSinks, xatuSink)

			for _, sink := range activeSinks {
				if serr := sink.Start(c.Context); serr != nil {
					return serr
				}
			}

			// Create beacon node with sinks
			beaconOpts := ethereum.Options{}
			ethConf := &ethereum.Config{
				BeaconNodeAddress:   cfg.BeaconNodeAddress,
				OverrideNetworkName: name,
			}

			b, err := ethereum.NewBeaconNode(log, ethConf, name, activeSinks, clockDriftService, &beaconOpts)
			if err != nil {
				return err
			}

			// Publish on done channel when we're finished cleaning up.
			done := make(chan struct{})

			go func() {
				<-c.Context.Done()

				log.Info("Context cancelled, starting shutdown")

				if err := b.Stop(context.Background()); err != nil {
					log.WithError(err).Error("Failed to stop beacon node")
				}

				for _, sink := range activeSinks {
					if err := sink.Stop(context.Background()); err != nil {
						log.WithError(err).WithField("sink", sink.Name()).Error("Failed to stop sink")
					}
				}

				log.Info("Shutdown complete")

				close(done)
			}()

			if err := b.Start(c.Context); err != nil {
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
