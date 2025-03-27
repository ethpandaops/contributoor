// Package services provides Ethereum consensus client related functionality
package services

import "strings"

// Client represents an Ethereum consensus client implementation.
type Client string

const (
	// ClientUnknown represents an unknown or unidentified consensus client.
	ClientUnknown Client = "unknown"
	// ClientLighthouse represents the Lighthouse consensus client.
	ClientLighthouse Client = "lighthouse"
	// ClientNimbus represents the Nimbus consensus client.
	ClientNimbus Client = "nimbus"
	// ClientTeku represents the Teku consensus client.
	ClientTeku Client = "teku"
	// ClientPrysm represents the Prysm consensus client.
	ClientPrysm Client = "prysm"
	// ClientLodestar represents the Lodestar consensus client.
	ClientLodestar Client = "lodestar"
	// ClientGrandine represents the Grandine consensus client.
	ClientGrandine Client = "grandine"
	// ClientCaplin represents the Caplin consensus client.
	ClientCaplin Client = "caplin"
)

// AllClients contains all known consensus client implementations.
var AllClients = []Client{
	ClientUnknown,
	ClientLighthouse,
	ClientNimbus,
	ClientTeku,
	ClientPrysm,
	ClientLodestar,
	ClientGrandine,
	ClientCaplin,
}

// clientIdentifiers maps client-specific strings to their respective Client type.
var clientIdentifiers = map[string]Client{
	"lighthouse": ClientLighthouse,
	"nimbus":     ClientNimbus,
	"teku":       ClientTeku,
	"prysm":      ClientPrysm,
	"lodestar":   ClientLodestar,
	"grandine":   ClientGrandine,
	"caplin":     ClientCaplin,
}

// ClientFromString identifies a consensus client from a string identifier.
// It performs a case-insensitive search for known client names within the input string.
// Returns ClientUnknown if no known client is identified.
func ClientFromString(client string) Client {
	asLower := strings.ToLower(client)

	for identifier, clientType := range clientIdentifiers {
		if strings.Contains(asLower, identifier) {
			return clientType
		}
	}

	return ClientUnknown
}
