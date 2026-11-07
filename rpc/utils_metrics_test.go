package evmrpc

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	utilmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/utils/metrics"
	legacymetrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordMetricsDoesNotEmitLegacyRequestMetrics(t *testing.T) {
	cases := []struct {
		name       string
		connection ConnectionType
		err        error
		panicValue any
		errorClass string
		codeBucket string
	}{
		{name: "success", connection: ConnectionTypeHTTP},
		{
			name: "association error", connection: ConnectionTypeHTTP,
			err: types.NewAssociationMissingErr("0xabc"), errorClass: errorClassAssociationMissing,
		},
		{
			name: "panic", connection: ConnectionTypeWS, panicValue: "request panic",
			errorClass: errorClassPanic, codeBucket: jsonrpcCodeBucketSpec,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			histogram, err := provider.Meter("evmrpc").Float64Histogram("evmrpc_request_latency_seconds")
			require.NoError(t, err)
			previousHistogram := metrics.requestLatencySeconds
			metrics.requestLatencySeconds = histogram
			t.Cleanup(func() {
				metrics.requestLatencySeconds = previousHistogram
				require.NoError(t, provider.Shutdown(context.Background()))
			})

			legacySink := legacymetrics.NewInmemSink(time.Hour, 2*time.Hour)
			legacyConfig := legacymetrics.DefaultConfig("")
			legacyConfig.EnableHostname = false
			legacyConfig.EnableRuntimeMetrics = false
			previousLegacyMetrics := legacymetrics.Default()
			_, err = legacymetrics.NewGlobal(legacyConfig, legacySink)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := legacymetrics.NewGlobal(legacyConfig, previousLegacyMetrics)
				require.NoError(t, err)
			})

			readLegacy := func() (requests, latencySamples, associationErrors float64) {
				for _, interval := range legacySink.Data() {
					interval.RLock()
					for _, sample := range interval.Counters {
						switch sample.Name {
						case "pax.rpc.request.counter":
							requests += sample.Sum
						case "pax.association.error":
							associationErrors += sample.Sum
						}
					}
					for _, sample := range interval.Samples {
						if sample.Name == "pax.rpc.request.latency_ms" {
							latencySamples += float64(sample.Count)
						}
					}
					interval.RUnlock()
				}
				return
			}
			readLatency := func() []metricdata.HistogramDataPoint[float64] {
				t.Helper()
				var data metricdata.ResourceMetrics
				require.NoError(t, reader.Collect(ctx, &data))
				var points []metricdata.HistogramDataPoint[float64]
				for _, scope := range data.ScopeMetrics {
					for _, instrument := range scope.Metrics {
						if instrument.Name == "evmrpc_request_latency_seconds" {
							histogram, ok := instrument.Data.(metricdata.Histogram[float64])
							require.True(t, ok, "RPC latency must be a float64 histogram")
							points = append(points, histogram.DataPoints...)
						}
					}
				}
				return points
			}

			const endpoint = "eth_chainId"
			utilmetrics.IncrementRpcRequestCounter(endpoint, string(tc.connection), true)
			utilmetrics.MeasureRpcRequestLatency(endpoint, string(tc.connection), time.Now().Add(-time.Millisecond))
			beforeRequests, beforeSamples, beforeErrors := readLegacy()
			require.Equal(t, float64(1), beforeRequests, "legacy request counter must be active")
			require.Equal(t, float64(1), beforeSamples, "legacy latency sink must be active")
			require.Zero(t, beforeErrors)
			require.Empty(t, readLatency())

			start := time.Now().Add(-time.Millisecond)
			record := func() {
				recordMetricsWithError(ctx, endpoint, tc.connection, start, tc.err, tc.panicValue)
			}
			if tc.panicValue != nil {
				require.PanicsWithValue(t, tc.panicValue, record)
			} else {
				require.NotPanics(t, record)
			}

			points := readLatency()
			require.Len(t, points, 1)
			require.Equal(t, uint64(1), points[0].Count)
			require.GreaterOrEqual(t, points[0].Sum, time.Millisecond.Seconds())
			wantAttributes := attribute.NewSet(
				attribute.String(endpointKey, endpoint),
				attribute.String(connectionKey, string(tc.connection)),
				attribute.Bool(successKey, tc.err == nil && tc.panicValue == nil),
				attribute.String(errorClassKey, tc.errorClass),
				attribute.String(jsonrpcCodeKey, tc.codeBucket),
			)
			require.True(t, points[0].Attributes.Equals(&wantAttributes))
			afterRequests, afterSamples, afterErrors := readLegacy()
			require.Equal(t, beforeRequests, afterRequests, "RPC must not emit the legacy request counter")
			require.Equal(t, beforeSamples, afterSamples, "RPC must not emit the legacy latency sample")
			if tc.err != nil {
				require.Equal(t, beforeErrors+1, afterErrors, "typed error metrics must still be recorded")
			} else {
				require.Equal(t, beforeErrors, afterErrors)
			}
		})
	}
}
