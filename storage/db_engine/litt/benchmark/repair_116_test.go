package benchmark

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/unit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/benchmark/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

func TestRepair_116_NewCohortSingletonRoundTrip(t *testing.T) {
	for _, keyIndex := range []uint64{0, 42, math.MaxUint64} {
		t.Run(fmt.Sprint(keyIndex), func(t *testing.T) {
			cohort, err := NewCohort(t.TempDir(), 7, keyIndex, keyIndex, 64, true)
			require.NoError(t, err)
			proveRepair116Singleton(t, cohort, keyIndex)
		})
	}
}

func TestRepair_116_NextCohortSingletonRoundTrip(t *testing.T) {
	for _, highIndex := range []uint64{0, 41, math.MaxUint64 - 1} {
		t.Run(fmt.Sprint(highIndex), func(t *testing.T) {
			previous, err := NewCohort(t.TempDir(), 7, highIndex, highIndex, 32, true)
			require.NoError(t, err)
			previous, err = LoadCohort(previous.Path())
			require.NoError(t, err)
			cohort, err := previous.NextCohort(1, 64)
			require.NoError(t, err)
			require.Equal(t, uint64(8), cohort.CohortIndex())
			proveRepair116Singleton(t, cohort, highIndex+1)
		})
	}
}

func proveRepair116Singleton(t *testing.T, cohort *Cohort, keyIndex uint64) {
	t.Helper()
	require.NoError(t, cohort.Write())
	loaded, err := LoadCohort(cohort.Path())
	require.NoError(t, err)
	require.Equal(t, cohort.CohortIndex(), loaded.CohortIndex())
	require.Equal(t, keyIndex, loaded.LowKeyIndex())
	require.Equal(t, keyIndex, loaded.HighKeyIndex())
	require.Equal(t, uint64(64), loaded.ValueSize())
	require.Equal(t, cohort.FirstValueTimestamp().Unix(), loaded.FirstValueTimestamp().Unix())
	require.True(t, loaded.IsLoadedFromDisk())
	require.False(t, loaded.IsComplete())
	require.True(t, loaded.IsExpired(time.Now(), time.Hour))
	_, err = loaded.GetKeyIndexForWriting()
	require.Error(t, err)
	_, err = loaded.GetKeyIndexForReading(rand.New(rand.NewSource(116)))
	require.Error(t, err)

	require.False(t, cohort.IsExhausted())
	require.Error(t, cohort.MarkComplete())
	key, err := cohort.GetKeyIndexForWriting()
	require.NoError(t, err)
	require.Equal(t, keyIndex, key)
	require.True(t, cohort.IsExhausted())
	_, err = cohort.GetKeyIndexForWriting()
	require.Error(t, err)
	require.NoError(t, cohort.MarkComplete())
	require.NoError(t, cohort.Write())

	loaded, err = LoadCohort(cohort.Path())
	require.NoError(t, err)
	require.True(t, loaded.IsComplete())
	require.True(t, loaded.IsLoadedFromDisk())
	require.Equal(t, cohort.CohortIndex(), loaded.CohortIndex())
	require.Equal(t, keyIndex, loaded.LowKeyIndex())
	require.Equal(t, keyIndex, loaded.HighKeyIndex())
	require.Equal(t, cohort.ValueSize(), loaded.ValueSize())
	require.Equal(t, cohort.FirstValueTimestamp().Unix(), loaded.FirstValueTimestamp().Unix())
	for i := 0; i < 8; i++ {
		key, err = loaded.GetKeyIndexForReading(rand.New(rand.NewSource(int64(i))))
		require.NoError(t, err)
		require.Equal(t, keyIndex, key)
	}
	_, err = loaded.GetKeyIndexForWriting()
	require.Error(t, err)
}

func TestRepair_116_InvalidRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		low  uint64
		high uint64
	}{
		{name: "reversed", low: 2, high: 1},
		{name: "count_overflow", low: 0, high: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			cohort, err := NewCohort(directory, 1, tc.low, tc.high, 64, false)
			require.Error(t, err)
			require.Nil(t, cohort)
			files, err := os.ReadDir(directory)
			require.NoError(t, err)
			require.Empty(t, files)

			cohort, err = NewCohort(directory, 1, 1, 2, 64, false)
			require.NoError(t, err)
			data, err := os.ReadFile(cohort.Path())
			require.NoError(t, err)
			binary.BigEndian.PutUint64(data[8:16], tc.low)
			binary.BigEndian.PutUint64(data[16:24], tc.high)
			require.NoError(t, os.WriteFile(cohort.Path(), data, 0600))
			loaded, err := LoadCohort(cohort.Path())
			require.Error(t, err)
			require.Nil(t, loaded)
		})
	}
}

func TestRepair_116_InclusiveBoundaryRanges(t *testing.T) {
	for _, indices := range [][2]uint64{{0, math.MaxUint64 - 1}, {1, math.MaxUint64}, {math.MaxUint64 - 1, math.MaxUint64}} {
		t.Run(fmt.Sprintf("%d_%d", indices[0], indices[1]), func(t *testing.T) {
			cohort, err := NewCohort(t.TempDir(), 1, indices[0], indices[1], 64, false)
			require.NoError(t, err)
			require.NoError(t, cohort.Write())
			loaded, err := LoadCohort(cohort.Path())
			require.NoError(t, err)
			require.Equal(t, indices[0], loaded.LowKeyIndex())
			require.Equal(t, indices[1], loaded.HighKeyIndex())
		})
	}
}

func TestRepair_116_NextCohortOverflow(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cohortIndex uint64
		highIndex   uint64
		keyCount    uint64
	}{
		{name: "zero_keys", highIndex: 10},
		{name: "zero_keys_at_boundary", highIndex: math.MaxUint64},
		{name: "cohort_index_overflow", cohortIndex: math.MaxUint64, highIndex: 10, keyCount: 1},
		{name: "low_index_overflow", highIndex: math.MaxUint64, keyCount: 1},
		{name: "high_index_overflow", highIndex: math.MaxUint64 - 1, keyCount: 2},
		{name: "large_count_overflow", highIndex: 10, keyCount: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			previous, err := NewCohort(directory, tc.cohortIndex, tc.highIndex, tc.highIndex, 64, false)
			require.NoError(t, err)
			before, err := os.ReadFile(previous.Path())
			require.NoError(t, err)
			next, err := previous.NextCohort(tc.keyCount, 64)
			require.Error(t, err)
			require.Nil(t, next)
			after, err := os.ReadFile(previous.Path())
			require.NoError(t, err)
			require.Equal(t, before, after)
			files, err := os.ReadDir(directory)
			require.NoError(t, err)
			require.Len(t, files, 1)
		})
	}
}

func TestRepair_116_SingletonTrackerRestart(t *testing.T) {
	cfg := config.DefaultBenchmarkConfig()
	cfg.MetadataDirectory = t.TempDir()
	cfg.CohortSize = 1
	cfg.RandomPoolSize = unit.MB
	cfg.ValueSizeMB = 1.0 / 1024
	cfg.WriteInfoChanelSize = 1
	cfg.ReadInfoChanelSize = 1
	monitor := util.NewErrorMonitor(t.Context(), slog.Default(), nil)
	tracker, err := NewDataTracker(t.Context(), cfg, monitor)
	require.NoError(t, err)
	t.Cleanup(tracker.Close)
	for i := uint64(0); i < 5; i++ {
		info := tracker.GetWriteInfo()
		require.NotNil(t, info)
		require.Equal(t, i, info.KeyIndex)
		tracker.ReportWrite(info.KeyIndex)
	}
	tracker.Close()
	_, _, cohorts, err := gatherCohorts(tracker.cohortDirectory)
	require.NoError(t, err)
	completed := cohorts[1]
	require.NotNil(t, completed)
	require.Equal(t, uint64(1), completed.LowKeyIndex())
	require.Equal(t, uint64(1), completed.HighKeyIndex())
	require.True(t, completed.IsComplete())
	lastHigh := tracker.activeCohort.HighKeyIndex()

	reopened, err := NewDataTracker(t.Context(), cfg, monitor)
	require.NoError(t, err)
	t.Cleanup(reopened.Close)
	info := reopened.GetWriteInfo()
	require.NotNil(t, info)
	require.Equal(t, lastHigh+1, info.KeyIndex)
	reopened.Close()
	restored := reopened.cohorts[1]
	require.NotNil(t, restored)
	require.True(t, restored.IsLoadedFromDisk())
	require.True(t, restored.IsComplete())
	key, err := restored.GetKeyIndexForReading(rand.New(rand.NewSource(116)))
	require.NoError(t, err)
	require.Equal(t, uint64(1), key)
	ok, err := monitor.IsOk()
	require.NoError(t, err)
	require.True(t, ok)
}
