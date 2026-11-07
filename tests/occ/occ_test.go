package occ

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/messages"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func assertEqualState(t *testing.T, expectedCtx sdk.Context, actualCtx sdk.Context, testName string) {
	expectedStoreKeys := expectedCtx.MultiStore().StoreKeys()
	actualStoreKeys := actualCtx.MultiStore().StoreKeys()
	require.NoError(t, compareStoreKeyNames(expectedStoreKeys, actualStoreKeys), testName)

	// Store keys are mapped by reference; pair the independently sorted names.
	for i, esk := range expectedStoreKeys {
		ask := actualStoreKeys[i]
		expected := expectedCtx.MultiStore().GetKVStore(esk)
		actual := actualCtx.MultiStore().GetKVStore(ask)
		utils.CompareStores(t, esk, expected, actual, testName)
	}
}

func compareStoreKeyNames(expected, actual []sdk.StoreKey) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("number of stores does not match: %d != %d", len(expected), len(actual))
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].Name() < expected[j].Name() })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name() < actual[j].Name() })
	for i := range expected {
		if expected[i].Name() != actual[i].Name() {
			return fmt.Errorf("store names do not match: %q != %q", expected[i].Name(), actual[i].Name())
		}
	}
	return nil
}

// assertEqualEventAttributes compares attribute occurrences, regardless of order.
func assertEqualEventAttributes(t *testing.T, testName string, expected, actual []types.EventAttribute) {
	require.NoError(t, compareEventAttributes(expected, actual), "%s: Event attributes do not match", testName)
}

func compareEventAttributes(expected, actual []types.EventAttribute) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("number of event attributes does not match: %d != %d", len(expected), len(actual))
	}
	type attributeKey struct {
		key, value string
		index      bool
	}
	counts := make(map[attributeKey]int, len(expected))
	for _, attr := range expected {
		counts[attributeKey{string(attr.Key), string(attr.Value), attr.Index}]++
	}
	for _, attr := range actual {
		key := attributeKey{string(attr.Key), string(attr.Value), attr.Index}
		if counts[key] == 0 {
			return fmt.Errorf("unexpected event attribute occurrence: %+v", attr)
		}
		counts[key]--
	}
	return nil
}

// assertEqualEvents checks if both event slices have the same events, regardless of order.
func assertEqualEvents(t *testing.T, expected, actual []types.Event, testName string) {
	require.Equal(t, len(expected), len(actual), "%s: Number of events do not match", testName)

	for _, expectedEvent := range expected {
		found := false
		for i, actualEvent := range actual {
			if expectedEvent.Type == actualEvent.Type {
				assertEqualEventAttributes(t, testName, expectedEvent.Attributes, actualEvent.Attributes)
				actual = append(actual[:i], actual[i+1:]...) // Remove the found event
				found = true
				break
			}
		}
		require.True(t, found, "%s: Expected event of type '%s' not found", testName, expectedEvent.Type)
	}
}

// assertEqualExecTxResults validates the code, so that all errors don't count as a success
func assertExecTxResultCode(t *testing.T, expected, actual []*types.ExecTxResult, code uint32, testName string) {
	for _, e := range expected {
		require.Equal(t, code, e.Code, "%s: Expected code %d, got %d", testName, code, e.Code)
	}
	for _, a := range actual {
		require.Equal(t, code, a.Code, "%s: Actual code %d, got %d", testName, code, a.Code)
	}
}

// assertEqualExecTxResults checks if both slices have the same transaction results, regardless of order.
func assertEqualExecTxResults(t *testing.T, expected, actual []*types.ExecTxResult, testName string) {
	require.NoError(t, compareExecTxResults(expected, actual), "%s: Transaction results do not match", testName)
}

func compareExecTxResults(expected, actual []*types.ExecTxResult) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("number of transaction results does not match: %d != %d", len(expected), len(actual))
	}
	consumed := make([]bool, len(actual))
	for _, expectedRes := range expected {
		found := false
		for i, actualRes := range actual {
			if !consumed[i] && reflect.DeepEqual(expectedRes, actualRes) {
				consumed[i] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected ExecTxResult occurrence not found: %+v", expectedRes)
		}
	}
	return nil
}

type Test struct {
	name    string
	runs    int
	shuffle bool
	before  func(tCtx *utils.TestContext)
	txs     func(tCtx *utils.TestContext) []*utils.TestMessage
}

func TestParallelTransactionsWasmInstantiate(t *testing.T) {
	runTest(t, Test{
		name: "Test wasm instantiations",
		runs: 3,
		before: func(tCtx *utils.TestContext) {
			require.Equal(t, tCtx.CW20CodeID, tCtx.CodeID)
		},
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.WasmInstantiate(tCtx, 10),
			)
		},
	})
}

func TestParallelTransactionsBankTransfer(t *testing.T) {
	runTest(t, Test{
		name: "Test bank transfer",
		runs: 3,
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.BankTransfer(tCtx, 2),
			)
		},
	})
}

func TestParallelTransactionsGovProposal(t *testing.T) {
	runTest(t, Test{
		name: "Test governance proposal",
		runs: 3,
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.GovernanceSubmitProposal(tCtx, 10),
			)
		},
	})
}

func TestParallelTransactionsEvmTransferNonConflicting(t *testing.T) {
	runTest(t, Test{
		name: "Test evm transfers non-conflicting",
		runs: 3,
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.EVMTransferNonConflicting(tCtx, 10),
			)
		},
	})
}

func TestParallelTransactionsEvmTransferConflicting(t *testing.T) {
	runTest(t, Test{
		name: "Test evm transfers conflicting",
		runs: 3,
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.EVMTransferConflicting(tCtx, 10),
			)
		},
	})
}

func TestParallelTransactionsPointerCreation(t *testing.T) {
	runTest(t, Test{
		name: "Test pointer creation",
		runs: 3,
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.ERC20toCWAssets(tCtx, 10),
			)
		},
	})
}

func TestParallelTransactionsCombined(t *testing.T) {
	runTest(t, Test{
		name:    "Test combinations",
		runs:    3,
		shuffle: true,
		txs: func(tCtx *utils.TestContext) []*utils.TestMessage {
			return utils.JoinMsgs(
				messages.WasmInstantiate(tCtx, 10),
				messages.BankTransfer(tCtx, 10),
				messages.GovernanceSubmitProposal(tCtx, 10),
				messages.EVMTransferConflicting(tCtx, 10),
				messages.ERC20toCWAssets(tCtx, 10),
				messages.EVMTransferNonConflicting(tCtx, 10),
			)
		},
	})
}

func runTest(t *testing.T, tt Test) {
	blockTime := time.Now()
	accts := utils.NewTestAccounts(5)

	// execute sequentially, then in parallel
	// the responses and state should match for both
	sCtx := utils.NewTestContext(t, accts, blockTime, 1, false)
	txs := tt.txs(sCtx)
	if tt.shuffle {
		txs = utils.Shuffle(txs)
	}

	if tt.before != nil {
		tt.before(sCtx)
	}

	sEvts, sResults, _, sErr := utils.RunWithoutOCC(sCtx, txs)
	require.NoError(t, sErr, tt.name)
	require.Len(t, sResults, len(txs))

	for i := 0; i < tt.runs; i++ {
		pCtx := utils.NewTestContext(t, accts, blockTime, config.DefaultConcurrencyWorkers, true)
		if tt.before != nil {
			tt.before(pCtx)
		}
		pEvts, pResults, _, pErr := utils.RunWithOCC(pCtx, txs)
		require.NoError(t, pErr, tt.name)
		require.Len(t, pResults, len(txs))

		assertExecTxResultCode(t, sResults, pResults, 0, tt.name)
		assertEqualEvents(t, sEvts, pEvts, tt.name)
		assertEqualExecTxResults(t, sResults, pResults, tt.name)
		assertEqualState(t, sCtx.Ctx, pCtx.Ctx, tt.name)
	}
}

// BenchmarkEVMTransactionsMixed benchmarks execution time for a mix of conflicting
// and non-conflicting EVM transactions that are shuffled together.
func BenchmarkEVMTransactionsMixed(b *testing.B) {
	blockTime := time.Now()
	accts := utils.NewTestAccounts(5)

	// Transaction counts
	conflictingCount := 50
	nonConflictingCount := 50
	totalTxCount := conflictingCount + nonConflictingCount

	b.Logf("Benchmark setup: %d conflicting + %d non-conflicting = %d total EVM transactions",
		conflictingCount, nonConflictingCount, totalTxCount)

	b.ResetTimer()

	var totalGasUsed int64
	var totalTimedDuration time.Duration

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Create a fresh context and prepare transactions (outside timer)
		b.StopTimer()
		iterCtx := utils.NewTestContext(b, accts, blockTime, config.DefaultConcurrencyWorkers, true)

		// Generate transactions for this iteration using the fresh context
		conflictingTxs := messages.EVMTransferConflicting(iterCtx, conflictingCount)
		nonConflictingTxs := messages.EVMTransferNonConflicting(iterCtx, nonConflictingCount)

		mixedTxs := utils.JoinMsgs(conflictingTxs, nonConflictingTxs)
		mixedTxs = utils.Shuffle(mixedTxs)

		txs := utils.ToTxBytes(iterCtx, mixedTxs)
		b.StartTimer()

		// Only measure ProcessBlock execution time
		startTime := time.Now()
		_, txResults, _, err := utils.ProcessBlockDirect(iterCtx, txs, true)
		timedDuration := time.Since(startTime)
		totalTimedDuration += timedDuration

		if err != nil {
			b.Fatalf("ProcessBlock returned error (unexpected): %v", err)
		}
		if len(txResults) != len(txs) {
			b.Fatalf("Expected %d transaction results, got %d", len(txs), len(txResults))
		}

		// Sum gas used from all transaction results
		for _, result := range txResults {
			totalGasUsed += result.GasUsed
		}
	}

	// Report metrics
	b.ReportMetric(float64(totalTxCount), "txns/op")
	avgGasPerOp := float64(totalGasUsed) / float64(b.N)
	b.ReportMetric(avgGasPerOp, "gas/op")

	// Calculate and report gas per second
	avgTimeSeconds := totalTimedDuration.Seconds() / float64(b.N)
	gasPerSecond := avgGasPerOp / avgTimeSeconds
	b.ReportMetric(gasPerSecond, "gas/sec")
}
