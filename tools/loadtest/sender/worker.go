package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/time/rate"

	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/types"
	"github.com/paxeer-network/pax-load/utils"
	"github.com/paxeer-network/pax-load/utils/service"
)

// Worker handles sending transactions to a specific endpoint
type ReceiptStats struct {
	Accepted   uint64
	Succeeded  uint64
	Failed     uint64
	TimedOut   uint64
	Unresolved uint64
	Pending    uint64
}

var ErrReceiptFailed = errors.New("transaction execution failed")

type Worker struct {
	runID          uint64
	receiptMu      sync.Mutex
	receipts       ReceiptStats
	receiptTimeout time.Duration
	id             int
	paxChainID     string
	endpoint       string
	txChan         chan *types.LoadTx
	sentTxs        chan *types.LoadTx
	dryRun         bool
	debug          bool
	collector      *stats.Collector
	logger         *stats.Logger
	workers        int
	trackReceipts  bool
	limiter        *rate.Limiter // Shared rate limiter for transaction sending
}

func newHttpClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
		},
	}
}

// NewWorker creates a new worker for a specific endpoint
func NewWorker(id int, paxChainID string, endpoint string, bufferSize int, workers int, limiter *rate.Limiter) *Worker {
	w := &Worker{
		id:             id,
		runID:          workerRunID.Add(1),
		receiptTimeout: 10 * time.Second,
		paxChainID:     paxChainID,
		endpoint:       endpoint,
		txChan:         make(chan *types.LoadTx, bufferSize),
		sentTxs:        make(chan *types.LoadTx, bufferSize),
		workers:        workers,
		trackReceipts:  false,
		limiter:        limiter,
	}
	return w
}

// SetStatsCollector sets the statistics collector for this worker
func (w *Worker) SetStatsCollector(collector *stats.Collector, logger *stats.Logger) {
	w.collector = collector
	w.logger = logger
}

// Start begins the worker's processing loop
func (w *Worker) Run(ctx context.Context) error {
	meterWorkerQueueLength(w)
	defer unmeterWorkerQueueLength(w)
	client := newHttpClient()
	defer client.CloseIdleConnections()
	err := service.Run(ctx, func(ctx context.Context, s service.Scope) error {
		for range w.workers {
			s.Spawn(func() error { return w.processTransactions(ctx, client) })
		}
		return w.watchTransactions(ctx)
	})
	// All producers and receipt consumers have stopped before final accounting.
	for {
		select {
		case <-w.sentTxs:
			w.recordReceiptOutcome(context.Canceled)
		default:
			if w.trackReceipts {
				log.Printf("Receipt outcomes for %s: %+v", w.endpoint, w.GetReceiptStats())
			}
			return err
		}
	}
}

func (w *Worker) GetReceiptStats() ReceiptStats {
	w.receiptMu.Lock()
	defer w.receiptMu.Unlock()
	return w.receipts
}

func (w *Worker) recordReceiptOutcome(err error) {
	w.receiptMu.Lock()
	defer w.receiptMu.Unlock()
	w.receipts.Pending--
	switch {
	case err == nil:
		w.receipts.Succeeded++
	case errors.Is(err, context.DeadlineExceeded):
		w.receipts.TimedOut++
	case errors.Is(err, context.Canceled):
		w.receipts.Unresolved++
	default:
		w.receipts.Failed++
	}
}

// Send queues a transaction for this worker to process
func (w *Worker) Send(ctx context.Context, tx *types.LoadTx) error {
	return utils.Send(ctx, w.txChan, tx)
}

// SetDebug sets the dry-run mode for the worker
func (w *Worker) SetDebug(debug bool) {
	w.debug = debug
}

// SetDryRun sets the dry-run mode for the worker
func (w *Worker) SetDryRun(dryRun bool) {
	w.dryRun = dryRun
}

// SetTrackReceipts sets the track-receipts mode for the worker
func (w *Worker) SetTrackReceipts(trackReceipts bool) {
	w.trackReceipts = trackReceipts
}

func (w *Worker) watchTransactions(ctx context.Context) error {
	if w.dryRun || !w.trackReceipts {
		return nil
	}
	eth, err := ethclient.DialContext(ctx, w.endpoint)
	if err != nil {
		return fmt.Errorf("ethclient.Dial(%q): %w", w.endpoint, err)
	}
	defer eth.Close()
	return service.Run(ctx, func(ctx context.Context, s service.Scope) error {
		concurrency := min(max(w.workers, 1), 16)
		for range concurrency {
			s.Spawn(func() error {
				for {
					tx, err := utils.Recv(ctx, w.sentTxs)
					if err != nil {
						return err
					}
					itemCtx, cancel := context.WithTimeout(ctx, w.receiptTimeout)
					err = w.waitForReceipt(itemCtx, eth, tx)
					cancel()
					w.recordReceiptOutcome(err)
					if err != nil {
						log.Printf("receipt: %v", err)
					}
				}
			})
		}
		return nil
	})
}

func (w *Worker) waitForReceipt(ctx context.Context, eth *ethclient.Client, tx *types.LoadTx) (_err error) {
	defer func(start time.Time) {
		metrics.receiptLatency.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("scenario", tx.Scenario.Name),
				attribute.String("endpoint", w.endpoint),
				attribute.Int("worker_id", w.id),
				attribute.String("chain_id", w.paxChainID),
				statusAttrFromError(_err)),
		)
	}(time.Now())
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if _, err := utils.Recv(ctx, ticker.C); err != nil {
			return fmt.Errorf("waiting for receipt for tx %s: %w", tx.EthTx.Hash().Hex(), err)
		}
		receipt, err := eth.TransactionReceipt(ctx, tx.EthTx.Hash())
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				continue
			}
			log.Printf("❌ error getting receipt for tx %s: %v", tx.EthTx.Hash().Hex(), err)
			continue
		}
		// Receipt found - log status and return
		if receipt.Status != 1 {
			return fmt.Errorf("tx %s: %w", tx.EthTx.Hash().Hex(), ErrReceiptFailed)
		}
		if w.debug {
			log.Printf("✅ tx %s, %s, gas=%d succeeded\n", tx.Scenario.Name, tx.EthTx.Hash().Hex(), receipt.GasUsed)
		}
		return nil
	}
	return ctx.Err()
}

// processTransactions is the main worker loop that processes transactions
func (w *Worker) processTransactions(ctx context.Context, client *http.Client) error {
	for ctx.Err() == nil {
		// Apply rate limiting before getting the next transaction
		if w.limiter != nil {
			if err := w.limiter.Wait(ctx); err != nil {
				return err
			}
		}

		tx, err := utils.Recv(ctx, w.txChan)
		if err != nil {
			return err
		}

		startTime := time.Now()
		err = w.sendTransaction(ctx, client, tx)
		// Record statistics if collector is available
		if w.collector != nil {
			w.collector.RecordTransaction(tx.Scenario.Name, w.endpoint, time.Since(startTime), err == nil)
		}
		if err != nil {
			log.Printf("%v", err)
		}
	}
	return ctx.Err()
}

// sendTransaction sends a single transaction to the endpoint
func (w *Worker) sendTransaction(ctx context.Context, client *http.Client, tx *types.LoadTx) (_err error) {
	defer func(start time.Time) {
		metrics.sendLatency.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("scenario", tx.Scenario.Name),
				attribute.String("endpoint", w.endpoint),
				attribute.Int("worker_id", w.id),
				attribute.String("chain_id", w.paxChainID),
				statusAttrFromError(_err)),
		)
	}(time.Now())
	if w.dryRun {
		// In dry-run mode, simulate processing time and mark as successful
		// Use very minimal delay to avoid channel overflow
		return utils.Sleep(ctx, 10*time.Microsecond) // Much faster simulation
	}

	// Create HTTP request with JSON-RPC payload
	req, err := http.NewRequestWithContext(ctx, "POST", w.endpoint, bytes.NewReader(tx.JSONRPCPayload))
	if err != nil {
		return fmt.Errorf("Worker %d: Failed to create request: %w", w.id, err)
	}

	// Set headers for JSON-RPC
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Worker %d: Failed to send transaction: %w", w.id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Worker %d: HTTP error %d for transaction to %s", w.id, resp.StatusCode, w.endpoint)
	}
	if err := validateTransactionResponse(resp.Body, tx); err != nil {
		return fmt.Errorf("Worker %d: invalid transaction response: %w", w.id, err)
	}

	if w.trackReceipts {
		w.receiptMu.Lock()
		w.receipts.Accepted++
		w.receipts.Pending++
		w.receiptMu.Unlock()
		// Tracking applies backpressure after admission; cancellation remains visible.
		select {
		case w.sentTxs <- tx:
		case <-ctx.Done():
			w.recordReceiptOutcome(ctx.Err())
		}
	}
	return nil
}

// GetChannelLength returns the current length of the worker's channel (for monitoring).
// This function is safe for concurrent calls.
func (w *Worker) GetChannelLength() int {
	return len(w.txChan)
}

// GetEndpoint returns the worker's endpoint
func (w *Worker) GetEndpoint() string {
	return w.endpoint
}

const maxTransactionResponseBytes = 64 * 1024

// TransactionRPCError preserves the node's JSON-RPC error for callers.
type TransactionRPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

func (e *TransactionRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

func (e *TransactionRPCError) ErrorCode() int         { return e.Code }
func (e *TransactionRPCError) ErrorData() interface{} { return e.Data }

func validateTransactionResponse(body io.Reader, tx *types.LoadTx) error {
	data, err := io.ReadAll(io.LimitReader(body, maxTransactionResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxTransactionResponseBytes {
		return fmt.Errorf("JSON-RPC response exceeds %d bytes", maxTransactionResponseBytes)
	}
	var response struct {
		Version string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	var request types.JSONRPCRequest
	if err := json.Unmarshal(tx.JSONRPCPayload, &request); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	var requestID, responseID bytes.Buffer
	if err := json.Compact(&requestID, request.ID); err != nil {
		return fmt.Errorf("invalid request id: %w", err)
	}
	if err := json.Compact(&responseID, response.ID); err != nil {
		return fmt.Errorf("invalid response id: %w", err)
	}
	var id interface{}
	if err := json.Unmarshal(request.ID, &id); err != nil {
		return err
	}
	switch id.(type) {
	case string, float64:
	default:
		return fmt.Errorf("invalid JSON-RPC request id")
	}
	if response.Version != "2.0" || !bytes.Equal(requestID.Bytes(), responseID.Bytes()) {
		return fmt.Errorf("JSON-RPC version or id mismatch")
	}
	if len(response.Error) != 0 {
		if len(response.Result) != 0 {
			return fmt.Errorf("JSON-RPC response contains both result and error")
		}
		var nodeError struct {
			Code    *int            `json:"code"`
			Message *string         `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Error, &nodeError); err != nil {
			return err
		}
		if nodeError.Code == nil || nodeError.Message == nil {
			return fmt.Errorf("malformed JSON-RPC error")
		}
		return &TransactionRPCError{Code: *nodeError.Code, Message: *nodeError.Message, Data: nodeError.Data}
	}
	var hash common.Hash
	if err := json.Unmarshal(response.Result, &hash); err != nil {
		return fmt.Errorf("invalid transaction hash: %w", err)
	}
	if tx.EthTx == nil || hash != tx.EthTx.Hash() {
		return fmt.Errorf("JSON-RPC transaction hash mismatch")
	}
	return nil
}
