package keeper_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory"
	tokenfactorywasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
)

func TestDenomPausedSurfaceConformance(t *testing.T) {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	k := s.App.TokenFactoryKeeper
	cases := []struct {
		name   string
		paused bool
		issuer string
		denom  string
	}{
		{name: "paused", paused: true, issuer: adminsIssuerDid(0xab)},
		{name: "unpaused", paused: false, issuer: adminsIssuerDid(0xcd)},
	}
	for i := range cases {
		tc := &cases[i]
		created, err := s.msgServer.CreateDenom(sdk.WrapSDKContext(s.Ctx),
			types.NewMsgCreateDenom(s.TestAccs[0].String(), tc.name))
		require.NoError(t, err)
		require.NotNil(t, created)
		tc.denom = created.GetNewTokenDenom()
		require.NotEmpty(t, tc.denom)
		require.NoError(t, k.SetIssuerDid(s.Ctx, tc.denom, tc.issuer))
		if tc.paused {
			require.NoError(t, k.SetDenomPausedByIssuer(s.Ctx, tc.denom, tc.issuer, true))
		}
	}

	_, err := s.App.Commit(context.Background())
	require.NoError(t, err)
	queryCtx, err := s.App.CreateQueryContext(0, false)
	require.NoError(t, err)
	queryClient := types.NewQueryClient(&baseapp.QueryServiceTestHelper{
		GRPCQueryRouter: s.App.GRPCQueryRouter(),
		Ctx:             queryCtx,
	})
	wasmHandler := tokenfactorywasm.NewTokenFactoryWasmQueryHandler(&k)

	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			response, err := s.App.Query(ctx, &abci.RequestQuery{
				Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove,
			})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: *response}, nil
		}),
	})
	rpcServer := httptest.NewServer(rpcMux)
	t.Cleanup(rpcServer.Close)
	rpcClient, err := rpchttp.NewWithTimeout(rpcServer.URL, 5*time.Second)
	require.NoError(t, err)
	gatewayMux := runtime.NewServeMux()
	tokenfactory.AppModuleBasic{}.RegisterGRPCGatewayRoutes(client.Context{}.WithClient(rpcClient), gatewayMux)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paused := k.IsDenomPaused(queryCtx, tc.denom)
			issuer, err := k.IssuerDid(queryCtx, tc.denom)
			require.NoError(t, err)
			require.Equal(t, tc.paused, paused)
			require.Equal(t, tc.issuer, issuer)
			expected := &types.QueryDenomPausedResponse{Paused: paused, IssuerDid: issuer}
			request := &types.QueryDenomPausedRequest{Denom: tc.denom}

			grpcResponse, err := queryClient.DenomPaused(sdk.WrapSDKContext(queryCtx), request)
			require.NoError(t, err)
			require.Equal(t, expected, grpcResponse, "app gRPC query router")

			recorder := httptest.NewRecorder()
			gatewayMux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
				"/pax-protocol/paxchain/tokenfactory/denom_paused?denom="+url.QueryEscape(tc.denom), nil))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			var gatewayResponse types.QueryDenomPausedResponse
			require.NoError(t, (&runtime.JSONPb{}).Unmarshal(recorder.Body.Bytes(), &gatewayResponse))
			require.Equal(t, expected, &gatewayResponse, "registered REST gateway")

			wasmResponse, err := wasmHandler.GetDenomPaused(queryCtx, request)
			require.NoError(t, err)
			require.Equal(t, expected, wasmResponse, "wasm query handler")
		})
	}
}
