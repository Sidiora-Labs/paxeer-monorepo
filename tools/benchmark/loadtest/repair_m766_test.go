package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/semaphore"
)

type m766Request struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func m766Reset(t *testing.T) {
	t.Helper()
	oldProduced, oldSent, oldPrev := producedCountPerMsgType, sentCountPerMsgType, prevSentCounterPerMsgType
	oldHeights, oldHashes := BlockHeightsWithTxs, EvmTxHashes
	producedCountPerMsgType = map[string]*int64{EVM: new(int64)}
	sentCountPerMsgType = map[string]*int64{EVM: new(int64)}
	prevSentCounterPerMsgType = map[string]*int64{EVM: new(int64)}
	BlockHeightsWithTxs = nil
	EvmTxHashes = newEvmTxHashStore(maxRetainedEvmTxHashes)
	t.Cleanup(func() {
		producedCountPerMsgType, sentCountPerMsgType, prevSentCounterPerMsgType = oldProduced, oldSent, oldPrev
		BlockHeightsWithTxs, EvmTxHashes = oldHeights, oldHashes
	})
}

func m766WriteRPC(w http.ResponseWriter, req m766Request, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func m766Await(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("benchmark did not join participants and reporter")
		return nil
	}
}

func m766Block(w http.ResponseWriter, height string) {
	fmt.Fprintf(w, `{"result":{"block":{"header":{"height":%q,"time":"2026-01-01T00:00:00Z"},"data":{"txs":["dHg="]}}}}`, height)
}

type m766Runtime struct {
	config     Config
	client     *LoadTestClient
	entered    chan struct{}
	submitted  atomic.Int64
	queryMu    sync.Mutex
	queryCalls map[string]int
	accepted   map[common.Hash]bool
	failBlock  atomic.Bool
	stall      bool
	release    chan struct{}
	server     *httptest.Server
}

func m766NewRuntime(t *testing.T, stall bool) *m766Runtime {
	t.Helper()
	fixture := &m766Runtime{entered: make(chan struct{}, 128), stall: stall, release: make(chan struct{}), queryCalls: map[string]int{}, accepted: map[common.Hash]bool{}}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/blockchain":
			fmt.Fprint(w, `{"result":{"last_height":"10"}}`)
			return
		case "/block":
			if fixture.failBlock.Load() {
				http.Error(w, "scrape unavailable", http.StatusServiceUnavailable)
				return
			}
			m766Block(w, r.URL.Query().Get("height"))
			return
		}
		var req m766Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		switch req.Method {
		case "eth_getTransactionCount":
			m766WriteRPC(w, req, "0x0")
		case "eth_getTransactionByHash":
			m766WriteRPC(w, req, nil)
		case "net_version":
			m766WriteRPC(w, req, "125")
		case "eth_sendRawTransaction":
			var raw string
			if len(req.Params) != 1 || json.Unmarshal(req.Params[0], &raw) != nil {
				t.Error("missing raw signed transaction")
				return
			}
			encoded, err := hex.DecodeString(strings.TrimPrefix(raw, "0x"))
			if err != nil {
				t.Error(err)
				return
			}
			var tx ethtypes.Transaction
			if err := tx.UnmarshalBinary(encoded); err != nil {
				t.Error(err)
				return
			}
			if _, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), &tx); err != nil {
				t.Error(err)
				return
			}
			select {
			case fixture.entered <- struct{}{}:
			default:
			}
			if fixture.stall {
				select {
				case <-r.Context().Done():
					return
				case <-fixture.release:
					return
				}
			}
			fixture.queryMu.Lock()
			fixture.accepted[tx.Hash()] = true
			fixture.queryMu.Unlock()
			fixture.submitted.Add(1)
			m766WriteRPC(w, req, tx.Hash())
		case "eth_getBlockByNumber":
			fixture.queryMu.Lock()
			fixture.queryCalls[req.Method]++
			fixture.queryMu.Unlock()
			if len(req.Params) != 2 || string(req.Params[0]) != `"0xa"` || string(req.Params[1]) != "true" {
				t.Errorf("block query did not use committed observation: %s", req.Params)
			}
			header := &ethtypes.Header{Number: big.NewInt(10), Difficulty: big.NewInt(1), GasLimit: 30000000, Time: 1767225600, UncleHash: ethtypes.EmptyUncleHash, TxHash: ethtypes.EmptyTxsHash}
			raw, err := json.Marshal(header)
			if err != nil {
				t.Error(err)
				return
			}
			var block map[string]any
			if err := json.Unmarshal(raw, &block); err != nil {
				t.Error(err)
				return
			}
			block["hash"] = header.Hash()
			block["uncles"] = []string{}
			block["transactions"] = []any{}
			m766WriteRPC(w, req, block)
		case "eth_getTransactionReceipt":
			fixture.queryMu.Lock()
			fixture.queryCalls[req.Method]++
			fixture.queryMu.Unlock()
			var hash common.Hash
			if len(req.Params) != 1 || json.Unmarshal(req.Params[0], &hash) != nil {
				t.Error("missing receipt hash")
				return
			}
			fixture.queryMu.Lock()
			accepted := fixture.accepted[hash]
			fixture.queryMu.Unlock()
			if !accepted {
				t.Errorf("receipt queried an unsubmitted hash %s", hash)
			}
			m766WriteRPC(w, req, &ethtypes.Receipt{Status: 1, CumulativeGasUsed: 21000, GasUsed: 21000, TxHash: hash, BlockNumber: big.NewInt(10), Logs: []*ethtypes.Log{}})
		case "eth_getLogs":
			fixture.queryMu.Lock()
			fixture.queryCalls[req.Method]++
			fixture.queryMu.Unlock()
			m766WriteRPC(w, req, []*ethtypes.Log{})
		default:
			t.Errorf("unexpected RPC method %s", req.Method)
			http.Error(w, "unexpected method", 500)
		}
	}))
	t.Cleanup(fixture.server.Close)
	t.Cleanup(func() { close(fixture.release) })
	key := secp256k1.GenPrivKey()
	eth, err := ethclient.Dial(fixture.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eth.Close)
	fixture.config = Config{TargetTps: 1, Ticks: 1, MessageTypes: []string{EVM}, BlockchainEndpoint: fixture.server.URL, EvmRpcEndpoints: fixture.server.URL}
	fixture.client = &LoadTestClient{LoadTestConfig: fixture.config, AccountKeys: []cryptotypes.PrivKey{key},
		EvmTxClients:          []*EvmTxClient{NewEvmTxClient(key, big.NewInt(125), big.NewInt(1), []*ethclient.Client{eth}, nil, false)},
		isAdminMessageMapping: map[string]bool{}, mtx: &sync.RWMutex{}}
	return fixture
}

func TestRepairM_766_Participant(t *testing.T) {
	m766Reset(t)
	fixture := m766NewRuntime(t, false)
	ticks := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- runLoadtestWorkers(ctx, fixture.client, fixture.config, ticks) }()
	select {
	case <-fixture.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real signed producer transaction never reached RPC")
	}
	deadline := time.After(2 * time.Second)
waitForAccepted:
	for atomic.LoadInt64(producedCountPerMsgType[EVM]) < 2 || atomic.LoadInt64(sentCountPerMsgType[EVM]) < 2 {
		select {
		case <-deadline:
			t.Fatal("sender did not record successful submission")
		case <-time.After(time.Millisecond):
			continue waitForAccepted
		}
	}
	ticks <- time.Now()
	if err := m766Await(t, result); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(BlockHeightsWithTxs, []int{10}) {
		t.Fatalf("reporter not joined: heights=%v", BlockHeightsWithTxs)
	}
	if fixture.submitted.Load() == 0 || atomic.LoadInt64(sentCountPerMsgType[EVM]) == 0 || atomic.LoadInt64(producedCountPerMsgType[EVM]) == 0 {
		t.Fatal("worker counters were lost")
	}
	if len(EvmTxHashes.Snapshot()) == 0 || int64(len(EvmTxHashes.Snapshot())) != fixture.submitted.Load() {
		t.Fatalf("successful transaction observations=%d, RPC submissions=%d", len(EvmTxHashes.Snapshot()), fixture.submitted.Load())
	}
	beforeSent, beforeProduced := atomic.LoadInt64(sentCountPerMsgType[EVM]), atomic.LoadInt64(producedCountPerMsgType[EVM])
	time.Sleep(20 * time.Millisecond)
	if beforeSent != atomic.LoadInt64(sentCountPerMsgType[EVM]) || beforeProduced != atomic.LoadInt64(producedCountPerMsgType[EVM]) {
		t.Fatal("workers still active after return")
	}
	queue := make(chan SignedTx)
	close(queue)
	if err := fixture.client.SendTxs(context.Background(), queue, 0, map[string]*int64{}, semaphore.NewWeighted(1)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		immediate := make(chan time.Time, 1)
		immediate <- time.Now()
		if err := runLoadtestWorkers(ctx, fixture.client, fixture.config, immediate); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepairM_766_ScrapeCancellation(t *testing.T) {
	m766Reset(t)
	fixture := m766NewRuntime(t, true)
	ticks := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- runLoadtestWorkers(ctx, fixture.client, fixture.config, ticks) }()
	select {
	case <-fixture.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("sender never entered RPC")
	}
	fixture.failBlock.Store(true)
	ticks <- time.Now()
	err := m766Await(t, result)
	if err == nil || !strings.Contains(err.Error(), "height 10") || !strings.Contains(err.Error(), "503") {
		t.Fatalf("missing detailed scrape failure: %v", err)
	}
	if len(EvmTxHashes.Snapshot()) != 0 {
		t.Fatal("failed or cancelled submission invented transaction observations")
	}
	bad := fixture.config
	bad.BlockchainEndpoint = ":bad-endpoint"
	if err := runLoadtestWorkers(ctx, fixture.client, bad, ticks); err == nil || !strings.Contains(err.Error(), "startup height") {
		t.Fatalf("missing startup failure: %v", err)
	}
}

func TestRepairM_766_Signals(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			m766Reset(t)
			fixture := m766NewRuntime(t, true)
			result := make(chan error, 1)
			go func() { result <- startLoadtestWorkers(fixture.client, fixture.config) }()
			select {
			case <-fixture.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("signal handler run did not start")
			}
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
			if err := m766Await(t, result); !errors.Is(err, context.Canceled) {
				t.Fatalf("signal did not cancel positive-tick benchmark: %v", err)
			}
		})
	}
}

func TestRepairM_766_DuplicateStatisticsHeight(t *testing.T) {
	m766Reset(t)
	var height atomic.Int64
	height.Store(10)
	var fail atomic.Bool
	var mu sync.Mutex
	var requested []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blockchain" {
			fmt.Fprintf(w, `{"last_height":%q}`, strconv.FormatInt(height.Load(), 10))
			return
		}
		h, _ := strconv.Atoi(r.URL.Query().Get("height"))
		mu.Lock()
		requested = append(requested, h)
		mu.Unlock()
		if fail.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		m766Block(w, strconv.Itoa(h))
	}))
	defer server.Close()
	next := 10
	first, _, err := scrapeStatistics(context.Background(), server.URL, &next)
	if err != nil || !reflect.DeepEqual(first, []int{10}) || next != 11 {
		t.Fatalf("first interval: %v next=%d err=%v", first, next, err)
	}
	second, _, err := scrapeStatistics(context.Background(), server.URL, &next)
	if err != nil || len(second) != 0 || next != 11 {
		t.Fatalf("no-new-block interval: %v next=%d err=%v", second, next, err)
	}
	height.Store(12)
	third, _, err := scrapeStatistics(context.Background(), server.URL, &next)
	if err != nil || !reflect.DeepEqual(third, []int{11, 12}) || next != 13 {
		t.Fatalf("next interval: %v next=%d err=%v", third, next, err)
	}
	height.Store(13)
	fail.Store(true)
	if _, _, err := scrapeStatistics(context.Background(), server.URL, &next); err == nil || !strings.Contains(err.Error(), "height 13") || next != 13 {
		t.Fatalf("failed-height reporting: next=%d err=%v", next, err)
	}
	fail.Store(false)
	if _, _, err := scrapeStatistics(context.Background(), server.URL, &next); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(BlockHeightsWithTxs, []int{10, 11, 12, 13}) {
		t.Fatalf("nonunique samples: %v", BlockHeightsWithTxs)
	}
	if !reflect.DeepEqual(requested, []int{10, 11, 12, 13, 13}) {
		t.Fatalf("unexpected requests: %v", requested)
	}
}

func TestRepairM_766_UnboundedRPCInput(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		block      bool
		want       string
	}{
		{"missing height", `{}`, 200, false, "missing last_height"},
		{"height type", `{"last_height":10}`, 200, false, "payload"},
		{"height overflow", `{"last_height":"999999999999999999999999999"}`, 200, false, "invalid last_height"},
		{"negative height", `{"last_height":"-1"}`, 200, false, "invalid last_height"},
		{"null result", `{"result":null}`, 200, false, "missing last_height"},
		{"HTTP error", `{"last_height":"10"}`, 503, false, "HTTP 503"},
		{"RPC error", `{"error":{"code":-32603,"message":"node unavailable"}}`, 200, true, "RPC -32603"},
		{"missing block", `{}`, 200, true, "missing block"},
		{"missing txs", `{"block":{"header":{"height":"10","time":"2026-01-01T00:00:00Z"},"data":{}}}`, 200, true, "missing block"},
		{"wrong height", `{"block":{"header":{"height":"9","time":"2026-01-01T00:00:00Z"},"data":{"txs":[]}}}`, 200, true, "differs from requested"},
		{"invalid timestamp", `{"block":{"header":{"height":"10","time":"bad"},"data":{"txs":[]}}}`, 200, true, "invalid block time"},
		{"wrong txs type", `{"block":{"header":{"height":"10","time":"2026-01-01T00:00:00Z"},"data":{"txs":{}}}}`, 200, true, "invalid block transactions"},
		{"null transaction", `{"block":{"header":{"height":"10","time":"2026-01-01T00:00:00Z"},"data":{"txs":[null]}}}`, 200, true, "invalid empty block transaction"},
		{"malformed", `{"block":`, 200, true, "JSON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			var err error
			if tc.block {
				_, _, err = getTxBlockInfo(context.Background(), server.URL, "10")
			} else {
				_, err = getLastHeight(context.Background(), server.URL)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
	t.Run("size bound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, strings.Repeat(" ", maxBenchmarkResponseBytes+1))
		}))
		defer server.Close()
		if _, err := getLastHeight(context.Background(), server.URL); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("unbounded response accepted: %v", err)
		}
	})
	t.Run("default deadline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		start := time.Now()
		if _, err := getLastHeight(context.Background(), server.URL); err == nil {
			t.Fatal("stalled RPC succeeded")
		}
		if elapsed := time.Since(start); elapsed > benchmarkRPCTimeout+time.Second {
			t.Fatalf("deadline exceeded: %s", elapsed)
		}
	})
	t.Run("shared cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		start := time.Now()
		if _, _, err := getTxBlockInfo(ctx, server.URL, "10"); err == nil {
			t.Fatal("cancelled RPC succeeded")
		}
		if time.Since(start) > time.Second {
			t.Fatal("caller cancellation not propagated")
		}
	})
}

func TestRepairM_766_PostQueryObservations(t *testing.T) {
	m766Reset(t)
	fixture := m766NewRuntime(t, false)
	ticks := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- runLoadtestWorkers(ctx, fixture.client, fixture.config, ticks) }()
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt64(producedCountPerMsgType[EVM]) < 2 || atomic.LoadInt64(sentCountPerMsgType[EVM]) < 2 {
		select {
		case <-deadline:
			t.Fatal("real signed transaction not submitted")
		case <-time.After(time.Millisecond):
		}
	}
	ticks <- time.Now()
	if err := m766Await(t, result); err != nil {
		t.Fatal(err)
	}
	config := fixture.config
	config.EvmRpcEndpoints = "  " + fixture.server.URL + "  "
	config.EVMAddresses = &EVMAddresses{ERC20: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	config.PostTxEvmQueries = PostTxEvmQueries{BlockByNumber: 3, Receipt: 3, Filters: 3}
	if err := runEvmQueries(config); err != nil {
		t.Fatal(err)
	}
	fixture.queryMu.Lock()
	calls := make(map[string]int)
	for method, count := range fixture.queryCalls {
		calls[method] = count
	}
	fixture.queryMu.Unlock()
	if !reflect.DeepEqual(calls, map[string]int{"eth_getBlockByNumber": 3, "eth_getTransactionReceipt": 3, "eth_getLogs": 3}) {
		t.Fatalf("query families not executed: %v", calls)
	}
	for _, endpoint := range []string{"", "  ", fixture.server.URL + ",", ":bad", "ftp://localhost"} {
		bad := config
		bad.EvmRpcEndpoints = endpoint
		if err := runEvmQueries(bad); err == nil {
			t.Fatalf("invalid endpoint %q accepted", endpoint)
		}
	}
	savedHeights, savedHashes := BlockHeightsWithTxs, EvmTxHashes
	BlockHeightsWithTxs = nil
	if err := runEvmQueries(config); err == nil || !strings.Contains(err.Error(), "committed") {
		t.Fatalf("missing heights: %v", err)
	}
	BlockHeightsWithTxs = savedHeights
	EvmTxHashes = newEvmTxHashStore(maxRetainedEvmTxHashes)
	if err := runEvmQueries(config); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("missing hashes: %v", err)
	}
	EvmTxHashes = savedHashes
	noContract := config
	noContract.EVMAddresses = nil
	if err := runEvmQueries(noContract); err == nil || !strings.Contains(err.Error(), "ERC20") {
		t.Fatalf("missing contract: %v", err)
	}
	disabled := Config{}
	if err := runEvmQueries(disabled); err != nil {
		t.Fatalf("disabled query families are not applicable: %v", err)
	}
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "node failed", 503) }))
	failed.Close()
	bad := config
	bad.EvmRpcEndpoints = failed.URL
	if err := runEvmQueries(bad); err == nil || !strings.Contains(err.Error(), "connect EVM endpoint") {
		t.Fatalf("failed connection: %v", err)
	}
}

func TestRepairM_766_PostQueryBoundsAndErrors(t *testing.T) {
	m766Reset(t)
	var active, peak, completed atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req m766Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method == "net_version" {
			m766WriteRPC(w, req, "125")
			return
		}
		current := active.Add(1)
		for old := peak.Load(); current > old; old = peak.Load() {
			if peak.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		completed.Add(1)
		m766WriteRPC(w, req, []*ethtypes.Log{})
	}))
	defer server.Close()
	config := Config{EvmRpcEndpoints: server.URL, EVMAddresses: &EVMAddresses{ERC20: common.HexToAddress("0x1")}, PostTxEvmQueries: PostTxEvmQueries{Filters: postQueryConcurrency * 3}}
	if err := runEvmQueries(config); err != nil {
		t.Fatal(err)
	}
	if peak.Load() > postQueryConcurrency || peak.Load() < 2 || completed.Load() != int64(config.PostTxEvmQueries.Filters) {
		t.Fatalf("query concurrency/count: peak=%d completed=%d", peak.Load(), completed.Load())
	}
	failures := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req m766Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method == "net_version" {
			m766WriteRPC(w, req, "125")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "query unavailable"}})
	}))
	defer failures.Close()
	config.EvmRpcEndpoints = failures.URL
	if err := runEvmQueries(config); err == nil || !strings.Contains(err.Error(), "query unavailable") {
		t.Fatalf("query error was reported as a measurement: %v", err)
	}
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req m766Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method == "net_version" {
			m766WriteRPC(w, req, "125")
			return
		}
		<-r.Context().Done()
	}))
	defer stalled.Close()
	config.EvmRpcEndpoints = stalled.URL
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := runEvmQueriesContext(ctx, config); err == nil {
		t.Fatal("cancelled queries reported success")
	}
	if time.Since(start) > time.Second {
		t.Fatal("query contexts were not cancelled")
	}
}
