package ethereum

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

//go:generate mockgen -package mock -destination mock/node_identity.mock.go github.com/ethpandaops/contributoor/pkg/ethereum NodeIdentity

// NodeIdentity provides access to beacon node identity information.
type NodeIdentity interface {
	// Start fetches and stores the node identity.
	Start(ctx context.Context) error
	// Stop performs cleanup.
	Stop() error
	// GetAttnets returns the list of subscribed attestation subnet IDs.
	GetAttnets() []int
}

// NodeIdentityData represents the beacon node identity response.
type NodeIdentityData struct {
	PeerID             string               `json:"peer_id"` //nolint:tagliatelle // beacon API uses snake_case
	ENR                string               `json:"enr"`
	P2PAddresses       []string             `json:"p2p_addresses"`       //nolint:tagliatelle // beacon API uses snake_case
	DiscoveryAddresses []string             `json:"discovery_addresses"` //nolint:tagliatelle // beacon API uses snake_case
	Metadata           NodeIdentityMetadata `json:"metadata"`
}

// NodeIdentityMetadata contains the node's metadata.
type NodeIdentityMetadata struct {
	SeqNumber         string `json:"seq_number"` //nolint:tagliatelle // beacon API uses snake_case
	Attnets           string `json:"attnets"`
	Syncnets          string `json:"syncnets"`
	CustodyGroupCount string `json:"custody_group_count"` //nolint:tagliatelle // beacon API uses snake_case
}

// nodeIdentityFullResponse wraps the identity data with full structure.
type nodeIdentityFullResponse struct {
	Data NodeIdentityData `json:"data"`
}

// nodeIdentity implements NodeIdentity.
type nodeIdentity struct {
	log        logrus.FieldLogger
	address    string
	headers    map[string]string
	httpClient *http.Client

	mu       sync.RWMutex
	identity *NodeIdentityData
}

// NewNodeIdentity creates a new NodeIdentity service.
func NewNodeIdentity(log logrus.FieldLogger, address string, headers map[string]string) NodeIdentity {
	return &nodeIdentity{
		log:     log.WithField("component", "node_identity"),
		address: address,
		headers: headers,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start fetches and stores the node identity.
func (n *nodeIdentity) Start(ctx context.Context) error {
	identity, err := n.fetchIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch node identity: %w", err)
	}

	n.mu.Lock()
	n.identity = identity
	n.mu.Unlock()

	n.log.WithFields(logrus.Fields{
		"peer_id":       identity.PeerID,
		"attnets_count": len(n.GetAttnets()),
	}).Info("Node identity fetched successfully")

	return nil
}

// Stop performs cleanup.
func (n *nodeIdentity) Stop() error {
	return nil
}

// GetAttnets returns the list of subscribed attestation subnet IDs.
func (n *nodeIdentity) GetAttnets() []int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.identity == nil {
		return nil
	}

	subnets, err := parseAttnetsBitmask(n.identity.Metadata.Attnets)
	if err != nil {
		n.log.WithError(err).Warn("Failed to parse attnets bitmask")

		return nil
	}

	return subnets
}

// fetchIdentity fetches the node identity from the beacon node.
func (n *nodeIdentity) fetchIdentity(ctx context.Context) (*NodeIdentityData, error) {
	url := fmt.Sprintf("%s/eth/v1/node/identity", n.address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range n.headers {
		req.Header.Set(key, value)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch identity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var identityResp nodeIdentityFullResponse
	if err := json.NewDecoder(resp.Body).Decode(&identityResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if identityResp.Data.PeerID == "" {
		return nil, errors.New("empty peer ID in response")
	}

	return &identityResp.Data, nil
}

// parseAttnetsBitmask returns active subnet ids from the attnets hex string.
func parseAttnetsBitmask(attnets string) ([]int, error) {
	attnets = strings.TrimPrefix(attnets, "0x")

	bytes, err := hex.DecodeString(attnets)
	if err != nil {
		return nil, fmt.Errorf("failed to decode attnets hex: %w", err)
	}

	var activeSubnets []int

	maxBytes := len(bytes)
	if maxBytes > 8 {
		maxBytes = 8
	}

	for byteIdx := 0; byteIdx < maxBytes; byteIdx++ {
		b := bytes[byteIdx]
		for bit := 0; bit < 8; bit++ {
			if b&(1<<(7-bit)) != 0 {
				subnetID := byteIdx*8 + bit
				if subnetID < 64 {
					activeSubnets = append(activeSubnets, subnetID)
				}
			}
		}
	}

	return activeSubnets, nil
}
