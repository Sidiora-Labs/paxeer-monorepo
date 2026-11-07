package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"gopkg.in/yaml.v3"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	kvsink "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	e2eapp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

// This consumer reads the published OpenAPI contract, resolves local references,
// and checks the schema keywords used by the repaired response contracts.
// Its input is always a response from a registered Environment handler.
type repair91Contract map[string]any

func repair91LoadContract(t *testing.T) repair91Contract {
	t.Helper()
	data, err := os.ReadFile("../../../rpc/openapi/openapi.yaml")
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(data, &doc))
	return repair91Contract(doc)
}

func (doc repair91Contract) schema(name string) map[string]any {
	return doc["components"].(map[string]any)["schemas"].(map[string]any)[name].(map[string]any)
}

func (doc repair91Contract) response(path, method string, status int, media string) map[string]any {
	return doc["paths"].(map[string]any)[path].(map[string]any)[method].(map[string]any)["responses"].(map[string]any)[strconv.Itoa(status)].(map[string]any)["content"].(map[string]any)[media].(map[string]any)["schema"].(map[string]any)
}

func (doc repair91Contract) validate(s map[string]any, v any) error {
	if name, ok := s["$ref"].(string); ok {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("nonlocal reference %s", name)
		}
		return doc.validate(doc.schema(strings.TrimPrefix(name, prefix)), v)
	}
	if v == nil && s["nullable"] == true {
		return nil
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		if list, ok := s[keyword].([]any); ok {
			matches := 0
			var failures []string
			for _, child := range list {
				if err := doc.validate(child.(map[string]any), v); err == nil {
					matches++
				} else {
					failures = append(failures, err.Error())
				}
			}
			if keyword == "allOf" && matches != len(list) || keyword == "anyOf" && matches == 0 || keyword == "oneOf" && matches != 1 {
				return fmt.Errorf("%s matched %d of %d: %s", keyword, matches, len(list), strings.Join(failures, "; "))
			}
		}
	}
	if neg, ok := s["not"].(map[string]any); ok && doc.validate(neg, v) == nil {
		return fmt.Errorf("forbidden shape")
	}
	if enum, ok := s["enum"].([]any); ok {
		encodedValue, err := json.Marshal(v)
		if err != nil {
			return err
		}
		found := false
		for _, x := range enum {
			encodedMember, err := json.Marshal(x)
			if err != nil {
				return err
			}
			if bytes.Equal(encodedMember, encodedValue) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("value %v is outside enum", v)
		}
	}
	switch s["type"] {
	case "object":
		if _, ok := v.(map[string]any); !ok {
			return fmt.Errorf("expected object, got %T", v)
		}
	case "array":
		if _, ok := v.([]any); !ok {
			return fmt.Errorf("expected array, got %T", v)
		}
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", v)
		}
	case "integer":
		n, ok := v.(float64)
		if !ok || math.Trunc(n) != n {
			return fmt.Errorf("expected integer, got %v", v)
		}
	}
	if fields, ok := v.(map[string]any); ok {
		if required, ok := s["required"].([]any); ok {
			for _, name := range required {
				if _, exists := fields[name.(string)]; !exists {
					return fmt.Errorf("missing field %s", name)
				}
			}
		}
		props, _ := s["properties"].(map[string]any)
		for name, value := range fields {
			if child, ok := props[name].(map[string]any); ok {
				if err := doc.validate(child, value); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			} else if s["additionalProperties"] == false {
				return fmt.Errorf("unexpected field %s", name)
			}
		}
	}
	if list, ok := v.([]any); ok {
		if limit, ok := s["maxItems"].(int); ok && len(list) > limit {
			return fmt.Errorf("too many items")
		}
		if item, ok := s["items"].(map[string]any); ok {
			for i, value := range list {
				if err := doc.validate(item, value); err != nil {
					return fmt.Errorf("item %d: %w", i, err)
				}
			}
		}
	}
	if value, ok := v.(string); ok {
		if pattern, ok := s["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(value) {
			return fmt.Errorf("%q does not match %s", value, pattern)
		}
		if s["format"] == "byte" {
			if _, err := base64.StdEncoding.DecodeString(value); err != nil {
				return err
			}
		}
	}
	if value, ok := v.(float64); ok {
		for _, bound := range []string{"minimum", "maximum"} {
			if limit, ok := s[bound]; ok {
				var n float64
				switch x := limit.(type) {
				case int:
					n = float64(x)
				case uint64:
					n = float64(x)
				case float64:
					n = x
				default:
					return fmt.Errorf("unsupported bound %T", limit)
				}
				if bound == "minimum" && value < n || bound == "maximum" && value > n {
					return fmt.Errorf("%v violates %s %v", value, bound, n)
				}
			}
		}
	}
	return nil
}

func repair91Server(t *testing.T, env *Environment) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func repair91Request(t *testing.T, ctx context.Context, server *httptest.Server, method, path, body string) (int, []byte, string) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, data, strings.Split(response.Header.Get("Content-Type"), ";")[0]
}

// Both transports feed the schema consumer and the real typed result decoder.
func repair91Call(t *testing.T, ctx context.Context, server *httptest.Server, name string, params map[string]any, resultSchema string, result any) []byte {
	t.Helper()
	doc := repair91LoadContract(t)
	var last []byte
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/" + name
		query := url.Values{}
		for key, value := range params {
			query.Set(key, fmt.Sprint(value))
		}
		path += "?" + query.Encode()
		body := ""
		if method == http.MethodPost {
			path = "/"
			payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "repair-91", "method": name, "params": params})
			require.NoError(t, err)
			body = string(payload)
		}
		status, data, media := repair91Request(t, ctx, server, method, path, body)
		require.Equal(t, http.StatusOK, status, "%s %s: %s", method, name, data)
		require.Equal(t, "application/json", media)
		var wire any
		require.NoError(t, json.Unmarshal(data, &wire))
		require.NoError(t, doc.validate(doc.response(map[bool]string{true: "/", false: "/" + name}[method == http.MethodPost], strings.ToLower(method), status, media), wire), "%s %s: %s", method, name, data)
		resultData := data
		if method == http.MethodPost {
			var envelope rpctypes.RPCResponse
			require.NoError(t, json.Unmarshal(data, &envelope))
			require.Nil(t, envelope.Error)
			require.Equal(t, `"repair-91"`, envelope.ID())
			resultData = envelope.Result
		} else {
			fields := wire.(map[string]any)
			require.NotContains(t, fields, "jsonrpc")
			require.NotContains(t, fields, "result")
		}
		var bare any
		require.NoError(t, json.Unmarshal(resultData, &bare))
		require.NoError(t, doc.validate(doc.schema(resultSchema), bare), "%s %s: %s", method, name, resultData)
		require.NoError(t, tmjson.Unmarshal(resultData, result))
		if last != nil {
			require.JSONEq(t, string(last), string(resultData))
		}
		last = bytes.Clone(resultData)
	}
	return last
}

type repair91ObservedApp struct {
	abci.Application
	committed utils.AtomicSend[int64]
}

func (app *repair91ObservedApp) Commit(ctx context.Context) (*abci.ResponseCommit, error) {
	response, err := app.Application.Commit(ctx)
	if err != nil {
		return nil, err
	}
	info, err := app.Application.Info(ctx, &abci.RequestInfo{})
	if err != nil {
		return nil, err
	}
	app.committed.Store(info.LastBlockHeight)
	return response, nil
}

func repair91WithBlock(t *testing.T, rng utils.Rng, application abci.Application, check func(context.Context, *Environment, *coretypes.ResultBlock)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	observed := &repair91ObservedApp{Application: application, committed: utils.NewAtomicSend[int64](0)}
	app := proxy.New(observed, proxy.NopMetrics())
	states := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, states.Close()) })
	blocks := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blocks.Close()) })
	sink := kvsink.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	router, giga := newAutobahnTestRouter(t, rng, "repair-91", app, states, 2, 1000)
	genesis := &types.GenesisDoc{ChainID: "repair-91", InitialHeight: 1}
	require.NoError(t, genesis.ValidateAndComplete())
	env := &Environment{App: app, Router: router, StateStore: states, BlockStore: blocks, EventSinks: []indexer.EventSink{sink}, GenDoc: genesis}
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for _, tx := range []types.Tx{types.Tx("repair-a=first"), types.Tx("repair-b=second"), types.Tx("seal=next")} {
			response, err := giga.InsertTx(ctx, tx)
			if err != nil {
				return err
			}
			require.True(t, response.IsOK())
		}
		_, err := observed.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= 1 })
		if err != nil {
			return err
		}
		block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(1))
		if err != nil {
			return err
		}
		require.Len(t, block.Block.Txs, 2)
		finalized, err := states.LoadFinalizeBlockResponses(1)
		if err != nil {
			return err
		}
		require.NoError(t, sink.IndexBlockEvents(types.EventDataNewBlockHeader{Header: block.Block.Header, NumTxs: int64(len(block.Block.Txs)), ResultFinalizeBlock: *finalized}))
		indexed := make([]*abci.TxResultV2, len(block.Block.Txs))
		for i, tx := range block.Block.Txs {
			indexed[i] = &abci.TxResultV2{Height: 1, Index: uint32(i), Tx: tx, Result: *finalized.TxResults[i]}
		}
		require.NoError(t, sink.IndexTxEvents(indexed))
		// Compare consumer values with the producer's decoded wire representation.
		// HexBytes encodes nil as an empty string; decoding creates an empty slice.
		encoded, err := tmjson.Marshal(block)
		require.NoError(t, err)
		var decoded coretypes.ResultBlock
		require.NoError(t, tmjson.Unmarshal(encoded, &decoded))
		check(ctx, env, &decoded)
		return nil
	}))
}

func TestRepair_91_BlockOperations(t *testing.T) {
	rng := utils.TestRng()
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	repair91WithBlock(t, rng, app, func(ctx context.Context, env *Environment, block *coretypes.ResultBlock) {
		server := repair91Server(t, env)
		var results coretypes.ResultBlockResults
		repair91Call(t, ctx, server, "block_results", map[string]any{"height": "1"}, "BlockResultsResponse", &results)
		require.Equal(t, int64(1), results.Height)
		require.Len(t, results.TxsResults, 2)
		var search coretypes.ResultBlockSearch
		repair91Call(t, ctx, server, "block_search", map[string]any{"query": "block.height=1"}, "BlockSearchResponse", &search)
		require.Equal(t, 1, search.TotalCount)
		require.Equal(t, []*coretypes.ResultBlock{block}, search.Blocks)
	})
}

func TestRepair_91_TransportFraming(t *testing.T) {
	env := &Environment{}
	server := repair91Server(t, env)
	var health coretypes.ResultHealth
	repair91Call(t, t.Context(), server, "health", map[string]any{}, "EmptyResponse", &health)
	doc := repair91LoadContract(t)
	status, data, media := repair91Request(t, t.Context(), server, http.MethodPost, "/", `[{"jsonrpc":"2.0","id":1,"method":"health"},{"jsonrpc":"2.0","id":2,"method":"health"}]`)
	require.Equal(t, http.StatusOK, status)
	var batch []any
	require.NoError(t, json.Unmarshal(data, &batch))
	require.Len(t, batch, 2)
	require.NoError(t, doc.validate(doc.response("/", "post", status, media), any(batch)))
	for i, v := range batch {
		require.Equal(t, float64(i+1), v.(map[string]any)["id"])
		require.Equal(t, map[string]any{}, v.(map[string]any)["result"])
	}
	status, data, media = repair91Request(t, t.Context(), server, http.MethodPost, "/", `[{"jsonrpc":"2.0","id":1,"method":"health"}]`)
	require.Equal(t, http.StatusOK, status)
	var single []any
	require.NoError(t, json.Unmarshal(data, &single))
	require.Len(t, single, 1)
	require.Equal(t, map[string]any{"jsonrpc": "2.0", "id": float64(1), "result": map[string]any{}}, single[0])
	require.NoError(t, doc.validate(doc.response("/", "post", status, media), single))
}

func TestRepair_91_StructuredErrors(t *testing.T) {
	env := &Environment{}
	server := repair91Server(t, env)
	doc := repair91LoadContract(t)
	for _, tc := range []struct {
		name, method, path, body string
		code                     int
	}{
		{"parse", http.MethodPost, "/", "{", int(rpctypes.CodeParseError)},
		{"post_invalid_params", http.MethodPost, "/", `{"jsonrpc":"2.0","id":1,"method":"header","params":{"height":"bad"}}`, int(rpctypes.CodeInvalidParams)},
		{"uri_invalid_params", http.MethodGet, "/header?height=bad", "", int(rpctypes.CodeInvalidParams)},
		{"post_unavailable", http.MethodPost, "/", `{"jsonrpc":"2.0","id":1,"method":"consensus_state"}`, int(rpctypes.CodeInternalError)},
		{"uri_unavailable", http.MethodGet, "/consensus_state", "", int(rpctypes.CodeInternalError)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, data, media := repair91Request(t, t.Context(), server, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusOK, status)
			var wire any
			require.NoError(t, json.Unmarshal(data, &wire))
			if tc.method == http.MethodPost {
				require.NoError(t, doc.validate(doc.response("/", "post", status, media), wire))
				var response rpctypes.RPCResponse
				require.NoError(t, json.Unmarshal(data, &response))
				require.NotNil(t, response.Error)
				require.Equal(t, tc.code, response.Error.Code)
				require.Empty(t, response.Result)
			} else {
				require.NoError(t, doc.validate(doc.response(strings.Split(tc.path, "?")[0], "get", status, media), wire))
				var refusal rpctypes.RPCError
				require.NoError(t, json.Unmarshal(data, &refusal))
				require.Equal(t, tc.code, refusal.Code)
			}
			fields := wire.(map[string]any)
			if tc.method == http.MethodPost {
				fields = fields["error"].(map[string]any)
			}
			require.NoError(t, doc.validate(doc.schema("RPCError"), fields))
			require.NotEmpty(t, fields["message"])
			require.NotEmpty(t, fields["data"])
		})
	}
	status, data, media := repair91Request(t, t.Context(), server, http.MethodGet, "/header_by_hash?hash=0xXYZ", "")
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "text/plain", media)
	require.NotEmpty(t, data)
	require.NoError(t, doc.validate(doc.response("/header_by_hash", "get", status, media), string(data)))
}

func TestRepair_91_Headers(t *testing.T) {
	rng := utils.TestRng()
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	repair91WithBlock(t, rng, app, func(ctx context.Context, env *Environment, block *coretypes.ResultBlock) {
		server := repair91Server(t, env)
		for _, tc := range []struct {
			name   string
			params map[string]any
			header *types.Header
		}{
			{"header", map[string]any{"height": "1"}, &block.Block.Header},
			{"header_by_hash", map[string]any{"hash": strings.ToUpper(hex.EncodeToString(block.BlockID.Hash))}, &block.Block.Header},
			{"header_by_hash", map[string]any{"hash": strings.Repeat("AB", 32)}, nil},
		} {
			var result coretypes.ResultHeader
			data := repair91Call(t, ctx, server, tc.name, tc.params, "HeaderResponse", &result)
			require.Equal(t, tc.header, result.Header)
			if tc.header == nil {
				require.JSONEq(t, `{"header":null}`, string(data))
			}
		}
	})
}

func TestRepair_91_FinalizeBlock(t *testing.T) {
	rng := utils.TestRng()
	app, err := e2eapp.NewApplication(e2eapp.DefaultConfig(t.TempDir()))
	require.NoError(t, err)
	repair91WithBlock(t, rng, app, func(ctx context.Context, env *Environment, _ *coretypes.ResultBlock) {
		var result coretypes.ResultBlockResults
		data := repair91Call(t, ctx, repair91Server(t, env), "block_results", map[string]any{"height": "1"}, "BlockResultsResponse", &result)
		stored, err := env.StateStore.LoadFinalizeBlockResponses(1)
		require.NoError(t, err)
		require.Equal(t, stored.Events, result.FinalizeBlockEvents)
		require.NotEmpty(t, result.FinalizeBlockEvents)
		require.Equal(t, stored.TxResults, result.TxsResults)
		require.Len(t, result.TxsResults, 2)
		var wire map[string]any
		require.NoError(t, json.Unmarshal(data, &wire))
		require.Contains(t, wire, "consensus_param_updates")
		require.NotContains(t, wire, "consensus_params_updates")
		require.NotContains(t, wire, "begin_block_events")
		require.NotContains(t, wire, "end_block")
		var gas int64
		for _, r := range stored.TxResults {
			gas += r.GasUsed
		}
		require.Equal(t, gas, result.TotalGasUsed)
	})

}

func TestRepair_91_NullCollections(t *testing.T) {
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	block := types.MakeBlock(1, nil, &types.Commit{}, nil)
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blocks := store.NewBlockStore(dbm.NewMemDB())
	defer func() { require.NoError(t, blocks.Close()) }()
	blocks.SaveBlock(block, parts, &types.Commit{Height: 1})
	finalized, err := app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{Header: block.Header.ToProto()})
	require.NoError(t, err)
	states := sm.NewStore(dbm.NewMemDB())
	defer func() { require.NoError(t, states.Close()) }()
	require.NoError(t, states.SaveFinalizeBlockResponses(1, finalized))
	env := &Environment{StateStore: states, BlockStore: blocks}
	var result coretypes.ResultBlockResults
	data := repair91Call(t, t.Context(), repair91Server(t, env), "block_results", map[string]any{"height": "1"}, "BlockResultsResponse", &result)
	require.Empty(t, result.TxsResults)
	require.Nil(t, result.FinalizeBlockEvents)
	require.Nil(t, result.ConsensusParamUpdates)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(data, &wire))
	require.Nil(t, wire["finalize_block_events"])
	require.Nil(t, wire["consensus_param_updates"])
}

func TestRepair_91_TransactionProofs(t *testing.T) {
	rng := utils.TestRng()
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	repair91WithBlock(t, rng, app, func(ctx context.Context, env *Environment, block *coretypes.ResultBlock) {
		server := repair91Server(t, env)
		tx := block.Block.Txs[0]
		var result coretypes.ResultTx
		repair91Call(t, ctx, server, "tx", map[string]any{"hash": strings.ToUpper(hex.EncodeToString(tx.Hash().Bytes())), "prove": true}, "TxResponse", &result)
		require.Equal(t, tx, result.Tx)
		require.NoError(t, result.Proof.Validate(block.Block.Txs.Hash()))
		require.NotEmpty(t, result.Proof.Proof.Aunts)
		require.NotEmpty(t, result.TxResult.Events)
		var search coretypes.ResultTxSearch
		repair91Call(t, ctx, server, "tx_search", map[string]any{"query": "tx.height=1", "prove": true}, "TxSearchResponse", &search)
		require.Equal(t, 2, search.TotalCount)
		require.Len(t, search.Txs, 2)
		found := false
		for _, got := range search.Txs {
			require.NoError(t, got.Proof.Validate(block.Block.Txs.Hash()))
			if bytes.Equal(got.Tx, tx) {
				require.Equal(t, result, *got)
				found = true
			}
		}
		require.True(t, found)
	})
}

// The adapter delegates Query to the real committed SDK store, without
// constructing response fields or proof operations in the test.
type repair91StoreApplication struct {
	abci.BaseApplication
	store *rootmultiv2.Store
}

func (app *repair91StoreApplication) Query(ctx context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	response := app.store.Query(ctx, *req)
	return &response, nil
}

func TestRepair_91_ABCIQueries(t *testing.T) {
	config := storageconfig.DefaultStateCommitConfig()
	config.MemIAVLConfig.AsyncCommitBuffer = 0
	state := rootmultiv2.NewStore(t.TempDir(), config, storageconfig.StateStoreConfig{}, nil)
	defer func() { require.NoError(t, state.Close()) }()
	key := storetypes.NewKVStoreKey("bank")
	state.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, state.LoadLatestVersion())
	cache := state.CacheMultiStore()
	cache.GetKVStore(key).Set([]byte("key"), []byte("nonempty value"))
	cache.Write()
	commit := state.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	env := &Environment{App: proxy.New(&repair91StoreApplication{store: state}, proxy.NopMetrics())}
	server := repair91Server(t, env)
	for _, tc := range []struct {
		name, path, key string
		exists, refused bool
	}{
		{"present", "/bank/key", "key", true, false},
		{"absence", "/bank/key", "missing", false, false},
		{"refusal", "invalid/path", "key", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result coretypes.ResultABCIQuery
			data := repair91Call(t, t.Context(), server, "abci_query", map[string]any{"path": tc.path, "data": hex.EncodeToString([]byte(tc.key)), "height": "1", "prove": true}, "ABCIQueryResponse", &result)
			if tc.refused {
				require.NotZero(t, result.Response.Code)
				require.Nil(t, result.Response.ProofOps)
				require.Nil(t, result.Response.Key)
				require.Nil(t, result.Response.Value)
				return
			}
			require.Zero(t, result.Response.Code)
			require.Equal(t, []byte(tc.key), result.Response.Key)
			require.Equal(t, commit.Version, result.Response.Height)
			require.NotNil(t, result.Response.ProofOps)
			require.Len(t, result.Response.ProofOps.Ops, 2)
			keypath := merkle.KeyPath{}.AppendKey([]byte("bank"), merkle.KeyEncodingHex).AppendKey([]byte(tc.key), merkle.KeyEncodingHex).String()
			runtime := rootmulti.DefaultProofRuntime()
			if tc.exists {
				require.Equal(t, []byte("nonempty value"), result.Response.Value)
				require.NoError(t, runtime.VerifyValue(result.Response.ProofOps, commit.Hash, keypath, result.Response.Value))
			} else {
				require.Empty(t, result.Response.Value)
				require.Nil(t, result.Response.Value)
				require.NoError(t, runtime.VerifyAbsence(result.Response.ProofOps, commit.Hash, keypath))
			}
			var wire map[string]any
			require.NoError(t, json.Unmarshal(data, &wire))
			response := wire["response"].(map[string]any)
			require.Contains(t, response, "proofOps")
			require.NotContains(t, response, "proof")
			require.NotContains(t, response, "proof_ops")
			require.Equal(t, base64.StdEncoding.EncodeToString([]byte(tc.key)), response["key"])
		})
	}
}

func TestRepair_91_BroadcastCommit(t *testing.T) {
	rng := utils.TestRng()
	app, err := e2eapp.NewApplication(e2eapp.DefaultConfig(t.TempDir()))
	require.NoError(t, err)
	repair91WithBlock(t, rng, app, func(ctx context.Context, env *Environment, block *coretypes.ResultBlock) {
		// The public broadcast route runs real CheckTx and reads the committed index.
		server := repair91Server(t, env)
		var result coretypes.ResultBroadcastTxCommit
		data := repair91Call(t, ctx, server, "broadcast_tx_commit", map[string]any{"tx": base64.StdEncoding.EncodeToString(block.Block.Txs[0])}, "BroadcastTxCommitResponse", &result)
		require.Zero(t, result.CheckTx.Code)
		require.Nil(t, result.CheckTx.Data)
		require.Zero(t, result.TxResult.Code)
		require.Equal(t, int64(1), result.Height)
		require.Equal(t, block.Block.Txs[0].Hash().Bytes(), result.Hash)
		require.Contains(t, string(data), `"tx_result"`)
		require.NotContains(t, string(data), "deliver_tx")
		var refused coretypes.ResultBroadcastTxCommit
		repair91Call(t, ctx, server, "broadcast_tx_commit", map[string]any{"tx": base64.StdEncoding.EncodeToString([]byte("invalid transaction"))}, "BroadcastTxCommitResponse", &refused)
		require.NotZero(t, refused.CheckTx.Code)
		require.Zero(t, refused.Height)
	})
}

func TestRepair_91_ConsensusParameters(t *testing.T) {
	rng := utils.TestRng()
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	repair91WithBlock(t, rng, app, func(ctx context.Context, env *Environment, _ *coretypes.ResultBlock) {
		server := repair91Server(t, env)
		var result coretypes.ResultConsensusParams
		repair91Call(t, ctx, server, "consensus_params", map[string]any{"height": "1"}, "ConsensusParamsResponse", &result)
		require.Equal(t, *env.GenDoc.ConsensusParams, result.ConsensusParams)
		require.Positive(t, result.ConsensusParams.Block.MaxGasWanted)
		require.Positive(t, result.ConsensusParams.Evidence.MaxAgeDuration)
		var genesis coretypes.ResultGenesis
		repair91Call(t, ctx, server, "genesis", map[string]any{}, "GenesisResponse", &genesis)
		require.Equal(t, env.GenDoc.ConsensusParams, genesis.Genesis.ConsensusParams)
	})
}

func TestRepair_91_LagErrors(t *testing.T) {
	rng := utils.TestRng()
	resume := make(chan struct{})
	app := &lagStatusAutobahnApplication{statusAutobahnObservedApplication: &statusAutobahnObservedApplication{Application: kvstore.NewApplication(), committed: utils.NewAtomicSend[int64](0)}, executing: make(chan struct{}), resume: resume}
	defer func() { require.NoError(t, app.Close()) }()
	proxyApp := proxy.New(app, proxy.NopMetrics())
	states := sm.NewStore(dbm.NewMemDB())
	defer func() { require.NoError(t, states.Close()) }()
	blocks := store.NewBlockStore(dbm.NewMemDB())
	defer func() { require.NoError(t, blocks.Close()) }()
	router, giga := newAutobahnTestRouter(t, rng, "repair-91-lag", proxyApp, states, 1, 1000)
	env := &Environment{App: proxyApp, Router: router, StateStore: states, BlockStore: blocks}
	server := repair91Server(t, env)
	doc := repair91LoadContract(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for i := range 5 {
			response, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value", i)))
			if err != nil {
				return err
			}
			require.True(t, response.IsOK())
		}
		select {
		case <-app.executing:
		case <-ctx.Done():
			return ctx.Err()
		}
		defer close(resume)
		produced, err := env.LagStatus(ctx)
		require.ErrorIs(t, err, coretypes.ErrLagIsTooHigh)
		require.Positive(t, produced.Lag)
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			path, body := "/lag_status", ""
			if method == http.MethodPost {
				path = "/"
				body = `{"jsonrpc":"2.0","id":1,"method":"lag_status"}`
			}
			status, data, media := repair91Request(t, ctx, server, method, path, body)
			require.Equal(t, http.StatusExpectationFailed, status)
			var wire any
			require.NoError(t, json.Unmarshal(data, &wire))
			require.NoError(t, doc.validate(doc.response(path, strings.ToLower(method), status, media), wire))
			resultData := data
			if method == http.MethodPost {
				var response rpctypes.RPCResponse
				require.NoError(t, json.Unmarshal(data, &response))
				require.NotNil(t, response.Error)
				require.Equal(t, int(rpctypes.CodeLagIsHighError), response.Error.Code)
				require.JSONEq(t, string(response.Result), response.Error.Data)
				resultData = response.Result
			}
			var result coretypes.ResultLagStatus
			require.NoError(t, tmjson.Unmarshal(resultData, &result))
			require.Equal(t, *produced, result)
		}
		return nil
	}))
}
