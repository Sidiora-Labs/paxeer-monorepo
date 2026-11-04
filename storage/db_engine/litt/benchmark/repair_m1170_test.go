package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/unit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/benchmark/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

func repairM1170Config(t *testing.T) (*config.BenchmarkConfig, string) {
	t.Helper()
	cfg := config.DefaultBenchmarkConfig()
	cfg.LittConfig.Paths = []string{t.TempDir()}
	cfg.LittConfig.KeymapType = keymap.MemKeymapType
	cfg.LittConfig.MetricsEnabled = false
	cfg.MetadataDirectory = t.TempDir()
	cfg.Seed = 1170
	cfg.RandomPoolSize = 256
	cfg.ValueSizeMB = 32.0 / unit.MB
	cfg.BatchSizeMB = 128.0 / unit.MB
	cfg.CohortSize = 2
	cfg.WriteInfoChanelSize = 2
	cfg.ReadInfoChanelSize = 2
	cfg.WriterParallelism = 2
	cfg.ReaderParallelism = 2
	cfg.MaximumWriteThroughputMB = 0.01
	cfg.MaximumReadThroughputMB = 0.01
	cfg.StartupSleepFactorSeconds = 0
	cfg.MetricsLoggingPeriodSeconds = 0
	cfg.CohortGCPeriodSeconds = 1
	cfg.Fsync = true
	cfg.TimeLimitSeconds = 0.25
	return cfg, filepath.Join(t.TempDir(), "benchmark.json")
}

func repairM1170SaveConfig(t *testing.T, cfg *config.BenchmarkConfig, filename string) {
	t.Helper()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filename, data, 0600))
}

func repairM1170Open(t *testing.T, cfg *config.BenchmarkConfig, filename string) *BenchmarkEngine {
	t.Helper()
	repairM1170SaveConfig(t, cfg, filename)
	engine, err := NewBenchmarkEngine(filename)
	require.NoError(t, err)
	t.Cleanup(func() {
		engine.cancel()
		engine.dataTracker.Close()
		_ = engine.db.Close()
	})
	return engine
}

// The metadata fixture is emitted by the real tracker after real durable writes.
func repairM1170Seed(t *testing.T, cfg *config.BenchmarkConfig, filename string, corrupt bool) {
	t.Helper()
	engine := repairM1170Open(t, cfg, filename)
	for i := uint64(0); i < cfg.CohortSize; i++ {
		info := engine.dataTracker.GetWriteInfo()
		require.NotNil(t, info)
		value := bytes.Clone(info.Value)
		if corrupt {
			value[0] ^= 0xff
		}
		require.NoError(t, engine.table.Put(info.Key, value))
		require.NoError(t, engine.table.Flush())
		engine.dataTracker.ReportWrite(info.KeyIndex)
	}
	require.NoError(t, engine.db.Close())
	engine.dataTracker.Close()
	cohort, err := LoadCohort(filepath.Join(cfg.MetadataDirectory, "cohorts", "0.cohort"))
	require.NoError(t, err)
	require.True(t, cohort.IsComplete())
}

func TestRepairM_1170_GeneratorSeed(t *testing.T) {
	first := NewDataGenerator(1170, 128)
	replay := NewDataGenerator(1170, 128)
	other := NewDataGenerator(1171, 128)
	for _, index := range []uint64{0, 1, 70, 1000} {
		reference := rand.New(rand.NewSource(1170 + int64(index)))
		key := make([]byte, 32)
		_, err := reference.Read(key)
		require.NoError(t, err)
		require.Equal(t, key, first.Key(index))
		require.Equal(t, first.Key(index), replay.Key(index))
		require.NotEqual(t, first.Key(index), other.Key(index))
		for _, size := range []uint64{64, 128, 129, 512} {
			require.Equal(t, first.Value(index, size), replay.Value(index, size))
			require.NotEqual(t, first.Value(index, size), other.Value(index, size))
		}
	}
}

func TestRepairM_1170_GeneratorExactPool(t *testing.T) {
	for _, poolSize := range []uint64{0, 1, 128} {
		generator := NewDataGenerator(1170, poolSize)
		sizes := []uint64{0, poolSize, poolSize + 1}
		if poolSize > 0 {
			sizes = append(sizes, poolSize-1)
		}
		for _, size := range sizes {
			t.Run(fmt.Sprintf("pool_%d_size_%d", poolSize, size), func(t *testing.T) {
				got := generator.Value(19, size)
				require.Len(t, got, int(size))
				require.Equal(t, got, NewDataGenerator(1170, poolSize).Value(19, size))
				if size == poolSize {
					require.Equal(t, generator.dataPool, got)
				}
			})
		}
	}
	generator := NewDataGenerator(1170, 128)
	sawLast := false
	for index := uint64(0); index < 128; index++ {
		rng := rand.New(rand.NewSource(1170 + int64(index)))
		start := rng.Intn(2) // A 127-byte value has valid starts 0 and 1.
		sawLast = sawLast || start == 1
		require.Equal(t, generator.dataPool[start:start+127], generator.Value(index, 127))
	}
	require.True(t, sawLast)
}

func TestRepairM_1170_GenesisCohortCount(t *testing.T) {
	for _, size := range []uint64{1, 2, 7} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			cfg, filename := repairM1170Config(t)
			cfg.CohortSize = size
			engine := repairM1170Open(t, cfg, filename)
			for i := uint64(0); i <= size; i++ {
				info := engine.dataTracker.GetWriteInfo()
				require.NotNil(t, info)
				require.Equal(t, i, info.KeyIndex)
			}
			engine.dataTracker.Close()
			require.NoError(t, engine.db.Close())
			lastHigh := engine.dataTracker.activeCohort.HighKeyIndex()
			require.Equal(t, size-1, engine.dataTracker.cohorts[0].HighKeyIndex())
			reopened := repairM1170Open(t, cfg, filename)
			info := reopened.dataTracker.GetWriteInfo()
			require.NotNil(t, info)
			require.Equal(t, lastHigh+1, info.KeyIndex)
			reopened.dataTracker.Close()
			for _, cohort := range reopened.dataTracker.cohorts {
				require.Equal(t, size, cohort.HighKeyIndex()-cohort.LowKeyIndex()+1)
			}
		})
	}
	cfg, _ := repairM1170Config(t)
	cfg.CohortSize = 0
	tracker, err := NewDataTracker(t.Context(), cfg, util.NewErrorMonitor(t.Context(), slog.Default(), nil))
	require.ErrorContains(t, err, "CohortSize must be positive")
	require.Nil(t, tracker)
}

func TestRepairM_1170_CohortDurabilityRestart(t *testing.T) {
	cfg, filename := repairM1170Config(t)
	cfg.Fsync = false
	repairM1170Seed(t, cfg, filename, false)
	cfg.Fsync = true
	engine := repairM1170Open(t, cfg, filename)
	for i := uint64(0); i < cfg.CohortSize*3; i++ {
		info := engine.dataTracker.GetWriteInfo()
		require.NotNil(t, info)
		require.NoError(t, engine.table.Put(info.Key, info.Value))
		require.NoError(t, engine.table.Flush())
		engine.dataTracker.ReportWrite(info.KeyIndex)
	}
	require.NoError(t, engine.db.Close())
	engine.dataTracker.Close()
	for _, cohort := range engine.dataTracker.cohorts {
		require.True(t, cohort.fsync, "cohort %d lost active durability policy", cohort.CohortIndex())
	}
	restarted := repairM1170Open(t, cfg, filename)
	read := restarted.dataTracker.GetReadInfoWithTimeout(time.Second)
	require.NotNil(t, read)
	actual, exists, err := restarted.table.Get(read.Key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, read.Value, actual)
	restarted.dataTracker.Close()
	require.True(t, restarted.dataTracker.activeCohort.fsync)
}

func TestRepairM_1170_CohortGapRecovery(t *testing.T) {
	cfg, _ := repairM1170Config(t)
	dir := filepath.Join(cfg.MetadataDirectory, "cohorts")
	require.NoError(t, os.MkdirAll(dir, 0700))
	first, err := NewCohort(dir, 0, 0, 1, 32, true)
	require.NoError(t, err)
	middle, err := first.NextCohort(2, 32)
	require.NoError(t, err)
	last, err := middle.NextCohort(2, 32)
	require.NoError(t, err)
	beforeFirst, err := os.ReadFile(first.Path())
	require.NoError(t, err)
	beforeLast, err := os.ReadFile(last.Path())
	require.NoError(t, err)
	require.NoError(t, os.Remove(middle.Path()))
	tracker, err := NewDataTracker(t.Context(), cfg, util.NewErrorMonitor(t.Context(), slog.Default(), nil))
	require.ErrorContains(t, err, "missing cohort 1 between 0 and 2")
	require.ErrorContains(t, err, dir)
	require.Nil(t, tracker)
	afterFirst, err := os.ReadFile(first.Path())
	require.NoError(t, err)
	afterLast, err := os.ReadFile(last.Path())
	require.NoError(t, err)
	require.Equal(t, beforeFirst, afterFirst)
	require.Equal(t, beforeLast, afterLast)
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)
}

func TestRepairM_1170_CohortRangeRecovery(t *testing.T) {
	for _, low := range []uint64{1, 3} {
		t.Run(fmt.Sprint(low), func(t *testing.T) {
			cfg, _ := repairM1170Config(t)
			dir := filepath.Join(cfg.MetadataDirectory, "cohorts")
			require.NoError(t, os.MkdirAll(dir, 0700))
			_, err := NewCohort(dir, 0, 0, 1, 32, true)
			require.NoError(t, err)
			second, err := NewCohort(dir, 1, low, low+1, 32, true)
			require.NoError(t, err)
			before, err := os.ReadFile(second.Path())
			require.NoError(t, err)
			tracker, err := NewDataTracker(t.Context(), cfg, util.NewErrorMonitor(t.Context(), slog.Default(), nil))
			require.ErrorContains(t, err, "inconsistent key ranges for cohorts 0")
			require.Nil(t, tracker)
			after, err := os.ReadFile(second.Path())
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestRepairM_1170_ErrorResult(t *testing.T) {
	cfg, filename := repairM1170Config(t)
	cfg.TimeLimitSeconds = 5
	engine := repairM1170Open(t, cfg, filename)
	require.NoError(t, engine.db.Close())
	started := time.Now()
	err := engine.Run() // The production writer attempts a Put on a closed table.
	require.ErrorContains(t, err, "failed to write data")
	require.Less(t, time.Since(started), 2*time.Second)
	_, original := engine.errorMonitor.IsOk()
	require.ErrorIs(t, err, original)
	select {
	case <-engine.dataTracker.doneChan:
	default:
		t.Fatal("tracker still running after Run returned")
	}
	count := engine.metrics.writeCount.Load()
	require.Zero(t, count)
}

func TestRepairM_1170_CLIErrorStatus(t *testing.T) {
	cfg, filename := repairM1170Config(t)
	repairM1170Seed(t, cfg, filename, false)
	require.NoError(t, os.RemoveAll(cfg.LittConfig.Paths[0])) // Lost storage, intact real metadata.
	var stderr bytes.Buffer
	require.Equal(t, 1, RunCLI([]string{filename}, &stderr))
	require.Contains(t, stderr.String(), "not found in database")
	require.NotContains(t, stderr.String(), "Failed to create benchmark engine")
}

func TestRepairM_1170_MissingReadSuccess(t *testing.T) {
	for _, panicOnFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(panicOnFailure), func(t *testing.T) {
			cfg, filename := repairM1170Config(t)
			cfg.PanicOnReadFailure = panicOnFailure
			repairM1170Seed(t, cfg, filename, false)
			require.NoError(t, os.RemoveAll(cfg.LittConfig.Paths[0]))
			engine := repairM1170Open(t, cfg, filename)
			require.ErrorContains(t, engine.Run(), "not found in database")
			require.Positive(t, engine.metrics.missingReadCount.Load())
			require.Zero(t, engine.metrics.readCount.Load())
			require.Zero(t, engine.metrics.bytesRead.Load())
		})
	}
}

func TestRepairM_1170_CorruptReadSuccess(t *testing.T) {
	cfg, filename := repairM1170Config(t)
	repairM1170Seed(t, cfg, filename, true)
	engine := repairM1170Open(t, cfg, filename)
	require.ErrorContains(t, engine.Run(), "does not match expected value")
	require.Positive(t, engine.metrics.invalidReadCount.Load())
	require.Zero(t, engine.metrics.readCount.Load())
	require.Zero(t, engine.metrics.bytesRead.Load())
	require.Zero(t, engine.metrics.missingReadCount.Load())
}

func TestRepairM_1170_ShutdownDrainRestart(t *testing.T) {
	cfg, filename := repairM1170Config(t)
	cfg.WriterParallelism = 4
	cfg.ReaderParallelism = 4
	cfg.MaximumWriteThroughputMB = 0.02
	cfg.BatchSizeMB = 512.0 / unit.MB
	engine := repairM1170Open(t, cfg, filename)
	require.NoError(t, engine.Run())
	require.Positive(t, engine.metrics.writeCount.Load())
	require.Positive(t, engine.metrics.readCount.Load())
	require.Zero(t, engine.metrics.missingReadCount.Load())
	select {
	case <-engine.dataTracker.doneChan:
	default:
		t.Fatal("tracker not joined")
	}
	writes := engine.metrics.writeCount.Load()
	highest := engine.dataTracker.highestWrittenKeyIndex
	reported := make([]uint64, 0, writes)
	for index := int64(0); index <= highest; index++ {
		reported = append(reported, uint64(index))
	}
	for index := range engine.dataTracker.writtenKeysSet {
		reported = append(reported, index)
	}
	require.Equal(t, writes, uint64(len(reported)), "every accepted write must be flushed and reported")
	_, err := engine.db.BuildTable(litt.DefaultTableConfig("after_close"))
	require.ErrorContains(t, err, "database is closing or closed")
	reopened := repairM1170Open(t, cfg, filename)
	for _, index := range reported {
		expected := reopened.dataTracker.generator
		got, exists, err := reopened.table.Get(expected.Key(index))
		require.NoError(t, err)
		require.True(t, exists, "accepted key %d lost across shutdown", index)
		require.Equal(t, expected.Value(index, 32), got)
	}
	read := reopened.dataTracker.GetReadInfoWithTimeout(time.Second)
	require.NotNil(t, read)
	actual, exists, err := reopened.table.Get(read.Key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, read.Value, actual)
	reopened.dataTracker.Close()
	complete := 0
	for _, cohort := range reopened.dataTracker.cohorts {
		if cohort.IsComplete() {
			complete++
		}
	}
	require.Positive(t, complete)
}

func TestRepairM_1170_ReadSafetyDeadline(t *testing.T) {
	cfg, filename := repairM1170Config(t)
	cfg.TimeLimitSeconds = 0.08
	cfg.TTLHours = 2.0 / 3600
	cfg.ReadSafetyMarginMinutes = 0
	repairM1170Seed(t, cfg, filename, false)
	engine := repairM1170Open(t, cfg, filename)
	queued := engine.dataTracker.GetReadInfoWithTimeout(time.Second)
	require.NotNil(t, queued)
	// Wait until the real producer's safety deadline. The queue still contains
	// requests made before expiration; no new writes can prolong those requests.
	time.Sleep(time.Until(queued.ValidUntil) + time.Millisecond)
	engine.config.WriterParallelism = 0
	require.NoError(t, engine.Run())
	require.Positive(t, engine.metrics.expiredReadCount.Load())
	require.Zero(t, engine.metrics.missingReadCount.Load())
	require.Zero(t, engine.metrics.readCount.Load())
	require.Zero(t, engine.metrics.bytesRead.Load())
}

func TestRepairM_1170_CloseError(t *testing.T) {
	cfg, _ := repairM1170Config(t)
	runtime := litt.DefaultRuntimeConfig()
	dbCtx, stopDB := context.WithCancel(context.Background())
	defer stopDB()
	runtime.CTX = dbCtx
	runtime.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	database, err := littbuilder.NewDB(cfg.LittConfig, runtime)
	require.NoError(t, err)
	table, err := database.BuildTable(litt.DefaultTableConfig("benchmark"))
	require.NoError(t, err)
	monitor := util.NewErrorMonitor(t.Context(), runtime.Logger, nil)
	tracker, err := NewDataTracker(t.Context(), cfg, monitor)
	require.NoError(t, err)
	defer tracker.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopDB() // The real DB control loop cannot acknowledge its close request.
	engine := &BenchmarkEngine{ctx: ctx, cancel: cancel, config: cfg, logger: runtime.Logger, db: database, table: table, dataTracker: tracker, errorMonitor: monitor}
	require.ErrorContains(t, engine.Run(), "failed to close database")
	select {
	case <-tracker.doneChan:
	default:
		t.Fatal("close error bypassed tracker drain")
	}
}
