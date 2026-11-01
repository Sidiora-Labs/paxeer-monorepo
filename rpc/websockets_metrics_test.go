package evmrpc

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	utilmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/utils/metrics"
	legacymetrics "github.com/armon/go-metrics"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestWSConnectDoesNotEmitLegacyCounter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	counter, err := provider.Meter("evmrpc").Int64Counter("evmrpc_websocket_connects_total")
	require.NoError(t, err)
	previousCounter := metrics.wsConnectionCount
	metrics.wsConnectionCount = counter
	t.Cleanup(func() { metrics.wsConnectionCount = previousCounter })

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

	readConnects := func() int64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &data))
		for _, scope := range data.ScopeMetrics {
			for _, metric := range scope.Metrics {
				if metric.Name != "evmrpc_websocket_connects_total" {
					continue
				}
				sum, ok := metric.Data.(metricdata.Sum[int64])
				require.True(t, ok, "websocket connections must be an int64 sum")
				require.True(t, sum.IsMonotonic)
				require.Equal(t, metricdata.CumulativeTemporality, sum.Temporality)
				require.Len(t, sum.DataPoints, 1)
				return sum.DataPoints[0].Value
			}
		}
		t.Fatal("evmrpc_websocket_connects_total was not collected")
		return 0
	}
	readLegacyConnects := func() float64 {
		var total float64
		for _, interval := range legacySink.Data() {
			interval.RLock()
			for _, sample := range interval.Counters {
				if sample.Name == "pax.websocket.connect" {
					total += sample.Sum
				}
			}
			interval.RUnlock()
		}
		return total
	}

	counter.Add(t.Context(), 0)
	utilmetrics.IncWebsocketConnects()
	beforeConnects := readConnects()
	beforeLegacy := readLegacyConnects()
	require.Equal(t, int64(0), beforeConnects)
	require.Equal(t, float64(1), beforeLegacy, "legacy telemetry must be active")

	rpcServer := gethrpc.NewServer()
	defer rpcServer.Stop()
	server := httptest.NewServer(NewWSConnectionHandler(rpcServer.WebsocketHandler([]string{"*"})))
	defer server.Close()
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.DialContext(t.Context(), "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer conn.Close()

	require.Equal(t, beforeConnects+1, readConnects())
	require.Equal(t, beforeLegacy, readLegacyConnects())
}
