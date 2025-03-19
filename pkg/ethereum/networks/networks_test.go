package networks

import (
	"testing"

	"github.com/ethpandaops/beacon/pkg/beacon/state"
	"github.com/stretchr/testify/assert"
)

// Mock state.Spec for testing
func createMockSpec(depositContractAddress string, depositChainID uint64, configName string) *state.Spec {
	return &state.Spec{
		DepositContractAddress: depositContractAddress,
		DepositChainID:         depositChainID,
		ConfigName:             configName,
	}
}

func TestNetworkConstants(t *testing.T) {
	// Verify network name constants
	assert.Equal(t, NetworkName("unknown"), NetworkNameUnknown)
	assert.Equal(t, NetworkName("mainnet"), NetworkNameMainnet)
	assert.Equal(t, NetworkName("sepolia"), NetworkNameSepolia)
	assert.Equal(t, NetworkName("holesky"), NetworkNameHolesky)
	assert.Equal(t, NetworkName("hoodi"), NetworkNameHoodi)
}

func TestKnownNetworks(t *testing.T) {
	// Verify KnownNetworks has the expected number of entries
	assert.Len(t, KnownNetworks, 4)

	// Verify specific network properties
	mainnetFound := false
	holeskiFound := false

	for _, network := range KnownNetworks {
		if network.Name == NetworkNameMainnet {
			mainnetFound = true
			assert.Equal(t, uint64(1), network.ID)
			assert.Equal(t, "0x00000000219ab540356cBB839Cbe05303d7705Fa", network.DepositContractAddress)
			assert.Equal(t, uint64(1), network.DepositChainID)
		}

		if network.Name == NetworkNameHolesky {
			holeskiFound = true
			assert.Equal(t, uint64(17000), network.ID)
			assert.Equal(t, "0x4242424242424242424242424242424242424242", network.DepositContractAddress)
			assert.Equal(t, uint64(17000), network.DepositChainID)
		}
	}

	assert.True(t, mainnetFound, "Mainnet network not found in KnownNetworks")
	assert.True(t, holeskiFound, "Holesky network not found in KnownNetworks")
}

func TestDeriveFromSpec(t *testing.T) {
	tests := []struct {
		name                  string
		spec                  *state.Spec
		expectedNetwork       *Network
		expectedErrorContains string
	}{
		{
			name: "mainnet",
			spec: createMockSpec("0x00000000219ab540356cBB839Cbe05303d7705Fa", 1, "mainnet"),
			expectedNetwork: &Network{
				Name:                   NetworkNameMainnet,
				ID:                     1,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         1,
			},
		},
		{
			name: "sepolia",
			spec: createMockSpec("0x7f02c3e3c98b133055b8b348b2ac625669ed295d", 11155111, "sepolia"),
			expectedNetwork: &Network{
				Name:                   NetworkNameSepolia,
				ID:                     11155111,
				DepositContractAddress: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
				DepositChainID:         11155111,
			},
		},
		{
			name: "holesky",
			spec: createMockSpec("0x4242424242424242424242424242424242424242", 17000, "holesky"),
			expectedNetwork: &Network{
				Name:                   NetworkNameHolesky,
				ID:                     17000,
				DepositContractAddress: "0x4242424242424242424242424242424242424242",
				DepositChainID:         17000,
			},
		},
		{
			name: "hoodi",
			spec: createMockSpec("0x00000000219ab540356cBB839Cbe05303d7705Fa", 560048, "hoodi"),
			expectedNetwork: &Network{
				Name:                   NetworkNameHoodi,
				ID:                     560048,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         560048,
			},
		},
		{
			name:                  "unknown network with no config name",
			spec:                  createMockSpec("0x1111111111111111111111111111111111111111", 123456, ""),
			expectedErrorContains: "network not found",
		},
		{
			name: "unknown network with custom config name",
			spec: createMockSpec("0x1111111111111111111111111111111111111111", 123456, "custom-testnet"),
			expectedNetwork: &Network{
				Name:                   "custom-testnet",
				ID:                     123456,
				DepositContractAddress: "0x1111111111111111111111111111111111111111",
				DepositChainID:         123456,
			},
		},
		{
			name:                  "conflicting config name",
			spec:                  createMockSpec("0x1111111111111111111111111111111111111111", 123456, "mainnet"),
			expectedErrorContains: "network not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := DeriveFromSpec(tt.spec)

			if tt.expectedErrorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, network)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedNetwork.Name, network.Name)
				assert.Equal(t, tt.expectedNetwork.ID, network.ID)
				assert.Equal(t, tt.expectedNetwork.DepositContractAddress, network.DepositContractAddress)
				assert.Equal(t, tt.expectedNetwork.DepositChainID, network.DepositChainID)
			}
		})
	}
}

func TestFindByName(t *testing.T) {
	tests := []struct {
		name                  string
		networkName           NetworkName
		expectedNetwork       *Network
		expectedErrorContains string
	}{
		{
			name:        "mainnet",
			networkName: NetworkNameMainnet,
			expectedNetwork: &Network{
				Name:                   NetworkNameMainnet,
				ID:                     1,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         1,
			},
		},
		{
			name:        "sepolia",
			networkName: NetworkNameSepolia,
			expectedNetwork: &Network{
				Name:                   NetworkNameSepolia,
				ID:                     11155111,
				DepositContractAddress: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
				DepositChainID:         11155111,
			},
		},
		{
			name:                  "unknown network",
			networkName:           "invalid-network",
			expectedErrorContains: "network not found",
		},
		{
			name:                  "unknown network with value",
			networkName:           NetworkNameUnknown,
			expectedErrorContains: "network not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := FindByName(tt.networkName)

			if tt.expectedErrorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, network)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedNetwork.Name, network.Name)
				assert.Equal(t, tt.expectedNetwork.ID, network.ID)
				assert.Equal(t, tt.expectedNetwork.DepositContractAddress, network.DepositContractAddress)
				assert.Equal(t, tt.expectedNetwork.DepositChainID, network.DepositChainID)
			}
		})
	}
}
