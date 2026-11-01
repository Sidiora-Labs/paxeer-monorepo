package evmrpc_test

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/stretchr/testify/require"
)

type sendPendingAccepter struct {
	next            evmrpc.SequencerAccepter
	pending         *pendingOpenSnapshot
	calls           int
	firstTx         []byte
	sealAfterAccept bool
	accepted        chan struct{}
	resume          <-chan struct{}
}

func (s *sendPendingAccepter) Accept(txBytes []byte) (evmrpc.AcceptedSend, error) {
	s.calls++
	if s.sealAfterAccept {
		if !s.pending.open.Load() {
			return evmrpc.AcceptedSend{}, evmrpc.ErrLayerNotOpen
		}
		accepted, err := s.next.Accept(txBytes)
		if err != nil {
			return evmrpc.AcceptedSend{}, err
		}
		s.pending.open.Store(false)
		s.pending.snapshot = s.pending.snapshot.WithBlockHeight(accepted.Height + 1)
		s.pending.open.Store(true)
		if s.accepted != nil {
			close(s.accepted)
			<-s.resume
		}
		return accepted, nil
	}
	if s.calls == 1 {
		s.firstTx = bytes.Clone(txBytes)
		return evmrpc.AcceptedSend{}, fmt.Errorf("sealed layer: %w", evmrpc.ErrLayerNotOpen)
	}
	if !bytes.Equal(s.firstTx, txBytes) {
		return evmrpc.AcceptedSend{}, fmt.Errorf("retry changed the transaction")
	}
	if !s.pending.open.Load() {
		return evmrpc.AcceptedSend{}, fmt.Errorf("retry before the next open: %w", evmrpc.ErrLayerNotOpen)
	}
	return s.next.Accept(txBytes)
}

func newSendPendingAPI(t *testing.T, h *sendSeqHarness) (*evmrpc.SendAPI, *sendPendingAccepter, *pendingOpenSnapshot, *evmrpc.WatermarkManager, *evmrpc.ReceiptLogFeed) {
	t.Helper()
	snapshot, err := h.exec.Snapshot()
	require.NoError(t, err)
	pending := &pendingOpenSnapshot{PendingSnapshot: h.exec, snapshot: snapshot, started: make(chan struct{})}
	watermarks := evmrpc.NewWatermarkManager(nil, nil, nil, nil)
	watermarks.SetPendingSnapshot(pending)
	accepter := &sendPendingAccepter{next: h.accepter, pending: pending}
	feed := evmrpc.NewReceiptLogFeed(sendSeqCommittedTip)
	option := evmrpc.NewSequencerOption(accepter, feed)
	config, err := option.SendConfig(evmrpcconfig.DefaultConfig)
	require.NoError(t, err)
	api := evmrpc.NewSendAPI(
		h.client,
		func(int64) client.TxConfig { return h.app.GetTxConfig() },
		config,
		&h.app.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) sdk.Context { return h.app.GetContextForDeliverTx([]byte{}) },
		"",
		nil,
		nil,
		nil,
		evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](),
		evmrpc.NewBlockCache(1),
		nil,
		watermarks,
	)
	return api, accepter, pending, watermarks, feed
}

func TestSendRawTransactionRetriesAcrossASealedLayer(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	api, accepter, pending, _, _ := newSendPendingAPI(t, h)
	tx, raw := sendSeqTransfer(t, sender, 0, common.HexToAddress("0x1234"), big.NewInt(1), sendSeqTransferGas)

	ctx, cancel := context.WithTimeout(context.Background(), evmrpc.PendingOpenWait)
	defer cancel()
	opened := make(chan struct{})
	go func() {
		defer close(opened)
		select {
		case <-ctx.Done():
			return
		case <-pending.started:
		}
		timer := time.NewTimer(20 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			pending.open.Store(true)
		}
	}()
	t.Cleanup(func() { cancel(); <-opened })
	start := time.Now()
	hash, err := api.SendRawTransaction(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, tx.Hash(), hash)
	require.Equal(t, 2, accepter.calls)
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	require.True(t, pending.open.Load())
	require.Len(t, h.accepter.receipts, 1)
	require.Equal(t, [32]byte(tx.Hash()), h.verifiedReceipt(t, 0).ActivityID)
	require.Zero(t, h.client.broadcastCount())
}

func TestSendRawTransactionRefusesWhenNoBlockReopens(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	api, accepter, pending, watermarks, _ := newSendPendingAPI(t, h)
	watermarks.SetPendingOpenWait(50 * time.Millisecond)
	_, raw := sendSeqTransfer(t, sender, 0, common.HexToAddress("0x1234"), big.NewInt(1), sendSeqTransferGas)

	start := time.Now()
	_, err := api.SendRawTransaction(context.Background(), raw)
	elapsed := time.Since(start)
	require.ErrorIs(t, err, evmrpc.ErrPendingBlockClosed)
	require.NotErrorIs(t, err, evmrpc.ErrLayerNotOpen)
	require.EqualError(t, err, "no block is open; retry")
	var rpcErr *evmrpc.ErrEVMNotSupported
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, -32000, rpcErr.ErrorCode())
	require.Equal(t, 1, accepter.calls)
	require.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	require.Less(t, elapsed, evmrpc.PendingOpenWait)
	require.False(t, pending.open.Load())
	require.Empty(t, h.accepter.receipts)
	require.Zero(t, h.client.broadcastCount())
}

func TestSendRawTransactionRecordsLogsWhenTheLayerSealsAfterAccept(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	api, accepter, pending, watermarks, feed := newSendPendingAPI(t, h)
	pending.open.Store(true)
	accepter.sealAfterAccept = true
	raw, wantHash := subscribeRawTx(t, sender, 0, nil, big.NewInt(0), subscribeTwoLogsGas, subscribeTwoLogsInitCode)
	filterAPI := pendingFilterAPI(t, &pendingSequencer{app: h.app, exec: h.exec, committed: sendSeqCommittedTip}, &MockClient{latestOverride: sendSeqCommittedTip}, feed)
	crit := filters.FilterCriteria{FromBlock: pendingFilterTag(), ToBlock: pendingFilterTag()}

	hash, err := api.SendRawTransaction(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, wantHash, hash)
	require.Equal(t, 1, accepter.calls)
	require.Len(t, h.accepter.receipts, 1)
	require.Equal(t, [32]byte(hash), h.verifiedReceipt(t, 0).ActivityID)
	current, err := watermarks.PendingContext(context.Background())
	require.NoError(t, err)
	require.Equal(t, sendSeqOpenHeight+1, current.BlockHeight())
	logs, err := filterAPI.GetLogs(context.Background(), crit)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	requireSendPendingLogs(t, logs, hash, crypto.CreateAddress(sender.EvmAddress, 0))
	require.NoError(t, feed.Committed(sendSeqOpenHeight))
	logs, err = filterAPI.GetLogs(context.Background(), crit)
	require.NoError(t, err)
	require.Empty(t, logs)
	require.Zero(t, h.client.broadcastCount())
}

func TestReceiptLogFeedCommittedWaitsForAnInFlightSend(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	api, accepter, pending, _, feed := newSendPendingAPI(t, h)
	pending.open.Store(true)
	accepter.sealAfterAccept = true
	accepter.accepted = make(chan struct{})
	resume := make(chan struct{})
	accepter.resume = resume
	var release sync.Once
	defer release.Do(func() { close(resume) })
	filterAPI := pendingFilterAPI(t, &pendingSequencer{app: h.app, exec: h.exec, committed: sendSeqCommittedTip}, &MockClient{latestOverride: sendSeqCommittedTip}, feed)
	crit := filters.FilterCriteria{FromBlock: pendingFilterTag(), ToBlock: pendingFilterTag()}
	id, err := filterAPI.NewFilter(context.Background(), crit)
	require.NoError(t, err)
	raw, wantHash := subscribeRawTx(t, sender, 0, nil, big.NewInt(0), subscribeTwoLogsGas, subscribeTwoLogsInitCode)
	type outcome struct {
		hash common.Hash
		err  error
	}
	sent := make(chan outcome, 1)
	go func() {
		hash, err := api.SendRawTransaction(context.Background(), raw)
		sent <- outcome{hash, err}
	}()
	select {
	case <-accepter.accepted:
	case result := <-sent:
		t.Fatalf("send returned before the receipt was released: %v", result.err)
	case <-time.After(5 * time.Second):
		t.Fatal("send did not reach acceptance")
	}
	logs, err := filterAPI.GetLogs(context.Background(), crit)
	require.NoError(t, err)
	require.Empty(t, logs, "the send still holds its receipt")
	committing := make(chan struct{})
	committed := make(chan error, 1)
	go func() {
		close(committing)
		committed <- feed.Committed(sendSeqOpenHeight)
	}()
	<-committing
	select {
	case err := <-committed:
		t.Fatalf("commit returned before the in-flight send recorded its receipt: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	release.Do(func() { close(resume) })
	select {
	case result := <-sent:
		require.NoError(t, result.err)
		require.Equal(t, wantHash, result.hash)
	case <-time.After(5 * time.Second):
		t.Fatal("send did not finish after receipt release")
	}
	select {
	case err := <-committed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("commit did not finish after receipt recording")
	}
	changes, err := filterAPI.GetFilterChanges(context.Background(), id)
	require.NoError(t, err)
	recorded, ok := changes.([]*evmrpc.FilterLog)
	require.True(t, ok)
	require.Len(t, recorded, 2, "the filter must receive the send's logs before commit drops them")
	requireSendPendingLogs(t, recorded, wantHash, crypto.CreateAddress(sender.EvmAddress, 0))
	logs, err = filterAPI.GetLogs(context.Background(), crit)
	require.NoError(t, err)
	require.Empty(t, logs, "commit must retire the in-flight send's logs")
	require.Equal(t, 1, accepter.calls)
	require.Zero(t, h.client.broadcastCount())
}

func requireSendPendingLogs(t *testing.T, logs []*evmrpc.FilterLog, hash common.Hash, address common.Address) {
	t.Helper()
	require.Len(t, logs, 2)
	for i, log := range logs {
		require.NotNil(t, log.Seq)
		require.Equal(t, uint64(1), *log.Seq)
		require.Equal(t, uint64(sendSeqOpenHeight), log.BlockNumber)
		require.Equal(t, uint(1), log.TxIndex)
		require.Equal(t, uint(i), log.Index)
		require.Equal(t, hash, log.TxHash)
		require.Equal(t, address, log.Address)
		require.Equal(t, []common.Hash{common.BigToHash(big.NewInt(int64(i + 1)))}, log.Topics)
		require.Empty(t, log.Data)
		require.False(t, log.Removed)
		require.Equal(t, common.Hash{}, log.BlockHash)
	}
}
