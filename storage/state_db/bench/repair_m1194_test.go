package bench

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/bench/wrappers"
)

func requireProducerDone(t *testing.T, stream *changesetStream) {
	t.Helper()
	select {
	case <-stream.done:
	default:
		t.Fatal("consumer returned before producer completion")
	}
}

func requireStoredKeys(t *testing.T, db wrappers.DBWrapper, count int64) {
	t.Helper()
	for i := int64(0); i < count; i++ {
		data, found, err := db.Read(keyFromIndex(i))
		require.NoError(t, err)
		require.True(t, found, "missing generated key %d", i)
		require.Len(t, data, ValueSize)
	}
	_, found, err := db.Read(keyFromIndex(count))
	require.NoError(t, err)
	require.False(t, found, "unexpected key beyond the generated stream")
}

func TestRepairM_1194_LastBlock(t *testing.T) {
	for _, backend := range []wrappers.DBType{wrappers.MemIAVL, wrappers.FlatKV} {
		for _, tc := range []struct {
			name                    string
			keys, blocks, committed int64
			distribution            KeyDistribution
		}{
			{"one_block", 4, 1, 4, EvenDistribution},
			{"all_blocks", 12, 3, 12, EvenDistribution},
			{"remainder", 10, 3, 9, EvenDistribution},
			{"bursty", 8, 4, 16, BurstyDistribution(1, 2, 3, 0)},
			{"empty_blocks", 0, 3, 0, EvenDistribution},
		} {
			t.Run(string(backend)+"/"+tc.name, func(t *testing.T) {
				scenario := TestScenario{Backend: backend, TotalKeys: tc.keys, NumBlocks: tc.blocks, Distribution: tc.distribution}
				dir := t.TempDir()
				db, err := scenario.OpenDB(t.Context(), dir)
				require.NoError(t, err)
				closed := false
				defer func() {
					if !closed {
						require.NoError(t, db.Close())
					}
				}()
				base := db.Version()
				stream := startChangesetGenerator(t.Context(), scenario)
				work, err := consumeChangesets(t.Context(), db, stream, nil)
				require.NoError(t, err)
				requireProducerDone(t, stream)
				require.Equal(t, benchmarkWork{keys: tc.committed, blocks: tc.blocks}, work)
				require.Equal(t, base+tc.blocks, db.Version())
				requireStoredKeys(t, db, tc.committed)
				require.NoError(t, db.Close())
				closed = true

				reopened, err := scenario.OpenDB(t.Context(), dir)
				require.NoError(t, err)
				defer func() { require.NoError(t, reopened.Close()) }()
				require.Equal(t, base+tc.blocks, reopened.Version())
				requireStoredKeys(t, reopened, tc.committed)
				t.Logf("backend=%s completed blocks=%d keys=%d producer=joined reopened=true", backend, work.blocks, work.keys)
			})
		}
	}
}

func TestRepairM_1194_ProducerCancellation(t *testing.T) {
	scenario := TestScenario{Backend: wrappers.FlatKV, TotalKeys: 1000, NumBlocks: 1000}
	db, err := scenario.OpenDB(t.Context(), t.TempDir())
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	ctx, cancel := context.WithCancel(t.Context())
	stream := startChangesetGenerator(ctx, scenario)
	// Receive a real generated block, leaving the remaining unbuffered stream
	// unconsumed. Cancellation must release that producer before returning.
	select {
	case cs := <-stream.changesets:
		require.Len(t, cs.Changeset.Pairs, 1)
	case <-time.After(5 * time.Second):
		t.Fatal("producer did not emit its first block")
	}
	cancel()
	work, err := consumeChangesets(ctx, db, stream, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, benchmarkWork{}, work)
	requireProducerDone(t, stream)
	require.Zero(t, db.Version())
	requireStoredKeys(t, db, 0)

	// Explicit shutdown must also join a producer with no consumer at all.
	stream = startChangesetGenerator(t.Context(), scenario)
	stream.stop()
	requireProducerDone(t, stream)
}

func TestRepairM_1194_Distributions(t *testing.T) {
	for _, backend := range []wrappers.DBType{wrappers.MemIAVL, wrappers.FlatKV} {
		scenarios := keyDistributionScenarios(backend, 1000, 10)
		require.Len(t, scenarios, 4)
		for index, scenario := range scenarios {
			t.Run(string(backend)+"/"+scenario.Name, func(t *testing.T) {
				require.NotNil(t, scenario.Distribution)
				var counts []int64
				var total int64
				for block := int64(0); block < scenario.NumBlocks; block++ {
					count := scenario.Distribution(scenario.NumBlocks, scenario.TotalKeys, block)
					counts = append(counts, count)
					total += count
				}
				switch scenario.Name {
				case "even_distribution":
					require.Equal(t, []int64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100}, counts)
				case "bursty_distribution":
					require.GreaterOrEqual(t, counts[0], int64(497))
					require.LessOrEqual(t, counts[0], int64(503))
					for _, count := range counts[1:] {
						require.GreaterOrEqual(t, count, int64(97))
						require.LessOrEqual(t, count, int64(103))
					}
				case "normal_distribution":
					require.Equal(t, int64(75), counts[0]) // seed 1, mean 100, sigma 20
					require.NotEqual(t, counts[0], counts[1])
					for _, count := range counts {
						require.GreaterOrEqual(t, count, int64(0))
					}
				case "ramp_distribution":
					require.Equal(t, int64(50), counts[0])
					require.Equal(t, int64(94), counts[4])
					require.Equal(t, int64(150), counts[9])
				default:
					t.Fatalf("unasserted distribution %q", scenario.Name)
				}
				// Recreate the seeded distribution after inspecting it. The same scenario
				// builder is used by both named benchmarks, and the real consumer commits
				// every key that this producer emits.
				fresh := keyDistributionScenarios(backend, 1000, 10)[index]
				db, err := fresh.OpenDB(t.Context(), t.TempDir())
				require.NoError(t, err)
				defer func() { require.NoError(t, db.Close()) }()
				stream := startChangesetGenerator(t.Context(), fresh)
				work, err := consumeChangesets(t.Context(), db, stream, nil)
				require.NoError(t, err)
				requireProducerDone(t, stream)
				require.Equal(t, benchmarkWork{keys: total, blocks: 10}, work)
				requireStoredKeys(t, db, total)
				t.Logf("counts=%v committed blocks=%d keys=%d", counts, work.blocks, work.keys)
			})
		}
	}
}

func TestRepairM_1194_IterationMetrics(t *testing.T) {
	// This exercises runBenchmark itself, including Go's multi-iteration run,
	// real FlatKV construction, producer/consumer, final flush and metric output.
	result := testing.Benchmark(func(b *testing.B) {
		runBenchmark(b, TestScenario{Backend: wrappers.FlatKV, TotalKeys: 8, NumBlocks: 4, Distribution: BurstyDistribution(1, 2, 3, 0)}, false)
	})
	require.Greater(t, result.N, 1)
	require.Equal(t, float64(16*result.N), result.Extra["keys"])
	require.Equal(t, float64(4*result.N), result.Extra["blocks"])
	seconds := result.Extra["seconds"]
	require.Positive(t, seconds)
	require.Equal(t, result.T.Seconds(), seconds)
	require.InDelta(t, float64(16*result.N)/seconds, result.Extra["keys/sec"], 1e-9)
	t.Logf("iterations=%d completed blocks=%.0f keys=%.0f elapsed=%fs", result.N, result.Extra["blocks"], result.Extra["keys"], seconds)
}

func TestRepairM_1194_BackendDocumentation(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	require.Contains(t, string(readme), "TestScenario.OpenDB")
	require.Contains(t, string(readme), "HistoricalOffloadConfig")
	require.Contains(t, string(readme), "TestRepairM_1194_")
	for _, backend := range []wrappers.DBType{
		wrappers.MemIAVL, wrappers.FlatKV, wrappers.CompositeCosmos,
		wrappers.CompositeSplit, wrappers.CompositeDual, wrappers.SSComposite,
		wrappers.CompositeDual_SSComposite,
	} {
		t.Run(string(backend), func(t *testing.T) {
			scenario := TestScenario{Backend: backend, TotalKeys: 3, NumBlocks: 1}
			db, err := scenario.OpenDB(t.Context(), t.TempDir())
			require.NoError(t, err)
			closed := false
			defer func() {
				if !closed {
					require.NoError(t, db.Close())
				}
			}()
			stream := startChangesetGenerator(t.Context(), scenario)
			work, err := consumeChangesets(t.Context(), db, stream, nil)
			require.NoError(t, err)
			requireProducerDone(t, stream)
			require.Equal(t, benchmarkWork{keys: 3, blocks: 1}, work)
			// SS writes are asynchronous; observe the real store's completion.
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				value, found, err := db.Read(keyFromIndex(2))
				assert.NoError(c, err)
				assert.True(c, found)
				assert.Len(c, value, ValueSize)
			}, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, db.Close())
			closed = true
		})
	}
	db, err := (TestScenario{Backend: wrappers.SSHistoricalOffload}).OpenDB(t.Context(), t.TempDir())
	require.Nil(t, db)
	require.ErrorContains(t, err, "requires TestScenario.HistoricalOffloadConfig")
}

func TestRepairM_1194_ProgressCounts(t *testing.T) {
	scenario := TestScenario{Backend: wrappers.FlatKV, TotalKeys: 8, NumBlocks: 4, Distribution: BurstyDistribution(1, 2, 3, 0)}
	db, err := scenario.OpenDB(t.Context(), t.TempDir())
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	reporter := NewProgressReporter(scenario.TotalKeys, scenario.NumBlocks, time.Hour)
	reporter.Start()
	defer reporter.Stop()
	stream := startChangesetGenerator(t.Context(), scenario)
	work, err := consumeChangesets(t.Context(), db, stream, reporter)
	require.NoError(t, err)
	requireProducerDone(t, stream)
	require.Equal(t, int64(16), work.keys)
	require.Equal(t, work.keys, reporter.keysWritten.Load())
	require.Equal(t, int64(4), reporter.blocksWritten.Load())
}
