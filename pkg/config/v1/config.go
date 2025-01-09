package config

import (
	"encoding/json"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
)

const (
	defaultMetricsHost = "127.0.0.1"
	defaultMetricsPort = "9090"
	defaultPprofHost   = "127.0.0.1"
	defaultPprofPort   = "6060"
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

// DisplayName returns the display name of the network.
func (n NetworkName) DisplayName() string {
	switch n {
	case NetworkName_NETWORK_NAME_MAINNET:
		return "Mainnet"
	case NetworkName_NETWORK_NAME_SEPOLIA:
		return "Sepolia"
	case NetworkName_NETWORK_NAME_HOLESKY:
		return "Holesky"
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
