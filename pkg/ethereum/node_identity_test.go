package ethereum_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeIdentity_Start(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		statusCode    int
		expectError   bool
		expectedPeer  string
		expectedENR   string
		expectedCount int
	}{
		{
			name: "successful identity fetch",
			response: `{
				"data": {
					"peer_id": "16Uiu2HAmCX8GX9dfHhSFQBRpATmB5kLuJMfTphKNgNg1iyvsgBKj",
					"enr": "enr:-Mu4QNCOSaZHwr4n-ZFi1GdReBAVDxBaa1TIApAVc26XiEXcTGhuNsYMQUfZKFvlvGhNo5UniU2sCrKRCLLwOx1nJ4gJh2F0dG5ldHOIAAAAABgAAACDY2NjgYCEZXRoMpC2LysOcJN1RP__________gmlkgnY0gmlwhJ9ZpJODbmZkhAAAAACJc2VjcDI1NmsxoQL9_E5h0zKeyxetPAst-T-T3ezyY5ACks6VI4YHvU1pfohzeW5jbmV0cw6DdGNwgiMog3VkcIIjKA",
					"p2p_addresses": ["/ip4/127.0.0.1/tcp/9000/p2p/16Uiu2HAmCX8GX9dfHhSFQBRpATmB5kLuJMfTphKNgNg1iyvsgBKj"],
					"discovery_addresses": ["/ip4/159.89.164.147/tcp/9000"],
					"metadata": {
						"seq_number": "2",
						"attnets": "0x0000000018000000",
						"syncnets": "0x0e",
						"custody_group_count": "128"
					}
				}
			}`,
			statusCode:    http.StatusOK,
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:        "http error",
			response:    `{"error": "not found"}`,
			statusCode:  http.StatusNotFound,
			expectError: true,
		},
		{
			name:        "invalid json response",
			response:    `{invalid json}`,
			statusCode:  http.StatusOK,
			expectError: true,
		},
		{
			name: "empty peer id",
			response: `{
				"data": {
					"peer_id": "",
					"metadata": {
						"attnets": "0x0000000000000000"
					}
				}
			}`,
			statusCode:  http.StatusOK,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/eth/v1/node/identity", r.URL.Path)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Create identity service
			log := logrus.New()
			log.SetLevel(logrus.DebugLevel)
			identity := ethereum.NewNodeIdentity(log, server.URL, nil)

			// Start the service
			err := identity.Start(context.Background())

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(identity.GetAttnets()))
			}

			// Stop the service
			assert.NoError(t, identity.Stop())
		})
	}
}

func TestNodeIdentity_GetMethods(t *testing.T) {
	// Create identity service without starting it
	log := logrus.New()
	identity := ethereum.NewNodeIdentity(log, "http://localhost:5052", nil)

	// Should return nil
	assert.Nil(t, identity.GetAttnets())
}

func TestNodeIdentity_Headers(t *testing.T) {
	// Create test server that verifies headers
	expectedHeaders := map[string]string{
		"Authorization": "Bearer token",
		"X-Custom":      "value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		for key, value := range expectedHeaders {
			assert.Equal(t, value, r.Header.Get(key))
		}

		// Return valid response
		response := `{
			"data": {
				"peer_id": "test-peer",
				"enr": "test-enr",
				"metadata": {
					"attnets": "0x00"
				}
			}
		}`

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create identity service with headers
	log := logrus.New()
	identity := ethereum.NewNodeIdentity(log, server.URL, expectedHeaders)

	// Start should succeed
	err := identity.Start(context.Background())
	require.NoError(t, err)
}

// TestNodeIdentity_GetAttnets tests the GetAttnets functionality through the public interface.
func TestNodeIdentity_GetAttnets(t *testing.T) {
	tests := []struct {
		name            string
		attnetsHex      string
		expectedSubnets []int
	}{
		{
			name:            "no subnets",
			attnetsHex:      "0x0000000000000000",
			expectedSubnets: nil,
		},
		{
			name:            "typical real-world example (2 subnets - 35, 36)",
			attnetsHex:      "0x0000000018000000",
			expectedSubnets: []int{35, 36},
		},
		{
			name:            "multiple subnets",
			attnetsHex:      "0xFF00000000000000",
			expectedSubnets: []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with specific attnets
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := `{
					"data": {
						"peer_id": "test-peer",
						"enr": "test-enr",
						"metadata": {
							"attnets": "` + tt.attnetsHex + `"
						}
					}
				}`

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()

			// Create and start identity service
			log := logrus.New()
			identity := ethereum.NewNodeIdentity(log, server.URL, nil)
			err := identity.Start(context.Background())
			require.NoError(t, err)

			// Check the subnets
			subnets := identity.GetAttnets()
			assert.Equal(t, tt.expectedSubnets, subnets)
		})
	}
}

func TestParseAttnetsBitmask(t *testing.T) {
	tests := []struct {
		name     string
		attnets  string
		expected []int
	}{
		{
			name:     "empty attnets",
			attnets:  "0x0000000000000000",
			expected: nil, // ParseAttnetsBitmask returns nil for empty, not []int{}
		},
		{
			name:     "single subnet 0",
			attnets:  "0x0100000000000000",
			expected: []int{0},
		},
		{
			name:     "single subnet 7",
			attnets:  "0x8000000000000000",
			expected: []int{7},
		},
		{
			name:     "single subnet 8",
			attnets:  "0x0001000000000000",
			expected: []int{8},
		},
		{
			name:     "single subnet 52",
			attnets:  "0x0000000000001000",
			expected: []int{52},
		},
		{
			name:     "single subnet 53",
			attnets:  "0x0000000000002000",
			expected: []int{53},
		},
		{
			name:     "subnets 50 and 51 (0x0c = 0b00001100 in byte 6, LSB-first)",
			attnets:  "0x0000000000000c00",
			expected: []int{50, 51},
		},
		{
			name:     "subnets 52 and 53 (0x30 = 0b00110000 in byte 6, LSB-first)",
			attnets:  "0x0000000000003000",
			expected: []int{52, 53},
		},
		{
			name:     "multiple subnets in first byte (0, 1, 7)",
			attnets:  "0x8300000000000000",
			expected: []int{0, 1, 7},
		},
		{
			name:     "subnets across multiple bytes",
			attnets:  "0x0303000000000000",
			expected: []int{0, 1, 8, 9},
		},
		{
			name:     "all subnets in first byte",
			attnets:  "0xff00000000000000",
			expected: []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			name:     "with 0x prefix",
			attnets:  "0x0100000000000000",
			expected: []int{0},
		},
		{
			name:     "without 0x prefix",
			attnets:  "0100000000000000",
			expected: []int{0},
		},
		{
			name:     "real example - subnets 46 and 47",
			attnets:  "0x0000000000c00000",
			expected: []int{46, 47},
		},
		{
			name:     "max subnet 63",
			attnets:  "0x0000000000000080",
			expected: []int{63},
		},
		{
			name:     "test MSB vs LSB - 0x01 should be subnet 0, not subnet 7",
			attnets:  "0x0100000000000000",
			expected: []int{0}, // LSB-first: bit 0
		},
		{
			name:     "test MSB vs LSB - 0x80 should be subnet 7, not subnet 0",
			attnets:  "0x8000000000000000",
			expected: []int{7}, // LSB-first: bit 7
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ethereum.ParseAttnetsBitmask(tt.attnets)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAttnetsBitmask_Error(t *testing.T) {
	tests := []struct {
		name    string
		attnets string
	}{
		{
			name:    "invalid hex",
			attnets: "0xgggggggggggggggg",
		},
		{
			name:    "odd length hex",
			attnets: "0x123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ethereum.ParseAttnetsBitmask(tt.attnets)
			assert.Error(t, err)
		})
	}
}

func TestParseAttnetsBitmask_BitOrdering(t *testing.T) {
	// This test specifically verifies LSB-first bit ordering
	// to ensure the fix for the MSB/LSB issue doesn't regress

	// Test each bit position in a byte
	for bitPos := 0; bitPos < 8; bitPos++ {
		// Create a bitmask with only one bit set
		byteValue := byte(1 << bitPos)
		attnets := fmt.Sprintf("0x%02x00000000000000", byteValue)

		result, err := ethereum.ParseAttnetsBitmask(attnets)
		require.NoError(t, err)
		require.Len(t, result, 1, "Expected exactly one subnet for bit %d", bitPos)
		assert.Equal(t, bitPos, result[0], "Bit %d should map to subnet %d", bitPos, bitPos)
	}

	// Test that 0x0c (0b00001100) in byte 6 gives subnets 50 and 51
	// This was the actual bug case from the logs where beacon advertised [52, 53]
	// but we incorrectly parsed 0x0000000000000c00 as [52, 53] instead of [50, 51]
	result, err := ethereum.ParseAttnetsBitmask("0x0000000000000c00")
	require.NoError(t, err)
	assert.Equal(t, []int{50, 51}, result, "0x0c in byte 6 should be subnets 50 and 51 (LSB-first), not 52 and 53 (MSB-first)")
}
