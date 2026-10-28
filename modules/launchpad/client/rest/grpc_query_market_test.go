package rest_test

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRESTMarketRoutes(t *testing.T) {
	cfg := network.DefaultConfig()
	cfg.NumValidators = 1
	cfg.SigningAlgo = string(hd.Secp256k1Type)
	cfg.AppConstructor = func(val sdknetwork.Validator) servertypes.Application {
		opts := viper.New()
		sc, ss := val.AppConfig.StateCommit, val.AppConfig.StateStore
		for name, value := range map[string]interface{}{
			"chain-id":                          cfg.ChainID,
			"evm.http_enabled":                  false,
			"evm.ws_enabled":                    false,
			app.FlagSCEnable:                    sc.Enable,
			app.FlagSCAsyncCommitBuffer:         sc.MemIAVLConfig.AsyncCommitBuffer,
			app.FlagSCSnapshotKeepRecent:        sc.MemIAVLConfig.SnapshotKeepRecent,
			app.FlagSCSnapshotInterval:          sc.MemIAVLConfig.SnapshotInterval,
			app.FlagSCSnapshotMinTimeInterval:   sc.MemIAVLConfig.SnapshotMinTimeInterval,
			app.FlagSCSnapshotWriterLimit:       sc.MemIAVLConfig.SnapshotWriterLimit,
			app.FlagSCSnapshotPrefetchThreshold: sc.MemIAVLConfig.SnapshotPrefetchThreshold,
			app.FlagSCSnapshotWriteRateMBps:     sc.MemIAVLConfig.SnapshotWriteRateMBps,
			app.FlagSSEnable:                    ss.Enable,
			app.FlagSSBackend:                   ss.Backend,
			app.FlagSSAsyncWriterBuffer:         ss.AsyncWriteBuffer,
			app.FlagSSKeepRecent:                ss.KeepRecent,
			app.FlagSSPruneInterval:             ss.PruneIntervalSeconds,
			app.FlagSSImportNumWorkers:          ss.ImportNumWorkers,
		} {
			opts.Set(name, value)
		}
		encoding := params.EncodingConfig{
			Marshaler: cfg.Codec, Amino: cfg.LegacyAmino,
			TxConfig: cfg.TxConfig, InterfaceRegistry: cfg.InterfaceRegistry,
		}
		return app.New(tmdb.NewMemDB(), nil, true, map[int64]bool{}, val.Ctx.Config.RootDir,
			0, false, val.Ctx.Config, encoding, wasm.EnableAllProposals, opts, nil, app.EmptyAppOptions,
			baseapp.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
			baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices))
	}
	var genesis types.GenesisState
	require.NoError(t, json.Unmarshal(cfg.GenesisState[types.ModuleName], &genesis))
	require.Empty(t, genesis.Markets)

	net := sdknetwork.New(t, cfg)
	t.Cleanup(net.Cleanup)
	require.Len(t, net.Validators, 1)
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	val := net.Validators[0]
	val.ClientCtx = val.ClientCtx.WithHeight(height)
	query := types.NewQueryClient(val.ClientCtx)
	headers := map[string]string{
		"Accept":                        "application/json",
		grpctypes.GRPCBlockHeightHeader: strconv.FormatInt(height, 10),
	}
	baseURL := val.APIAddress + "/pax-protocol/paxchain/launchpad/"
	ctx := context.Background()
	compare := func(t *testing.T, body []byte, got, want proto.Message) {
		t.Helper()
		require.NoError(t, val.ClientCtx.Codec.UnmarshalAsJSON(body, got), "REST body: %s", body)
		wantProto, err := proto.Marshal(want)
		require.NoError(t, err)
		gotProto, err := proto.Marshal(got)
		require.NoError(t, err)
		require.Equal(t, wantProto, gotProto)
		wantJSON, err := val.ClientCtx.Codec.MarshalAsJSON(want)
		require.NoError(t, err)
		require.JSONEq(t, string(wantJSON), string(body))
	}

	t.Run("params", func(t *testing.T) {
		body, err := testutil.GetRequestWithHeaders(baseURL+"params", headers)
		require.NoError(t, err)
		want, err := query.Params(ctx, &types.QueryParamsRequest{})
		require.NoError(t, err)
		var got types.QueryParamsResponse
		compare(t, body, &got, want)
		require.Equal(t, genesis.Params, got.Params)
		require.Equal(t, genesis.Params, want.Params)
	})

	t.Run("markets", func(t *testing.T) {
		request := &types.QueryMarketsRequest{Offset: 0, Limit: 10}
		values := url.Values{
			"offset": {strconv.FormatUint(request.Offset, 10)},
			"limit":  {strconv.FormatUint(request.Limit, 10)},
		}
		body, err := testutil.GetRequestWithHeaders(baseURL+"markets?"+values.Encode(), headers)
		require.NoError(t, err)
		want, err := query.Markets(ctx, request)
		require.NoError(t, err)
		var got types.QueryMarketsResponse
		compare(t, body, &got, want)
		require.Empty(t, got.Markets)
		require.Zero(t, got.Total)
		require.Empty(t, want.Markets)
		require.Zero(t, want.Total)
	})

	t.Run("markets_by_creator", func(t *testing.T) {
		request := &types.QueryMarketsByCreatorRequest{Creator: val.Address.String(), Offset: 0, Limit: 10}
		values := url.Values{
			"offset": {strconv.FormatUint(request.Offset, 10)},
			"limit":  {strconv.FormatUint(request.Limit, 10)},
		}
		body, err := testutil.GetRequestWithHeaders(baseURL+"markets_by_creator/"+url.PathEscape(request.Creator)+"?"+values.Encode(), headers)
		require.NoError(t, err)
		want, err := query.MarketsByCreator(ctx, request)
		require.NoError(t, err)
		var got types.QueryMarketsByCreatorResponse
		compare(t, body, &got, want)
		require.Empty(t, got.Markets)
		require.Empty(t, want.Markets)
	})

	t.Run("market_unknown_denom", func(t *testing.T) {
		request := &types.QueryMarketRequest{Denom: "launchpad/0x0000000000000000000000000000000000000001"}
		requestBytes, err := proto.Marshal(request)
		require.NoError(t, err)
		abciResponse, err := val.RPCClient.ABCIQueryWithOptions(ctx,
			"/paxprotocol.paxchain.launchpad.Query/Market", requestBytes,
			rpcclient.ABCIQueryOptions{Height: height})
		require.NoError(t, err)
		require.Equal(t, types.ErrUnknownMarket.Codespace(), abciResponse.Response.Codespace)
		require.Equal(t, uint32(12), abciResponse.Response.Code)
		require.Equal(t, types.ErrUnknownMarket.ABCICode(), abciResponse.Response.Code)
		require.Contains(t, abciResponse.Response.Log, "LXP_LAUNCHPAD_UNKNOWN_MARKET")
		values := url.Values{"denom": {request.Denom}}
		body, err := testutil.GetRequestWithHeaders(baseURL+"market?"+values.Encode(), headers)
		require.NoError(t, err)
		response, err := query.Market(ctx, request)
		require.Error(t, err)
		require.Nil(t, response)
		want := status.Convert(err)
		require.Equal(t, codes.Unknown, want.Code())
		require.Equal(t, abciResponse.Response.Log, want.Message())
		var got statuspb.Status
		require.NoError(t, json.Unmarshal(body, &got), "REST body: %s", body)
		require.Equal(t, int32(want.Code()), got.Code)
		require.Equal(t, want.Message(), got.Message)
		require.Contains(t, got.Message, types.ErrUnknownMarket.Error())
		require.Empty(t, got.Details)
	})
}
