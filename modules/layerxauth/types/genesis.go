package types

// DefaultGenesis returns an independent Params-only genesis with the module
// unpaused and idempotency pruning disabled.
func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

// Validate delegates to Params.Validate. Malformed protobuf input must be
// rejected during decoding, before validating the resulting GenesisState.
func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
