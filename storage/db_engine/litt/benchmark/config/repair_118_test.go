package config

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	commonmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
	littmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/metrics"
)

type repair118Target struct {
	Expr         string
	LegendFormat string
	RefID        string `json:"refId"`
	EditorMode   string
	Range        bool
	Hide         bool
}

type repair118Panel struct {
	Title       string
	Targets     []repair118Target
	FieldConfig struct {
		Defaults struct{ Unit string }
	}
}

type repair118Scrape struct {
	families map[string]*dto.MetricFamily
	samples  model.Vector
}

func TestRepair_118_DashboardLatencyShape(t *testing.T) {
	panels := repair118Panels(t)
	before, after := repair118Export(t)
	cases := []struct {
		title       string
		metric      string
		multipliers []int
	}{
		{"Write Latency", "litt_write_latency_seconds", []int{1}},
		{"Flush Latency", "litt_flush_latency_seconds", []int{2}},
		{"Segment Flush Latency", "litt_segment_flush_latency_seconds", []int{3}},
		{"Keymap Flush Latency", "litt_keymap_flush_latency_seconds", []int{4}},
		{"GC Latency", "litt_garbage_collection_latency_seconds", []int{5}},
		{"Read Latency", "litt_read_latency_seconds", []int{6, 7}},
		{"Cache Miss Latency", "litt_cache_miss_latency_seconds", []int{6}},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			panel, ok := panels[tc.title]
			require.True(t, ok, "missing latency panel")
			require.Equal(t, "ms", panel.FieldConfig.Defaults.Unit)
			require.Len(t, panel.Targets, 4)
			want := map[string]string{
				"{{table}} average": fmt.Sprintf("1000 * sum by (table) (rate(%s_sum[$__rate_interval])) / sum by (table) (rate(%s_count[$__rate_interval]))", tc.metric, tc.metric),
			}
			for _, percentile := range []struct{ quantile, label string }{
				{"0.5", "p50"}, {"0.9", "p90"}, {"0.99", "p99"},
			} {
				want["{{table}} "+percentile.label] = fmt.Sprintf("1000 * histogram_quantile(%s, sum by (table, le) (rate(%s_bucket[$__rate_interval])))", percentile.quantile, tc.metric)
			}
			refs := map[string]bool{}
			for _, target := range panel.Targets {
				expr, ok := want[target.LegendFormat]
				require.True(t, ok, "unexpected or duplicate latency legend %q", target.LegendFormat)
				require.Equal(t, expr, target.Expr, "percentiles must aggregate buckets; averages must weight by observation count")
				require.Equal(t, "code", target.EditorMode)
				require.True(t, target.Range)
				require.False(t, target.Hide)
				require.NotEmpty(t, target.RefID)
				require.False(t, refs[target.RefID], "duplicate target reference")
				refs[target.RefID] = true
				delete(want, target.LegendFormat)
				repair118AssertSelectors(t, target.Expr, before, after)
			}
			require.Empty(t, want, "missing latency targets")
			for table, delays := range map[string][]int{"alpha": {1, 2, 4}, "beta": {10, 20, 30, 40, 50}} {
				for phase, scrape := range []repair118Scrape{before, after} {
					observations := delays
					if phase == 0 {
						observations = delays[:1]
					}
					family := scrape.families[tc.metric]
					require.NotNil(t, family, "missing exported histogram %s", tc.metric)
					require.Equal(t, dto.MetricType_HISTOGRAM, family.GetType())
					hist := repair118Metric(t, family, table).GetHistogram()
					require.NotNil(t, hist)
					var seconds []float64
					var sum float64
					for _, delay := range observations {
						for _, multiplier := range tc.multipliers {
							value := (time.Duration(delay*multiplier) * time.Millisecond).Seconds()
							seconds = append(seconds, value)
							sum += value
						}
					}
					require.Equal(t, uint64(len(seconds)), hist.GetSampleCount())
					require.InDelta(t, sum, hist.GetSampleSum(), 1e-12, "exported sums must remain in seconds")
					require.Len(t, hist.Bucket, len(commonmetrics.LatencyBuckets)+1)
					for i, bucket := range hist.Bucket {
						bound := math.Inf(1)
						if i < len(commonmetrics.LatencyBuckets) {
							bound = commonmetrics.LatencyBuckets[i]
						}
						require.Equal(t, bound, bucket.GetUpperBound())
						var count uint64
						for _, value := range seconds {
							if value <= bound {
								count++
							}
						}
						require.Equal(t, count, bucket.GetCumulativeCount(), "wrong histogram bucket for %s/%s at %g", tc.metric, table, bound)
					}
				}
			}
		})
	}
}

func TestRepair_118_DashboardCounterNames(t *testing.T) {
	panels := repair118Panels(t)
	before, after := repair118Export(t)
	cases := []struct {
		title  string
		metric string
		alpha  float64
		beta   float64
	}{
		{"Bytes Written / Second", "litt_bytes_written_bytes_total", 1024, 2048},
		{"Keys Written / Second", "litt_keys_written_total", 3, 3},
		{"Flushes / Second", "litt_flush_count_total", 1, 1},
		{"Bytes Read / Second", "litt_bytes_read_bytes_total", 1024, 2048},
		{"Keys Read / Second", "litt_keys_read_total", 1, 1},
		{"Cache Hits / Second", "litt_cache_hits_total", 1, 1},
		{"Cache Misses / Second", "litt_cache_misses_total", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			panel, ok := panels[tc.title]
			require.True(t, ok, "missing counter panel")
			var active int
			for _, target := range panel.Targets {
				if target.Expr == "" {
					continue
				}
				active++
				require.Equal(t, fmt.Sprintf("rate(%s[$__rate_interval])", tc.metric), target.Expr)
				require.True(t, target.Range)
				require.False(t, target.Hide)
				repair118AssertSelectors(t, target.Expr, before, after)
			}
			require.Equal(t, 1, active)
			for table, value := range map[string]float64{"alpha": tc.alpha, "beta": tc.beta} {
				initial := before.families[tc.metric]
				final := after.families[tc.metric]
				require.NotNil(t, initial)
				require.NotNil(t, final)
				require.Equal(t, dto.MetricType_COUNTER, final.GetType())
				first := repair118Metric(t, initial, table).GetCounter()
				last := repair118Metric(t, final, table).GetCounter()
				require.NotNil(t, first)
				require.NotNil(t, last)
				require.Equal(t, value, first.GetValue())
				operations := 2.0
				if table == "beta" {
					operations = 4
				}
				require.Equal(t, operations*value, last.GetValue()-first.GetValue(), "counter samples consumed by rate must include every reported operation with exporter units")
			}
		})
	}
}

func repair118Panels(t *testing.T) map[string]repair118Panel {
	t.Helper()
	data, err := os.ReadFile("benchmark-grafana-dashboard.json")
	require.NoError(t, err)
	var dashboard struct{ Panels []repair118Panel }
	require.NoError(t, json.Unmarshal(data, &dashboard))
	panels := make(map[string]repair118Panel)
	for _, panel := range dashboard.Panels {
		_, exists := panels[panel.Title]
		require.False(t, exists, "duplicate dashboard panel %s", panel.Title)
		panels[panel.Title] = panel
	}
	return panels
}

func repair118Export(t *testing.T) (repair118Scrape, repair118Scrape) {
	t.Helper()
	previous := otel.GetMeterProvider()
	registry, shutdown, err := commonmetrics.SetupOtelPrometheus()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, shutdown(context.Background()))
		otel.SetMeterProvider(previous)
	})
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	reporter := littmetrics.NewLittDBMetrics()
	emit := func(table string, delay int, size uint64) {
		latency := time.Duration(delay) * time.Millisecond
		reporter.ReportWriteOperation(table, latency, 3, size)
		reporter.ReportFlushOperation(table, latency*2)
		reporter.ReportSegmentFlushLatency(table, latency*3)
		reporter.ReportKeymapFlushLatency(table, latency*4)
		reporter.ReportGarbageCollectionLatency(table, latency*5)
		reporter.ReportReadOperation(table, latency*6, size, false)
		reporter.ReportReadOperation(table, latency*7, size, true)
	}
	scrape := func() repair118Scrape {
		response, err := server.Client().Get(server.URL + "/metrics")
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		parser := expfmt.NewTextParser(model.LegacyValidation)
		families, err := parser.TextToMetricFamilies(response.Body)
		require.NoError(t, err)
		var all []*dto.MetricFamily
		for _, family := range families {
			all = append(all, family)
		}
		samples, err := expfmt.ExtractSamples(&expfmt.DecodeOptions{}, all...)
		require.NoError(t, err)
		return repair118Scrape{families, samples}
	}
	emit("alpha", 1, 1024)
	emit("beta", 10, 2048)
	before := scrape()
	for _, delay := range []int{2, 4} {
		emit("alpha", delay, 1024)
	}
	for _, delay := range []int{20, 30, 40, 50} {
		emit("beta", delay, 2048)
	}
	return before, scrape()
}

func repair118Metric(t *testing.T, family *dto.MetricFamily, table string) *dto.Metric {
	t.Helper()
	var matches []*dto.Metric
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == "table" && label.GetValue() == table {
				matches = append(matches, metric)
			}
		}
	}
	require.Len(t, matches, 1, "missing or duplicate exported sample %s/%s", family.GetName(), table)
	return matches[0]
}

func repair118AssertSelectors(t *testing.T, expr string, scrapes ...repair118Scrape) {
	t.Helper()
	selectors := regexp.MustCompile(`\blitt_[a-z_]+\b`).FindAllString(expr, -1)
	require.NotEmpty(t, selectors)
	for _, selector := range selectors {
		for _, scrape := range scrapes {
			for _, table := range []string{"alpha", "beta"} {
				var matches int
				for _, sample := range scrape.samples {
					if string(sample.Metric[model.MetricNameLabel]) == selector && string(sample.Metric["table"]) == table {
						matches++
						if sample.Metric["le"] != "" {
							require.Empty(t, sample.Metric["quantile"], "histogram buckets must not be treated as summary quantiles")
						}
					}
				}
				require.Positive(t, matches, "dashboard selector %s has no /metrics samples for %s", selector, table)
			}
		}
	}
}
