package types

// DefaultGenesis returns an independent, params-only genesis with the ledger
// unpaused. Kernel records belong to the shared host store, not this state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

// Validate checks genesis params. A paused ledger is valid genesis state;
// malformed serialized input must first be rejected by the protobuf decoder.
func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
