package pricefeed_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/golang/protobuf/jsonpb"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func RegisterQueryHandlerClient(t *testing.T, mux *runtime.ServeMux, conn *grpc.ClientConn, height int64) {
	t.Helper()
	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, request *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			if request.Path != "/paxprotocol.paxchain.pricefeed.Query/Price" || int64(request.Height) != height || request.Prove {
				return nil, fmt.Errorf("unsupported query: %+v", request)
			}
			var query types.QueryPriceRequest
			if err := query.Unmarshal(request.Data); err != nil {
				return nil, err
			}
			var response types.QueryPriceResponse
			if err := conn.Invoke(ctx, request.Path, &query, &response); err != nil {
				return nil, err
			}
			value, err := response.Marshal()
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: value, Height: height}}, nil
		}),
	})
	rpcHTTP := httptest.NewServer(rpcMux)
	t.Cleanup(rpcHTTP.Close)
	rpcClient, err := rpchttp.NewWithClient(rpcHTTP.URL, rpcHTTP.Client())
	require.NoError(t, err)
	clientCtx := (client.Context{}).WithClient(rpcClient).WithHeight(height)
	(pricefeed.AppModuleBasic{}).RegisterGRPCGatewayRoutes(clientCtx, mux)
}

func TestGatewayServesPricefeedPrice(t *testing.T) {
	_, _, k, _, sdkCtx := newModule(t, 42)
	index := sdk.NewIntWithDecimal(64250, 18)
	mark := sdk.NewIntWithDecimal(64254, 18)
	const timestampMs = uint64(moduleNowMs - 1000)
	genesis := types.DefaultGenesis()
	require.Equal(t, "BTC", genesis.Params.Markets[0].Symbol)
	genesis.Aggregates = []types.GenesisAggregate{{
		MarketID: 0,
		Aggregate: types.Aggregate{
			IndexPrice: index, MarkPrice: mark, Basis: mark.Sub(index),
			TimestampMs: timestampMs, Height: sdkCtx.BlockHeight(), Sources: 3,
		},
	}}
	require.NoError(t, k.InitGenesis(sdkCtx, *genesis))

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(sdkCtx.WithContext(ctx)), req)
	}))
	types.RegisterQueryServer(server, keeper.NewQueryServerImpl(k))
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		require.NoError(t, listener.Close())
		require.NoError(t, <-serveResult)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "passthrough:///pricefeed",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	mux := runtime.NewServeMux()
	RegisterQueryHandlerClient(t, mux, conn, sdkCtx.BlockHeight())
	request := httptest.NewRequest(http.MethodGet, "/paxprotocol/paxchain/pricefeed/price/0", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Header().Get("Content-Type"), "application/json")
	var price types.QueryPriceResponse
	require.NoError(t, jsonpb.Unmarshal(response.Body, &price))
	require.Equal(t, types.QueryPriceResponse{
		Index: "64250000000000000000000", Mark: "64254000000000000000000",
		TimestampMs: timestampMs, Height: 42, Sources: 3, Fresh: true,
	}, price)
}
