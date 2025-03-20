package networks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethpandaops/beacon/pkg/beacon/state"
)

type NetworkName string

type Network struct {
	Name                   NetworkName
	ID                     uint64
	DepositContractAddress string
	DepositChainID         uint64
}

var (
	NetworkNameUnknown NetworkName = "unknown"
	NetworkNameMainnet NetworkName = "mainnet"
	NetworkNameSepolia NetworkName = "sepolia"
	NetworkNameHolesky NetworkName = "holesky"
	NetworkNameHoodi   NetworkName = "hoodi"
)

var (
	ErrNetworkNotFound = errors.New("network not found")
)

var (
	// KnownNetworks is a list of known networks with their deposit contract addresses and deposit network IDs that
	// contributoor supports.
	KnownNetworks = []Network{
		{
			Name:                   NetworkNameMainnet,
			ID:                     1,
			DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
			DepositChainID:         1,
		},
		{
			Name:                   NetworkNameSepolia,
			ID:                     11155111,
			DepositContractAddress: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
			DepositChainID:         11155111,
		},
		{
			Name:                   NetworkNameHolesky,
			ID:                     17000,
			DepositContractAddress: "0x4242424242424242424242424242424242424242",
			DepositChainID:         17000,
		},
		{
			Name:                   NetworkNameHoodi,
			ID:                     560048,
			DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
			DepositChainID:         560048,
		},
	}
)

// DeriveFromSpec derives a network from a spec.
// If the deposit contract address and chain ID aren't known by contributoor, then
// it attempts to derive the network name from the CONFIG_NAME.
// If this CONFIG_NAME is one of our known networks, then we return an error.
// This ensures that we:
// - We have safety garauntees for our defined networks
// - We still support networks that are not in our list of known networks.
func DeriveFromSpec(spec *state.Spec) (*Network, error) {
	for _, network := range KnownNetworks {
		if strings.EqualFold(network.DepositContractAddress, spec.DepositContractAddress) &&
			network.DepositChainID == spec.DepositChainID {
			return &network, nil
		}
	}

	// Attempt to support networks that are not in our list of known networks
	// by using the spec config name.
	if spec.ConfigName != "" {
		// Check if the spec config name is one of our known networks
		if _, err := FindByName(NetworkName(spec.ConfigName)); err == nil {
			// We've somehow found a network that is not in our list of known networks
			// but the CONFIG_NAME matches one of our known networks
			// We'll return an error here to ensure that we don't send incorrect network information.
			// Realistically, this should never happen.
			return nil, fmt.Errorf("incorrect network detected: %s", spec.ConfigName)
		}

		// The spec config name is not one of our known networks, so we'll return the network
		// with the given deposit contract address and chain ID.
		return &Network{
			Name:                   NetworkName(spec.ConfigName),
			ID:                     spec.DepositChainID,
			DepositContractAddress: spec.DepositContractAddress,
			DepositChainID:         spec.DepositChainID,
		}, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrNetworkNotFound, spec.ConfigName)
}

// FindByName returns a network with the given name or an error if not found.
func FindByName(name NetworkName) (*Network, error) {
	for _, network := range KnownNetworks {
		if network.Name == name {
			return &network, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNetworkNotFound, name)
}
