package flatkv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	paxdbtypes "github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/ktype"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/lthash"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/vtype"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
	"go.opentelemetry.io/otel/metric"
)

const (
	importBatchSize = 20000
	ingestChanSize  = 1 << 16 // 64K buffered main channel
	workerChanSize  = 1024    // per-DB worker channel
)

var _ types.Importer = (*KVImporter)(nil)

// flushHookForTest, when set by tests in this package, is invoked at the
// start of every dbWorker flush. It exists solely for whitebox tests of
// the backpressure / fail-fast paths (see importer_test.go) and loads
// nil in production.
//
// Stored via atomic.Pointer (rather than a bare package-level func) so
// that any future test that calls t.Parallel() and concurrently swaps
// the hook does not race with worker goroutines reading it. The hot-path
// cost is a single atomic load per flush, equivalent to an aligned
// pointer read.
var flushHookForTest atomic.Pointer[func(string)]

// dbWorker owns a single PebbleDB and its LtHash accumulation. It reads
// key/value pairs from its channel, buffers them into a PebbleDB batch,
// and flushes (commit + LtHash update) when the buffer is full or the
// channel is closed.
type dbWorker struct {
	ctx     context.Context
	dir     string
	db      paxdbtypes.KeyValueDB
	ch      chan rawKVPair
	batch   paxdbtypes.Batch
	ltPairs []lthash.KVPairWithLastValue
	latest  map[string][]byte
	ltHash  *lthash.LtHash
	flushes int64
	pairs   int64
}

func newDBWorker(ctx context.Context, dir string, db paxdbtypes.KeyValueDB, ltHash *lthash.LtHash) *dbWorker {
	return &dbWorker{
		ctx:     ctx,
		dir:     dir,
		db:      db,
		ch:      make(chan rawKVPair, workerChanSize),
		batch:   db.NewBatch(),
		ltPairs: make([]lthash.KVPairWithLastValue, 0, importBatchSize),
		ltHash:  ltHash,
		latest:  make(map[string][]byte),
	}
}

// run drains the worker channel until closed, flushing whenever the
// buffer reaches importBatchSize. If done fires, the worker abandons
// remaining work and exits immediately.
func (w *dbWorker) run(done <-chan struct{}) error {
	defer func() {
		if w.batch != nil {
			_ = w.batch.Close()
			w.batch = nil
		}
	}()
	for {
		select {
		case kv, ok := <-w.ch:
			if !ok {
				return w.flush()
			}
			// Read the preceding occurrence from this batch or a completed batch.
			// Keeping only this batch's values bounds the replacement cache.
			last, ok := w.latest[string(kv.Key)]
			if !ok {
				var err error
				last, err = w.db.Get(kv.Key)
				if err != nil && !errorutils.IsNotFound(err) {
					return fmt.Errorf("%s read previous value: %w", w.dir, err)
				}
				if errorutils.IsNotFound(err) {
					last = nil
				}
			}
			if err := w.batch.Set(kv.Key, kv.Value); err != nil {
				return fmt.Errorf("%s set: %w", w.dir, err)
			}
			w.ltPairs = append(w.ltPairs, lthash.KVPairWithLastValue{
				Key:       kv.Key,
				Value:     kv.Value,
				LastValue: last,
			})
			w.latest[string(kv.Key)] = kv.Value
			if len(w.ltPairs) >= importBatchSize {
				if err := w.flush(); err != nil {
					return err
				}
			}
		case <-done:
			return nil
		}
	}
}

// flush commits the current PebbleDB batch and updates the running LtHash.
func (w *dbWorker) flush() (err error) {
	if len(w.ltPairs) == 0 {
		return nil
	}
	if hook := flushHookForTest.Load(); hook != nil {
		(*hook)(w.dir)
	}
	start := time.Now()
	pairCount := len(w.ltPairs)
	defer func() {
		otelMetrics.ImportWorkerFlushLatency.Record(w.ctx, secondsSince(start),
			metric.WithAttributes(dbAttr(w.dir), successAttr(err)))
	}()

	// TODO:In theory, we could offload lattice hash calculation to a work pool and get parallelism between DB operations and hash calculations. Cryptosim performance makes me think we could probably get a 2-3x speedup from this, assuming receiving data from the network isn't the bottleneck.
	newHash, _ := lthash.ComputeLtHash(w.ltHash, w.ltPairs)

	syncOpt := paxdbtypes.WriteOptions{Sync: false}
	if err := w.batch.Commit(syncOpt); err != nil {
		return fmt.Errorf("%s commit: %w", w.dir, err)
	}

	// Commit does not release a Pebble batch. Close before replacing it;
	// run's defer still owns cleanup if Set or Commit fails.
	closeErr := w.batch.Close()
	w.batch = nil
	if closeErr != nil {
		return fmt.Errorf("%s close batch: %w", w.dir, closeErr)
	}
	w.ltHash = newHash

	addImportKVPairs(w.ctx, w.dir, pairCount)
	w.flushes++
	w.pairs += int64(pairCount)
	w.batch = w.db.NewBatch()
	clear(w.ltPairs)
	w.ltPairs = w.ltPairs[:0]
	clear(w.latest)
	return nil
}

// KVImporter implements types.Importer using a channel-based pipeline with
// per-DB worker goroutines. AddNode sends pairs into a buffered channel; a
// dispatcher goroutine routes each pair to the correct DB worker; each worker
// independently batches writes and computes LtHash.
type KVImporter struct {
	store   *CommitStore
	version int64

	ingestCh chan rawKVPair
	workers  map[paxdbtypes.KeyValueDB]*dbWorker
	wg       sync.WaitGroup

	// done is closed on the first pipeline error so that AddNode,
	// the dispatcher, and all workers bail immediately.
	done       chan struct{}
	closeOnce  sync.Once
	firstErr   atomic.Pointer[error]
	finishOnce sync.Once
	finishErr  error
}

func NewKVImporter(store *CommitStore, version int64) types.Importer {
	imp := &KVImporter{
		store:    store,
		version:  version,
		ingestCh: make(chan rawKVPair, ingestChanSize),
		workers:  make(map[paxdbtypes.KeyValueDB]*dbWorker, 4),
		done:     make(chan struct{}),
	}

	if version <= 0 {
		imp.setErr(fmt.Errorf("flatkv import: target version must be positive, got %d", version))
	}

	// Imports bypass the WAL and may flush raw rows before validation fails
	// (or the process stops). Until WriteSnapshot publishes the complete import,
	// this working directory cannot be reused as a committed snapshot clone.
	// A successful WriteSnapshot installs a new SNAPSHOT_BASE marker.
	marker := filepath.Join(store.flatkvDir(), workingDirName, snapshotBaseFile)
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		imp.setErr(fmt.Errorf("flatkv import: invalidate working snapshot base: %w", err))
	}

	for _, ndb := range store.namedDataDBs() {
		w := newDBWorker(
			store.ctx,
			ndb.dir,
			ndb.db,
			store.perDBWorkingLtHash[ndb.dir],
		)
		imp.workers[ndb.db] = w
	}

	for _, w := range imp.workers {
		imp.wg.Add(1)
		go func(w *dbWorker) {
			defer imp.wg.Done()
			if err := w.run(imp.done); err != nil {
				imp.setErr(err)
			}
		}(w)
	}

	imp.wg.Add(1)
	go func() {
		defer imp.wg.Done()
		imp.dispatch()
	}()

	return imp
}

// dispatch reads from the main ingest channel, routes each pair, and sends
// it to the appropriate worker channel. It exits when ingestCh is closed
// (normal shutdown) or done fires (error fast-path).
func (imp *KVImporter) dispatch() {
	defer func() {
		for _, w := range imp.workers {
			close(w.ch)
		}
	}()

	for {
		select {
		case kv, ok := <-imp.ingestCh:
			if !ok {
				return
			}
			db, err := imp.store.routePhysicalKey(kv.Key)
			if err != nil {
				imp.setErr(fmt.Errorf("route key: %w", err))
				return
			}
			select {
			case imp.workers[db].ch <- kv:
			case <-imp.done:
				return
			}
		case <-imp.done:
			return
		}
	}
}

func (imp *KVImporter) setErr(err error) {
	if imp.firstErr.CompareAndSwap(nil, &err) {
		imp.closeOnce.Do(func() { close(imp.done) })
	}
}

func (imp *KVImporter) getErr() error {
	p := imp.firstErr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (imp *KVImporter) Err() error {
	return imp.getErr()
}

func (imp *KVImporter) AddModule(_ string) error {
	return nil
}

// validateNode checks the physical leaf contract shared by KVExporter and
// ImportTranslator. Snapshot corruption is fatal, never a skipped leaf.
func (imp *KVImporter) validateNode(node *types.SnapshotNode) error {
	if node == nil {
		return errors.New("flatkv import: nil node")
	}
	if node.Height != 0 {
		return fmt.Errorf("flatkv import: expected leaf height 0, got %d", node.Height)
	}
	if node.Version != imp.version {
		return fmt.Errorf("flatkv import: node version %d does not match target %d", node.Version, imp.version)
	}
	if len(node.Key) == 0 {
		return errors.New("flatkv import: missing physical key")
	}
	if ktype.IsMetaKey(node.Key) {
		return errors.New("flatkv import: reserved metadata key")
	}
	db, err := imp.store.routePhysicalKey(node.Key)
	if err != nil {
		return fmt.Errorf("route key: %w", err)
	}
	module, inner, err := ktype.StripModulePrefix(node.Key)
	if err != nil {
		return fmt.Errorf("route key: %w", err)
	}
	if module == "" || (module == keys.EVMStoreKey && len(inner) == 0) {
		return errors.New("flatkv import: empty module or EVM key")
	}
	if module == keys.EVMStoreKey {
		// Optimized physical keys use the same fixed widths as logical keys.
		// Account fragments are merged under the nonce prefix, never codehash.
		for _, kind := range []keys.EVMKeyKind{ktype.EVMKeyAccount, keys.EVMKeyCodeHash, keys.EVMKeyCode, keys.EVMKeyStorage} {
			prefix, _ := keys.EVMKeyPrefixByte(kind)
			if inner[0] != prefix {
				continue
			}
			if kind == keys.EVMKeyCodeHash || len(inner) != 1+keys.InternalKeyLen(kind) {
				return fmt.Errorf("flatkv import: noncanonical physical key %x", node.Key)
			}
		}
	}

	var value interface {
		vtype.VType
		GetBlockHeight() int64
	}
	switch db {
	case imp.store.accountDB:
		value, err = vtype.DeserializeAccountData(node.Value)
	case imp.store.storageDB:
		value, err = vtype.DeserializeStorageData(node.Value)
	case imp.store.codeDB:
		value, err = vtype.DeserializeCodeData(node.Value)
	case imp.store.legacyDB:
		value, err = vtype.DeserializeLegacyData(node.Value)
	default:
		return errors.New("flatkv import: unknown target database")
	}
	if err != nil {
		return fmt.Errorf("flatkv import: invalid value envelope for %x: %w", node.Key, err)
	}
	if value.IsDelete() {
		return fmt.Errorf("flatkv import: deletion envelope for %x", node.Key)
	}
	if height := value.GetBlockHeight(); height < 0 || height > imp.version {
		return fmt.Errorf("flatkv import: value height %d outside target %d", height, imp.version)
	}
	return nil
}

func (imp *KVImporter) AddNode(node *types.SnapshotNode) {
	if imp.getErr() != nil {
		return
	}
	if node != nil {
		// The asynchronous pipeline owns its bytes, even when a producer reuses
		// its buffers immediately after AddNode returns.
		copyNode := *node
		copyNode.Key = bytes.Clone(node.Key)
		copyNode.Value = bytes.Clone(node.Value)
		node = &copyNode
	}
	if err := imp.validateNode(node); err != nil {
		imp.setErr(err)
		return
	}
	select {
	case imp.ingestCh <- rawKVPair{Key: node.Key, Value: node.Value}:
	case <-imp.done:
	}
}

// Abort tears down the worker pipeline without finalizing the import.
// It records reason as the first pipeline error (so any in-flight worker
// also bails fast) and then runs Close, which observes the non-nil error
// and skips FinalizeImport / WriteSnapshot. The on-disk FlatKV directory
// is left at its pre-import committed version, allowing the operator to
// retry without --force.
//
// Use this when an external error (context cancellation, exporter
// failure, translator failure, etc.) makes the in-progress import
// unsafe to commit. Abort is idempotent and safe to interleave with
// Close: whichever runs first wins; later calls are no-ops.
func (imp *KVImporter) Abort(reason error) error {
	if reason == nil {
		reason = errors.New("flatkv import aborted")
	}
	imp.setErr(reason)
	return imp.Close()
}

// Close is idempotent: the first call drains workers, finalizes the import,
// and writes a snapshot; subsequent calls just return the cached result.
// Idempotency is required because the import-from-memiavl tool may invoke
// Close on both the success and error paths.
//
// If the first pipeline error has already been recorded (either by a
// worker or by Abort), Close skips FinalizeImport / WriteSnapshot so the
// store stays at its pre-import version.
func (imp *KVImporter) Close() error {
	imp.finishOnce.Do(func() {
		start := time.Now()
		var err error
		defer func() {
			otelMetrics.ImportLatency.Record(imp.store.ctx, secondsSince(start),
				metric.WithAttributes(successAttr(err)))
			flushes, pairs := imp.importStats()
			if err == nil {
				otelMetrics.CurrentVersion.Record(imp.store.ctx, imp.store.committedVersion)
				otelMetrics.CurrentSnapshotHeight.Record(imp.store.ctx, imp.store.committedVersion)
				logger.Info("FlatKV import complete",
					"version", imp.version,
					"flushes", flushes,
					"pairs", pairs,
					"elapsed", time.Since(start))
			} else {
				logger.Error("FlatKV import failed",
					"version", imp.version,
					"flushes", flushes,
					"pairs", pairs,
					"elapsed", time.Since(start),
					"err", err)
			}
			imp.finishErr = err
		}()

		close(imp.ingestCh)
		imp.wg.Wait()

		if err = imp.getErr(); err != nil {
			return
		}

		for _, w := range imp.workers {
			imp.store.perDBWorkingLtHash[w.dir] = w.ltHash
		}

		if err = imp.store.FinalizeImport(imp.version); err != nil {
			err = fmt.Errorf("failed to finalize import: %w", err)
			return
		}

		// Write a snapshot so the imported data survives store reopen / restart.
		// Import bypasses the WAL, so without a snapshot the next LoadVersion
		// would clone from the pre-import snapshot and lose all imported data.
		if err = imp.store.WriteSnapshot(""); err != nil {
			err = fmt.Errorf("failed to import when writing snapshot: %w", err)
			return
		}
	})

	return imp.finishErr
}

func (imp *KVImporter) importStats() (flushes int64, pairs int64) {
	for _, w := range imp.workers {
		flushes += w.flushes
		pairs += w.pairs
	}
	return flushes, pairs
}
