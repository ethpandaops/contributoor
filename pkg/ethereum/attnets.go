package ethereum

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NodeIdentityResponse represents the response from /eth/v1/node/identity endpoint.
type NodeIdentityResponse struct {
	Data struct {
		Metadata struct {
			Attnets string `json:"attnets"`
		} `json:"metadata"`
	} `json:"data"`
}

// CheckAttestationSubnetParticipation checks how many attestation subnets the beacon node is participating in.
func CheckAttestationSubnetParticipation(ctx context.Context, address string, headers map[string]string) (int, error) {
	// Ensure address ends without trailing slash
	address = strings.TrimRight(address, "/")

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/eth/v1/node/identity", nil)
	if err != nil {
		return -1, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Create client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return -1, fmt.Errorf("failed to fetch node identity: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return -1, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var identity NodeIdentityResponse
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return -1, fmt.Errorf("failed to decode identity response: %w", err)
	}

	// Parse attnets bitmask
	return ParseAttnetsBitmask(identity.Data.Metadata.Attnets)
}

// ParseAttnetsBitmask counts active subnets from the attnets hex string.
func ParseAttnetsBitmask(attnets string) (int, error) {
	attnets = strings.TrimPrefix(attnets, "0x")

	bytes, err := hex.DecodeString(attnets)
	if err != nil {
		return -1, fmt.Errorf("failed to decode attnets hex: %w", err)
	}

	count := 0

	for _, b := range bytes {
		for bit := 0; bit < 8; bit++ {
			if b&(1<<bit) != 0 {
				count++
			}
		}
	}

	// Cap at 64 (max subnets in Ethereum)
	if count > 64 {
		count = 64
	}

	return count, nil
}
