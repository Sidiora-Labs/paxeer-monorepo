package cryptosim

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	crand "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/rand"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	dbm "github.com/tendermint/tm-db"
)

// Ring entries provide bounded, written query targets for the Pebble backend.
const defaultTxHashRingSize = 3_000_000

// txHashEntry stores a written tx hash along with its block and contract address,
// used by reader goroutines to generate realistic log filter queries.
type txHashEntry struct {
	txHash          common.Hash
	blockNumber     uint64
	contractAddress common.Address
}

// txHashRing is a fixed-size ring buffer of recently written tx hashes.
// Writers call Push from the main loop; readers call RandomEntry from goroutines.
type txHashRing struct {
	mu      sync.RWMutex
	entries []txHashEntry
	size    int
	head    int
	count   int
}

func newTxHashRing(size int) *txHashRing {
	return &txHashRing{
		entries: make([]txHashEntry, size),
		size:    size,
	}
}

// Push appends a tx hash entry to the ring, overwriting the oldest entry when full.
func (r *txHashRing) Push(txHash common.Hash, blockNumber uint64, contractAddress common.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.head] = txHashEntry{
		txHash:          txHash,
		blockNumber:     blockNumber,
		contractAddress: contractAddress,
	}
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// RandomEntry returns a random entry from the ring, using CannedRandom to
// avoid potential rand.Rand hotspots under high-concurrency benchmarks.
func (r *txHashRing) RandomEntry(crand *crand.CannedRandom) *txHashEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.count == 0 {
		return nil
	}
	idx := int(crand.Int64Range(0, int64(r.count)))
	entry := r.entries[idx]
	return &entry
}

const maxRingSampleAttempts = 100

// RandomEntryInBlockRange samples a random entry whose blockNumber falls within
// [minBlock, maxBlock]. Returns nil if no matching entry is found after a
// bounded number of attempts.
func (r *txHashRing) RandomEntryInBlockRange(crand *crand.CannedRandom, minBlock, maxBlock uint64) *txHashEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.count == 0 {
		return nil
	}
	for range maxRingSampleAttempts {
		idx := int(crand.Int64Range(0, int64(r.count)))
		entry := r.entries[idx]
		if entry.blockNumber >= minBlock && entry.blockNumber <= maxBlock {
			return &entry
		}
	}
	return nil
}

// A simulated receipt store with concurrent reads, writes, and pruning
// backed by the production Pebble receipt.ReceiptStore.
type RecieptStoreSimulator struct {
	ctx    context.Context
	cancel context.CancelFunc

	config *CryptoSimConfig

	recieptsChan chan *block

	store       receipt.ReceiptStore
	crand       *crand.CannedRandom
	txRing      *txHashRing
	metrics     *CryptosimMetrics
	done        chan struct{}
	readers     sync.WaitGroup
	err         error
	readContext sdk.Context
	legacyDB    dbm.DB
}

// NewRecieptStoreSimulator starts a receipt writer and optional backend readers.
// The owner must stop producers, close recieptsChan and call Wait. Cancelling
// ctx stops readers but never discards queued writes.
func NewRecieptStoreSimulator(
	ctx context.Context,
	config *CryptoSimConfig,
	recieptsChan chan *block,
	metrics *CryptosimMetrics,
	crand *crand.CannedRandom,
) (*RecieptStoreSimulator, error) {
	derivedCtx, cancel := context.WithCancel(ctx)

	storeCfg := dbconfig.ReceiptStoreConfig{
		DBDirectory:          filepath.Join(config.DataDir, "receipts"),
		Backend:              "pebbledb",
		KeepRecent:           int(config.ReceiptKeepRecent),
		PruneIntervalSeconds: int(config.ReceiptPruneIntervalSeconds),
	}

	// Standalone runs have no legacy chain receipts. Supply a real empty store
	// for GetReceipt's legacy lookup when an async write is not yet visible.
	legacyDB := dbm.NewMemDB()
	legacyKey := sdk.NewMemoryStoreKeys("cryptosim-legacy-receipts")["cryptosim-legacy-receipts"]
	legacyStore := store.NewCommitMultiStore(legacyDB)
	legacyStore.MountStoreWithDB(legacyKey, sdk.StoreTypeMemory, nil)
	if err := legacyStore.LoadLatestVersion(); err != nil {
		cancel()
		return nil, errors.Join(err, legacyDB.Close())
	}
	store, err := receipt.NewReceiptStore(storeCfg, legacyKey)
	if err != nil {
		cancel()
		return nil, errors.Join(fmt.Errorf("failed to create receipt store: %w", err), legacyDB.Close())
	}

	ringSize := defaultTxHashRingSize
	if config.ReceiptKeepRecent > 0 && config.ReceiptKeepRecent < int64(ringSize/max(1, config.TransactionsPerBlock)) {
		ringSize = int(config.ReceiptKeepRecent) * max(1, config.TransactionsPerBlock)
	}
	txRing := newTxHashRing(ringSize)

	r := &RecieptStoreSimulator{
		ctx:          derivedCtx,
		cancel:       cancel,
		config:       config,
		recieptsChan: recieptsChan,
		store:        store,
		crand:        crand,
		txRing:       txRing,
		metrics:      metrics,
		done:         make(chan struct{}),
		readContext:  sdk.NewContext(legacyStore, tmproto.Header{}, false),
		legacyDB:     legacyDB,
	}

	if config.ReceiptReadConcurrency > 0 && config.ReceiptReadsPerSecond > 0 {
		r.startReceiptReaders()
	}
	if config.ReceiptLogFilterReadConcurrency > 0 && config.ReceiptLogFilterReadsPerSecond > 0 {
		r.startLogFilterReaders()
	}

	go r.mainLoop()
	return r, nil
}

// Wait joins the draining writer, all readers, and the store close.
func (r *RecieptStoreSimulator) Wait() error {
	<-r.done
	return r.err
}

func (r *RecieptStoreSimulator) mainLoop() {
	defer close(r.done)
	for blk := range r.recieptsChan {
		if err := r.processBlock(blk); err != nil {
			r.err = errors.Join(r.err, err)
		}
	}
	r.cancel()
	r.readers.Wait()
	r.err = errors.Join(r.err, r.store.Close(), r.legacyDB.Close())
}

// Processes a block of receipts using the production ReceiptStore.SetReceipts path,
// then populates the ring buffer with contract addresses for log filter reads.
func (r *RecieptStoreSimulator) processBlock(blk *block) error {
	blockNumber := uint64(blk.BlockNumber()) //nolint:gosec

	records := make([]receipt.ReceiptRecord, 0, len(blk.reciepts))
	var marshalErr error

	type ringEntry struct {
		txHash          common.Hash
		contractAddress common.Address
	}
	ringEntries := make([]ringEntry, 0, len(blk.reciepts))

	for _, rcpt := range blk.reciepts {
		if rcpt == nil {
			continue
		}

		receiptBytes, err := rcpt.Marshal()
		if err != nil {
			marshalErr = errors.Join(marshalErr, err)
			r.metrics.ReportReceiptError()
			continue
		}

		txHash := common.HexToHash(rcpt.TxHashHex)
		records = append(records, receipt.ReceiptRecord{
			TxHash:       txHash,
			Receipt:      rcpt,
			ReceiptBytes: receiptBytes,
		})

		ringEntries = append(ringEntries, ringEntry{
			txHash:          txHash,
			contractAddress: common.HexToAddress(rcpt.ContractAddress),
		})
	}

	if marshalErr != nil {
		return fmt.Errorf("marshal block %d receipts: %w", blockNumber, marshalErr)
	}

	if len(records) > 0 {
		sdkCtx := sdk.NewContext(nil, tmproto.Header{Height: int64(blockNumber)}, false) //nolint:gosec

		start := time.Now()
		if err := r.store.SetReceipts(sdkCtx, records); err != nil {
			r.metrics.ReportReceiptError()
			return fmt.Errorf("write block %d receipts: %w", blockNumber, err)
		}
		r.metrics.RecordReceiptBlockWriteDuration(time.Since(start))
		r.metrics.ReportReceiptsWritten(int64(len(records)))
	}

	for _, entry := range ringEntries {
		r.txRing.Push(entry.txHash, blockNumber, entry.contractAddress)
	}

	if err := r.store.SetLatestVersion(int64(blockNumber)); err != nil { //nolint:gosec
		r.metrics.ReportReceiptError()
		return fmt.Errorf("update receipt version %d: %w", blockNumber, err)
	}
	return nil
}

// startReceiptReaders launches dedicated goroutines for receipt-by-hash lookups.
func (r *RecieptStoreSimulator) startReceiptReaders() {
	readerCount := r.config.ReceiptReadConcurrency
	totalReadsPerSec := r.config.ReceiptReadsPerSecond
	if totalReadsPerSec <= 0 {
		totalReadsPerSec = 1000
	}

	readsPerReader := totalReadsPerSec / readerCount
	if readsPerReader < 1 {
		readsPerReader = 1
	}

	for i := 0; i < readerCount; i++ {
		readerCrand := r.crand.Clone(true)
		r.readers.Add(1)
		go r.tickerLoop(readsPerReader, readerCrand, r.executeReceiptRead)
	}

	fmt.Printf("Started %d receipt reader goroutines (%d reads/sec each)\n",
		readerCount, readsPerReader)
}

// startLogFilterReaders launches dedicated goroutines for log filter (eth_getLogs) queries.
func (r *RecieptStoreSimulator) startLogFilterReaders() {
	readerCount := r.config.ReceiptLogFilterReadConcurrency
	totalReadsPerSec := r.config.ReceiptLogFilterReadsPerSecond
	if totalReadsPerSec <= 0 {
		totalReadsPerSec = 100
	}

	readsPerReader := totalReadsPerSec / readerCount
	if readsPerReader < 1 {
		readsPerReader = 1
	}

	for i := 0; i < readerCount; i++ {
		readerCrand := r.crand.Clone(true)
		r.readers.Add(1)
		go r.tickerLoop(readsPerReader, readerCrand, r.executeLogFilterRead)
	}

	fmt.Printf("Started %d log filter reader goroutines (%d reads/sec each)\n",
		readerCount, readsPerReader)
}

func (r *RecieptStoreSimulator) tickerLoop(readsPerSecond int, crand *crand.CannedRandom, fn func(*crand.CannedRandom)) {
	defer r.readers.Done()
	interval := max(time.Nanosecond, time.Second/time.Duration(readsPerSecond))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			fn(crand)
		}
	}
}

// readWindow derives a supported query range from backend retention metadata.
func (r *RecieptStoreSimulator) readWindow() (uint64, uint64, bool) {
	latest := r.store.LatestVersion()
	if latest <= 0 {
		return 0, 0, false
	}
	minimum := max(int64(1), r.store.EarliestVersion())
	if r.config.ReceiptKeepRecent > 0 {
		minimum = max(minimum, latest-r.config.ReceiptKeepRecent+1)
	}
	return uint64(minimum), uint64(latest), minimum <= latest
}

// executeReceiptRead queries a written hash in the active backend's read window.
func (r *RecieptStoreSimulator) executeReceiptRead(crand *crand.CannedRandom) {
	minimum, latest, ok := r.readWindow()
	if !ok {
		return
	}
	entry := r.txRing.RandomEntryInBlockRange(crand, minimum, latest)
	if entry == nil {
		return
	}

	r.metrics.ReportReceiptRead()

	sdkCtx := r.readContext
	start := time.Now()
	rcpt, err := r.store.GetReceipt(sdkCtx, entry.txHash)
	r.metrics.RecordReceiptReadDuration(time.Since(start).Seconds())

	if errors.Is(err, receipt.ErrNotFound) {
		r.metrics.ReportReceiptReadNotFound()
		return
	}
	if err != nil {
		r.metrics.ReportReceiptError()
		return
	}
	if rcpt != nil {
		r.metrics.ReportReceiptReadFound()
		return
	}
	r.metrics.ReportReceiptReadNotFound()
}

// executeLogFilterRead observes a real FilterLogs call. Pebble explicitly
// refuses range queries; record that outcome separately from storage failures.
func (r *RecieptStoreSimulator) executeLogFilterRead(crand *crand.CannedRandom) {
	minimum, latest, ok := r.readWindow()
	if !ok {
		return
	}
	entry := r.txRing.RandomEntryInBlockRange(crand, minimum, latest)
	if entry == nil {
		return
	}
	rangeSize := uint64(crand.Int64Range(
		int64(r.config.ReceiptLogFilterMinBlockRange),
		int64(r.config.ReceiptLogFilterMaxBlockRange)+1))
	fromBlock := entry.blockNumber
	toBlock := fromBlock + min(rangeSize-1, latest-fromBlock)

	crit := filters.FilterCriteria{
		Addresses: []common.Address{entry.contractAddress},
	}

	sdkCtx := r.readContext
	start := time.Now()
	r.metrics.ReportReceiptLogFilterRead()
	logs, err := r.store.FilterLogs(sdkCtx, fromBlock, toBlock, crit)
	r.metrics.RecordReceiptLogFilterDuration(time.Since(start).Seconds())
	r.metrics.RecordLogFilterLogsReturned(int64(len(logs)))

	if errors.Is(err, receipt.ErrRangeQueryNotSupported) {
		r.metrics.ReportReceiptLogFilterUnsupported()
	} else if err != nil {
		r.metrics.ReportReceiptError()
	}
}

// convertLogsForTx converts evmtypes.Log entries to ethtypes.Log entries.
// Mirrors receipt.getLogsForTx.
func convertLogsForTx(rcpt *evmtypes.Receipt, logStartIndex uint) []*ethtypes.Log {
	logs := make([]*ethtypes.Log, 0, len(rcpt.Logs))
	for _, l := range rcpt.Logs {
		logs = append(logs, convertLogEntry(l, rcpt, logStartIndex))
	}
	return logs
}

// convertLogEntry converts a single evmtypes.Log to an ethtypes.Log.
// Mirrors receipt.convertLog.
func convertLogEntry(l *evmtypes.Log, rcpt *evmtypes.Receipt, logStartIndex uint) *ethtypes.Log {
	return &ethtypes.Log{
		Address:     common.HexToAddress(l.Address),
		Topics:      mapTopics(l.Topics),
		Data:        l.Data,
		BlockNumber: rcpt.BlockNumber,
		TxHash:      common.HexToHash(rcpt.TxHashHex),
		TxIndex:     uint(rcpt.TransactionIndex),
		Index:       uint(l.Index) + logStartIndex,
	}
}

// mapTopics converts hex-encoded topic strings to common.Hash values.
func mapTopics(topics []string) []common.Hash {
	result := make([]common.Hash, len(topics))
	for i, t := range topics {
		result[i] = common.HexToHash(t)
	}
	return result
}
