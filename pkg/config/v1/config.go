package config

import (
	"encoding/json"
	"os"

	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
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
