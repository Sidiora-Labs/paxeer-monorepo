package keeper_test

import (
	"math"
	"testing"

	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestAuthKeeperParamsRoundTrip(t *testing.T) {
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	require.Same(t, app.AppCodec(), h.Keeper().TestCodec())
	require.Same(t, h.StoreKey(), h.Keeper().TestStoreKey())
	require.Same(t, h.Host(), h.Keeper().TestHost())
	require.Same(t, &app.AuthzKeeper, *h.Keeper().TestAuthzKeeper())
	require.Same(t, &app.EvmKeeper, h.Keeper().TestEVMKeeper())
	require.Equal(t, h.Authority(), h.Keeper().TestAuthority())
	require.Nil(t, h.Context().KVStore(h.StoreKey()).Get(types.ParamsKey))
	require.Equal(t, types.DefaultParams(), h.Keeper().GetParams(*h.Context()))
	require.Nil(t, h.Context().KVStore(h.StoreKey()).Get(types.ParamsKey))

	for _, params := range []types.Params{
		{Paused: true, IdempotencyRetention: 4096},
		{Paused: false, IdempotencyRetention: math.MaxUint64},
		types.DefaultParams(),
	} {
		require.NoError(t, h.Keeper().SetParams(*h.Context(), params))
		require.Equal(t, params, h.Keeper().GetParams(*h.Context()))
		var stored types.Params
		h.Keeper().TestCodec().MustUnmarshal(h.Context().KVStore(h.StoreKey()).Get(types.ParamsKey), &stored)
		require.Equal(t, params, stored)
		fresh := keeper.NewKeeper(app.AppCodec(), h.StoreKey(), h.Host(), &app.AuthzKeeper, &app.EvmKeeper, h.Authority())
		require.Equal(t, params, fresh.GetParams(*h.Context()))

		func() {
			iterator := h.Context().KVStore(h.StoreKey()).Iterator(nil, nil)
			defer iterator.Close()
			require.True(t, iterator.Valid())
			require.Equal(t, types.ParamsKey, iterator.Key())
			require.Equal(t, h.Keeper().TestCodec().MustMarshal(&params), iterator.Value())
			iterator.Next()
			require.False(t, iterator.Valid(), "auth store must contain only ParamsKey")
		}()
	}
}
