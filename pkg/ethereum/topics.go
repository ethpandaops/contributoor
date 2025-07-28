package ethereum

const (
	TopicBlock               = "block"
	TopicBlockGossip         = "block_gossip"
	TopicHead                = "head"
	TopicFinalizedCheckpoint = "finalized_checkpoint"
	TopicBlobSidecar         = "blob_sidecar"
	TopicChainReorg          = "chain_reorg"
	TopicSingleAttestation   = "single_attestation"
)

// Define all available topics.
var defaultAllTopics = []string{
	TopicBlock,
	TopicBlockGossip,
	TopicHead,
	TopicFinalizedCheckpoint,
	TopicBlobSidecar,
	TopicChainReorg,
	TopicSingleAttestation,
}

// Define opt-in topics.
var optInTopics = []string{
	TopicSingleAttestation,
}

// GetDefaultAllTopics returns all available topics.
func GetDefaultAllTopics() []string {
	return defaultAllTopics
}

// GetOptInTopics returns opt-in topics.
func GetOptInTopics() []string {
	return optInTopics
}
