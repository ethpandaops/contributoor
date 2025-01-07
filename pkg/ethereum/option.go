package ethereum

// Options defines the options for the Ethereum beacon node.
type Options struct {
	FetchBeaconCommittees bool
	FetchProposerDuties   bool
}

// WithFetchBeaconCommittees sets the option to fetch beacon committees.
func (o *Options) WithFetchBeaconCommittees(fetchBeaconCommittees bool) *Options {
	o.FetchBeaconCommittees = fetchBeaconCommittees

	return o
}

// WithFetchProposerDuties sets the option to fetch proposer duties.
func (o *Options) WithFetchProposerDuties(fetchProposerDuties bool) *Options {
	o.FetchProposerDuties = fetchProposerDuties

	return o
}
