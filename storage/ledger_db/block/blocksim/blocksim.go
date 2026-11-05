package blocksim

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/rand"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/block"
	memblockdb "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/block/mem_block_db"
	"golang.org/x/time/rate"
)

// The benchmark runner for the blocksim benchmark.
type BlockSim struct {
	ctx    context.Context
	cancel context.CancelFunc

	config *BlocksimConfig

	db        block.BlockDB
	generator *BlockGenerator
	metrics   *BlocksimMetrics

	// Console reporting state.
	consoleUpdatePeriod         time.Duration
	lastConsoleUpdateTime       time.Time
	lastConsoleUpdateBlockCount int64
	startTimestamp              time.Time
	totalBlocksWritten          int64
	totalTransactionsWritten    int64
	totalBytesWritten           int64
	highestBlockHeight          uint64

	// Closed after teardown publishes the first workload or teardown error.
	closeChan chan struct{}
	err       error

	// Suspend/resume toggle channel.
	suspendChan chan bool

	// Enforces a maximum block write rate (if enabled).
	rateLimiter *rate.Limiter
}

// Creates a new blocksim benchmark runner.
func NewBlockSim(
	ctx context.Context,
	config *BlocksimConfig,
	metrics *BlocksimMetrics,
) (*BlockSim, error) {

	var err error
	config.DataDir, err = utils.ResolveAndCreateDir(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve data directory: %w", err)
	}
	config.LogDir, err = utils.ResolveAndCreateDir(config.LogDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve log directory: %w", err)
	}

	if config.CleanDataOnStart {
		fmt.Printf("CleanDataOnStart is enabled, removing contents of: %s\n", config.DataDir)
		if err := removeContents(config.DataDir); err != nil {
			return nil, fmt.Errorf("failed to clean data directory: %w", err)
		}
	}
	if config.CleanLogsOnStart {
		fmt.Printf("CleanLogsOnStart is enabled, removing contents of: %s\n", config.LogDir)
		if err := removeContents(config.LogDir); err != nil {
			return nil, fmt.Errorf("failed to clean log directory: %w", err)
		}
	}

	fmt.Printf("Running blocksim benchmark from data directory: %s\n", config.DataDir)
	fmt.Printf("Logs are being routed to: %s\n", config.LogDir)

	db, err := openBlockDB(config.Backend, config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	fmt.Printf("Initializing random number generator.\n")
	rng := rand.NewCannedRandom(int(config.CannedRandomSize), config.Seed) //nolint:gosec

	ctx, cancel := context.WithCancel(ctx)

	generator := NewBlockGenerator(ctx, config, rng, 1)

	consoleUpdatePeriod := time.Duration(config.ConsoleUpdateIntervalSeconds * float64(time.Second))

	var rateLimiter *rate.Limiter
	if config.MaxBlocksPerSecond > 0 {
		rateLimiter = rate.NewLimiter(rate.Limit(config.MaxBlocksPerSecond), 1)
	}

	start := time.Now()

	b := &BlockSim{
		ctx:                   ctx,
		cancel:                cancel,
		config:                config,
		db:                    db,
		generator:             generator,
		metrics:               metrics,
		consoleUpdatePeriod:   consoleUpdatePeriod,
		lastConsoleUpdateTime: start,
		startTimestamp:        start,
		closeChan:             make(chan struct{}),
		suspendChan:           make(chan bool, 1),
		rateLimiter:           rateLimiter,
	}

	metrics.StartBlockDBPolling(ctx, db, config.BackgroundMetricsScrapeInterval)

	go b.run()
	return b, nil
}

// The main loop of the benchmark.
func (b *BlockSim) run() {
	defer b.teardown()

	var timeoutChan <-chan time.Time
	if b.config.MaxRuntimeSeconds > 0 {
		timeoutChan = time.After(time.Duration(b.config.MaxRuntimeSeconds) * time.Second)
	}

	for {
		b.metrics.SetMainThreadPhase("get_block")

		select {
		case <-b.ctx.Done():
			b.generateConsoleReport(true)
			fmt.Printf("\nBenchmark halted.\n")
			return
		case isSuspended := <-b.suspendChan:
			if isSuspended {
				b.suspend()
				if b.err != nil {
					return
				}
			}
		case <-timeoutChan:
			fmt.Printf("\nBenchmark timed out after %s.\n",
				utils.FormatDuration(time.Since(b.startTimestamp), 1))
			b.cancel()
			return
		case blk := <-b.generator.blocksChan:
			b.maybeThrottle()
			b.handleNextBlock(blk)
			if b.err != nil {
				return
			}
		}

		b.generateConsoleReport(false)
	}
}

func (b *BlockSim) maybeThrottle() {
	if b.rateLimiter == nil {
		return
	}
	b.metrics.SetMainThreadPhase("throttling")
	if err := b.rateLimiter.Wait(b.ctx); err != nil {
		return
	}
}

func (b *BlockSim) handleNextBlock(blk *block.BinaryBlock) {
	b.metrics.SetMainThreadPhase("write_block")
	if err := b.db.WriteBlock(b.ctx, blk); err != nil {
		b.recordError(fmt.Errorf("failed to write block %d: %w", blk.Height, err))
		b.cancel()
		return
	}

	txCount := int64(len(blk.Transactions))
	blockBytes := int64(len(blk.Hash) + len(blk.BlockData))
	for _, tx := range blk.Transactions {
		blockBytes += int64(len(tx.Hash) + len(tx.Transaction))
	}
	b.totalBlocksWritten++
	b.totalTransactionsWritten += txCount
	b.totalBytesWritten += blockBytes
	b.highestBlockHeight = blk.Height
	b.metrics.ReportBlockWritten(txCount, blockBytes)

	// Periodic flush.
	if b.config.FlushIntervalBlocks > 0 && b.totalBlocksWritten%int64(b.config.FlushIntervalBlocks) == 0 { //nolint:gosec
		b.metrics.SetMainThreadPhase("flush")
		if err := b.db.Flush(b.ctx); err != nil {
			b.recordError(fmt.Errorf("failed to flush: %w", err))
			b.cancel()
			return
		}
		b.metrics.ReportFlush()
	}

	// Periodic prune.
	if blk.Height > b.config.UnprunedBlocks && blk.Height%b.config.PruneIntervalBlocks == 0 {
		b.metrics.SetMainThreadPhase("prune")
		lowestToKeep := blk.Height - b.config.UnprunedBlocks
		if err := b.db.Prune(b.ctx, lowestToKeep); err != nil {
			b.recordError(fmt.Errorf("failed to prune: %w", err))
			b.cancel()
			return
		}
		b.metrics.ReportPrune()
	}
}

func (b *BlockSim) suspend() {
	// Flush before suspending so state is durable.
	if err := b.db.Flush(b.ctx); err != nil {
		b.recordError(fmt.Errorf("failed to flush on suspend: %w", err))
		b.cancel()
		return
	}

	fmt.Printf("Benchmark suspended.\n")
	b.metrics.SetMainThreadPhase("suspended")

	for {
		select {
		case <-b.ctx.Done():
			return
		case suspended := <-b.suspendChan:
			if suspended {
				break
			}
			// Reset console metrics on resume.
			b.totalBlocksWritten = 0
			b.totalBytesWritten = 0
			b.totalTransactionsWritten = 0
			b.startTimestamp = time.Now()
			fmt.Printf("Benchmark resumed.\n")
			return
		}
	}
}

func (b *BlockSim) teardown() {
	defer close(b.closeChan)
	b.cancel()
	ctx := context.WithoutCancel(b.ctx)
	fmt.Printf("Flushing and closing database.\n")
	if err := b.db.Flush(ctx); err != nil {
		b.recordError(fmt.Errorf("failed to flush during teardown: %w", err))
	}
	if err := b.db.Close(ctx); err != nil {
		b.recordError(fmt.Errorf("failed to close database: %w", err))
	}

	if b.config.CleanDataOnExit {
		fmt.Printf("CleanDataOnExit is enabled, removing contents of: %s\n", b.config.DataDir)
		if err := removeContents(b.config.DataDir); err != nil {
			b.recordError(fmt.Errorf("failed to clean data directory on exit: %w", err))
		}
	}
	if b.config.CleanLogsOnExit {
		fmt.Printf("CleanLogsOnExit is enabled, removing contents of: %s\n", b.config.LogDir)
		if err := removeContents(b.config.LogDir); err != nil {
			b.recordError(fmt.Errorf("failed to clean log directory on exit: %w", err))
		}
	}
}

func (b *BlockSim) recordError(err error) {
	if b.err == nil {
		b.err = err
	}
	fmt.Printf("%v\n", err)
}

func (b *BlockSim) generateConsoleReport(force bool) {
	now := time.Now()
	timeSinceLastUpdate := now.Sub(b.lastConsoleUpdateTime)
	blocksSinceLastUpdate := b.totalBlocksWritten - b.lastConsoleUpdateBlockCount

	if !force &&
		timeSinceLastUpdate < b.consoleUpdatePeriod &&
		blocksSinceLastUpdate < int64(b.config.ConsoleUpdateIntervalBlocks) { //nolint:gosec
		return
	}

	b.lastConsoleUpdateTime = now
	b.lastConsoleUpdateBlockCount = b.totalBlocksWritten

	elapsed := now.Sub(b.startTimestamp)
	bytesPerSecond := float64(b.totalBytesWritten) / elapsed.Seconds()

	fmt.Printf("%s blocks in %s | %s written | %s/sec      \r",
		utils.Int64Commas(b.totalBlocksWritten),
		utils.FormatDuration(elapsed, 1),
		utils.FormatBytes(b.totalBytesWritten),
		utils.FormatBytes(int64(bytesPerSecond)))
}

// Blocks until the benchmark has halted.
func (b *BlockSim) BlockUntilHalted() error {
	<-b.closeChan
	return b.err
}

// Close shuts down the benchmark and releases resources.
func (b *BlockSim) Close() error {
	b.cancel()
	if err := b.BlockUntilHalted(); err != nil {
		return err
	}
	fmt.Printf("Benchmark terminated successfully.\n")
	return nil
}

// CLIExitCode reports the benchmark result and returns the process exit status.
func CLIExitCode(err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// Suspend the benchmark. Call Resume() to continue.
func (b *BlockSim) Suspend() {
	select {
	case <-b.ctx.Done():
	case b.suspendChan <- true:
	}
}

// Resume the benchmark after a Suspend().
func (b *BlockSim) Resume() {
	select {
	case <-b.ctx.Done():
	case b.suspendChan <- false:
	}
}

// openBlockDB creates a BlockDB for the given backend name.
func openBlockDB(backend string, dataDir string) (block.BlockDB, error) {
	switch backend {
	case "mem":
		return memblockdb.NewMemBlockDB(), nil
	default:
		return nil, fmt.Errorf("unknown BlockDB backend: %q", backend)
	}
}

// removeContents deletes all entries inside dir without removing dir itself.
func removeContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(fmt.Sprintf("%s/%s", dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
