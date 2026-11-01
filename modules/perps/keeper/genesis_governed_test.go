package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisRoundTripAfterGovernanceChanges(t *testing.T) {
	k, ctx, sourceKey, _ := newMsgMarketKeeper(t)
	require.NoError(t, keeper.SeedV6120(ctx, *k))
	seeded := k.ExportGenesis(ctx)
	require.Equal(t, types.DefaultParams(), seeded.Params)
	require.Len(t, seeded.Markets, 18)
	require.False(t, seeded.Params.Paused)

	params := seeded.Params
	params.InsuranceSharePpm = 100_000
	params.MarkEmaSeconds = 90
	params.MinOrderValueUsd = sdk.NewIntWithDecimal(25, types.USDDecimals)
	server := keeper.NewMsgServerImpl(k)
	goCtx := sdk.WrapSDKContext(ctx)
	updateResponse, err := server.UpdateParams(goCtx, &types.MsgUpdateParams{
		Authority: k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, updateResponse)
	storedParams, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, params, storedParams)

	eth, err := k.GetMarket(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "ETH", eth.Symbol)
	require.True(t, eth.Enabled)
	eth.Enabled = false
	marketResponse, err := server.SetMarket(goCtx, &types.MsgSetMarket{
		Authority: k.Authority(), Market: eth,
	})
	require.NoError(t, err)
	require.NotNil(t, marketResponse)
	wantMarkets := append([]types.Market(nil), seeded.Markets...)
	require.Equal(t, eth.ID, wantMarkets[1].ID)
	wantMarkets[1] = eth

	pauseResponse, err := server.SetPaused(goCtx, &types.MsgSetPaused{
		Authority: k.Authority(), Paused: true,
	})
	require.NoError(t, err)
	require.NotNil(t, pauseResponse)
	params.Paused = true

	exported := k.ExportGenesis(ctx)
	require.Equal(t, params, exported.Params)
	require.True(t, exported.Params.Paused)
	require.Equal(t, wantMarkets, exported.Markets)
	exportedJSON, err := json.Marshal(exported)
	require.NoError(t, err)
	var imported types.GenesisState
	require.NoError(t, json.Unmarshal(exportedJSON, &imported))

	importKey := sdk.NewKVStoreKey("perps_genesis_governed_import")
	require.NotEqual(t, sourceKey.Name(), importKey.Name())
	importCtx := sdktestutil.DefaultContext(importKey, sdk.NewTransientStoreKey("transient_perps_genesis_governed_import")).
		WithBlockHeight(ctx.BlockHeight()).WithBlockTime(ctx.BlockTime())
	require.Empty(t, storeContents(t, importCtx.KVStore(importKey)))
	restored := keeper.NewKeeper(importKey, nil, nil)
	restored.InitGenesis(importCtx, imported)

	restoredParams, err := restored.GetParams(importCtx)
	require.NoError(t, err)
	require.Equal(t, params, restoredParams)
	require.True(t, restoredParams.Paused)
	var restoredMarkets []types.Market
	require.NoError(t, restored.IterateMarkets(importCtx, func(market types.Market) bool {
		restoredMarkets = append(restoredMarkets, market)
		return false
	}))
	require.Equal(t, wantMarkets, restoredMarkets)
	restoredETH, err := restored.GetMarket(importCtx, eth.ID)
	require.NoError(t, err)
	require.Equal(t, eth, restoredETH)
	require.False(t, restoredETH.Enabled)

	reexportedJSON, err := json.Marshal(restored.ExportGenesis(importCtx))
	require.NoError(t, err)
	require.Equal(t, exportedJSON, reexportedJSON)
}
