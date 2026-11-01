package evmrpc

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/export"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPaxTransactionMethodsRecordPaxLabels(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	latency, err := provider.Meter("evmrpc").Float64Histogram("evmrpc_request_latency_seconds")
	require.NoError(t, err)
	originalLatency := metrics.requestLatencySeconds
	metrics.requestLatencySeconds = latency
	t.Cleanup(func() {
		metrics.requestLatencySeconds = originalLatency
		require.NoError(t, provider.Shutdown(ctx))
	})

	eth := NewTransactionAPI(nil, nil, nil, nil, "",
		ConnectionTypeHTTP, utils.None[time.Duration](), nil, nil, &sync.Mutex{})
	pax := NewPaxTransactionAPI(nil, nil, nil, nil, "",
		ConnectionTypeHTTP, utils.None[time.Duration](), nil, nil, nil, &sync.Mutex{})
	require.Equal(t, "eth", eth.namespace)
	require.Equal(t, "pax", pax.namespace)

	// setup_test.go starts the package HTTP server and enables the pax methods.
	client, err := rpc.DialHTTP("http://127.0.0.1:7777")
	require.NoError(t, err)
	defer client.Close()
	var pool map[string]map[string]map[string]*export.RPCTransaction
	require.NoError(t, client.CallContext(ctx, &pool, "txpool_content"))
	require.Len(t, pool["pending"], 1)
	var transaction *export.RPCTransaction
	for _, transactions := range pool["pending"] {
		require.Len(t, transactions, 1)
		for _, pending := range transactions {
			transaction = pending
		}
	}
	require.NotNil(t, transaction)
	hash, sender := transaction.Hash, transaction.From

	counts := func(t *testing.T) map[string]uint64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &data))
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
				require.True(t, ok, "latency metric must be a histogram")
				for _, point := range histogram.DataPoints {
					endpoint, ok := point.Attributes.Value(attribute.Key("endpoint"))
					require.True(t, ok, "latency sample must name its endpoint")
					result[endpoint.AsString()] += point.Count
				}
			}
		}
		return result
	}

	t.Run("HTTP", func(t *testing.T) {
		before := counts(t)
		var result json.RawMessage
		err := client.CallContext(ctx, &result, "pax_getTransactionByHash",
			common.HexToHash("0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"))
		require.NoError(t, err)
		require.JSONEq(t, "null", string(result))
		after := counts(t)
		require.Equal(t, before["pax_getTransactionByHash"]+1, after["pax_getTransactionByHash"])
		require.Equal(t, before["eth_getTransactionByHash"], after["eth_getTransactionByHash"],
			"pax HTTP call must not record an eth endpoint")
	})

	for _, method := range []struct {
		name string
		call func(*testing.T, *TransactionAPI)
	}{
		{"getTransactionByHash", func(t *testing.T, api *TransactionAPI) {
			var got *export.RPCTransaction
			err := client.CallContext(ctx, &got, api.namespace+"_getTransactionByHash", hash)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, hash, got.Hash)
			require.Equal(t, sender, got.From)
		}},
		{"getTransactionCount", func(t *testing.T, api *TransactionAPI) {
			var got *hexutil.Uint64
			err := client.CallContext(ctx, &got, api.namespace+"_getTransactionCount",
				common.HexToAddress("0x1234567890123456789012345678901234567890"), "latest")
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, hexutil.Uint64(1), *got)
		}},
		{"sign", func(t *testing.T, api *TransactionAPI) {
			_, err := api.Sign(ctx, sender, []byte("metric labels"))
			var unsupported *ErrEVMNotSupported
			require.ErrorAs(t, err, &unsupported)
		}},
	} {
		t.Run(method.name, func(t *testing.T) {
			before := counts(t)
			method.call(t, pax.TransactionAPI)
			afterPax := counts(t)
			require.Equal(t, before["pax_"+method.name]+1, afterPax["pax_"+method.name])
			require.Equal(t, before["eth_"+method.name], afterPax["eth_"+method.name], "pax call must not record an eth endpoint")
			method.call(t, eth)
			afterEth := counts(t)
			require.Equal(t, afterPax["eth_"+method.name]+1, afterEth["eth_"+method.name])
			require.Equal(t, afterPax["pax_"+method.name], afterEth["pax_"+method.name])
		})
	}
}
