package evmrpc_test

// Task 22.33.1292 repair record, audit 22.30.68:
// rpc-instructions-drift -> TestRepairM_1292_RPCInstructions.
// profiled-empty-tracer -> TestRepairM_1292_ProfiledDefaultTracer.
// profiled-empty-test -> TestRepairM_1292_ProfiledDefaultTracer.
// profiled-parent-cancel -> TestRepairM_1292_ProfiledParentCancel.
// tenant-ws-test -> TestRepairM_1292_TenantWSReads.
// skipped-filter-cases -> TestRepairM_1292_FilterCases.
// legacy-body-truncation -> TestRepairM_1292_LegacyBodyLimit.
// legacy-single-notification -> TestRepairM_1292_LegacyNotification.
// Baseline verify was not run: the controlling ordered instructions require
// complete implementation before hf-build/hf-done and prohibit a manual verify.
// No pre-change test failures or gates are claimed. No out-of-path wiring is
// required; this record lives here because the task explicitly forbids spec edits.
// Qualification repair: use wasm/x/wasm/types for native receipts and commit
// the mixed-domain producer state before querying its retained versions.
// Consensus peers are committed block transcripts; all receipt producers,
// keepers, stores, tracing, filters, routing, codecs and clients execute in process.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	rpcfixtures "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/tests"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type repairM1292TraceFixture struct {
	app       *app.App
	consensus *rpcfixtures.MockClient
	block     *coretypes.ResultBlock
	hashes    []common.Hash
	api       *evmrpc.DebugAPI
}

func newRepairM1292TraceFixture(t *testing.T, count int) *repairM1292TraceFixture {
	t.Helper()
	a, consensus := rpcfixtures.SetupCommittedHeaderFixture(t)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	_, evm := rpcfixtures.CommittedHeaderTransactions(t, a)
	// Record the actual parent before adding the next committed block transcript.
	one := int64(1)
	parent, err := consensus.Block(context.Background(), &one)
	require.NoError(t, err)
	parentResults, err := consensus.BlockResults(context.Background(), &one)
	require.NoError(t, err)
	transactions := evm[:count]
	txs := make([]tmtypes.Tx, len(transactions))
	for i, tx := range transactions {
		txs[i] = tx
	}
	block := tmtypes.MakeBlock(2, txs, &tmtypes.Commit{Height: 1}, nil)
	block.Header = parent.Block.Header
	block.Height = 2
	block.Time = parent.Block.Time.Add(time.Second)
	block.LastBlockID = parent.BlockID
	block.AppHash = a.LastCommitID().Hash
	block.DataHash = block.Txs.Hash()
	block.LastResultsHash = common.HexToHash("0x1292").Bytes()
	hash := common.BytesToHash(block.Hash())
	response, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: block.Header.ToProto(), Hash: hash.Bytes(), Txs: transactions})
	require.NoError(t, err)
	require.Len(t, response.TxResults, count)
	for _, result := range response.TxResults {
		require.Equal(t, abci.CodeTypeOK, result.Code, result.Log)
	}
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	consensus.RecordCommittedBlock(parent, parentResults)
	recorded := &coretypes.ResultBlock{BlockID: tmtypes.BlockID{Hash: hash.Bytes()}, Block: block}
	consensus.RecordCommittedBlock(recorded, &coretypes.ResultBlockResults{Height: 2, TxsResults: response.TxResults})
	f := &repairM1292TraceFixture{app: a, consensus: consensus, block: recorded}
	for _, raw := range transactions {
		decoded, err := a.GetTxConfig().TxDecoder()(raw)
		require.NoError(t, err)
		msg := decoded.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
		tx, _ := msg.AsTransaction()
		require.NotNil(t, tx)
		f.hashes = append(f.hashes, tx.Hash())
	}
	store := a.EvmKeeper.ReceiptStore()
	require.Eventually(t, func() bool {
		for _, hash := range f.hashes {
			receipt, err := a.EvmKeeper.GetReceipt(a.RPCContextProvider(evmrpc.LatestCtxHeight), hash)
			if err != nil {
				return false
			}
			if receipt.Status != 1 || len(receipt.Logs) != 1 {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond)
	require.NoError(t, store.SetLatestVersion(2))
	require.NoError(t, store.SetEarliestVersion(1))
	wm := evmrpc.NewWatermarkManager(consensus, a.RPCContextProvider, nil, store)
	config := rpcconfig.DefaultConfig
	config.EnableParallelizedBlockTrace = true
	config.MaxConcurrentTraceCalls = 1
	config.TraceTimeout = 10 * time.Second
	f.api = evmrpc.NewDebugAPI(consensus, &a.EvmKeeper, a.BeginBlockKeepers, a.RPCContextProvider,
		func(int64) client.TxConfig { return a.GetTxConfig() }, &evmrpc.SimulateConfig{GasCap: 10000000, EVMTimeout: 5 * time.Second}, a.BaseApp, a.TracerAnteHandler,
		evmrpc.ConnectionTypeHTTP, config, evmrpc.NewBlockCache(3000), &sync.Mutex{}, wm)
	return f
}

func TestRepairM_1292_ProfiledDefaultTracer(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprintf("transactions_%d", count), func(t *testing.T) {
			f := newRepairM1292TraceFixture(t, count)
			evmrpc.AssertProfiledDefaultExecution(t, f.api, rpc.BlockNumber(2), common.BytesToHash(f.block.BlockID.Hash), f.hashes)
		})
	}
}

func TestRepairM_1292_ProfiledParentCancel(t *testing.T) {
	f := newRepairM1292TraceFixture(t, 1)
	evmrpc.AssertProfiledParentCancellation(t, f.api)
	// The very next real routed trace must execute successfully, not just acquire.
	evmrpc.AssertProfiledDefaultExecution(t, f.api, rpc.BlockNumber(2), common.BytesToHash(f.block.BlockID.Hash), f.hashes)
}

func TestRepairM_1292_TenantWSReads(t *testing.T) {
	t.Run("ordinary_reads_reconnect", TestChainPathOrdinaryWSReadsMatchHTTPAfterReconnect)
	t.Run("shared_heads", TestChainPathNewHeadsIsUnchanged)
}

func TestRepairM_1292_FilterCases(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"eth_logs", TestFilterGetLogs}, {"pax_logs", TestFilterPaxGetLogs},
		{"mixed_synthetic", TestFilterEthEndpointReturnsNormalEvmLogEvenIfSyntheticLogIsInSameBlock},
		{"filter_logs", TestFilterGetFilterLogs}, {"filter_cursor", TestFilterGetFilterChanges},
		{"canonical_hash", TestGetLogsBlockHashIsNotZero},
	} {
		t.Run(test.name, test.run)
	}
}

func repairM1292LegacyServer(t *testing.T, allowed []string) (*evmrpc.HTTPServer, string) {
	t.Helper()
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8).WithChainID("hyperpax_125-1")
	operator := common.HexToAddress("0x1292")
	params := a.LayerXChainsKeeper.GetParams(ctx)
	coins := sdk.NewCoins(sdk.NewCoin(a.EvmKeeper.GetBaseDenom(ctx), params.MinBond.Add(params.CreationFee)))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", sdk.AccAddress(operator.Bytes()), coins))
	h := &domainScopeHarness{app: a, ctx: ctx, operator: operator}
	h.create(t, "legacy-repair")
	config := rpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.IPRateLimitRPS = 0
	config.EnabledLegacyPaxApis = allowed
	config.FilterTimeout = time.Minute
	provider := func(int64) client.TxConfig { return TxConfig }
	service, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp, h.app.TracerAnteHandler,
		func(height int64) sdk.Context {
			if height == evmrpc.LatestCtxHeight {
				return h.ctx
			}
			return h.ctx.WithBlockHeight(height)
		}, provider, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := httptest.NewServer(service.(http.Handler))
	t.Cleanup(server.Close)
	return service.(*evmrpc.HTTPServer), server.URL
}

func repairM1292Post(t *testing.T, url string, body []byte, chunked bool) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	if chunked {
		request.ContentLength = -1
		request.TransferEncoding = []string{"chunked"}
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, raw
}

func TestRepairM_1292_LegacyBodyLimit(t *testing.T) {
	server, url := repairM1292LegacyServer(t, []string{"pax_newFilter"})
	const cap = 5 * 1024 * 1024
	prefix := []byte(`{"jsonrpc":"2.0","id":1,"method":"pax_newFilter","params":[{}]}`)
	for _, path := range []string{"/", "/chain/1"} {
		for _, chunked := range []bool{false, true} {
			body := append(bytes.Clone(prefix), bytes.Repeat([]byte(" "), cap+1-len(prefix))...)
			status, raw := repairM1292Post(t, url+path, body, chunked)
			require.Equal(t, http.StatusRequestEntityTooLarge, status)
			require.Contains(t, string(raw), "content length too large")
			evmrpc.AssertLegacyHTTPFilterCount(t, server, 0)
		}
	}
	// The exact cap is still valid and the allowlisted producer executes.
	body := append(bytes.Clone(prefix), bytes.Repeat([]byte(" "), cap-len(prefix))...)
	status, raw := repairM1292Post(t, url, body, false)
	require.Equal(t, http.StatusOK, status)
	var reply struct {
		Result rpc.ID
		Error  any
	}
	require.NoError(t, json.Unmarshal(raw, &reply))
	require.Nil(t, reply.Error)
	require.NotEmpty(t, reply.Result)
	evmrpc.AssertLegacyHTTPFilterCount(t, server, 1)
}

func TestRepairM_1292_LegacyNotification(t *testing.T) {
	server, url := repairM1292LegacyServer(t, []string{})
	for _, path := range []string{"/", "/chain/1"} {
		for _, body := range []string{
			`{"jsonrpc":"2.0","method":"pax_newFilter","params":[{}]}`,
			`[{"jsonrpc":"2.0","method":"pax_newFilter","params":[{}]}]`,
		} {
			status, raw := repairM1292Post(t, url+path, []byte(body), false)
			require.Equal(t, http.StatusOK, status)
			require.Empty(t, raw)
		}
		for _, id := range []string{`7`, `"request"`, `null`} {
			body := `{"jsonrpc":"2.0","method":"pax_newFilter","params":[{}],"id":` + id + `}`
			status, raw := repairM1292Post(t, url+path, []byte(body), false)
			require.Equal(t, http.StatusOK, status)
			var reply struct {
				ID    json.RawMessage
				Error struct {
					Code int
					Data string
				}
			}
			require.NoError(t, json.Unmarshal(raw, &reply))
			require.JSONEq(t, id, string(reply.ID))
			require.Equal(t, -32601, reply.Error.Code)
			require.Equal(t, "legacy_pax_deprecated", reply.Error.Data)
		}
		for _, body := range []string{
			`{"method":"pax_newFilter","params":[{}]}`,
			`{"jsonrpc":"1.0","method":"pax_newFilter"}`,
			`{"jsonrpc":"2.0","method":"pax_newFilter","id":{}}`,
			`{"jsonrpc":"2.0","method":7}`, `null`,
		} {
			status, raw := repairM1292Post(t, url+path, []byte(body), false)
			require.Equal(t, http.StatusOK, status)
			var reply struct{ Error struct{ Code int } }
			require.NoError(t, json.Unmarshal(raw, &reply))
			require.Equal(t, -32600, reply.Error.Code)
		}
		evmrpc.AssertLegacyHTTPFilterCount(t, server, 0)
	}
	// A valid notification sent to the enabled real service creates a filter,
	// proving that the negative assertions above observe an executable method.
	enabled, enabledURL := repairM1292LegacyServer(t, []string{"pax_newFilter"})
	status, raw := repairM1292Post(t, enabledURL, []byte(`{"jsonrpc":"2.0","method":"pax_newFilter","params":[{}]}`), false)
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, raw)
	evmrpc.AssertLegacyHTTPFilterCount(t, enabled, 1)
}

func TestRepairM_1292_RPCInstructions(t *testing.T) {
	doc, err := os.ReadFile("AGENTS.md")
	require.NoError(t, err)
	text := strings.Join(strings.Fields(string(doc)), " ")
	for _, claim := range []string{"idle", "PendingSnapshot", "receipt log feed", "ICS23", "null", "zero", "debug_getRawTransaction", "not registered", "execution timeout", "repair_m1292_test.go"} {
		require.Contains(t, text, claim)
	}
	f := newRepairM1292TraceFixture(t, 1)
	config := rpcconfig.DefaultConfig
	config.IPRateLimitRPS = 0
	config.HTTPPort = 0
	service, err := evmrpc.NewEVMHTTPServer(config, f.consensus, &f.app.EvmKeeper, f.app.BeginBlockKeepers, f.app.BaseApp, f.app.TracerAnteHandler,
		f.app.RPCContextProvider, func(int64) client.TxConfig { return f.app.GetTxConfig() }, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := httptest.NewServer(service.(http.Handler))
	t.Cleanup(server.Close)
	consumer, err := rpc.DialHTTP(server.URL)
	require.NoError(t, err)
	t.Cleanup(consumer.Close)
	ctx := context.Background()
	hash := common.BytesToHash(f.block.BlockID.Hash)
	for _, call := range []struct {
		method string
		args   []any
		want   string
	}{
		{"eth_getUncleByBlockNumberAndIndex", []any{"0x2", "0x0"}, "null"},
		{"eth_getUncleByBlockHashAndIndex", []any{hash, "0x0"}, "null"},
		{"eth_getUncleCountByBlockNumber", []any{"0x2"}, `"0x0"`},
		{"eth_getUncleCountByBlockHash", []any{hash}, `"0x0"`},
		{"eth_getUncleCountByBlockNumber", []any{"0xffff"}, "null"},
	} {
		var raw json.RawMessage
		require.NoError(t, consumer.CallContext(ctx, &raw, call.method, call.args...))
		require.JSONEq(t, call.want, string(raw))
	}
	// The committed transcript returns a transport error for unknown hash keys.
	// The existing null-block transcript records consensus's missing-block answer.
	missingPeer := &MockClient{}
	missingWatermarks := evmrpc.NewWatermarkManager(missingPeer, f.app.RPCContextProvider, nil, f.app.EvmKeeper.ReceiptStore())
	missingConsumer, _ := repairM1147HTTP(t, []rpc.API{{Namespace: "eth", Service: evmrpc.NewUncleAPI(missingPeer, missingWatermarks, evmrpc.ConnectionTypeHTTP), Public: true}}, nil)
	var missing json.RawMessage
	require.NoError(t, missingConsumer.CallContext(ctx, &missing, "eth_getUncleCountByBlockHash", common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")))
	require.JSONEq(t, "null", string(missing))
	var envelope hexutil.Bytes
	require.NoError(t, consumer.CallContext(ctx, &envelope, "debug_getRawTransaction", f.hashes[0]))
	var tx ethtypes.Transaction
	require.NoError(t, tx.UnmarshalBinary(envelope))
	require.Equal(t, f.hashes[0], tx.Hash())
	for _, call := range []struct {
		method string
		args   []any
		code   int
	}{
		{"eth_mining", nil, -32601}, {"eth_hashrate", nil, -32601}, {"eth_blobBaseFee", nil, -32000},
		{"debug_getRawHeader", []any{"0x2"}, -32000}, {"debug_getRawBlock", []any{"0x2"}, -32000},
		{"debug_getRawReceipts", []any{"0x2"}, -32000}, {"eth_newPendingTransactionFilter", nil, -32000},
		{"eth_getProof", []any{common.HexToAddress("0x1460"), []string{"0x0"}, "pending"}, -32000},
	} {
		var result any
		err := consumer.CallContext(ctx, &result, call.method, call.args...)
		var wire rpc.Error
		require.ErrorAs(t, err, &wire, call.method)
		require.Equal(t, call.code, wire.ErrorCode(), call.method)
	}
	var proof struct {
		Address      common.Address
		HexValues    []string
		StorageProof []struct{ Ops []struct{ Type string } }
	}
	require.NoError(t, consumer.CallContext(ctx, &proof, "eth_getProof", common.HexToAddress("0x1460"), []string{"0x0"}, "0x2"))
	require.Equal(t, common.HexToAddress("0x1460"), proof.Address)
	require.Len(t, proof.HexValues, 1)
	require.Len(t, proof.StorageProof, 1)
	require.NotEmpty(t, proof.StorageProof[0].Ops)
	for _, op := range proof.StorageProof[0].Ops {
		require.Contains(t, op.Type, "ics23")
	}
	// These focused producer/consumer cases cover configured pending capability,
	// idle fallback, explicit refusal without the executor and feed advancement.
	t.Run("pending_consumer", TestRepairM_1147_PendingConsumerPolicy)
	t.Run("pending_open_idle", TestRepairM_1147_AccessListProposerOpenIdle)
	t.Run("pending_receipt_feed", TestPendingFilterPendingTransactions)
}
