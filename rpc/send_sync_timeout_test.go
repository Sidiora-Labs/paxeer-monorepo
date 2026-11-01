package evmrpc_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	receiptpkg "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestSendRawTransactionSyncTimeoutReturnsHash(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	ctxProvider := resolutionCtxProvider1315(h)
	h.api = evmrpc.NewSendAPI(
		h.client,
		func(int64) client.TxConfig { return h.app.GetTxConfig() },
		&evmrpc.SendConfig{Sequencer: h.accepter},
		&h.app.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		ctxProvider,
		"",
		nil,
		nil,
		nil,
		evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](),
		evmrpc.NewBlockCache(1),
		nil,
		nil,
	)
	to := common.BytesToAddress(sendSeqLabel("send sync timeout recipient")[:20])
	transfer, raw := sendSeqTransfer(t, sender, 0, to, big.NewInt(1000), sendSeqTransferGas)
	_, err := h.app.EvmKeeper.GetReceipt(ctxProvider(evmrpc.LatestCtxHeight), transfer.Hash())
	require.ErrorIs(t, err, receiptpkg.ErrNotFound)

	client := resolutionInProc1315(t, h.api)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	timeoutMs := hexutil.Uint64(200)
	var receipt map[string]interface{}
	start := time.Now()
	err = client.CallContext(ctx, &receipt, "eth_sendRawTransactionSync", raw, timeoutMs)
	elapsed := time.Since(start)
	require.Less(t, elapsed, time.Second)
	require.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
	var rpcErr rpc.Error
	require.True(t, errors.As(err, &rpcErr), "error: %v", err)
	require.Equal(t, 4, rpcErr.ErrorCode())
	require.Equal(t, "transaction not yet included", rpcErr.Error())
	var dataErr rpc.DataError
	require.True(t, errors.As(err, &dataErr), "error: %v", err)
	require.Equal(t, transfer.Hash().Hex(), dataErr.ErrorData())
	require.Nil(t, receipt)

	require.Len(t, h.accepter.receipts, 1)
	accepted := h.verifiedReceipt(t, 0)
	require.Equal(t, [32]byte(transfer.Hash()), accepted.ActivityID)
	require.Zero(t, accepted.ResultCode)
	require.Zero(t, big.NewInt(1000).Cmp(h.openBalance(t, to)))
	_, err = h.app.EvmKeeper.GetReceipt(ctxProvider(evmrpc.LatestCtxHeight), transfer.Hash())
	require.ErrorIs(t, err, receiptpkg.ErrNotFound)

	t.Run("caller cancellation", func(t *testing.T) {
		_, raw := sendSeqTransfer(t, sender, 1, to, big.NewInt(1000), sendSeqTransferGas)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		timer := time.AfterFunc(50*time.Millisecond, cancel)
		defer timer.Stop()
		start := time.Now()
		receipt, err := h.api.SendRawTransactionSync(ctx, raw, &timeoutMs)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, receipt)
		require.Less(t, time.Since(start), 200*time.Millisecond)
		require.Len(t, h.accepter.receipts, 2)
	})
}
