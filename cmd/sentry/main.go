package main

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/bufbuild/protovalidate-go"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile string
	log     = logrus.New()
)

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
			"run_method":     config.RunMethod,
			"network_name":   config.NetworkName,
			"beacon_address": config.BeaconNodeAddress,
			"output_server":  config.OutputServer.Address,
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

func loadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// First unmarshal YAML into a map
	var yamlMap map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlMap); err != nil {
		return nil, err
	}

	// Convert YAML to JSON
	jsonBytes, err := json.Marshal(yamlMap)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{}
	if err := protojson.Unmarshal(jsonBytes, cfg); err != nil {
		return nil, err
	}

	validator, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	if err = validator.Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
