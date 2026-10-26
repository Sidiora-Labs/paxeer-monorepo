package evmrpc_test

import (
	"testing"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestDebugGetRawTransactionReturnsEnvelope(t *testing.T) {
	for _, tx := range []*types.Transaction{multiTxBlockTx1, multiTxBlockTx2, multiTxBlockTx3} {
		t.Run(tx.Hash().Hex(), func(t *testing.T) {
			response := sendRequestGoodWithNamespace(t, "debug", "getRawTransaction", tx.Hash().Hex())
			require.NotContains(t, response, "error")
			encoded, ok := response["result"].(string)
			require.True(t, ok, "expected a hex-encoded transaction envelope")
			raw, err := hexutil.Decode(encoded)
			require.NoError(t, err)
			require.NotEmpty(t, raw)
			want, err := tx.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, want, raw)
			var decoded types.Transaction
			require.NoError(t, decoded.UnmarshalBinary(raw))
			require.Equal(t, tx.Hash(), decoded.Hash())
		})
	}

	t.Run("unknown hash", func(t *testing.T) {
		hash := common.HexToHash("0xdeadbeef")
		response := sendRequestGoodWithNamespace(t, "debug", "getRawTransaction", hash.Hex())
		require.NotContains(t, response, "result")
		errObj, ok := response["error"].(map[string]interface{})
		require.True(t, ok, "expected an RPC error for an unknown transaction")
		require.Equal(t, float64(-32000), errObj["code"])
		require.Contains(t, errObj["message"], "not found")
		require.Contains(t, errObj["message"], hash.Hex())
	})

	for _, method := range []string{"getRawHeader", "getRawBlock", "getRawReceipts"} {
		t.Run(method+" remains unsupported", func(t *testing.T) {
			response := sendRequestGoodWithNamespace(t, "debug", method, "latest")
			require.NotContains(t, response, "result")
			errObj, ok := response["error"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, float64(evmrpc.ErrCodeEVMNotSupported), errObj["code"])
			require.Contains(t, errObj["message"], "debug_"+method)
		})
	}
}
