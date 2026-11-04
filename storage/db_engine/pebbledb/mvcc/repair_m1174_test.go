package mvcc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	smetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
)

func repair1174Open(t *testing.T, dir string, cfg config.StateStoreConfig) *Database {
	t.Helper()
	store, err := OpenDB(dir, cfg)
	require.NoError(t, err)
	db := store.(*Database)
	db.metricsCancel()
	db.metricsWG.Wait()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func repair1174SeedLegacy(t *testing.T, dir string) {
	t.Helper()
	raw, err := pebble.Open(dir, &pebble.Options{Comparer: MVCCComparer})
	require.NoError(t, err)
	batch, err := NewBatch(raw, 0, false)
	require.NoError(t, err)
	require.NoError(t, batch.Write())
	require.NoError(t, raw.Close())
}

func repair1174Scraper(t *testing.T) func() map[string]*dto.MetricFamily {
	t.Helper()
	previousProvider, previousMetrics := otel.GetMeterProvider(), otelMetrics
	registry, shutdown, err := smetrics.SetupOtelPrometheus()
	require.NoError(t, err)
	otelMetrics = newMVCCMetrics(otel.Meter("paxdb_pebble"))
	t.Cleanup(func() {
		otelMetrics = previousMetrics
		otel.SetMeterProvider(previousProvider)
		require.NoError(t, shutdown(context.Background()))
	})
	server := httptest.NewServer(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 5 * time.Second}
	return func() map[string]*dto.MetricFamily {
		t.Helper()
		response, err := client.Get(server.URL + "/metrics")
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		parser := expfmt.NewTextParser(model.UTF8Validation)
		families, err := parser.TextToMetricFamilies(response.Body)
		require.NoError(t, err)
		return families
	}
}

// Pebble's wire batch has a 12-byte header, a kind byte per operation,
// and varint-length-prefixed keys and values. Metadata uses the same encoding.
func repair1174EncodedBytes(ops []batchOp, metadata bool) int64 {
	var scratch [binary.MaxVarintLen64]byte
	field := func(n int) int64 {
		return int64(binary.PutUvarint(scratch[:], uint64(n)) + n)
	}
	total := int64(12)
	for _, op := range ops {
		total += 1 + field(len(op.key))
		if !op.delete {
			total += field(len(op.value))
		}
	}
	if metadata {
		total += 1 + field(len(latestVersionKey)) + field(VersionSize)
	}
	return total
}

func TestRepairM_1174_BatchBytes(t *testing.T) {
	scrape := repair1174Scraper(t)
	db := repair1174Open(t, t.TempDir(), config.DefaultStateStoreConfig())
	var total int64
	var count uint64
	sizes := make([]int64, 0, 2)
	for i, size := range []int{3, 4096} {
		value := bytes.Repeat([]byte{byte(i + 1)}, size)
		changes := []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("same"), Value: value}}}}}
		expectedBatch, err := NewBatch(db.storage, int64(i+1), db.descending)
		require.NoError(t, err)
		require.NoError(t, expectedBatch.Set("bank", []byte("same"), value))
		sizeBytes := repair1174EncodedBytes(expectedBatch.ops, true)
		sizes = append(sizes, sizeBytes)
		require.NoError(t, db.ApplyChangesetSync(int64(i+1), changes))
		got, err := db.Get("bank", int64(i+1), []byte("same"))
		require.NoError(t, err)
		require.Equal(t, value, got)
		total += sizeBytes
		count++
		histogram := proofNodeHistogram(t, scrape(), "pebble_batch_size_bytes", "", count, smetrics.ByteSizeBuckets)
		require.Equal(t, float64(total), histogram.GetSampleSum())
	}
	require.Greater(t, sizes[1]-sizes[0], int64(4096-3), "the larger value also grows its varint length")
	raw, err := NewRawBatch(db.storage, db.descending)
	require.NoError(t, err)
	require.NoError(t, raw.Set("bank", []byte("raw"), []byte("value"), 3))
	total += repair1174EncodedBytes(raw.ops, false)
	require.NoError(t, raw.Write())
	count++
	deletion, err := NewBatch(db.storage, 3, db.descending)
	require.NoError(t, err)
	require.NoError(t, deletion.HardDelete("bank", []byte("raw")))
	total += repair1174EncodedBytes(deletion.ops, true)
	require.NoError(t, deletion.Write())
	count++
	histogram := proofNodeHistogram(t, scrape(), "pebble_batch_size_bytes", "", count, smetrics.ByteSizeBuckets)
	require.Equal(t, float64(total), histogram.GetSampleSum())
}

func TestRepairM_1174_MetricsCloseRace(t *testing.T) {
	store, err := OpenDB(t.TempDir(), config.DefaultStateStoreConfig())
	require.NoError(t, err)
	db := store.(*Database)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.ApplyChangesetAsync(1, []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("k"), Value: []byte("v")}}}}}))
	db.WaitForPendingWrites()
	// Hold the snapshot's storage lock until the actual ten-second metrics
	// worker has entered its poll. Shutdown must let that participant finish.
	db.storageMu.Lock()
	locked := true
	defer func() {
		if locked {
			db.storageMu.Unlock()
		}
	}()
	require.Eventually(t, func() bool {
		if db.metricsMu.TryLock() {
			db.metricsMu.Unlock()
			return false
		}
		return true
	}, 12*time.Second, time.Millisecond)
	closed := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { closed <- db.Close() }()
	}
	select {
	case err := <-closed:
		t.Fatalf("close returned before the active metrics poll finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	db.storageMu.Unlock()
	locked = false
	for i := 0; i < 8; i++ {
		select {
		case err := <-closed:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown did not join its participants")
		}
	}
	require.Nil(t, db.PebbleMetrics())
	db.collectAndRecordMetrics(context.Background())
	require.NoError(t, db.Close())
}

func TestRepairM_1174_MetadataVersions(t *testing.T) {
	overflow := make([]byte, VersionSize)
	binary.LittleEndian.PutUint64(overflow, uint64(math.MaxInt64)+1)
	cases := []struct {
		name  string
		value []byte
		valid bool
	}{
		{"empty", []byte{}, false}, {"oversized", make([]byte, 9), false},
		{"overflow", overflow, false}, {"zero", make([]byte, VersionSize), true},
	}
	maximum := make([]byte, VersionSize)
	binary.LittleEndian.PutUint64(maximum, math.MaxInt64)
	cases = append(cases, struct {
		name  string
		value []byte
		valid bool
	}{"maximum", maximum, true})
	for size := 1; size < VersionSize; size++ {
		cases = append(cases, struct {
			name  string
			value []byte
			valid bool
		}{fmt.Sprintf("short-%d", size), bytes.Repeat([]byte{1}, size), false})
	}
	for _, key := range []string{latestVersionKey, earliestVersionKey} {
		for _, tc := range cases {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				cfg := config.DefaultStateStoreConfig()
				db := repair1174Open(t, dir, cfg)
				require.NoError(t, db.Close())
				raw, err := pebble.Open(dir, &pebble.Options{Comparer: MVCCComparer})
				require.NoError(t, err)
				require.NoError(t, raw.Set([]byte(key), tc.value, pebble.Sync))
				require.NoError(t, raw.Close())
				var reopened types.StateStore
				require.NotPanics(t, func() { reopened, err = OpenDB(dir, cfg) })
				if tc.valid {
					require.NoError(t, err)
					want := int64(binary.LittleEndian.Uint64(tc.value))
					if key == latestVersionKey {
						require.Equal(t, want, reopened.GetLatestVersion())
					} else {
						require.Equal(t, want, reopened.GetEarliestVersion())
					}
					require.NoError(t, reopened.Close())
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), key)
					require.Nil(t, reopened)
					if tc.name == "overflow" {
						require.Contains(t, err.Error(), "overflows")
					} else {
						require.Contains(t, err.Error(), "bytes")
					}
					// A failed open releases its files and does not silently repair corruption.
					raw, openErr := pebble.Open(dir, &pebble.Options{Comparer: MVCCComparer})
					require.NoError(t, openErr)
					value, closer, getErr := raw.Get([]byte(key))
					require.NoError(t, getErr)
					require.Equal(t, tc.value, value)
					require.NoError(t, closer.Close())
					require.NoError(t, raw.Close())
				}
			})
		}
	}
	t.Run("absent", func(t *testing.T) {
		db := repair1174Open(t, t.TempDir(), config.DefaultStateStoreConfig())
		require.Zero(t, db.GetLatestVersion())
		require.Zero(t, db.GetEarliestVersion())
	})
}

func repair1174Versions(t *testing.T, db *Database, module string, key []byte) []int64 {
	t.Helper()
	var versions []int64
	stopped, err := db.iteratePhysical(module, func(got, _ []byte, version int64) (bool, error) {
		if bytes.Equal(got, key) {
			versions = append(versions, version)
		}
		return false, nil
	})
	require.NoError(t, err)
	require.False(t, stopped)
	return versions
}

func TestRepairM_1174_PruneDirtyMapRestart(t *testing.T) {
	for _, keepLast := range []bool{true, false} {
		for _, mode := range []string{"populated", "reopened", "imported"} {
			t.Run(fmt.Sprintf("%s/keep-last-%t", mode, keepLast), func(t *testing.T) {
				cfg := config.DefaultStateStoreConfig()
				cfg.KeepLastVersion = keepLast
				dir := t.TempDir()
				db := repair1174Open(t, dir, cfg)
				if mode == "imported" {
					applyVersion(t, db, "dormant", 1, []byte("k"), []byte("seed"))
					require.NoError(t, db.Prune(5))
				}
				for _, version := range []int64{10, 20, 30} {
					if mode == "imported" {
						// The snapshot is produced by a real versioned store and its iterator.
						source := repair1174Open(t, t.TempDir(), cfg)
						for _, module := range []string{"active", "dormant"} {
							applyVersion(t, source, module, version, []byte("k"), []byte(fmt.Sprint(version)))
						}
						nodes := make(chan types.SnapshotNode, 2)
						for _, module := range []string{"active", "dormant"} {
							itr, err := source.Iterator(module, version, nil, nil)
							require.NoError(t, err)
							for ; itr.Valid(); itr.Next() {
								nodes <- types.SnapshotNode{StoreKey: module, Key: bytes.Clone(itr.Key()), Value: bytes.Clone(itr.Value())}
							}
							require.NoError(t, itr.Close())
						}
						close(nodes)
						require.NoError(t, db.Import(version, nodes))
						require.NoError(t, source.Close())
					} else {
						for _, module := range []string{"active", "dormant"} {
							applyVersion(t, db, module, version, []byte("k"), []byte(fmt.Sprint(version)))
						}
					}
				}
				initial := []int64{10, 20, 30}
				if mode == "imported" && keepLast {
					initial = append(initial, 1)
				}
				require.ElementsMatch(t, initial, repair1174Versions(t, db, "dormant", []byte("k")))
				if mode == "reopened" {
					require.NoError(t, db.Close())
					db = repair1174Open(t, dir, cfg)
				}
				applyVersion(t, db, "active", 40, []byte("k"), []byte("40"))
				require.NoError(t, db.Prune(25))
				want := []int64{30}
				if keepLast {
					want = append(want, 20)
				}
				require.ElementsMatch(t, want, repair1174Versions(t, db, "dormant", []byte("k")))
				require.Equal(t, int64(26), db.GetEarliestVersion())
				require.NoError(t, db.Close())
				db = repair1174Open(t, dir, cfg)
				require.ElementsMatch(t, want, repair1174Versions(t, db, "dormant", []byte("k")))
				require.NoError(t, db.Prune(35))
				var retained []int64
				if keepLast {
					retained = []int64{30}
				}
				require.ElementsMatch(t, retained, repair1174Versions(t, db, "dormant", []byte("k")))
				require.NoError(t, db.Close())
				db = repair1174Open(t, dir, cfg)
				require.Equal(t, int64(36), db.GetEarliestVersion())
				require.ElementsMatch(t, retained, repair1174Versions(t, db, "dormant", []byte("k")))
			})
		}
	}
}

func TestRepairM_1174_RawExportModuleIdentity(t *testing.T) {
	for _, mode := range []string{"descending", "legacy", "default-comparer"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.DefaultStateStoreConfig()
			cfg.UseDefaultComparer = mode == "default-comparer"
			dir := t.TempDir()
			if mode == "legacy" {
				repair1174SeedLegacy(t, dir)
			}
			source := repair1174Open(t, dir, cfg)
			key := []byte{'/', 'k', 0, 255}
			modules := []string{"b", "long_module"}
			for _, version := range []int64{10, 20} {
				for _, module := range modules {
					applyVersion(t, source, module, version, key, []byte(fmt.Sprintf("%s-%d", module, version)))
				}
			}
			targetDir := t.TempDir()
			if mode == "legacy" {
				repair1174SeedLegacy(t, targetDir)
			}
			target := repair1174Open(t, targetDir, cfg)
			raw, err := NewRawBatch(target.storage, target.descending)
			require.NoError(t, err)
			snapshots := make(chan types.SnapshotNode, 2)
			seen := map[string]int{}
			stopped, err := source.RawIterate("", func(qualified, value []byte, version int64) bool {
				module, parseErr := parseStoreKey(qualified)
				require.NoError(t, parseErr)
				require.Contains(t, modules, module)
				require.Equal(t, prependStoreKey(module, key), qualified)
				require.Equal(t, []byte(fmt.Sprintf("%s-%d", module, version)), value)
				seen[fmt.Sprintf("%s/%d", module, version)]++
				// Empty-store RawBatch consumes the qualified key emitted by RawIterate.
				require.NoError(t, raw.Set("", qualified, value, version))
				if version == 20 {
					snapshots <- types.SnapshotNode{StoreKey: module, Key: bytes.Clone(qualified[len(storePrefix(module)):]), Value: bytes.Clone(value)}
				}
				return false
			})
			require.NoError(t, err)
			require.False(t, stopped)
			require.Len(t, seen, 4)
			for _, count := range seen {
				require.Equal(t, 1, count)
			}
			require.NoError(t, raw.Write())
			require.NoError(t, target.SetLatestVersion(20))
			close(snapshots)
			imported := repair1174Open(t, t.TempDir(), cfg)
			require.NoError(t, imported.Import(20, snapshots))
			require.NoError(t, target.Close())
			target = repair1174Open(t, targetDir, cfg)
			for _, module := range modules {
				for _, version := range []int64{10, 20} {
					got, err := target.Get(module, version, key)
					require.NoError(t, err)
					require.Equal(t, []byte(fmt.Sprintf("%s-%d", module, version)), got)
				}
				got, err := imported.Get(module, 20, key)
				require.NoError(t, err)
				require.Equal(t, []byte(fmt.Sprintf("%s-20", module)), got)
				count := 0
				_, err = source.RawIterate(module, func(got, _ []byte, _ int64) bool { require.Equal(t, key, got); count++; return false })
				require.NoError(t, err)
				require.Equal(t, 2, count)
			}
			stopped, err = source.RawIterate("", func(_, _ []byte, _ int64) bool { return true })
			require.NoError(t, err)
			require.True(t, stopped)
		})
	}
}

func TestRepairM_1174_RollbackLeavesTombstones(t *testing.T) {
	for _, mode := range []string{"descending", "legacy", "default-comparer"} {
		for _, count := range []int{1, 49, 50, 51, 101} {
			t.Run(fmt.Sprintf("%s/keys-%d", mode, count), func(t *testing.T) {
				cfg := config.DefaultStateStoreConfig()
				cfg.UseDefaultComparer = mode == "default-comparer"
				dir := t.TempDir()
				if mode == "legacy" {
					repair1174SeedLegacy(t, dir)
				}
				db := repair1174Open(t, dir, cfg)
				initial, changed := make([]*proto.KVPair, 0, count), make([]*proto.KVPair, 0, count)
				for i := 0; i < count; i++ {
					key := []byte(fmt.Sprintf("k-%03d", i))
					initial = append(initial, &proto.KVPair{Key: key, Value: []byte("before")})
					var value []byte
					if i%2 == 1 {
						value = []byte("after")
					}
					changed = append(changed, &proto.KVPair{Key: key, Value: value})
				}
				require.NoError(t, db.ApplyChangesetSync(1, []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: initial}}}))
				require.NoError(t, db.ApplyChangesetSync(2, []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: changed}}, {Name: "other", Changeset: proto.ChangeSet{Pairs: initial}}}))
				for i, pair := range changed {
					got, err := db.Get("bank", 2, pair.Key)
					require.NoError(t, err)
					if i%2 == 0 {
						require.Nil(t, got)
					} else {
						require.Equal(t, []byte("after"), got)
					}
					require.Contains(t, repair1174Versions(t, db, "bank", pair.Key), int64(2))
				}
				// Physical recovery must not raise a caller's explicitly reset head.
				require.NoError(t, db.SetLatestVersion(1))
				require.NoError(t, db.DeleteKeysAtVersion("bank", 2))
				require.NoError(t, db.DeleteKeysAtVersion("bank", 2))
				require.NoError(t, db.DeleteKeysAtVersion("bank", 99))
				require.Error(t, db.DeleteKeysAtVersion("bank", -1))
				check := func() {
					require.Equal(t, int64(1), db.GetLatestVersion())
					for _, pair := range initial {
						require.ElementsMatch(t, []int64{1}, repair1174Versions(t, db, "bank", pair.Key))
						got, err := db.Get("bank", 2, pair.Key)
						require.NoError(t, err)
						require.Equal(t, pair.Value, got)
						require.ElementsMatch(t, []int64{2}, repair1174Versions(t, db, "other", pair.Key))
					}
				}
				check()
				require.NoError(t, db.Close())
				db = repair1174Open(t, dir, cfg)
				check()
				require.NoError(t, db.DeleteKeysAtVersion("", 2))
				for _, pair := range initial {
					require.Empty(t, repair1174Versions(t, db, "other", pair.Key))
				}
				require.Equal(t, int64(1), db.GetLatestVersion())
			})
		}
	}
}

func TestRepairM_1174_RollbackCallbackErrors(t *testing.T) {
	for _, descending := range []bool{true, false} {
		t.Run(fmt.Sprint(descending), func(t *testing.T) {
			dir := t.TempDir()
			cfg := config.DefaultStateStoreConfig()
			if !descending {
				repair1174SeedLegacy(t, dir)
			}
			db := repair1174Open(t, dir, cfg)
			pairs := make([]*proto.KVPair, 2*DeleteCommitBatchSize+1)
			for i := range pairs {
				pairs[i] = &proto.KVPair{Key: []byte(fmt.Sprintf("k-%03d", i)), Value: []byte("value")}
			}
			require.NoError(t, db.ApplyChangesetSync(2, []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: pairs}}}))
			require.NoError(t, db.Close())
			raw, err := pebble.Open(dir, &pebble.Options{Comparer: MVCCComparer, ReadOnly: true})
			require.NoError(t, err)
			readonly := &Database{storage: raw, descending: descending}
			t.Cleanup(func() { require.NoError(t, readonly.Close()) })
			err = readonly.DeleteKeysAtVersion("bank", 2)
			require.ErrorIs(t, err, pebble.ErrReadOnly)
			require.Contains(t, err.Error(), "after 0 committed deletions")
			require.Contains(t, err.Error(), "earlier batches remain committed")
			// Drive the same physical callback transport with an actual failed
			// intermediate commit and prove it propagates immediately, before EOF.
			batch := raw.NewBatch()
			defer batch.Close()
			visited := 0
			_, err = readonly.iteratePhysical("bank", func(key, _ []byte, version int64) (bool, error) {
				visited++
				if err := batch.Delete(readonly.mvccEncode(prependStoreKey("bank", key), version), nil); err != nil {
					return false, err
				}
				if visited == DeleteCommitBatchSize {
					return false, batch.Commit(pebble.NoSync)
				}
				return false, nil
			})
			require.ErrorIs(t, err, pebble.ErrReadOnly)
			require.Equal(t, DeleteCommitBatchSize, visited)
			head, err := retrieveLatestVersion(raw)
			require.NoError(t, err)
			require.Equal(t, int64(2), head)
			require.NoError(t, readonly.Close())
			db = repair1174Open(t, dir, cfg)
			for _, pair := range pairs {
				require.Equal(t, []int64{2}, repair1174Versions(t, db, "bank", pair.Key))
				got, err := db.Get("bank", 2, pair.Key)
				require.NoError(t, err)
				require.Equal(t, pair.Value, got)
			}
		})
	}
}

func repair1174CounterSnapshot(m *pebble.Metrics) map[string]float64 {
	values := map[string]float64{
		"pebble_compaction_count_total":          float64(m.Compact.Count),
		"pebble_flush_count_total":               float64(m.Flush.Count),
		"pebble_flush_bytes_written_bytes_total": float64(m.Flush.WriteThroughput.Bytes),
		"pebble_cache_hits_total":                float64(m.BlockCache.Hits),
		"pebble_cache_misses_total":              float64(m.BlockCache.Misses),
	}
	for _, level := range m.Levels {
		values["pebble_compaction_bytes_read_bytes_total"] += float64(level.TableBytesIn)
		values["pebble_compaction_bytes_written_bytes_total"] += float64(level.TableBytesCompacted)
	}
	return values
}

func TestRepairM_1174_CounterDoubleCount(t *testing.T) {
	scrape := repair1174Scraper(t)
	cfg := config.DefaultStateStoreConfig()
	dir := t.TempDir()
	db := repair1174Open(t, dir, cfg)
	activity := func(version int64) {
		pairs := make([]*proto.KVPair, 32)
		for i := range pairs {
			pairs[i] = &proto.KVPair{Key: []byte(fmt.Sprintf("k-%03d", i)), Value: bytes.Repeat([]byte(fmt.Sprintf("value-%d-%d", version, i)), 512)}
		}
		require.NoError(t, db.ApplyChangesetSync(version, []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: pairs}}}))
		require.NoError(t, db.storage.Flush())
		// Overlapping SSTables require actual compaction rather than a table move.
		require.NoError(t, db.ApplyChangesetSync(version, []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: pairs}}}))
		require.NoError(t, db.storage.Flush())
		require.NoError(t, db.storage.Compact(context.Background(), db.mvccEncode(storePrefix("bank"), 0), db.mvccEncode(prefixEnd(storePrefix("bank")), 0), false))
		for repeat := 0; repeat < 2; repeat++ {
			for _, pair := range pairs {
				got, err := db.Get("bank", version, pair.Key)
				require.NoError(t, err)
				require.Equal(t, pair.Value, got)
			}
		}
	}
	assertCounters := func(want map[string]float64) {
		t.Helper()
		db.collectAndRecordMetrics(context.Background())
		observed := scrape()
		for name, value := range want {
			require.Equal(t, value, proofNodeCounter(t, observed, name), name)
		}
		db.collectAndRecordMetrics(context.Background())
		repeated := scrape()
		for name := range want {
			require.Equal(t, observed[name], repeated[name], "unchanged snapshot changed %s", name)
		}
	}
	activity(1)
	first := repair1174CounterSnapshot(db.PebbleMetrics())
	require.Positive(t, first["pebble_cache_hits_total"])
	require.Positive(t, first["pebble_cache_misses_total"])
	require.Positive(t, first["pebble_flush_count_total"])
	require.Positive(t, first["pebble_compaction_bytes_read_bytes_total"])
	assertCounters(first)
	activity(2)
	second := repair1174CounterSnapshot(db.PebbleMetrics())
	require.Greater(t, second["pebble_flush_count_total"], first["pebble_flush_count_total"])
	require.Greater(t, second["pebble_cache_hits_total"], first["pebble_cache_hits_total"])
	assertCounters(second)
	require.NoError(t, db.Close())
	db = repair1174Open(t, dir, cfg)
	activity(3)
	reopened := repair1174CounterSnapshot(db.PebbleMetrics())
	for name, value := range second {
		reopened[name] += value
	}
	assertCounters(reopened)
	// Multiple poll callers share the same prior snapshot.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); db.collectAndRecordMetrics(context.Background()) }()
	}
	wg.Wait()
	for name, value := range reopened {
		require.Equal(t, value, proofNodeCounter(t, scrape(), name), name)
	}
}
