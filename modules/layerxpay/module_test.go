package layerxpay_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	proofcodec "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
)

func TestPayAppModuleGenesisAndServices(t *testing.T) {
	basic := layerxpay.NewAppModuleBasic()
	require.Equal(t, "layerxpay", basic.Name())
	registry := cdctypes.NewInterfaceRegistry()
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	authority := sdk.AccAddress([]byte("pay-module-authority")).String()
	msg := &types.MsgUpdateParams{Authority: authority, Params: types.Params{Paused: true}}
	packed, err := cdctypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	var unpacked sdk.Msg
	require.NoError(t, cdc.UnpackAny(packed, &unpacked))
	require.Equal(t, msg, unpacked)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	basic.RegisterLegacyAminoCodec(amino)
	var aminoMsg sdk.Msg
	require.NoError(t, amino.UnmarshalAsJSON(amino.MustMarshalJSON(msg), &aminoMsg))
	require.Equal(t, msg, aminoMsg)
	require.Nil(t, basic.GetTxCmd())
	queryCmd := basic.GetQueryCmd()
	require.Equal(t, types.ModuleName, queryCmd.Use)
	paramsCmd, _, err := queryCmd.Find([]string{"params"})
	require.NoError(t, err)
	require.Equal(t, "params", paramsCmd.Use)
	require.NotPanics(t, func() {
		basic.RegisterGRPCGatewayRoutes(client.Context{}, runtime.NewServeMux())
	})

	key := sdk.NewKVStoreKey(types.StoreKey)
	hostKey := sdk.NewKVStoreKey("lxphost")
	evmKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	newContext := func() sdk.Context {
		db := dbm.NewMemDB()
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		cms := store.NewCommitMultiStore(db)
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
		cms.MountStoreWithDB(hostKey, sdk.StoreTypeIAVL, db)
		cms.MountStoreWithDB(evmKey, sdk.StoreTypeIAVL, db)
		require.NoError(t, cms.LoadLatestVersion())
		return sdk.NewContext(cms, tmproto.Header{ChainID: "hyperpax_125-1", Height: 1}, false)
	}
	ctx := newContext()
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	k, err := keeper.NewKeeper(cdc, key, hostKey, host, testutil.NewEVMBindingReader(cdc, evmKey), nil, authority)
	require.NoError(t, err)
	am := layerxpay.NewAppModule(k)
	require.Equal(t, "layerxpay", am.Name())
	require.Equal(t, uint64(1), am.ConsensusVersion())
	for _, candidate := range []interface{}{am, &am} {
		_, hasBeginBlock := candidate.(module.BeginBlockAppModule)
		_, hasEndBlock := candidate.(module.EndBlockAppModule)
		require.False(t, hasBeginBlock)
		require.False(t, hasEndBlock)
	}

	defaults := basic.DefaultGenesis(cdc)
	var defaultState types.GenesisState
	require.NoError(t, json.Unmarshal(defaults, &defaultState))
	require.Equal(t, types.DefaultGenesis(), &defaultState)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, defaults))
	require.Empty(t, am.InitGenesis(ctx, cdc, defaults))
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))
	require.JSONEq(t, string(defaults), string(am.ExportGenesis(ctx, cdc)))

	enableKey, err := types.KernelModuleEnableKey(types.EscrowModuleID)
	require.NoError(t, err)
	enabled := make([]byte, 32)
	enabled[31] = 1
	escrow := types.EscrowRecord{
		EscrowID: [32]byte{1}, Owner: [32]byte{2}, EscrowAccount: [32]byte{3},
		Beneficiary: [32]byte{4}, Arbiter: [32]byte{5}, AssetID: [32]byte{6},
		LockedAmount: proofcodec.U128{Lo: 100}, State: 1,
		ExpiryMs: 1000000, DisputeWindowMs: 1000,
		TermsHash: [32]byte{7}, AgreementReference: [32]byte{8},
	}
	var escrowBytes bytes.Buffer
	require.NoError(t, binary.Write(&escrowBytes, binary.BigEndian, escrow))
	require.Len(t, escrowBytes.Bytes(), types.EscrowRecordBytes)
	decodedEscrow, err := types.DecodeEscrowRecord(escrowBytes.Bytes())
	require.NoError(t, err)
	require.Equal(t, escrow, decodedEscrow)
	escrowKey := types.KernelKVKey(types.EscrowModuleID, append([]byte("hold:"), escrow.EscrowID[:]...))
	genesis := types.GenesisState{
		Params: types.Params{Paused: false, EscrowEnabled: true},
		KernelRecords: []kv.Pair{
			{Key: escrowKey, Value: escrowBytes.Bytes()},
			{Key: enableKey, Value: enabled},
		},
	}
	genesisJSON, err := json.Marshal(genesis)
	require.NoError(t, err)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, genesisJSON))
	require.Empty(t, am.InitGenesis(ctx, cdc, genesisJSON))
	storedEscrow, found, err := k.GetEscrow(ctx, escrow.EscrowID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, escrow, storedEscrow)
	require.Equal(t, genesis.Params, k.GetParams(ctx))
	require.NotEmpty(t, ctx.KVStore(key).Get(types.ParamsKey))
	for _, record := range genesis.KernelRecords {
		require.Equal(t, record.Value, ctx.KVStore(hostKey).Get(record.Key))
		require.Nil(t, ctx.KVStore(key).Get(record.Key))
	}
	var exported types.GenesisState
	require.NoError(t, json.Unmarshal(am.ExportGenesis(ctx, cdc), &exported))
	require.Equal(t, genesis, exported)
	require.Equal(t, k.ExportGenesis(ctx), &exported)
	chunks := 0
	for chunk := range am.ExportGenesisStream(ctx, cdc) {
		chunks++
		require.JSONEq(t, string(genesisJSON), string(chunk))
	}
	require.Equal(t, 1, chunks)
	restoredCtx := newContext()
	require.Empty(t, am.InitGenesis(restoredCtx, cdc, am.ExportGenesis(ctx, cdc)))
	require.Equal(t, k.ExportGenesis(ctx), k.ExportGenesis(restoredCtx))
	for _, corrupt := range []struct {
		name             string
		key              []byte
		wantGenesisError string
	}{
		{"unknown_key", types.KernelKVKey(types.EscrowModuleID, []byte("escrow-state")), keeper.ErrEscrowCorruptState.Error() + ": unknown escrow key: invalid genesis"},
		{"hold", escrowKey, "invalid hold: record body: LXP_NON_CANONICAL: invalid genesis"},
	} {
		t.Run("placeholder/"+corrupt.name, func(t *testing.T) {
			invalid := genesis
			invalid.KernelRecords = []kv.Pair{
				{Key: corrupt.key, Value: []byte{1, 2, 3}},
				{Key: enableKey, Value: enabled},
			}
			invalidJSON, err := json.Marshal(invalid)
			require.NoError(t, err)
			err = k.InitGenesis(ctx, invalid)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.EqualError(t, err, corrupt.wantGenesisError)
			require.Panics(t, func() { am.InitGenesis(ctx, cdc, invalidJSON) })
			require.JSONEq(t, string(genesisJSON), string(am.ExportGenesis(ctx, cdc)))
			if corrupt.name == "hold" {
				corruptCtx := newContext()
				corruptCtx.KVStore(hostKey).Set(escrowKey, []byte{1, 2, 3})
				record, found, err := k.GetEscrow(corruptCtx, escrow.EscrowID)
				require.ErrorIs(t, err, keeper.ErrEscrowCorruptState)
				require.False(t, found)
				require.Equal(t, types.EscrowRecord{}, record)
			}
		})
	}

	malformed := json.RawMessage(`{"params":`)
	err = basic.ValidateGenesis(cdc, nil, malformed)
	require.ErrorContains(t, err, "failed to unmarshal layerxpay genesis state")
	require.Panics(t, func() { am.InitGenesis(ctx, cdc, malformed) })
	for _, invalid := range []types.GenesisState{
		{Params: types.Params{BudgetEnabled: true}},
		{KernelRecords: []kv.Pair{{Key: []byte{1}, Value: []byte{1}}}},
		{KernelRecords: []kv.Pair{genesis.KernelRecords[0], genesis.KernelRecords[0]}},
	} {
		invalidJSON, err := json.Marshal(invalid)
		require.NoError(t, err)
		require.ErrorIs(t, basic.ValidateGenesis(cdc, nil, invalidJSON), types.ErrInvalidGenesis)
		require.Panics(t, func() { am.InitGenesis(ctx, cdc, invalidJSON) })
		require.JSONEq(t, string(genesisJSON), string(am.ExportGenesis(ctx, cdc)))
	}
	validStream := make(chan json.RawMessage, 1)
	validStream <- defaults
	close(validStream)
	require.NoError(t, basic.ValidateGenesisStream(cdc, nil, validStream))
	invalidStream := make(chan json.RawMessage, 2)
	invalidStream <- defaults
	invalidStream <- malformed
	close(invalidStream)
	require.Error(t, basic.ValidateGenesisStream(cdc, nil, invalidStream))

	msgServer, queryServer := grpc.NewServer(), grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	t.Cleanup(queryServer.Stop)
	am.RegisterServices(module.NewConfigurator(cdc, msgServer, queryServer))
	for _, service := range []struct {
		server *grpc.Server
		name   string
		method string
	}{
		{msgServer, "paxprotocol.paxchain.layerxpay.Msg", "UpdateParams"},
		{queryServer, "paxprotocol.paxchain.layerxpay.Query", "Params"},
	} {
		services := service.server.GetServiceInfo()
		require.Len(t, services, 1)
		info, ok := services[service.name]
		require.True(t, ok)
		require.Len(t, info.Methods, 1)
		require.Equal(t, service.method, info.Methods[0].Name)
	}
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, msgRouter, queryRouter))
	query := queryRouter.Route("/paxprotocol.paxchain.layerxpay.Query/Params")
	require.NotNil(t, query)
	queryParams := func() types.Params {
		response, err := query(ctx, abci.RequestQuery{Data: cdc.MustMarshal(&types.QueryParamsRequest{})})
		require.NoError(t, err)
		var result types.QueryParamsResponse
		require.NoError(t, cdc.Unmarshal(response.Value, &result))
		return result.Params
	}
	require.Equal(t, genesis.Params, queryParams())
	handler := msgRouter.Handler(msg)
	require.NotNil(t, handler)
	result, err := handler(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	require.Equal(t, msg.Params, queryParams())
	require.NoError(t, json.Unmarshal(am.ExportGenesis(ctx, cdc), &exported))
	require.Equal(t, msg.Params, exported.Params)
	require.Equal(t, genesis.KernelRecords, exported.KernelRecords)
	unauthorized := &types.MsgUpdateParams{
		Authority: sdk.AccAddress([]byte("other-pay-authority!")).String(),
		Params:    genesis.Params,
	}
	result, err = handler(ctx, unauthorized)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, result)
	require.Equal(t, msg.Params, queryParams())
	require.Equal(t, genesis.KernelRecords, k.ExportGenesis(ctx).KernelRecords)
}
