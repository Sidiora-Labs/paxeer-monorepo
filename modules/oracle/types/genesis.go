package types

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
)

const GenesisExchangeRateVersion uint32 = 1

// NewGenesisState creates a new GenesisState object
func NewGenesisState(
	params Params, rates []ExchangeRateTuple,
	feederDelegations []FeederDelegation, penaltyCounters []PenaltyCounter,
	aggregateExchangeRateVotes []AggregateExchangeRateVote,
	priceSnapshots []PriceSnapshot,
) *GenesisState {
	return &GenesisState{
		Params:                     params,
		ExchangeRates:              rates,
		FeederDelegations:          feederDelegations,
		PenaltyCounters:            penaltyCounters,
		AggregateExchangeRateVotes: aggregateExchangeRateVotes,
		PriceSnapshots:             priceSnapshots,
	}
}

// DefaultGenesisState - default GenesisState used by columbus-2
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:                     DefaultParams(),
		ExchangeRateVersion:        GenesisExchangeRateVersion,
		OracleExchangeRates:        []GenesisExchangeRate{},
		ExchangeRates:              []ExchangeRateTuple{},
		FeederDelegations:          []FeederDelegation{},
		PenaltyCounters:            []PenaltyCounter{},
		AggregateExchangeRateVotes: []AggregateExchangeRateVote{},
		PriceSnapshots:             PriceSnapshots{},
	}
}

// ValidateGenesis validates the oracle genesis state
func ValidateGenesis(data *GenesisState) error {
	if err := data.Params.Validate(); err != nil {
		return err
	}
	return ValidateGenesisExchangeRates(data)
}

// ValidateGenesisExchangeRates rejects unknown formats and ambiguous imports.
func ValidateGenesisExchangeRates(data *GenesisState) error {
	switch data.ExchangeRateVersion {
	case 0:
		if len(data.OracleExchangeRates) != 0 {
			return fmt.Errorf("legacy oracle genesis cannot contain versioned exchange rates")
		}
		// Legacy rates lack freshness metadata and remain unavailable on import.
		return nil
	case GenesisExchangeRateVersion:
		if len(data.ExchangeRates) != 0 {
			return fmt.Errorf("versioned oracle genesis cannot contain legacy exchange rates")
		}
	default:
		return fmt.Errorf("unsupported oracle genesis exchange rate version %d", data.ExchangeRateVersion)
	}
	seen := make(map[string]struct{}, len(data.OracleExchangeRates))
	for _, entry := range data.OracleExchangeRates {
		if entry.Denom == "" {
			return fmt.Errorf("oracle genesis exchange rate denom is empty")
		}
		if _, exists := seen[entry.Denom]; exists {
			return fmt.Errorf("duplicate oracle genesis exchange rate %q", entry.Denom)
		}
		seen[entry.Denom] = struct{}{}
		rate := entry.OracleExchangeRate
		if rate.ExchangeRate.IsNil() || rate.LastUpdate.IsNil() {
			return fmt.Errorf("oracle genesis exchange rate %q is missing rate or source height", entry.Denom)
		}
		if rate.LastUpdate.IsNegative() {
			return fmt.Errorf("oracle genesis exchange rate %q has a negative source height", entry.Denom)
		}
	}
	return nil
}

// GetGenesisStateFromAppState returns modules/oracle GenesisState given raw application
// genesis state.
func GetGenesisStateFromAppState(cdc codec.JSONCodec, appState map[string]json.RawMessage) *GenesisState {
	var genesisState GenesisState

	if appState[ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[ModuleName], &genesisState)
	}

	return &genesisState
}
