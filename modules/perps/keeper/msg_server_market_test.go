package keeper_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func newMsgMarketKeeper(t *testing.T) (*keeper.Keeper, sdk.Context, sdk.StoreKey, pricefeedkeeper.Keeper) {
	t.Helper()
	key := sdk.NewKVStoreKey(types.StoreKey)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(feedKey, sdk.StoreTypeIAVL, nil)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 20, Time: time.Unix(listingTime, 0).UTC()}, false)
	feed := pricefeedkeeper.NewKeeper(feedKey, nil, nil, nil)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	return keeper.NewKeeper(key, nil, feed), ctx, key, feed
}

func TestMsgSetMarketListsNewMarket(t *testing.T) {
	for _, tc := range []struct {
		name        string
		enabled     bool
		feedEnabled bool
	}{
		{"enabled", true, true},
		{"disabled", false, true},
		{"listed with disabled feed", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key, feed := newMsgMarketKeeper(t)
			params := feed.GetParams(ctx)
			params.Markets = append(params.Markets, pricefeedtypes.Market{
				ID: 18, Symbol: "NEW", ChainflowSymbol: "NEW", MinPrice: sdk.OneInt(),
				MaxPrice: sdk.NewIntWithDecimal(1, 30), Enabled: tc.feedEnabled,
			})
			require.NoError(t, feed.UpdateParams(ctx, params.Authority, params))
			ctx = ctx.WithEventManager(sdk.NewEventManager())
			market := classEMarket(18, "NEW")
			market.Symbol = "NEW-PERP"
			market.OiCapUsd = sdk.ZeroInt()
			market.Enabled = tc.enabled
			_, err := k.GetMarket(ctx, market.ID)
			require.ErrorIs(t, err, types.ErrUnknownMarket)

			response, err := keeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
				Authority: govAuthority(), Market: market,
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			stored, err := k.GetMarket(ctx, market.ID)
			require.NoError(t, err)
			if tc.enabled {
				market.ListedAt = uint64(listingTime)
			}
			require.Equal(t, market, stored)
			require.NotEmpty(t, ctx.KVStore(key).Get(types.MarketKey(market.ID)))
			require.Equal(t, sdk.Events{sdk.NewEvent("perps_market_set",
				sdk.NewAttribute("market_id", "18"),
				sdk.NewAttribute("enabled", map[bool]string{true: "true", false: "false"}[tc.enabled]))},
				ctx.EventManager().Events())
		})
	}

	t.Run("authority and validation", func(t *testing.T) {
		k, ctx, key, _ := newMsgMarketKeeper(t)
		server := keeper.NewMsgServerImpl(k)
		market := classEMarket(0, "BTC")
		response, err := server.SetMarket(sdk.WrapSDKContext(ctx), nil)
		require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
		require.Nil(t, response)
		for _, authority := range []string{"", k.ModuleAddress().String()} {
			response, err = server.SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
				Authority: authority, Market: market,
			})
			require.ErrorIs(t, err, types.ErrUnauthorized)
			require.Nil(t, response)
		}
		market.ImpactNotionalUsd = sdk.ZeroInt()
		response, err = server.SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
			Authority: govAuthority(), Market: market,
		})
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Nil(t, response)
		require.Nil(t, ctx.KVStore(key).Get(types.MarketKey(market.ID)))
		require.Empty(t, ctx.EventManager().Events())
	})
}

func TestMsgSetMarketRefusesUnlistedPricefeedMarket(t *testing.T) {
	for _, tc := range []struct {
		name   string
		market types.Market
	}{
		{"unlisted id", classEMarket(30, "BTC")},
		{"wrong oracle symbol", classEMarket(1, "BTC")},
		{"case mismatch", classEMarket(0, "btc")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key, _ := newMsgMarketKeeper(t)
			require.NoError(t, tc.market.Validate())
			response, err := keeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
				Authority: govAuthority(), Market: tc.market,
			})
			require.ErrorIs(t, err, types.ErrUnknownMarket)
			require.Nil(t, response)
			require.Nil(t, ctx.KVStore(key).Get(types.MarketKey(tc.market.ID)))
			_, err = k.GetMarket(ctx, tc.market.ID)
			require.ErrorIs(t, err, types.ErrUnknownMarket)
			require.Empty(t, ctx.EventManager().Events())
		})
	}
}

func TestMsgSetMarketRefusesIdentityChange(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*types.Market)
	}{
		{"Symbol", func(m *types.Market) { m.Symbol = "BTC-PERP" }},
		{"OracleSymbol", func(m *types.Market) { m.OracleSymbol = "ETH" }},
		{"SzDecimals", func(m *types.Market) { m.SzDecimals-- }},
		{"PxDecimals", func(m *types.Market) { m.PxDecimals-- }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key, _ := newMsgMarketKeeper(t)
			require.NoError(t, keeper.SeedV6120(ctx, *k))
			stored, err := k.GetMarket(ctx, 0)
			require.NoError(t, err)
			before := append([]byte(nil), ctx.KVStore(key).Get(types.MarketKey(0))...)
			changed := stored
			tc.change(&changed)
			require.NoError(t, changed.Validate())
			response, err := keeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
				Authority: govAuthority(), Market: changed,
			})
			require.ErrorIs(t, err, types.ErrInvalidParams)
			require.Nil(t, response)
			require.Equal(t, before, ctx.KVStore(key).Get(types.MarketKey(0)))
			after, err := k.GetMarket(ctx, 0)
			require.NoError(t, err)
			require.Equal(t, stored, after)
			require.Empty(t, ctx.EventManager().Events())
		})
	}

	t.Run("mutable settings and enabled", func(t *testing.T) {
		k, ctx, _, _ := newMsgMarketKeeper(t)
		require.NoError(t, keeper.SeedV6120(ctx, *k))
		market, err := k.GetMarket(ctx, 0)
		require.NoError(t, err)
		market.TierClass = types.TierClassB
		market.ImpactNotionalUsd = sdk.NewIntWithDecimal(6000, types.USDDecimals)
		market.OiCapUsd = sdk.NewIntWithDecimal(100_000_000, types.USDDecimals)
		market.FundingCapPpmPerHour = 20_000
		market.Enabled = false
		server := keeper.NewMsgServerImpl(k)
		response, err := server.SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
			Authority: govAuthority(), Market: market,
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		stored, err := k.GetMarket(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, market, stored)
		require.Equal(t, sdk.Events{sdk.NewEvent("perps_market_set",
			sdk.NewAttribute("market_id", "0"), sdk.NewAttribute("enabled", "false"))}, ctx.EventManager().Events())

		ctx = ctx.WithBlockTime(time.Unix(listingTime+60, 0)).WithEventManager(sdk.NewEventManager())
		market.Enabled = true
		response, err = server.SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
			Authority: govAuthority(), Market: market,
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		stored, err = k.GetMarket(ctx, 0)
		require.NoError(t, err)
		market.ListedAt = uint64(listingTime + 60)
		require.Equal(t, market, stored)
		require.Equal(t, sdk.Events{sdk.NewEvent("perps_market_set",
			sdk.NewAttribute("market_id", "0"), sdk.NewAttribute("enabled", "true"))}, ctx.EventManager().Events())
	})

	t.Run("MaxLeverage", func(t *testing.T) {
		k, ctx, key, _ := newMsgMarketKeeper(t)
		require.NoError(t, keeper.SeedV6120(ctx, *k))
		market, err := k.GetMarket(ctx, 0)
		require.NoError(t, err)
		before := append([]byte(nil), ctx.KVStore(key).Get(types.MarketKey(market.ID))...)
		changed := market
		changed.MaxLeverage = 500
		response, err := keeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
			Authority: govAuthority(), Market: changed,
		})
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Contains(t, err.Error(), "differs from its tier 0 leverage")
		require.Nil(t, response)
		require.Equal(t, before, ctx.KVStore(key).Get(types.MarketKey(market.ID)))
		stored, err := k.GetMarket(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, market, stored)
		require.Empty(t, ctx.EventManager().Events())
	})
}
