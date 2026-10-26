package keeper_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestAdministrationIsRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnvWithoutCheckpoint(t, 0)
	assetID := vectorArray(t, e.exit, "asset")
	asset, found := e.k.GetAsset(e.ctx, assetID)
	require.True(t, found)
	const planHeight int64 = 20

	t.Run("fresh chain genesis", func(t *testing.T) {
		ctx, _ := e.ctx.WithBlockHeight(0).CacheContext()
		genesis := types.DefaultGenesis()
		genesis.Assets = []types.AssetMapping{asset}
		require.False(t, e.k.Retired(ctx))
		require.NotPanics(t, func() { e.k.InitGenesis(ctx, *genesis) })
		require.Equal(t, genesis.Params, e.k.GetParams(ctx))
		got, found := e.k.GetAsset(ctx, assetID)
		require.True(t, found)
		require.Equal(t, asset, got)
		require.False(t, e.k.GetEmergency(ctx))
	})

	app := testkeeper.EVMTestApp
	app.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(planHeight), types.MergePlanUpgrade)
	require.Equal(t, planHeight, app.UpgradeKeeper.GetDoneHeight(e.ctx, types.MergePlanUpgrade))

	t.Run("below plan height", func(t *testing.T) {
		ctx, _ := e.ctx.WithBlockHeight(planHeight - 1).CacheContext()
		require.False(t, e.k.Retired(ctx))
		params := e.k.GetParams(ctx)
		params.WithdrawalDelaySeconds++
		require.NoError(t, e.k.SetParams(ctx, params))
		require.Equal(t, params, e.k.GetParams(ctx))
		updated := asset
		updated.Paused = true
		require.NoError(t, e.k.SetAsset(ctx, updated))
		got, found := e.k.GetAsset(ctx, assetID)
		require.True(t, found)
		require.Equal(t, updated, got)
		require.NoError(t, e.k.SetEmergency(ctx, true))
		require.True(t, e.k.GetEmergency(ctx))
		require.NoError(t, e.k.SetEmergency(ctx, false))
		require.False(t, e.k.GetEmergency(ctx))
	})

	for _, height := range []int64{planHeight, planHeight + 1} {
		for _, emergency := range []bool{false, true} {
			t.Run(fmt.Sprintf("height %d emergency %t", height, emergency), func(t *testing.T) {
				ctx, _ := e.ctx.WithBlockHeight(planHeight - 1).CacheContext()
				require.NoError(t, e.k.SetEmergency(ctx, emergency))
				ctx = ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
				require.True(t, e.k.Retired(ctx))
				params := e.k.GetParams(ctx)
				updatedParams := params
				updatedParams.WithdrawalDelaySeconds++
				require.NoError(t, updatedParams.Validate())
				updatedAsset := asset
				updatedAsset.Paused = true
				updatedAsset.Pointer = common.HexToAddress("0x00000000000000000000000000000000000f00f1").Hex()
				require.NoError(t, updatedAsset.Validate())
				newAssetID := [32]byte{0xaa}
				newAsset := types.AssetMapping{AssetId: types.Hash32(newAssetID), Denom: "uretired", Enabled: true}
				require.NoError(t, newAsset.Validate())

				for _, operation := range []struct {
					name string
					run  func(sdk.Context) error
				}{
					{"params", func(ctx sdk.Context) error { return e.k.SetParams(ctx, updatedParams) }},
					{"invalid params", func(ctx sdk.Context) error { return e.k.SetParams(ctx, types.Params{}) }},
					{"asset replacement", func(ctx sdk.Context) error { return e.k.SetAsset(ctx, updatedAsset) }},
					{"asset creation", func(ctx sdk.Context) error { return e.k.SetAsset(ctx, newAsset) }},
					{"invalid asset", func(ctx sdk.Context) error { return e.k.SetAsset(ctx, types.AssetMapping{}) }},
					{"enable emergency", func(ctx sdk.Context) error { return e.k.SetEmergency(ctx, true) }},
					{"disable emergency", func(ctx sdk.Context) error { return e.k.SetEmergency(ctx, false) }},
				} {
					t.Run(operation.name, func(t *testing.T) {
						callCtx := ctx.WithGasMeter(sdk.NewGasMeter(1, 1, 1))
						err := operation.run(callCtx)
						require.ErrorIs(t, err, types.ErrModuleRetired)
						require.ErrorContains(t, err, fmt.Sprintf("%s from height %d", types.MergePlanUpgrade, height))
						require.Zero(t, callCtx.GasMeter().GasConsumed())
						require.Equal(t, params, e.k.GetParams(ctx))
						got, found := e.k.GetAsset(ctx, assetID)
						require.True(t, found)
						require.Equal(t, asset, got)
						got, found = e.k.GetAssetByPointer(ctx, tokenPointer)
						require.True(t, found)
						require.Equal(t, asset, got)
						_, found = e.k.GetAssetByPointer(ctx, common.HexToAddress(updatedAsset.Pointer))
						require.False(t, found)
						_, found = e.k.GetAsset(ctx, newAssetID)
						require.False(t, found)
						require.Equal(t, emergency, e.k.GetEmergency(ctx))
						require.Empty(t, ctx.EventManager().Events())
					})
				}
			})
		}
	}
}
