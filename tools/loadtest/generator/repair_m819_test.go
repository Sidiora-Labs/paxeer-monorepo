package generator

// Task 22.33.819 repair record:
// prewarm-coverage: TestRepairM_819_PrewarmCoverage,
// TestRepairM_819_FreshOnly, TestRepairM_819_PrewarmReceipts.
// weighted-empty: TestRepairM_819_WeightAdmission,
// TestRepairM_819_ConfigAdmission, TestRepairM_819_CumulativeWeights.
// Baseline tests were not run: the explicit order requires a complete implementation
// before hf-build and hf-done and forbids running verify_cmd manually.
// Wiring request (domain 16): tools/loadtest/sender/dispatcher.go must fail on any
// prewarm send/cancellation error and call PrewarmGenerator.Wait with its RPC client.
// Wiring request (domain 16): tools/loadtest/config/cli/command.go must reset the
// measurement collectors only after successful receipt waiting, before measured traffic.
// Wiring request (domain 16): tools/loadtest/types/account_pool.go should expose
// FixedAccounts for externally constructed pools; config-based pools expose it here.
// Gate revision and private logs are recorded by hf-build/hf-done, not guessed here.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/types"
)

func repair819Config() *config.LoadConfig {
	return &config.LoadConfig{ChainID: 125, Accounts: &config.AccountConfig{Accounts: 3}, Scenarios: []config.Scenario{
		{Name: scenarios.EVMTransfer, Weight: 2},
		{Name: scenarios.EVMTransfer, Weight: 3},
		{Name: scenarios.EVMTransfer, Weight: 1, Accounts: &config.AccountConfig{Accounts: 2, NewAccountRate: 0.999}},
		{Name: scenarios.EVMTransfer, Weight: 1, Accounts: &config.AccountConfig{Accounts: 0, NewAccountRate: 1}},
		{Name: scenarios.EVMTransfer, Weight: 1, Accounts: &config.AccountConfig{Accounts: 1}},
	}}
}

func repair819Generator(t *testing.T, cfg *config.LoadConfig) Generator {
	t.Helper()
	gen, err := NewConfigBasedGenerator(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return gen
}

func repair819Scenario(t *testing.T) Generator {
	t.Helper()
	cfg := &config.LoadConfig{ChainID: 125, Accounts: &config.AccountConfig{Accounts: 1}, Scenarios: []config.Scenario{{Name: scenarios.EVMTransfer, Weight: 1}}}
	return repair819Generator(t, cfg)
}

// Repair819RPC is a loopback protocol peer retaining the real submitted signed transactions.
type Repair819RPC struct {
	mu           sync.Mutex
	txs          map[common.Hash]*ethtypes.Transaction
	mined        bool
	failed       bool
	receiptError bool
	queried      chan struct{}
}

func (r *Repair819RPC) SendRawTransaction(raw hexutil.Bytes) (common.Hash, error) {
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	if tx.ChainId().Cmp(big.NewInt(125)) != 0 {
		return common.Hash{}, fmt.Errorf("unexpected chain ID")
	}
	from, err := ethtypes.Sender(ethtypes.NewCancunSigner(big.NewInt(125)), &tx)
	if err != nil {
		return common.Hash{}, err
	}
	if tx.To() == nil || *tx.To() != from {
		return common.Hash{}, fmt.Errorf("prewarm is not a signed self-transfer")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.txs[tx.Hash()]; exists {
		return common.Hash{}, fmt.Errorf("duplicate transaction")
	}
	r.txs[tx.Hash()] = &tx
	return tx.Hash(), nil
}

func (r *Repair819RPC) GetTransactionReceipt(hash common.Hash) (*ethtypes.Receipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case r.queried <- struct{}{}:
	default:
	}
	if r.receiptError {
		return nil, fmt.Errorf("receipt service unavailable")
	}
	tx := r.txs[hash]
	if tx == nil || !r.mined {
		return nil, nil
	}
	status := uint64(ethtypes.ReceiptStatusSuccessful)
	if r.failed {
		status = ethtypes.ReceiptStatusFailed
	}
	return &ethtypes.Receipt{Type: tx.Type(), Status: status, CumulativeGasUsed: 21000, GasUsed: 21000,
		TxHash: hash, BlockHash: common.HexToHash("0x819"), BlockNumber: big.NewInt(1), Logs: []*ethtypes.Log{}}, nil
}

func repair819Server(t *testing.T) (*Repair819RPC, *httptest.Server, *ethclient.Client) {
	t.Helper()
	peer := &Repair819RPC{txs: make(map[common.Hash]*ethtypes.Transaction), queried: make(chan struct{}, 1)}
	server := rpc.NewServer()
	if err := server.RegisterName("eth", peer); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	client, err := ethclient.Dial(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return peer, httpServer, client
}

func repair819Submit(t *testing.T, url string, tx *types.LoadTx) {
	t.Helper()
	response, err := http.Post(url, "application/json", bytes.NewReader(tx.JSONRPCPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Result common.Hash
		Error  *json.RawMessage
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || result.Error != nil || result.Result != tx.EthTx.Hash() {
		t.Fatalf("submission failed: %s", body)
	}
}

func TestRepairM_819_PrewarmCoverage(t *testing.T) {
	cfg := repair819Config()
	main := repair819Generator(t, cfg)
	pools := main.GetAccountPools()
	if len(pools) != 4 {
		t.Fatalf("got %d pools, want 4 distinct pools", len(pools))
	}
	expected := make(map[common.Address]uint64)
	for i, pool := range pools {
		for _, account := range pool.(FixedAccountPool).FixedAccounts() {
			account.Nonce = uint64(i + 7)
			expected[account.Address] = account.Nonce
		}
	}
	if len(expected) != 6 {
		t.Fatalf("inventory = %d, want 6", len(expected))
	}
	// A second pool exposing the same accounts must not repeat those addresses.
	shared := pools[0].(FixedAccountPool).FixedAccounts()
	duplicatePool, err := newFixedAccountPool(&types.AccountConfig{Accounts: shared})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := NewScenarioGenerator(duplicatePool, scenarios.NewEVMTransferScenario(config.Scenario{}))
	combined, err := NewWeightedGenerator(WeightedConfig(9, main), WeightedConfig(4, duplicate))
	if err != nil {
		t.Fatal(err)
	}
	prewarm := NewPrewarmGenerator(cfg, combined)
	peer, server, client := repair819Server(t)
	txs := prewarm.GenerateN(20)
	if len(txs) != 6 {
		t.Fatalf("got %d prewarm transactions, want 6", len(txs))
	}
	seen := make(map[common.Address]bool)
	for _, tx := range txs {
		address := tx.Scenario.Sender.Address
		nonce, exists := expected[address]
		if !exists || seen[address] || tx.EthTx.Nonce() != nonce {
			t.Fatalf("unexpected/repeated account or nonce: %s/%d", address, tx.EthTx.Nonce())
		}
		seen[address] = true
		repair819Submit(t, server.URL, tx)
	}
	for range 3 {
		if tx, ok := prewarm.Generate(); tx != nil || ok {
			t.Fatal("finite prewarm did not terminate")
		}
	}
	peer.mu.Lock()
	if len(peer.txs) != 6 {
		t.Fatal("transport lost prewarm transactions")
	}
	peer.mined = true
	peer.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := prewarm.Wait(ctx, client); err != nil {
		t.Fatal(err)
	}
	// Prewarming advanced the actual accounts used by the measured generator.
	for _, pool := range pools {
		for _, account := range pool.(FixedAccountPool).FixedAccounts() {
			if account.Nonce != expected[account.Address]+1 {
				t.Fatal("prewarm did not advance the workload account exactly once")
			}
		}
	}
	// Returned inventory/pool slices cannot alter subsequent enumeration.
	pools[0] = nil
	shared[0] = nil
	if main.GetAccountPools()[0] == nil || main.GetAccountPools()[0].(FixedAccountPool).FixedAccounts()[0] == nil {
		t.Fatal("inventory snapshot leaked its slice")
	}
}

func TestRepairM_819_FreshOnly(t *testing.T) {
	for _, count := range []int{0, 3} {
		cfg := &config.LoadConfig{ChainID: 125, Accounts: &config.AccountConfig{Accounts: count, NewAccountRate: 1}, Scenarios: []config.Scenario{{Name: scenarios.EVMTransfer, Weight: 1}}}
		main := repair819Generator(t, cfg)
		prewarm := NewPrewarmGenerator(cfg, main)
		if got := prewarm.GenerateN(5); len(got) != 0 {
			t.Fatal("fresh-only pool has no finite population to prewarm")
		}
		if err := prewarm.Wait(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		txs := main.GenerateN(2)
		if len(txs) != 2 || txs[0].EthTx.Nonce() != 0 || txs[1].EthTx.Nonce() != 0 || txs[0].Scenario.Sender.Address == txs[1].Scenario.Sender.Address {
			t.Fatal("fresh-only measured traffic lost fresh account behavior")
		}
	}
}

func TestRepairM_819_PrewarmReceipts(t *testing.T) {
	cfg := &config.LoadConfig{ChainID: 125, Accounts: &config.AccountConfig{Accounts: 2}, Scenarios: []config.Scenario{{Name: scenarios.EVMTransfer, Weight: 1}}}
	prewarm := NewPrewarmGenerator(cfg, repair819Generator(t, cfg))
	peer, server, client := repair819Server(t)
	if err := prewarm.Wait(context.Background(), client); err == nil {
		t.Fatal("incomplete generation accepted")
	}
	txs := prewarm.GenerateN(2)
	for _, tx := range txs {
		repair819Submit(t, server.URL, tx)
	}
	if err := prewarm.Wait(context.Background(), nil); err == nil {
		t.Fatal("missing receipt reader accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	err := prewarm.Wait(ctx, client)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending submissions must not finish prewarming: %v", err)
	}
	peer.mu.Lock()
	// Only the first transaction is mined: waiting must include the later account.
	second := peer.txs[txs[1].EthTx.Hash()]
	delete(peer.txs, txs[1].EthTx.Hash())
	peer.mined = true
	peer.mu.Unlock()
	ctx, cancel = context.WithTimeout(context.Background(), 40*time.Millisecond)
	err = prewarm.Wait(ctx, client)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("missing later receipt accepted: %v", err)
	}
	peer.mu.Lock()
	peer.txs[second.Hash()] = second
	peer.failed = true
	peer.mu.Unlock()
	if err := prewarm.Wait(context.Background(), client); err == nil || !strings.Contains(err.Error(), "failed with status 0") {
		t.Fatalf("failed receipt was not reported: %v", err)
	}
	peer.mu.Lock()
	peer.failed = false
	peer.receiptError = true
	peer.mu.Unlock()
	if err := prewarm.Wait(context.Background(), client); err == nil || !strings.Contains(err.Error(), "receipt service unavailable") {
		t.Fatalf("RPC receipt error was not preserved: %v", err)
	}
	peer.mu.Lock()
	peer.receiptError = false
	peer.mu.Unlock()
	if err := prewarm.Wait(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if err := prewarm.Wait(ctx, client); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestRepairM_819_WeightAdmission(t *testing.T) {
	gen := repair819Scenario(t)
	var typedNil *scenarioGenerator
	cases := []struct {
		name string
		cfgs []*WeightedCfg
	}{
		{"empty", nil}, {"nil config", []*WeightedCfg{nil}},
		{"nil generator", []*WeightedCfg{WeightedConfig(1, nil)}},
		{"typed nil generator", []*WeightedCfg{WeightedConfig(1, typedNil)}},
		{"single zero", []*WeightedCfg{WeightedConfig(0, gen)}},
		{"single negative", []*WeightedCfg{WeightedConfig(-1, gen)}},
		{"all zero", []*WeightedCfg{WeightedConfig(0, gen), WeightedConfig(0, gen)}},
		{"negative mixed", []*WeightedCfg{WeightedConfig(1, gen), WeightedConfig(-1, gen)}},
		{"above bound", []*WeightedCfg{WeightedConfig(int(MaxTotalWeight)+1, gen)}},
		{"sum overflow", []*WeightedCfg{WeightedConfig(int(MaxTotalWeight), gen), WeightedConfig(1, gen)}},
		{"integer max", []*WeightedCfg{WeightedConfig(math.MaxInt, gen), WeightedConfig(math.MaxInt, gen)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewWeightedGenerator(tc.cfgs...)
			if err == nil || got != nil {
				t.Fatalf("invalid weights accepted: %v/%v", got, err)
			}
		})
	}
	valid, err := NewWeightedGenerator(WeightedConfig(0, gen), WeightedConfig(1, gen))
	if err != nil {
		t.Fatal(err)
	}
	if tx, ok := valid.Generate(); !ok || tx == nil {
		t.Fatal("zero-disabled entry blocked a positive entry")
	}
}

func TestRepairM_819_ConfigAdmission(t *testing.T) {
	if gen, err := NewConfigBasedGenerator(nil); err == nil || gen != nil {
		t.Fatal("nil configuration accepted")
	}
	for _, weights := range [][]int{nil, {0}, {-1}, {0, 0}, {1, -1}, {int(MaxTotalWeight), 1}, {math.MaxInt}} {
		cfg := &config.LoadConfig{Accounts: &config.AccountConfig{Accounts: -1}}
		for _, weight := range weights {
			cfg.Scenarios = append(cfg.Scenarios, config.Scenario{Name: "invalid scenario must never be constructed", Weight: weight})
		}
		gen, err := NewConfigBasedGenerator(cfg)
		if err == nil || gen != nil {
			t.Fatalf("invalid config weights accepted: %v", weights)
		}
	}
}

func TestRepairM_819_CumulativeWeights(t *testing.T) {
	first, second := repair819Scenario(t), repair819Scenario(t)
	gen, err := NewWeightedGenerator(WeightedConfig(2, first), WeightedConfig(0, second), WeightedConfig(3, second))
	if err != nil {
		t.Fatal(err)
	}
	w := gen.(*weightedGenerator)
	if len(w.generators) != 2 || len(w.cumulative) != 2 || w.total != 5 {
		t.Fatal("weights were expanded or total is incorrect")
	}
	w.counter = 0
	address := first.GetAccountPools()[0].(FixedAccountPool).FixedAccounts()[0].Address
	counts := make(map[common.Address]int)
	for _, tx := range gen.GenerateN(100) {
		counts[tx.Scenario.Sender.Address]++
	}
	if len(counts) != 2 || counts[address] != 40 {
		t.Fatalf("2:3 selection cycle changed: %v", counts)
	}
	huge, err := NewWeightedGenerator(WeightedConfig(int(MaxTotalWeight)-1, first), WeightedConfig(1, second))
	if err != nil {
		t.Fatal(err)
	}
	h := huge.(*weightedGenerator)
	if len(h.generators) != 2 || len(h.cumulative) != 2 {
		t.Fatal("large weight amplified storage")
	}
	h.counter = MaxTotalWeight - 2
	txs := h.GenerateN(3)
	if len(txs) != 3 || txs[0].Scenario.Sender.Address != address || txs[1].Scenario.Sender.Address == address || txs[2].Scenario.Sender.Address != address {
		t.Fatal("cumulative boundary or wraparound is incorrect")
	}
	dedup, err := NewWeightedGenerator(WeightedConfig(2, first), WeightedConfig(7, first))
	if err != nil {
		t.Fatal(err)
	}
	if len(dedup.GetAccountPools()) != 1 {
		t.Fatal("weighted duplicate pool was repeated")
	}
}
