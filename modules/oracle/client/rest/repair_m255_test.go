package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

// This fixture serves the production query router over loopback JSON-RPC. The
// REST route still uses client.Context and its real protobuf/ABCI transport.
type repairM255Fixture struct {
	input    testutils.TestInput
	cdc      codec.Codec
	router   *mux.Router
	server   *httptest.Server
	mu       sync.Mutex
	requests []abci.RequestQuery
}

func newRepairM255Fixture(t *testing.T) *repairM255Fixture {
	t.Helper()
	input := testutils.CreateTestInput(t)
	input.Ctx = input.Ctx.WithBlockHeight(100).WithBlockTime(time.Unix(5400, 0))
	input.OracleKeeper.SetBaseExchangeRate(input.Ctx, "uatom", sdk.NewDec(50))
	input.OracleKeeper.SetVoteTarget(input.Ctx, "uatom")
	input.OracleKeeper.SetFeederDelegation(input.Ctx, testutils.ValAddrs[0], testutils.Addrs[1])
	input.OracleKeeper.SetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0], 3, 4, 5)
	for _, observation := range []struct{ timestamp, price int64 }{{5340, 30}, {5370, 50}} {
		input.OracleKeeper.SetPriceSnapshot(input.Ctx, types.NewPriceSnapshot(types.PriceSnapshotItems{
			types.NewPriceSnapshotItem("uatom", types.OracleExchangeRate{
				ExchangeRate: sdk.NewDec(observation.price), LastUpdate: sdk.NewInt(100), LastUpdateTimestamp: observation.timestamp * 1000,
			}),
		}, observation.timestamp))
	}
	encoding := testutils.MakeEncodingConfig(t)
	f := &repairM255Fixture{input: input, cdc: encoding.Marshaler, router: mux.NewRouter()}
	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(encoding.InterfaceRegistry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(encoding.InterfaceRegistry)
	appModule := oracle.NewAppModule(encoding.Marshaler, input.OracleKeeper, input.AccountKeeper, input.BankKeeper)
	appModule.RegisterServices(module.NewConfigurator(encoding.Marshaler, msgRouter, queryRouter))
	require.Nil(t, appModule.LegacyQuerierHandler(encoding.Amino))
	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			height := int64(req.Height)
			if height == 0 {
				height = input.Ctx.BlockHeight()
			}
			request := abci.RequestQuery{Path: req.Path, Data: req.Data, Height: height, Prove: req.Prove}
			f.mu.Lock()
			defer f.mu.Unlock()
			f.requests = append(f.requests, request)
			route := queryRouter.Route(req.Path)
			if route == nil {
				return nil, fmt.Errorf("unregistered query route %q", req.Path)
			}
			response, err := route(input.Ctx.WithBlockHeight(height), request)
			if err != nil {
				response.Codespace, response.Code, response.Log = sdkerrors.ABCIInfo(err, false)
			}
			return &coretypes.ResultABCIQuery{Response: response}, nil
		}),
	})
	f.server = httptest.NewServer(rpcMux)
	t.Cleanup(f.server.Close)
	rpcClient, err := rpchttp.New(f.server.URL)
	require.NoError(t, err)
	cliCtx := client.Context{}.WithClient(rpcClient).WithCodec(encoding.Marshaler).WithLegacyAmino(encoding.Amino)
	oracle.AppModuleBasic{}.RegisterRESTRoutes(cliCtx, f.router)
	return f
}

func (f *repairM255Fixture) request(t *testing.T, path string, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, expectedStatus, response.Code, response.Body.String())
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	require.Equal(t, "true", response.Header().Get("Deprecation"))
	require.NotEmpty(t, response.Body.String())
	return response
}

func repairM255Result(t *testing.T, response *httptest.ResponseRecorder, height int64) json.RawMessage {
	t.Helper()
	var envelope struct {
		Height string          `json:"height"`
		Result json.RawMessage `json:"result"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, strconv.FormatInt(height, 10), envelope.Height)
	require.NotEmpty(t, envelope.Result)
	return envelope.Result
}

// Finding 015: the mounted route declares and validates its query parameter,
// then obtains the actual weighted result from the keeper through QueryClient.
func TestRepairM_255_TwapLookback(t *testing.T) {
	f := newRepairM255Fixture(t)
	for _, tc := range []struct {
		lookback string
		price    int64
	}{{"60", 40}, {"30", 50}, {"1", 50}} {
		t.Run("valid_"+tc.lookback, func(t *testing.T) {
			if tc.lookback == "1" {
				currentTime := f.input.Ctx.BlockTime().Unix()
				for _, timestamp := range []int64{currentTime - 1, currentTime} {
					f.input.OracleKeeper.SetPriceSnapshot(f.input.Ctx, types.NewPriceSnapshot(types.PriceSnapshotItems{
						types.NewPriceSnapshotItem("uatom", types.OracleExchangeRate{
							ExchangeRate: sdk.NewDec(tc.price), LastUpdate: sdk.NewInt(100), LastUpdateTimestamp: timestamp * 1000,
						}),
					}, timestamp))
				}
			}
			response := f.request(t, "/oracle/denoms/twaps?lookback_seconds="+tc.lookback, http.StatusOK)
			var result types.QueryTwapsResponse
			require.NoError(t, f.cdc.UnmarshalAsJSON(repairM255Result(t, response, 100), &result))
			require.Len(t, result.OracleTwaps, 1)
			require.Equal(t, "uatom", result.OracleTwaps[0].Denom)
			require.Equal(t, sdk.NewDec(tc.price), result.OracleTwaps[0].Twap)
			lookback, err := strconv.ParseInt(tc.lookback, 10, 64)
			require.NoError(t, err)
			require.Equal(t, lookback, result.OracleTwaps[0].LookbackSeconds)
		})
	}
	for _, tc := range []struct {
		name, query string
		requests    int
	}{
		{"absent", "", 0}, {"empty", "?lookback_seconds=", 0},
		{"nonnumeric", "?lookback_seconds=abc", 0}, {"zero", "?lookback_seconds=0", 0},
		{"negative", "?lookback_seconds=-1", 0}, {"overflow", "?lookback_seconds=9223372036854775808", 0},
		{"duplicate", "?lookback_seconds=30&lookback_seconds=60", 0}, {"malformed", "?lookback_seconds=%zz", 0},
		{"above_configured_max", "?lookback_seconds=" + strconv.FormatUint(f.input.OracleKeeper.LookbackDuration(f.input.Ctx)+1, 10), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.mu.Lock()
			before := len(f.requests)
			f.mu.Unlock()
			response := f.request(t, "/oracle/denoms/twaps"+tc.query, http.StatusBadRequest)
			var result struct {
				Code  int    `json:"code"`
				Error string `json:"error"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.Equal(t, http.StatusBadRequest, result.Code)
			require.NotEmpty(t, result.Error)
			f.mu.Lock()
			defer f.mu.Unlock()
			require.Len(t, f.requests, before+tc.requests)
		})
	}
}

// Finding 014: all retained registrations reach the module's real typed query
// service despite LegacyQuerierHandler returning nil, preserving response data.
func TestRepairM_255_TypedQueryRoutes(t *testing.T) {
	f := newRepairM255Fixture(t)
	q := keeper.NewQuerier(f.input.OracleKeeper)
	ctx := sdk.WrapSDKContext(f.input.Ctx)
	voter := testutils.ValAddrs[0].String()
	for _, tc := range []struct {
		path, method string
		expected     func() (proto.Message, error)
	}{
		{"/oracle/denoms/uatom/exchange_rate", "ExchangeRate", func() (proto.Message, error) {
			return q.ExchangeRate(ctx, &types.QueryExchangeRateRequest{Denom: "uatom"})
		}},
		{"/oracle/denoms/exchange_rates", "ExchangeRates", func() (proto.Message, error) { return q.ExchangeRates(ctx, &types.QueryExchangeRatesRequest{}) }},
		{"/oracle/denoms/price_snapshot_history", "PriceSnapshotHistory", func() (proto.Message, error) {
			return q.PriceSnapshotHistory(ctx, &types.QueryPriceSnapshotHistoryRequest{})
		}},
		{"/oracle/denoms/twaps?lookback_seconds=60", "Twaps", func() (proto.Message, error) { return q.Twaps(ctx, &types.QueryTwapsRequest{LookbackSeconds: 60}) }},
		{"/oracle/denoms/actives", "Actives", func() (proto.Message, error) { return q.Actives(ctx, &types.QueryActivesRequest{}) }},
		{"/oracle/denoms/vote_targets", "VoteTargets", func() (proto.Message, error) { return q.VoteTargets(ctx, &types.QueryVoteTargetsRequest{}) }},
		{"/oracle/voters/" + voter + "/feeder", "FeederDelegation", func() (proto.Message, error) {
			return q.FeederDelegation(ctx, &types.QueryFeederDelegationRequest{ValidatorAddr: voter})
		}},
		{"/oracle/voters/" + voter + "/vote_penalty_counter", "VotePenaltyCounter", func() (proto.Message, error) {
			return q.VotePenaltyCounter(ctx, &types.QueryVotePenaltyCounterRequest{ValidatorAddr: voter})
		}},
		{"/oracle/parameters", "Params", func() (proto.Message, error) { return q.Params(ctx, &types.QueryParamsRequest{}) }},
	} {
		t.Run(tc.method, func(t *testing.T) {
			expected, err := tc.expected()
			require.NoError(t, err)
			expectedJSON, err := f.cdc.MarshalAsJSON(expected)
			require.NoError(t, err)
			response := f.request(t, tc.path, http.StatusOK)
			require.JSONEq(t, string(expectedJSON), string(repairM255Result(t, response, 100)))
			f.mu.Lock()
			defer f.mu.Unlock()
			require.Equal(t, "/paxprotocol.paxchain.oracle.Query/"+tc.method, f.requests[len(f.requests)-1].Path)
		})
	}
	// A later store write is visible through the mounted route, including metadata.
	f.input.OracleKeeper.SetBaseExchangeRate(f.input.Ctx, "uatom", sdk.NewDec(77))
	response := f.request(t, "/oracle/denoms/uatom/exchange_rate?height=100", http.StatusOK)
	var result types.QueryExchangeRateResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(repairM255Result(t, response, 100), &result))
	require.Equal(t, sdk.NewDec(77), result.OracleExchangeRate.ExchangeRate)
	require.Equal(t, sdk.NewInt(100), result.OracleExchangeRate.LastUpdate)
	require.Equal(t, int64(5400000), result.OracleExchangeRate.LastUpdateTimestamp)
}

func TestRepairM_255_QueryValidationAndFailure(t *testing.T) {
	f := newRepairM255Fixture(t)
	for _, path := range []string{
		"/oracle/parameters?height=abc", "/oracle/parameters?height=-1",
		"/oracle/denoms/x/exchange_rate", "/oracle/voters/invalid/feeder", "/oracle/voters/invalid/vote_penalty_counter",
	} {
		t.Run(path, func(t *testing.T) { f.request(t, path, http.StatusBadRequest) })
	}
	f.mu.Lock()
	require.Empty(t, f.requests)
	f.mu.Unlock()
	// Transport errors must remain explicit errors rather than empty successes.
	f.server.Close()
	response := f.request(t, "/oracle/parameters", http.StatusInternalServerError)
	var result struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.NotEmpty(t, result.Error)
}
