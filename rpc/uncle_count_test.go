package evmrpc_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestUncleCountKnownBlockIsZero(t *testing.T) {
	tmClient := &MockClient{latestOverride: MockHeight8}
	watermarks := evmrpc.NewWatermarkManager(tmClient, nil, nil, nil)
	api := evmrpc.NewUncleAPI(tmClient, watermarks, evmrpc.ConnectionTypeHTTP)
	ctx := context.Background()

	for _, number := range []rpc.BlockNumber{
		rpc.BlockNumber(1),
		rpc.EarliestBlockNumber,
		rpc.LatestBlockNumber,
		rpc.SafeBlockNumber,
		rpc.FinalizedBlockNumber,
	} {
		t.Run("by_number/"+number.String(), func(t *testing.T) {
			count, err := api.GetUncleCountByBlockNumber(ctx, number)
			require.NoError(t, err)
			require.NotNil(t, count)
			require.Equal(t, hexutil.Uint(0), *count)
			encoded, err := json.Marshal(count)
			require.NoError(t, err)
			require.JSONEq(t, `"0x0"`, string(encoded))
		})
	}

	t.Run("by_hash", func(t *testing.T) {
		count, err := api.GetUncleCountByBlockHash(ctx, common.HexToHash(TestBlockHash))
		require.NoError(t, err)
		require.NotNil(t, count)
		require.Equal(t, hexutil.Uint(0), *count)
		encoded, err := json.Marshal(count)
		require.NoError(t, err)
		require.JSONEq(t, `"0x0"`, string(encoded))
	})

	t.Run("pending_without_snapshot", func(t *testing.T) {
		count, err := api.GetUncleCountByBlockNumber(ctx, rpc.PendingBlockNumber)
		require.ErrorIs(t, err, evmrpc.ErrPendingServedBySequencer)
		require.Nil(t, count)
	})
}

func TestUncleCountUnknownBlockIsNull(t *testing.T) {
	tmClient := &MockClient{latestOverride: MockHeight8}
	watermarks := evmrpc.NewWatermarkManager(tmClient, nil, nil, nil)
	api := evmrpc.NewUncleAPI(tmClient, watermarks, evmrpc.ConnectionTypeHTTP)
	ctx := context.Background()

	t.Run("by_number", func(t *testing.T) {
		count, err := api.GetUncleCountByBlockNumber(ctx, rpc.BlockNumber(MockHeight103+1))
		require.NoError(t, err)
		require.Nil(t, count)
		encoded, err := json.Marshal(count)
		require.NoError(t, err)
		require.Equal(t, "null", string(encoded))
	})

	for _, tc := range []struct {
		name string
		hash common.Hash
	}{
		{name: "unknown", hash: common.HexToHash("0x" + strings.Repeat("bb", 32))},
		{name: "above_watermark", hash: common.HexToHash(DebugTraceBlockHash)},
	} {
		t.Run("by_hash/"+tc.name, func(t *testing.T) {
			count, err := api.GetUncleCountByBlockHash(ctx, tc.hash)
			require.NoError(t, err)
			require.Nil(t, count)
			encoded, err := json.Marshal(count)
			require.NoError(t, err)
			require.Equal(t, "null", string(encoded))
		})
	}
}
