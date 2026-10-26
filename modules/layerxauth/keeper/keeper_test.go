package keeper

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestAuthKeeperParamsRoundTrip(t *testing.T) {
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	require.Same(t, app.AppCodec(), h.keeper.cdc)
	require.Same(t, h.key, h.keeper.storeKey)
	require.Same(t, h.host, h.keeper.host)
	require.Same(t, &app.AuthzKeeper, h.keeper.authzKeeper)
	require.Same(t, &app.EvmKeeper, h.keeper.evmKeeper)
	require.Equal(t, h.authority, h.keeper.authority)
	require.Nil(t, h.ctx.KVStore(h.key).Get(types.ParamsKey))
	require.Equal(t, types.DefaultParams(), h.keeper.GetParams(h.ctx))
	require.Nil(t, h.ctx.KVStore(h.key).Get(types.ParamsKey))

	for _, params := range []types.Params{
		{Paused: true, IdempotencyRetention: 4096},
		{Paused: false, IdempotencyRetention: math.MaxUint64},
		types.DefaultParams(),
	} {
		require.NoError(t, h.keeper.SetParams(h.ctx, params))
		require.Equal(t, params, h.keeper.GetParams(h.ctx))
		var stored types.Params
		h.keeper.cdc.MustUnmarshal(h.ctx.KVStore(h.key).Get(types.ParamsKey), &stored)
		require.Equal(t, params, stored)
		fresh := NewKeeper(app.AppCodec(), h.key, h.host, &app.AuthzKeeper, &app.EvmKeeper, h.authority)
		require.Equal(t, params, fresh.GetParams(h.ctx))

		func() {
			iterator := h.ctx.KVStore(h.key).Iterator(nil, nil)
			defer iterator.Close()
			require.True(t, iterator.Valid())
			require.Equal(t, types.ParamsKey, iterator.Key())
			require.Equal(t, h.keeper.cdc.MustMarshal(&params), iterator.Value())
			iterator.Next()
			require.False(t, iterator.Valid(), "auth store must contain only ParamsKey")
		}()
	}
}
