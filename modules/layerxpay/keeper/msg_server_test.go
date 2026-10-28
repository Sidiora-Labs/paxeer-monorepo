package keeper

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestPayMsgUpdateParamsAuthority(t *testing.T) {
	h := newPayHarness(t)
	server := NewMsgServerImpl(h.keeper)
	ctx := sdk.WrapSDKContext(h.ctx)
	allEnabled := types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
	for _, authority := range []string{"", "other-governance"} {
		t.Run(fmt.Sprintf("unauthorized_%q", authority), func(t *testing.T) {
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: authority, Params: allEnabled})
			require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
			require.Nil(t, response)
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		})
	}
	for _, params := range []types.Params{{Paused: false}, {Paused: true}} {
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.keeper.Authority(), Params: params})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.True(t, h.ctx.KVStore(h.storeKey).Has(types.ParamsKey))
		require.Equal(t, params, h.keeper.GetParams(h.ctx))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	}
	for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
		key, err := types.KernelModuleEnableKey(moduleID)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
	}
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.keeper.Authority(), Params: allEnabled})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, allEnabled, h.keeper.GetParams(h.ctx))
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	response, err = server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: "other-governance", Params: types.DefaultParams()})
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, response)
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
}

func TestPayMsgUpdateParamsKernelGate(t *testing.T) {
	for _, family := range []struct {
		name     string
		moduleID uint16
		params   types.Params
	}{
		{"escrow", types.EscrowModuleID, types.Params{EscrowEnabled: true}},
		{"budget", types.BudgetModuleID, types.Params{BudgetEnabled: true}},
		{"stream", types.StreamModuleID, types.Params{StreamEnabled: true}},
		{"service", types.ServiceModuleID, types.Params{ServiceEnabled: true}},
	} {
		t.Run(family.name, func(t *testing.T) {
			h := newPayHarness(t)
			server := NewMsgServerImpl(h.keeper)
			ctx := sdk.WrapSDKContext(h.ctx)
			key, err := types.KernelModuleEnableKey(family.moduleID)
			require.NoError(t, err)
			for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
				otherKey, err := types.KernelModuleEnableKey(moduleID)
				require.NoError(t, err)
				h.ctx.KVStore(h.hostKey).Set(otherKey, payGateEnabledRecord())
			}
			h.ctx.KVStore(h.storeKey).Set(key, payGateEnabledRecord())
			nonCanonical := payGateEnabledRecord()
			nonCanonical[0] = 1
			for _, params := range []types.Params{
				family.params,
				{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true},
			} {
				for _, paused := range []bool{false, true} {
					params.Paused = paused
					for _, record := range []struct {
						name  string
						value []byte
					}{
						{"absent", nil},
						{"disabled", make([]byte, 32)},
						{"short", []byte{1}},
						{"long", append(payGateEnabledRecord(), 0)},
						{"invalid_boolean", append(make([]byte, 31), 2)},
						{"noncanonical", nonCanonical},
						{"enabled", payGateEnabledRecord()},
					} {
						t.Run(fmt.Sprintf("%v/paused_%t/%s", params, paused, record.name), func(t *testing.T) {
							if record.value == nil {
								h.ctx.KVStore(h.hostKey).Delete(key)
							} else {
								h.ctx.KVStore(h.hostKey).Set(key, record.value)
							}
							moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
							hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
							response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.keeper.Authority(), Params: params})
							if record.name == "enabled" {
								require.NoError(t, err)
								require.NotNil(t, response)
								require.Equal(t, params, h.keeper.GetParams(h.ctx))
							} else {
								require.ErrorIs(t, err, types.ErrKernelGateMismatch)
								require.Contains(t, err.Error(), family.name)
								require.Nil(t, response)
								require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
							}
							require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
							if record.name == "enabled" {
								h.ctx.KVStore(h.hostKey).Delete(key)
								moduleBefore = payStoreSnapshot(h.ctx, h.storeKey)
								response, err = server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.keeper.Authority(), Params: params})
								require.ErrorIs(t, err, types.ErrKernelGateMismatch)
								require.Nil(t, response)
								require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
							}
						})
					}
				}
			}
			for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
				gateKey, err := types.KernelModuleEnableKey(moduleID)
				require.NoError(t, err)
				h.ctx.KVStore(h.hostKey).Delete(gateKey)
			}
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.keeper.Authority(), Params: types.DefaultParams()})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, types.DefaultParams(), h.keeper.GetParams(h.ctx))
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		})
	}
}
