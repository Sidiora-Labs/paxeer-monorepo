package perps_test

import (
	"math/big"
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
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func viewGetMarketConfig(t *testing.T, env *viewsEnv, marketID uint64) viewsMarketConfig {
	t.Helper()
	result, err := env.call("getMarketConfig", new(big.Int).SetUint64(marketID))
	require.NoError(t, err)
	outputs, err := env.abi.Methods["getMarketConfig"].Outputs.Unpack(result)
	require.NoError(t, err)
	require.Len(t, outputs, 1)
	return *abi.ConvertType(outputs[0], new(viewsMarketConfig)).(*viewsMarketConfig)
}

func TestGovernedMarketConfigView(t *testing.T) {
	app := testkeeper.EVMTestApp
	base, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(11).
		WithBlockTime(time.Unix(viewsNow, 0)).WithEventManager(sdk.NewEventManager()).CacheContext()
	key := sdk.NewKVStoreKey(perpstypes.StoreKey)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	ctx := viewsWithStore(viewsWithStore(base, key), feedKey)
	feed := pricefeedkeeper.NewKeeper(feedKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	k := perpskeeper.NewKeeper(key, &app.EvmKeeper, feed)
	require.NoError(t, perpskeeper.SeedV6120(ctx, *k))
	market, err := k.GetMarket(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "BTC", market.Symbol)
	require.Equal(t, perpstypes.TierClassA, market.TierClass)
	require.Equal(t, uint64(1000), market.MaxLeverage)
	symbol, enabled, err := feed.MarketSymbol(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, market.OracleSymbol, symbol)
	require.True(t, enabled)

	precompile, err := perps.NewPrecompile(viewsKeepers{Keepers: app.GetPrecompileKeepers(), perps: k})
	require.NoError(t, err)
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	_, caller := testkeeper.MockAddressPair()
	stateDB := state.NewDBImpl(ctx, &app.EvmKeeper, true)
	env := &viewsEnv{t: t, ctx: ctx, keeper: k, abi: parsed, precompile: precompile, caller: caller,
		evm: &vm.EVM{StateDB: stateDB, TxContext: vm.TxContext{Origin: caller}}}
	before := viewGetMarketConfig(t, env, market.ID)
	require.Equal(t, "BTC", before.Symbol)
	require.Equal(t, uint8(perpstypes.TierClassA), before.TierClass)
	require.Zero(t, big.NewInt(1000).Cmp(before.MaxLeverage))
	require.Zero(t, new(big.Int).SetUint64(market.MaxLeverage).Cmp(before.MaxLeverage))
	require.Zero(t, market.OiCapUsd.BigInt().Cmp(before.OiCapUsd))

	market.TierClass = perpstypes.TierClassB
	market.OiCapUsd = sdk.NewIntWithDecimal(250_000_000, perpstypes.USDDecimals)
	require.NotEqual(t, uint8(market.TierClass), before.TierClass)
	require.NotZero(t, market.OiCapUsd.BigInt().Cmp(before.OiCapUsd))
	response, err := perpskeeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &perpstypes.MsgSetMarket{
		Authority: k.Authority(), Market: market,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := k.GetMarket(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, perpstypes.TierClassB, stored.TierClass)
	require.Equal(t, uint64(1000), stored.MaxLeverage)
	require.Equal(t, market.MaxLeverage, stored.MaxLeverage)
	require.True(t, market.OiCapUsd.Equal(stored.OiCapUsd))

	after := viewGetMarketConfig(t, env, market.ID)
	require.Equal(t, "BTC", after.Symbol)
	require.Equal(t, uint8(perpstypes.TierClassB), after.TierClass, "governed TierClass")
	require.Zero(t, big.NewInt(1000).Cmp(after.MaxLeverage), "MaxLeverage remains pinned to tier 0")
	require.Zero(t, before.MaxLeverage.Cmp(after.MaxLeverage), "MaxLeverage unchanged by governance")
	require.Zero(t, market.OiCapUsd.BigInt().Cmp(after.OiCapUsd), "governed OiCapUsd in 18-decimal USD")
}
