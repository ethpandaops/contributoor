package ethereum_test

import (
	"context"
	"testing"

	"github.com/ethpandaops/contributoor/pkg/ethereum"
	eth2v1 "github.com/ethpandaops/go-eth2-client/api/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestDefaultTopics_Gloas(t *testing.T) {
	tm := ethereum.NewTopicManager(logrus.New(), &ethereum.TopicConfig{
		AllTopics:   ethereum.GetDefaultAllTopics(),
		OptInTopics: ethereum.GetOptInTopics(),
	})

	enabled := tm.GetEnabledTopics(context.Background())

	// Gloas topics are subscribed by default.
	for _, topic := range []string{
		ethereum.TopicHeadV2,
		ethereum.TopicExecutionPayload,
		ethereum.TopicExecutionPayloadGossip,
		ethereum.TopicExecutionPayloadAvailable,
		ethereum.TopicExecutionPayloadBid,
		ethereum.TopicProposerPreferences,
	} {
		require.Contains(t, enabled, topic)
	}

	// Validator-linkable topics stay opt-in.
	require.NotContains(t, enabled, ethereum.TopicSingleAttestation)
	require.NotContains(t, enabled, ethereum.TopicPayloadAttestationMessage)

	// The pre-Gloas topics are unchanged.
	for _, topic := range []string{
		ethereum.TopicBlock,
		ethereum.TopicBlockGossip,
		ethereum.TopicHead,
		ethereum.TopicFinalizedCheckpoint,
		ethereum.TopicChainReorg,
		ethereum.TopicDataColumnSidecar,
		ethereum.TopicFastConfirmation,
	} {
		require.Contains(t, enabled, topic)
	}
}

// Every topic must be known to go-eth2-client: the upstream subscriber aborts
// the remaining subscriptions on the first topic it rejects locally.
func TestDefaultTopics_SupportedByClient(t *testing.T) {
	for _, topic := range ethereum.GetDefaultAllTopics() {
		require.True(t, eth2v1.SupportedEventTopics[topic], "topic %q not supported by go-eth2-client", topic)
	}
}
