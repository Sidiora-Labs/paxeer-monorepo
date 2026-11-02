package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestQueryCmdHasPricefeedSubcommands(t *testing.T) {
	for name, root := range map[string]func() *cobra.Command{
		"cli":    cli.GetQueryCmd,
		"module": pricefeed.AppModuleBasic{}.GetQueryCmd,
	} {
		t.Run(name, func(t *testing.T) {
			cmd := root()
			require.NotNil(t, cmd)
			require.Equal(t, "pricefeed", cmd.Name())
			var names []string
			for _, subcommand := range cmd.Commands() {
				names = append(names, subcommand.Name())
				require.NotNil(t, subcommand.RunE)
				require.NotNil(t, subcommand.Flags().Lookup("output"))
			}
			require.ElementsMatch(t, []string{"params", "price", "prices", "feed-key", "validator-by-feed-account", "observation"}, names)
		})
	}
	require.Nil(t, pricefeed.AppModuleBasic{}.GetTxCmd())
}

func TestPriceCmdParsesMarketId(t *testing.T) {
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("pricefeed-query-transient"))
	ctx = ctx.WithBlockTime(time.UnixMilli(100000)).WithBlockHeight(10)
	k := keeper.NewKeeper(key, nil, nil, nil)
	params := types.DefaultParams(types.DefaultAuthority())
	params.StalenessWindowMs = 12000
	require.NoError(t, k.SetParams(ctx, params))
	aggregate := types.Aggregate{
		IndexPrice: sdk.NewInt(107), MarkPrice: sdk.NewInt(207), Basis: sdk.ZeroInt(),
		TimestampMs: 100000, Height: 10, Sources: 3,
	}
	data, err := json.Marshal(aggregate)
	require.NoError(t, err)
	ctx.KVStore(key).Set(types.AggregateKey(7), data)
	registry := cdctypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterQueryServer(router, keeper.NewQueryServerImpl(k))
	requests := make(chan abci.RequestQuery, 16)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			query := abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove}
			requests <- query
			handler := router.Route(query.Path)
			if handler == nil {
				return nil, fmt.Errorf("unknown query route %q", query.Path)
			}
			response, err := handler(ctx, query)
			return &coretypes.ResultABCIQuery{Response: response}, err
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)
	execute := func(command string, args ...string) (string, error) {
		var output bytes.Buffer
		clientCtx := client.Context{}.WithClient(rpc).WithCodec(cdc).WithLegacyAmino(amino).WithOutput(&output)
		cmd := pricefeed.AppModuleBasic{}.GetQueryCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs(append([]string{command, "--output", "json", "--"}, args...))
		err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
		return output.String(), err
	}

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing", nil, "accepts 1 arg(s), received 0"},
		{"extra", []string{"7", "8"}, "accepts 1 arg(s), received 2"},
		{"non-numeric", []string{"abc"}, "invalid syntax"},
		{"negative", []string{"-1"}, "invalid syntax"},
		{"empty", []string{""}, "invalid syntax"},
		{"fraction", []string{"7.5"}, "invalid syntax"},
		{"hex", []string{"0x7"}, "invalid syntax"},
		{"overflow", []string{"18446744073709551616"}, "value out of range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := execute("price", tc.args...)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, output)
			require.Empty(t, requests, "invalid input must fail before any query")
		})
	}

	output, err := execute("price", "7")
	require.NoError(t, err)
	require.Len(t, requests, 1)
	request := <-requests
	require.Equal(t, "/paxprotocol.paxchain.pricefeed.Query/Price", request.Path)
	var priceRequest types.QueryPriceRequest
	require.NoError(t, cdc.Unmarshal(request.Data, &priceRequest))
	require.Equal(t, types.QueryPriceRequest{MarketId: 7}, priceRequest)
	var response types.QueryPriceResponse
	require.NoError(t, cdc.UnmarshalAsJSON([]byte(output), &response))
	require.Equal(t, types.QueryPriceResponse{
		Index: "107", Mark: "207", TimestampMs: 100000, Height: 10, Sources: 3, Fresh: true,
	}, response)

	t.Run("full uint64 range and query error", func(t *testing.T) {
		output, err := execute("price", "18446744073709551615")
		require.ErrorContains(t, err, "market id 18446744073709551615")
		require.Empty(t, output)
		require.Len(t, requests, 1)
		request := <-requests
		require.Equal(t, "/paxprotocol.paxchain.pricefeed.Query/Price", request.Path)
		var got types.QueryPriceRequest
		require.NoError(t, cdc.Unmarshal(request.Data, &got))
		require.Equal(t, types.QueryPriceRequest{MarketId: ^uint64(0)}, got)
	})

	t.Run("params", func(t *testing.T) {
		output, err := execute("params", "extra")
		require.ErrorContains(t, err, "unknown command")
		require.Empty(t, output)
		require.Empty(t, requests)
		output, err = execute("params")
		require.NoError(t, err)
		require.Len(t, requests, 1)
		request := <-requests
		require.Equal(t, "/paxprotocol.paxchain.pricefeed.Query/Params", request.Path)
		var got types.QueryParamsRequest
		require.NoError(t, cdc.Unmarshal(request.Data, &got))
		var response types.QueryParamsResponse
		require.NoError(t, amino.UnmarshalAsJSON([]byte(output), &response))
		require.Equal(t, params.Authority, response.Authority)
		require.Equal(t, params.Markets, response.Markets)
		require.Equal(t, params.StalenessWindowMs, response.StalenessWindowMs)
	})
}
