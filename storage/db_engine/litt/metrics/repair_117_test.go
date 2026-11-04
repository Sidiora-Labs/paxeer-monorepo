package metrics_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	commonmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/dbcache"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/metrics"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
)

func TestRepair_117_DiskReadMetricSemantics(t *testing.T) {
	previousProvider := otel.GetMeterProvider()
	registry, shutdown, err := commonmetrics.SetupOtelPrometheus()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, shutdown(context.Background()))
		otel.SetMeterProvider(previousProvider)
	})
	reporter := metrics.NewLittDBMetrics()
	root := t.TempDir()
	const tableName = "repair117"
	keymapPath := filepath.Join(root, tableName, keymap.KeymapDirectoryName)
	require.NoError(t, os.MkdirAll(keymapPath, 0750))
	typeFile := keymap.NewKeymapTypeFile(keymapPath, keymap.MemKeymapType)
	require.NoError(t, typeFile.Write())
	runtimeConfig := litt.DefaultRuntimeConfig()
	keys, _, err := keymap.NewMemKeymap(runtimeConfig.Logger, "", true)
	require.NoError(t, err)
	config, err := litt.DefaultConfig(root)
	require.NoError(t, err)
	tableConfig := litt.DefaultTableConfig(tableName)
	tableConfig.ShardingFactor = 1
	base, err := disktable.NewDiskTable(config, runtimeConfig, tableName, tableConfig,
		keys, keymapPath, typeFile, []string{root}, true, reporter)
	require.NoError(t, err)
	writeCache := util.NewFIFOCache[string, []byte](1024, nil, reporter.GetWriteCacheMetrics())
	readCache := util.NewFIFOCache[string, []byte](1024, nil, reporter.GetReadCacheMetrics())
	table := dbcache.NewCachedTable(base, writeCache, readCache, reporter)
	t.Cleanup(func() { require.NoError(t, table.Close()) })

	key, value := []byte("key"), []byte("disk and memory reads")
	require.NoError(t, table.Put(key, value))
	read := func(t *testing.T, cacheOnly, wantHot bool) {
		t.Helper()
		got, exists, hot, err := table.CacheAwareGet(key, cacheOnly)
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, wantHot, hot)
		require.Equal(t, value, got)
	}
	check := func(t *testing.T, logical, disk, hits uint64) {
		t.Helper()
		assertRepair117Samples(t, registry, tableName, map[string]float64{
			"litt_logical_bytes_read_bytes_total": float64(logical * uint64(len(value))),
			"litt_logical_keys_read_total":        float64(logical),
			"litt_bytes_read_bytes_total":         float64(disk * uint64(len(value))),
			"litt_keys_read_total":                float64(disk),
			"litt_cache_hits_total":               float64(hits),
			"litt_cache_misses_total":             float64(disk),
			"litt_read_latency_seconds":           float64(logical),
			"litt_cache_miss_latency_seconds":     float64(disk),
		})
	}

	t.Run("write_cache", func(t *testing.T) {
		read(t, true, true)
		check(t, 1, 0, 1)
	})
	t.Run("unflushed_memory", func(t *testing.T) {
		require.NoError(t, table.SetWriteCacheSize(0))
		require.Zero(t, writeCache.Size())
		require.Zero(t, readCache.Size())
		read(t, true, true)
		check(t, 2, 0, 2)
	})
	t.Run("disk", func(t *testing.T) {
		require.NoError(t, table.Flush())
		require.NoError(t, table.SetReadCacheSize(0))
		require.Zero(t, readCache.Size())
		got, exists, hot, err := table.CacheAwareGet(key, true)
		require.NoError(t, err)
		require.True(t, exists)
		require.False(t, hot)
		require.Nil(t, got)
		check(t, 2, 0, 2)
		require.NoError(t, table.SetReadCacheSize(1024))
		read(t, false, false)
		check(t, 3, 1, 2)
	})
	t.Run("read_cache", func(t *testing.T) {
		require.Equal(t, 1, readCache.Size())
		read(t, true, true)
		check(t, 4, 1, 3)
	})
	t.Run("absent_key", func(t *testing.T) {
		got, exists, hot, err := table.CacheAwareGet([]byte("absent"), false)
		require.NoError(t, err)
		require.False(t, exists)
		require.False(t, hot)
		require.Nil(t, got)
		check(t, 4, 1, 3)
	})
}

func assertRepair117Samples(t *testing.T, registry *prometheus.Registry, table string, expected map[string]float64) {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	for name, want := range expected {
		var matches int
		var got float64
		for _, family := range families {
			if family.GetName() != name {
				continue
			}
			if name == "litt_bytes_read_bytes_total" {
				require.Equal(t, "The number of bytes read from disk since startup.", family.GetHelp())
			}
			if name == "litt_keys_read_total" {
				require.Equal(t, "The number of keys read from disk since startup.", family.GetHelp())
			}
			for _, sample := range family.GetMetric() {
				for _, label := range sample.GetLabel() {
					if label.GetName() == "table" && label.GetValue() == table {
						matches++
						switch family.GetType() {
						case dto.MetricType_COUNTER:
							got = sample.GetCounter().GetValue()
						case dto.MetricType_HISTOGRAM:
							got = float64(sample.GetHistogram().GetSampleCount())
						default:
							t.Fatalf("unexpected metric type for %s: %v", name, family.GetType())
						}
					}
				}
			}
		}
		if want == 0 {
			require.LessOrEqual(t, matches, 1, name)
		} else {
			require.Equal(t, 1, matches, "missing or duplicated exported sample %s", name)
		}
		require.Equal(t, want, got, name)
	}
}
