package cryptosim

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/bench/wrappers"
	"github.com/stretchr/testify/require"
)

func repair1196Await(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("participant did not complete within the bounded drain window")
	}
}

func TestRepairM_1196_CancelFlushRecovery(t *testing.T) {
	cfg := repair1195Config(t)
	c := repair1195Prepare(t, cfg, nil)
	repair1195StartWriter(t, c)
	c.database.ResetTransactionCount()
	c.blockBuilder.nextBlockNumber = int64(c.database.nextBlockNumber)
	blk := c.blockBuilder.buildBlock()
	require.Len(t, blk.transactions, cfg.TransactionsPerBlock)
	require.Len(t, blk.reciepts, cfg.TransactionsPerBlock)

	executor := c.executors[0]
	// Hold the production worker at a real queue acknowledgement. Its queue
	// must contain the whole block and finalization barrier at cancellation.
	barrier := flushRequest{doneChan: make(chan error)}
	executor.workChan <- barrier
	require.Eventually(t, func() bool {
		return len(executor.workChan) == 0
	}, 5*time.Second, time.Millisecond)
	handled := make(chan struct{})
	go func() {
		c.handleNextBlock(blk)
		close(handled)
	}()
	require.Eventually(t, func() bool {
		return len(executor.workChan) == len(blk.transactions)+1
	}, 5*time.Second, time.Millisecond)
	c.cancel()
	select {
	case <-handled:
		t.Fatal("cancelled context was treated as an execution acknowledgement")
	default:
	}

	closed := make(chan struct{})
	var closeErr error
	go func() {
		closeErr = executor.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("executor closed before draining queued transactions")
	default:
	}
	require.NoError(t, <-barrier.doneChan)
	repair1196Await(t, handled)
	repair1196Await(t, closed)
	require.NoError(t, closeErr)
	require.NoError(t, c.closeErr)
	require.Equal(t, blk.TransactionCount(), c.database.TransactionCount())
	require.Empty(t, executor.workChan)
	require.ErrorIs(t, executor.ScheduleForExecution(blk.transactions[0]), ErrExecutorClosed)
	require.NoError(t, executor.Close())

	// Compute the exact last writer for all five production transaction writes.
	expected := make(map[string][]byte)
	for _, txn := range blk.transactions {
		expected[string(txn.srcAccount)] = txn.newSrcBalance
		expected[string(txn.dstAccount)] = txn.newDstBalance
		expected[string(txn.srcAccountSlot)] = txn.newSrcAccountSlot
		expected[string(txn.dstAccountSlot)] = txn.newDstAccountSlot
		expected[string(c.dataGenerator.FeeCollectionAddress())] = txn.newFeeBalance
	}
	c.blockBuilder.Start()
	c.teardown()
	require.NoError(t, c.Close())
	for _, done := range []<-chan struct{}{c.blockBuilder.done, executor.done, c.receiptSimulator.done, c.closeChan} {
		select {
		case <-done:
		default:
			t.Fatal("shutdown completion preceded participant join")
		}
	}

	backend, err := wrappers.NewDBImpl(context.Background(), cfg.Backend, cfg.DataDir, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, backend.Close()) }()
	durable := NewDatabase(cfg, backend, nil, 0)
	for key, value := range expected {
		got, found, err := durable.Get([]byte(key))
		require.NoError(t, err)
		require.True(t, found, "missing durable transaction key %x", key)
		require.Equal(t, value, got, "wrong durable transaction value for %x", key)
	}
	require.EqualValues(t, blk.NextAccountID(), repair1195Counter(t, durable, AccountIDCounterKey()))
	require.EqualValues(t, blk.NextErc20ContractID(), repair1195Counter(t, durable, Erc20IDCounterKey()))
	require.EqualValues(t, blk.BlockNumber()+1, repair1195Counter(t, durable, BlockNumberCounterKey()))
	store, err := receipt.NewReceiptStore(dbconfig.ReceiptStoreConfig{
		DBDirectory: filepath.Join(cfg.DataDir, "receipts"), Backend: "pebbledb",
	}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	repair1195AssertReceipt(t, store, blk)
	require.Equal(t, blk.BlockNumber(), store.LatestVersion())
}

func TestRepairM_1196_FlushFailureIsExplicit(t *testing.T) {
	cfg := repair1195Config(t)
	cfg.GenerateReceipts = false
	c := repair1195Prepare(t, cfg, nil)
	c.database.ResetTransactionCount()
	initialHeight := c.database.nextBlockNumber
	initialVersion := c.database.db.Version()
	c.blockBuilder.nextBlockNumber = int64(initialHeight)
	blk := c.blockBuilder.buildBlock()
	require.NoError(t, c.executors[0].Close())
	require.ErrorIs(t, c.executors[0].Flush(), ErrExecutorClosed)
	require.ErrorIs(t, c.database.FinalizeBlock(blk.NextAccountID(), blk.NextErc20ContractID(), true), ErrExecutorClosed)
	require.Equal(t, initialHeight, c.database.nextBlockNumber)
	require.Equal(t, initialVersion, c.database.db.Version())
	c.handleNextBlock(blk)
	require.ErrorIs(t, c.closeErr, ErrExecutorClosed)
	require.Zero(t, c.database.TransactionCount(), "rejected admission must not be counted")
	c.blockBuilder.Start()
	c.teardown()
	require.ErrorIs(t, c.Close(), ErrExecutorClosed)
	require.ErrorIs(t, c.Close(), ErrExecutorClosed)
	backend, err := wrappers.NewDBImpl(context.Background(), cfg.Backend, cfg.DataDir, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, backend.Close()) }()
	durable := NewDatabase(cfg, backend, nil, 0)
	require.Equal(t, initialHeight, repair1195Counter(t, durable, BlockNumberCounterKey()))
	require.Equal(t, initialVersion, backend.Version())
}

func TestRepairM_1196_ReceiptWorkloadRecovery(t *testing.T) {
	assertReceiptSimulatorBackendWorkload(t)
}

func TestRepairM_1196_DatabaseCloseFlushFailure(t *testing.T) {
	cfg := repair1195Config(t)
	cfg.GenerateReceipts = false
	c := repair1195Prepare(t, cfg, nil)
	initialHeight := c.database.nextBlockNumber
	initialVersion := c.database.db.Version()
	require.NoError(t, c.executors[0].Close())
	c.cancel()
	c.blockBuilder.Start()
	repair1196Await(t, c.blockBuilder.done)

	// A failed final barrier must reach the caller, while Close still releases
	// the real store's directory lock so recovery can open it immediately.
	err := c.database.Close(c.dataGenerator.NextAccountID(), c.dataGenerator.NextErc20ContractID())
	require.ErrorIs(t, err, ErrExecutorClosed)
	require.ErrorContains(t, err, "failed to flush executors")
	backend, err := wrappers.NewDBImpl(context.Background(), cfg.Backend, cfg.DataDir, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, backend.Close()) }()
	require.Equal(t, initialVersion, backend.Version())
	durable := NewDatabase(cfg, backend, nil, 0)
	require.Equal(t, initialHeight, repair1195Counter(t, durable, BlockNumberCounterKey()))
}
