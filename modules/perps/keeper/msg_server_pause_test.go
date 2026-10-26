package keeper_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestMsgSetPausedStopsOpeningOrders(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 1_000)
	server := keeper.NewMsgServerImpl(env.k)
	order := limitBuy(100, 59_000)
	_, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.NoError(t, err)
	require.NoError(t, env.k.CheckPause(env.ctx, "openPosition"))

	response, err := server.SetPaused(sdk.WrapSDKContext(env.ctx), &types.MsgSetPaused{
		Authority: env.k.Authority(), Paused: true,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	params, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused)
	before := storeContents(t, env.ctx.KVStore(env.perpsKey))
	result, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.ErrorIs(t, err, types.ErrPaused)
	require.Equal(t, keeper.PlaceResult{}, result)
	require.ErrorIs(t, env.k.CheckPause(env.ctx, "openPosition"), types.ErrPaused)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))
	for _, method := range []string{"closePosition", "cancelOrder", "deposit", "addCollateral"} {
		require.NoError(t, env.k.CheckPause(env.ctx, method), method)
	}
	order.ReduceOnly = true
	_, err = env.k.PlaceOrder(env.ctx, traderB, order)
	require.ErrorIs(t, err, types.ErrNoAccount)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))

	response, err = server.SetPaused(sdk.WrapSDKContext(env.ctx), &types.MsgSetPaused{
		Authority: env.k.Authority(), Paused: false,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.NoError(t, env.k.CheckPause(env.ctx, "openPosition"))
	order.ReduceOnly = false
	_, err = env.k.PlaceOrder(env.ctx, traderA, order)
	require.NoError(t, err)
}

func TestMsgSetPausedEmitsEvent(t *testing.T) {
	for _, state := range []string{"default", "stored_false", "stored_true"} {
		t.Run(state, func(t *testing.T) {
			k, ctx, key := newKeeper(t)
			store := ctx.KVStore(key)
			params := types.DefaultParams()
			if state != "default" {
				params.Paused = state == "stored_true"
				params.LiquidationsPerBlock = 3
				params.SweepPositionsPerBlock = 128
				require.NoError(t, k.UpdateParams(ctx, govAuthority(), params))
				encoded, err := json.MarshalIndent(params, "", "  ")
				require.NoError(t, err)
				store.Set(types.ParamsKey, encoded)
			}
			require.NoError(t, k.SetMarket(ctx, govAuthority(), classEMarket(2, "PAX")))
			server := keeper.NewMsgServerImpl(k)
			expectedEvents := sdk.Events{sdk.NewEvent("existing_event")}
			ctx.EventManager().EmitEvents(expectedEvents)
			for _, paused := range []bool{params.Paused, !params.Paused, !params.Paused, params.Paused} {
				before := storeContents(t, store)
				response, err := server.SetPaused(sdk.WrapSDKContext(ctx), &types.MsgSetPaused{
					Authority: govAuthority(), Paused: paused,
				})
				require.NoError(t, err)
				require.NotNil(t, response)
				if params.Paused == paused {
					require.Equal(t, before, storeContents(t, store))
				}
				params.Paused = paused
				stored, err := k.GetParams(ctx)
				require.NoError(t, err)
				require.Equal(t, params, stored)
				expectedEvents = append(expectedEvents, sdk.NewEvent("perps_pause_set",
					sdk.NewAttribute("paused", strconv.FormatBool(paused))))
				require.Equal(t, expectedEvents, ctx.EventManager().Events())
			}
		})
	}

	t.Run("refusals_leave_state_and_events_unchanged", func(t *testing.T) {
		for _, state := range []string{"default", "stored_false", "stored_true"} {
			t.Run(state, func(t *testing.T) {
				k, ctx, key := newKeeper(t)
				if state != "default" {
					require.NoError(t, k.SetPaused(ctx, govAuthority(), state == "stored_true"))
				}
				before := storeContents(t, ctx.KVStore(key))
				server := keeper.NewMsgServerImpl(k)
				for _, authority := range []string{"", "invalid-authority", k.ModuleAddress().String()} {
					for _, paused := range []bool{true, false} {
						response, err := server.SetPaused(sdk.WrapSDKContext(ctx), &types.MsgSetPaused{
							Authority: authority, Paused: paused,
						})
						require.ErrorIs(t, err, types.ErrUnauthorized)
						require.Nil(t, response)
						require.Equal(t, before, storeContents(t, ctx.KVStore(key)))
						require.Empty(t, ctx.EventManager().Events())
					}
				}
				response, err := server.SetPaused(sdk.WrapSDKContext(ctx), nil)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, response)
				require.Equal(t, before, storeContents(t, ctx.KVStore(key)))
				require.Empty(t, ctx.EventManager().Events())
			})
		}
	})

	t.Run("corrupt_params_emit_nothing", func(t *testing.T) {
		k, ctx, key := newKeeper(t)
		ctx.KVStore(key).Set(types.ParamsKey, []byte("invalid JSON"))
		before := storeContents(t, ctx.KVStore(key))
		response, err := keeper.NewMsgServerImpl(k).SetPaused(sdk.WrapSDKContext(ctx), &types.MsgSetPaused{
			Authority: govAuthority(), Paused: true,
		})
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Nil(t, response)
		require.Equal(t, before, storeContents(t, ctx.KVStore(key)))
		require.Empty(t, ctx.EventManager().Events())
	})
}
