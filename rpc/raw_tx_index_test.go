package evmrpc_test

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func newRawTransactionIndexAPI() *evmrpc.RawTransactionAPI {
	ctxProvider := func(height int64) sdk.Context {
		ctx := Ctx
		if height == MockHeight2 {
			ctx = MultiTxCtx
		}
		if height == evmrpc.LatestCtxHeight {
			height = MockHeight103
		}
		return ctx.WithBlockHeight(height).WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
	}
	tmClient := &MockClient{latestOverride: MockHeight2}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, nil)
	return evmrpc.NewRawTransactionAPI(tmClient, EVMKeeper, ctxProvider, func(int64) client.TxConfig { return TxConfig }, "", evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), watermarks, evmrpc.NewBlockCache(8), &sync.Mutex{})
}

func requireRawTransactionEnvelope(t *testing.T, raw hexutil.Bytes, want *types.Transaction) {
	t.Helper()
	require.NotEmpty(t, raw)
	var decoded types.Transaction
	require.NoError(t, decoded.UnmarshalBinary(raw))
	require.Equal(t, want.Hash(), decoded.Hash())
	wantRaw, err := want.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, wantRaw, []byte(raw))
}

func TestGetRawTransactionByBlockNumberAndIndex(t *testing.T) {
	api := newRawTransactionIndexAPI()
	ctx := context.Background()
	for _, blockNr := range []rpc.BlockNumber{MockHeight2, rpc.LatestBlockNumber, rpc.SafeBlockNumber, rpc.FinalizedBlockNumber} {
		t.Run(blockNr.String(), func(t *testing.T) {
			for index, want := range []*types.Transaction{multiTxBlockTx1, multiTxBlockTx2, multiTxBlockTx3} {
				raw, err := api.GetRawTransactionByBlockNumberAndIndex(ctx, blockNr, hexutil.Uint(index))
				require.NoError(t, err)
				requireRawTransactionEnvelope(t, raw, want)
			}
		})
	}
	for _, index := range []hexutil.Uint{3, math.MaxUint32, ^hexutil.Uint(0)} {
		t.Run("past end "+index.String(), func(t *testing.T) {
			raw, err := api.GetRawTransactionByBlockNumberAndIndex(ctx, MockHeight2, index)
			require.NoError(t, err)
			require.Nil(t, raw)
		})
	}
	if strconv.IntSize == 64 {
		index := uint64(math.MaxUint32) + 1
		raw, err := api.GetRawTransactionByBlockNumberAndIndex(ctx, MockHeight2, hexutil.Uint(index))
		require.NoError(t, err)
		require.Nil(t, raw)
	}
	t.Run("unknown block", func(t *testing.T) {
		raw, err := api.GetRawTransactionByBlockNumberAndIndex(ctx, MockHeight103+1, 0)
		require.NoError(t, err)
		require.Nil(t, raw)
	})
	t.Run("pending unavailable", func(t *testing.T) {
		raw, err := api.GetRawTransactionByBlockNumberAndIndex(ctx, rpc.PendingBlockNumber, 0)
		require.ErrorIs(t, err, evmrpc.ErrPendingServedBySequencer)
		require.Nil(t, raw)
	})
}

func TestGetRawTransactionByBlockHashAndIndex(t *testing.T) {
	api := newRawTransactionIndexAPI()
	ctx := context.Background()
	hash := common.HexToHash(MultiTxBlockHash)
	for index, want := range []*types.Transaction{multiTxBlockTx1, multiTxBlockTx2, multiTxBlockTx3} {
		t.Run(hexutil.Uint(index).String(), func(t *testing.T) {
			raw, err := api.GetRawTransactionByBlockHashAndIndex(ctx, hash, hexutil.Uint(index))
			require.NoError(t, err)
			requireRawTransactionEnvelope(t, raw, want)
		})
	}
	for _, index := range []hexutil.Uint{3, math.MaxUint32, ^hexutil.Uint(0)} {
		t.Run("past end "+index.String(), func(t *testing.T) {
			raw, err := api.GetRawTransactionByBlockHashAndIndex(ctx, hash, index)
			require.NoError(t, err)
			require.Nil(t, raw)
		})
	}
	if strconv.IntSize == 64 {
		index := uint64(math.MaxUint32) + 1
		raw, err := api.GetRawTransactionByBlockHashAndIndex(ctx, hash, hexutil.Uint(index))
		require.NoError(t, err)
		require.Nil(t, raw)
	}
	t.Run("unknown hash", func(t *testing.T) {
		raw, err := api.GetRawTransactionByBlockHashAndIndex(ctx, common.HexToHash("0x"+strings.Repeat("b", 64)), 0)
		require.NoError(t, err)
		require.Nil(t, raw)
	})
	t.Run("above watermark", func(t *testing.T) {
		raw, err := api.GetRawTransactionByBlockHashAndIndex(ctx, common.HexToHash(DebugTraceBlockHash), 0)
		require.NoError(t, err)
		require.Nil(t, raw)
	})
}
