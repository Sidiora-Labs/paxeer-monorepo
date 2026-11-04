package cryptosim

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	crand "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/rand"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/bench/wrappers"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func repair1195Config(t *testing.T) *CryptoSimConfig {
	t.Helper()
	cfg := DefaultCryptoSimConfig()
	cfg.DataDir, cfg.LogDir = t.TempDir(), t.TempDir()
	cfg.Backend = wrappers.MemIAVL
	cfg.NumberOfHotAccounts = 2
	cfg.MinimumNumberOfColdAccounts = 2
	cfg.MinimumNumberOfDormantAccounts = 4
	cfg.MinimumNumberOfErc20Contracts = 3
	cfg.HotErc20ContractSetSize = 1
	cfg.Erc20ContractSize = 64
	cfg.CannedRandomSize = 1 << 20
	cfg.TransactionsPerBlock = 2
	cfg.BlocksPerCommit = 8
	cfg.ThreadsPerCore, cfg.ConstantThreadCount = 0, 1
	cfg.ExecutorQueueSize, cfg.BlockChannelCapacity = 4, 1
	cfg.RecieptChannelCapacity = 8
	cfg.BackgroundMetricsScrapeInterval = 0
	cfg.MetricsAddr = ""
	cfg.GenerateReceipts = true
	cfg.ReceiptKeepRecent, cfg.ReceiptPruneIntervalSeconds = 64, 0
	cfg.NewAccountProbability = 1
	require.NoError(t, cfg.Validate())
	return cfg
}

// This harness controls admission only. All generation, setup, execution,
// finalization, receipt transport, persistence and shutdown use production code.
func repair1195Prepare(t *testing.T, cfg *CryptoSimConfig, metrics *CryptosimMetrics) *CryptoSim {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	backend, err := wrappers.NewDBImpl(ctx, cfg.Backend, cfg.DataDir, nil)
	require.NoError(t, err)
	db := NewDatabase(cfg, backend, metrics, 0)
	rand := crand.NewCannedRandom(cfg.CannedRandomSize, cfg.Seed)
	generator, err := NewDataGenerator(cfg, db, rand, metrics)
	require.NoError(t, err)
	db.nextBlockNumber = generator.InitialNextBlockNumber()
	c := &CryptoSim{
		ctx: ctx, cancel: cancel, config: cfg, database: db,
		dataGenerator: generator, metrics: metrics,
		closeChan: make(chan struct{}), suspendChan: make(chan bool, 1),
	}
	c.executors = []*TransactionExecutor{NewTransactionExecutor(ctx, cancel, cfg, db,
		generator.FeeCollectionAddress(), cfg.ExecutorQueueSize, metrics)}
	db.SetFlushFunc(c.flushExecutors)
	// Construct before setup, exactly as NewCryptoSim does: Start must pick up
	// the blocks finalized by setup as well as the recovered counter.
	c.blockBuilder = NewBlockBuilder(ctx, cfg, metrics, generator)
	require.NoError(t, c.setup())
	return c
}

func repair1195StartWriter(t *testing.T, c *CryptoSim) {
	t.Helper()
	if c.recieptsChan == nil {
		c.recieptsChan = make(chan *block, c.config.RecieptChannelCapacity)
	}
	var err error
	c.receiptSimulator, err = NewRecieptStoreSimulator(c.ctx, c.config, c.recieptsChan,
		c.metrics, c.dataGenerator.Rand().Clone(false))
	require.NoError(t, err)
}

func repair1195Stop(t *testing.T, c *CryptoSim) {
	t.Helper()
	c.cancel()
	c.teardown()
	require.NoError(t, c.Close())
}

func repair1195NextBlock(t *testing.T, c *CryptoSim) *block {
	t.Helper()
	select {
	case blk := <-c.blockBuilder.blocksChan:
		require.Len(t, blk.transactions, c.config.TransactionsPerBlock)
		require.Len(t, blk.reciepts, c.config.TransactionsPerBlock)
		c.handleNextBlock(blk)
		require.NoError(t, c.closeErr)
		return blk
	case <-time.After(5 * time.Second):
		t.Fatal("block producer did not deliver")
		return nil
	}
}

func repair1195Counter(t *testing.T, db *Database, key []byte) uint64 {
	t.Helper()
	value, found, err := db.Get(key)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, value, 8)
	return binary.BigEndian.Uint64(value)
}

func repair1195AssertReceipt(t *testing.T, store receipt.ReceiptStore, blk *block) {
	t.Helper()
	ctx := sdk.NewContext(nil, tmproto.Header{}, false)
	for _, expected := range blk.reciepts {
		actual, err := store.GetReceipt(ctx, common.HexToHash(expected.TxHashHex))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}

func repair1195Reopen(t *testing.T) {
	t.Helper()
	cfg := repair1195Config(t)
	first := repair1195Prepare(t, cfg, nil)
	repair1195StartWriter(t, first)
	initial := first.database.nextBlockNumber
	require.Positive(t, initial)
	first.blockBuilder.Start()
	blk1 := repair1195NextBlock(t, first)
	require.Equal(t, initial, uint64(blk1.BlockNumber()))
	repair1195Stop(t, first)

	second := repair1195Prepare(t, cfg, nil)
	require.Equal(t, uint64(blk1.BlockNumber()+1), second.database.nextBlockNumber)
	require.Equal(t, blk1.NextAccountID(), second.dataGenerator.NextAccountID())
	require.Equal(t, blk1.NextErc20ContractID(), second.dataGenerator.NextErc20ContractID())
	require.Equal(t, uint64(blk1.BlockNumber()+1), repair1195Counter(t, second.database, BlockNumberCounterKey()))
	require.Equal(t, uint64(blk1.NextAccountID()), repair1195Counter(t, second.database, AccountIDCounterKey()))
	require.Equal(t, uint64(blk1.NextErc20ContractID()), repair1195Counter(t, second.database, Erc20IDCounterKey()))
	repair1195StartWriter(t, second)
	repair1195AssertReceipt(t, second.receiptSimulator.store, blk1)
	second.blockBuilder.Start()
	blk2 := repair1195NextBlock(t, second)
	require.Equal(t, blk1.BlockNumber()+1, blk2.BlockNumber())
	for _, old := range blk1.reciepts {
		for _, next := range blk2.reciepts {
			require.NotEqual(t, old.TxHashHex, next.TxHashHex)
		}
	}
	repair1195Stop(t, second)

	third := repair1195Prepare(t, cfg, nil)
	require.Equal(t, uint64(blk2.BlockNumber()+1), third.database.nextBlockNumber)
	require.Equal(t, blk2.NextAccountID(), third.dataGenerator.NextAccountID())
	require.Equal(t, blk2.NextErc20ContractID(), third.dataGenerator.NextErc20ContractID())
	repair1195StartWriter(t, third)
	repair1195AssertReceipt(t, third.receiptSimulator.store, blk1)
	repair1195AssertReceipt(t, third.receiptSimulator.store, blk2)
	require.Equal(t, blk2.BlockNumber(), third.receiptSimulator.store.LatestVersion())
	// No workload on this reopen: setup counters must stay unchanged.
	third.cancel()
	third.blockBuilder.Start()
	repair1195Stop(t, third)
}

func TestRepairM_1195_BlockRestart(t *testing.T) {
	repair1195Reopen(t)
}

func TestRepairM_1195_PersistenceDoc(t *testing.T) {
	doc, err := os.ReadFile("README.md")
	require.NoError(t, err)
	for _, phrase := range []string{"next unused", "drain", "TestRepairM_1195_PersistenceDoc", "Seed", "CannedRandomSize", "crash"} {
		require.Contains(t, string(doc), phrase)
	}
	// Execute the documented bounded reopen example and check all four durable
	// counters plus the decoded receipts after both stops.
	repair1195Reopen(t)
}

func TestRepairM_1195_ForceFlush(t *testing.T) {
	cfg := repair1195Config(t)
	backend, err := wrappers.NewDBImpl(context.Background(), cfg.Backend, cfg.DataDir, nil)
	require.NoError(t, err)
	db := NewDatabase(cfg, backend, nil, 0)
	gen, err := NewDataGenerator(cfg, db, crand.NewCannedRandom(cfg.CannedRandomSize, cfg.Seed), nil)
	require.NoError(t, err)
	_, address, _, err := gen.CreateNewAccount(cfg.PaddedAccountSize, true)
	require.NoError(t, err)
	_, contract, err := gen.CreateNewErc20Contract(cfg.Erc20ContractSize, true)
	require.NoError(t, err)
	accountValue, _, err := db.Get(address)
	require.NoError(t, err)
	contractValue, _, err := db.Get(contract)
	require.NoError(t, err)
	db.IncrementTransactionCount()
	require.NoError(t, db.FinalizeBlock(gen.NextAccountID(), gen.NextErc20ContractID(), false))
	require.Zero(t, db.transactionsInCurrentBlock)
	require.EqualValues(t, 1, db.uncommittedBlockCount)
	require.Zero(t, backend.Version())
	require.NoError(t, db.Close(gen.NextAccountID(), gen.NextErc20ContractID()))

	reopened, err := wrappers.NewDBImpl(context.Background(), cfg.Backend, cfg.DataDir, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, reopened.Close()) }()
	require.EqualValues(t, 1, reopened.Version())
	durable := NewDatabase(cfg, reopened, nil, 0)
	for key, expected := range map[string][]byte{string(address): accountValue, string(contract): contractValue} {
		value, found, err := durable.Get([]byte(key))
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expected, value)
	}
	for _, key := range [][]byte{AccountIDCounterKey(), Erc20IDCounterKey(), BlockNumberCounterKey()} {
		require.EqualValues(t, 1, repair1195Counter(t, durable, key))
	}
}

func TestRepairM_1195_ContractRestart(t *testing.T) {
	cfg := repair1195Config(t)
	first := repair1195Prepare(t, cfg, nil)
	contracts := first.dataGenerator.NextErc20ContractID()
	first.cancel()
	first.blockBuilder.Start()
	repair1195Stop(t, first)

	second := repair1195Prepare(t, cfg, nil)
	require.Equal(t, contracts, second.nextERC20ContractID)
	repair1195StartWriter(t, second)
	second.blockBuilder.Start()
	blk := repair1195NextBlock(t, second)
	// Exercise the same suspend/forced-commit path as the main loop, with a
	// queued resume to bound the pause without a scheduler-dependent sleep.
	second.Resume()
	second.suspend()
	require.Zero(t, second.database.uncommittedBlockCount)
	repair1195Stop(t, second)

	third := repair1195Prepare(t, cfg, nil)
	require.Equal(t, contracts, third.nextERC20ContractID)
	require.Equal(t, contracts, third.dataGenerator.NextErc20ContractID())
	require.Equal(t, blk.NextAccountID(), third.dataGenerator.NextAccountID())
	third.cancel()
	third.blockBuilder.Start()
	repair1195Stop(t, third)
}

func TestRepairM_1195_ReceiptDrain(t *testing.T) {
	cfg := repair1195Config(t)
	c := repair1195Prepare(t, cfg, nil)
	c.recieptsChan = make(chan *block, cfg.RecieptChannelCapacity)
	// Queue finalized, producer-built blocks before the writer starts. The
	// final block is guaranteed queued when cancellation happens.
	var blocks []*block
	c.blockBuilder.nextBlockNumber = int64(c.database.nextBlockNumber)
	for range 3 {
		blk := c.blockBuilder.buildBlock()
		c.handleNextBlock(blk)
		blocks = append(blocks, blk)
	}
	require.Len(t, c.recieptsChan, 3)
	c.cancel()
	repair1195StartWriter(t, c)
	c.blockBuilder.Start()
	repair1195Stop(t, c)
	require.Empty(t, c.recieptsChan)
	select {
	case <-c.receiptSimulator.done:
	default:
		t.Fatal("completion preceded receipt writer join")
	}
	store, err := receipt.NewReceiptStore(dbconfig.ReceiptStoreConfig{
		DBDirectory: filepath.Join(cfg.DataDir, "receipts"), Backend: "pebbledb",
	}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	require.Equal(t, blocks[len(blocks)-1].BlockNumber(), store.LatestVersion())
	for _, blk := range blocks {
		repair1195AssertReceipt(t, store, blk)
	}
}

func TestRepairM_1195_ReceiptStorageError(t *testing.T) {
	cfg := repair1195Config(t)
	c := repair1195Prepare(t, cfg, nil)
	repair1195StartWriter(t, c)
	// A negative producer height is refused by the real backend's metadata
	// writer. No fake store or injected error implementation is involved.
	c.recieptsChan <- NewBlock(cfg, nil, -1, 0)
	c.cancel()
	c.blockBuilder.Start()
	c.teardown()
	require.ErrorContains(t, c.Close(), "version must be non-negative")
	require.ErrorContains(t, c.Close(), "version must be non-negative")
	// Close released ownership even after the error.
	store, err := receipt.NewReceiptStore(dbconfig.ReceiptStoreConfig{
		DBDirectory: filepath.Join(cfg.DataDir, "receipts"), Backend: "pebbledb",
	}, nil)
	require.NoError(t, err)
	require.NoError(t, store.Close())
}

func repair1195Metrics(t *testing.T, reader *otelmetric.ManualReader) map[string]int64 {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	values := make(map[string]int64)
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch v := m.Data.(type) {
			case metricdata.Sum[int64]:
				for _, p := range v.DataPoints {
					values[m.Name] += p.Value
				}
			case metricdata.Histogram[float64]:
				for _, p := range v.DataPoints {
					values[m.Name] += int64(p.Count)
				}
			}
		}
	}
	return values
}

func repair1195ReadWorkload(t *testing.T) {
	t.Helper()
	reader := otelmetric.NewManualReader()
	provider := otelmetric.NewMeterProvider(otelmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(previous)
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()
	cfg := repair1195Config(t)
	metrics := NewCryptosimMetrics(context.Background(), nil, cfg)
	c := repair1195Prepare(t, cfg, metrics)
	repair1195StartWriter(t, c)
	c.blockBuilder.Start()
	blk := repair1195NextBlock(t, c)
	// Wait for the real write path to publish its query targets, then drive
	// the same reader functions used by configured ticker goroutines.
	require.Eventually(t, func() bool {
		entry := c.receiptSimulator.txRing.RandomEntry(crand.NewCannedRandom(128, 5))
		return entry != nil
	}, 5*time.Second, time.Millisecond)
	rand := crand.NewCannedRandom(4096, cfg.Seed)
	require.Eventually(t, func() bool {
		c.receiptSimulator.executeReceiptRead(rand)
		return repair1195Metrics(t, reader)["cryptosim_receipt_reads_found_total"] > 0
	}, 5*time.Second, time.Millisecond)
	c.receiptSimulator.executeLogFilterRead(rand)
	values := repair1195Metrics(t, reader)
	require.EqualValues(t, cfg.TransactionsPerBlock, values["cryptosim_receipts_written_total"])
	require.Positive(t, values["cryptosim_receipt_reads_total"])
	require.Positive(t, values["cryptosim_receipt_read_duration_seconds"])
	require.EqualValues(t, 1, values["cryptosim_receipt_log_filter_reads_total"])
	require.EqualValues(t, 1, values["cryptosim_receipt_log_filter_unsupported_total"])
	require.EqualValues(t, 1, values["cryptosim_receipt_log_filter_duration_seconds"])
	require.Zero(t, values["cryptosim_receipt_errors_total"])
	for name := range values {
		require.NotContains(t, name, "cache")
	}
	_, _, ok := c.receiptSimulator.readWindow()
	require.True(t, ok)
	_, err := c.receiptSimulator.store.FilterLogs(c.receiptSimulator.readContext,
		uint64(blk.BlockNumber()), uint64(blk.BlockNumber()), filters.FilterCriteria{})
	require.True(t, errors.Is(err, receipt.ErrRangeQueryNotSupported))
	repair1195Stop(t, c)
}

func TestRepairM_1195_CacheObserver(t *testing.T) {
	repair1195ReadWorkload(t)
}

func TestRepairM_1195_CacheWindow(t *testing.T) {
	cfg := repair1195Config(t)
	cfg.ReceiptReadMode = "cache"
	require.ErrorContains(t, cfg.Validate(), "ReceiptReadMode")
	cfg.ReceiptReadMode = "recent"
	cfg.ReceiptLogFilterReadMode = "cache"
	require.ErrorContains(t, cfg.Validate(), "ReceiptLogFilterReadMode")
	cfg.ReceiptLogFilterReadMode = "recent"
	require.NoError(t, cfg.Validate())
	repair1195ReadWorkload(t)
}

func TestRepairM_1195_BuildFailure(t *testing.T) {
	// The bounded gate is independent of make, bash and a second Go toolchain.
	// Check the launcher's fail-closed shell contract directly, including the
	// original status and metadata read from the actual executable.
	script, err := os.ReadFile("cryptosim.sh")
	require.NoError(t, err)
	text := string(script)
	build := strings.Index(text, `make -C "$SCRIPT_DIR" build || exit "$?"`)
	metadata := strings.Index(text, `go version -m "$BINARY" || exit "$?"`)
	digest := strings.Index(text, `sha256sum "$BINARY" || exit "$?"`)
	logger := strings.Index(text, "LOGGER_OUTPUT=")
	run := strings.Index(text, `exec "$BINARY" "$@"`)
	require.GreaterOrEqual(t, build, 0)
	require.Greater(t, metadata, build)
	require.Greater(t, digest, metadata)
	require.Greater(t, logger, digest)
	require.Greater(t, run, logger)
}

func TestRepairM_1195_ConstructorShutdown(t *testing.T) {
	reader := otelmetric.NewManualReader()
	provider := otelmetric.NewMeterProvider(otelmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(previous)
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()
	cfg := repair1195Config(t)
	cfg.MaxRuntimeSeconds, cfg.MaxTPS = 1, 10
	cfg.ReceiptReadConcurrency, cfg.ReceiptLogFilterReadConcurrency = 1, 1
	c, err := NewCryptoSim(context.Background(), cfg)
	require.NoError(t, err)
	select {
	case <-c.closeChan:
	case <-time.After(10 * time.Second):
		t.Fatal("bounded workload failed to stop")
	}
	require.NoError(t, c.Close())
	require.NotNil(t, c.mostRecentBlock)
	values := repair1195Metrics(t, reader)
	require.Positive(t, values["cryptosim_receipt_reads_found_total"])
	require.Positive(t, values["cryptosim_receipt_log_filter_reads_total"])
	require.Equal(t, values["cryptosim_receipt_log_filter_reads_total"], values["cryptosim_receipt_log_filter_unsupported_total"])
	store, err := receipt.NewReceiptStore(dbconfig.ReceiptStoreConfig{
		DBDirectory: filepath.Join(cfg.DataDir, "receipts"), Backend: "pebbledb",
	}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	repair1195AssertReceipt(t, store, c.mostRecentBlock)
}
