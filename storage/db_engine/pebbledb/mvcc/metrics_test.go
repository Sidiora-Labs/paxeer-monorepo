package mvcc

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	smetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb"
)

type mvccHistogramCase struct {
	name        string
	description string
	unit        string
	bounds      []float64
	float64     metric.Float64Histogram
	int64       metric.Int64Histogram
}

func mvccHistogramCases(m mvccMetrics) []mvccHistogramCase {
	return []mvccHistogramCase{
		{"pebble_get_latency", "Time taken to get a key from PebbleDB", "s", smetrics.LatencyBuckets, m.getLatency, nil},
		{"pebble_apply_changeset_latency", "Time taken to apply changeset to PebbleDB", "s", smetrics.LatencyBuckets, m.applyChangesetLatency, nil},
		{"pebble_apply_changeset_async_latency", "Time taken to queue changeset for async write", "s", smetrics.LatencyBuckets, m.applyChangesetAsyncLatency, nil},
		{"pebble_prune_latency", "Time taken to prune old versions from PebbleDB", "s", smetrics.LatencyBuckets, m.pruneLatency, nil},
		{"pebble_import_latency", "Time taken to import snapshot data to PebbleDB", "s", smetrics.LatencyBuckets, m.importLatency, nil},
		{"pebble_batch_write_latency", "Time taken to write a batch to PebbleDB", "s", smetrics.LatencyBuckets, m.batchWriteLatency, nil},
		{"pebble_compaction_duration", "Duration of compaction operations", "s", smetrics.LatencyBuckets, m.compactionDuration, nil},
		{"pebble_flush_duration", "Duration of memtable flush operations", "s", smetrics.LatencyBuckets, m.flushDuration, nil},
		{"pebble_batch_size", "Size of batches written to PebbleDB", "By", smetrics.ByteSizeBuckets, nil, m.batchSize},
		{"pebble_iterator_iterations", "Number of iterations per iterator", "{count}", smetrics.CountBuckets, m.iteratorIterations, nil},
	}
}

func newMVCCMetricsReader(t *testing.T) (*sdkmetric.MeterProvider, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})
	return provider, reader
}

func recordMVCCHistogram(ctx context.Context, h mvccHistogramCase) {
	if h.int64 != nil {
		h.int64.Record(ctx, 1024)
	} else {
		h.float64.Record(ctx, 0.01)
	}
}

func assertMVCCHistograms(t *testing.T, reader *sdkmetric.ManualReader, cases []mvccHistogramCase, count uint64) {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	for _, h := range cases {
		t.Run(h.name, func(t *testing.T) {
			streams := 0
			for _, scope := range collected.ScopeMetrics {
				for _, exported := range scope.Metrics {
					if exported.Name != h.name {
						continue
					}
					streams++
					if scope.Scope.Name != "paxdb_pebble" {
						t.Errorf("scope = %q, want paxdb_pebble", scope.Scope.Name)
					}
					if exported.Description != h.description || exported.Unit != h.unit {
						t.Errorf("descriptor = (%q, %q), want (%q, %q)", exported.Description, exported.Unit, h.description, h.unit)
					}
					if h.int64 != nil {
						data, ok := exported.Data.(metricdata.Histogram[int64])
						if !ok || len(data.DataPoints) != 1 {
							t.Fatalf("expected one int64 histogram data point, got %#v", exported.Data)
						}
						point := data.DataPoints[0]
						assertMVCCHistogramPoint(t, point.Bounds, point.BucketCounts, point.Count, h.bounds, count)
						if point.Sum != int64(count)*1024 {
							t.Errorf("sum = %d, want %d", point.Sum, int64(count)*1024)
						}
					} else {
						data, ok := exported.Data.(metricdata.Histogram[float64])
						if !ok || len(data.DataPoints) != 1 {
							t.Fatalf("expected one float64 histogram data point, got %#v", exported.Data)
						}
						point := data.DataPoints[0]
						assertMVCCHistogramPoint(t, point.Bounds, point.BucketCounts, point.Count, h.bounds, count)
						if point.Sum != float64(count)*0.01 {
							t.Errorf("sum = %g, want %g", point.Sum, float64(count)*0.01)
						}
					}
				}
			}
			if streams != 1 {
				t.Errorf("metric streams = %d, want 1", streams)
			}
		})
	}
}

func assertMVCCHistogramPoint(t *testing.T, bounds []float64, buckets []uint64, count uint64, wantBounds []float64, wantCount uint64) {
	t.Helper()
	if !slices.Equal(bounds, wantBounds) {
		t.Errorf("bucket boundaries = %v, want %v", bounds, wantBounds)
	}
	if count != wantCount {
		t.Errorf("count = %d, want %d", count, wantCount)
	}
	if len(buckets) != len(wantBounds)+1 {
		t.Errorf("bucket count = %d, want %d", len(buckets), len(wantBounds)+1)
	}
	var total uint64
	for _, bucket := range buckets {
		total += bucket
	}
	if total != wantCount {
		t.Errorf("sum of bucket counts = %d, want %d", total, wantCount)
	}
}

func TestMVCCHistogramsUseSharedBucketBoundaries(t *testing.T) {
	provider, reader := newMVCCMetricsReader(t)
	cases := mvccHistogramCases(newMVCCMetrics(provider.Meter("paxdb_pebble")))
	for _, h := range cases {
		recordMVCCHistogram(context.Background(), h)
	}
	assertMVCCHistograms(t, reader, cases, 1)
}

func TestMVCCAndPebbleMetricsShareOneDescriptor(t *testing.T) {
	for _, mvccFirst := range []bool{true, false} {
		name := "pebble_first"
		if mvccFirst {
			name = "mvcc_first"
		}
		t.Run(name, func(t *testing.T) {
			provider, reader := newMVCCMetricsReader(t)
			db, err := pebble.Open(t.TempDir(), &pebble.Options{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Errorf("close pebble: %v", err)
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var instruments mvccMetrics
			if mvccFirst {
				instruments = newMVCCMetrics(provider.Meter("paxdb_pebble"))
			}
			pebbledb.NewPebbleMetrics(ctx, db, "descriptor-test", time.Hour)
			cases := mvccHistogramCases(instruments)
			pebbleMeter := provider.Meter("paxdb_pebble")
			// Retrieve the SDK handles registered by NewPebbleMetrics, which keeps them private.
			for _, h := range cases {
				if h.unit == "By" {
					instrument, err := pebbleMeter.Int64Histogram(h.name, metric.WithDescription(h.description), metric.WithUnit(h.unit))
					if err != nil {
						t.Fatal(err)
					}
					instrument.Record(ctx, 1024)
				} else {
					instrument, err := pebbleMeter.Float64Histogram(h.name, metric.WithDescription(h.description), metric.WithUnit(h.unit))
					if err != nil {
						t.Fatal(err)
					}
					instrument.Record(ctx, 0.01)
				}
			}
			if !mvccFirst {
				instruments = newMVCCMetrics(provider.Meter("paxdb_pebble"))
			}
			cases = mvccHistogramCases(instruments)
			for _, h := range cases {
				recordMVCCHistogram(ctx, h)
			}
			assertMVCCHistograms(t, reader, cases, 2)
		})
	}
}
