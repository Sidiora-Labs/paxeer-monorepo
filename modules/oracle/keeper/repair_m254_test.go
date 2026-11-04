package keeper_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/stretchr/testify/require"
)

func TestRepairM_254_011_AtomicRejection(t *testing.T) {
	assertMigrate5To6AtomicRejection(t)
}

func TestRepairM_254_011_ZeroSuccessBoundary(t *testing.T) {
	assertMigrate5To6ZeroSuccessBoundary(t)
}

func TestRepairM_254_010_DefaultBackfill(t *testing.T) {
	assertMigrate5To6DefaultBackfill(t)
}

func TestRepairM_254_010_HookBoundaries(t *testing.T) {
	assertMigrate5To6HookBoundaries(t)
}

func assertMigrate5To6HookBoundaries(t *testing.T) {
	t.Helper()
	for _, period := range []uint64{2, 3, 4} {
		for _, window := range []uint64{10, 12} {
			for _, height := range []int64{2, 9, 10, 11, 12, 13, 14, 15, 23, 24, 25, 26, 27} {
				t.Run(fmt.Sprintf("period_%d_window_%d_upgrade_%d", period, window, height), func(t *testing.T) {
					input := testutils.CreateTestInput(t)
					params := input.OracleKeeper.GetParams(input.Ctx)
					params.VotePeriod, params.SlashWindow = period, window
					params.MinValidPerWindow = sdk.ZeroDec()
					input.OracleKeeper.SetParams(input.Ctx, params)
					_, err := staking.NewHandler(input.StakingKeeper)(input.Ctx,
						testutils.NewTestMsgCreateValidator(testutils.ValAddrs[0], testutils.ValPubKeys[0],
							sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction)))
					require.NoError(t, err)
					staking.EndBlocker(input.Ctx, input.StakingKeeper)
					require.True(t, input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[0]).IsBonded())

					var expected types.VotePenaltyCounter
					observation := 0
					for block := int64(1); block < height; block++ {
						input.Ctx = input.Ctx.WithBlockHeight(block).WithEventManager(sdk.NewEventManager())
						if utils.IsPeriodLastBlock(input.Ctx, period) {
							outcome := observation % 3
							if outcome != 2 {
								rates := "1" + utils.MicroAtomDenom
								if outcome == 0 {
									rates += ",1" + utils.MicroEthDenom
								}
								_, err := keeper.NewMsgServerImpl(input.OracleKeeper).AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx),
									types.NewMsgAggregateExchangeRateVote(rates, testutils.Addrs[0], testutils.ValAddrs[0]))
								require.NoError(t, err)
							}
							switch outcome {
							case 0:
								expected.SuccessCount++
							case 1:
								expected.MissCount++
							case 2:
								expected.AbstainCount++
							}
							observation++
						}
						oracle.MidBlocker(input.Ctx, input.OracleKeeper)
						oracle.EndBlocker(input.Ctx, input.OracleKeeper)
						if utils.IsPeriodLastBlock(input.Ctx, window) {
							expected = types.VotePenaltyCounter{}
						}
						require.Equal(t, expected, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]), "producer at block %d", block)
					}

					input.Ctx = input.Ctx.WithBlockHeight(height)
					require.Equal(t, migrationHookObservations(input.Ctx, period, window), expected.MissCount+expected.AbstainCount+expected.SuccessCount)
					// Encode only legacy fields, then consume those exact store bytes in the migration.
					legacy := types.VotePenaltyCounter{MissCount: expected.MissCount, AbstainCount: expected.AbstainCount}
					store := input.Ctx.KVStore(input.OracleKeeper.GetStoreKey())
					key := types.GetVotePenaltyCounterKey(testutils.ValAddrs[0])
					if store.Has(key) {
						store.Set(key, input.OracleKeeper.GetCdc().MustMarshal(&legacy))
					}
					require.NoError(t, keeper.NewMigrator(input.OracleKeeper, keeper.VotePeriodHistory{
						StartHeight: 1, EndHeight: uint64(height), VotePeriod: period, SlashWindow: window,
					}).Migrate5To6(input.Ctx))
					require.Equal(t, expected, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
					if expected.MissCount+expected.AbstainCount+expected.SuccessCount == 0 {
						require.False(t, store.Has(key), "rollover must not resurrect a counter")
					}
				})
			}
		}
	}
}
