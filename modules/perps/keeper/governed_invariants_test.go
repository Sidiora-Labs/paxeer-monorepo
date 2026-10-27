package keeper_test

import (
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func RegisterInvariants(t *testing.T, k *keeper.Keeper) *invRegistry {
	t.Helper()
	registry := &invRegistry{routes: make(map[string]sdk.Invariant)}
	var registrar sdk.InvariantRegistry = registry
	keeper.RegisterInvariants(registrar, k)
	names := []string{
		keeper.SolvencyInvariantName, keeper.OpenInterestInvariantName, keeper.BookInvariantName,
		keeper.OrdersInvariantName, keeper.PositionsInvariantName, keeper.FundingInvariantName,
		keeper.CountersInvariantName, keeper.DeterminismInvariantName, keeper.IndexesInvariantName,
	}
	require.Len(t, registry.routes, 9)
	for _, name := range names {
		require.Contains(t, registry.routes, types.ModuleName+"/"+name)
		require.NotNil(t, registry.routes[types.ModuleName+"/"+name])
	}
	return registry
}

func TestInvariantsHoldAfterGovernanceChanges(t *testing.T) {
	env := newAccountEnv(t)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	env.ctx = pricefeedtestutil.WithPriceFeedStore(env.ctx, feedKey)
	feed := pricefeedkeeper.NewKeeper(feedKey, env.app.StakingKeeper, env.app.AccountKeeper, &env.app.EvmKeeper)
	require.NoError(t, feed.SetParams(env.ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	env.k = keeper.NewKeeper(env.key, &env.app.EvmKeeper, feed)
	prices := markEnv{k: env.k, ctx: env.ctx, pfKey: feedKey}
	prices.setAggregate(t, 0, 50_000, 50_000, markNowMs-1_000)
	_, err := env.k.UpdatePrices(env.ctx)
	require.NoError(t, err)

	const depositUnits int64 = accountUsdlSupply / 3
	env.call(env.a, "transfer(address,uint256)", common.LeftPadBytes(env.b.Bytes(), 32), accountWord(big.NewInt(depositUnits)))
	env.deposit(env.a, depositUnits)
	env.deposit(env.b, depositUnits)
	require.Equal(t, 2*depositUnits, env.balanceOf(env.module))
	triggerAccount := accountAddress("governance trigger account")
	env.call(env.a, "transfer(address,uint256)", common.LeftPadBytes(triggerAccount.Bytes(), 32), accountWord(big.NewInt(depositUnits)))
	env.deposit(triggerAccount, depositUnits)
	require.Equal(t, 3*depositUnits, env.balanceOf(env.module))

	order := keeper.OrderParams{
		MarketID: 0, IsLong: false, OrderType: uint8(types.OrderTypeLimit),
		TimeInForce: uint8(types.TimeInForceGTC), SizeBase: invNum(1, 16),
		Leverage: e18(20), LimitPrice: e18(50_000),
	}
	maker, err := env.k.PlaceOrder(env.ctx, env.b, order)
	require.NoError(t, err)
	require.Empty(t, maker.Fills)
	order.IsLong = true
	taker, err := env.k.PlaceOrder(env.ctx, env.a, order)
	require.NoError(t, err)
	require.Len(t, taker.Fills, 1)

	var positions []types.Position
	for _, address := range []common.Address{env.a, env.b} {
		idBytes := env.ctx.KVStore(env.key).Get(types.AccountMarketPositionKey(address, 0))
		require.Len(t, idBytes, 32)
		var id uint256.Int
		id.SetBytes(idBytes)
		position, err := env.k.GetPosition(env.ctx, id)
		require.NoError(t, err)
		require.Equal(t, address, position.Account)
		require.Equal(t, uint64(0), position.MarketID)
		require.Equal(t, order.SizeBase, position.SizeBase)
		positions = append(positions, position)
	}
	require.True(t, positions[0].IsLong)
	require.False(t, positions[1].IsLong)

	var restingOrders []types.Order
	for _, resting := range []struct {
		account common.Address
		isLong  bool
		price   int64
	}{{env.a, true, 49_000}, {env.b, false, 51_000}} {
		order.IsLong = resting.isLong
		order.LimitPrice = e18(resting.price)
		result, err := env.k.PlaceOrder(env.ctx, resting.account, order)
		require.NoError(t, err)
		require.Empty(t, result.Fills)
		var stored types.Order
		require.NoError(t, stored.Unmarshal(env.ctx.KVStore(env.key).Get(types.OrderKey(result.OrderID))))
		require.Equal(t, types.OrderStatusResting, stored.Status)
		require.Equal(t, resting.account, stored.Account)
		restingOrders = append(restingOrders, stored)
		account := env.account(resting.account)
		require.Equal(t, uint64(1), account.OpenOrderCount)
		require.Equal(t, uint64(1), account.PositionCount)
	}

	triggerResult, err := env.k.PlaceOrder(env.ctx, triggerAccount, keeper.OrderParams{
		MarketID: 0, IsLong: true, OrderType: uint8(types.OrderTypeStopLimit),
		TimeInForce: uint8(types.TimeInForceGTC), SizeBase: order.SizeBase,
		Leverage: order.Leverage, LimitPrice: e18(52_000), TriggerPrice: e18(52_000),
	})
	require.NoError(t, err)
	require.Empty(t, triggerResult.Fills)
	var trigger types.Order
	require.NoError(t, trigger.Unmarshal(env.ctx.KVStore(env.key).Get(types.OrderKey(triggerResult.OrderID))))
	require.Equal(t, types.OrderStatusUntriggered, trigger.Status)
	require.Equal(t, triggerAccount, trigger.Account)
	require.Equal(t, uint64(1), env.account(triggerAccount).OpenOrderCount)
	require.Zero(t, env.account(triggerAccount).PositionCount)

	registry := RegisterInvariants(t, env.k)
	var routes []string
	for route := range registry.routes {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	runInvariants := func(t *testing.T) {
		t.Helper()
		for _, route := range routes {
			t.Run(route, func(t *testing.T) {
				message, broken := registry.routes[route](env.ctx)
				require.False(t, broken, message)
				require.Equal(t, sdk.FormatInvariant(types.ModuleName,
					strings.TrimPrefix(route, types.ModuleName+"/"), ""), message)
			})
		}
	}
	checkInvariants := func(stage string) {
		t.Helper()
		t.Run(stage, func(t *testing.T) {
			require.Equal(t, trigger.Marshal(), env.ctx.KVStore(env.key).Get(types.OrderKey(trigger.ID)))
			triggerHolder, err := env.k.GetAccount(env.ctx, triggerAccount)
			require.NoError(t, err)
			require.Equal(t, uint64(1), triggerHolder.OpenOrderCount)
			require.Zero(t, triggerHolder.PositionCount)
			for _, position := range positions {
				stored, err := env.k.GetPosition(env.ctx, position.ID)
				require.NoError(t, err)
				require.Equal(t, position, stored)
			}
			for _, order := range restingOrders {
				require.Equal(t, order.Marshal(), env.ctx.KVStore(env.key).Get(types.OrderKey(order.ID)))
				account, err := env.k.GetAccount(env.ctx, order.Account)
				require.NoError(t, err)
				require.Equal(t, uint64(1), account.OpenOrderCount)
				require.Equal(t, uint64(1), account.PositionCount)
			}
			runInvariants(t)
		})
	}
	checkInvariants("before_governance")

	server := keeper.NewMsgServerImpl(env.k)
	params, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	params.MaxOpenOrdersPerAccount = 1
	params.InsuranceSharePpm = 250_000
	params.MarkEmaSeconds = 300
	params.LiquidationsPerBlock = 3
	params.FundingCapPpmPerHour = 20_000
	paramsResponse, err := server.UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: env.k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, paramsResponse)
	storedParams, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Equal(t, params, storedParams)
	checkInvariants("after_MsgUpdateParams")

	market, err := env.k.GetMarket(env.ctx, 0)
	require.NoError(t, err)
	market.TierClass = types.TierClassB
	market.ImpactNotionalUsd = sdk.NewIntWithDecimal(6_000, types.USDDecimals)
	market.OiCapUsd = sdk.NewIntWithDecimal(100_000_000, types.USDDecimals)
	market.FundingCapPpmPerHour = 10_000
	for _, enabled := range []bool{false, true} {
		market.Enabled = enabled
		response, err := server.SetMarket(sdk.WrapSDKContext(env.ctx), &types.MsgSetMarket{
			Authority: env.k.Authority(), Market: market,
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		stored, err := env.k.GetMarket(env.ctx, market.ID)
		require.NoError(t, err)
		require.Equal(t, market, stored)
		if enabled {
			checkInvariants("after_MsgSetMarket_enabled")
		} else {
			checkInvariants("after_MsgSetMarket_disabled")
		}
	}

	for _, paused := range []bool{true, false} {
		response, err := server.SetPaused(sdk.WrapSDKContext(env.ctx), &types.MsgSetPaused{
			Authority: env.k.Authority(), Paused: paused,
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		stored, err := env.k.GetParams(env.ctx)
		require.NoError(t, err)
		params.Paused = paused
		require.Equal(t, params, stored)
		if paused {
			checkInvariants("after_MsgSetPaused_true")
		} else {
			checkInvariants("after_MsgSetPaused_false")
		}
	}

	env.k.EndBlock(env.ctx)
	for _, event := range env.ctx.EventManager().Events() {
		require.NotEqual(t, "perps_endblock_error", event.Type, "%v", event.Attributes)
	}
	require.True(t, env.ctx.KVStore(env.key).Has(types.SweepCursorKey))
	require.Equal(t, trigger.Marshal(), env.ctx.KVStore(env.key).Get(types.OrderKey(trigger.ID)))
	for _, position := range positions {
		stored, err := env.k.GetPosition(env.ctx, position.ID)
		require.NoError(t, err)
		require.Equal(t, position.Account, stored.Account)
		require.Equal(t, position.SizeBase, stored.SizeBase)
	}
	for _, order := range restingOrders {
		require.Equal(t, order.Marshal(), env.ctx.KVStore(env.key).Get(types.OrderKey(order.ID)))
	}
	t.Run("after_EndBlock", runInvariants)
}
