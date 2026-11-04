package keeper_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair11Input(t *testing.T, period, window uint64) (testutils.TestInput, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := dbm.NewDB("oracle", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	input := testutils.CreateTestInputWithDB(t, db)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod, params.SlashWindow = period, window
	params.MinValidPerWindow = sdk.ZeroDec()
	input.OracleKeeper.SetParams(input.Ctx, params)
	_, err = staking.NewHandler(input.StakingKeeper)(input.Ctx,
		testutils.NewTestMsgCreateValidator(testutils.ValAddrs[0], testutils.ValPubKeys[0],
			sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction)))
	require.NoError(t, err)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)
	require.True(t, input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[0]).IsBonded())
	return input, dir
}

func repair11Restart(t *testing.T, input *testutils.TestInput, dir string) {
	t.Helper()
	previous := input.Ctx.MultiStore().(storetypes.CommitMultiStore)
	keys := previous.StoreKeys()
	storeTypes := make(map[sdk.StoreKey]sdk.StoreType, len(keys))
	keysByName := make(map[string]sdk.StoreKey, len(keys))
	for _, key := range keys {
		storeTypes[key] = previous.GetStore(key).GetStoreType()
		keysByName[key.Name()] = key
	}
	committed := previous.Commit(true)
	require.NoError(t, previous.Close())
	db, err := dbm.NewDB("oracle", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	reopened := store.NewCommitMultiStore(db)
	for _, key := range keys {
		reopened.MountStoreWithDB(key, storeTypes[key], nil)
	}
	require.NoError(t, reopened.LoadLatestVersion())
	require.Equal(t, committed, reopened.LastCommitID())
	input.Ctx = input.Ctx.WithMultiStore(reopened).WithEventManager(sdk.NewEventManager())
	cdc := input.OracleKeeper.GetCdc()
	paramKeeper := paramskeeper.NewKeeper(cdc, input.Cdc, keysByName[paramstypes.StoreKey], keysByName[paramstypes.TStoreKey])
	input.OracleKeeper = keeper.NewKeeper(cdc, keysByName[types.StoreKey], keysByName[types.MemStoreKey],
		paramKeeper.Subspace(types.ModuleName), input.AccountKeeper, input.BankKeeper,
		input.DistrKeeper, input.StakingKeeper, distrtypes.ModuleName)
}

func repair11Close(t *testing.T, input *testutils.TestInput) {
	t.Helper()
	require.NoError(t, input.Ctx.MultiStore().(storetypes.CommitMultiStore).Close())
}

func repair11Block(t *testing.T, input *testutils.TestInput, height int64, outcome string) {
	t.Helper()
	input.Ctx = input.Ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
	if outcome != "abstain" && utils.IsPeriodLastBlock(input.Ctx, input.OracleKeeper.VotePeriod(input.Ctx)) {
		rates := "1" + utils.MicroAtomDenom
		if outcome == "success" {
			rates += ",1" + utils.MicroEthDenom
		}
		_, err := keeper.NewMsgServerImpl(input.OracleKeeper).AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx),
			types.NewMsgAggregateExchangeRateVote(rates, testutils.Addrs[0], testutils.ValAddrs[0]))
		require.NoError(t, err)
	}
	oracle.MidBlocker(input.Ctx, input.OracleKeeper)
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)
}

func repair11LegacyCounter(input *testutils.TestInput) types.VotePenaltyCounter {
	operator := testutils.ValAddrs[0]
	counter := input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, operator)
	if input.Ctx.KVStore(input.OracleKeeper.GetStoreKey()).Has(types.GetVotePenaltyCounterKey(operator)) {
		input.OracleKeeper.SetVotePenaltyCounter(input.Ctx, operator, counter.MissCount, counter.AbstainCount, 0)
	}
	return counter
}

func TestRepair_11_RestartBoundaries(t *testing.T) {
	for _, period := range []uint64{2, 3, 4} {
		for _, height := range []uint64{1, period - 1, period, period + 1, 10, 11, 12, 13, 14, 15, 16, 23, 24, 25, 26} {
			t.Run(fmt.Sprintf("period_%d_height_%d", period, height), func(t *testing.T) {
				input, dir := repair11Input(t, period, 12)
				defer repair11Close(t, &input)
				outcomes := []string{"success", "success", "miss", "abstain"}
				observation := 0
				for block := uint64(1); block < height; block++ {
					repair11Block(t, &input, int64(block), outcomes[observation%len(outcomes)])
					if (block+1)%period == 0 {
						observation++
					}
				}
				expected := repair11LegacyCounter(&input)
				input.Ctx = input.Ctx.WithBlockHeight(int64(height))
				repair11Restart(t, &input, dir)
				var history []keeper.VotePeriodHistory
				if height > 1 {
					history = append(history, keeper.VotePeriodHistory{
						StartHeight: 1, EndHeight: height, VotePeriod: period, SlashWindow: 12,
					})
				}
				require.NoError(t, keeper.NewMigrator(input.OracleKeeper, history...).Migrate5To6(input.Ctx))
				repair11Restart(t, &input, dir)
				require.Equal(t, expected, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
				// Resume the real producer at the upgrade block; a boundary tally
				// must be counted exactly once and the end-window reset still runs.
				repair11Block(t, &input, int64(height), "success")
				if (height+1)%12 == 0 {
					expected = types.VotePenaltyCounter{}
				} else if (height+1)%period == 0 {
					expected.SuccessCount++
				}
				require.Equal(t, expected, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
			})
		}
	}
}

func TestRepair_11_HistoricalPeriodAndWindowRecovery(t *testing.T) {
	cases := []struct {
		name         string
		history      []keeper.VotePeriodHistory
		observations uint64
	}{
		{"period change", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 6, VotePeriod: 3, SlashWindow: 12},
			{StartHeight: 6, EndHeight: 10, VotePeriod: 2, SlashWindow: 12},
		}, 4},
		{"period changed back", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 5, VotePeriod: 2, SlashWindow: 24},
			{StartHeight: 5, EndHeight: 10, VotePeriod: 4, SlashWindow: 24},
			{StartHeight: 10, EndHeight: 18, VotePeriod: 2, SlashWindow: 24},
		}, 7},
		{"new window boundary was not an old reset", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 12, VotePeriod: 4, SlashWindow: 16},
			{StartHeight: 12, EndHeight: 16, VotePeriod: 3, SlashWindow: 12},
		}, 4},
		{"new window resets inside the interval", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 6, VotePeriod: 3, SlashWindow: 12},
			{StartHeight: 6, EndHeight: 10, VotePeriod: 2, SlashWindow: 8},
		}, 1},
		{"nonaligned period and window boundaries", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 15, VotePeriod: 3, SlashWindow: 10},
		}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, dir := repair11Input(t, tc.history[0].VotePeriod, tc.history[0].SlashWindow)
			defer repair11Close(t, &input)
			for _, epoch := range tc.history {
				params := input.OracleKeeper.GetParams(input.Ctx)
				params.VotePeriod, params.SlashWindow = epoch.VotePeriod, epoch.SlashWindow
				input.OracleKeeper.SetParams(input.Ctx, params)
				for height := epoch.StartHeight; height < epoch.EndHeight; height++ {
					repair11Block(t, &input, int64(height), "success")
				}
			}
			expected := repair11LegacyCounter(&input)
			require.Equal(t, types.VotePenaltyCounter{SuccessCount: tc.observations}, expected)
			input.Ctx = input.Ctx.WithBlockHeight(int64(tc.history[len(tc.history)-1].EndHeight))
			repair11Restart(t, &input, dir)
			require.NoError(t, keeper.NewMigrator(input.OracleKeeper, tc.history...).Migrate5To6(input.Ctx))
			repair11Restart(t, &input, dir)
			require.Equal(t, expected, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
		})
	}
}

func TestRepair_11_DefaultWindowRecoveryAndSlashing(t *testing.T) {
	input, dir := repair11Input(t, types.DefaultVotePeriod, types.DefaultSlashWindow)
	defer repair11Close(t, &input)
	window := uint64(types.DefaultSlashWindow)
	// Start at an actual reset, so the preceding window cannot contribute.
	input.Ctx = input.Ctx.WithBlockHeight(int64(window - 1))
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)
	observation := 0
	for height := window; height < window+10000; height++ {
		outcome := "success"
		if observation < 12 {
			outcome = "miss"
		} else if observation < 25 {
			outcome = "abstain"
		}
		repair11Block(t, &input, int64(height), outcome)
		if utils.IsPeriodLastBlock(input.Ctx, types.DefaultVotePeriod) {
			observation++
		}
	}
	expected := repair11LegacyCounter(&input)
	require.Equal(t, types.VotePenaltyCounter{MissCount: 12, AbstainCount: 13, SuccessCount: 4975}, expected)
	input.Ctx = input.Ctx.WithBlockHeight(int64(window + 10000))
	repair11Restart(t, &input, dir)
	require.NoError(t, keeper.NewMigrator(input.OracleKeeper, keeper.VotePeriodHistory{
		StartHeight: 1, EndHeight: window + 10000,
		VotePeriod: types.DefaultVotePeriod, SlashWindow: window,
	}).Migrate5To6(input.Ctx))
	repair11Restart(t, &input, dir)
	require.Equal(t, expected, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
	rate, updated, _, err := input.OracleKeeper.GetBaseExchangeRate(input.Ctx, utils.MicroEthDenom)
	require.NoError(t, err)
	require.Equal(t, sdk.OneDec(), rate)
	require.Equal(t, int64(window+9999), updated.Int64())
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.MinValidPerWindow = sdk.NewDecWithPrec(996, 3)
	params.SlashFraction = sdk.NewDecWithPrec(1, 2)
	input.OracleKeeper.SetParams(input.Ctx, params)
	validator := input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[0])
	amount := validator.GetBondedTokens()
	require.False(t, validator.IsJailed())
	input.OracleKeeper.SlashAndResetCounters(input.Ctx)
	validator = input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[0])
	require.True(t, validator.IsJailed())
	require.Equal(t, amount.Sub(params.SlashFraction.MulInt(amount).TruncateInt()), validator.GetBondedTokens())
	require.False(t, input.Ctx.KVStore(input.OracleKeeper.GetStoreKey()).Has(types.GetVotePenaltyCounterKey(testutils.ValAddrs[0])))
	repair11Restart(t, &input, dir)
	require.True(t, input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[0]).IsJailed())
	require.Equal(t, types.VotePenaltyCounter{}, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
}

func TestRepair_11_RejectsUnreconstructableHistoryAfterRestart(t *testing.T) {
	cases := []struct {
		name    string
		history []keeper.VotePeriodHistory
	}{
		{"missing", nil},
		{"unknown genesis", []keeper.VotePeriodHistory{{StartHeight: 2, EndHeight: 10, VotePeriod: 3, SlashWindow: 12}}},
		{"incomplete", []keeper.VotePeriodHistory{{StartHeight: 1, EndHeight: 9, VotePeriod: 3, SlashWindow: 12}}},
		{"future blocks", []keeper.VotePeriodHistory{{StartHeight: 1, EndHeight: 11, VotePeriod: 3, SlashWindow: 12}}},
		{"zero period", []keeper.VotePeriodHistory{{StartHeight: 1, EndHeight: 10, SlashWindow: 12}}},
		{"zero window", []keeper.VotePeriodHistory{{StartHeight: 1, EndHeight: 10, VotePeriod: 3}}},
		{"gap", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 5, VotePeriod: 3, SlashWindow: 12},
			{StartHeight: 6, EndHeight: 10, VotePeriod: 3, SlashWindow: 12},
		}},
		{"overlap", []keeper.VotePeriodHistory{
			{StartHeight: 1, EndHeight: 6, VotePeriod: 3, SlashWindow: 12},
			{StartHeight: 5, EndHeight: 10, VotePeriod: 3, SlashWindow: 12},
		}},
		{"empty interval", []keeper.VotePeriodHistory{{StartHeight: 1, EndHeight: 1, VotePeriod: 3, SlashWindow: 12}}},
		{"reversed interval", []keeper.VotePeriodHistory{{StartHeight: 1, EndHeight: 0, VotePeriod: 3, SlashWindow: 12}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, dir := repair11Input(t, 3, 12)
			defer repair11Close(t, &input)
			for height := int64(1); height < 10; height++ {
				repair11Block(t, &input, height, "success")
			}
			repair11LegacyCounter(&input)
			input.Ctx = input.Ctx.WithBlockHeight(10)
			repair11Restart(t, &input, dir)
			key := types.GetVotePenaltyCounterKey(testutils.ValAddrs[0])
			before := input.Ctx.KVStore(input.OracleKeeper.GetStoreKey()).Get(key)
			require.ErrorContains(t, keeper.NewMigrator(input.OracleKeeper, tc.history...).Migrate5To6(input.Ctx), "unreconstructable")
			require.Equal(t, before, input.Ctx.KVStore(input.OracleKeeper.GetStoreKey()).Get(key))
			repair11Restart(t, &input, dir)
			require.Equal(t, before, input.Ctx.KVStore(input.OracleKeeper.GetStoreKey()).Get(key))
		})
	}
}

func TestRepair_11_RejectsInvalidCountersAtomically(t *testing.T) {
	for _, counter := range []types.VotePenaltyCounter{
		{MissCount: 4}, {MissCount: 2, AbstainCount: 2},
		{MissCount: math.MaxUint64, AbstainCount: 1}, {AbstainCount: math.MaxUint64},
	} {
		t.Run(counter.String(), func(t *testing.T) {
			input := testutils.CreateTestInput(t)
			ctx := input.Ctx.WithBlockHeight(10)
			first := sdk.ValAddress(make([]byte, 20))
			second := sdk.ValAddress(append(make([]byte, 19), 1))
			input.OracleKeeper.SetVotePenaltyCounter(ctx, first, 1, 1, 0)
			input.OracleKeeper.SetVotePenaltyCounter(ctx, second, counter.MissCount, counter.AbstainCount, counter.SuccessCount)
			store := ctx.KVStore(input.OracleKeeper.GetStoreKey())
			firstBefore := store.Get(types.GetVotePenaltyCounterKey(first))
			secondBefore := store.Get(types.GetVotePenaltyCounterKey(second))
			require.ErrorContains(t, keeper.NewMigrator(input.OracleKeeper, keeper.VotePeriodHistory{
				StartHeight: 1, EndHeight: 10, VotePeriod: 3, SlashWindow: 12,
			}).Migrate5To6(ctx), "penalties exceed completed vote periods")
			require.Equal(t, firstBefore, store.Get(types.GetVotePenaltyCounterKey(first)))
			require.Equal(t, secondBefore, store.Get(types.GetVotePenaltyCounterKey(second)))
		})
	}
}
