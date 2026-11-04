package migration

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	commonmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
)

// All migration builders sample exact cursors in logs at this interval.
const migrationBoundarySnapshotInterval = 10 * time.Second

// migrationRunStats holds the in-process aggregate counts for a single
// MigrationManager run. Populated by MigrationMetrics.RecordBatch and
// emitted via MigrationManager.logMigrationCompleteSummary at completion.
// These are not persisted; after a restart they summarize the resumed
// segment only.
type migrationRunStats struct {
	batches                  int64
	keysMigrated             int64
	keyBytesMigrated         int64
	valueBytesMigrated       int64
	originalPairsRoutedOldDB int64
	originalPairsRoutedNewDB int64
	oldDBPairsWritten        int64
	newDBPairsWritten        int64
}

// migrationBatchStats captures the per-ApplyChangeSets counters that
// MigrationManager hands to MigrationMetrics.RecordBatch.
type migrationBatchStats struct {
	keysMigrated             int64
	keyBytesMigrated         int64
	valueBytesMigrated       int64
	originalPairsRoutedOldDB int64
	originalPairsRoutedNewDB int64
	oldDBPairsWritten        int64
	newDBPairsWritten        int64
}

// MigrationMetrics has two responsibilities, intentionally colocated so
// the manager only has to track one collaborator:
//
//  1. OTel telemetry sink. NewMigrationMetrics wires counters/gauges
//     through the global MeterProvider; SetVersion, SetBoundary,
//     RecordBatch, RecordApplyDuration, and the boundary snapshot loop
//     all emit through it. When OTel handles are absent (nil exporter,
//     newLocalMigrationMetrics) the corresponding Record/Add calls are
//     skipped per-counter, so emission is best-effort.
//
//  2. In-process run-stat aggregator. RecordBatch also accumulates a
//     migrationRunStats summary under mu; MigrationManager reads it via
//     RunStats / Elapsed to emit the completion log. The aggregator
//     keeps working even when no OTel exporter is configured (tests,
//     embedded use), which is why NewMigrationManager substitutes a
//     local-only metrics instance when the caller passes nil.
//
// All methods are nil-safe. Callers (and tests) that do not care about
// metrics can pass a nil *MigrationMetrics to NewMigrationManager.
//
// Unit convention: durations in "s", bytes in "By", counts via curly-brace
// annotations (UCUM, https://ucum.org/ucum).
type MigrationMetrics struct {
	ctx    context.Context
	cancel context.CancelFunc

	// wg tracks the background boundary-snapshot goroutine (if any) so
	// Close can block until it exits.
	wg sync.WaitGroup

	// targetVersion is captured at construction time so the boundary
	// snapshot goroutine can tell, without any DB access, when the
	// migration has completed and it can stop logging cursor snapshots.
	targetVersion uint64

	keysMigratedTotal             metric.Int64Counter
	keyBytesMigratedTotal         metric.Int64Counter
	valueBytesMigratedTotal       metric.Int64Counter
	batchesTotal                  metric.Int64Counter
	originalPairsRoutedOldDBTotal metric.Int64Counter
	originalPairsRoutedNewDBTotal metric.Int64Counter
	oldDBPairsWrittenTotal        metric.Int64Counter
	newDBPairsWrittenTotal        metric.Int64Counter
	applyDuration                 metric.Float64Histogram
	version                       metric.Int64Gauge
	boundarySnapshot              metric.Int64Gauge
	keysProgress                  metric.Int64Gauge
	bytesProgress                 metric.Int64Gauge

	// startedAt is captured when the metrics object is constructed and
	// reported as the elapsed-time anchor in the completion summary.
	startedAt time.Time

	mu              sync.Mutex
	currentBoundary MigrationBoundary
	currentVersion  uint64
	runStats        migrationRunStats
}

// NewMigrationMetrics constructs a MigrationMetrics using the global OTel
// MeterProvider. The caller must have configured the MeterProvider with a
// Prometheus or other exporter before calling this.
//
// targetVersion is the version the associated migration is transitioning
// to; the boundary-snapshot goroutine stops logging once that version
// is observed. All gauges use a fixed, unlabeled series.
//
// When boundarySnapshotInterval <= 0 the snapshot goroutine is not started;
// everything else still works. When ctx is cancelled, or Close is called,
// the snapshot goroutine exits.
func NewMigrationMetrics(
	ctx context.Context,
	targetVersion uint64,
	boundarySnapshotInterval time.Duration,
) *MigrationMetrics {
	ctx, cancel := context.WithCancel(ctx)
	meter := otel.Meter("paxdb_migration")

	keysMigratedTotal, _ := meter.Int64Counter(
		"paxdb_migration_keys_migrated_total",
		metric.WithDescription("Total number of keys promoted from the old DB to the new DB"),
		metric.WithUnit("{count}"),
	)
	keyBytesMigratedTotal, _ := meter.Int64Counter(
		"paxdb_migration_key_bytes_migrated_total",
		metric.WithDescription("Running sum of bytes of migrated keys (len(Key))"),
		metric.WithUnit("By"),
	)
	valueBytesMigratedTotal, _ := meter.Int64Counter(
		"paxdb_migration_value_bytes_migrated_total",
		metric.WithDescription("Running sum of bytes of migrated values (len(Value))"),
		metric.WithUnit("By"),
	)
	batchesTotal, _ := meter.Int64Counter(
		"paxdb_migration_batches_total",
		metric.WithDescription("Total ApplyChangeSets calls processed by MigrationManager"),
		metric.WithUnit("{count}"),
	)
	originalPairsRoutedOldDBTotal, _ := meter.Int64Counter(
		"paxdb_migration_original_pairs_routed_old_db_total",
		metric.WithDescription("Caller-supplied KV pairs routed to the old DB during active migration"),
		metric.WithUnit("{count}"),
	)
	originalPairsRoutedNewDBTotal, _ := meter.Int64Counter(
		"paxdb_migration_original_pairs_routed_new_db_total",
		metric.WithDescription("Caller-supplied KV pairs routed to the new DB during active migration"),
		metric.WithUnit("{count}"),
	)
	oldDBPairsWrittenTotal, _ := meter.Int64Counter(
		"paxdb_migration_old_db_pairs_written_total",
		metric.WithDescription("KV pairs written to the old DB per ApplyChangeSets (migration deletes + caller writes)"),
		metric.WithUnit("{count}"),
	)
	newDBPairsWrittenTotal, _ := meter.Int64Counter(
		"paxdb_migration_new_db_pairs_written_total",
		metric.WithDescription("KV pairs written to the new DB per ApplyChangeSets (migrated values + caller writes + boundary metadata)"),
		metric.WithUnit("{count}"),
	)
	applyDuration, _ := meter.Float64Histogram(
		"paxdb_migration_apply_change_sets_duration_seconds",
		metric.WithDescription("Wall-clock time spent in each MigrationManager.ApplyChangeSets call"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(commonmetrics.LatencyBuckets...),
	)
	version, _ := meter.Int64Gauge(
		"paxdb_migration_version",
		metric.WithDescription("Currently-observed migration version. Equals targetVersion once the "+
			"migration is complete, startVersion while in progress."),
		metric.WithUnit("{version}"),
	)
	boundarySnapshot, _ := meter.Int64Gauge(
		"paxdb_migration_boundary_snapshot",
		metric.WithDescription("Migration status: 0 not started, 1 in progress, 2 complete. Exact cursors are logged."),
		metric.WithUnit("{status}"),
	)

	keysProgress, _ := meter.Int64Gauge(
		"paxdb_migration_keys_migrated",
		metric.WithDescription("Keys migrated in this process run; resets on restart, not a completion percentage"),
		metric.WithUnit("{count}"),
	)
	bytesProgress, _ := meter.Int64Gauge(
		"paxdb_migration_bytes_migrated",
		metric.WithDescription("Key and value bytes migrated in this process run; resets on restart"),
		metric.WithUnit("By"),
	)

	m := &MigrationMetrics{
		ctx:                           ctx,
		cancel:                        cancel,
		targetVersion:                 targetVersion,
		keysMigratedTotal:             keysMigratedTotal,
		keyBytesMigratedTotal:         keyBytesMigratedTotal,
		valueBytesMigratedTotal:       valueBytesMigratedTotal,
		batchesTotal:                  batchesTotal,
		originalPairsRoutedOldDBTotal: originalPairsRoutedOldDBTotal,
		originalPairsRoutedNewDBTotal: originalPairsRoutedNewDBTotal,
		oldDBPairsWrittenTotal:        oldDBPairsWrittenTotal,
		newDBPairsWrittenTotal:        newDBPairsWrittenTotal,
		applyDuration:                 applyDuration,
		version:                       version,
		boundarySnapshot:              boundarySnapshot,
		keysProgress:                  keysProgress,
		bytesProgress:                 bytesProgress,
		startedAt:                     time.Now(),
	}

	if boundarySnapshotInterval > 0 {
		m.startBoundarySnapshotLoop(boundarySnapshotInterval)
	}
	return m
}

// SetBoundary updates the in-memory current boundary. No DB access. Safe
// to call concurrently with the snapshot ticker; not safe to call
// concurrently with itself from multiple goroutines, but the
// MigrationManager only updates the boundary from a single ApplyChangeSets
// caller at a time.
func (m *MigrationMetrics) SetBoundary(b MigrationBoundary) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.currentBoundary = b
	m.recordProgressLocked()
	m.mu.Unlock()
}

// SetVersion updates the in-memory current migration version and records
// the version gauge immediately so Grafana sees the transition without
// waiting for the next snapshot tick.
func (m *MigrationMetrics) SetVersion(v uint64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.currentVersion = v
	m.recordProgressLocked()
	m.mu.Unlock()
}

// newLocalMigrationMetrics returns a *MigrationMetrics with no OTel
// counters wired up but with a live startedAt and runStats aggregator.
// Used internally by MigrationManager when the caller passes nil so
// completion-summary aggregation does not depend on a configured
// MeterProvider.
func newLocalMigrationMetrics() *MigrationMetrics {
	return &MigrationMetrics{startedAt: time.Now()}
}

// RecordBatch aggregates per-ApplyChangeSets counters into the run
// summary and emits the corresponding OTel counters. Safe to call on a
// nil receiver. Counters with no configured exporter (e.g. tests using
// newLocalMigrationMetrics) are skipped individually; in-process
// aggregation still runs so the completion summary stays accurate.
func (m *MigrationMetrics) RecordBatch(batch migrationBatchStats) {
	if m == nil {
		return
	}

	m.mu.Lock()
	m.runStats.batches++
	m.runStats.keysMigrated += batch.keysMigrated
	m.runStats.keyBytesMigrated += batch.keyBytesMigrated
	m.runStats.valueBytesMigrated += batch.valueBytesMigrated
	m.runStats.originalPairsRoutedOldDB += batch.originalPairsRoutedOldDB
	m.runStats.originalPairsRoutedNewDB += batch.originalPairsRoutedNewDB
	m.runStats.oldDBPairsWritten += batch.oldDBPairsWritten
	m.runStats.newDBPairsWritten += batch.newDBPairsWritten
	m.recordProgressLocked()
	m.mu.Unlock()

	ctx := context.Background()
	if m.batchesTotal != nil {
		m.batchesTotal.Add(ctx, 1)
	}
	if m.keysMigratedTotal != nil && batch.keysMigrated > 0 {
		m.keysMigratedTotal.Add(ctx, batch.keysMigrated)
	}
	if m.keyBytesMigratedTotal != nil && batch.keyBytesMigrated > 0 {
		m.keyBytesMigratedTotal.Add(ctx, batch.keyBytesMigrated)
	}
	if m.valueBytesMigratedTotal != nil && batch.valueBytesMigrated > 0 {
		m.valueBytesMigratedTotal.Add(ctx, batch.valueBytesMigrated)
	}
	if m.originalPairsRoutedOldDBTotal != nil && batch.originalPairsRoutedOldDB > 0 {
		m.originalPairsRoutedOldDBTotal.Add(ctx, batch.originalPairsRoutedOldDB)
	}
	if m.originalPairsRoutedNewDBTotal != nil && batch.originalPairsRoutedNewDB > 0 {
		m.originalPairsRoutedNewDBTotal.Add(ctx, batch.originalPairsRoutedNewDB)
	}
	if m.oldDBPairsWrittenTotal != nil && batch.oldDBPairsWritten > 0 {
		m.oldDBPairsWrittenTotal.Add(ctx, batch.oldDBPairsWritten)
	}
	if m.newDBPairsWrittenTotal != nil && batch.newDBPairsWritten > 0 {
		m.newDBPairsWrittenTotal.Add(ctx, batch.newDBPairsWritten)
	}
}

// RunStats returns a copy of the in-process aggregated counters under the
// metrics mutex. Returns the zero value on a nil receiver. Reserved for
// MigrationManager.logMigrationCompleteSummary and tests; callers must
// not mutate the returned struct.
func (m *MigrationMetrics) RunStats() migrationRunStats {
	if m == nil {
		return migrationRunStats{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runStats
}

// Elapsed returns the wall-clock duration since the metrics object was
// constructed. Returns zero on a nil receiver, or on a zero-value
// MigrationMetrics that skipped the construction helpers; the latter
// guard prevents tens-of-years durations from accidental struct literals.
func (m *MigrationMetrics) Elapsed() time.Duration {
	if m == nil || m.startedAt.IsZero() {
		return 0
	}
	return time.Since(m.startedAt)
}

// RecordApplyDuration records the wall-clock time spent in a single
// ApplyChangeSets call. Invoked from a defer in the manager so both
// success and error paths are captured.
func (m *MigrationMetrics) RecordApplyDuration(d time.Duration) {
	if m == nil || m.applyDuration == nil {
		return
	}
	m.applyDuration.Record(context.Background(), d.Seconds())
}

// snapshot returns a safe copy of the in-memory boundary and version
// under the mutex. The returned boundary shares its internal key slice
// with the stored value; callers must not mutate it.
func (m *MigrationMetrics) snapshot() (MigrationBoundary, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentBoundary, m.currentVersion
}

// startBoundarySnapshotLoop refreshes fixed-series gauges and logs the exact
// cursor every interval. No cursor or version is a metric label. Cardinality
// stays constant regardless of migration duration or collector retention.
// The loop exits after logging completion, on cancellation, or on Close.
func (m *MigrationMetrics) startBoundarySnapshotLoop(interval time.Duration) {
	if m == nil || m.boundarySnapshot == nil {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				boundary, version := m.snapshot()
				m.recordBoundarySnapshot(boundary, version)
				if version == m.targetVersion {
					return
				}
			}
		}
	}()
}

// Release resources held by the metrics collector. Safe on nil receivers
// and on instances constructed without a context (e.g.
// newLocalMigrationMetrics, which has no boundary-snapshot goroutine to
// stop and therefore no cancel func to call).
func (m *MigrationMetrics) Close() {
	if m == nil {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

// recordProgressLocked records numeric values only. The caller holds mu so
// a ticker cannot overwrite a newer progress value with an older snapshot.
func (m *MigrationMetrics) recordProgressLocked() {
	ctx := context.Background()
	if m.version != nil {
		m.version.Record(ctx, int64(m.currentVersion)) //nolint:gosec // migration versions are bounded
	}
	status := m.currentBoundary.Status()
	if m.currentVersion == m.targetVersion {
		status = MigrationComplete
	}
	if m.boundarySnapshot != nil {
		m.boundarySnapshot.Record(ctx, int64(status))
	}
	if m.keysProgress != nil {
		m.keysProgress.Record(ctx, m.runStats.keysMigrated)
	}
	if m.bytesProgress != nil {
		m.bytesProgress.Record(ctx, m.runStats.keyBytesMigrated+m.runStats.valueBytesMigrated)
	}
}

// recordBoundarySnapshot keeps the exact cursor in structured logs rather than
// in exporter labels. These are observations of applied state, not commit proofs.
func (m *MigrationMetrics) recordBoundarySnapshot(boundary MigrationBoundary, version uint64) {
	m.mu.Lock()
	m.recordProgressLocked()
	m.mu.Unlock()
	logger.Info("migration boundary snapshot",
		"version", version,
		"targetVersion", m.targetVersion,
		"status", boundary.Status().String(),
		"module", boundary.ModuleName(),
		"boundary_hex", hex.EncodeToString(boundary.Serialize()))
}
