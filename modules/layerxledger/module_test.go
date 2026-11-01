package layerxledger_test

import (
	"encoding/json"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
)

func TestLedgerModuleServices(t *testing.T) {
	basic := layerxledger.NewAppModuleBasic()
	require.Equal(t, "layerxledger", basic.Name())
	registry := cdctypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	defaultGenesis := basic.DefaultGenesis(cdc)
	var genesis types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(defaultGenesis, &genesis))
	require.Equal(t, *types.DefaultGenesis(), genesis)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, defaultGenesis))
	for _, invalid := range []json.RawMessage{
		json.RawMessage(`{`),
		json.RawMessage(`{"params":{"paused":"invalid"}}`),
	} {
		err := basic.ValidateGenesis(cdc, nil, invalid)
		require.ErrorContains(t, err, "failed to unmarshal layerxledger genesis state")
	}

	genesisCh := make(chan json.RawMessage, 1)
	genesisCh <- defaultGenesis
	close(genesisCh)
	require.NoError(t, basic.ValidateGenesisStream(cdc, nil, genesisCh))
	invalidCh := make(chan json.RawMessage, 1)
	invalidCh <- json.RawMessage(`{`)
	close(invalidCh)
	require.Error(t, basic.ValidateGenesisStream(cdc, nil, invalidCh))

	authority := authtypes.NewModuleAddress("ledger-admin").String()
	msg := &types.MsgUpdateParams{Authority: authority, Params: types.Params{Paused: true}}
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	basic.RegisterLegacyAminoCodec(amino)
	encoded, err := amino.MarshalAsJSON(msg)
	require.NoError(t, err)
	var aminoMsg sdk.Msg
	require.NoError(t, amino.UnmarshalAsJSON(encoded, &aminoMsg))
	require.Equal(t, msg, aminoMsg)
	encoded, err = cdc.MarshalInterface(msg)
	require.NoError(t, err)
	var protoMsg sdk.Msg
	require.NoError(t, cdc.UnmarshalInterface(encoded, &protoMsg))
	require.Equal(t, msg, protoMsg)

	db := tmdb.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	stores := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	stores.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmtypes.Header{}, false)
	k := keeper.NewKeeper(cdc, key, nil, nil, nil, authority)
	am := layerxledger.NewAppModule(k)
	require.Equal(t, "layerxledger", am.Name())
	require.Equal(t, uint64(1), am.ConsensusVersion())
	require.Empty(t, am.InitGenesis(ctx, cdc, defaultGenesis))
	require.True(t, ctx.KVStore(key).Has(types.ParamsKey))
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))

	msgServer, queryServer := grpc.NewServer(), grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	t.Cleanup(queryServer.Stop)
	am.RegisterServices(module.NewConfigurator(cdc, msgServer, queryServer))
	services := msgServer.GetServiceInfo()
	require.Len(t, services, 1)
	require.Contains(t, services, "paxprotocol.paxchain.layerxledger.Msg")
	methods := services["paxprotocol.paxchain.layerxledger.Msg"].Methods
	require.Len(t, methods, 1)
	require.Equal(t, "UpdateParams", methods[0].Name)
	require.False(t, methods[0].IsClientStream)
	require.False(t, methods[0].IsServerStream)
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

	unauthorized := &types.MsgUpdateParams{
		Authority: authtypes.NewModuleAddress("other-admin").String(),
		Params:    types.Params{Paused: false},
	}
	result, err = handler(ctx, unauthorized)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, result)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	require.Empty(t, queryServer.GetServiceInfo())

	exported := am.ExportGenesis(ctx, cdc)
	require.NoError(t, cdc.UnmarshalAsJSON(exported, &genesis))
	require.Equal(t, types.GenesisState{Params: msg.Params}, genesis)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, exported))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exported, &fields))
	require.Len(t, fields, 1)
	require.Contains(t, fields, "params")
	var streamed []json.RawMessage
	for chunk := range am.ExportGenesisStream(ctx, cdc) {
		streamed = append(streamed, chunk)
	}
	require.Equal(t, []json.RawMessage{exported}, streamed)

	require.Empty(t, am.InitGenesis(ctx, cdc, defaultGenesis))
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))
	require.Empty(t, am.InitGenesis(ctx, cdc, exported))
	require.Equal(t, genesis.Params, k.GetParams(ctx))
	require.JSONEq(t, string(exported), string(am.ExportGenesis(ctx, cdc)))
}
