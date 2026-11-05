package evmrpc

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type ManualReader = sdkmetric.ManualReader

func TestTraceBakerCountersExported(t *testing.T) {
	var reader *ManualReader = sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousMeter, previousMetrics := rpcTelemetryMeter, metrics
	rpcTelemetryMeter = provider.Meter("evmrpc")
	metrics = newRPCMetrics(rpcTelemetryMeter)
	t.Cleanup(func() {
		rpcTelemetryMeter, metrics = previousMeter, previousMetrics
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	names := []string{
		"evmrpc_trace_bake_dropped_total",
		"evmrpc_trace_bake_baked_total",
		"evmrpc_trace_bake_failed_total",
	}
	collect := func() map[string]int64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &data))
		values := make(map[string]int64)
		for _, scope := range data.ScopeMetrics {
			for _, instrument := range scope.Metrics {
				for _, name := range names {
					if instrument.Name != name {
						continue
					}
					require.Equal(t, "evmrpc", scope.Scope.Name)
					require.Equal(t, "{count}", instrument.Unit)
					sum, ok := instrument.Data.(metricdata.Sum[int64])
					require.True(t, ok, "%s must be an Int64 counter", name)
					require.True(t, sum.IsMonotonic, name)
					require.Equal(t, metricdata.CumulativeTemporality, sum.Temporality, name)
					require.Len(t, sum.DataPoints, 1, name)
					_, duplicate := values[name]
					require.False(t, duplicate, name)
					values[name] = sum.DataPoints[0].Value
				}
			}
		}
		return values
	}

	require.Nil(t, StartTraceBakerForDebugAPI(nil, TraceBakerConfig{}))
	k := new(keeper.Keeper)
	api := &DebugAPI{keeper: k}
	require.Nil(t, StartTraceBakerForDebugAPI(api, TraceBakerConfig{}))
	require.Empty(t, collect())

	cache, err := keeper.NewTraceDB(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })
	k.SetTraceDB(cache)
	baker := StartTraceBakerForDebugAPI(api, TraceBakerConfig{})
	require.NotNil(t, baker)
	t.Cleanup(baker.Stop)
	require.Equal(t, map[string]int64{names[0]: 0, names[1]: 0, names[2]: 0}, collect())

	atomic.AddUint64(&baker.dropped, 3)
	atomic.AddUint64(&baker.baked, 5)
	atomic.AddUint64(&baker.failed, 7)
	require.Equal(t, map[string]int64{names[0]: 3, names[1]: 5, names[2]: 7}, collect())

	atomic.AddUint64(&baker.dropped, 11)
	atomic.AddUint64(&baker.baked, 13)
	atomic.AddUint64(&baker.failed, 17)
	require.Equal(t, map[string]int64{names[0]: 14, names[1]: 18, names[2]: 24}, collect())

	baker.Stop()
	baker.Stop()
	require.Empty(t, collect(), "stopped baker must unregister its callback")

	replacement := StartTraceBakerForDebugAPI(api, TraceBakerConfig{})
	require.NotNil(t, replacement)
	t.Cleanup(replacement.Stop)
	atomic.AddUint64(&replacement.dropped, 2)
	atomic.AddUint64(&replacement.baked, 4)
	atomic.AddUint64(&replacement.failed, 6)
	require.Equal(t, map[string]int64{names[0]: 2, names[1]: 4, names[2]: 6}, collect())
	replacement.Stop()
	require.Empty(t, collect())
}
