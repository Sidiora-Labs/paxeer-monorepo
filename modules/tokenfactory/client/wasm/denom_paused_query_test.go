package wasm_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm/bindings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func TestWasmGetDenomPaused(t *testing.T) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	transientKey := sdk.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, transientKey)
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), storeKey, transientKey, types.ModuleName)
	k := keeper.NewKeeper(cdc, storeKey, params, nil, nil, nil)
	handler := wasm.NewTokenFactoryWasmQueryHandler(&k)
	denom := "factory/creator/paused"
	issuerDid := "did:layerx:" + strings.Repeat("ab", 32)
	metadata := types.DenomAuthorityMetadata{IssuerDid: issuerDid}
	require.NoError(t, metadata.Validate())
	metadataBytes, err := proto.Marshal(&metadata)
	require.NoError(t, err)
	k.GetDenomPrefixStore(ctx, denom).Set([]byte(types.DenomAuthorityMetadataKey), metadataBytes)

	t.Run("json binding", func(t *testing.T) {
		raw := []byte(`{"denom_paused":{"denom":"factory/creator/paused"}}`)
		var query bindings.PaxTokenFactoryQuery
		require.NoError(t, json.Unmarshal(raw, &query))
		require.NotNil(t, query.DenomPaused)
		require.Equal(t, denom, query.DenomPaused.Denom)
		encoded, err := json.Marshal(query)
		require.NoError(t, err)
		require.JSONEq(t, string(raw), string(encoded))
		encoded, err = json.Marshal(bindings.PaxTokenFactoryQuery{})
		require.NoError(t, err)
		require.JSONEq(t, `{}`, string(encoded))
	})

	t.Run("pause transitions", func(t *testing.T) {
		req := &types.QueryDenomPausedRequest{Denom: denom}
		res, err := handler.GetDenomPaused(ctx, req)
		require.NoError(t, err)
		require.Equal(t, &types.QueryDenomPausedResponse{Paused: false, IssuerDid: issuerDid}, res)
		for _, paused := range []bool{true, false} {
			require.NoError(t, k.SetDenomPausedByIssuer(ctx, denom, issuerDid, paused))
			res, err = handler.GetDenomPaused(ctx, req)
			require.NoError(t, err)
			require.Equal(t, &types.QueryDenomPausedResponse{Paused: paused, IssuerDid: issuerDid}, res)
			keeperRes, err := k.DenomPaused(sdk.WrapSDKContext(ctx), req)
			require.NoError(t, err)
			require.Equal(t, keeperRes, res)
		}
	})

	for _, tc := range []struct {
		name string
		req  *types.QueryDenomPausedRequest
		code codes.Code
	}{
		{name: "nil request", code: codes.InvalidArgument},
		{name: "empty denom", req: &types.QueryDenomPausedRequest{}, code: codes.InvalidArgument},
		{name: "unknown denom", req: &types.QueryDenomPausedRequest{Denom: "factory/creator/unknown"}, code: codes.NotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := handler.GetDenomPaused(ctx, tc.req)
			require.Error(t, err)
			require.Equal(t, tc.code, status.Code(err))
			require.Nil(t, res)
		})
	}

	t.Run("keeper metadata error", func(t *testing.T) {
		corruptDenom := "factory/creator/corrupt"
		k.GetDenomPrefixStore(ctx, corruptDenom).Set([]byte(types.DenomAuthorityMetadataKey), []byte{0xff})
		req := &types.QueryDenomPausedRequest{Denom: corruptDenom}
		keeperRes, keeperErr := k.DenomPaused(sdk.WrapSDKContext(ctx), req)
		require.Error(t, keeperErr)
		res, err := handler.GetDenomPaused(ctx, req)
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
		require.EqualError(t, err, keeperErr.Error())
		require.Nil(t, keeperRes)
		require.Nil(t, res)
	})
}
