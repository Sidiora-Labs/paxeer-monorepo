package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func feedQueryContext(t *testing.T) (sdk.Context, sdk.StoreKey, client.Context, <-chan abci.RequestQuery) {
	t.Helper()
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("pricefeed-cli-transient"))
	registry := codectypes.NewInterfaceRegistry()
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterQueryServer(router, keeper.NewQueryServerImpl(keeper.NewKeeper(key, nil, nil, nil)))
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
	clientCtx := client.Context{}.WithClient(node).WithCodec(codec.NewProtoCodec(registry)).WithOutputFormat("json")
	return ctx, key, clientCtx, requests
}

func runFeedQuery(cmd *cobra.Command, clientCtx client.Context, args ...string) (string, error) {
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

func putFeedQueryState(t *testing.T, ctx sdk.Context, storeKey sdk.StoreKey, key []byte, value interface{}) {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	ctx.KVStore(storeKey).Set(key, encoded)
}

func TestFeedKeyCmdParsesValoper(t *testing.T) {
	ctx, storeKey, clientCtx, requests := feedQueryContext(t)
	operator := sdk.ValAddress(bytes.Repeat([]byte{1}, 20))
	account := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	feedKey := types.FeedKey{PubKey: "02d7c823a38b5b96d74bb55eaade0ecfb4ca09fb711c7d90ff06968e44a7f7e2d8c", FeedAccount: account.String()}
	putFeedQueryState(t, ctx, storeKey, types.FeedKeyKey(operator), feedKey)
	putFeedQueryState(t, ctx, storeKey, types.FeedAccountKey(account), operator)

	for _, args := range [][]string{nil, {operator.String(), "extra"}, {account.String()}, {"invalid"}, {""}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			output, err := runFeedQuery(CmdFeedKey(), clientCtx, args...)
			require.Error(t, err)
			require.Empty(t, output)
			require.Empty(t, requests, "invalid arguments must fail before querying")
		})
	}

	output, err := runFeedQuery(CmdFeedKey(), clientCtx, operator.String())
	require.NoError(t, err)
	require.Len(t, requests, 1)
	request := <-requests
	require.Equal(t, "/paxprotocol.paxchain.pricefeed.Query/FeedKey", request.Path)
	var got types.QueryFeedKeyRequest
	require.NoError(t, proto.Unmarshal(request.Data, &got))
	require.Equal(t, types.QueryFeedKeyRequest{Validator: operator.String()}, got)
	var response types.QueryFeedKeyResponse
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), &response))
	require.Equal(t, types.QueryFeedKeyResponse{PubKey: feedKey.PubKey, FeedAccount: account.String()}, response)

	for _, args := range [][]string{nil, {account.String(), "extra"}, {operator.String()}, {"invalid"}, {""}} {
		output, err := runFeedQuery(CmdValidatorByFeedAccount(), clientCtx, args...)
		require.Error(t, err)
		require.Empty(t, output)
		require.Empty(t, requests)
	}
	output, err = runFeedQuery(CmdValidatorByFeedAccount(), clientCtx, account.String())
	require.NoError(t, err)
	require.Len(t, requests, 1)
	request = <-requests
	require.Equal(t, "/paxprotocol.paxchain.pricefeed.Query/ValidatorByFeedAccount", request.Path)
	var reverseRequest types.QueryValidatorByFeedAccountRequest
	require.NoError(t, proto.Unmarshal(request.Data, &reverseRequest))
	require.Equal(t, types.QueryValidatorByFeedAccountRequest{FeedAccount: account.String()}, reverseRequest)
	var reverseResponse types.QueryValidatorByFeedAccountResponse
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), &reverseResponse))
	require.Equal(t, operator.String(), reverseResponse.Validator)
}

func TestObservationCmdParsesArgs(t *testing.T) {
	ctx, storeKey, clientCtx, requests := feedQueryContext(t)
	operator := sdk.ValAddress(bytes.Repeat([]byte{3}, 20))
	account := sdk.AccAddress(operator).String()
	for _, args := range [][]string{
		nil, {"0"}, {"0", operator.String(), "extra"}, {"abc", operator.String()},
		{"-1", operator.String()}, {"18446744073709551616", operator.String()},
		{"1.5", operator.String()}, {"0x10", operator.String()}, {"", operator.String()},
		{"0", account}, {"0", "invalid"}, {"0", ""},
	} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			output, err := runFeedQuery(CmdObservation(), clientCtx, args...)
			require.Error(t, err)
			require.Empty(t, output)
			require.Empty(t, requests, "invalid arguments must fail before querying")
		})
	}

	for i, marketID := range []uint64{0, 42, math.MaxUint64} {
		t.Run(strconv.FormatUint(marketID, 10), func(t *testing.T) {
			record := types.ObservationRecord{
				Sequence: uint64(i + 7), IndexPrice: sdk.NewInt(64000), MarkPrice: sdk.NewInt(64001),
				SourceTimestampMs: 1800000000123, Height: 123,
			}
			putFeedQueryState(t, ctx, storeKey, types.ObservationKey(marketID, operator), record)
			output, err := runFeedQuery(CmdObservation(), clientCtx, strconv.FormatUint(marketID, 10), operator.String())
			require.NoError(t, err)
			require.Len(t, requests, 1)
			request := <-requests
			require.Equal(t, "/paxprotocol.paxchain.pricefeed.Query/Observation", request.Path)
			var got types.QueryObservationRequest
			require.NoError(t, proto.Unmarshal(request.Data, &got))
			require.Equal(t, types.QueryObservationRequest{MarketId: marketID, Validator: operator.String()}, got)
			var response types.QueryObservationResponse
			require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), &response))
			require.Equal(t, types.QueryObservationResponse{
				Sequence: record.Sequence, IndexPrice: record.IndexPrice.String(), MarkPrice: record.MarkPrice.String(),
				SourceTimestampMs: record.SourceTimestampMs, Height: record.Height,
			}, response)
		})
	}

	output, err := runFeedQuery(CmdObservation(), clientCtx, "1", operator.String())
	require.ErrorContains(t, err, "no observation for market 1 and validator "+operator.String())
	require.Empty(t, output)
	require.Len(t, requests, 1)
}
