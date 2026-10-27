package evmrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestInfoAPIMetricLabelsUseServedNames(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	latency, err := provider.Meter("evmrpc").Float64Histogram("evmrpc_request_latency_seconds", metric.WithUnit("s"))
	require.NoError(t, err)
	// Imported fixtures bind the global OTel instruments before test initialization.
	previous := metrics.requestLatencySeconds
	metrics.requestLatencySeconds = latency
	t.Cleanup(func() {
		metrics.requestLatencySeconds = previous
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	// These HTTP servers are started by the external package's setup_test.go.
	const TestPort = 7777
	const TestFeePort = 7785
	sendRequest := func(t *testing.T, port int, method string) map[string]interface{} {
		t.Helper()
		body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_%s","params":[],"id":"metric-label"}`, method)
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d", port), strings.NewReader(body))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		return result
	}

	counts := func(t *testing.T) map[string]uint64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &data))
		result := make(map[string]uint64)
		for _, scope := range data.ScopeMetrics {
			if scope.Scope.Name != "evmrpc" {
				continue
			}
			for _, metric := range scope.Metrics {
				if metric.Name != "evmrpc_request_latency_seconds" {
					continue
				}
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "unexpected latency aggregation %T", metric.Data)
				for _, point := range histogram.DataPoints {
					connection, ok := point.Attributes.Value(attribute.Key("connection"))
					require.True(t, ok)
					if connection.AsString() != string(ConnectionTypeHTTP) {
						continue
					}
					endpoint, ok := point.Attributes.Value(attribute.Key("endpoint"))
					require.True(t, ok)
					result[endpoint.AsString()] += point.Count
				}
			}
		}
		return result
	}

	cases := []struct {
		method      string
		served      string
		legacy      string
		port        int
		unsupported string
	}{
		{method: "blockNumber", served: "eth_blockNumber", legacy: "eth_BlockNumber", port: TestPort},
		{method: "chainId", served: "eth_chainId", legacy: "eth_ChainId", port: TestPort},
		{method: "coinbase", served: "eth_coinbase", legacy: "eth_Coinbase", port: TestPort},
		{method: "accounts", served: "eth_accounts", legacy: "eth_Accounts", port: TestPort, unsupported: "eth_accounts is disabled; use an external wallet"},
		{method: "gasPrice", served: "eth_gasPrice", legacy: "eth_GasPrice", port: TestFeePort},
		{method: "blobBaseFee", served: "eth_blobBaseFee", legacy: "eth_BlobBaseFee", port: TestPort, unsupported: "blobs not supported on this chain"},
		{method: "syncing", served: "eth_syncing", legacy: "eth_Syncing", port: TestPort},
	}
	for _, tc := range cases {
		t.Run(tc.served, func(t *testing.T) {
			before := counts(t)
			response := sendRequest(t, tc.port, tc.method)
			if tc.unsupported != "" {
				rpcError, ok := response["error"].(map[string]interface{})
				require.True(t, ok, "expected unsupported error: %v", response)
				require.Equal(t, float64(ErrCodeEVMNotSupported), rpcError["code"])
				require.Equal(t, tc.unsupported, rpcError["message"])
			} else {
				require.NotContains(t, response, "error")
				require.Contains(t, response, "result")
			}
			after := counts(t)
			require.Equal(t, before[tc.served]+1, after[tc.served], "served method must record one latency observation")
			require.NotContains(t, after, tc.legacy, "capitalised metric label must not be recorded")
		})
	}
}
