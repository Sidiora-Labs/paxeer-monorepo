package perps_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func NewLayerXProposal(t *testing.T, msgs ...sdk.Msg) *layerxgovtypes.LayerXProposal {
	t.Helper()
	proposal, err := layerxgovtypes.NewLayerXProposal("Perps governance", "Update the perpetuals venue through governance.", msgs...)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())

	registry := cdctypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	encoded, err := cdc.Marshal(proposal)
	require.NoError(t, err)
	var decoded layerxgovtypes.LayerXProposal
	require.NoError(t, cdc.Unmarshal(encoded, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	unpacked, err := decoded.GetMessages()
	require.NoError(t, err)
	require.Equal(t, msgs, unpacked)
	return &decoded
}

func NewProposalHandler(t *testing.T) (govtypes.Handler, sdk.Context, *keeper.Keeper, pricefeedkeeper.Keeper) {
	t.Helper()
	key := sdk.NewKVStoreKey(types.StoreKey)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(feedKey, sdk.StoreTypeIAVL, nil)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 20, Time: time.Unix(1_800_000_000, 0).UTC()}, false)
	feed := pricefeedkeeper.NewKeeper(feedKey, nil, nil, nil)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	k := keeper.NewKeeper(key, nil, feed)
	require.Equal(t, layerxgovtypes.GovernanceAuthority(), k.Authority())
	require.NoError(t, k.UpdateParams(ctx, k.Authority(), types.DefaultParams()))

	am := perps.NewAppModule(k)
	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	for _, msg := range []sdk.Msg{&types.MsgUpdateParams{}, &types.MsgSetMarket{}, &types.MsgSetPaused{}} {
		require.NotNil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(msg)))
	}
	return layerxgov.NewProposalHandler(router), ctx, k, feed
}

func TestLayerXProposalRoutesPerpsMsgUpdateParams(t *testing.T) {
	handler, ctx, k, _ := NewProposalHandler(t)
	before, err := k.GetParams(ctx)
	require.NoError(t, err)
	params := before
	params.FundingCapPpmPerHour = 20_000
	require.NotEqual(t, before.FundingCapPpmPerHour, params.FundingCapPpmPerHour)
	proposal := NewLayerXProposal(t, &types.MsgUpdateParams{
		Authority: layerxgovtypes.GovernanceAuthority(), Params: params,
	})
	require.NoError(t, proposal.ValidateBasic())
	require.NoError(t, handler(ctx, proposal))
	after, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(20_000), after.FundingCapPpmPerHour)
	require.Equal(t, params, after)
}

func TestLayerXProposalRunsPerpsMsgsAtomically(t *testing.T) {
	handler, ctx, k, feed := NewProposalHandler(t)
	before, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.False(t, before.Paused)
	market := types.Market{
		ID: 30, Symbol: "NEW-PERP", Name: "New market", OracleSymbol: "NEW",
		SzDecimals: 3, PxDecimals: 2, MaxLeverage: types.VenueMaxLeverage,
		TierClass: types.TierClassE, ImpactNotionalUsd: sdk.NewIntWithDecimal(1_000, types.USDDecimals),
		OiCapUsd: sdk.ZeroInt(), FundingCapPpmPerHour: 40_000, Enabled: true,
	}
	require.NoError(t, market.Validate())
	_, _, err = feed.MarketSymbol(ctx, market.ID)
	require.ErrorIs(t, err, pricefeedtypes.ErrUnknownMarket)
	pause := &types.MsgSetPaused{Authority: layerxgovtypes.GovernanceAuthority(), Paused: true}
	probe, _ := ctx.CacheContext()
	require.NoError(t, handler(probe, NewLayerXProposal(t, pause)))
	paused, err := k.GetParams(probe)
	require.NoError(t, err)
	require.True(t, paused.Paused)

	proposal := NewLayerXProposal(t, pause, &types.MsgSetMarket{
		Authority: layerxgovtypes.GovernanceAuthority(), Market: market,
	})
	require.NoError(t, proposal.ValidateBasic())
	err = handler(ctx, proposal)
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	require.Contains(t, err.Error(), "pricefeed market 30")
	after, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.False(t, after.Paused)
	require.Equal(t, before, after)
	_, err = k.GetMarket(ctx, market.ID)
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	require.Empty(t, ctx.EventManager().Events())
}
