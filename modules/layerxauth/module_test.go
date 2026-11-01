package layerxauth_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAuthModuleServices(t *testing.T) {
	basic := layerxauth.NewAppModuleBasic()
	require.Equal(t, "layerxauth", basic.Name())
	registry := cdctypes.NewInterfaceRegistry()
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	authority := sdk.AccAddress([]byte("auth-module-authority")).String()
	msg := &types.MsgUpdateParams{
		Authority: authority,
		Params:    types.Params{Paused: true, IdempotencyRetention: 64},
	}
	packedMsg, err := cdctypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	var unpackedMsg sdk.Msg
	require.NoError(t, cdc.UnpackAny(packedMsg, &unpackedMsg))
	require.Equal(t, msg, unpackedMsg)
	packedAuth, err := cdctypes.NewAnyWithValue(&types.LayerXScopeAuthorization{})
	require.NoError(t, err)
	var unpackedAuth authz.Authorization
	require.NoError(t, cdc.UnpackAny(packedAuth, &unpackedAuth))
	require.IsType(t, &types.LayerXScopeAuthorization{}, unpackedAuth)

	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	basic.RegisterLegacyAminoCodec(amino)
	var aminoMsg sdk.Msg
	require.NoError(t, amino.UnmarshalAsJSON(amino.MustMarshalJSON(msg), &aminoMsg))
	require.Equal(t, msg, aminoMsg)

	defaults := basic.DefaultGenesis(cdc)
	var defaultState types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(defaults, &defaultState))
	require.Equal(t, types.DefaultGenesis(), &defaultState)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, defaults))
	malformed := json.RawMessage(`{"params":`)
	err = basic.ValidateGenesis(cdc, nil, malformed)
	require.ErrorContains(t, err, "failed to unmarshal layerxauth genesis state")
	validStream := make(chan json.RawMessage, 1)
	validStream <- defaults
	close(validStream)
	require.NoError(t, basic.ValidateGenesisStream(cdc, nil, validStream))
	invalidStream := make(chan json.RawMessage, 2)
	invalidStream <- defaults
	invalidStream <- malformed
	close(invalidStream)
	require.Error(t, basic.ValidateGenesisStream(cdc, nil, invalidStream))

	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("layerxauth_transient"))
	k := keeper.NewKeeper(cdc, key, nil, nil, nil, authority)
	am := layerxauth.NewAppModule(k)
	require.Equal(t, "layerxauth", am.Name())
	require.Equal(t, uint64(1), am.ConsensusVersion())
	genesis := types.GenesisState{Params: types.Params{IdempotencyRetention: 32}}
	genesisJSON := cdc.MustMarshalJSON(&genesis)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, genesisJSON))
	require.Empty(t, am.InitGenesis(ctx, cdc, genesisJSON))
	require.Equal(t, genesis.Params, k.GetParams(ctx))
	var persisted types.Params
	cdc.MustUnmarshal(ctx.KVStore(key).Get(types.ParamsKey), &persisted)
	require.Equal(t, genesis.Params, persisted)
	var exported types.GenesisState
	cdc.MustUnmarshalJSON(am.ExportGenesis(ctx, cdc), &exported)
	require.Equal(t, genesis, exported)
	require.Equal(t, k.ExportGenesis(ctx), &exported)
	chunks := 0
	for chunk := range am.ExportGenesisStream(ctx, cdc) {
		chunks++
		require.JSONEq(t, string(genesisJSON), string(chunk))
	}
	require.Equal(t, 1, chunks)
	require.Panics(t, func() { am.InitGenesis(ctx, cdc, malformed) })
	require.Equal(t, genesis.Params, k.GetParams(ctx))

	msgServer := grpc.NewServer()
	queryServer := grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	t.Cleanup(queryServer.Stop)
	am.RegisterServices(module.NewConfigurator(cdc, msgServer, queryServer))
	services := msgServer.GetServiceInfo()
	require.Len(t, services, 1)
	service, ok := services["paxprotocol.paxchain.layerxauth.Msg"]
	require.True(t, ok)
	require.Len(t, service.Methods, 1)
	require.Equal(t, "UpdateParams", service.Methods[0].Name)
	require.Empty(t, queryServer.GetServiceInfo())

	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, queryServer))
	handler := router.Handler(msg)
	require.NotNil(t, handler)
	result, err := handler(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	cdc.MustUnmarshalJSON(am.ExportGenesis(ctx, cdc), &exported)
	require.Equal(t, msg.Params, exported.Params)

	unauthorized := &types.MsgUpdateParams{
		Authority: sdk.AccAddress([]byte("other-auth-authority")).String(),
		Params:    types.DefaultParams(),
	}
	result, err = handler(ctx, unauthorized)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, result)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	require.Empty(t, queryServer.GetServiceInfo())

	restoredCtx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("restored_auth_transient"))
	require.Empty(t, am.InitGenesis(restoredCtx, cdc, am.ExportGenesis(ctx, cdc)))
	require.Equal(t, k.GetParams(ctx), k.GetParams(restoredCtx))
}
