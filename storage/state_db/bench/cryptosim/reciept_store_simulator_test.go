package cryptosim

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	crand "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/rand"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestRandomEntryInBlockRange(t *testing.T) {
	ring := newTxHashRing(100)
	crand := crand.NewCannedRandom(42, 1024*1024)

	for i := uint64(1); i <= 50; i++ {
		ring.Push(common.BigToHash(common.Big0), i, common.Address{})
	}

	tests := []struct {
		name     string
		minBlock uint64
		maxBlock uint64
		wantNil  bool
	}{
		{
			name:     "range covers all entries",
			minBlock: 1,
			maxBlock: 50,
			wantNil:  false,
		},
		{
			name:     "range covers recent entries only",
			minBlock: 40,
			maxBlock: 50,
			wantNil:  false,
		},
		{
			name:     "range covers old entries only",
			minBlock: 1,
			maxBlock: 10,
			wantNil:  false,
		},
		{
			name:     "range outside all entries",
			minBlock: 100,
			maxBlock: 200,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ring.RandomEntryInBlockRange(crand, tt.minBlock, tt.maxBlock)
			if tt.wantNil && entry != nil {
				t.Fatalf("expected nil, got block %d", entry.blockNumber)
			}
			if !tt.wantNil && entry == nil {
				t.Fatalf("expected entry in [%d,%d], got nil", tt.minBlock, tt.maxBlock)
			}
			if entry != nil && (entry.blockNumber < tt.minBlock || entry.blockNumber > tt.maxBlock) {
				t.Fatalf("expected block in [%d,%d], got %d", tt.minBlock, tt.maxBlock, entry.blockNumber)
			}
		})
	}
}

func TestReceiptSimulatorBackendWorkload(t *testing.T) {
	assertReceiptSimulatorBackendWorkload(t)
}

func assertReceiptSimulatorBackendWorkload(t *testing.T) {
	t.Helper()
	reader := otelmetric.NewManualReader()
	provider := otelmetric.NewMeterProvider(otelmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(previous)
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()

	cfg := repair1195Config(t)
	cfg.ReceiptReadConcurrency, cfg.ReceiptLogFilterReadConcurrency = 2, 2
	cfg.ReceiptReadsPerSecond, cfg.ReceiptLogFilterReadsPerSecond = 200, 200
	cfg.ReceiptLogFilterMinBlockRange, cfg.ReceiptLogFilterMaxBlockRange = 1, 2
	metrics := NewCryptosimMetrics(context.Background(), nil, cfg)
	c := repair1195Prepare(t, cfg, metrics)
	repair1195StartWriter(t, c)
	require.Equal(t, "pebble", receipt.BackendTypeName(c.receiptSimulator.store))
	c.blockBuilder.nextBlockNumber = int64(c.database.nextBlockNumber)
	var blocks []*block
	for range 2 {
		blk := c.blockBuilder.buildBlock()
		require.Len(t, blk.transactions, cfg.TransactionsPerBlock)
		require.Len(t, blk.reciepts, cfg.TransactionsPerBlock)
		c.handleNextBlock(blk)
		require.NoError(t, c.closeErr)
		blocks = append(blocks, blk)
	}

	// Observe the configured ticker goroutines themselves, rather than
	// manually invoking the read functions or counting ring samples.
	require.Eventually(t, func() bool {
		values := repair1195Metrics(t, reader)
		return values["cryptosim_receipt_reads_found_total"] > 0 &&
			values["cryptosim_receipt_log_filter_unsupported_total"] > 0
	}, 5*time.Second, time.Millisecond)
	for _, blk := range blocks {
		for _, expected := range blk.reciepts {
			require.Eventually(t, func() bool {
				_, err := c.receiptSimulator.store.GetReceipt(c.receiptSimulator.readContext,
					common.HexToHash(expected.TxHashHex))
				return err == nil
			}, 5*time.Second, time.Millisecond)
		}
		repair1195AssertReceipt(t, c.receiptSimulator.store, blk)
	}

	// The actual backend's refused shape must be visible as a refusal, never
	// a successful empty result or a generic storage failure.
	first, last := blocks[0], blocks[len(blocks)-1]
	logs, err := c.receiptSimulator.store.FilterLogs(c.receiptSimulator.readContext,
		uint64(first.BlockNumber()), uint64(last.BlockNumber()), filters.FilterCriteria{
			Addresses: []common.Address{common.HexToAddress(first.reciepts[0].ContractAddress)},
		})
	require.True(t, errors.Is(err, receipt.ErrRangeQueryNotSupported))
	require.Empty(t, logs)
	c.cancel()
	c.blockBuilder.Start()
	c.teardown()
	require.NoError(t, c.Close())
	values := repair1195Metrics(t, reader)
	require.EqualValues(t, len(blocks)*cfg.TransactionsPerBlock, values["cryptosim_receipts_written_total"])
	require.EqualValues(t, len(blocks), values["cryptosim_receipt_block_write_duration_seconds"])
	require.Positive(t, values["cryptosim_receipt_reads_found_total"])
	require.Equal(t, values["cryptosim_receipt_reads_total"],
		values["cryptosim_receipt_reads_found_total"]+values["cryptosim_receipt_reads_not_found_total"])
	require.Equal(t, values["cryptosim_receipt_reads_total"], values["cryptosim_receipt_read_duration_seconds"])
	require.Positive(t, values["cryptosim_receipt_log_filter_reads_total"])
	require.Equal(t, values["cryptosim_receipt_log_filter_reads_total"], values["cryptosim_receipt_log_filter_unsupported_total"])
	require.Equal(t, values["cryptosim_receipt_log_filter_reads_total"], values["cryptosim_receipt_log_filter_duration_seconds"])
	require.Zero(t, values["cryptosim_receipt_errors_total"])

	store, err := receipt.NewReceiptStore(dbconfig.ReceiptStoreConfig{
		DBDirectory: filepath.Join(cfg.DataDir, "receipts"), Backend: "pebbledb",
	}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	require.Equal(t, last.BlockNumber(), store.LatestVersion())
	for _, blk := range blocks {
		repair1195AssertReceipt(t, store, blk)
		for _, expected := range blk.reciepts {
			actual, err := store.GetReceipt(c.receiptSimulator.readContext, common.HexToHash(expected.TxHashHex))
			require.NoError(t, err)
			require.EqualValues(t, blk.BlockNumber(), actual.BlockNumber)
			require.Len(t, actual.Logs, 1)
			require.Equal(t, erc20TransferEventSignatureHex, actual.Logs[0].Topics[0])
			require.Equal(t, expected.Logs[0].Data, actual.Logs[0].Data)
		}
	}
	logs, err = store.FilterLogs(c.receiptSimulator.readContext,
		uint64(first.BlockNumber()), uint64(last.BlockNumber()), filters.FilterCriteria{})
	require.ErrorIs(t, err, receipt.ErrRangeQueryNotSupported)
	require.Empty(t, logs)
}
