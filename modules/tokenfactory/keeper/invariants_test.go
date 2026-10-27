package keeper

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestPausedDenomsInvariantBroken(t *testing.T) {
	for _, tc := range []struct {
		name    string
		paused  bool
		present bool
		corrupt bool
		broken  bool
	}{
		{name: "unpaused denom without metadata"},
		{name: "paused denom without metadata", paused: true, broken: true},
		{name: "paused denom with metadata", paused: true, present: true},
		{name: "paused denom with corrupt metadata", paused: true, present: true, corrupt: true, broken: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeKey := sdk.NewKVStoreKey(types.StoreKey)
			ctx := testutil.DefaultContext(storeKey, sdk.NewTransientStoreKey("transient_test"))
			k := Keeper{storeKey: storeKey}
			invariant := PausedDenomsInvariant(k)
			message, broken := invariant(ctx)
			require.False(t, broken, message)

			creator := sdk.AccAddress(make([]byte, 20)).String()
			healthyDenom := "factory/" + creator + "/a"
			require.NoError(t, k.setAuthorityMetadata(ctx, healthyDenom, types.DenomAuthorityMetadata{Admin: creator}))
			require.NoError(t, k.setDenomPaused(ctx, healthyDenom, true))

			denom := "factory/" + creator + "/z"
			if tc.present {
				require.NoError(t, k.setAuthorityMetadata(ctx, denom, types.DenomAuthorityMetadata{}))
			}
			if tc.corrupt {
				k.GetDenomPrefixStore(ctx, denom).Set([]byte(types.DenomAuthorityMetadataKey), []byte{0xff})
			}
			if tc.paused {
				ctx.KVStore(storeKey).Set(types.GetDenomPausedKey(denom), []byte{1})
			}

			message, broken = invariant(ctx)
			require.Equal(t, tc.broken, broken, message)
			require.Contains(t, message, "tokenfactory: paused-denoms invariant")
			if tc.broken {
				require.Contains(t, message, denom)
				require.NoError(t, k.setDenomPaused(ctx, denom, false))
				message, broken = invariant(ctx)
				require.False(t, broken, message)
			}
		})
	}
}
