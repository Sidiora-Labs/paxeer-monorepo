package evmrpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestUncleByBlockIndexIsNull(t *testing.T) {
	watermarks := NewWatermarkManager(nil, nil, nil, nil)
	api := NewUncleAPI(nil, watermarks)
	require.Same(t, watermarks, api.watermarks)
	ctx := context.Background()
	index := hexutil.Uint(0)

	t.Run("by_number", func(t *testing.T) {
		uncle, err := api.GetUncleByBlockNumberAndIndex(ctx, rpc.BlockNumber(1), index)
		require.NoError(t, err)
		require.Nil(t, uncle)
		encoded, err := json.Marshal(uncle)
		require.NoError(t, err)
		require.Equal(t, "null", string(encoded))
	})

	t.Run("by_hash", func(t *testing.T) {
		uncle, err := api.GetUncleByBlockHashAndIndex(ctx, common.HexToHash("0x01"), index)
		require.NoError(t, err)
		require.Nil(t, uncle)
		encoded, err := json.Marshal(uncle)
		require.NoError(t, err)
		require.Equal(t, "null", string(encoded))
	})
}
