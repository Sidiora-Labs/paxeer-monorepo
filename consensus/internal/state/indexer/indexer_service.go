package indexer

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/service"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/paxeer-network/paxlog"
)

var logger = paxlog.NewLogger("tendermint", "internal", "state", "indexer")

// Service connects event bus, transaction and block indexers together in
// order to index transactions and blocks coming from the event bus.
type Service struct {
	service.BaseService

	eventSinks   []EventSink
	eventBus     *eventbus.EventBus
	metrics      *Metrics
	recovery     utils.Option[RecoveryArgs]
	mu           sync.Mutex
	runCtx       context.Context
	cancel       context.CancelFunc
	indexed      map[EventSinkType]int64
	progressKeys map[EventSinkType][]byte

	currentBlock struct {
		header types.EventDataNewBlockHeader
		height int64
		batch  *Batch
	}
}

// NewService constructs a new indexer service from the given arguments.
func NewService(args ServiceArgs) *Service {
	is := &Service{
		eventSinks: args.Sinks,
		eventBus:   args.EventBus,
		metrics:    args.Metrics,
		recovery:   args.Recovery,
	}
	if is.metrics == nil {
		is.metrics = NopMetrics()
	}
	is.BaseService = *service.NewBaseService("IndexerService", is)
	return is
}

// publish publishes a pubsub message to the service. The service blocks until
// the message has been fully processed.
func (is *Service) publish(msg pubsub.Message) error {
	is.mu.Lock()
	defer is.mu.Unlock()
	if err := is.runCtx.Err(); err != nil {
		return err
	}

	switch data := msg.Data().(type) {
	case types.EventDataNewBlockHeader:
		if is.recovery.IsPresent() {
			if data.Header.Height <= is.indexedThrough() {
				return nil // Startup replay may already have indexed this queued event.
			}
			if err := is.replay(is.runCtx, data.Header.Height-1); err != nil {
				return err
			}
			if is.currentBlock.batch != nil && is.currentBlock.height <= is.indexedThrough() {
				is.currentBlock.batch = nil // Replay completed a block with lost live events.
			}
		}
		if is.currentBlock.batch != nil {
			return fmt.Errorf("received block %d before completing block %d", data.Header.Height, is.currentBlock.height)
		}
		if data.NumTxs < 0 {
			return fmt.Errorf("negative transaction count at %d", data.Header.Height)
		}
		is.currentBlock.header = data
		is.currentBlock.height = data.Header.Height
		is.currentBlock.batch = NewBatch(data.NumTxs)
	case types.EventDataTx:
		tx := data.TxResultV2
		if is.recovery.IsPresent() && tx.Height <= is.indexedThrough() {
			return nil
		}
		if is.currentBlock.batch == nil || tx.Height != is.currentBlock.height {
			return fmt.Errorf("transaction at %d has no matching block header", tx.Height)
		}
		if uint64(tx.Index) >= uint64(is.currentBlock.batch.Size()) {
			return fmt.Errorf("transaction index %d exceeds block size at %d", tx.Index, tx.Height)
		}
		if err := is.currentBlock.batch.Add(&tx); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected indexer event %T", data)
	}

	curr := is.currentBlock.batch
	if curr.Pending == 0 {
		if err := is.indexBatch(is.runCtx, is.currentBlock.header, curr); err != nil {
			return err
		}
		is.currentBlock.batch = nil // Return to WAIT only after every sink acknowledges.
	}
	return nil
}

// retry keeps failed sink writes unacknowledged until they succeed or shutdown
// interrupts them. A crash leaves the durable watermark at the preceding height.
func (is *Service) retry(ctx context.Context, height int64, operation string, write func() error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := write(); err == nil {
			return nil
		} else {
			logger.Error("indexer write failed; retrying", "height", height, "operation", operation, "err", err)
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (is *Service) indexBatch(ctx context.Context, header types.EventDataNewBlockHeader, batch *Batch) error {
	height := header.Header.Height
	for _, sink := range is.eventSinks {
		if sink.Type() == NULL {
			continue
		}
		if is.recovery.IsPresent() {
			if height <= is.indexed[sink.Type()] {
				continue
			}
			if height != is.indexed[sink.Type()]+1 {
				return fmt.Errorf("non-contiguous %s index progress at %d", sink.Type(), height)
			}
		}
		start := time.Now()
		if err := is.retry(ctx, height, "block", func() error { return sink.IndexBlockEvents(header) }); err != nil {
			return err
		}
		is.metrics.BlockEventsSeconds.Observe(time.Since(start).Seconds())
		is.metrics.BlocksIndexed.Add(1)
		logger.Debug("indexed block", "height", height, "sink", sink.Type())
		if batch.Size() != 0 {
			start = time.Now()
			if err := is.retry(ctx, height, "transactions", func() error { return sink.IndexTxEvents(batch.Ops) }); err != nil {
				return err
			}
			is.metrics.TxEventsSeconds.Observe(time.Since(start).Seconds())
			is.metrics.TransactionsIndexed.Add(float64(batch.Size()))
			logger.Debug("indexed txs", "height", height, "sink", sink.Type())
		}
		if r, ok := is.recovery.Get(); ok {
			data := binary.BigEndian.AppendUint64(nil, uint64(height))
			if err := is.retry(ctx, height, "watermark", func() error {
				return r.ProgressStore.SetSync(is.progressKeys[sink.Type()], data)
			}); err != nil {
				return err
			}
			is.indexed[sink.Type()] = height
		}
	}
	return nil
}

// OnStart implements part of service.Service. It registers an observer for the
// indexer if the underlying event sinks support indexing.
//
// TODO(creachadair): Can we get rid of the "enabled" check?
func (is *Service) OnStart(ctx context.Context) (err error) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.runCtx, is.cancel = context.WithCancel(ctx)
	defer func() {
		if err != nil {
			is.cancel()
		}
	}()
	if IndexingEnabled(is.eventSinks) {
		if err := is.initRecovery(); err != nil {
			return err
		}
		// Register while holding mu, so live events cannot overtake startup replay.
		if err := is.eventBus.Observe(is.runCtx, is.publish,
			types.EventQueryNewBlockHeader, types.EventQueryTx); err != nil {
			return err
		}
		if r, ok := is.recovery.Get(); ok {
			state, err := r.StateStore.Load()
			if err != nil {
				return err
			}
			return is.replay(is.runCtx, state.LastBlockHeight)
		}
	}
	return nil
}

// OnStop implements service.Service by closing the event sinks.
func (is *Service) OnStop() {
	is.cancel()
	is.mu.Lock()
	defer is.mu.Unlock()
	for _, sink := range is.eventSinks {
		if err := sink.Stop(); err != nil {
			logger.Error("failed to close eventsink", "eventsink", sink.Type(), "err", err)
		}
	}
}

// ServiceArgs are arguments for constructing a new indexer service.
type ServiceArgs struct {
	Sinks    []EventSink
	EventBus *eventbus.EventBus
	Metrics  *Metrics
	Recovery utils.Option[RecoveryArgs]
}

// KVSinkEnabled returns the given eventSinks is containing KVEventSink.
func KVSinkEnabled(sinks []EventSink) bool {
	for _, sink := range sinks {
		if sink.Type() == KV {
			return true
		}
	}

	return false
}

// IndexingEnabled returns the given eventSinks is supporting the indexing services.
func IndexingEnabled(sinks []EventSink) bool {
	for _, sink := range sinks {
		if sink.Type() == KV || sink.Type() == PSQL {
			return true
		}
	}

	return false
}
