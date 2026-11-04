package keeper_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func repairM1223Transport(t *testing.T) (*ibctesting.TestChain, *grpc.ClientConn) {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 1)
	chain := coordinator.GetChain(ibctesting.GetChainID(1))
	app := chain.GetSimApp()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	coordinator.CommitBlock(chain)
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(chain.GetContext().WithContext(ctx)), req)
	}))
	msgServer := grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	ibc.NewAppModule(app.IBCKeeper).RegisterServices(module.NewConfigurator(app.AppCodec(), msgServer, server))
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		require.NoError(t, listener.Close())
		require.NoError(t, <-served)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "passthrough:///repair-m1223", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return chain, conn
}

func repairM1223AssertConsensus(t *testing.T, chain *ibctesting.TestChain, response *clienttypes.QueryUpgradedConsensusStateResponse) {
	t.Helper()
	require.NotNil(t, response)
	require.NotNil(t, response.UpgradedConsensusState)
	expected := chain.LastHeader.ConsensusState()
	require.True(t, clienttypes.MustPackConsensusState(expected).Equal(response.UpgradedConsensusState))
	var actual exported.ConsensusState
	require.NoError(t, chain.GetSimApp().InterfaceRegistry().UnpackAny(response.UpgradedConsensusState, &actual))
	require.Equal(t, expected, actual)
}

func TestRepairM_1223_AppModuleService(t *testing.T) {
	chain, conn := repairM1223Transport(t)
	app := chain.GetSimApp()
	sdkCtx := chain.GetContext()
	client := clienttypes.NewQueryClient(conn)
	state := chain.LastHeader.ConsensusState()
	require.NoError(t, state.ValidateBasic())
	encoded := clienttypes.MustMarshalConsensusState(chain.Codec, state)
	missingMessage := fmt.Sprintf("%s, height %d", clienttypes.ErrConsensusStateNotFound.Error(), sdkCtx.BlockHeight())
	query := func(t *testing.T, code codes.Code) *clienttypes.QueryUpgradedConsensusStateResponse {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		response, err := client.UpgradedConsensusState(ctx, &clienttypes.QueryUpgradedConsensusStateRequest{})
		require.Equal(t, code, status.Code(err))
		if code == codes.OK {
			require.NoError(t, err)
			repairM1223AssertConsensus(t, chain, response)
		} else {
			require.Nil(t, response)
			if code == codes.NotFound {
				require.Equal(t, missingMessage, status.Convert(err).Message())
			} else {
				require.NotEmpty(t, status.Convert(err).Message())
			}
		}
		return response
	}
	t.Run("nil_request", func(t *testing.T) {
		response, err := app.IBCKeeper.UpgradedConsensusState(sdk.WrapSDKContext(sdkCtx), nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, "empty request", status.Convert(err).Message())
	})
	t.Run("absent", func(t *testing.T) { query(t, codes.NotFound) })
	t.Run("other_height_is_absent", func(t *testing.T) {
		require.NoError(t, app.UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight()+1, encoded))
		query(t, codes.NotFound)
	})
	t.Run("stored", func(t *testing.T) {
		require.NoError(t, app.UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight(), encoded))
		query(t, codes.OK)
	})
	t.Run("removed", func(t *testing.T) {
		app.UpgradeKeeper.ClearIBCState(sdkCtx, sdkCtx.BlockHeight())
		query(t, codes.NotFound)
	})
	t.Run("invalid_encoding", func(t *testing.T) {
		require.NoError(t, app.UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight(), []byte{0xff}))
		query(t, codes.Internal)
	})
}

func TestRepairM_1223_AppModuleGateway(t *testing.T) {
	chain, conn := repairM1223Transport(t)
	app := chain.GetSimApp()
	sdkCtx := chain.GetContext()
	marshaler := &gateway.JSONPb{OrigName: true, EmitDefaults: true, AnyResolver: app.InterfaceRegistry()}
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaler))
	require.NoError(t, clienttypes.RegisterQueryHandler(context.Background(), mux, conn))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	state := chain.LastHeader.ConsensusState()
	require.NoError(t, state.ValidateBasic())
	encoded := clienttypes.MustMarshalConsensusState(chain.Codec, state)
	missingMessage := fmt.Sprintf("%s, height %d", clienttypes.ErrConsensusStateNotFound.Error(), sdkCtx.BlockHeight())
	query := func(t *testing.T, httpCode int, code codes.Code) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/ibc/core/client/v1/upgraded_consensus_states", nil)
		require.NoError(t, err)
		response, err := server.Client().Do(req)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, httpCode, response.StatusCode, string(body))
		if code == codes.OK {
			var decoded clienttypes.QueryUpgradedConsensusStateResponse
			require.NoError(t, marshaler.Unmarshal(body, &decoded))
			repairM1223AssertConsensus(t, chain, &decoded)
		} else {
			var decoded struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(body, &decoded))
			require.Equal(t, int(code), decoded.Code)
			if code == codes.NotFound {
				require.Equal(t, missingMessage, decoded.Message)
			} else {
				require.NotEmpty(t, decoded.Message)
			}
		}
	}
	t.Run("absent", func(t *testing.T) { query(t, http.StatusNotFound, codes.NotFound) })
	t.Run("other_height_is_absent", func(t *testing.T) {
		require.NoError(t, app.UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight()+1, encoded))
		query(t, http.StatusNotFound, codes.NotFound)
	})
	t.Run("stored", func(t *testing.T) {
		require.NoError(t, app.UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight(), encoded))
		query(t, http.StatusOK, codes.OK)
	})
	t.Run("removed", func(t *testing.T) {
		app.UpgradeKeeper.ClearIBCState(sdkCtx, sdkCtx.BlockHeight())
		query(t, http.StatusNotFound, codes.NotFound)
	})
	t.Run("invalid_encoding", func(t *testing.T) {
		require.NoError(t, app.UpgradeKeeper.SetUpgradedConsensusState(sdkCtx, sdkCtx.BlockHeight(), []byte{0xff}))
		query(t, http.StatusInternalServerError, codes.Internal)
	})
}
