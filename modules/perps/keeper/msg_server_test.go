package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMsgUpdateParamsApplies(t *testing.T) {
	k, ctx, key := newKeeper(t)
	initial := types.DefaultParams()
	require.NoError(t, k.UpdateParams(ctx, govAuthority(), initial))
	updated := types.DefaultParams()
	updated.LiquidationsPerBlock = 3
	updated.SweepPositionsPerBlock = 128
	updated.Paused = true

	response, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
		Authority: govAuthority(), Params: updated,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, updated, stored)
	require.NotEmpty(t, ctx.KVStore(key).Get(types.ParamsKey))
	require.Equal(t, sdk.Events{sdk.NewEvent("perps_params_updated",
		sdk.NewAttribute("authority", govAuthority()))}, ctx.EventManager().Events())
}

func TestMsgUpdateParamsRefusesCollateralTokenChange(t *testing.T) {
	for _, state := range []string{"default", "stored", "stored custom token"} {
		t.Run(state, func(t *testing.T) {
			k, ctx, key := newKeeper(t)
			initial := types.DefaultParams()
			if state == "stored custom token" {
				initial.CollateralToken = common.HexToAddress("0x00000000000000000000000000000000000c0001")
			}
			if state != "default" {
				require.NoError(t, k.UpdateParams(ctx, govAuthority(), initial))
			}
			before := append([]byte(nil), ctx.KVStore(key).Get(types.ParamsKey)...)
			updated := types.DefaultParams()
			if state != "stored custom token" {
				updated.CollateralToken = common.HexToAddress("0x00000000000000000000000000000000000c0002")
			}
			updated.LiquidationsPerBlock = 3
			require.NoError(t, updated.Validate())

			response, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
				Authority: govAuthority(), Params: updated,
			})
			require.ErrorIs(t, err, types.ErrInvalidParams)
			require.Nil(t, response)
			require.Equal(t, before, ctx.KVStore(key).Get(types.ParamsKey))
			stored, err := k.GetParams(ctx)
			require.NoError(t, err)
			require.Equal(t, initial, stored)
			require.Empty(t, ctx.EventManager().Events())
		})
	}
}

func TestMsgUpdateParamsRefusesNonGovAuthority(t *testing.T) {
	for _, state := range []string{"default", "stored"} {
		t.Run(state, func(t *testing.T) {
			k, ctx, key := newKeeper(t)
			initial := types.DefaultParams()
			if state == "stored" {
				require.NoError(t, k.UpdateParams(ctx, govAuthority(), initial))
			}
			before := append([]byte(nil), ctx.KVStore(key).Get(types.ParamsKey)...)
			user := sdk.AccAddress(common.HexToAddress("0x00000000000000000000000000000000000b0001").Bytes()).String()
			for _, authority := range []string{user, k.ModuleAddress().String(), ""} {
				for _, changeToken := range []bool{false, true} {
					updated := types.DefaultParams()
					updated.LiquidationsPerBlock = 3
					if changeToken {
						updated.CollateralToken = common.HexToAddress("0x00000000000000000000000000000000000c0002")
					}
					response, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
						Authority: authority, Params: updated,
					})
					require.ErrorIs(t, err, types.ErrUnauthorized)
					require.Nil(t, response)
					require.Equal(t, before, ctx.KVStore(key).Get(types.ParamsKey))
					stored, err := k.GetParams(ctx)
					require.NoError(t, err)
					require.Equal(t, initial, stored)
					require.Empty(t, ctx.EventManager().Events())
				}
			}
		})
	}
}
