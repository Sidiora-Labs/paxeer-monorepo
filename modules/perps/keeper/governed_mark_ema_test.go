package keeper_test

import (
	"testing"
	"time"

	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type MarkEmaSeconds struct {
	seconds   uint64
	basisStep string
	mark      string
}

func (want MarkEmaSeconds) updateMarketPrice(t *testing.T, k *keeper.Keeper, ctx sdk.Context) types.MarketState {
	t.Helper()
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, want.seconds, params.MarkEmaSeconds)
	records := k.EndBlock(ctx)
	for _, event := range ctx.EventManager().Events() {
		require.NotEqual(t, keeper.EventTypeEndBlockError, event.Type, "%v", event.Attributes)
	}
	state, err := k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	require.True(t, state.Fresh)
	require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), state.PriceTimestampMs)
	require.Equal(t, e18(110), state.IndexPrice)
	require.Equal(t, e18(110), state.PricefeedMark)
	require.Equal(t, negative(wordOf(t, want.basisStep)), state.PremiumEma)
	require.Equal(t, wordOf(t, want.mark), state.MarkPrice)
	require.Equal(t, []string{types.EventMarkPriceUpdated, "0", want.mark, e18Dec(110), e18Dec(100)},
		eventStrings(endblockFirst(t, records, types.EventMarkPriceUpdated)))
	return state
}

func TestGovernedMarkEmaSecondsSmoothsMark(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(10).
		WithBlockTime(time.UnixMilli(markNowMs).UTC()).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	feed, ctx := pricefeedtestutil.NewKeeper(testApp, ctx)
	perpsKey := sdk.NewKVStoreKey(types.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, perpsKey)
	k := keeper.NewKeeper(perpsKey, &testApp.EvmKeeper, feed)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	require.NoError(t, k.SetMarket(ctx, k.Authority(), listedMarket(0, "BTC", 5, 1, types.TierClassA, 20_000, 500_000_000)))

	pubkey := ed25519.GenPrivKeyFromSecret([]byte("governed mark consensus")).PubKey()
	operator := sdk.ValAddress(pubkey.Address())
	bond := sdk.NewCoin(testApp.StakingKeeper.BondDenom(ctx), testApp.StakingKeeper.TokensFromConsensusPower(ctx, 1))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(bond)))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
		sdk.AccAddress(operator), sdk.NewCoins(bond)))
	commission := testApp.StakingKeeper.MinCommissionRate(ctx)
	create, err := stakingtypes.NewMsgCreateValidator(operator, pubkey, bond, stakingtypes.Description{},
		stakingtypes.NewCommissionRates(commission, commission, sdk.ZeroDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(testApp.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), create)
	require.NoError(t, err)
	_, err = testApp.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)
	require.Len(t, testApp.StakingKeeper.GetBondedValidatorsByPower(ctx), 1)
	feeder, _, err := feed.RegisterFeedKey(ctx, operator,
		secp256k1.GenPrivKeyFromSecret([]byte("governed mark feed")).PubKey().Bytes())
	require.NoError(t, err)

	aggregate := func(ctx sdk.Context, sequence uint64, index int64) {
		t.Helper()
		accepted, refused, err := feed.SubmitPrices(ctx, pricefeedtypes.MsgSubmitPrices{
			Feeder: feeder.String(), Observations: []pricefeedtypes.Observation{{
				MarketId: 0, Sequence: sequence, IndexPrice: e18Dec(index), MarkPrice: e18Dec(index),
				SourceTimestampMs: uint64(ctx.BlockTime().UnixMilli()),
			}},
		})
		require.NoError(t, err)
		require.Equal(t, uint32(1), accepted)
		require.Zero(t, refused)
		feed.Aggregate(ctx)
		price, err := feed.GetPrice(ctx, 0)
		require.NoError(t, err)
		require.True(t, price.Fresh)
		require.Equal(t, uint32(1), price.Sources)
		require.Equal(t, ctx.BlockHeight(), price.Height)
		require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), price.TimestampMs)
		require.Equal(t, e18Dec(index), price.Index.String())
		require.Equal(t, e18Dec(index), price.Mark.String())
	}

	for i, price := range []int64{99, 101} {
		require.NoError(t, k.BookInsert(ctx, types.Order{
			ID: *uint256.NewInt(uint64(i + 1)), MarketID: 0, IsLong: i == 0,
			Type: types.OrderTypeLimit, TimeInForce: types.TimeInForceGTC, Status: types.OrderStatusResting,
			LimitPrice: e18(price), SizeBase: e18(1), Sequence: uint64(i + 1),
		}))
	}
	aggregate(ctx, 1, 100)
	k.EndBlock(ctx)
	for _, event := range ctx.EventManager().Events() {
		require.NotEqual(t, keeper.EventTypeEndBlockError, event.Type, "%v", event.Attributes)
	}
	initial, err := k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, e18(100), initial.MarkPrice)
	require.True(t, initial.PremiumEma.IsZero())
	require.Equal(t, uint64(markNowMs), initial.PriceTimestampMs)

	controlCtx, _ := ctx.CacheContext()
	governedCtx, _ := ctx.CacheContext()
	controlCtx = controlCtx.WithBlockHeight(11).WithBlockTime(ctx.BlockTime().Add(time.Second)).
		WithEventManager(sdk.NewEventManager())
	governedCtx = governedCtx.WithBlockHeight(11).WithBlockTime(controlCtx.BlockTime()).
		WithEventManager(sdk.NewEventManager())
	params, err := k.GetParams(governedCtx)
	require.NoError(t, err)
	require.Equal(t, uint64(150), params.MarkEmaSeconds)
	params.MarkEmaSeconds = 10
	response, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(governedCtx), &types.MsgUpdateParams{
		Authority: k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := k.GetParams(governedCtx)
	require.NoError(t, err)
	require.Equal(t, params, stored)

	aggregate(controlCtx, 2, 110)
	aggregate(governedCtx, 2, 110)
	// With a fixed mid of 100, the 10% index jump gives a -10 premium.
	// After one second its weights are 1/151 and 1/11, truncated toward zero.
	control := (MarkEmaSeconds{150, "66225165562913907", "109933774834437086093"}).updateMarketPrice(t, k, controlCtx)
	governed := (MarkEmaSeconds{10, "909090909090909090", "109090909090909090910"}).updateMarketPrice(t, k, governedCtx)
	require.True(t, governed.MarkPrice.Lt(&control.MarkPrice))
	require.True(t, governed.MarkPrice.Gt(&initial.MarkPrice))
}
