package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
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

func TestPricesCmdParsesIdList(t *testing.T) {
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_pricefeed"))
	ctx = ctx.WithBlockTime(time.UnixMilli(100000)).WithBlockHeight(10)
	k := keeper.NewKeeper(key, nil, nil, nil)
	for _, id := range []uint64{0, 7} {
		aggregate := types.Aggregate{
			IndexPrice: sdk.NewInt(int64(100 + id)),
			MarkPrice:  sdk.NewInt(int64(200 + id)),
			Basis:      sdk.ZeroInt(), TimestampMs: 100000, Height: 10, Sources: 3,
		}
		data, err := json.Marshal(aggregate)
		require.NoError(t, err)
		ctx.KVStore(key).Set(types.AggregateKey(id), data)
	}
	registry := cdctypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterQueryServer(router, keeper.NewQueryServerImpl(k))
	var calls atomic.Uint64
	requests := make(chan types.QueryPricesRequest, 1)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, request *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			calls.Add(1)
			if request.Path != "/paxprotocol.paxchain.pricefeed.Query/Prices" {
				return nil, fmt.Errorf("unexpected query path %q", request.Path)
			}
			var pricesRequest types.QueryPricesRequest
			if err := cdc.Unmarshal(request.Data, &pricesRequest); err != nil {
				return nil, err
			}
			requests <- pricesRequest
			response, err := router.Route(request.Path)(ctx, abci.RequestQuery{
				Path: request.Path, Data: request.Data, Height: int64(request.Height), Prove: request.Prove,
			})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: response}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)

	execute := func(args []string) (string, error) {
		t.Helper()
		var output bytes.Buffer
		clientCtx := client.Context{}.WithClient(rpc).WithCodec(cdc).WithOutput(&output)
		cmd := cli.CmdPrices()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"--output", "json", "--"}, args...))
		err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
		return output.String(), err
	}
	boundary := make([]uint64, 64)
	for i := range boundary {
		boundary[i] = uint64(i%2) * 7
	}
	for _, tc := range []struct {
		name string
		ids  []uint64
	}{
		{"single", []uint64{7}},
		{"order and duplicates", []uint64{7, 0, 7}},
		{"64 ids", boundary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := make([]string, len(tc.ids))
			for i, id := range tc.ids {
				ids[i] = strconv.FormatUint(id, 10)
			}
			before := calls.Load()
			output, err := execute([]string{strings.Join(ids, ",")})
			require.NoError(t, err)
			require.Equal(t, before+1, calls.Load())
			require.Equal(t, types.QueryPricesRequest{MarketIds: tc.ids}, <-requests)
			var response types.QueryPricesResponse
			require.NoError(t, cdc.UnmarshalAsJSON([]byte(output), &response))
			require.Len(t, response.Prices, len(tc.ids))
			for i, id := range tc.ids {
				require.Equal(t, types.QueryPriceResponse{
					Index: strconv.FormatUint(100+id, 10), Mark: strconv.FormatUint(200+id, 10),
					TimestampMs: 100000, Height: 10, Sources: 3, Fresh: true,
				}, response.Prices[i])
			}
		})
	}

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing argument", nil, "accepts 1 arg(s), received 0"},
		{"extra argument", []string{"0", "7"}, "accepts 1 arg(s), received 2"},
		{"empty list", []string{""}, "invalid syntax"},
		{"empty element", []string{"0,,7"}, "invalid syntax"},
		{"trailing comma", []string{"0,"}, "invalid syntax"},
		{"leading comma", []string{",7"}, "invalid syntax"},
		{"negative", []string{"-1"}, "invalid syntax"},
		{"invalid last id", []string{"0,7,no"}, "invalid syntax"},
		{"fraction", []string{"0,1.5"}, "invalid syntax"},
		{"hex", []string{"0x7"}, "invalid syntax"},
		{"overflow", []string{"18446744073709551616"}, "value out of range"},
		{"65 ids", []string{strings.Repeat("0,", 64) + "0"}, "at most 64"},
		{"65 invalid ids", []string{strings.Repeat("no,", 64) + "no"}, "at most 64"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := calls.Load()
			output, err := execute(tc.args)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, output)
			require.Equal(t, before, calls.Load(), "invalid input must not query")
		})
	}

	t.Run("full uint64 range and query error", func(t *testing.T) {
		before := calls.Load()
		output, err := execute([]string{"18446744073709551615"})
		require.ErrorContains(t, err, "market id 18446744073709551615")
		require.Empty(t, output)
		require.Equal(t, before+1, calls.Load())
		require.Equal(t, types.QueryPricesRequest{MarketIds: []uint64{^uint64(0)}}, <-requests)
	})
}
