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
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestDisabledMarketInactiveInView(t *testing.T) {
	app := testkeeper.EVMTestApp
	base, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(11).
		WithBlockTime(time.Unix(viewsNow, 0)).CacheContext()
	key := sdk.NewKVStoreKey(perpstypes.StoreKey)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	ctx := viewsWithStore(viewsWithStore(base, key), feedKey)
	feed := pricefeedkeeper.NewKeeper(feedKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	k := perpskeeper.NewKeeper(key, &app.EvmKeeper, feed)
	require.NoError(t, perpskeeper.SeedV6120(ctx, *k))
	gs := k.ExportGenesis(ctx)
	viewsState(t, gs, perpstypes.MarketState{MarketID: 1, Fresh: true}, "3000")
	k.InitGenesis(ctx, *gs)

	precompile, err := perps.NewPrecompile(viewsKeepers{Keepers: app.GetPrecompileKeepers(), perps: k})
	require.NoError(t, err)
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	_, caller := testkeeper.MockAddressPair()
	stateDB := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &app.EvmKeeper, true)
	env := &viewsEnv{t: t, ctx: ctx, keeper: k, abi: parsed, precompile: precompile, caller: caller,
		evm: &vm.EVM{StateDB: stateDB, TxContext: vm.TxContext{Origin: caller}}}
	ctx = stateDB.Ctx()

	eth, err := k.GetMarket(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "ETH", eth.Symbol)
	require.True(t, eth.Enabled)
	before, err := k.GetMarketState(ctx, eth.ID)
	require.NoError(t, err)
	require.True(t, before.Fresh)
	env.expect("isMarketActive", []interface{}{viewsInt(1)}, true)
	activeIDs := make([]*big.Int, 18)
	for id := range activeIDs {
		activeIDs[id] = viewsInt(int64(id))
	}
	env.expect("getActiveMarketIds", nil, activeIDs)

	eth.Enabled = false
	response, err := perpskeeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &perpstypes.MsgSetMarket{
		Authority: k.Authority(), Market: eth,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := k.GetMarket(ctx, eth.ID)
	require.NoError(t, err)
	require.Equal(t, eth, stored)
	after, err := k.GetMarketState(ctx, eth.ID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.True(t, after.Fresh)

	env.expect("isMarketActive", []interface{}{viewsInt(1)}, false)
	wantIDs := append(activeIDs[:1:1], activeIDs[2:]...)
	env.expect("getActiveMarketIds", nil, wantIDs)
	env.expect("totalMarkets", nil, viewsInt(18))
}
