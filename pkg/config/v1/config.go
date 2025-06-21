package config

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
)

const (
	defaultMetricsHost     = "127.0.0.1"
	defaultMetricsPort     = "9090"
	defaultPprofHost       = "127.0.0.1"
	defaultPprofPort       = "6060"
	defaultHealthCheckHost = "127.0.0.1"
	defaultHealthCheckPort = "9191"
)

// NewConfigFromPath loads a config from a YAML file and validates it.
func NewConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var yamlMap map[string]interface{}
	if yerr := yaml.Unmarshal(data, &yamlMap); yerr != nil {
		return nil, yerr
	}

	jsonBytes, err := json.Marshal(yamlMap)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if jerr := protojson.Unmarshal(jsonBytes, cfg); jerr != nil {
		return nil, jerr
	}

	validator, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	if verr := validator.Validate(cfg); verr != nil {
		return nil, verr
	}

	return cfg, nil
}

// NewDefaultConfig returns a new default config.
func NewDefaultConfig() *Config {
	return &Config{
		LogLevel:              "info",
		Version:               "",
		ContributoorDirectory: "~/.contributoor",
		RunMethod:             RunMethod_RUN_METHOD_DOCKER,
		NetworkName:           "",
		BeaconNodeAddress:     "http://localhost:5052",
		MetricsAddress:        "",
		PprofAddress:          "",
		OutputServer: &OutputServer{
			Address:     "xatu.primary.production.platform.ethpandaops.io:443",
			Credentials: "",
			Tls:         true,
		},
	}
}

// DisplayName returns the display name of the network.
func (n NetworkName) DisplayName() string {
	switch n {
	case NetworkName_NETWORK_NAME_MAINNET:
		return "Mainnet"
	case NetworkName_NETWORK_NAME_SEPOLIA:
		return "Sepolia"
	case NetworkName_NETWORK_NAME_HOLESKY:
		return "Holesky"
	case NetworkName_NETWORK_NAME_HOODI:
		return "Hoodi"
	default:
		return "Unknown"
	}
}

// DisplayName returns the display name of the run method.
func (r RunMethod) DisplayName() string {
	switch r {
	case RunMethod_RUN_METHOD_DOCKER:
		return "Docker"
	case RunMethod_RUN_METHOD_SYSTEMD:
		return "Systemd"
	case RunMethod_RUN_METHOD_BINARY:
		return "Binary"
	default:
		return "Unknown"
	}
}

// ParseAddress parses an address string into host and port components.
// If the address is empty, returns the default host and port.
// If only port is specified (":8080"), returns default host and the specified port.
func ParseAddress(address, defaultHost, defaultPort string) (host, port string) {
	if address == "" {
		return defaultHost, defaultPort
	}

	// Handle ":port" format.
	if strings.HasPrefix(address, ":") {
		return defaultHost, strings.TrimPrefix(address, ":")
	}

	// Parse as URL to handle http:// format.
	u, err := url.Parse(address)
	if err == nil && u.Host != "" {
		h, p, e := net.SplitHostPort(u.Host)
		if e == nil {
			return h, p
		}
	}

	// Try to split raw host:port.
	host, port, err = net.SplitHostPort(address)
	if err == nil {
		return host, port
	}

	return defaultHost, defaultPort
}

// GetMetricsHostPort returns the metrics host and port.
// If MetricsAddress is not set, returns default values.
func (c *Config) GetMetricsHostPort() (host, port string) {
	if c.MetricsAddress == "" {
		return "", ""
	}

	return ParseAddress(c.MetricsAddress, defaultMetricsHost, defaultMetricsPort)
}

// GetPprofHostPort returns the pprof host and port.
// If PprofAddress is not set, returns empty strings.
func (c *Config) GetPprofHostPort() (host, port string) {
	if c.PprofAddress == "" {
		return "", ""
	}

	return ParseAddress(c.PprofAddress, defaultPprofHost, defaultPprofPort)
}

// GetHealthCheckHostPort returns the health check host and port.
// If HealthCheckAddress is not set, returns empty strings.
func (c *Config) GetHealthCheckHostPort() (host, port string) {
	if c.HealthCheckAddress == "" {
		return "", ""
	}

	return ParseAddress(c.HealthCheckAddress, defaultHealthCheckHost, defaultHealthCheckPort)
}

// SetNetwork sets the network name.
func (c *Config) SetNetwork(network string) error {
	if network == "" {
		return errors.New("network is required")
	}

	// Just set the network name directly as a string
	c.NetworkName = network

	return nil
}

// SetBeaconNodeAddress sets the beacon node address.
func (c *Config) SetBeaconNodeAddress(address string) {
	if address == "" {
		return
	}

	c.BeaconNodeAddress = address
}

// SetMetricsAddress sets the metrics address.
func (c *Config) SetMetricsAddress(address string) {
	if address == "" {
		return
	}

	c.MetricsAddress = address
}

// SetHealthCheckAddress sets the health check address.
func (c *Config) SetHealthCheckAddress(address string) {
	if address == "" {
		return
	}

	c.HealthCheckAddress = address
}

// SetLogLevel sets the log level.
func (c *Config) SetLogLevel(level string) {
	if level == "" {
		return
	}

	c.LogLevel = level
}

// SetOutputServerAddress sets the output server address.
func (c *Config) SetOutputServerAddress(address string) {
	if address == "" {
		return
	}

	if c.OutputServer == nil {
		c.OutputServer = &OutputServer{}
	}

	c.OutputServer.Address = address
}

// SetOutputServerCredentials sets the output server credentials.
func (c *Config) SetOutputServerCredentials(creds string) {
	if creds == "" {
		return
	}

	if c.OutputServer == nil {
		c.OutputServer = &OutputServer{}
	}

	c.OutputServer.Credentials = creds
}

// SetOutputServerTLS sets whether to use TLS for the output server.
func (c *Config) SetOutputServerTLS(useTLS bool) {
	if c.OutputServer == nil {
		c.OutputServer = &OutputServer{}
	}

	c.OutputServer.Tls = useTLS
}

// SetContributoorDirectory sets the contributoor directory.
func (c *Config) SetContributoorDirectory(dir string) {
	if dir == "" {
		return
	}

	c.ContributoorDirectory = dir
}

// IsRunMethodSystemd determines if contributoor is being run via systemd.
func (c *Config) IsRunMethodSystemd() bool {
	if c.RunMethod == RunMethod_RUN_METHOD_SYSTEMD {
		return true
	}

	// INVOCATION_ID is set by systemd for each invocation.
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}

	// JOURNAL_STREAM is set when stdout/stderr are connected to the journal.
	if os.Getenv("JOURNAL_STREAM") != "" {
		return true
	}

	// Additional check for systemd notify socket.
	if os.Getenv("NOTIFY_SOCKET") != "" {
		return true
	}

	return false
}
