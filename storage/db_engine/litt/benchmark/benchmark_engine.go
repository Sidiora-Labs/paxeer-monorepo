package benchmark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/unit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/benchmark/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"golang.org/x/time/rate"
)

// BenchmarkEngine is a tool for benchmarking LittDB performance.
type BenchmarkEngine struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger

	// The configuration for the benchmark.
	config *config.BenchmarkConfig

	// The database to be benchmarked.
	db litt.DB

	// The table in the database where data is stored.
	table litt.Table

	// Keeps track of data to read and write.
	dataTracker *DataTracker

	// The maximum write throughput in bytes per second for each worker thread.
	writeBytesPerSecondPerThread uint64

	// The maximum read throughput in bytes per second for each worker thread.
	readBytesPerSecondPerThread uint64

	// The burst size for write rate limiting.
	writeBurstSize uint64

	// The burst size for read rate limiting.
	readBurstSize uint64

	// Records benchmark metrics.
	metrics *metrics

	// errorMonitor is used to handle fatal errors in the benchmark engine.
	errorMonitor *util.ErrorMonitor
}

// NewBenchmarkEngine creates a new BenchmarkEngine with the given configuration.
func NewBenchmarkEngine(configPath string) (*BenchmarkEngine, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
	}

	runtimeConfig := litt.DefaultRuntimeConfig()

	if len(cfg.LittConfig.Paths) > litt.MaxShardingFactor {
		return nil, fmt.Errorf("too many paths configured (%d), max is %d",
			len(cfg.LittConfig.Paths), litt.MaxShardingFactor)
	}

	db, err := littbuilder.NewDB(cfg.LittConfig, runtimeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create db: %w", err)
	}

	tableConfig := litt.DefaultTableConfig("benchmark")
	tableConfig.ShardingFactor = uint8(len(cfg.LittConfig.Paths)) //nolint:gosec // bounded above by MaxShardingFactor
	tableConfig.TTL = time.Duration(cfg.TTLHours * float64(time.Hour))

	table, err := db.BuildTable(tableConfig)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to create table: %w", err), db.Close())
	}

	ctx, cancel := context.WithCancel(context.Background())

	errorMonitor := util.NewErrorMonitor(context.Background(), runtimeConfig.Logger, nil)

	dataTracker, err := NewDataTracker(context.Background(), cfg, errorMonitor)
	if err != nil {
		cancel()
		return nil, errors.Join(fmt.Errorf("failed to create data tracker: %w", err), db.Close())
	}

	writeBytesPerSecond := uint64(cfg.MaximumWriteThroughputMB * float64(unit.MB))
	writeBytesPerSecondPerThread := writeBytesPerSecond / uint64(cfg.WriterParallelism) //nolint:gosec // parallelism positive

	// If we set the write burst size smaller than an individual value, then the rate limiter will never
	// permit any writes. Ideally, we'd just set the burst size to 0 since we don't want bursty/volatile writes,
	// but since we are using the rate.Limiter utility, we are required to set a burst size, and a burst size
	// smaller than an individual value will cause the rate limiter to never permit writes.
	writeBurstSize := uint64(cfg.ValueSizeMB * float64(unit.MB))

	readBytesPerSecond := uint64(cfg.MaximumReadThroughputMB * float64(unit.MB))
	readBytesPerSecondPerThread := readBytesPerSecond / uint64(cfg.ReaderParallelism) //nolint:gosec // parallelism positive

	// If we set the read burst size smaller than an individual value we need to read, then the rate limiter will
	// never permit us to read that value.
	readBurstSize := dataTracker.LargestReadableValueSize()

	return &BenchmarkEngine{
		ctx:                          ctx,
		cancel:                       cancel,
		logger:                       runtimeConfig.Logger,
		config:                       cfg,
		db:                           db,
		table:                        table,
		dataTracker:                  dataTracker,
		writeBytesPerSecondPerThread: writeBytesPerSecondPerThread,
		readBytesPerSecondPerThread:  readBytesPerSecondPerThread,
		writeBurstSize:               writeBurstSize,
		readBurstSize:                readBurstSize,
		metrics:                      newMetrics(ctx, runtimeConfig.Logger, cfg),
		errorMonitor:                 errorMonitor,
	}, nil
}

// Logger returns the logger used by the benchmark engine.
func (b *BenchmarkEngine) Logger() *slog.Logger {
	return b.logger
}

// Run executes the benchmark. This method blocks forever, or until the benchmark is stopped via control-C or
// encounters an error.
func (b *BenchmarkEngine) Run() error {
	if b.config.TimeLimitSeconds > 0 {
		timer := time.AfterFunc(time.Duration(b.config.TimeLimitSeconds*float64(time.Second)), b.cancel)
		defer timer.Stop()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	var workers sync.WaitGroup
	startWorker := func(worker func()) bool {
		delay := time.Duration(b.config.StartupSleepFactorSeconds * float64(time.Second) * 2 * rand.Float64())
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-b.ctx.Done():
			return false
		case <-b.errorMonitor.ImmediateShutdownRequired():
			return false
		case <-sigChan:
			b.cancel()
			return false
		case <-timer.C:
		}
		if b.ctx.Err() != nil {
			return false
		}
		workers.Add(1)
		go func() { defer workers.Done(); worker() }()
		return true
	}
	for i := 0; i < b.config.WriterParallelism; i++ {
		if !startWorker(b.writer) {
			break
		}
	}
	for i := 0; i < b.config.ReaderParallelism; i++ {
		if !startWorker(b.reader) {
			break
		}
	}

	select {
	case <-b.ctx.Done():
	case <-b.errorMonitor.ImmediateShutdownRequired():
	case <-sigChan:
	}
	b.cancel()
	workers.Wait()
	// Every ReportWrite follows a completed Flush. Close the DB before draining
	// those reports so a successful return has both data and metadata durable.
	closeErr := b.db.Close()
	b.dataTracker.Close()
	_, workloadErr := b.errorMonitor.IsOk()
	if closeErr != nil {
		closeErr = fmt.Errorf("failed to close database: %w", closeErr)
	}
	return errors.Join(workloadErr, closeErr)
}

func (b *BenchmarkEngine) waitForQuota(throttle *rate.Limiter, size int) bool {
	reservation := throttle.ReserveN(time.Now(), size)
	if !reservation.OK() {
		b.errorMonitor.Panic(fmt.Errorf("failed to reserve quota for %d bytes", size))
		return false
	}
	timer := time.NewTimer(reservation.Delay())
	defer timer.Stop()
	select {
	case <-b.ctx.Done():
		reservation.Cancel()
		return false
	case <-b.errorMonitor.ImmediateShutdownRequired():
		reservation.Cancel()
		return false
	case <-timer.C:
		return b.ctx.Err() == nil
	}
}

// writer flushes and reports any accepted partial batch before leaving.
func (b *BenchmarkEngine) writer() {
	maxBatchSize := uint64(b.config.BatchSizeMB * float64(unit.MB))
	throttle := rate.NewLimiter(rate.Limit(b.writeBytesPerSecondPerThread), int(b.writeBurstSize))
	for b.ctx.Err() == nil {
		batchSize := uint64(0)
		writtenIndices := make([]uint64, 0)
		for batchSize < maxBatchSize && b.ctx.Err() == nil {
			writeInfo := b.dataTracker.getWriteInfo(b.ctx)
			if writeInfo == nil || !b.waitForQuota(throttle, len(writeInfo.Value)) {
				break
			}
			start := time.Now()
			if err := b.table.Put(writeInfo.Key, writeInfo.Value); err != nil {
				b.errorMonitor.Panic(fmt.Errorf("failed to write data: %w", err))
				return
			}
			b.metrics.reportWrite(time.Since(start), uint64(len(writeInfo.Value)))
			writtenIndices = append(writtenIndices, writeInfo.KeyIndex)
			batchSize += uint64(len(writeInfo.Value))
		}
		if len(writtenIndices) == 0 {
			return
		}
		start := time.Now()
		if err := b.table.Flush(); err != nil {
			b.errorMonitor.Panic(fmt.Errorf("failed to flush data: %w", err))
			return
		}
		b.metrics.reportFlush(time.Since(start))
		for _, index := range writtenIndices {
			b.dataTracker.ReportWrite(index)
		}
	}
}

// verifyValue checks if the actual value read from the database matches the expected value.
func (b *BenchmarkEngine) verifyValue(expected *ReadInfo, actual []byte) error {
	if len(actual) != len(expected.Value) {
		return fmt.Errorf("read value size %d does not match expected size %d for key %s",
			len(actual), len(expected.Value), expected.Key)
	}
	for i := range actual {
		if actual[i] != expected.Value[i] {
			return fmt.Errorf("read value does not match expected value for key %s", expected.Key)
		}
	}
	return nil
}

// reader runs on a goroutine and reads data from the database.
func (b *BenchmarkEngine) reader() {
	throttle := rate.NewLimiter(rate.Limit(b.readBytesPerSecondPerThread), int(b.readBurstSize))
	for b.ctx.Err() == nil {
		readInfo := b.dataTracker.getReadInfo(b.ctx)
		if readInfo == nil || !b.waitForQuota(throttle, len(readInfo.Value)) {
			return
		}
		if !time.Now().Before(readInfo.ValidUntil) {
			b.metrics.expiredReadCount.Add(1)
			continue
		}
		start := time.Now()
		value, exists, err := b.table.Get(readInfo.Key)
		if err != nil {
			b.errorMonitor.Panic(fmt.Errorf("failed to read data: %w", err))
			return
		}
		// A queued request can cross its safety deadline while waiting or reading.
		if !time.Now().Before(readInfo.ValidUntil) {
			b.metrics.expiredReadCount.Add(1)
			continue
		}
		if !exists {
			b.metrics.missingReadCount.Add(1)
			b.errorMonitor.Panic(fmt.Errorf("key %x not found in database", readInfo.Key))
			return
		}
		if err := b.verifyValue(readInfo, value); err != nil {
			b.metrics.invalidReadCount.Add(1)
			b.errorMonitor.Panic(err)
			return
		}
		b.metrics.reportRead(time.Since(start), uint64(len(value)))
	}
}
