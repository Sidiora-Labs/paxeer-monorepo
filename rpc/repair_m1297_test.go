package evmrpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// Task 22.33.1297, audit 22.30.69 proof index:
// filter-ticker-panic: TestRepairM_1297_FilterTickerPanic.
// filter-expiry-race: TestRepairM_1297_FilterExpiryRace.
// committed-receipt-nil-view: TestRepairM_1297_CommittedReceiptNilView.
// fee-token-vector-reversed: TestRepairM_1297_FeeTokenVector.
// estimate-limiter-not-proven: TestRepairM_1297_EstimateLimiter.
// log-subscription-skipped: TestRepairM_1297_LogSubscription.
// legacy-trace-lookback: TestRepairM_1297_LegacyTraceLookback.
// transaction-tests-vacuous: TestRepairM_1297_TransactionLookups.
// The test-only registry connects the node-backed external-package harnesses
// without introducing an import cycle into the production RPC package.
// Baseline verification was not run: the explicit implementation-first gate
// order prohibits it, and these new proving tests did not exist on the base.
// Wiring request: sdk/platform/conformance/fixtures/sequencer-receipt-fee-token-v1.json,
// SDK domain: regenerate from TestSequencerReceiptFeeTokenVector with -update.
// Wiring request: go.mod and sdk/platform/go, SDK domain: wire the nested Go SDK
// module into the root module so this gate can call VerifySequencerReceipt.
// Wiring request: spec/layerx-merge/spec.kvx, spec domain: carry this proof index
// and the runner's verified revision; spec files are outside this task's scope.

func TestRepairM_1297_FilterTickerPanic(t *testing.T) {
	for _, timeout := range []time.Duration{-time.Second, 0, time.Nanosecond, config.MinFilterTimeout} {
		t.Run(timeout.String(), func(t *testing.T) {
			opts := viper.New()
			opts.Set("evm.filter_timeout", timeout.String())
			cfg, err := config.ReadConfig(opts)
			construct := func() {
				api := NewFilterAPI(nil, nil, nil, nil, NewFilterConfig(timeout, 0, 0),
					ConnectionTypeHTTP, "eth", nil, nil, &sync.Mutex{}, NewLogSlicePool(), nil)
				api.shutdown()
			}
			if timeout < config.MinFilterTimeout {
				require.ErrorContains(t, err, "evm.filter_timeout")
				require.PanicsWithValue(t, "evm.filter_timeout must be at least 2ns so the cleanup ticker interval is positive", construct)
			} else {
				require.NoError(t, err)
				require.Equal(t, timeout, cfg.FilterTimeout)
				require.NotPanics(t, construct)
			}
		})
	}
	require.Panics(t, func() {
		NewFilterAPI(nil, nil, nil, nil, nil, ConnectionTypeHTTP, "eth", nil, nil, nil, nil, nil)
	})
}

func TestRepairM_1297_FilterExpiryRace(t *testing.T) {
	_, apis, transport := newFilterShutdownServer(t)
	api := apis["eth"]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id rpc.ID
	require.NoError(t, transport.CallContext(ctx, &id, "eth_newFilter", map[string]string{"fromBlock": "pending", "toBlock": "pending"}))
	api.filtersMu.Lock()
	entry := api.filters[id]
	entry.lastAccess = time.Now().Add(-2 * time.Minute)
	api.filters[id] = entry
	api.filtersMu.Unlock()

	// The first cleanup pass selected this expired entry. Hold its removal until
	// an actual RPC poll has refreshed the entry, reproducing the old race window.
	selectedAt := time.Now()
	polled := make(chan struct{})
	defer func() {
		select {
		case <-polled:
		default:
			close(polled)
		}
	}()
	removed := make(chan struct{})
	go func() {
		<-polled
		api.removeExpiredFilters(selectedAt, time.Minute, []rpc.ID{id})
		close(removed)
	}()
	var logs []FilterLog
	require.NoError(t, transport.CallContext(ctx, &logs, "eth_getFilterChanges", id))
	close(polled)
	select {
	case <-removed:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	api.filtersMu.RLock()
	refreshed, installed := api.filters[id]
	api.filtersMu.RUnlock()
	require.True(t, installed, "a refreshed candidate must survive cleanup")
	require.True(t, refreshed.lastAccess.After(selectedAt))
	api.receiptLogs.mu.Lock()
	_, subscribed := api.receiptLogs.filters[refreshed.pending]
	api.receiptLogs.mu.Unlock()
	require.True(t, subscribed, "cleanup must not cancel the refreshed feed subscription")
	require.NoError(t, transport.CallContext(ctx, &logs, "eth_getFilterChanges", id))

	api.filtersMu.Lock()
	refreshed = api.filters[id]
	refreshed.lastAccess = time.Now().Add(-2 * time.Minute)
	api.filters[id] = refreshed
	api.filtersMu.Unlock()
	api.cleanupExpiredFilters(time.Minute)
	api.filtersMu.RLock()
	_, installed = api.filters[id]
	api.filtersMu.RUnlock()
	require.False(t, installed, "a still-expired filter must be removed")
	api.receiptLogs.mu.Lock()
	_, subscribed = api.receiptLogs.filters[refreshed.pending]
	api.receiptLogs.mu.Unlock()
	require.False(t, subscribed, "expiry must cancel the feed subscription")
}

// RepairM1297Proofs is populated by the external test package at init, before
// the Go test runner executes any test. It keeps every gate entry point here
// while allowing node/sequencer (which imports rpc) to exercise its real host.
var RepairM1297Proofs = map[string]func(*testing.T){}

func repairM1297RunProof(t *testing.T, finding string) {
	t.Helper()
	proof := RepairM1297Proofs[finding]
	require.NotNil(t, proof, "missing real-code proof for %s", finding)
	proof(t)
}

func TestRepairM_1297_CommittedReceiptNilView(t *testing.T) {
	repairM1297RunProof(t, "committed-receipt-nil-view")
}
func TestRepairM_1297_FeeTokenVector(t *testing.T) {
	repairM1297RunProof(t, "fee-token-vector-reversed")
}
func TestRepairM_1297_EstimateLimiter(t *testing.T) {
	repairM1297RunProof(t, "estimate-limiter-not-proven")
}
func TestRepairM_1297_LogSubscription(t *testing.T) {
	repairM1297RunProof(t, "log-subscription-skipped")
}
func TestRepairM_1297_LegacyTraceLookback(t *testing.T) {
	repairM1297RunProof(t, "legacy-trace-lookback")
}
func TestRepairM_1297_TransactionLookups(t *testing.T) {
	repairM1297RunProof(t, "transaction-tests-vacuous")
}
