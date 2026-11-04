package shadow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

// TraceKeySource derives a block's touched accounts and storage slots from a
// full prestate trace and a diff-mode prestate trace on an EVM JSON-RPC endpoint.
// Full prestate retains read-only accounts, unchanged code and accessed slots;
// diff mode adds changed and newly created post-state accounts and slots. Both
// traces are pinned to the block hash and must cover every EVM transaction.
// Balance, nonce and code (including empty code) are checked for every account
// in this union. Requires the debug_ namespace on an operator-owned node.
//
// Coverage boundary: this is the per-block TOUCHED set. Keys migrated but never
// touched by any transaction (cold state), and non-EVM Cosmos-module state, are
// not covered here — that breadth is the corpus harness's job (Arm A) plus a
// periodic StaticKeySource sweep. Layer 2 over a trace source is a hot-state
// sampling oracle, not a full-keyspace check.
type TraceKeySource struct {
	client *gethrpc.Client
}

// NewTraceKeySource dials the EVM JSON-RPC endpoint used to fetch prestate
// traces. The same touched set applies to both chains (identical transactions),
// so a single endpoint (typically the canonical node) is sufficient.
func NewTraceKeySource(evmRPC string) (*TraceKeySource, error) {
	c, err := gethrpc.Dial(evmRPC)
	if err != nil {
		return nil, fmt.Errorf("dialing EVM RPC %q: %w", evmRPC, err)
	}
	return &TraceKeySource{client: c}, nil
}

// Close releases the underlying RPC connection. No return value, matching
// go-ethereum's *ethclient.Client / *rpc.Client Close() so Comparator.Close can
// treat all closeables uniformly.
func (t *TraceKeySource) Close() {
	if t.client != nil {
		t.client.Close()
	}
}

// prestateAccount is the subset of the prestateTracer's per-account output the
// key source needs: which slots were accessed.
type prestateAccount struct {
	Storage map[common.Hash]common.Hash `json:"storage"`
}

// prestateResult holds a full prestate in Pre, or the changed pre/post
// accounts from diff mode. Diff mode alone omits unchanged accessed state.
type prestateResult struct {
	Pre  map[common.Address]*prestateAccount `json:"pre"`
	Post map[common.Address]*prestateAccount `json:"post"`
}

// prestateTxTrace wraps one transaction's tracer result.
type prestateTxTrace struct {
	TxHash common.Hash     `json:"txHash"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

func (t *TraceKeySource) TouchedAccounts(ctx context.Context, height int64) ([]TouchedAccount, error) {
	if height < 0 {
		return nil, fmt.Errorf("invalid trace height %d", height)
	}
	blockArg := hexutil.EncodeUint64(uint64(height))
	var block *struct {
		Hash         common.Hash   `json:"hash"`
		Number       *hexutil.Big  `json:"number"`
		Transactions []common.Hash `json:"transactions"`
	}
	if err := t.client.CallContext(ctx, &block, "eth_getBlockByNumber", blockArg, false); err != nil {
		return nil, fmt.Errorf("eth_getBlockByNumber at height %d: %w", height, err)
	}
	if block == nil || block.Hash == (common.Hash{}) || block.Number == nil ||
		block.Number.ToInt().Cmp(new(big.Int).SetInt64(height)) != 0 || block.Transactions == nil {
		return nil, fmt.Errorf("missing or invalid EVM block at height %d", height)
	}
	seen := make(map[common.Hash]bool, len(block.Transactions))
	for _, hash := range block.Transactions {
		if hash == (common.Hash{}) || seen[hash] {
			return nil, fmt.Errorf("invalid EVM transaction inventory at height %d", height)
		}
		seen[hash] = true
	}
	full, err := t.traceBlock(ctx, block.Hash, block.Transactions, false)
	if err != nil {
		return nil, err
	}
	diff, err := t.traceBlock(ctx, block.Hash, block.Transactions, true)
	if err != nil {
		return nil, err
	}
	return mergePrestateTraces(append(full, diff...)), nil
}

func (t *TraceKeySource) traceBlock(ctx context.Context, hash common.Hash, transactions []common.Hash, diff bool) ([]prestateResult, error) {
	var traces []prestateTxTrace
	cfg := map[string]any{"tracer": "prestateTracer", "tracerConfig": map[string]any{
		"diffMode": diff, "disableCode": false, "disableStorage": false,
	}}
	if err := t.client.CallContext(ctx, &traces, "debug_traceBlockByHash", hash, cfg); err != nil {
		return nil, fmt.Errorf("debug_traceBlockByHash %s (diffMode=%t): %w", hash, diff, err)
	}
	if traces == nil || len(traces) != len(transactions) {
		return nil, fmt.Errorf("trace coverage for block %s (diffMode=%t): got %d traces, want %d", hash, diff, len(traces), len(transactions))
	}
	results := make([]prestateResult, 0, len(traces))
	for i, trace := range traces {
		if trace.TxHash != transactions[i] {
			return nil, fmt.Errorf("trace coverage for block %s (diffMode=%t), transaction %d: got %s, want %s", hash, diff, i, trace.TxHash, transactions[i])
		}
		if trace.Error != "" {
			return nil, fmt.Errorf("trace %s (diffMode=%t) failed: %s", trace.TxHash, diff, trace.Error)
		}
		var result prestateResult
		var err error
		if diff {
			err = json.Unmarshal(trace.Result, &result)
		} else {
			err = json.Unmarshal(trace.Result, &result.Pre)
		}
		if err != nil || result.Pre == nil || (diff && result.Post == nil) || (!diff && len(result.Pre) == 0) {
			return nil, fmt.Errorf("trace %s (diffMode=%t): missing or invalid prestate result: %v", trace.TxHash, diff, err)
		}
		for _, accounts := range []map[common.Address]*prestateAccount{result.Pre, result.Post} {
			for addr, account := range accounts {
				if account == nil {
					return nil, fmt.Errorf("trace %s (diffMode=%t): missing account %s", trace.TxHash, diff, addr)
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// mergePrestateTraces unions validated full prestate and diff results. All
// account axes are checked, even when the canonical account has no code, so
// unexpected code on the shadow cannot escape comparison. Output is sorted.
func mergePrestateTraces(traces []prestateResult) []TouchedAccount {
	slots := map[common.Address]map[common.Hash]struct{}{}

	absorb := func(accts map[common.Address]*prestateAccount) {
		for addr, acct := range accts {
			if slots[addr] == nil {
				slots[addr] = map[common.Hash]struct{}{}
			}
			for slot := range acct.Storage {
				slots[addr][slot] = struct{}{}
			}
		}
	}
	for _, tx := range traces {
		absorb(tx.Pre)
		absorb(tx.Post)
	}

	out := make([]TouchedAccount, 0, len(slots))
	for addr, slotSet := range slots {
		ta := TouchedAccount{Addr: addr, CheckCode: true, CheckNonce: true, CheckBalance: true}
		for slot := range slotSet {
			ta.Slots = append(ta.Slots, slot)
		}
		sort.Slice(ta.Slots, func(i, j int) bool { return bytes.Compare(ta.Slots[i][:], ta.Slots[j][:]) < 0 })
		out = append(out, ta)
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].Addr[:], out[j].Addr[:]) < 0 })
	return out
}

// StaticKeySource compares a fixed, curated set of accounts on every block — the
// fallback when prestate tracing (debug_) is unavailable, and the hook for a
// periodic cold-key / hot-contract sweep that the trace source does not cover.
type StaticKeySource struct {
	Accounts []TouchedAccount
}

func (s StaticKeySource) TouchedAccounts(_ context.Context, _ int64) ([]TouchedAccount, error) {
	return s.Accounts, nil
}
