package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	clientcli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/client/cli"
	clientkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/keeper"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
)

func repairM1215Execute(ctx client.Context) ([]byte, []byte, error) {
	var output, diagnostics bytes.Buffer
	ctx = ctx.WithOutput(&output)
	cmd := clientcli.GetCmdParams()
	cmd.SetArgs([]string{})
	cmd.SetOut(&diagnostics)
	cmd.SetErr(&diagnostics)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx))
	return output.Bytes(), diagnostics.Bytes(), err
}

func TestRepairM_1215_ParamsQueryUnavailable(t *testing.T) {
	server := httptest.NewServer(http.NewServeMux())
	server.Close()
	rpc, err := rpchttp.NewWithTimeout(server.URL, 2*time.Second)
	require.NoError(t, err)
	ctx := client.Context{}.WithClient(rpc).WithOutputFormat("json")
	response, originalErr := clienttypes.NewQueryClient(ctx).ClientParams(t.Context(), &clienttypes.QueryClientParamsRequest{})
	require.Nil(t, response)
	require.Error(t, originalErr)

	output, diagnostics, err := repairM1215Execute(ctx)
	require.EqualError(t, err, originalErr.Error())
	var transportErr *url.Error
	require.ErrorAs(t, err, &transportErr)
	var dialErr *net.OpError
	require.ErrorAs(t, err, &dialErr)
	require.Equal(t, "dial", dialErr.Op)
	require.Empty(t, output)
	require.Empty(t, diagnostics)
}

func TestRepairM_1215_ParamsQueryConcreteResponse(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	clienttypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	key := sdk.NewKVStoreKey("repair_m1215")
	tkey := sdk.NewTransientStoreKey("repair_m1215_transient")
	sdkCtx := testutil.DefaultContext(key, tkey).WithBlockHeight(17)
	space := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), key, tkey, "client")
	keeper := clientkeeper.NewKeeper(cdc, key, space, nil, nil)
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(registry)
	clienttypes.RegisterQueryServer(router, keeper)
	requests := make(chan coretypes.RequestABCIQuery, 1)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			handler := router.Route(req.Path)
			if handler == nil {
				return nil, fmt.Errorf("unregistered query path: %s", req.Path)
			}
			requests <- *req
			response, err := handler(sdkCtx, abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: response}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.NewWithTimeout(server.URL, 2*time.Second)
	require.NoError(t, err)
	ctx := client.Context{}.WithClient(rpc).WithCodec(cdc).WithInterfaceRegistry(registry).
		WithOutputFormat("json").WithHeight(17)
	for _, params := range []clienttypes.Params{clienttypes.DefaultParams(), clienttypes.NewParams("07-tendermint")} {
		keeper.SetParams(sdkCtx, params)
		require.Equal(t, params, keeper.GetParams(sdkCtx))
		output, diagnostics, err := repairM1215Execute(ctx)
		require.NoError(t, err)
		require.Empty(t, diagnostics)
		var printed clienttypes.Params
		require.NoError(t, cdc.UnmarshalAsJSON(output, &printed))
		require.Equal(t, params, printed)
		expected, err := cdc.MarshalAsJSON(&params)
		require.NoError(t, err)
		require.JSONEq(t, string(expected), string(output))
		require.Len(t, requests, 1)
		request := <-requests
		require.Equal(t, "/ibc.core.client.v1.Query/ClientParams", request.Path)
		require.Equal(t, coretypes.Int64(17), request.Height)
		require.False(t, request.Prove)
		var decoded clienttypes.QueryClientParamsRequest
		require.NoError(t, cdc.Unmarshal(request.Data, &decoded))
	}
}
