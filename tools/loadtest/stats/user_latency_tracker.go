package stats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type trackedSend struct {
	Endpoint string
	SentAt   time.Time
	Done     bool
}

type UserLatencyEndpoint struct {
	Committed        uint64        `json:"committed"`
	FailedExecutions uint64        `json:"failed_executions"`
	MissingReceipts  uint64        `json:"missing_receipts"`
	SampleCount      int           `json:"sample_count"`
	LatencyP50       time.Duration `json:"latency_p50"`
	LatencyP99       time.Duration `json:"latency_p99"`
}

type UserLatencyStats struct {
	Scope           string                         `json:"scope"`
	SampleCount     int                            `json:"sample_count"`
	MissingReceipts uint64                         `json:"missing_receipts"`
	BlocksConsumed  int                            `json:"blocks_consumed"`
	LastError       string                         `json:"last_error,omitempty"`
	Endpoints       map[string]UserLatencyEndpoint `json:"endpoints"`
}

// UserLatencyTracker correlates registered sends with committed receipts. The
// payment value and traffic from other senders never supply timing evidence.
type UserLatencyTracker struct {
	interval  time.Duration
	startedAt time.Time
	pollMu    sync.Mutex
	mu        sync.Mutex
	sends     map[common.Hash]trackedSend
	blocks    map[common.Hash]*types.Block
	latencies map[string][]time.Duration
	outcomes  map[string]UserLatencyEndpoint
	lastError string
}

func NewUserLatencyTracker(interval time.Duration) *UserLatencyTracker {
	return &UserLatencyTracker{interval: interval, startedAt: time.Now(), sends: make(map[common.Hash]trackedSend), blocks: make(map[common.Hash]*types.Block), latencies: make(map[string][]time.Duration), outcomes: make(map[string]UserLatencyEndpoint)}
}

// TrackTransaction must be called with the signed hash and retained send time
// before dispatch, including for zero-value and contract-call transactions.
func (ult *UserLatencyTracker) TrackTransaction(hash common.Hash, endpoint string, sentAt time.Time) error {
	ult.mu.Lock()
	defer ult.mu.Unlock()
	if hash == (common.Hash{}) || endpoint == "" || sentAt.Before(ult.startedAt) || sentAt.After(time.Now()) {
		return fmt.Errorf("invalid transaction send evidence")
	}
	if previous, ok := ult.sends[hash]; ok {
		if previous.Endpoint != endpoint || !previous.SentAt.Equal(sentAt) {
			return fmt.Errorf("conflicting transaction send evidence for %s", hash)
		}
		return nil
	}
	ult.sends[hash] = trackedSend{Endpoint: endpoint, SentAt: sentAt}
	return nil
}

func (ult *UserLatencyTracker) Run(ctx context.Context, endpoint string) error {
	if ult.interval <= 0 {
		return fmt.Errorf("user latency interval must be positive")
	}
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.Close()
	ticker := time.NewTicker(ult.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Errors remain visible in the report while the next poll can recover.
			if err := ult.Poll(ctx, client); err != nil {
				log.Printf("user latency receipt observation failed: %v", err)
			}
			observed := ult.GetStats()
			log.Printf("user latency scope=%s samples=%d missing_receipts=%d insufficient_samples=%t endpoints=%v", observed.Scope, observed.SampleCount, observed.MissingReceipts, observed.SampleCount == 0, observed.Endpoints)
		}
	}
}

// Poll follows each registered hash to its receipt and block. Blocks are fetched
// once even when several sends share a block or receipts arrive on later polls.
func (ult *UserLatencyTracker) Poll(ctx context.Context, client *ethclient.Client) (err error) {
	ult.pollMu.Lock()
	defer ult.pollMu.Unlock()
	defer func() {
		ult.mu.Lock()
		defer ult.mu.Unlock()
		if err != nil {
			ult.lastError = err.Error()
		} else {
			ult.lastError = ""
		}
	}()
	ult.mu.Lock()
	pending := make(map[common.Hash]trackedSend)
	for hash, send := range ult.sends {
		if !send.Done {
			pending[hash] = send
		}
	}
	ult.mu.Unlock()
	for hash, send := range pending {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		receipt, requestErr := client.TransactionReceipt(requestCtx, hash)
		cancel()
		if errors.Is(requestErr, ethereum.NotFound) {
			continue
		}
		if requestErr != nil {
			return fmt.Errorf("receipt %s: %w", hash, requestErr)
		}
		if receipt.TxHash != hash || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockHash == (common.Hash{}) {
			return fmt.Errorf("invalid committed receipt for %s", hash)
		}
		block := ult.blocks[receipt.BlockHash]
		if block == nil {
			requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			block, requestErr = client.BlockByHash(requestCtx, receipt.BlockHash)
			cancel()
			if requestErr != nil {
				return fmt.Errorf("receipt block %s: %w", receipt.BlockHash, requestErr)
			}
			if block.Hash() != receipt.BlockHash || block.NumberU64() != receipt.BlockNumber.Uint64() {
				return fmt.Errorf("receipt block mismatch for %s", hash)
			}
			ult.mu.Lock()
			ult.blocks[receipt.BlockHash] = block
			ult.mu.Unlock()
		}
		txs := block.Transactions()
		if block.NumberU64() != receipt.BlockNumber.Uint64() || uint64(receipt.TransactionIndex) >= uint64(len(txs)) || txs[receipt.TransactionIndex].Hash() != hash {
			return fmt.Errorf("receipt transaction mismatch for %s", hash)
		}
		if receipt.Status != types.ReceiptStatusSuccessful && receipt.Status != types.ReceiptStatusFailed {
			return fmt.Errorf("invalid receipt status for %s", hash)
		}
		latency := time.Since(send.SentAt)
		if latency < 0 {
			return fmt.Errorf("negative send latency for %s", hash)
		}
		ult.mu.Lock()
		current := ult.sends[hash]
		current.Done = true
		ult.sends[hash] = current
		outcome := ult.outcomes[send.Endpoint]
		outcome.Committed++
		if receipt.Status == types.ReceiptStatusFailed {
			outcome.FailedExecutions++
		}
		ult.outcomes[send.Endpoint] = outcome
		ult.latencies[send.Endpoint] = append(ult.latencies[send.Endpoint], latency)
		ult.mu.Unlock()
	}
	return nil
}

func (ult *UserLatencyTracker) GetStats() UserLatencyStats {
	ult.mu.Lock()
	defer ult.mu.Unlock()
	result := UserLatencyStats{Scope: "full_run_send_to_committed_receipt_observation", BlocksConsumed: len(ult.blocks), LastError: ult.lastError, Endpoints: make(map[string]UserLatencyEndpoint)}
	for endpoint, outcome := range ult.outcomes {
		result.Endpoints[endpoint] = outcome
	}
	for _, send := range ult.sends {
		if !send.Done {
			outcome := result.Endpoints[send.Endpoint]
			outcome.MissingReceipts++
			result.Endpoints[send.Endpoint] = outcome
			result.MissingReceipts++
		}
	}
	for endpoint, samples := range ult.latencies {
		sorted := append([]time.Duration(nil), samples...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		outcome := result.Endpoints[endpoint]
		outcome.SampleCount = len(sorted)
		outcome.LatencyP50 = calculatePercentile(sorted, 50)
		outcome.LatencyP99 = calculatePercentile(sorted, 99)
		result.SampleCount += len(sorted)
		result.Endpoints[endpoint] = outcome
	}
	return result
}
