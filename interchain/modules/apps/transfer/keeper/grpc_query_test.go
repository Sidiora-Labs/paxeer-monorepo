package keeper_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func (suite *KeeperTestSuite) TestQueryDenomTrace() {
	repairM1205QueryTransport(suite.T())
}

func (suite *KeeperTestSuite) TestQueryDenomTraces() {
	repairM1205QueryTransport(suite.T())
}

func (suite *KeeperTestSuite) TestQueryDenomHash() {
	repairM1205QueryTransport(suite.T())
}

func repairM1205QueryTransport(t *testing.T) {
	t.Helper()
	s := repairM1205Suite(t)
	empty, err := s.queryClient.DenomTraces(context.Background(), &types.QueryDenomTracesRequest{})
	require.NoError(t, err)
	require.Empty(t, empty.DenomTraces)
	traces := types.Traces{
		repairM1205Produce(t, s, sdk.DefaultBondDenom),
		repairM1205Produce(t, s, "factory/creator/sub/denom"),
	}
	chain := s.chainC
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(context.WithValue(ctx, sdk.SdkContextKey, chain.GetContext()), req)
	}))
	types.RegisterQueryServer(server, chain.GetSimApp().TransferKeeper)
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); require.NoError(t, <-serveResult) })
	dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := types.NewQueryClient(conn)
	mux := runtime.NewServeMux()
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), mux, client))
	rest := httptest.NewServer(mux)
	rest.Client().Timeout = 10 * time.Second
	t.Cleanup(rest.Close)
	get := func(path string, wantStatus int) []byte {
		t.Helper()
		response, err := rest.Client().Get(rest.URL + path)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, wantStatus, response.StatusCode, string(body))
		return body
	}
	for _, trace := range traces {
		for _, hash := range []string{trace.Hash().String(), trace.IBCDenom()} {
			response, err := client.DenomTrace(context.Background(), &types.QueryDenomTraceRequest{Hash: hash})
			require.NoError(t, err)
			require.Equal(t, &trace, response.DenomTrace)
		}
		hash, err := client.DenomHash(context.Background(), &types.QueryDenomHashRequest{Trace: trace.GetFullDenomPath()})
		require.NoError(t, err)
		require.Equal(t, trace.Hash().String(), hash.Hash)
		var response types.QueryDenomTraceResponse
		chain.GetSimApp().AppCodec().MustUnmarshalJSON(get("/ibc/apps/transfer/v1/denom_traces/"+trace.Hash().String(), http.StatusOK), &response)
		require.Equal(t, &trace, response.DenomTrace)
		for _, tracePath := range []string{trace.GetFullDenomPath(), url.PathEscape(trace.GetFullDenomPath())} {
			var hashResponse types.QueryDenomHashResponse
			chain.GetSimApp().AppCodec().MustUnmarshalJSON(get("/ibc/apps/transfer/v1/denom_hashes/"+tracePath, http.StatusOK), &hashResponse)
			require.Equal(t, hash.Hash, hashResponse.Hash)
		}
	}
	for _, request := range []*types.QueryDenomTracesRequest{
		{}, {Pagination: &query.PageRequest{Limit: 5, CountTotal: false}},
	} {
		response, err := client.DenomTraces(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, traces.Sort(), response.DenomTraces)
	}
	var listed types.QueryDenomTracesResponse
	chain.GetSimApp().AppCodec().MustUnmarshalJSON(get("/ibc/apps/transfer/v1/denom_traces?pagination.limit=5", http.StatusOK), &listed)
	require.Equal(t, traces.Sort(), listed.DenomTraces)
	for _, invalid := range []string{"!@#!@#!", "xyz"} {
		_, err := client.DenomTrace(context.Background(), &types.QueryDenomTraceRequest{Hash: invalid})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	missing := types.ParseDenomTrace("transfer/channel-999/uatom")
	_, err = client.DenomTrace(context.Background(), &types.QueryDenomTraceRequest{Hash: missing.IBCDenom()})
	require.Equal(t, codes.NotFound, status.Code(err))
	get("/ibc/apps/transfer/v1/denom_traces/"+missing.Hash().String(), http.StatusNotFound)
	get("/ibc/apps/transfer/v1/denom_traces/xyz", http.StatusBadRequest)
	for _, invalid := range []string{"transfer/channelToA/transfer/", "transfer/channel-999/uatom"} {
		_, err := client.DenomHash(context.Background(), &types.QueryDenomHashRequest{Trace: invalid})
		require.Error(t, err)
	}
	for _, refusal := range []struct {
		trace      string
		grpcCode   codes.Code
		httpStatus int
	}{
		{"transfer?/channel-0/uatom", codes.InvalidArgument, http.StatusBadRequest},
		{"transfer/channel-999/uatom", codes.NotFound, http.StatusNotFound},
	} {
		_, err := client.DenomHash(context.Background(), &types.QueryDenomHashRequest{Trace: refusal.trace})
		require.Equal(t, refusal.grpcCode, status.Code(err))
		get("/ibc/apps/transfer/v1/denom_hashes/"+url.PathEscape(refusal.trace), refusal.httpStatus)
	}
}

func (suite *KeeperTestSuite) TestQueryParams() {
	ctx := sdk.WrapSDKContext(suite.chainA.GetContext())
	expParams := types.DefaultParams()
	res, _ := suite.queryClient.Params(ctx, &types.QueryParamsRequest{})
	suite.Require().Equal(&expParams, res.Params)
}

func (suite *KeeperTestSuite) TestEscrowAddress() {
	var req *types.QueryEscrowAddressRequest

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"success",
			func() {
				req = &types.QueryEscrowAddressRequest{
					PortId:    ibctesting.TransferPort,
					ChannelId: ibctesting.FirstChannelID,
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()
			ctx := sdk.WrapSDKContext(suite.chainA.GetContext())

			res, err := suite.queryClient.EscrowAddress(ctx, req)

			if tc.expPass {
				suite.Require().NoError(err)
				expected := types.GetEscrowAddress(ibctesting.TransferPort, ibctesting.FirstChannelID).String()
				suite.Require().Equal(expected, res.EscrowAddress)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
