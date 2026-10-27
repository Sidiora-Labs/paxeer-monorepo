package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestQueryDenomPausedReportsState(t *testing.T) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(storeKey, sdk.NewTransientStoreKey("transient_test"))
	k := Keeper{storeKey: storeKey}
	queryCtx := sdk.WrapSDKContext(ctx)
	denom := "factory/creator/bitcoin"
	issuer := evmtypes.LayerXDid([32]byte{1})
	require.NoError(t, k.setAuthorityMetadata(ctx, denom, types.DenomAuthorityMetadata{IssuerDid: issuer}))

	for _, tc := range []struct {
		name   string
		paused bool
	}{
		{name: "initially unpaused", paused: false},
		{name: "paused", paused: true},
		{name: "unpaused again", paused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name != "initially unpaused" {
				require.NoError(t, k.setDenomPaused(ctx, denom, tc.paused))
			}
			metadata, err := k.GetAuthorityMetadata(ctx, denom)
			require.NoError(t, err)
			require.Equal(t, issuer, metadata.IssuerDid)
			require.Equal(t, tc.paused, k.IsDenomPaused(ctx, denom))

			response, err := k.DenomPaused(queryCtx, &types.QueryDenomPausedRequest{Denom: denom})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tc.paused, response.Paused)
			require.Equal(t, k.IsDenomPaused(ctx, denom), response.Paused)
			require.Equal(t, metadata.IssuerDid, response.IssuerDid)
		})
	}

	t.Run("known denom without issuer", func(t *testing.T) {
		other := "factory/creator/ether"
		require.NoError(t, k.setAuthorityMetadata(ctx, other, types.DenomAuthorityMetadata{
			Admin: sdk.AccAddress(make([]byte, 20)).String(),
		}))
		response, err := k.DenomPaused(queryCtx, &types.QueryDenomPausedRequest{Denom: other})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.False(t, response.Paused)
		require.Empty(t, response.IssuerDid)
	})

	t.Run("unknown denom", func(t *testing.T) {
		missing := "factory/creator/missing"
		require.False(t, k.GetDenomPrefixStore(ctx, missing).Has([]byte(types.DenomAuthorityMetadataKey)))
		response, err := k.DenomPaused(queryCtx, &types.QueryDenomPausedRequest{Denom: missing})
		require.Equal(t, codes.NotFound, status.Code(err))
		require.Nil(t, response)
	})
}

func TestQueryDenomPausedRejectsEmptyDenom(t *testing.T) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(storeKey, sdk.NewTransientStoreKey("transient_test"))
	k := Keeper{storeKey: storeKey}

	for _, tc := range []struct {
		name    string
		request *types.QueryDenomPausedRequest
	}{
		{name: "nil request", request: nil},
		{name: "empty denom", request: &types.QueryDenomPausedRequest{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := k.DenomPaused(sdk.WrapSDKContext(ctx), tc.request)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Nil(t, response)
		})
	}
}
