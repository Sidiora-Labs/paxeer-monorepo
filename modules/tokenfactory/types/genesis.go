package types

import (
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// this line is used by starport scaffolding # genesis/types/import

// DefaultIndex is the default capability global index
const DefaultIndex uint64 = 1

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:        DefaultParams(),
		FactoryDenoms: []GenesisDenom{},
		PausedDenoms:  []string{},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	err := gs.Params.Validate()
	if err != nil {
		return err
	}

	seenDenoms := map[string]bool{}
	seenPaused := map[string]bool{}

	for _, denom := range gs.GetFactoryDenoms() {
		if seenDenoms[denom.GetDenom()] {
			return sdkerrors.Wrapf(ErrInvalidGenesis, "duplicate denom: %s", denom.GetDenom())
		}
		seenDenoms[denom.GetDenom()] = true

		_, _, err := DeconstructDenom(denom.GetDenom())
		if err != nil {
			return err
		}

		if err := denom.AuthorityMetadata.Validate(); err != nil {
			return sdkerrors.Wrapf(ErrInvalidAuthorityMetadata, "denom %s: %s", denom.GetDenom(), err)
		}
	}

	for _, denom := range gs.GetPausedDenoms() {
		if !seenDenoms[denom] {
			return sdkerrors.Wrapf(ErrInvalidGenesis, "paused denom %s is not a factory denom", denom)
		}
		if seenPaused[denom] {
			return sdkerrors.Wrapf(ErrInvalidGenesis, "duplicate paused denom: %s", denom)
		}
		seenPaused[denom] = true
	}

	return nil
}
