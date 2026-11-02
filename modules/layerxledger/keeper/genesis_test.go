package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/stretchr/testify/require"
)

func TestLedgerGenesisRoundTrip(t *testing.T) {
	h := newHarness(t)
	store := h.ctx.KVStore(h.storeKey)
	hostStore := h.ctx.KVStore(h.hostKey)
	hostValue := []byte("shared kernel state")
	hostStore.Set(types.ParamsKey, hostValue)

	require.Equal(t, types.DefaultGenesis(), h.keeper.ExportGenesis(h.ctx))
	require.False(t, store.Has(types.ParamsKey), "exporting defaults must not write params")

	for _, tc := range []struct {
		name   string
		paused bool
	}{
		{name: "paused", paused: true},
		{name: "unpaused", paused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := types.GenesisState{Params: types.Params{Paused: tc.paused}}
			require.NoError(t, genesis.Validate())
			h.keeper.InitGenesis(h.ctx, genesis)
			require.True(t, store.Has(types.ParamsKey), "init must persist params, including defaults")
			require.Equal(t, genesis.Params, h.keeper.GetParams(h.ctx))

			exported := h.keeper.ExportGenesis(h.ctx)
			require.Equal(t, &genesis, exported)
			require.NoError(t, exported.Validate())
			encoded, err := json.Marshal(exported)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &fields))
			require.Len(t, fields, 1, "kernel records must not be exported by the ledger")
			require.Contains(t, fields, "params")
			require.Equal(t, hostValue, hostStore.Get(types.ParamsKey), "genesis must preserve shared host state")

			restored := newHarness(t)
			restoredHost := restored.ctx.KVStore(restored.hostKey)
			require.False(t, restoredHost.Has(types.ParamsKey))
			restored.keeper.InitGenesis(restored.ctx, *exported)
			require.Equal(t, genesis.Params, restored.keeper.GetParams(restored.ctx))
			require.Equal(t, exported, restored.keeper.ExportGenesis(restored.ctx))
			require.False(t, restoredHost.Has(types.ParamsKey), "ledger genesis must not import host records")
		})
	}
}
