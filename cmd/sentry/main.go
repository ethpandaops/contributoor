package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile string
	log     = logrus.New()
)

type ContributoorConfig struct {
	Version               string         `yaml:"version"`
	ContributoorDirectory string         `yaml:"contributoorDirectory"`
	Network               *NetworkConfig `yaml:"network"`
}

type NetworkConfig struct {
	Name              Parameter `yaml:"name"`
	BeaconNodeAddress Parameter `yaml:"beaconNodeAddress"`
}

// A parameter that can be configured by the user
type Parameter struct {
	Value interface{} `yaml:"value,omitempty"`
}

var rootCmd = &cobra.Command{
	Use:   "sentry",
	Short: "Contributoor sentry node",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := loadConfig(cfgFile)
		if err != nil {
			log.Fatal(err)
		}

		logCtx := log.WithFields(logrus.Fields{
			"config_path":    config.ContributoorDirectory,
			"network_name":   config.Network.Name.Value,
			"beacon_address": config.Network.BeaconNodeAddress.Value,
			"version":        config.Version,
		})

		logCtx.Info("Starting sentry...")

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info("Shutting down...")
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func loadConfig(path string) (*ContributoorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &ContributoorConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
