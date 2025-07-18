package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// applyConfigOverridesFromFlags applies CLI flags to the config if they are set.
func applyConfigOverridesFromFlags(cfg *config.Config, c *cli.Context) error {
	// Apply network configuration
	if err := applyNetworkConfig(cfg, c); err != nil {
		return err
	}

	// Apply service addresses
	applyServiceAddresses(cfg, c)

	// Apply logging configuration
	applyLoggingConfig(cfg, c)

	// Apply output server configuration
	if err := applyOutputServerConfig(cfg, c); err != nil {
		return err
	}

	// Apply contributoor directory
	applyContributoorDirectory(cfg, c)

	// Apply attestation subnet configuration
	if err := applyAttestationSubnetConfig(cfg, c); err != nil {
		return err
	}

	return nil
}

// applyNetworkConfig applies network configuration from env and CLI.
func applyNetworkConfig(cfg *config.Config, c *cli.Context) error {
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

	return nil
}

// applyServiceAddresses applies service address configurations.
func applyServiceAddresses(cfg *config.Config, c *cli.Context) {
	// Beacon node address
	if addr := os.Getenv("CONTRIBUTOOR_BEACON_NODE_ADDRESS"); addr != "" {
		log.Infof("Setting beacon node address from env")
		cfg.SetBeaconNodeAddress(addr)
	}

	if c.String("beacon-node-address") != "" {
		log.Infof("Overriding beacon node address from CLI")
		cfg.SetBeaconNodeAddress(c.String("beacon-node-address"))
	}

	// Metrics address
	if addr := os.Getenv("CONTRIBUTOOR_METRICS_ADDRESS"); addr != "" {
		log.Infof("Setting metrics address from env to %s", addr)
		cfg.SetMetricsAddress(addr)
	}

	if c.String("metrics-address") != "" {
		log.Infof("Overriding metrics address from CLI to %s", c.String("metrics-address"))
		cfg.SetMetricsAddress(c.String("metrics-address"))
	}

	// Health check address
	if addr := os.Getenv("CONTRIBUTOOR_HEALTH_CHECK_ADDRESS"); addr != "" {
		log.Infof("Setting health check address from env to %s", addr)
		cfg.SetHealthCheckAddress(addr)
	}

	if c.String("health-check-address") != "" {
		log.Infof("Overriding health check address from CLI to %s", c.String("health-check-address"))
		cfg.SetHealthCheckAddress(c.String("health-check-address"))
	}
}

// applyLoggingConfig applies logging configuration.
func applyLoggingConfig(cfg *config.Config, c *cli.Context) {
	if level := os.Getenv("CONTRIBUTOOR_LOG_LEVEL"); level != "" {
		log.Infof("Setting log level from env to %s", level)
		cfg.SetLogLevel(level)
	}

	if c.String("log-level") != "" {
		log.Infof("Overriding log level from CLI to %s", c.String("log-level"))
		cfg.SetLogLevel(c.String("log-level"))
	}
}

// applyOutputServerConfig applies output server configuration.
func applyOutputServerConfig(cfg *config.Config, c *cli.Context) error {
	// Address
	if addr := os.Getenv("CONTRIBUTOOR_OUTPUT_SERVER_ADDRESS"); addr != "" {
		log.Infof("Setting output server address from env")
		cfg.SetOutputServerAddress(addr)
	}

	if c.String("output-server-address") != "" {
		log.Infof("Overriding output server address from CLI")
		cfg.SetOutputServerAddress(c.String("output-server-address"))
	}

	// Credentials
	if err := applyOutputServerCredentials(cfg, c); err != nil {
		return err
	}

	// TLS
	return applyOutputServerTLS(cfg, c)
}

// applyOutputServerCredentials applies output server credentials.
func applyOutputServerCredentials(cfg *config.Config, c *cli.Context) error {
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

	return nil
}

// applyOutputServerTLS applies output server TLS configuration.
func applyOutputServerTLS(cfg *config.Config, c *cli.Context) error {
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

// applyContributoorDirectory applies contributoor directory configuration.
func applyContributoorDirectory(cfg *config.Config, c *cli.Context) {
	if dir := os.Getenv("CONTRIBUTOOR_DIRECTORY"); dir != "" {
		log.Infof("Setting contributoor directory from env to %s", dir)
		cfg.ContributoorDirectory = dir
	}

	if c.String("contributoor-directory") != "" {
		log.Infof("Overriding contributoor directory from CLI to %s", c.String("contributoor-directory"))
		cfg.ContributoorDirectory = c.String("contributoor-directory")
	}
}

// applyAttestationSubnetConfig applies attestation subnet configuration.
func applyAttestationSubnetConfig(cfg *config.Config, c *cli.Context) error {
	// Handle enabled flag from env
	if enabled := os.Getenv("CONTRIBUTOOR_ATTESTATION_SUBNET_CHECK_ENABLED"); enabled != "" {
		log.Infof("Setting attestation subnet check enabled from env to %s", enabled)

		enabledBool, err := strconv.ParseBool(enabled)
		if err != nil {
			return errors.Wrap(err, "failed to parse attestation subnet check enabled env var")
		}

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.Enabled = enabledBool
		cfg.AttestationSubnetCheck.MaxSubnets = 2
	}

	// Handle max subnets from env
	if maxSubnets := os.Getenv("CONTRIBUTOOR_ATTESTATION_SUBNET_MAX_SUBNETS"); maxSubnets != "" {
		log.Infof("Setting attestation subnet max subnets from env to %s", maxSubnets)

		maxSubnetsInt, err := strconv.ParseUint(maxSubnets, 10, 32)
		if err != nil {
			return errors.Wrap(err, "failed to parse attestation subnet max subnets env var")
		}

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.MaxSubnets = uint32(maxSubnetsInt)
	}

	// CLI flag overrides env var for enabled
	if c.Bool("attestation-subnet-check-enabled") {
		log.Infof("Setting attestation subnet check enabled from CLI to true")

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.Enabled = true
	}

	// CLI flag overrides env var for max subnets
	if c.Int("attestation-subnet-max-subnets") >= 0 { // -1 means not set
		log.Infof("Setting attestation subnet max subnets from CLI to %d", c.Int("attestation-subnet-max-subnets"))

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.MaxSubnets = uint32(c.Int("attestation-subnet-max-subnets")) //nolint:gosec // conversion fine.
	}

	return nil
}
