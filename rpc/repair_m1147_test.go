package evmrpc_test

// Task 22.33.1147 repair record (audit 22.30.52):
// rpc-agents-pending-policy-stale: TestRepairM_1147_PendingConsumerPolicy.
// rpc-readme-pending-contradiction: TestRepairM_1147_PendingConsumerPolicy and
// TestRepairM_1147_AccessListProposerOpenIdle.
// rpc-readme-recommends-deprecated-api: TestRepairM_1147_LegacyMigration.
// access-list-omitted-tag-full-node: TestRepairM_1147_AccessListFullNode and
// TestRepairM_1147_AccessListProposerOpenIdle.
// Baseline verification was not run: the controlling gate order requires full
// implementation before hf-build/hf-done and prohibits a manual verify run.
// No pre-change test failures are claimed.
// Wiring request: rpc/pending_simulate_test.go, owning domain 7 (rpc-api), update
// TestPendingSimulateRefusedOffSequencer to require success for omitted access-list
// tags (direct and HTTP) and retain refusal assertions with explicit pending tags.
// The existing pending fixtures execute signed transactions through the real App,
// executor, keeper and stores; their committed consensus fixtures are peers of the
// production simulation/state/block handlers exercised here over loopback HTTP.

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func repairM1147HTTP(t *testing.T, apis []rpc.API, enabled []string) (*rpc.Client, string) {
	t.Helper()
	handler := evmrpc.NewHTTPServer(rpc.HTTPTimeouts{})
	var modules []string
	seen := make(map[string]bool)
	for _, api := range apis {
		if !seen[api.Namespace] {
			modules = append(modules, api.Namespace)
			seen[api.Namespace] = true
		}
	}
	require.NoError(t, handler.EnableRPC(apis, evmrpc.HTTPConfig{
		Modules: modules, Vhosts: []string{"*"},
		PaxLegacyAllowlist: evmrpc.BuildPaxLegacyEnabledSet(enabled),
	}))
	t.Cleanup(handler.Stop)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	consumer, err := rpc.DialHTTP(server.URL)
	require.NoError(t, err)
	t.Cleanup(consumer.Close)
	return consumer, server.URL
}

func repairM1147APIs(n pendingSimulateNode) []rpc.API {
	return []rpc.API{
		{Namespace: "eth", Service: n.sim, Public: true},
		{Namespace: "eth", Service: n.state, Public: true},
		{Namespace: "eth", Service: n.block, Public: true},
	}
}

func repairM1147AccessList(t *testing.T, consumer *rpc.Client, args any, tags ...any) *evmrpc.AccessListResult {
	t.Helper()
	params := append([]any{args}, tags...)
	var result *evmrpc.AccessListResult
	require.NoError(t, consumer.CallContext(context.Background(), &result, "eth_createAccessList", params...))
	require.NotNil(t, result)
	require.NotNil(t, result.Accesslist)
	require.Empty(t, result.Error)
	return result
}

func repairM1147Docs(t *testing.T, path string, claims ...string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	// Markdown line wrapping is irrelevant to the documented contract.
	doc := strings.Join(strings.Fields(string(contents)), " ")
	for _, claim := range claims {
		require.Contains(t, doc, claim, path)
	}
}

func TestRepairM_1147_AccessListFullNode(t *testing.T) {
	sender := utils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	wm := evmrpc.NewWatermarkManager(&MockClient{latestOverride: s.committed}, s.ctxProvider, nil, nil)
	n := pendingSimulateAPIs(s, wm)
	consumer, _ := repairM1147HTTP(t, repairM1147APIs(n), nil)
	args := pendingSimulateArgs(sender.EvmAddress, utils.NewSigner().EvmAddress, big.NewInt(0), nil)
	latest := repairM1147AccessList(t, consumer, args, "latest")
	require.Equal(t, hexutil.Uint64(pendingTransferGas), latest.GasUsed)
	require.Empty(t, *latest.Accesslist)
	for _, tags := range [][]any{nil, {nil}, {"0x1"}, {"safe"}, {"finalized"}} {
		require.Equal(t, latest, repairM1147AccessList(t, consumer, args, tags...))
	}
	var result *evmrpc.AccessListResult
	err := consumer.CallContext(context.Background(), &result, "eth_createAccessList", args, "pending")
	require.Nil(t, result)
	var wireError rpc.Error
	require.ErrorAs(t, err, &wireError)
	require.Equal(t, -32000, wireError.ErrorCode())
	require.EqualError(t, err, "pending served by sequencer")
	// The same role-aware policy is already used by call and gas estimation.
	var gas hexutil.Uint64
	require.NoError(t, consumer.CallContext(context.Background(), &gas, "eth_estimateGas", args))
	require.Equal(t, latest.GasUsed, gas)
	var output hexutil.Bytes
	require.NoError(t, consumer.CallContext(context.Background(), &output, "eth_call", args))
	require.Empty(t, output)
}

func TestRepairM_1147_AccessListProposerOpenIdle(t *testing.T) {
	sender := utils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	s.wm.SetPendingOpenWait(20 * time.Millisecond)
	n := pendingSimulateAPIs(s, s.wm)
	consumer, _ := repairM1147HTTP(t, repairM1147APIs(n), nil)
	creation, set, get := pendingSimulateStorage(t)
	contract := crypto.CreateAddress(sender.EvmAddress, 0)
	args := pendingSimulateArgs(sender.EvmAddress, contract, big.NewInt(0), set)
	getArgs := pendingSimulateArgs(sender.EvmAddress, contract, big.NewInt(0), get)
	pendingSimulateAccept(t, s.exec, pendingSimulateTx(t, sender, 0, nil, big.NewInt(0), creation, pendingSimulateDeployGas))

	open := repairM1147AccessList(t, consumer, args)
	require.Equal(t, ethtypes.AccessList{{Address: contract, StorageKeys: []common.Hash{{}}}}, *open.Accesslist)
	require.GreaterOrEqual(t, uint64(open.GasUsed), uint64(pendingSimulateSetFloorGas))
	require.Equal(t, open, repairM1147AccessList(t, consumer, args, nil))
	require.Equal(t, open, repairM1147AccessList(t, consumer, args, "pending"))
	latest := repairM1147AccessList(t, consumer, args, "latest")
	require.Empty(t, *latest.Accesslist)
	require.Less(t, latest.GasUsed, open.GasUsed)
	var value hexutil.Bytes
	require.NoError(t, consumer.CallContext(context.Background(), &value, "eth_call", getArgs))
	require.Equal(t, pendingSimulateWord(0), value, "access-list execution must not mutate open state")
	pendingSimulateAccept(t, s.exec, pendingSimulateTx(t, sender, 1, &contract, big.NewInt(0), set, pendingSimulateSetGas))
	require.NoError(t, consumer.CallContext(context.Background(), &value, "eth_call", getArgs))
	require.Equal(t, pendingSimulateWord(pendingSimulateStored), value)
	require.NoError(t, consumer.CallContext(context.Background(), &value, "eth_getStorageAt", contract, "0x0", "pending"))
	require.Equal(t, pendingSimulateWord(pendingSimulateStored), value)
	require.NoError(t, consumer.CallContext(context.Background(), &value, "eth_call", getArgs, "latest"))
	require.Empty(t, value)

	// Sealing leaves the provider wired but idle. Omission must retain the
	// bounded pending policy and then use the committed state, without committing
	// the uncommitted deployment or its storage write.
	_, err := s.exec.Seal(s.committed + 1)
	require.NoError(t, err)
	require.Equal(t, latest, repairM1147AccessList(t, consumer, args))
	require.Equal(t, latest, repairM1147AccessList(t, consumer, args, "pending"))
	require.NoError(t, consumer.CallContext(context.Background(), &value, "eth_getCode", contract, "pending"))
	require.Empty(t, value)
	var block map[string]any
	err = consumer.CallContext(context.Background(), &block, "eth_getBlockByNumber", "pending", false)
	require.ErrorContains(t, err, "no block is open")

	// Opening again restores open-state simulation; provider capability was not
	// lost during the idle interval.
	require.NoError(t, s.exec.Open(s.committed+1, time.Now().UTC().UnixMilli()))
	pendingSimulateAccept(t, s.exec, pendingSimulateTx(t, sender, 0, nil, big.NewInt(0), creation, pendingSimulateDeployGas))
	require.Equal(t, open, repairM1147AccessList(t, consumer, args))
	require.Equal(t, latest, repairM1147AccessList(t, consumer, args, "latest"))
}

func TestRepairM_1147_PendingConsumerPolicy(t *testing.T) {
	repairM1147Docs(t, "AGENTS.md", "Proposer capability", "idle", "request cancellation",
		"pending served by sequencer", "repair_m1147_test.go", "pending_simulate_test.go")
	repairM1147Docs(t, "README.md", "Select `latest` for committed state", "Omitting the block tag (or passing null)",
		"Cancellation ends the wait with the request error", "eth_createAccessList", "repair_m1147_test.go")
	sender := utils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	full := pendingSimulateAPIs(s, evmrpc.NewWatermarkManager(&MockClient{latestOverride: s.committed}, s.ctxProvider, nil, nil))
	consumer, _ := repairM1147HTTP(t, repairM1147APIs(full), nil)
	args := pendingSimulateArgs(sender.EvmAddress, utils.NewSigner().EvmAddress, big.NewInt(0), nil)
	for _, request := range []struct {
		method string
		params []any
	}{
		{"eth_getBalance", []any{sender.EvmAddress, "pending"}},
		{"eth_getCode", []any{sender.EvmAddress, "pending"}},
		{"eth_getStorageAt", []any{sender.EvmAddress, "0x0", "pending"}},
		{"eth_call", []any{args, "pending"}},
		{"eth_estimateGas", []any{args, "pending"}},
		{"eth_createAccessList", []any{args, "pending"}},
		{"eth_getBlockByNumber", []any{"pending", false}},
	} {
		t.Run(request.method, func(t *testing.T) {
			var result any
			err := consumer.CallContext(context.Background(), &result, request.method, request.params...)
			require.Nil(t, result)
			var wireError rpc.Error
			require.ErrorAs(t, err, &wireError)
			require.Equal(t, -32000, wireError.ErrorCode())
			require.EqualError(t, err, "pending served by sequencer")
		})
	}
	var count *hexutil.Uint
	require.NoError(t, consumer.CallContext(context.Background(), &count, "eth_getBlockTransactionCountByNumber", "pending"))
	require.Nil(t, count, "full-node pending block counts return null")
	for _, node := range []pendingSimulateNode{full, pendingSimulateAPIs(s, s.wm)} {
		local, _ := repairM1147HTTP(t, repairM1147APIs(node), nil)
		var proof any
		err := local.CallContext(context.Background(), &proof, "eth_getProof", sender.EvmAddress, []string{}, "pending")
		require.EqualError(t, err, "pending state has no committed root")
		var wireError rpc.Error
		require.ErrorAs(t, err, &wireError)
		require.Equal(t, -32000, wireError.ErrorCode())
	}
	_, err := s.exec.Seal(s.committed + 1)
	require.NoError(t, err)
	proposer := pendingSimulateAPIs(s, s.wm)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = proposer.sim.CreateAccessList(cancelled, args, nil)
	require.ErrorIs(t, err, context.Canceled)
	_, err = proposer.state.GetBalance(cancelled, sender.EvmAddress, pendingSimulateTag)
	require.ErrorIs(t, err, context.Canceled)
	deadline, finish := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer finish()
	_, err = proposer.sim.CreateAccessList(deadline, args, nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRepairM_1147_LegacyMigration(t *testing.T) {
	repairM1147Docs(t, "README.md", "### Migration by method", "legacy-only", "eth_getBlockByNumber",
		"eth_getBlockReceipts", "eth_getLogs", "debug_traceBlockByNumber", "debug_traceTransaction",
		"No direct standard Ethereum equivalent", "pax_sequencerReceipt", "legacy_pax_deprecated",
		"Pax-Legacy-RPC-Deprecation", "Cosmos-originating", "EVM-originating")
	sender := utils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	n := pendingSimulateAPIs(s, s.wm)
	recipient := utils.NewSigner().EvmAddress
	txBytes := pendingTransfer(t, sender, 0, recipient, big.NewInt(1))
	pendingSimulateAccept(t, s.exec, txBytes)
	hash := pendingBlockEthHash(t, s, txBytes)
	txConfigProvider := func(int64) client.TxConfig { return s.app.GetTxConfig() }
	tm := &MockClient{latestOverride: s.committed}
	apis := repairM1147APIs(n)
	apis = append(apis,
		rpc.API{Namespace: "pax", Service: evmrpc.NewPaxBlockAPI(tm, &s.app.EvmKeeper, s.ctxProvider, txConfigProvider,
			evmrpc.ConnectionTypeHTTP, s.wm, evmrpc.NewBlockCache(10), &sync.Mutex{}), Public: true},
		rpc.API{Namespace: "pax2", Service: evmrpc.NewPax2BlockAPI(tm, &s.app.EvmKeeper, s.ctxProvider, txConfigProvider,
			evmrpc.ConnectionTypeHTTP, s.wm, evmrpc.NewBlockCache(10), &sync.Mutex{}), Public: true})
	methods := []string{"pax_getBlockByNumber", "pax2_getBlockByNumber"}
	disabled, _ := repairM1147HTTP(t, apis, nil)
	allowed, allowedURL := repairM1147HTTP(t, apis, methods)
	for _, method := range methods {
		var result map[string]any
		err := disabled.CallContext(context.Background(), &result, method, "pending", false)
		var wireError rpc.Error
		require.ErrorAs(t, err, &wireError)
		require.Equal(t, -32601, wireError.ErrorCode())
		var dataError rpc.DataError
		require.ErrorAs(t, err, &dataError)
		require.Equal(t, "legacy_pax_deprecated", dataError.ErrorData())
		require.ErrorContains(t, err, "should not be used for new integrations")
		require.NoError(t, allowed.CallContext(context.Background(), &result, method, "pending", false))
		require.Equal(t, "0x2", result["number"])
		require.Equal(t, []any{hash.Hex()}, result["transactions"])
		var standard map[string]any
		require.NoError(t, disabled.CallContext(context.Background(), &standard, "eth_getBlockByNumber", "pending", false))
		require.Equal(t, standard, result, "EVM-originating transaction remains visible in the supported view")
		request, err := http.NewRequest(http.MethodPost, allowedURL, strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":["pending",false]}`))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, evmrpc.PaxLegacyDeprecationMessage, response.Header.Get(evmrpc.PaxLegacyDeprecationHTTPHeader))
		require.NoError(t, response.Body.Close())
	}
}
