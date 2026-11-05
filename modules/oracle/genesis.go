package oracle

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
)

// InitGenesis initialize default parameters
// and the keeper's address to pubkey map
func InitGenesis(ctx sdk.Context, keeper keeper.Keeper, data *types.GenesisState) {
	if err := types.ValidateGenesisExchangeRates(data); err != nil {
		panic(err)
	}
	if err := data.Params.Validate(); err != nil {
		panic(err)
	}
	keeper.SetParams(ctx, data.Params)
	// Genesis carries the whitelist rather than target records. Reconstruct it
	// before restored ballots can be accepted or tallied in the first period.
	targets := make(map[string]types.Denom)
	keeper.IterateVoteTargets(ctx, func(denom string, info types.Denom) bool {
		targets[denom] = info
		return false
	})
	keeper.ApplyWhitelist(ctx, data.Params.Whitelist, targets)
	for _, d := range data.FeederDelegations {
		voter, err := sdk.ValAddressFromBech32(d.ValidatorAddress)
		if err != nil {
			panic(err)
		}

		feeder, err := sdk.AccAddressFromBech32(d.FeederAddress)
		if err != nil {
			panic(err)
		}

		keeper.SetFeederDelegation(ctx, voter, feeder)
	}

	for _, ex := range data.ExchangeRates {
		// A legacy rate has no provable observation time. It must stay unavailable
		// until a new valid observation, even if the target store already has it.
		keeper.DeleteBaseExchangeRate(ctx, ex.Denom)
	}
	for _, ex := range data.OracleExchangeRates {
		keeper.RestoreBaseExchangeRate(ctx, ex.Denom, ex.OracleExchangeRate)
	}

	for _, pc := range data.PenaltyCounters {
		operator, err := sdk.ValAddressFromBech32(pc.ValidatorAddress)
		if err != nil {
			panic(err)
		}

		keeper.SetVotePenaltyCounter(ctx, operator, pc.VotePenaltyCounter.MissCount, pc.VotePenaltyCounter.AbstainCount, pc.VotePenaltyCounter.SuccessCount)
	}

	for _, av := range data.AggregateExchangeRateVotes {
		valAddr, err := sdk.ValAddressFromBech32(av.Voter)
		if err != nil {
			panic(err)
		}

		keeper.SetAggregateExchangeRateVote(ctx, valAddr, av)
	}

	for _, priceSnapshot := range data.PriceSnapshots {
		keeper.AddPriceSnapshot(ctx, priceSnapshot)
	}

	// check if the module account exists
	moduleAcc := keeper.GetOracleAccount(ctx)
	if moduleAcc == nil {
		panic(fmt.Sprintf("%s module account has not been set", types.ModuleName))
	}
}

// ExportGenesis writes the current store values
// to a genesis file, which can be imported again
// with InitGenesis
func ExportGenesis(ctx sdk.Context, keeper keeper.Keeper) *types.GenesisState {
	params := keeper.GetParams(ctx)
	feederDelegations := []types.FeederDelegation{}
	keeper.IterateFeederDelegations(ctx, func(valAddr sdk.ValAddress, feederAddr sdk.AccAddress) (stop bool) {
		feederDelegations = append(feederDelegations, types.FeederDelegation{
			FeederAddress:    feederAddr.String(),
			ValidatorAddress: valAddr.String(),
		})
		return false
	})

	exchangeRates := []types.GenesisExchangeRate{}
	keeper.IterateBaseExchangeRates(ctx, func(denom string, rate types.OracleExchangeRate) (stop bool) {
		exchangeRates = append(exchangeRates, types.GenesisExchangeRate{Denom: denom, OracleExchangeRate: rate})
		return false
	})

	penaltyCounters := []types.PenaltyCounter{}
	keeper.IterateVotePenaltyCounters(ctx, func(operator sdk.ValAddress, votePenaltyCounter types.VotePenaltyCounter) (stop bool) {
		penaltyCounters = append(penaltyCounters, types.PenaltyCounter{
			ValidatorAddress:   operator.String(),
			VotePenaltyCounter: &votePenaltyCounter,
		})
		return false
	})

	aggregateExchangeRateVotes := []types.AggregateExchangeRateVote{}
	keeper.IterateAggregateExchangeRateVotes(ctx, func(_ sdk.ValAddress, aggregateVote types.AggregateExchangeRateVote) bool {
		aggregateExchangeRateVotes = append(aggregateExchangeRateVotes, aggregateVote)
		return false
	})

	priceSnapshots := types.PriceSnapshots{}
	keeper.IteratePriceSnapshots(ctx, func(snapshot types.PriceSnapshot) bool {
		priceSnapshots = append(priceSnapshots, snapshot)
		return false
	})

	genesis := types.NewGenesisState(
		params,
		[]types.ExchangeRateTuple{},
		feederDelegations,
		penaltyCounters,
		aggregateExchangeRateVotes,
		priceSnapshots,
	)
	genesis.ExchangeRateVersion = types.GenesisExchangeRateVersion
	genesis.OracleExchangeRates = exchangeRates
	return genesis
}
