package evmrpc_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebugTraceBlockByNumberEntriesMatchBlockTxs(t *testing.T) {
	blockNumber := fmt.Sprintf("0x%x", MockHeight2)
	blockResponse := sendRequestGood(t, "getBlockByNumber", blockNumber, false)
	require.NotContains(t, blockResponse, "error")
	block, ok := blockResponse["result"].(map[string]interface{})
	require.True(t, ok, "expected a block object")
	require.Equal(t, blockNumber, block["number"])
	txs, ok := block["transactions"].([]interface{})
	require.True(t, ok, "expected the block transaction hash list")
	require.Equal(t, []interface{}{
		multiTxBlockTx1.Hash().Hex(),
		multiTxBlockTx2.Hash().Hex(),
		multiTxBlockTx3.Hash().Hex(),
	}, txs, "the multi-transaction block must expose its EVM transactions in order")

	traceResponse := sendRequestGoodWithNamespace(t, "debug", "traceBlockByNumber", blockNumber,
		map[string]interface{}{"tracer": "callTracer"})
	require.NotContains(t, traceResponse, "error")
	entries, ok := traceResponse["result"].([]interface{})
	require.True(t, ok, "expected a block trace entry list")
	require.Len(t, entries, len(txs), "expected one trace entry per EVM transaction")
	for i, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]interface{})
		require.True(t, ok, "trace entry %d must be an object", i)
		hash, ok := entry["txHash"].(string)
		require.True(t, ok, "trace entry %d must include its transaction hash", i)
		require.Equal(t, txs[i], hash, "trace entry %d must match the block transaction order", i)
	}
}
