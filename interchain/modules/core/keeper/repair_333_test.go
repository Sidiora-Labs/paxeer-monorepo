package keeper_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ibc "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func repair333QueryTransport(t *testing.T) (*ibctesting.TestChain, *grpc.ClientConn) {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 1)
	chain := coordinator.GetChain(ibctesting.GetChainID(1))
	app := chain.GetSimApp()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	listener := bufconn.Listen(1024 * 1024)
	queryServer := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(chain.GetContext().WithContext(ctx)), req)
	}))
	msgServer := grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	ibc.NewAppModule(app.IBCKeeper).RegisterServices(module.NewConfigurator(app.AppCodec(), msgServer, queryServer))
	serveResult := make(chan error, 1)
	go func() { serveResult <- queryServer.Serve(listener) }()
	t.Cleanup(func() {
		queryServer.Stop()
		require.NoError(t, listener.Close())
		require.NoError(t, <-serveResult)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "passthrough:///repair-333", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return chain, conn
}

func TestRepair_333_UpgradedConsensusStateGRPC(t *testing.T) {
	chain, conn := repair333QueryTransport(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	queryClient := clienttypes.NewQueryClient(conn)
	sdkCtx := chain.GetContext()
	keeper := chain.GetSimApp().IBCKeeper
	response, err := keeper.UpgradedConsensusState(sdk.WrapSDKContext(sdkCtx), nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	assertMissing := func() {
		t.Helper()
		response, err := queryClient.UpgradedConsensusState(ctx, &clienttypes.QueryUpgradedConsensusStateRequest{})
		require.Nil(t, response)
		require.Equal(t, codes.NotFound, status.Code(err))
		require.Equal(t, fmt.Sprintf("%s, height %d", clienttypes.ErrConsensusStateNotFound.Error(), sdkCtx.BlockHeight()), status.Convert(err).Message())
	}
	assertMissing()
	consensusState := chain.LastHeader.ConsensusState()
	bz := clienttypes.MustMarshalConsensusState(chain.Codec, consensusState)
	require.NoError(t, chain.GetSimApp().UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight()+1, bz))
	assertMissing()
	require.NoError(t, chain.GetSimApp().UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight(), bz))
	response, err = queryClient.UpgradedConsensusState(ctx, &clienttypes.QueryUpgradedConsensusStateRequest{})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.True(t, clienttypes.MustPackConsensusState(consensusState).Equal(response.UpgradedConsensusState))
	var actual exported.ConsensusState
	require.NoError(t, chain.GetSimApp().InterfaceRegistry().UnpackAny(response.UpgradedConsensusState, &actual))
	require.Equal(t, consensusState, actual)
	chain.GetSimApp().UpgradeKeeper.ClearIBCState(sdkCtx, sdkCtx.BlockHeight())
	assertMissing()
}

func TestRepair_333_UpgradedConsensusStateGateway(t *testing.T) {
	chain, conn := repair333QueryTransport(t)
	marshaler := &gateway.JSONPb{OrigName: true, EmitDefaults: true, AnyResolver: chain.GetSimApp().InterfaceRegistry()}
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaler))
	require.NoError(t, clienttypes.RegisterQueryHandler(context.Background(), mux, conn))
	sdkCtx := chain.GetContext()
	query := func() *httptest.ResponseRecorder {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ibc/core/client/v1/upgraded_consensus_states", nil).WithContext(ctx))
		return recorder
	}
	assertMissing := func() {
		t.Helper()
		recorder := query()
		require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
		var body struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, int(codes.NotFound), body.Code)
		require.Equal(t, fmt.Sprintf("%s, height %d", clienttypes.ErrConsensusStateNotFound.Error(), sdkCtx.BlockHeight()), body.Message)
	}
	assertMissing()
	consensusState := chain.LastHeader.ConsensusState()
	bz := clienttypes.MustMarshalConsensusState(chain.Codec, consensusState)
	require.NoError(t, chain.GetSimApp().UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight()+1, bz))
	assertMissing()
	require.NoError(t, chain.GetSimApp().UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight(), bz))
	recorder := query()
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response clienttypes.QueryUpgradedConsensusStateResponse
	require.NoError(t, marshaler.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, clienttypes.MustPackConsensusState(consensusState).Equal(response.UpgradedConsensusState))
	var actual exported.ConsensusState
	require.NoError(t, chain.GetSimApp().InterfaceRegistry().UnpackAny(response.UpgradedConsensusState, &actual))
	require.Equal(t, consensusState, actual)
	chain.GetSimApp().UpgradeKeeper.ClearIBCState(sdkCtx, sdkCtx.BlockHeight())
	assertMissing()
}
