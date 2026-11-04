package types

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
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
	if data == nil {
		return fmt.Errorf("oracle genesis is nil")
	}
	for _, param := range []struct {
		name  string
		value sdk.Dec
	}{
		{"VoteThreshold", data.Params.VoteThreshold},
		{"RewardBand", data.Params.RewardBand},
		{"SlashFraction", data.Params.SlashFraction},
		{"MinValidPerWindow", data.Params.MinValidPerWindow},
	} {
		if param.value.IsNil() || !param.value.IsInValidRange() {
			return fmt.Errorf("oracle genesis parameter %s is missing or out of range", param.name)
		}
	}
	if err := data.Params.Validate(); err != nil {
		return err
	}
	if err := validateVoteThreshold(data.Params.VoteThreshold); err != nil {
		return err
	}
	if err := validateLookbackDuration(data.Params.LookbackDuration); err != nil {
		return err
	}
	whitelist := make(map[string]struct{}, len(data.Params.Whitelist))
	for _, denom := range data.Params.Whitelist {
		if err := validateGenesisDenom(denom.Name, whitelist); err != nil {
			return fmt.Errorf("oracle genesis whitelist: %w", err)
		}
	}
	if err := ValidateGenesisExchangeRates(data); err != nil {
		return err
	}
	if err := validateGenesisTuples(data.ExchangeRates, false); err != nil {
		return fmt.Errorf("oracle genesis legacy exchange rates: %w", err)
	}
	delegations := make(map[string]struct{}, len(data.FeederDelegations))
	for i, entry := range data.FeederDelegations {
		msg := MsgDelegateFeedConsent{Operator: entry.ValidatorAddress, Delegate: entry.FeederAddress}
		if err := msg.ValidateBasic(); err != nil {
			return fmt.Errorf("oracle genesis feeder delegation %d: %w", i, err)
		}
		operator, _ := sdk.ValAddressFromBech32(entry.ValidatorAddress)
		if _, exists := delegations[string(operator)]; exists {
			return fmt.Errorf("duplicate oracle genesis feeder delegation %q", entry.ValidatorAddress)
		}
		delegations[string(operator)] = struct{}{}
	}
	counters := make(map[string]struct{}, len(data.PenaltyCounters))
	for i, entry := range data.PenaltyCounters {
		operator, err := sdk.ValAddressFromBech32(entry.ValidatorAddress)
		if err != nil {
			return fmt.Errorf("oracle genesis penalty counter %d: %w", i, err)
		}
		if _, exists := counters[string(operator)]; exists {
			return fmt.Errorf("duplicate oracle genesis penalty counter %q", entry.ValidatorAddress)
		}
		counters[string(operator)] = struct{}{}
		counter := entry.VotePenaltyCounter
		if counter == nil {
			return fmt.Errorf("oracle genesis penalty counter %d is nil", i)
		}
		if counter.AbstainCount > math.MaxUint64-counter.MissCount ||
			counter.SuccessCount > math.MaxUint64-counter.MissCount-counter.AbstainCount {
			return fmt.Errorf("oracle genesis penalty counter %d total overflows", i)
		}
	}
	voters := make(map[string]struct{}, len(data.AggregateExchangeRateVotes))
	for i, vote := range data.AggregateExchangeRateVotes {
		voter, err := sdk.ValAddressFromBech32(vote.Voter)
		if err != nil {
			return fmt.Errorf("oracle genesis aggregate vote %d: %w", i, err)
		}
		if _, exists := voters[string(voter)]; exists {
			return fmt.Errorf("duplicate oracle genesis aggregate voter %q", vote.Voter)
		}
		voters[string(voter)] = struct{}{}
		if err := validateGenesisTuples(vote.ExchangeRateTuples, true); err != nil {
			return fmt.Errorf("oracle genesis aggregate vote %d: %w", i, err)
		}
		rates := make([]string, len(vote.ExchangeRateTuples))
		for j, tuple := range vote.ExchangeRateTuples {
			rates[j] = tuple.ExchangeRate.String() + tuple.Denom
		}
		msg := NewMsgAggregateExchangeRateVote(strings.Join(rates, ","), sdk.AccAddress(voter), voter)
		if err := msg.ValidateBasic(); err != nil {
			return fmt.Errorf("oracle genesis aggregate vote %d: %w", i, err)
		}
	}
	for i, snapshot := range data.PriceSnapshots {
		if snapshot.SnapshotTimestamp < 0 ||
			snapshot.SnapshotTimestamp > math.MaxInt64-int64(data.Params.LookbackDuration) {
			return fmt.Errorf("oracle genesis snapshot %d has invalid timestamp", i)
		}
		if i > 0 && snapshot.SnapshotTimestamp <= data.PriceSnapshots[i-1].SnapshotTimestamp {
			return fmt.Errorf("oracle genesis snapshots must have strictly increasing timestamps")
		}
		denoms := make(map[string]struct{}, len(snapshot.PriceSnapshotItems))
		for _, item := range snapshot.PriceSnapshotItems {
			if err := validateGenesisDenom(item.Denom, denoms); err != nil {
				return fmt.Errorf("oracle genesis snapshot %d: %w", i, err)
			}
			if err := validateGenesisOracleRate(item.OracleExchangeRate); err != nil {
				return fmt.Errorf("oracle genesis snapshot %d denom %q: %w", i, item.Denom, err)
			}
		}
	}
	return nil
}

func validateGenesisDenom(denom string, seen map[string]struct{}) error {
	if err := sdk.ValidateDenom(denom); err != nil {
		return err
	}
	if _, exists := seen[denom]; exists {
		return fmt.Errorf("duplicate denom %q", denom)
	}
	seen[denom] = struct{}{}
	return nil
}

func validateGenesisTuples(tuples ExchangeRateTuples, allowAbstain bool) error {
	seen := make(map[string]struct{}, len(tuples))
	for _, tuple := range tuples {
		if err := validateGenesisDenom(tuple.Denom, seen); err != nil {
			return err
		}
		rate := tuple.ExchangeRate
		if rate.IsNil() || !rate.IsInValidRange() || rate.IsNegative() || (!allowAbstain && rate.IsZero()) {
			return fmt.Errorf("invalid oracle exchange rate for %q", tuple.Denom)
		}
	}
	return nil
}

func validateGenesisOracleRate(rate OracleExchangeRate) error {
	if rate.ExchangeRate.IsNil() || !rate.ExchangeRate.IsInValidRange() || !rate.ExchangeRate.IsPositive() {
		return fmt.Errorf("missing, nonpositive or out-of-range exchange rate")
	}
	if rate.LastUpdate.IsNil() || rate.LastUpdate.IsNegative() {
		return fmt.Errorf("missing or negative source height")
	}
	if rate.LastUpdateTimestamp < 0 {
		return fmt.Errorf("negative observation timestamp")
	}
	return nil
}

// ValidateGenesisExchangeRates rejects unknown formats and ambiguous imports.
func ValidateGenesisExchangeRates(data *GenesisState) error {
	if data == nil {
		return fmt.Errorf("oracle genesis is nil")
	}
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
		if err := validateGenesisDenom(entry.Denom, seen); err != nil {
			return fmt.Errorf("oracle genesis exchange rate: %w", err)
		}
		if err := validateGenesisOracleRate(entry.OracleExchangeRate); err != nil {
			return fmt.Errorf("oracle genesis exchange rate %q: %w", entry.Denom, err)
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
