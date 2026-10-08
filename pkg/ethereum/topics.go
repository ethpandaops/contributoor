package ethereum

const (
	TopicBlock               = "block"
	TopicBlockGossip         = "block_gossip"
	TopicHead                = "head"
	TopicFinalizedCheckpoint = "finalized_checkpoint"
	TopicChainReorg          = "chain_reorg"
	TopicSingleAttestation   = "single_attestation"
	TopicDataColumnSidecar   = "data_column_sidecar"
	TopicFastConfirmation    = "fast_confirmation"

	// Gloas (EIP-7732) topics. Beacon nodes only emit these once Gloas is
	// active; before that the subscriptions simply stay quiet. A beacon node
	// that doesn't know a topic rejects that one stream only (each topic is its
	// own SSE stream), leaving the others unaffected.
	TopicHeadV2                    = "head_v2"
	TopicExecutionPayload          = "execution_payload"
	TopicExecutionPayloadGossip    = "execution_payload_gossip"
	TopicExecutionPayloadAvailable = "execution_payload_available"
	TopicExecutionPayloadBid       = "execution_payload_bid"
	TopicPayloadAttestationMessage = "payload_attestation_message"
	TopicProposerPreferences       = "proposer_preferences"
)

// Define all available topics.
var defaultAllTopics = []string{
	TopicBlock,
	TopicBlockGossip,
	TopicHead,
	TopicFinalizedCheckpoint,
	TopicChainReorg,
	TopicSingleAttestation,
	TopicDataColumnSidecar,
	TopicFastConfirmation,
	TopicHeadV2,
	TopicExecutionPayload,
	TopicExecutionPayloadGossip,
	TopicExecutionPayloadAvailable,
	TopicExecutionPayloadBid,
	TopicPayloadAttestationMessage,
	TopicProposerPreferences,
}

// Define opt-in topics.
//
// payload_attestation_message is opt-in for the same reason as
// single_attestation: the beacon node also emits the PTC votes of its own
// validators, and their near-zero propagation time would link validator
// indices to this node. There is no condition registered for it yet, so it is
// not subscribed to.
var optInTopics = []string{
	TopicSingleAttestation,
	TopicPayloadAttestationMessage,
}

// GetDefaultAllTopics returns all available topics.
func GetDefaultAllTopics() []string {
	return defaultAllTopics
}

// GetOptInTopics returns opt-in topics.
func GetOptInTopics() []string {
	return optInTopics
}
