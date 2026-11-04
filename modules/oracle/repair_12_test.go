package oracle_test

import (
	"testing"
	"time"

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

func repair12Input(t *testing.T) (*testutils.TestInput, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := dbm.NewDB("oracle", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	input := testutils.CreateTestInputWithDB(t, db)
	t.Cleanup(func() {
		require.NoError(t, input.Ctx.MultiStore().(storetypes.CommitMultiStore).Close())
	})
	return &input, dir
}

func repair12Restart(t *testing.T, input *testutils.TestInput, dir string) {
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

func repair12Validator(t *testing.T, input *testutils.TestInput) {
	t.Helper()
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	input.OracleKeeper.SetParams(input.Ctx, params)
	_, err := staking.NewHandler(input.StakingKeeper)(input.Ctx,
		testutils.NewTestMsgCreateValidator(testutils.ValAddrs[0], testutils.ValPubKeys[0], stakingAmt))
	require.NoError(t, err)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)
	require.True(t, input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[0]).IsBonded())
}

func repair12Observe(t *testing.T, input *testutils.TestInput, height int64, observed time.Time, rates string) {
	t.Helper()
	input.Ctx = input.Ctx.WithBlockHeight(height).WithBlockTime(observed).WithEventManager(sdk.NewEventManager())
	msg := types.NewMsgAggregateExchangeRateVote(rates, testutils.Addrs[0], testutils.ValAddrs[0])
	require.NoError(t, msg.ValidateBasic())
	_, err := oracle.NewHandler(input.OracleKeeper)(input.Ctx, msg)
	require.NoError(t, err)
	oracle.MidBlocker(input.Ctx, input.OracleKeeper)
}

func repair12Transport(t *testing.T, format string, genesis *types.GenesisState) *types.GenesisState {
	t.Helper()
	cdc := testutils.MakeTestCodec(t)
	// Protobuf has no distinction between nil and empty repeated fields. Start
	// with empty collections so structural equality still checks every value.
	recovered := types.GenesisState{
		FeederDelegations:          []types.FeederDelegation{},
		ExchangeRates:              types.ExchangeRateTuples{},
		PenaltyCounters:            []types.PenaltyCounter{},
		AggregateExchangeRateVotes: []types.AggregateExchangeRateVote{},
		PriceSnapshots:             types.PriceSnapshots{},
		OracleExchangeRates:        []types.GenesisExchangeRate{},
	}
	if format == "json" {
		payload, err := cdc.MarshalAsJSON(genesis)
		require.NoError(t, err)
		require.NoError(t, cdc.UnmarshalAsJSON(payload, &recovered))
	} else {
		payload, err := cdc.Marshal(genesis)
		require.NoError(t, err)
		require.Equal(t, len(payload), genesis.Size())
		require.NoError(t, cdc.Unmarshal(payload, &recovered))
	}
	return &recovered
}

func repair12Rate(t *testing.T, input *testutils.TestInput, denom string) types.OracleExchangeRate {
	t.Helper()
	rate, height, timestamp, err := input.OracleKeeper.GetBaseExchangeRate(input.Ctx, denom)
	require.NoError(t, err)
	want := types.OracleExchangeRate{ExchangeRate: rate, LastUpdate: height, LastUpdateTimestamp: timestamp}
	querier := keeper.NewQuerier(input.OracleKeeper)
	response, err := querier.ExchangeRate(sdk.WrapSDKContext(input.Ctx), &types.QueryExchangeRateRequest{Denom: denom})
	require.NoError(t, err)
	require.Equal(t, want, response.OracleExchangeRate)
	cdc := testutils.MakeTestCodec(t)
	var decoded types.QueryExchangeRateResponse
	require.NoError(t, cdc.Unmarshal(cdc.MustMarshal(response), &decoded))
	require.Equal(t, want, decoded.OracleExchangeRate)
	all, err := querier.ExchangeRates(sdk.WrapSDKContext(input.Ctx), &types.QueryExchangeRatesRequest{})
	require.NoError(t, err)
	require.Contains(t, all.DenomOracleExchangeRatePairs,
		types.DenomOracleExchangeRatePair{Denom: denom, OracleExchangeRate: want})
	return want
}

func TestRepair_12_ExportImportPreservesFreshness(t *testing.T) {
	for _, format := range []string{"protobuf", "json"} {
		t.Run(format, func(t *testing.T) {
			source, sourceDir := repair12Input(t)
			repair12Validator(t, source)
			observed := time.UnixMilli(1700000000123).UTC()
			repair12Observe(t, source, 101, observed, "12.5uatom,25ueth")
			repair12Observe(t, source, 103, observed.Add(2*time.Second), "123.456789012345678901uatom")
			wants := map[string]types.OracleExchangeRate{
				utils.MicroAtomDenom: {ExchangeRate: sdk.MustNewDecFromStr("123.456789012345678901"), LastUpdate: sdk.NewInt(103), LastUpdateTimestamp: observed.Add(2 * time.Second).UnixMilli()},
				utils.MicroEthDenom:  {ExchangeRate: sdk.NewDec(25), LastUpdate: sdk.NewInt(101), LastUpdateTimestamp: observed.UnixMilli()},
			}
			repair12Restart(t, source, sourceDir)
			for denom, want := range wants {
				require.Equal(t, want, repair12Rate(t, source, denom))
			}
			source.Ctx = source.Ctx.WithBlockHeight(500).WithBlockTime(observed.Add(24 * time.Hour))
			exported := oracle.ExportGenesis(source.Ctx, source.OracleKeeper)
			require.Equal(t, types.GenesisExchangeRateVersion, exported.ExchangeRateVersion)
			require.Empty(t, exported.ExchangeRates)
			require.Len(t, exported.OracleExchangeRates, 2)
			genesis := repair12Transport(t, format, exported)
			require.Equal(t, exported, genesis)
			require.NoError(t, types.ValidateGenesis(genesis))

			recovered, recoveredDir := repair12Input(t)
			recovered.Ctx = recovered.Ctx.WithBlockHeight(9000).WithBlockTime(observed.Add(48 * time.Hour))
			oracle.InitGenesis(recovered.Ctx, recovered.OracleKeeper, genesis)
			repair12Restart(t, recovered, recoveredDir)
			for denom, want := range wants {
				rate := repair12Rate(t, recovered, denom)
				require.Equal(t, want, rate)
				require.Greater(t, recovered.Ctx.BlockTime().UnixMilli()-rate.LastUpdateTimestamp,
					int64(recovered.OracleKeeper.GetParams(recovered.Ctx).LookbackDuration)*1000)
			}
			require.Equal(t, exported, oracle.ExportGenesis(recovered.Ctx, recovered.OracleKeeper))
		})
	}
}

func repair12Unavailable(t *testing.T, input *testutils.TestInput, denom string) {
	t.Helper()
	_, _, _, err := input.OracleKeeper.GetBaseExchangeRate(input.Ctx, denom)
	require.ErrorIs(t, err, types.ErrUnknownDenom)
	querier := keeper.NewQuerier(input.OracleKeeper)
	response, err := querier.ExchangeRate(sdk.WrapSDKContext(input.Ctx), &types.QueryExchangeRateRequest{Denom: denom})
	require.ErrorIs(t, err, types.ErrUnknownDenom)
	require.Nil(t, response)
	all, err := querier.ExchangeRates(sdk.WrapSDKContext(input.Ctx), &types.QueryExchangeRatesRequest{})
	require.NoError(t, err)
	require.Empty(t, all.DenomOracleExchangeRatePairs)
	actives, err := querier.Actives(sdk.WrapSDKContext(input.Ctx), &types.QueryActivesRequest{})
	require.NoError(t, err)
	require.Empty(t, actives.Actives)
}

func TestRepair_12_LegacyRatesUnavailableUntilVote(t *testing.T) {
	for _, format := range []string{"protobuf", "json"} {
		t.Run(format, func(t *testing.T) {
			source, _ := repair12Input(t)
			repair12Validator(t, source)
			observed := time.UnixMilli(1700000000123).UTC()
			repair12Observe(t, source, 101, observed, "12.5uatom")
			exported := oracle.ExportGenesis(source.Ctx, source.OracleKeeper)
			legacyRates := make([]types.ExchangeRateTuple, 0, len(exported.OracleExchangeRates))
			for _, entry := range exported.OracleExchangeRates {
				legacyRates = append(legacyRates, types.NewExchangeRateTuple(entry.Denom, entry.OracleExchangeRate.ExchangeRate))
			}
			legacy := types.NewGenesisState(exported.Params, legacyRates, exported.FeederDelegations,
				exported.PenaltyCounters, exported.AggregateExchangeRateVotes, exported.PriceSnapshots)
			genesis := repair12Transport(t, format, legacy)
			require.Zero(t, genesis.ExchangeRateVersion)
			require.Len(t, genesis.ExchangeRates, 1)
			require.NoError(t, types.ValidateGenesis(genesis))
			recovered, recoveredDir := repair12Input(t)
			recovered.Ctx = recovered.Ctx.WithBlockHeight(9000).WithBlockTime(observed.Add(48 * time.Hour))
			// Even an existing target value cannot grant freshness to a legacy import.
			recovered.OracleKeeper.SetBaseExchangeRate(recovered.Ctx, utils.MicroAtomDenom, sdk.NewDec(777))
			oracle.InitGenesis(recovered.Ctx, recovered.OracleKeeper, genesis)
			repair12Unavailable(t, recovered, utils.MicroAtomDenom)
			repair12Restart(t, recovered, recoveredDir)
			repair12Unavailable(t, recovered, utils.MicroAtomDenom)
			repair12Validator(t, recovered)
			oracle.MidBlocker(recovered.Ctx, recovered.OracleKeeper)
			repair12Unavailable(t, recovered, utils.MicroAtomDenom)
			freshTime := recovered.Ctx.BlockTime().Add(time.Second)
			repair12Observe(t, recovered, 9001, freshTime, "17uatom")
			want := types.OracleExchangeRate{ExchangeRate: sdk.NewDec(17), LastUpdate: sdk.NewInt(9001), LastUpdateTimestamp: freshTime.UnixMilli()}
			require.Equal(t, want, repair12Rate(t, recovered, utils.MicroAtomDenom))
			repair12Restart(t, recovered, recoveredDir)
			require.Equal(t, want, repair12Rate(t, recovered, utils.MicroAtomDenom))
		})
	}
}

func TestRepair_12_RejectsAmbiguousGenesisBeforeWrites(t *testing.T) {
	source, _ := repair12Input(t)
	repair12Validator(t, source)
	repair12Observe(t, source, 101, time.UnixMilli(1700000000123).UTC(), "12.5uatom,25ueth")
	exported := oracle.ExportGenesis(source.Ctx, source.OracleKeeper)
	for _, tc := range []struct {
		name   string
		mutate func(*types.GenesisState)
	}{
		{"unknown_version", func(g *types.GenesisState) { g.ExchangeRateVersion++ }},
		{"unversioned_metadata", func(g *types.GenesisState) { g.ExchangeRateVersion = 0 }},
		{"mixed_formats", func(g *types.GenesisState) {
			g.ExchangeRates = types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.OneDec())}
		}},
		{"duplicate_denom", func(g *types.GenesisState) {
			g.OracleExchangeRates = append(g.OracleExchangeRates, g.OracleExchangeRates[0])
		}},
		{"empty_denom", func(g *types.GenesisState) { g.OracleExchangeRates[1].Denom = "" }},
		{"missing_rate", func(g *types.GenesisState) { g.OracleExchangeRates[1].OracleExchangeRate.ExchangeRate = sdk.Dec{} }},
		{"missing_height", func(g *types.GenesisState) { g.OracleExchangeRates[1].OracleExchangeRate.LastUpdate = sdk.Int{} }},
		{"negative_height", func(g *types.GenesisState) { g.OracleExchangeRates[1].OracleExchangeRate.LastUpdate = sdk.NewInt(-1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := repair12Transport(t, "protobuf", exported)
			tc.mutate(genesis)
			require.Error(t, types.ValidateGenesis(genesis))
			target, _ := repair12Input(t)
			before := oracle.ExportGenesis(target.Ctx, target.OracleKeeper)
			require.Panics(t, func() { oracle.InitGenesis(target.Ctx, target.OracleKeeper, genesis) })
			require.Equal(t, before, oracle.ExportGenesis(target.Ctx, target.OracleKeeper))
		})
	}
}

func TestRepair_12_ZeroMetadataRemainsZero(t *testing.T) {
	source, _ := repair12Input(t)
	source.Ctx = source.Ctx.WithBlockHeight(0).WithBlockTime(time.UnixMilli(0).UTC())
	source.OracleKeeper.SetBaseExchangeRate(source.Ctx, utils.MicroAtomDenom, sdk.OneDec())
	want := repair12Rate(t, source, utils.MicroAtomDenom)
	require.True(t, want.LastUpdate.IsZero())
	require.Zero(t, want.LastUpdateTimestamp)
	genesis := repair12Transport(t, "json", oracle.ExportGenesis(source.Ctx, source.OracleKeeper))
	require.NoError(t, types.ValidateGenesis(genesis))
	recovered, dir := repair12Input(t)
	recovered.Ctx = recovered.Ctx.WithBlockHeight(9000).WithBlockTime(time.UnixMilli(1700000000123).UTC())
	oracle.InitGenesis(recovered.Ctx, recovered.OracleKeeper, genesis)
	repair12Restart(t, recovered, dir)
	require.Equal(t, want, repair12Rate(t, recovered, utils.MicroAtomDenom))
}
