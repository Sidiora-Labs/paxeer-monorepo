package evmrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type servedLabelsServer struct {
	*evmrpc.HTTPServer
}

func (s servedLabelsServer) HTTPMethods() []string {
	return s.HTTPServer.HTTPMethods()
}

func TestServedMethodsRecordServedEndpointLabel(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	latency, err := provider.Meter("evmrpc").Float64Histogram("evmrpc_request_latency_seconds", metric.WithUnit("s"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	t.Cleanup(evmrpc.SwapRequestLatencyForTest(latency))
	httpServer, wsServer, httpClient, wsClient := newRegistrationServers(t)
	server := servedLabelsServer{HTTPServer: httpServer}

	const address = "0x1234567890123456789023456789012345678901"
	raw, err := multiTxBlockTx1.MarshalBinary()
	require.NoError(t, err)
	txArgs := map[string]interface{}{"from": address, "to": address, "gas": "0x5208"}
	criteria := map[string]interface{}{"fromBlock": "0x2", "toBlock": "0x2"}
	params := map[string][]interface{}{
		"debug_getRawBlock":                          {"0x2"},
		"debug_getRawHeader":                         {"0x2"},
		"debug_getRawReceipts":                       {"0x2"},
		"debug_getRawTransaction":                    {multiTxBlockTx1.Hash()},
		"debug_traceBlockByHash":                     {MultiTxBlockHash},
		"debug_traceBlockByNumber":                   {"0x2"},
		"debug_traceCall":                            {txArgs, "0x2"},
		"debug_traceStateAccess":                     {multiTxBlockTx1.Hash()},
		"debug_traceTransaction":                     {multiTxBlockTx1.Hash()},
		"debug_traceTransactionProfile":              {multiTxBlockTx1.Hash()},
		"eth_accounts":                               {},
		"eth_blobBaseFee":                            {},
		"eth_blockNumber":                            {},
		"eth_call":                                   {txArgs, "0x2"},
		"eth_chainId":                                {},
		"eth_coinbase":                               {},
		"eth_createAccessList":                       {txArgs, "0x2"},
		"eth_estimateGas":                            {txArgs, "0x2"},
		"eth_estimateGasAfterCalls":                  {txArgs, []interface{}{}, "0x2"},
		"eth_feeHistory":                             {"0x1", "0x2", []float64{50}},
		"eth_gasPrice":                               {},
		"eth_getBalance":                             {address, "0x2"},
		"eth_getBlockByHash":                         {MultiTxBlockHash, false},
		"eth_getBlockByNumber":                       {"0x2", false},
		"eth_getBlockReceipts":                       {"0x2"},
		"eth_getBlockTransactionCountByHash":         {MultiTxBlockHash},
		"eth_getBlockTransactionCountByNumber":       {"0x2"},
		"eth_getCode":                                {address, "0x2"},
		"eth_getFilterChanges":                       {"0x1"},
		"eth_getFilterLogs":                          {"0x1"},
		"eth_getLogs":                                {criteria},
		"eth_getNonce":                               {address},
		"eth_getProof":                               {address, []string{}, "0x2"},
		"eth_getRawTransactionByBlockHashAndIndex":   {MultiTxBlockHash, "0x0"},
		"eth_getRawTransactionByBlockNumberAndIndex": {"0x2", "0x0"},
		"eth_getRawTransactionByHash":                {multiTxBlockTx1.Hash()},
		"eth_getStorageAt":                           {address, "0x0", "0x2"},
		"eth_getTransactionByBlockHashAndIndex":      {MultiTxBlockHash, "0x0"},
		"eth_getTransactionByBlockNumberAndIndex":    {"0x2", "0x0"},
		"eth_getTransactionByHash":                   {multiTxBlockTx1.Hash()},
		"eth_getTransactionCount":                    {address, "0x2"},
		"eth_getTransactionErrorByHash":              {multiTxBlockTx1.Hash()},
		"eth_getTransactionReceipt":                  {multiTxBlockTx1.Hash()},
		"eth_getUncleByBlockHashAndIndex":            {MultiTxBlockHash, "0x0"},
		"eth_getUncleByBlockNumberAndIndex":          {"0x2", "0x0"},
		"eth_getUncleCountByBlockHash":               {MultiTxBlockHash},
		"eth_getUncleCountByBlockNumber":             {"0x2"},
		"eth_getVMError":                             {multiTxBlockTx1.Hash()},
		"eth_maxPriorityFeePerGas":                   {},
		"eth_newBlockFilter":                         {},
		"eth_newFilter":                              {criteria},
		"eth_newPendingTransactionFilter":            {},
		"eth_sendRawTransaction":                     {hexutil.Encode(raw)},
		"eth_sendRawTransactionSync":                 {hexutil.Encode(raw)},
		"eth_sendTransaction":                        {txArgs},
		"eth_sign":                                   {address, "0x"},
		"eth_signTransaction":                        {txArgs},
		"eth_syncing":                                {},
		"eth_uninstallFilter":                        {"0x1"},
		"net_listening":                              {},
		"net_peerCount":                              {},
		"net_version":                                {},
		"txpool_content":                             {},
		"txpool_status":                              {},
		"web3_clientVersion":                         {},
		"web3_sha3":                                  {"0x"},
	}

	counts := func(t *testing.T, connectionType evmrpc.ConnectionType) map[string]uint64 {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &data))
		result := make(map[string]uint64)
		for _, scope := range data.ScopeMetrics {
			if scope.Scope.Name != "evmrpc" {
				continue
			}
			for _, instrument := range scope.Metrics {
				if instrument.Name != "evmrpc_request_latency_seconds" {
					continue
				}
				histogram, ok := instrument.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "latency metric must be a float64 histogram")
				for _, point := range histogram.DataPoints {
					endpoint, ok := point.Attributes.Value(attribute.Key("endpoint"))
					require.True(t, ok, "latency point must have an endpoint label")
					connection, ok := point.Attributes.Value(attribute.Key("connection"))
					require.True(t, ok, "latency point must have a connection label")
					if connection.AsString() == string(connectionType) {
						result[endpoint.AsString()] += point.Count
					}
				}
			}
		}
		return result
	}

	checkCall := func(t *testing.T, client *rpc.Client, connectionType evmrpc.ConnectionType, method string) {
		t.Helper()
		arguments, ok := params[method]
		require.True(t, ok, "served method needs an explicit valid argument fixture")
		before := counts(t, connectionType)[method]
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		var result json.RawMessage
		err := client.CallContext(ctx, &result, method, arguments...)
		if err != nil {
			var rpcError rpc.Error
			if !errors.As(err, &rpcError) {
				t.Errorf("%s did not return a JSON-RPC response: %v", method, err)
			} else if rpcError.ErrorCode() == -32601 {
				t.Errorf("%s is advertised but was not dispatched: %v", method, err)
			} else if rpcError.ErrorCode() == -32602 {
				t.Errorf("%s rejected its argument fixture: %v", method, err)
			}
		}
		after := counts(t, connectionType)[method]
		if after <= before {
			t.Errorf("%s recorded no evmrpc_request_latency_seconds point under its served endpoint name on %s (before=%d, after=%d, response error=%v)", method, connectionType, before, after, err)
		}
	}

	namespaces := map[string]int{"eth": 0, "net": 0, "web3": 0, "txpool": 0, "debug": 0}
	for _, method := range server.HTTPMethods() {
		namespace, _, _ := strings.Cut(method, "_")
		if _, included := namespaces[namespace]; !included {
			continue
		}
		namespaces[namespace]++
		t.Run(method, func(t *testing.T) {
			checkCall(t, httpClient, evmrpc.ConnectionTypeHTTP, method)
		})
	}
	for namespace, called := range namespaces {
		require.Positive(t, called, "must call served %s_ methods", namespace)
	}

	t.Run("WebSocket", func(t *testing.T) {
		for _, method := range []string{
			"web3_clientVersion",
			"web3_sha3",
			"eth_getUncleByBlockNumberAndIndex",
			"eth_getUncleByBlockHashAndIndex",
			"eth_getUncleCountByBlockNumber",
			"eth_getUncleCountByBlockHash",
		} {
			t.Run(method, func(t *testing.T) {
				require.Contains(t, wsServer.WSMethods(), method)
				checkCall(t, wsClient, evmrpc.ConnectionTypeWS, method)
			})
		}
	})
}
