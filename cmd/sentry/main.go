package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	contr "github.com/ethpandaops/contributoor/internal/contributoor"
	"github.com/ethpandaops/contributoor/pkg/application"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/ssgreg/journalhook"
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
				Name:     "attestation-subnet-check-enabled",
				Usage:    "enable attestation subnet checking for single_attestation topic filtering",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "attestation-subnet-max-subnets",
				Usage:    "maximum number of subnets a node can be subscribed to before single_attestation topic is disabled (0-64)",
				Value:    -1, // -1 indicates not set via CLI
				Required: false,
			},
			&cli.IntFlag{
				Name:     "attestation-subnet-mismatch-detection-window",
				Usage:    "number of slots to track for subnet activity",
				Value:    -1, // -1 indicates not set via CLI
				Required: false,
			},
			&cli.IntFlag{
				Name:     "attestation-subnet-mismatch-threshold",
				Usage:    "number of mismatches required before triggering reconnection",
				Value:    -1, // -1 indicates not set via CLI
				Required: false,
			},
			&cli.IntFlag{
				Name:     "attestation-subnet-mismatch-cooldown-seconds",
				Usage:    "cooldown period between reconnections in seconds",
				Value:    -1, // -1 indicates not set via CLI
				Required: false,
			},
			&cli.IntFlag{
				Name:     "attestation-subnet-high-water-mark",
				Usage:    "number of additional temporary subnets allowed without triggering a restart",
				Value:    -1, // -1 indicates not set via CLI
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
			// Create configuration
			cfg, err := createConfig(c)
			if err != nil {
				return err
			}

			// Apply log level
			if cfg.LogLevel != "" {
				level, lerr := logrus.ParseLevel(cfg.LogLevel)
				if lerr != nil {
					log.WithField("level", cfg.LogLevel).WithError(lerr).Warn("Invalid log level, defaulting to info")

					level = logrus.InfoLevel
				}

				log.SetLevel(level)
				log.WithField("level", level.String()).Info("Log level set")
			}

			// Add journald hook if running under systemd
			if cfg.IsRunMethodSystemd() {
				hook, herr := journalhook.NewJournalHook()
				if herr == nil {
					log.AddHook(hook)
					log.Info("Added systemd journal hook for priority mapping")
				} else {
					log.WithError(herr).Warn("Failed to add systemd journal hook")
				}
			}

			// Create application
			app, err := application.New(application.Options{
				Config: cfg,
				Logger: log.WithField("module", "contributoor"),
				Debug:  c.Bool("debug"),
			})
			if err != nil {
				return fmt.Errorf("failed to create application: %w", err)
			}

			log.WithFields(logrus.Fields{
				"config_path": cfg.ContributoorDirectory,
				"version":     cfg.Version,
				"commit":      contr.GitCommit,
				"release":     contr.Release,
			}).Info("Starting contributoor")

			// Start application
			if err := app.Start(ctx); err != nil {
				return fmt.Errorf("failed to start application: %w", err)
			}

			// Wait for shutdown
			<-ctx.Done()

			// Stop application
			if err := app.Stop(context.Background()); err != nil {
				log.WithError(err).Error("Failed to stop application")
			}

			return nil
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		cancel()
		log.Fatal(err)
	}
}

// createConfig creates a configuration from CLI flags and environment variables.
func createConfig(c *cli.Context) (*config.Config, error) {
	cfg := config.NewDefaultConfig()

	// Load from file if specified
	configLocation := c.String("config")
	if configLocation != "" {
		configFromFile, err := config.NewConfigFromPath(configLocation)
		if err != nil {
			return nil, err
		}

		cfg = configFromFile
	}

	// Apply overrides from flags and environment variables
	if err := applyConfigOverridesFromFlags(cfg, c); err != nil {
		return nil, fmt.Errorf("failed to apply config overrides: %w", err)
	}

	return cfg, nil
}
