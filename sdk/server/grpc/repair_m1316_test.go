package grpc_test

import (
	"bytes"
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/grpc/gogoreflection"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func repairM1316Bank(t *testing.T) (network.Config, sdk.Context, bankkeeper.BaseKeeper, sdk.Coins) {
	t.Helper()
	cfg := network.DefaultConfig(t)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1, ChainID: cfg.ChainID}, false)
	pk := paramskeeper.NewKeeper(cfg.Codec, cfg.LegacyAmino, keys[paramtypes.StoreKey], tkey)
	ak := authkeeper.NewAccountKeeper(cfg.Codec, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{})
	bk := bankkeeper.NewBaseKeeper(cfg.Codec, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), map[string]bool{})
	genesis := banktypes.DefaultGenesisState()
	genesis.Balances = []banktypes.Balance{
		{Address: sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("alpha", 71), sdk.NewInt64Coin("beta", 19))},
		{Address: sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("alpha", 29), sdk.NewInt64Coin("gamma", 300))},
	}
	expected := sdk.NewCoins(sdk.NewInt64Coin("alpha", 100), sdk.NewInt64Coin("beta", 19), sdk.NewInt64Coin("gamma", 300))
	genesis.Supply = expected
	require.NoError(t, genesis.Validate())
	cfg.GenesisState[banktypes.ModuleName] = cfg.Codec.MustMarshalJSON(genesis)
	bk.InitGenesis(ctx, genesis)
	return cfg, ctx, bk, expected
}

func repairM1316Server(t *testing.T, ctx sdk.Context, bk bankkeeper.BaseKeeper) *grpc.Server {
	t.Helper()
	server := grpc.NewServer(grpc.UnaryInterceptor(func(callCtx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(ctx.WithContext(callCtx)), req)
	}))
	banktypes.RegisterQueryServer(server, bk)
	gogoreflection.Register(server)
	t.Cleanup(server.Stop)
	return server
}

func TestRepairM_1316_GRPCWebSupply(t *testing.T) {
	cfg, ctx, bk, expected := repairM1316Bank(t)
	server := repairM1316Server(t, ctx, bk)
	web := httptest.NewServer(grpcweb.WrapServer(server))
	t.Cleanup(web.Close)
	webCfg := config.DefaultConfig()
	webCfg.GRPCWeb.Address = strings.TrimPrefix(web.URL, "http://")
	s := &GRPCWebTestSuite{
		cfg:      cfg,
		network:  &network.Network{Config: cfg, Validators: []*network.Validator{{AppConfig: webCfg}}},
		protoCdc: codec.NewProtoCodec(cfg.InterfaceRegistry),
	}
	s.SetT(t)
	s.Test_Total_Supply()

	headers, trailers, messages, err := s.makeGrpcRequest("/cosmos.bank.v1beta1.Query/TotalSupply", headerWithFlag(), serializeProtoMessages([]proto.Message{&banktypes.QueryTotalSupplyRequest{}}), false)
	require.NoError(t, err)
	require.Equal(t, grpcWebContentType, headers.Get("Content-Type"))
	require.Equal(t, "0", trailers.Get("grpc-status"))
	require.Len(t, messages, 1)
	var response banktypes.QueryTotalSupplyResponse
	require.NoError(t, s.protoCdc.Unmarshal(messages[0], &response))
	require.Equal(t, expected, response.Supply)
	// A truncated payload from the real producer must fail protobuf decoding.
	require.Error(t, s.protoCdc.Unmarshal(messages[0][:len(messages[0])-1], &banktypes.QueryTotalSupplyResponse{}))
}

func TestRepairM_1316_GRPCConstructorsAndCredentials(t *testing.T) {
	cfg, ctx, bk, expected := repairM1316Bank(t)
	require.NotNil(t, cfg.AppConstructor)
	app := cfg.AppConstructor(network.Validator{})
	require.NotNil(t, app)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	server := repairM1316Server(t, ctx, bk)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		require.NoError(t, <-served)
	})
	callCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(callCtx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	response, err := banktypes.NewQueryClient(conn).TotalSupply(callCtx, &banktypes.QueryTotalSupplyRequest{})
	require.NoError(t, err)
	require.Equal(t, expected, response.Supply)
	stream, err := rpb.NewServerReflectionClient(conn).ServerReflectionInfo(callCtx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&rpb.ServerReflectionRequest{MessageRequest: &rpb.ServerReflectionRequest_ListServices{ListServices: ""}}))
	reflected, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, reflected.GetListServicesResponse())
	services := []string{}
	for _, service := range reflected.GetListServicesResponse().Service {
		services = append(services, service.Name)
	}
	require.Contains(t, services, "cosmos.bank.v1beta1.Query")
	require.NoError(t, stream.CloseSend())
}
