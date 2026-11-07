package oracle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestExportInitGenesis(t *testing.T) {
	input, _ := setup(t)
	observed := time.UnixMilli(1700000000123).UTC()
	input.Ctx = input.Ctx.WithBlockHeight(7).WithBlockTime(observed)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.Whitelist = append(params.Whitelist, types.Denom{Name: "denom"}, types.Denom{Name: "denom2"})
	input.OracleKeeper.SetParams(input.Ctx, params)

	input.OracleKeeper.SetFeederDelegation(input.Ctx, testutils.ValAddrs[0], testutils.Addrs[1])
	input.OracleKeeper.SetBaseExchangeRate(input.Ctx, "denom", sdk.NewDec(123))
	input.OracleKeeper.SetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[0], types.NewAggregateExchangeRateVote(types.ExchangeRateTuples{{Denom: "foo", ExchangeRate: sdk.NewDec(123)}}, testutils.ValAddrs[0]))
	input.OracleKeeper.SetVoteTarget(input.Ctx, "denom")
	input.OracleKeeper.SetVoteTarget(input.Ctx, "denom2")
	input.OracleKeeper.SetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0], 2, 3, 0)
	input.OracleKeeper.SetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[1], 4, 5, 0)
	input.OracleKeeper.AddPriceSnapshot(input.Ctx, types.NewPriceSnapshot(
		types.PriceSnapshotItems{
			{
				Denom: "uhpx",
				OracleExchangeRate: types.OracleExchangeRate{
					ExchangeRate: sdk.NewDec(12),
					LastUpdate:   sdk.NewInt(3600),
				},
			},
			{
				Denom: "uatom",
				OracleExchangeRate: types.OracleExchangeRate{
					ExchangeRate: sdk.NewDec(10),
					LastUpdate:   sdk.NewInt(3600),
				},
			},
		},
		int64(3600),
	))
	input.OracleKeeper.AddPriceSnapshot(input.Ctx, types.NewPriceSnapshot(
		types.PriceSnapshotItems{
			{
				Denom: "uhpx",
				OracleExchangeRate: types.OracleExchangeRate{
					ExchangeRate: sdk.NewDec(15),
					LastUpdate:   sdk.NewInt(3700),
				},
			},
			{
				Denom: "uatom",
				OracleExchangeRate: types.OracleExchangeRate{
					ExchangeRate: sdk.NewDec(13),
					LastUpdate:   sdk.NewInt(3700),
				},
			},
		},
		int64(3700),
	))
	genesis := oracle.ExportGenesis(input.Ctx, input.OracleKeeper)

	newInput := testutils.CreateTestInput(t)
	newInput.Ctx = newInput.Ctx.WithBlockHeight(9000).WithBlockTime(observed.Add(48 * time.Hour))
	newInput.OracleKeeper.ClearVoteTargets(newInput.Ctx)
	oracle.InitGenesis(newInput.Ctx, newInput.OracleKeeper, genesis)
	newGenesis := oracle.ExportGenesis(newInput.Ctx, newInput.OracleKeeper)

	require.Equal(t, genesis, newGenesis)
	require.Equal(t, input.OracleKeeper.GetVoteTargets(input.Ctx), newInput.OracleKeeper.GetVoteTargets(newInput.Ctx))
	rate, height, timestamp, err := newInput.OracleKeeper.GetBaseExchangeRate(newInput.Ctx, "denom")
	require.NoError(t, err)
	require.Equal(t, sdk.NewDec(123), rate)
	require.Equal(t, sdk.NewInt(7), height)
	require.Equal(t, observed.UnixMilli(), timestamp)
	t.Run("resumed-period", func(t *testing.T) { repairM252Recovery(t, false) })
}
