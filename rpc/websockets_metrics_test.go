package evmrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	utilmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/utils/metrics"
	legacymetrics "github.com/armon/go-metrics"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestWSConnectEmitsBothCounters(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	previousMetrics := metrics
	metrics = newRPCMetrics(provider.Meter("evmrpc"))
	t.Cleanup(func() { metrics = previousMetrics })

	legacySink := legacymetrics.NewInmemSink(time.Hour, 2*time.Hour)
	legacyConfig := legacymetrics.DefaultConfig("")
	legacyConfig.EnableHostname = false
	legacyConfig.EnableRuntimeMetrics = false
	previousLegacyMetrics := legacymetrics.Default()
	_, err := legacymetrics.NewGlobal(legacyConfig, legacySink)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := legacymetrics.NewGlobal(legacyConfig, previousLegacyMetrics)
		require.NoError(t, err)
	})

	readCount := func(name string) int64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &data))
		for _, scope := range data.ScopeMetrics {
			for _, metric := range scope.Metrics {
				if metric.Name != name {
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
		t.Fatalf("%s was not collected", name)
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

	metrics.wsConnectionCount.Add(t.Context(), 0)
	metrics.wsConnectionAttemptCount.Add(t.Context(), 0)
	utilmetrics.IncWebsocketConnects()
	beforeLegacy := readLegacyConnects()
	require.Equal(t, float64(1), beforeLegacy, "legacy telemetry must be active")

	// Exercise both the ordinary and deny-list upgrade paths through the real stack.
	var attempts, connects int64
	for _, denyList := range [][]string{nil, {"web3_sha3"}} {
		server := NewHTTPServer(gethrpc.DefaultHTTPTimeouts)
		require.NoError(t, server.EnableWS([]gethrpc.API{{Namespace: "web3", Service: &Web3API{}}}, WsConfig{
			Origins: []string{"http://allowed.example"}, DenyList: denyList,
			RPCEndpointConfig: RPCEndpointConfig{JwtSecret: []byte("handshake-test-secret"), readLimit: 4096},
		}))
		t.Cleanup(server.Stop)
		listener := httptest.NewServer(server)
		t.Cleanup(listener.Close)
		endpoint := "ws" + strings.TrimPrefix(listener.URL, "http")
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
		}).SignedString([]byte("handshake-test-secret"))
		require.NoError(t, err)
		dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
		for _, scenario := range []struct {
			origin, auth string
			status       int
		}{
			{"http://allowed.example", "", http.StatusUnauthorized},
			{"http://rejected.example", "Bearer " + token, http.StatusForbidden},
			{"http://allowed.example", "Bearer " + token, http.StatusSwitchingProtocols},
		} {
			header := http.Header{"Origin": {scenario.origin}}
			if scenario.auth != "" {
				header.Set("Authorization", scenario.auth)
			}
			conn, response, err := dialer.DialContext(t.Context(), endpoint, header)
			require.NotNil(t, response)
			require.Equal(t, scenario.status, response.StatusCode)
			attempts++
			if scenario.status == http.StatusSwitchingProtocols {
				require.NoError(t, err)
				require.NoError(t, conn.WriteJSON(map[string]interface{}{
					"jsonrpc": "2.0", "id": 1, "method": "web3_clientVersion", "params": []interface{}{},
				}))
				require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
				var result struct {
					Result string
					Error  interface{}
				}
				require.NoError(t, conn.ReadJSON(&result))
				require.Nil(t, result.Error)
				require.Equal(t, "Geth/"+runtime.GOOS+"-"+runtime.GOARCH+"/"+runtime.Version(), result.Result)
				require.NoError(t, conn.Close())
				connects++
			} else {
				require.Error(t, err)
				require.Nil(t, conn)
				require.NoError(t, response.Body.Close())
			}
			require.Equal(t, attempts, readCount("evmrpc_websocket_connect_attempts_total"))
			require.Equal(t, connects, readCount("evmrpc_websocket_connects_total"))
			require.Equal(t, beforeLegacy+float64(connects), readLegacyConnects())
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, listener.URL, nil)
		require.NoError(t, err)
		request.Header = http.Header{
			"Connection": {"Upgrade"}, "Upgrade": {"websocket"},
			"Sec-Websocket-Version": {"12"}, "Sec-Websocket-Key": {"dGhlIHNhbXBsZSBub25jZQ=="},
			"Origin": {"http://allowed.example"}, "Authorization": {"Bearer " + token},
		}
		response, err := listener.Client().Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
		require.NoError(t, response.Body.Close())
		attempts++
		require.Equal(t, attempts, readCount("evmrpc_websocket_connect_attempts_total"))
		require.Equal(t, connects, readCount("evmrpc_websocket_connects_total"))
		require.Equal(t, beforeLegacy+float64(connects), readLegacyConnects())
	}
}
