package ethereum_test

import (
	"context"
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
