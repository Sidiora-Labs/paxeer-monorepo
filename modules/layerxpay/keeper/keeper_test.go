package keeper

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestPayKeeperParamsRoundTrip(t *testing.T) {
	h := newPayHarness(t)
	require.Equal(t, []byte{0x05}, types.ParamsKey)
	require.Empty(t, payStoreSnapshot(h.ctx, h.storeKey))
	require.Equal(t, types.DefaultParams(), h.keeper.GetParams(h.ctx))
	require.True(t, h.keeper.GetParams(h.ctx).Paused)
	require.Empty(t, payStoreSnapshot(h.ctx, h.storeKey))
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	require.NotEmpty(t, hostBefore)
	for mask := 0; mask < 32; mask++ {
		params := types.Params{
			Paused: mask&1 != 0, EscrowEnabled: mask&2 != 0, BudgetEnabled: mask&4 != 0,
			StreamEnabled: mask&8 != 0, ServiceEnabled: mask&16 != 0,
		}
		h.keeper.SetParams(h.ctx, params)
		raw := h.ctx.KVStore(h.storeKey).Get([]byte{0x05})
		require.True(t, json.Valid(raw))
		var fields map[string]bool
		require.NoError(t, json.Unmarshal(raw, &fields))
		require.Equal(t, map[string]bool{
			"paused": params.Paused, "escrow_enabled": params.EscrowEnabled, "budget_enabled": params.BudgetEnabled,
			"stream_enabled": params.StreamEnabled, "service_enabled": params.ServiceEnabled,
		}, fields)
		require.Equal(t, params, h.keeper.GetParams(h.ctx))
		require.Len(t, payStoreSnapshot(h.ctx, h.storeKey), 1)
	}
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	h.cms.Commit(true)
	reopened, err := NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, h.evm, h.custody, h.keeper.Authority())
	require.NoError(t, err)
	require.Equal(t, types.Params{Paused: true, EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}, reopened.GetParams(h.ctx))
	h.ctx.KVStore(h.storeKey).Delete(types.ParamsKey)
	require.Equal(t, types.DefaultParams(), reopened.GetParams(h.ctx))
	h.ctx.KVStore(h.storeKey).Set(types.ParamsKey, []byte("invalid JSON"))
	require.Panics(t, func() { reopened.GetParams(h.ctx) })
}

func TestPayKeeperRequiresHostAndKeys(t *testing.T) {
	h := newPayHarness(t)
	require.Same(t, h.host, h.keeper.Host())
	require.Equal(t, "pay-governance", h.keeper.Authority())
	require.Same(t, h.hostKey, h.keeper.hostKey)
	require.Same(t, h.storeKey, h.keeper.storeKey)
	require.Same(t, h.cdc, h.keeper.cdc)
	require.Same(t, h.evm, h.keeper.evm)
	require.Same(t, h.custody, h.keeper.custody)
	for _, tc := range []struct {
		name      string
		storeKey  sdk.StoreKey
		hostKey   sdk.StoreKey
		host      *lxphost.Host
		authority string
	}{
		{"nil host", h.storeKey, h.hostKey, nil, "pay-governance"},
		{"nil host key", h.storeKey, nil, h.host, "pay-governance"},
		{"nil module key", nil, h.hostKey, h.host, "pay-governance"},
		{"empty authority", h.storeKey, h.hostKey, h.host, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, err := NewKeeper(h.cdc, tc.storeKey, tc.hostKey, tc.host, h.evm, h.custody, tc.authority)
			require.Error(t, err)
			require.Equal(t, Keeper{}, k)
		})
	}
	for _, actor := range h.actors {
		public, found := h.keeper.evm.GetLayerXDid(h.ctx, actor.address)
		require.True(t, found)
		require.Equal(t, actor.public, public)
		identity, err := h.keeper.Host().ReadIdentityState(h.ctx, actor.did)
		require.NoError(t, err)
		require.Equal(t, actor.public, identity.PrimaryKey)
	}
	require.Empty(t, payStoreSnapshot(h.ctx, h.storeKey))
	require.NotEmpty(t, payStoreSnapshot(h.ctx, h.hostKey))
}
