package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func denomQueryContext(t *testing.T) (sdk.Context, bankkeeper.BaseKeeper, client.Context, <-chan abci.RequestQuery) {
	t.Helper()
	key := sdk.NewKVStoreKey(banktypes.StoreKey)
	tkey := sdk.NewTransientStoreKey("tokenfactory-cli-transient")
	ctx := testutil.DefaultContext(key, tkey)
	registry := cdctypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	bank := bankkeeper.NewBaseKeeper(cdc, key, nil, paramtypes.NewSubspace(cdc, amino, key, tkey, banktypes.ModuleName), nil)
	tokenfactory := keeper.NewKeeper(cdc, sdk.NewKVStoreKey(types.StoreKey),
		paramtypes.NewSubspace(cdc, amino, key, tkey, types.ModuleName), nil, bank)
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterQueryServer(router, tokenfactory)
	requests := make(chan abci.RequestQuery, 16)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			query := abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove}
			requests <- query
			handler := router.Route(query.Path)
			if handler == nil {
				return nil, fmt.Errorf("unknown query route %s", query.Path)
			}
			response, err := handler(ctx, query)
			return &coretypes.ResultABCIQuery{Response: response}, err
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	node, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)
	clientCtx := client.Context{}.WithClient(node).WithCodec(cdc).WithOutputFormat("json")
	return ctx, bank, clientCtx, requests
}

func runDenomQuery(cmd *cobra.Command, clientCtx client.Context, args ...string) (string, error) {
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func assertDenomQueryArgs(t *testing.T, name string, standalone *cobra.Command, clientCtx client.Context, requests <-chan abci.RequestQuery) {
	t.Helper()
	root := GetQueryCmd()
	registered, remaining, err := root.Find([]string{name})
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.NotSame(t, root, registered)
	require.Same(t, root, registered.Parent())
	for _, cmd := range []*cobra.Command{standalone, registered} {
		require.Equal(t, name+" [denom]", cmd.Use)
		require.NotNil(t, cmd.RunE)
		require.NotNil(t, cmd.Args)
		for _, flag := range []string{flags.FlagNode, flags.FlagHeight, flags.FlagOutput} {
			require.NotNil(t, cmd.Flags().Lookup(flag), "missing query flag %s", flag)
		}
		require.EqualError(t, cmd.ValidateArgs(nil), "accepts 1 arg(s), received 0")
		require.NoError(t, cmd.ValidateArgs([]string{"factory/creator/token"}))
		require.EqualError(t, cmd.ValidateArgs([]string{"factory/creator/token", "extra"}), "accepts 1 arg(s), received 2")
	}
	for _, tc := range []struct {
		args []string
		err  string
	}{
		{args: []string{name}, err: "accepts 1 arg(s), received 0"},
		{args: []string{name, "factory/creator/token", "extra"}, err: "accepts 1 arg(s), received 2"},
	} {
		output, err := runDenomQuery(GetQueryCmd(), clientCtx, tc.args...)
		require.EqualError(t, err, tc.err)
		require.Empty(t, output)
		require.Empty(t, requests, "invalid argument counts must fail before querying")
	}
}

func TestGetCmdDenomMetadataArgs(t *testing.T) {
	ctx, bank, clientCtx, requests := denomQueryContext(t)
	assertDenomQueryArgs(t, "denom-metadata", GetCmdDenomMetadata(), clientCtx, requests)
	denom := "factory/creator/token"
	metadata := banktypes.Metadata{
		Base: denom, Display: "TOKEN", Name: "Test Token", Symbol: "TOKEN",
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: denom, Exponent: 0, Aliases: []string{"utoken"}},
			{Denom: "TOKEN", Exponent: 6, Aliases: []string{"token"}},
		},
	}
	bank.SetDenomMetaData(ctx, metadata)
	output, err := runDenomQuery(GetQueryCmd(), clientCtx, "denom-metadata", denom, "--height", "42")
	require.NoError(t, err)
	require.Len(t, requests, 1)
	request := <-requests
	require.Equal(t, "/paxprotocol.paxchain.tokenfactory.Query/DenomMetadata", request.Path)
	require.Equal(t, int64(42), request.Height)
	var got types.QueryDenomMetadataRequest
	require.NoError(t, proto.Unmarshal(request.Data, &got))
	require.Equal(t, types.QueryDenomMetadataRequest{Denom: denom}, got)
	var response types.QueryDenomMetadataResponse
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), &response))
	require.Equal(t, metadata, response.Metadata)

	output, err = runDenomQuery(GetQueryCmd(), clientCtx, "denom-metadata", denom+"-missing")
	require.ErrorContains(t, err, "client metadata for denom "+denom+"-missing")
	require.Empty(t, output)
	require.Len(t, requests, 1)
}

func TestGetCmdDenomAllowListArgs(t *testing.T) {
	ctx, bank, clientCtx, requests := denomQueryContext(t)
	assertDenomQueryArgs(t, "denom-allow-list", GetCmdDenomAllowList(), clientCtx, requests)
	denom := "factory/creator/token"
	allowList := banktypes.AllowList{Addresses: []string{sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()}}
	bank.SetDenomAllowList(ctx, denom, allowList)
	output, err := runDenomQuery(GetQueryCmd(), clientCtx, "denom-allow-list", denom, "--height", "43")
	require.NoError(t, err)
	require.Len(t, requests, 1)
	request := <-requests
	require.Equal(t, "/paxprotocol.paxchain.tokenfactory.Query/DenomAllowList", request.Path)
	require.Equal(t, int64(43), request.Height)
	var got types.QueryDenomAllowListRequest
	require.NoError(t, proto.Unmarshal(request.Data, &got))
	require.Equal(t, types.QueryDenomAllowListRequest{Denom: denom}, got)
	var response types.QueryDenomAllowListResponse
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), &response))
	require.Equal(t, allowList, response.AllowList)

	output, err = runDenomQuery(GetQueryCmd(), clientCtx, "denom-allow-list", "")
	require.ErrorContains(t, err, "invalid denom")
	require.Empty(t, output)
	require.Len(t, requests, 1)
}
