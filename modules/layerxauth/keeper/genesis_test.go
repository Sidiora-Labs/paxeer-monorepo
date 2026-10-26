package keeper

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/stretchr/testify/require"
)

func TestAuthGenesisRoundTrip(t *testing.T) {
	h := newHarness(t)
	store := h.ctx.KVStore(h.key)
	hostStore := h.ctx.KVStore(h.hostKey)
	hostValue := []byte("shared kernel state")
	hostStore.Set(types.ParamsKey, hostValue)

	require.Equal(t, types.DefaultGenesis(), h.keeper.ExportGenesis(h.ctx))
	require.False(t, store.Has(types.ParamsKey), "exporting defaults must not write params")
	require.Equal(t, hostValue, hostStore.Get(types.ParamsKey))

	for _, tc := range []struct {
		name   string
		params types.Params
	}{
		{name: "retention 64", params: types.Params{IdempotencyRetention: 64}},
		{name: "paused with retention", params: types.Params{Paused: true, IdempotencyRetention: 64}},
		{name: "maximum retention", params: types.Params{Paused: true, IdempotencyRetention: math.MaxUint64}},
		{name: "defaults", params: types.DefaultParams()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := types.GenesisState{Params: tc.params}
			require.NoError(t, genesis.Validate())
			h.keeper.InitGenesis(h.ctx, genesis)
			require.True(t, store.Has(types.ParamsKey), "init must persist params, including defaults")
			require.Equal(t, genesis.Params, h.keeper.GetParams(h.ctx))
			var persisted types.Params
			h.keeper.cdc.MustUnmarshal(store.Get(types.ParamsKey), &persisted)
			require.Equal(t, genesis.Params, persisted)

			exported := h.keeper.ExportGenesis(h.ctx)
			require.Equal(t, &genesis, exported)
			require.NoError(t, exported.Validate())
			encoded, err := json.Marshal(exported)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &fields))
			require.Len(t, fields, 1, "kernel records must not be exported by auth genesis")
			require.Contains(t, fields, "params")
			require.Equal(t, hostValue, hostStore.Get(types.ParamsKey), "genesis must preserve shared host state")

			iterator := store.Iterator(nil, nil)
			defer iterator.Close()
			require.True(t, iterator.Valid())
			require.Equal(t, types.ParamsKey, iterator.Key())
			iterator.Next()
			require.False(t, iterator.Valid(), "auth genesis must store only params")

			restored := newHarness(t)
			restoredHost := restored.ctx.KVStore(restored.hostKey)
			restored.keeper.InitGenesis(restored.ctx, *exported)
			require.Equal(t, genesis.Params, restored.keeper.GetParams(restored.ctx))
			require.Equal(t, exported, restored.keeper.ExportGenesis(restored.ctx))
			hostIterator := restoredHost.Iterator(nil, nil)
			defer hostIterator.Close()
			require.False(t, hostIterator.Valid(), "auth genesis must not import host records")
		})
	}
}
