package perps_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func viewGetFundingDetails(t *testing.T, env *viewsEnv) []byte {
	t.Helper()
	result, err := env.call("getFundingDetails", viewsInt(0))
	require.NoError(t, err, "getFundingDetails reverted with %x", result)
	return result
}

func viewGetFeeTier(t *testing.T, env *viewsEnv) []byte {
	t.Helper()
	result, err := env.call("getFeeTier", viewsB)
	require.NoError(t, err, "getFeeTier reverted with %x", result)
	return result
}

func TestViewGoldenFundingAndFees(t *testing.T) {
	app := testkeeper.EVMTestApp
	base, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(11).
		WithBlockTime(time.Unix(viewsNow, 0)).WithEventManager(sdk.NewEventManager()).CacheContext()
	key := sdk.NewKVStoreKey(perpstypes.StoreKey)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	ctx := viewsWithStore(viewsWithStore(base, key), feedKey)
	feed := pricefeedkeeper.NewKeeper(feedKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	k := perpskeeper.NewKeeper(key, &app.EvmKeeper, feed)
	gs := perpstypes.DefaultGenesis()
	gs.Params.InsuranceSharePpm = 100_000
	gs.Markets = []perpstypes.Market{{
		ID: 0, Symbol: "BTC", Name: "Bitcoin", OracleSymbol: "BTC", SzDecimals: 5, PxDecimals: 1,
		MaxLeverage: 1000, TierClass: perpstypes.TierClassA,
		ImpactNotionalUsd: sdk.NewIntWithDecimal(20_000, 18), OiCapUsd: sdk.NewIntWithDecimal(500_000_000, 18),
		FundingCapPpmPerHour: 40_000, Enabled: true,
	}}
	initial := viewsState(t, gs, perpstypes.MarketState{MarketID: 0, Fresh: true}, "60000")
	initial.LastIndexHour--
	k.InitGenesis(ctx, *gs)
	market, err := k.GetMarket(ctx, 0)
	require.NoError(t, err)
	records, err := k.SettleFunding(ctx, market)
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, perpstypes.EventFundingSettled, records[1].Name)
	settled, err := k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, viewsWord(t, "0.75"), settled.FundingIndex)
	require.Equal(t, viewsWord(t, "0.0000125"), settled.HourlyRate)
	require.Equal(t, uint64(viewsNow/3600), settled.LastIndexHour)

	for i, price := range []string{"59880", "59940"} {
		require.NoError(t, k.BookInsert(ctx, perpstypes.Order{
			ID: viewsID(uint64(i + 1)), Account: viewsA, MarketID: 0, IsLong: i == 0,
			Type: perpstypes.OrderTypeLimit, TimeInForce: perpstypes.TimeInForceGTC,
			Status: perpstypes.OrderStatusResting, LimitPrice: viewsWord(t, price),
			SizeBase: viewsWord(t, "1"), Leverage: viewsWord(t, "1"), Sequence: uint64(i + 1),
		}))
	}
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	for sample := uint64(1); sample <= 2; sample++ {
		ctx = ctx.WithBlockHeight(10 + int64(sample)).
			WithBlockTime(time.Unix(viewsNow+int64(sample-1)*int64(params.PremiumIntervalSeconds), 0)).
			WithEventManager(sdk.NewEventManager())
		price, err := json.Marshal(pricefeedtypes.Aggregate{
			IndexPrice: sdk.NewIntWithDecimal(60_000, 18), MarkPrice: sdk.NewIntWithDecimal(60_000, 18),
			Basis: sdk.ZeroInt(), TimestampMs: uint64(ctx.BlockTime().UnixMilli()), Height: ctx.BlockHeight(), Sources: 1,
		})
		require.NoError(t, err)
		ctx.KVStore(feedKey).Set(pricefeedtypes.AggregateKey(0), price)
		k.EndBlock(ctx)
		for _, event := range ctx.EventManager().Events() {
			require.NotEqual(t, "perps_endblock_error", event.Type, "%v", event)
		}
		sampled, err := k.GetMarketState(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, sample, sampled.PremiumCount)
		wantSum := "-0.001"
		if sample == 2 {
			wantSum = "-0.002"
		}
		require.Equal(t, viewsWord(t, wantSum), sampled.PremiumSum)
		require.Equal(t, uint64(ctx.BlockTime().Unix()), sampled.LastSampleTime)
		require.Equal(t, settled.FundingIndex, sampled.FundingIndex)
		require.Equal(t, settled.HourlyRate, sampled.HourlyRate)
	}
	require.NoError(t, k.RecordVolume(ctx, viewsA, viewsB, viewsWord(t, "6000000")))

	precompile, err := perps.NewPrecompile(viewsKeepers{Keepers: app.GetPrecompileKeepers(), perps: k})
	require.NoError(t, err)
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	_, caller := testkeeper.MockAddressPair()
	stateDB := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &app.EvmKeeper, true)
	env := &viewsEnv{t: t, ctx: ctx, keeper: k, abi: parsed, precompile: precompile, caller: caller,
		evm: &vm.EVM{StateDB: stateDB, TxContext: vm.TxContext{Origin: caller}}}
	require.Equal(t, "0x0000000000000000000000000000000000001015", precompile.Address().Hex())

	fixture, err := os.ReadFile("testdata/views_funding_golden.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(fixture, &golden))
	require.Len(t, golden, 5)
	actual := make(map[string]string, 5)
	for _, name := range []string{"getCurrentFundingRate", "getFundingState", "getFundingDetails", "getFees", "getFeeTier"} {
		t.Run(name, func(t *testing.T) {
			var result []byte
			switch name {
			case "getFundingDetails":
				result = viewGetFundingDetails(t, env)
			case "getFeeTier":
				result = viewGetFeeTier(t, env)
			case "getFees":
				result, err = env.call(name)
				require.NoError(t, err, "%s reverted with %x", name, result)
			default:
				result, err = env.call(name, viewsInt(0))
				require.NoError(t, err, "%s reverted with %x", name, result)
			}
			encoded, ok := golden[name]
			require.True(t, ok, "missing golden for %s", name)
			want, err := hexutil.Decode(encoded)
			require.NoError(t, err, name)
			require.Equal(t, want, result, name)
			actual[name] = hexutil.Encode(result)
		})
	}
	encoded, err := json.MarshalIndent(actual, "", "  ")
	require.NoError(t, err)
	require.Equal(t, fixture, append(encoded, '\n'))
}
