package keeper_test

import (
	"testing"

	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestKeeperParamsRoundTrip(t *testing.T) {
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	store := h.ctx.KVStore(h.storeKey)
	hostStore := h.ctx.KVStore(h.hostKey)
	hostValue := []byte("shared host state")
	hostStore.Set(types.ParamsKey, hostValue)

	require.False(t, store.Has(types.ParamsKey))
	require.Equal(t, types.DefaultParams(), h.keeper.GetParams(h.ctx))
	require.False(t, store.Has(types.ParamsKey), "reading defaults must not write params")

	for _, paused := range []bool{true, false} {
		params := types.Params{Paused: paused}
		require.NoError(t, h.keeper.SetParams(h.ctx, params))
		require.True(t, store.Has(types.ParamsKey))
		require.Equal(t, app.AppCodec().MustMarshal(&params), store.Get(types.ParamsKey))
		require.Equal(t, params, h.keeper.GetParams(h.ctx))

		reopened := ledgerkeeper.NewKeeper(app.AppCodec(), h.storeKey, h.host, app.BankKeeper, &app.EvmKeeper, h.authority)
		require.Equal(t, params, reopened.GetParams(h.ctx))
		require.Equal(t, hostValue, hostStore.Get(types.ParamsKey), "ledger params must use their own store")
	}
}
