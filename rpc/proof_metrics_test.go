// Proves: TraceBaker counters / newRPCMetrics Int64ObservableCounter -> SetupOtelMetricsProvider / Prometheus HTTP scrape -> expfmt.TextParser.TextToMetricFamilies.
//
// The test is the first scrape consumer of the three trace-baker totals. It uses
// the production callback and exporter, with real TraceBaker state and TraceDB;
// no tracer backend is needed because this contract begins at the counters.
// These totals are process-local, so repeated fresh scrapes, updates, callback
// retirement and a replacement baker prove their lifecycle instead of persistence.
// Malformed samples are derived from an actual scrape and refused by the same
// Prometheus decoder that consumes the successful responses.
package evmrpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	utilmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/utils/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestProofMetricsProducerTransportConsumer(t *testing.T) {
	// Keep both the application setup and the producer registration real while
	// isolating their global registries from the other package tests.
	registry := prometheus.NewRegistry()
	previousRegisterer, previousGatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	previousProvider := otel.GetMeterProvider()
	previousMeter, previousMetrics := rpcTelemetryMeter, metrics
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() {
		prometheus.DefaultRegisterer, prometheus.DefaultGatherer = previousRegisterer, previousGatherer
		otel.SetMeterProvider(previousProvider)
		rpcTelemetryMeter, metrics = previousMeter, previousMetrics
	})
	require.NoError(t, utilmetrics.SetupOtelMetricsProvider())
	provider, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	require.True(t, ok, "application setup must install the SDK MeterProvider")
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	rpcTelemetryMeter = provider.Meter("evmrpc")
	metrics = newRPCMetrics(rpcTelemetryMeter)

	server := httptest.NewServer(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 5 * time.Second

	names := []string{
		"pax_chain_evmrpc_trace_bake_dropped_total",
		"pax_chain_evmrpc_trace_bake_baked_total",
		"pax_chain_evmrpc_trace_bake_failed_total",
	}
	scrape := func() []byte {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/metrics", nil)
		require.NoError(t, err)
		request.Header.Set("Accept", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
		response, err := client.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, expfmt.TypeTextPlain, expfmt.ResponseFormat(response.Header).FormatType())
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return body
	}
	decode := func(body []byte) (map[string]*dto.MetricFamily, error) {
		parser := expfmt.NewTextParser(model.UTF8Validation)
		return parser.TextToMetricFamilies(bytes.NewReader(body))
	}
	observe := func(want map[string]float64) []byte {
		t.Helper()
		body := scrape()
		families, err := decode(body)
		require.NoError(t, err)
		observed := make(map[string]float64)
		for _, name := range names {
			family, present := families[name]
			if _, expected := want[name]; !expected {
				require.False(t, present, "retired producer must not leave a cached series: %s", name)
				continue
			}
			require.True(t, present, "consumer must see %s on the scrape", name)
			require.Equal(t, name, family.GetName())
			require.NotEmpty(t, family.GetHelp())
			require.Equal(t, dto.MetricType_COUNTER, family.GetType(), name)
			require.Len(t, family.Metric, 1, name)
			point := family.Metric[0]
			require.NotNil(t, point.Counter, name)
			labels := make(map[string]string)
			for _, label := range point.Label {
				labels[label.GetName()] = label.GetValue()
			}
			require.Equal(t, "evmrpc", labels["otel_scope_name"], name)
			observed[name] = point.Counter.GetValue()
		}
		require.Equal(t, want, observed)
		return body
	}

	api := &DebugAPI{keeper: new(keeper.Keeper)}
	require.Nil(t, StartTraceBakerForDebugAPI(nil, TraceBakerConfig{}))
	require.Nil(t, StartTraceBakerForDebugAPI(api, TraceBakerConfig{}))
	observe(map[string]float64{})

	cache, err := keeper.NewTraceDB(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })
	api.keeper.SetTraceDB(cache)
	baker := StartTraceBakerForDebugAPI(api, TraceBakerConfig{})
	require.NotNil(t, baker)
	t.Cleanup(baker.Stop)
	zero := map[string]float64{names[0]: 0, names[1]: 0, names[2]: 0}
	observe(zero)

	// Populate the actual state read by the registered callback, rather than
	// constructing metric data or supplying a replacement collection callback.
	atomic.AddUint64(&baker.dropped, 3)
	atomic.AddUint64(&baker.baked, 5)
	atomic.AddUint64(&baker.failed, 7)
	first := map[string]float64{names[0]: 3, names[1]: 5, names[2]: 7}
	observe(first)
	observe(first)

	atomic.AddUint64(&baker.dropped, 11)
	atomic.AddUint64(&baker.baked, 13)
	atomic.AddUint64(&baker.failed, 17)
	second := map[string]float64{names[0]: 14, names[1]: 18, names[2]: 24}
	observe(second)
	body := observe(second)
	require.Equal(t, uint64(14), baker.DroppedCount())
	require.Equal(t, uint64(18), baker.BakedCount())
	require.Equal(t, uint64(24), baker.FailedCount())

	for _, name := range names {
		t.Run("malformed_"+name, func(t *testing.T) {
			var sample string
			for _, line := range strings.Split(string(body), "\n") {
				if strings.HasPrefix(line, name+"{") || strings.HasPrefix(line, name+" ") {
					require.Empty(t, sample, "expected exactly one producer sample")
					sample = line
				}
			}
			require.NotEmpty(t, sample)
			valueStart := strings.LastIndexByte(sample, ' ')
			require.Greater(t, valueStart, 0)
			malformed := strings.Replace(string(body), sample, sample[:valueStart+1]+"invalid-counter", 1)
			_, err := decode([]byte(malformed))
			require.ErrorContains(t, err, `expected float as value, got "invalid-counter"`)
			// A refusal must not poison a later, fresh decode of the producer.
			observe(second)
		})
	}

	baker.Stop()
	baker.Stop()
	observe(map[string]float64{})
	observe(map[string]float64{})

	replacement := StartTraceBakerForDebugAPI(api, TraceBakerConfig{})
	require.NotNil(t, replacement)
	t.Cleanup(replacement.Stop)
	observe(zero)
	atomic.AddUint64(&replacement.dropped, 2)
	atomic.AddUint64(&replacement.baked, 4)
	atomic.AddUint64(&replacement.failed, 6)
	reset := map[string]float64{names[0]: 2, names[1]: 4, names[2]: 6}
	observe(reset)
	observe(reset)
	replacement.Stop()
	observe(map[string]float64{})
}
