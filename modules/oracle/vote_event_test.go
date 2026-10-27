package oracle_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestOracleVotePeriodEmitsNoVoteEvent(t *testing.T) {
	input, handler := setup(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 3
	params.Whitelist = types.DenomList{{Name: utils.MicroAtomDenom}}
	input.OracleKeeper.SetParams(input.Ctx, params)
	input.OracleKeeper.ClearVoteTargets(input.Ctx)
	input.OracleKeeper.SetVoteTarget(input.Ctx, utils.MicroAtomDenom)
	ctx := input.Ctx.WithBlockHeight(0).WithEventManager(sdk.NewEventManager())

	for i := 0; i < 3; i++ {
		msg := types.NewMsgAggregateExchangeRateVote(
			randomExchangeRate.String()+utils.MicroAtomDenom,
			testutils.Addrs[i], testutils.ValAddrs[i],
		)
		result, err := handler(ctx, msg)
		require.NoError(t, err)
		require.NotNil(t, result)
		aggregateVoteEmitted := false
		for _, event := range result.Events {
			require.NotEqual(t, "vote", event.Type)
			if event.Type == types.EventTypeAggregateVote {
				aggregateVoteEmitted = true
			}
		}
		require.True(t, aggregateVoteEmitted, "aggregate votes must still emit their event")
	}

	for height := int64(0); height < int64(params.VotePeriod); height++ {
		ctx = ctx.WithBlockHeight(height)
		oracle.MidBlocker(ctx, input.OracleKeeper)
		oracle.EndBlocker(ctx, input.OracleKeeper)
	}

	rate, lastUpdate, _, err := input.OracleKeeper.GetBaseExchangeRate(ctx, utils.MicroAtomDenom)
	require.NoError(t, err)
	require.Equal(t, randomExchangeRate, rate)
	require.Equal(t, ctx.BlockHeight(), lastUpdate.Int64())

	exchangeRateUpdateEmitted := false
	for _, event := range ctx.EventManager().Events() {
		require.NotEqual(t, "vote", event.Type)
		if event.Type == types.EventTypeExchangeRateUpdate {
			exchangeRateUpdateEmitted = true
		}
	}
	require.True(t, exchangeRateUpdateEmitted, "the vote period must emit an exchange rate update")
}
