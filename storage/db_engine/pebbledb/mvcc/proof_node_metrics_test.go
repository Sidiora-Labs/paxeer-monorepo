// Proves: Database.ApplyChangesetSync/ApplyChangesetAsync/collectAndRecordMetrics -> mvccMetrics + metrics.SetupOtelPrometheus + promhttp.HandlerFor -> HTTP scrape consumer (TestProofNodeMetrics).
//
// The store is closed and reopened before the consumer's first scrape; the
// process-local exporter stays alive, as it does when a node reopens a store.
// Replaying the same version preserves the stored state, while operation
// histograms correctly count the replay as another request. Scraping alone
// never increments them. A negative version is refused and exported as failure.
package mvcc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	smetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
)

func TestProofNodeMetrics(t *testing.T) {
	proofNodeMetrics(t)
}

func proofNodeMetrics(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	previousProvider, previousMetrics := otel.GetMeterProvider(), otelMetrics
	registry, shutdown, err := smetrics.SetupOtelPrometheus()
	require.NoError(t, err)
	// Bind the actual package instruments to this node's real exporter. No
	// instrument, storage engine, collector or HTTP handler is a test double.
	otelMetrics = newMVCCMetrics(otel.Meter("paxdb_pebble"))
	t.Cleanup(func() {
		otelMetrics = previousMetrics
		otel.SetMeterProvider(previousProvider)
		require.NoError(t, shutdown(ctx))
	})

	dir := t.TempDir()
	cfg := config.DefaultStateStoreConfig()
	open := func() *Database {
		store, openErr := OpenDB(dir, cfg)
		require.NoError(t, openErr)
		db := store.(*Database)
		// Use the production collection method synchronously so that a ticker
		// cannot introduce an extra cumulative poll during the assertions.
		db.metricsCancel()
		db.metricsWG.Wait()
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return db
	}
	db := open()
	const pairs = 16
	changes := func(version int) []*proto.NamedChangeSet {
		cs := &proto.ChangeSet{}
		for i := 0; i < pairs; i++ {
			cs.Pairs = append(cs.Pairs, &proto.KVPair{
				Key:   []byte(fmt.Sprintf("key-%02d", i)),
				Value: bytes.Repeat([]byte(fmt.Sprintf("value-%d-%02d", version, i)), 256),
			})
		}
		return []*proto.NamedChangeSet{{Name: "proof", Changeset: *cs}}
	}
	first, second := changes(1), changes(2)
	encodedBytes := func(version int64, changes []*proto.NamedChangeSet) int64 {
		// Independently account for Pebble's header, record kinds and
		// varint-prefixed fields, including the committed head metadata.
		var scratch [binary.MaxVarintLen64]byte
		fieldBytes := func(size int) int64 {
			return int64(binary.PutUvarint(scratch[:], uint64(size)) + size)
		}
		total := int64(12) + 1 + fieldBytes(len(latestVersionKey)) + fieldBytes(VersionSize)
		for _, cs := range changes {
			for _, pair := range cs.Changeset.Pairs {
				key := db.mvccEncode(prependStoreKey(cs.Name, pair.Key), version)
				value := db.mvccEncode(pair.Value, 0)
				total += 1 + fieldBytes(len(key)) + fieldBytes(len(value))
			}
		}
		return total
	}
	firstBytes, secondBytes := encodedBytes(1, first), encodedBytes(2, second)
	require.NoError(t, db.ApplyChangesetSync(1, first))
	require.NoError(t, db.storage.Flush())
	require.NoError(t, db.ApplyChangesetAsync(2, second))
	db.WaitForPendingWrites()
	require.NoError(t, db.storage.Flush())
	start := db.mvccEncode(storePrefix("proof"), 0)
	end := db.mvccEncode(prefixEnd(storePrefix("proof")), 0)
	require.NoError(t, db.storage.Compact(ctx, start, end, false))
	assertState := func(db *Database) {
		t.Helper()
		require.Equal(t, int64(2), db.GetLatestVersion())
		for version, cs := range map[int64][]*proto.NamedChangeSet{1: first, 2: second} {
			for _, pair := range cs[0].Changeset.Pairs {
				value, getErr := db.Get("proof", version, pair.Key)
				require.NoError(t, getErr)
				require.Equal(t, pair.Value, value)
			}
		}
		versions := make(map[string]int)
		stopped, iterateErr := db.RawIterate("proof", func(key, value []byte, version int64) bool {
			versions[fmt.Sprintf("%s/%d", key, version)]++
			return false
		})
		require.NoError(t, iterateErr)
		require.False(t, stopped)
		require.Len(t, versions, 2*pairs)
		for _, count := range versions {
			require.Equal(t, 1, count)
		}
	}
	assertState(db)
	assertState(db) // Read the SSTables again to exercise actual block-cache hits.
	snapshot := db.PebbleMetrics()
	require.Positive(t, snapshot.BlockCache.Hits)
	require.Positive(t, snapshot.BlockCache.Misses)
	require.Positive(t, snapshot.BlockCache.Size)
	var bytesRead uint64
	for _, level := range snapshot.Levels {
		bytesRead += level.TableBytesIn
	}
	require.Positive(t, bytesRead)
	db.collectAndRecordMetrics(ctx)
	db.collectAndRecordMetrics(ctx)
	require.NoError(t, db.Close())
	db = open()
	assertState(db)

	// This is the same exporter and handler used by StartMetricsServer. The
	// consumer traverses HTTP and parses the Prometheus wire representation.
	server := httptest.NewServer(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 5 * time.Second}
	scrape := func() map[string]*dto.MetricFamily {
		t.Helper()
		response, getErr := client.Get(server.URL + "/metrics")
		require.NoError(t, getErr)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		parser := expfmt.NewTextParser(model.UTF8Validation)
		families, parseErr := parser.TextToMetricFamilies(response.Body)
		require.NoError(t, parseErr)
		return families
	}
	observed := scrape()
	proofNodeHistogram(t, observed, "pebble_apply_changeset_async_latency_seconds", "true", 1, smetrics.LatencyBuckets)
	proofNodeHistogram(t, observed, "pebble_apply_changeset_latency_seconds", "true", 2, smetrics.LatencyBuckets)
	proofNodeHistogram(t, observed, "pebble_batch_write_latency_seconds", "true", 2, smetrics.LatencyBuckets)
	batch := proofNodeHistogram(t, observed, "pebble_batch_size_bytes", "", 2, smetrics.ByteSizeBuckets)
	require.Equal(t, float64(firstBytes+secondBytes), batch.GetSampleSum(), "producer reports encoded bytes including metadata")
	require.Equal(t, float64(snapshot.BlockCache.Hits), proofNodeCounter(t, observed, "pebble_cache_hits_total"))
	require.Equal(t, float64(snapshot.BlockCache.Misses), proofNodeCounter(t, observed, "pebble_cache_misses_total"))
	cache := proofNodeMetric(t, observed, "pebble_cache_size_bytes", "")
	require.NotNil(t, cache.Gauge)
	require.Equal(t, float64(snapshot.BlockCache.Size), cache.GetGauge().GetValue())
	require.Equal(t, float64(bytesRead), proofNodeCounter(t, observed, "pebble_compaction_bytes_read_bytes_total"))
	levels := observed["pebble_compaction_bytes_read_bytes_total"].GetMetric()
	expectedLevels := 0
	for _, level := range snapshot.Levels {
		if level.TableBytesIn > 0 {
			expectedLevels++
		}
	}
	require.Len(t, levels, expectedLevels)
	for i, level := range snapshot.Levels {
		var matching []*dto.Metric
		for _, point := range levels {
			for _, label := range point.GetLabel() {
				if label.GetName() == "level" && label.GetValue() == fmt.Sprint(i) {
					matching = append(matching, point)
				}
			}
		}
		if level.TableBytesIn == 0 {
			require.Empty(t, matching, "zero deltas do not create a counter series")
		} else {
			require.Len(t, matching, 1)
			require.Equal(t, float64(level.TableBytesIn), matching[0].GetCounter().GetValue())
		}
	}

	// The reopened database contributes its fresh snapshot once to the
	// continuing exporter. Polling the unchanged snapshot adds no counters.
	reopenedSnapshot := db.PebbleMetrics()
	db.collectAndRecordMetrics(ctx)
	collected := scrape()
	require.Equal(t, float64(snapshot.BlockCache.Hits+reopenedSnapshot.BlockCache.Hits), proofNodeCounter(t, collected, "pebble_cache_hits_total"))
	require.Equal(t, float64(snapshot.BlockCache.Misses+reopenedSnapshot.BlockCache.Misses), proofNodeCounter(t, collected, "pebble_cache_misses_total"))
	db.collectAndRecordMetrics(ctx)
	unchanged := scrape()
	for _, name := range []string{
		"pebble_compaction_count_total", "pebble_flush_count_total",
		"pebble_flush_bytes_written_bytes_total", "pebble_compaction_bytes_read_bytes_total",
		"pebble_compaction_bytes_written_bytes_total", "pebble_cache_hits_total", "pebble_cache_misses_total",
	} {
		proofNodeCounter(t, collected, name)
		proofNodeCounter(t, unchanged, name)
		require.Equal(t, collected[name], unchanged[name], "unchanged collection must preserve %s series and totals", name)
	}
	observed = unchanged

	// A second consumer scrape must not change any of the eight streams.
	repeated := scrape()
	for _, name := range []string{
		"pebble_apply_changeset_async_latency_seconds", "pebble_apply_changeset_latency_seconds",
		"pebble_batch_size_bytes", "pebble_batch_write_latency_seconds", "pebble_cache_hits_total",
		"pebble_cache_misses_total", "pebble_cache_size_bytes", "pebble_compaction_bytes_read_bytes_total",
	} {
		require.Equal(t, observed[name], repeated[name], "scrape mutated %s", name)
	}

	// A replay writes the same logical version, without adding stored records.
	require.NoError(t, db.ApplyChangesetSync(2, second))
	assertState(db)
	// Malformed input reaches the real rejection path and the failure series.
	err = db.ApplyChangesetSync(-1, second)
	require.EqualError(t, err, "version must be non-negative")
	assertState(db)
	refused := scrape()
	proofNodeHistogram(t, refused, "pebble_apply_changeset_latency_seconds", "true", 3, smetrics.LatencyBuckets)
	proofNodeHistogram(t, refused, "pebble_apply_changeset_latency_seconds", "false", 1, smetrics.LatencyBuckets)
	proofNodeHistogram(t, refused, "pebble_apply_changeset_async_latency_seconds", "true", 1, smetrics.LatencyBuckets)
	proofNodeHistogram(t, refused, "pebble_batch_write_latency_seconds", "true", 3, smetrics.LatencyBuckets)
	batch = proofNodeHistogram(t, refused, "pebble_batch_size_bytes", "", 3, smetrics.ByteSizeBuckets)
	require.Equal(t, float64(firstBytes+2*secondBytes), batch.GetSampleSum(), "refused input must not write a batch")
	require.Len(t, refused["pebble_batch_write_latency_seconds"].GetMetric(), 1, "refusal happens before batch creation")
	require.NoError(t, db.Close())
	db = open()
	assertState(db) // Replay and refusal survive a second stop/reopen too.
}

func proofNodeMetric(t *testing.T, families map[string]*dto.MetricFamily, name, success string) *dto.Metric {
	t.Helper()
	family, ok := families[name]
	require.True(t, ok, "scrape lacks %s", name)
	var matches []*dto.Metric
	for _, point := range family.GetMetric() {
		if success == "" {
			matches = append(matches, point)
			continue
		}
		for _, label := range point.GetLabel() {
			if label.GetName() == "success" && label.GetValue() == success {
				matches = append(matches, point)
			}
		}
	}
	require.Len(t, matches, 1, "expected one %s series with success=%s", name, success)
	return matches[0]
}

func proofNodeHistogram(t *testing.T, families map[string]*dto.MetricFamily, name, success string, count uint64, bounds []float64) *dto.Histogram {
	t.Helper()
	point := proofNodeMetric(t, families, name, success)
	require.NotNil(t, point.Histogram, "%s must be a histogram", name)
	histogram := point.GetHistogram()
	require.Equal(t, count, histogram.GetSampleCount(), name)
	require.Positive(t, histogram.GetSampleSum(), name)
	var actual []float64
	var previous uint64
	for _, bucket := range histogram.GetBucket() {
		require.GreaterOrEqual(t, bucket.GetCumulativeCount(), previous, name)
		require.LessOrEqual(t, bucket.GetCumulativeCount(), count, name)
		previous = bucket.GetCumulativeCount()
		if !math.IsInf(bucket.GetUpperBound(), 1) {
			actual = append(actual, bucket.GetUpperBound())
		}
	}
	require.Equal(t, bounds, actual, name)
	return histogram
}

func proofNodeCounter(t *testing.T, families map[string]*dto.MetricFamily, name string) float64 {
	t.Helper()
	family, ok := families[name]
	require.True(t, ok, "scrape lacks %s", name)
	require.Equal(t, dto.MetricType_COUNTER, family.GetType(), name)
	require.NotEmpty(t, family.GetMetric(), name)
	var total float64
	for _, point := range family.GetMetric() {
		require.NotNil(t, point.Counter, name)
		total += point.GetCounter().GetValue()
	}
	return total
}
