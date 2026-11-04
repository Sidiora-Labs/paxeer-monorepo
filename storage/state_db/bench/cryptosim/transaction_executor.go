package cryptosim

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
)

var ErrExecutorClosed = errors.New("transaction executor admission closed")

type TransactionExecutor struct {
	cancel context.CancelFunc
	config *CryptoSimConfig

	// The database for the benchmark.
	database *Database

	// The address of the fee collection account.
	feeCollectionAddress []byte

	// Admission is serialized with queue closure. Work already admitted by the
	// block loop must finish even when its workload context is cancelled.
	workChan chan any
	mu       sync.Mutex
	closed   bool
	done     chan struct{}
	err      error // Written by the worker, read after done closes.

	// Used to time the execution of transactions.
	phaseTimer *metrics.PhaseTimer
}

// A request to flush the transaction executor.
type flushRequest struct {
	doneChan chan error
}

// A single threaded transaction executor. The caller stops block admission on
// context cancellation, then must Close to drain and join this worker.
func NewTransactionExecutor(
	_ context.Context,
	cancel context.CancelFunc,
	config *CryptoSimConfig,
	database *Database,
	feeCollectionAddress []byte,
	queueSize int,
	metrics *CryptosimMetrics,
) *TransactionExecutor {
	e := &TransactionExecutor{
		cancel:               cancel,
		config:               config,
		database:             database,
		feeCollectionAddress: feeCollectionAddress,
		workChan:             make(chan any, queueSize),
		done:                 make(chan struct{}),
		phaseTimer:           metrics.GetTransactionPhaseTimerInstance(),
	}

	go e.mainLoop()

	return e
}

// Schedule a transaction from an admitted block for execution.
func (e *TransactionExecutor) ScheduleForExecution(txn *transaction) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrExecutorClosed
	}
	e.workChan <- txn
	return nil
}

// Flush waits for an execution acknowledgement, independently of cancellation.
func (e *TransactionExecutor) Flush() error {
	request := flushRequest{doneChan: make(chan error, 1)}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		<-e.done
		return errors.Join(ErrExecutorClosed, e.err)
	}
	e.workChan <- request
	e.mu.Unlock()
	return <-request.doneChan
}

// Close stops admission, drains every accepted request and joins the worker.
// Repeated and concurrent callers observe the same execution result.
func (e *TransactionExecutor) Close() error {
	e.mu.Lock()
	if !e.closed {
		e.closed = true
		close(e.workChan)
	}
	e.mu.Unlock()
	<-e.done
	return e.err
}

func (e *TransactionExecutor) mainLoop() {
	defer close(e.done)

	for request := range e.workChan {
		switch request := request.(type) {
		case *transaction:
			if e.config.DisableTransactionExecution {
				continue
			}

			var phaseTimer *metrics.PhaseTimer
			if request.ShouldCaptureMetrics() {
				phaseTimer = e.phaseTimer
			}

			if err := request.Execute(e.database, e.feeCollectionAddress, phaseTimer); err != nil {
				log.Printf("transaction execution error: %v", err)
				if e.err == nil {
					e.err = fmt.Errorf("transaction execution failed: %w", err)
					e.cancel()
				}
			}
		case flushRequest:
			request.doneChan <- e.err
		}
	}
}
