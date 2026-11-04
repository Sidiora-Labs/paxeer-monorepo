package types

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	for i, association := range gs.AddressAssociations {
		if association == nil {
			return fmt.Errorf("missing address association %d", i+1)
		}
		if _, err := sdk.AccAddressFromBech32(association.PaxAddress); err != nil {
			return fmt.Errorf("invalid address association %d Pax address: %w", i+1, err)
		}
		if !common.IsHexAddress(association.EthAddress) {
			return fmt.Errorf("invalid address association %d Ethereum address", i+1)
		}
	}
	return nil
}

// ValidateStream requires a nonempty stream of chunks, each carrying complete,
// valid Params, including chunks containing only state or repeated parameters.
// Address associations must contain parseable Pax and Ethereum addresses.
// Drain the stream even on error so an unbuffered importer can finish sending.
func ValidateStream(genesisStateCh <-chan GenesisState) error {
	if genesisStateCh == nil {
		return fmt.Errorf("missing genesis stream")
	}
	count := 0
	var validationErr error
	for genesisState := range genesisStateCh {
		count++
		if err := genesisState.Validate(); err != nil && validationErr == nil {
			validationErr = fmt.Errorf("invalid genesis chunk %d: %w", count, err)
		}
	}
	if count == 0 {
		return fmt.Errorf("empty genesis stream: parameters required")
	}
	return validationErr
}
