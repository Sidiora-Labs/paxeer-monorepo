package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/paxeer-network/pax-load/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type blockCollectorStats struct {
	// Cumulative data (for final stats)
	allBlockTimes []time.Duration // All block times
	allGasUsed    []uint64        // All gas used values
	maxBlockNum   uint64          // Highest block number seen
	lastBlockTime time.Time       // Timestamp of last block

	headers       []*types.Header
	hashes        map[uint64]common.Hash
	deliveryLags  []time.Duration
	incomplete    bool
	recoveryError string

	// Window-based data (for periodic reporting)
	windowBlockTimes []time.Duration // Block times in current window
	windowGasUsed    []uint64        // Gas used in current window
	windowStart      time.Time       // Start of current window
}

// BlockCollector subscribes to new blocks and tracks block metrics
type BlockCollector struct {
	paxChainID     string
	stats          utils.Mutex[*blockCollectorStats]
	checkpointPath string
}

type BlockStatsProvider interface {
	GetBlockStats() BlockStats
	GetWindowBlockStats() BlockStats
	GetWindowBlockTimePercentile(percentile int) time.Duration
	ResetWindowStats()
}

// NewBlockCollector creates a new block data collector
func NewBlockCollector(paxChainID string) *BlockCollector {
	return &BlockCollector{
		paxChainID: paxChainID,
		stats: utils.NewMutex(&blockCollectorStats{
			hashes:           make(map[uint64]common.Hash),
			allBlockTimes:    make([]time.Duration, 0),
			allGasUsed:       make([]uint64, 0),
			windowBlockTimes: make([]time.Duration, 0),
			windowGasUsed:    make([]uint64, 0),
		}),
	}
}

type blockCheckpoint struct {
	ChainID       string          `json:"chain_id"`
	Headers       []*types.Header `json:"headers"`
	DeliveryLags  []time.Duration `json:"delivery_lags"`
	Incomplete    bool            `json:"incomplete"`
	RecoveryError string          `json:"recovery_error,omitempty"`
}

// OpenBlockCollector resumes a run's committed observations from a local checkpoint.
func OpenBlockCollector(chainID, path string) (*BlockCollector, error) {
	bc := NewBlockCollector(chainID)
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		var saved blockCheckpoint
		if err := json.Unmarshal(data, &saved); err != nil {
			return nil, fmt.Errorf("decode block checkpoint: %w", err)
		}
		if saved.ChainID != chainID || len(saved.Headers) != len(saved.DeliveryLags) {
			return nil, fmt.Errorf("block checkpoint does not match run")
		}
		for _, header := range saved.Headers {
			if err := bc.processNewBlock(header); err != nil {
				return nil, fmt.Errorf("invalid block checkpoint: %w", err)
			}
		}
		for state := range bc.stats.Lock() {
			if len(state.headers) != len(saved.Headers) {
				return nil, fmt.Errorf("duplicate block in checkpoint")
			}
			state.deliveryLags = saved.DeliveryLags
			state.incomplete, state.recoveryError = saved.Incomplete, saved.RecoveryError
		}
	}
	bc.checkpointPath = path
	return bc, nil
}

func (bc *BlockCollector) saveCheckpoint(state *blockCollectorStats) error {
	if bc.checkpointPath == "" {
		return nil
	}
	data, err := json.Marshal(blockCheckpoint{bc.paxChainID, state.headers, state.deliveryLags, state.incomplete, state.recoveryError})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(bc.checkpointPath), ".block-checkpoint-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), bc.checkpointPath); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(bc.checkpointPath))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (bc *BlockCollector) fail(err error) error {
	for state := range bc.stats.Lock() {
		state.incomplete = true
		state.recoveryError = err.Error()
		if saveErr := bc.saveCheckpoint(state); saveErr != nil {
			return errors.Join(err, saveErr)
		}
	}
	return err
}

// Run reconnects after transient transport failures. Every session reconciles
// its checkpoint with the node before admitting subscription notifications.
func (bc *BlockCollector) Run(ctx context.Context, firstEndpoint string) error {
	if observed := bc.GetBlockStats(); observed.Incomplete {
		return fmt.Errorf("incomplete block observations: %s", observed.RecoveryError)
	}
	failures := 0
	for {
		before := bc.GetBlockStats().MaxBlockNumber
		err := bc.runSubscription(ctx, utils.GetWSEndpoint(firstEndpoint))
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if bc.GetBlockStats().Incomplete {
			return err
		}
		if bc.GetBlockStats().MaxBlockNumber > before {
			failures = 0
		}
		failures++
		if failures >= 5 {
			return bc.fail(fmt.Errorf("block subscription recovery exhausted: %w", err))
		}
		delay := 100 * time.Millisecond * time.Duration(1<<(failures-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (bc *BlockCollector) runSubscription(ctx context.Context, endpoint string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	client, err := ethclient.DialContext(dialCtx, endpoint)
	cancel()
	if err != nil {
		return err
	}
	defer client.Close()
	headers := make(chan *types.Header, 128)
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	sub, err := client.SubscribeNewHead(subCtx, headers)
	cancel()
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	latest, err := client.HeaderByNumber(requestCtx, nil)
	cancel()
	if err != nil {
		return err
	}
	if err := bc.backfill(ctx, client, latest); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-sub.Err():
			if !ok || err == nil {
				return fmt.Errorf("block subscription closed")
			}
			return err
		case header, ok := <-headers:
			if !ok {
				return fmt.Errorf("block header stream closed")
			}
			if err := bc.backfill(ctx, client, header); err != nil {
				return err
			}
		}
	}
}

func (bc *BlockCollector) backfill(ctx context.Context, client *ethclient.Client, target *types.Header) error {
	if target == nil || target.Number == nil || !target.Number.IsUint64() {
		return bc.fail(fmt.Errorf("invalid committed header"))
	}
	var last *types.Header
	var firstHeight uint64
	for state := range bc.stats.Lock() {
		if len(state.headers) != 0 {
			last = state.headers[len(state.headers)-1]
			firstHeight = state.headers[0].Number.Uint64()
		}
	}
	if last == nil {
		return bc.processNewBlock(target)
	}
	if target.Number.Uint64() < firstHeight {
		return nil
	}
	if target.Number.Uint64() <= last.Number.Uint64() {
		return bc.processNewBlock(target)
	}
	// Verify the last checkpoint against the actual node, including after restart.
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	anchor, err := client.HeaderByNumber(requestCtx, last.Number)
	cancel()
	if err != nil {
		return fmt.Errorf("recover committed height %d: %w", last.Number.Uint64(), err)
	}
	if anchor.Hash() != last.Hash() {
		return bc.fail(fmt.Errorf("committed hash changed at height %d", last.Number.Uint64()))
	}
	for height := last.Number.Uint64() + 1; ; height++ {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		header, err := client.HeaderByNumber(requestCtx, new(big.Int).SetUint64(height))
		cancel()
		if err != nil {
			return fmt.Errorf("backfill committed height %d: %w", height, err)
		}
		if header.Number == nil || header.Number.Uint64() != height {
			return bc.fail(fmt.Errorf("backfill returned wrong height for %d", height))
		}
		if height == target.Number.Uint64() && header.Hash() != target.Hash() {
			return bc.fail(fmt.Errorf("committed notification hash mismatch at %d", height))
		}
		if err := bc.processNewBlock(header); err != nil {
			return err
		}
		if height == target.Number.Uint64() {
			return nil
		}
	}
}

func (bc *BlockCollector) processNewBlock(header *types.Header) error {
	if header == nil || header.Number == nil || !header.Number.IsUint64() || header.Time > uint64(1<<63-1) {
		return bc.fail(fmt.Errorf("invalid committed header"))
	}
	for state := range bc.stats.Lock() {
		height, hash := header.Number.Uint64(), header.Hash()
		if known, ok := state.hashes[height]; ok {
			if known == hash {
				return nil
			}
			state.incomplete, state.recoveryError = true, fmt.Sprintf("conflicting committed hash at height %d", height)
			if err := bc.saveCheckpoint(state); err != nil {
				return err
			}
			return errors.New(state.recoveryError)
		}
		if len(state.headers) != 0 {
			last := state.headers[len(state.headers)-1]
			if height != state.maxBlockNum+1 || header.ParentHash != last.Hash() || header.Time < last.Time {
				state.incomplete, state.recoveryError = true, fmt.Sprintf("non-contiguous committed header at height %d after %d", height, state.maxBlockNum)
				if err := bc.saveCheckpoint(state); err != nil {
					return err
				}
				return errors.New(state.recoveryError)
			}
			interval := time.Unix(int64(header.Time), 0).Sub(state.lastBlockTime)
			state.allBlockTimes = append(state.allBlockTimes, interval)
			state.windowBlockTimes = append(state.windowBlockTimes, interval)
			metrics.blockTime.Record(context.Background(), interval.Seconds(), metric.WithAttributes(attribute.String("chain_id", bc.paxChainID)))
		}
		state.headers = append(state.headers, types.CopyHeader(header))
		state.hashes[height] = hash
		state.maxBlockNum = height
		state.lastBlockTime = time.Unix(int64(header.Time), 0)
		state.allGasUsed = append(state.allGasUsed, header.GasUsed)
		state.windowGasUsed = append(state.windowGasUsed, header.GasUsed)
		state.deliveryLags = append(state.deliveryLags, time.Since(state.lastBlockTime))
		metrics.gasUsed.Record(context.Background(), int64(header.GasUsed), metric.WithAttributes(attribute.String("chain_id", bc.paxChainID)))
		metrics.blockNumber.Record(context.Background(), int64(height), metric.WithAttributes(attribute.String("chain_id", bc.paxChainID)))
		if err := bc.saveCheckpoint(state); err != nil {
			state.incomplete, state.recoveryError = true, fmt.Sprintf("persist block observations: %v", err)
			return errors.New(state.recoveryError)
		}
	}
	return nil
}

// GetBlockStats returns current block statistics
func (bc *BlockCollector) GetBlockStats() BlockStats {
	for bc := range bc.stats.Lock() {
		stats := BlockStats{
			MaxBlockNumber: bc.maxBlockNum,
			Scope:          "full_run_committed_headers",
			Incomplete:     bc.incomplete, RecoveryError: bc.recoveryError,
			GasSampleCount:      len(bc.allGasUsed),
			DeliverySampleCount: len(bc.deliveryLags),
			SampleCount:         len(bc.allBlockTimes),
		}

		if len(bc.deliveryLags) > 0 {
			lags := append([]time.Duration(nil), bc.deliveryLags...)
			sort.Slice(lags, func(i, j int) bool { return lags[i] < lags[j] })
			stats.P50DeliveryLag = calculatePercentile(lags, 50)
			stats.P99DeliveryLag = calculatePercentile(lags, 99)
		}

		// Calculate block time percentiles
		if len(bc.allBlockTimes) > 0 {
			sortedTimes := make([]time.Duration, len(bc.allBlockTimes))
			copy(sortedTimes, bc.allBlockTimes)
			sort.Slice(sortedTimes, func(i, j int) bool {
				return sortedTimes[i] < sortedTimes[j]
			})

			stats.P50BlockTime = calculatePercentile(sortedTimes, 50)
			stats.P99BlockTime = calculatePercentile(sortedTimes, 99)
			stats.MaxBlockTime = sortedTimes[len(sortedTimes)-1]
		}

		// Calculate gas used percentiles
		if len(bc.allGasUsed) > 0 {
			sortedGas := make([]uint64, len(bc.allGasUsed))
			copy(sortedGas, bc.allGasUsed)
			sort.Slice(sortedGas, func(i, j int) bool {
				return sortedGas[i] < sortedGas[j]
			})

			stats.P50GasUsed = calculateGasPercentile(sortedGas, 50)
			stats.P99GasUsed = calculateGasPercentile(sortedGas, 99)
			stats.MaxGasUsed = sortedGas[len(sortedGas)-1]
		}

		return stats
	}
	panic("unreachable")
}

// GetWindowBlockStats returns current window-based block statistics
func (bc *BlockCollector) GetWindowBlockStats() BlockStats {
	for bc := range bc.stats.Lock() {
		stats := BlockStats{
			MaxBlockNumber: bc.maxBlockNum,
			Scope:          "reporting_window_committed_headers",
			Incomplete:     bc.incomplete, RecoveryError: bc.recoveryError,
			GasSampleCount: len(bc.windowGasUsed),
			SampleCount:    len(bc.windowBlockTimes),
		}

		// Calculate block time percentiles for current window
		if len(bc.windowBlockTimes) > 0 {
			sortedTimes := make([]time.Duration, len(bc.windowBlockTimes))
			copy(sortedTimes, bc.windowBlockTimes)
			sort.Slice(sortedTimes, func(i, j int) bool {
				return sortedTimes[i] < sortedTimes[j]
			})

			stats.P50BlockTime = calculatePercentile(sortedTimes, 50)
			stats.P99BlockTime = calculatePercentile(sortedTimes, 99)
			stats.MaxBlockTime = sortedTimes[len(sortedTimes)-1]
		}

		// Calculate gas used percentiles for current window
		if len(bc.windowGasUsed) > 0 {
			sortedGas := make([]uint64, len(bc.windowGasUsed))
			copy(sortedGas, bc.windowGasUsed)
			sort.Slice(sortedGas, func(i, j int) bool {
				return sortedGas[i] < sortedGas[j]
			})

			stats.P50GasUsed = calculateGasPercentile(sortedGas, 50)
			stats.P99GasUsed = calculateGasPercentile(sortedGas, 99)
			stats.MaxGasUsed = sortedGas[len(sortedGas)-1]
		}

		return stats
	}
	panic("unreachable")
}

func (bc *BlockCollector) GetWindowBlockTimePercentile(percentile int) time.Duration {
	for bc := range bc.stats.Lock() {
		sortedTimes := make([]time.Duration, len(bc.windowBlockTimes))
		copy(sortedTimes, bc.windowBlockTimes)
		sort.Slice(sortedTimes, func(i, j int) bool {
			return sortedTimes[i] < sortedTimes[j]
		})
		return calculatePercentile(sortedTimes, percentile)
	}
	panic("unreachable")
}

// ResetWindowStats resets the window-based statistics for the next reporting period
func (bc *BlockCollector) ResetWindowStats() {
	for bc := range bc.stats.Lock() {
		bc.windowBlockTimes = make([]time.Duration, 0)
		bc.windowGasUsed = make([]uint64, 0)
		bc.windowStart = time.Now()
	}
}

// calculateGasPercentile calculates the given percentile from sorted gas values
func calculateGasPercentile(sorted []uint64, percentile int) uint64 {
	if len(sorted) == 0 {
		return 0
	}

	index := (percentile * (len(sorted) - 1)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// BlockStats represents block-related statistics
type BlockStats struct {
	Scope               string        `json:"scope"`
	Incomplete          bool          `json:"incomplete"`
	RecoveryError       string        `json:"recovery_error,omitempty"`
	GasSampleCount      int           `json:"gas_sample_count"`
	DeliverySampleCount int           `json:"delivery_sample_count"`
	P50DeliveryLag      time.Duration `json:"p50_delivery_lag"`
	P99DeliveryLag      time.Duration `json:"p99_delivery_lag"`
	MaxBlockNumber      uint64        `json:"max_block_number"`
	P50BlockTime        time.Duration `json:"p50_block_time"`
	P99BlockTime        time.Duration `json:"p99_block_time"`
	MaxBlockTime        time.Duration `json:"max_block_time"`
	P50GasUsed          uint64        `json:"p50_gas_used"`
	P99GasUsed          uint64        `json:"p99_gas_used"`
	MaxGasUsed          uint64        `json:"max_gas_used"`
	SampleCount         int           `json:"sample_count"`
}

// FormatBlockStats returns a formatted string representation of block statistics
func (bs BlockStats) FormatBlockStats() string {
	if bs.Incomplete {
		return "block stats: incomplete: " + bs.RecoveryError
	}
	if bs.SampleCount == 0 {
		return "block stats: no data available"
	}

	return fmt.Sprintf("block scope="+bs.Scope+" height=%d, committed_intervals(p50=%v p99=%v max=%v), gas(p50=%d p99=%d max=%d) samples=%d",
		bs.MaxBlockNumber,
		bs.P50BlockTime.Round(time.Millisecond),
		bs.P99BlockTime.Round(time.Millisecond),
		bs.MaxBlockTime.Round(time.Millisecond),
		bs.P50GasUsed,
		bs.P99GasUsed,
		bs.MaxGasUsed,
		bs.SampleCount,
	)
}
