package processor

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type recordedWorkload struct {
	result  source.FetchResult
	err     error
	options []source.FetchOptions
}

func (s *recordedWorkload) FetchLogs(ctx context.Context, options source.FetchOptions) (source.FetchResult, error) {
	s.options = append(s.options, options)
	if ctx.Err() != nil {
		return source.FetchResult{}, ctx.Err()
	}
	return s.result, s.err
}
func (s *recordedWorkload) Name() string { return "recorded-workload" }
func (s *recordedWorkload) Close() error { return nil }

func TestLegacyBatchProjectsRowsAndAcknowledgesCursor(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	batch := fixtureBatch(t)
	sort.Slice(batch.Records, func(i, j int) bool { return batch.Records[i].LogIndex < batch.Records[j].LogIndex })
	logs := make([]types.Log, 0, len(batch.Records)+3)
	for _, raw := range batch.Records {
		log := types.Log{Address: common.HexToAddress(raw.Emitter), BlockHash: common.HexToHash(raw.BlockHash), TxHash: common.HexToHash(raw.TransactionHash), BlockNumber: uint64(raw.Height), TxIndex: uint(raw.TransactionIndex), Index: uint(raw.LogIndex), Data: raw.Data}
		for _, topic := range raw.Topics {
			log.Topics = append(log.Topics, common.HexToHash(topic))
		}
		logs = append(logs, log)
	}
	malformed := logs[1]
	malformed.Data = []byte{0xff}
	logs = append(logs, malformed)
	unknown := logs[1]
	unknown.Topics = []common.Hash{common.HexToHash("0x1234")}
	logs = append(logs, unknown)
	absentTimestamp := logs[1]
	absentTimestamp.BlockNumber = 102
	logs = append(logs, absentTimestamp)
	workload := &recordedWorkload{result: source.FetchResult{Logs: logs, BlockTimestamps: map[uint64]int64{101: 123456}, TxFromMap: map[string]string{}}}
	p := New(Deps{Store: store.New(pool), Source: workload, Decoder: decode.NewDecoder(), ChainID: 125, EventEmitterAddress: batch.Records[0].Emitter, Concurrency: 4})
	events, err := p.collect(ctx, 101, 102)
	if err != nil || len(events) != 4 {
		t.Fatalf("legacy events %d: %v", len(events), err)
	}
	if events[0].EventName != "MarketCreated" || events[1].EventName != "Swap" || events[1].Args["poolAddress"] != "0x"+strings.Repeat("2", 40) || events[1].Args["blockTimestamp"] != int64(123456) {
		t.Fatalf("legacy projection %+v", events)
	}
	if len(workload.options) != 1 || workload.options[0].FromBlock != 101 || workload.options[0].ToBlock != 102 || workload.options[0].Concurrency != 4 || !reflect.DeepEqual(workload.options[0].MonitoredAddresses, p.monitored) {
		t.Fatalf("source options %+v", workload.options)
	}
	if err = p.ProcessBatch(ctx, 101, 102); err != nil {
		t.Fatal(err)
	}
	cursor, err := p.deps.Store.GetCursor(ctx, 125)
	if err != nil || cursor == nil || *cursor != 102 {
		t.Fatalf("legacy cursor %v: %v", cursor, err)
	}
	var markets, swaps int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.pools),(SELECT count(*) FROM indexer.swaps)`).Scan(&markets, &swaps); err != nil || markets != 1 || swaps != 3 {
		t.Fatalf("typed rows %d/%d: %v", markets, swaps, err)
	}
	workload.err = store.ErrIntegrity
	if err = p.ProcessBatch(ctx, 103, 103); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("source error acknowledged: %v", err)
	}
	after, err := p.deps.Store.GetCursor(ctx, 125)
	if err != nil || after == nil || *after != 102 {
		t.Fatalf("source error advanced cursor %v: %v", after, err)
	}
	workload.err = nil
	pool.Close()
	if err = p.ProcessBatch(ctx, 103, 103); err == nil {
		t.Fatal("unavailable typed storage acknowledged")
	}
}

func TestTypedEventFamiliesPersistExactValues(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	p := processorFor(pool)
	poolID := common.HexToHash(airdropPoolID)
	holder := common.HexToAddress(airdropHolder)
	n := big.NewInt
	logs := []types.Log{
		encodedLog(t, bindings.EventEmitterMetaData, "FeeRecorded", 1, []any{poolID}, n(7), n(2), n(5), n(1760000500), n(777)),
		encodedLog(t, bindings.EventEmitterMetaData, "FeeDistributed", 2, []any{poolID}, n(42), uint8(1), n(7), holder, n(1760000500), n(777)),
		encodedLog(t, bindings.EventEmitterMetaData, "FeeStrategyChanged", 3, []any{poolID}, n(42), uint8(0), uint8(1), n(1760000500), n(777)),
		encodedLog(t, bindings.EventEmitterMetaData, "OpticalExecuted", 4, []any{poolID, holder}, "beforeSwap", []byte{1, 2}, n(1760000500), n(777)),
		encodedLog(t, bindings.EventEmitterMetaData, "ConfigUpdated", 5, []any{poolID}, n(1), n(2), n(1760000500), n(777)),
		encodedLog(t, bindings.EventEmitterMetaData, "PoolStateUpdated", 6, []any{poolID}, n(11), n(12), n(13), n(14), n(1760000500), n(777)),
	}
	for _, log := range logs {
		event, err := p.deps.Decoder.Decode(log)
		if err != nil || event == nil {
			t.Fatalf("typed event decode: %v", err)
		}
		webhook, err := p.processEvent(ctx, event, 1760000500, nil)
		if err != nil || webhook == nil || webhook.EventName != event.EventName || webhook.TxHash != event.TxHash || webhook.LogIndex != int(log.Index) {
			t.Fatalf("typed webhook %+v: %v", webhook, err)
		}
	}
	event := &decode.Event{EventName: "TokenForTokenSwap", BlockNumber: 777, TxHash: airdropTx, LogIndex: 7, Args: map[string]any{"sender": airdropHolder, "tokenIn": airdropPool, "tokenOut": airdropOther, "poolIn": airdropPool, "poolOut": airdropOther, "amountIn": "100", "intermediateUsdl": "90", "amountOut": "80", "feeIn": "2", "feeOut": "3"}}
	webhook, err := p.processEvent(ctx, event, 1760000500, nil)
	if err != nil || webhook == nil || webhook.Args["actors"] == nil {
		t.Fatalf("token swap webhook %+v: %v", webhook, err)
	}
	for _, table := range []string{"fee_events", "fee_distributions", "fee_strategy_changes", "optical_executions", "config_updates", "pool_state_snapshots", "token_for_token_swaps"} {
		var count int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM indexer."+table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s rows %d: %v", table, count, err)
		}
	}
	var oldStrategy, newStrategy int32
	var nft int64
	if err = pool.QueryRow(ctx, `SELECT nft_id,old_strategy,new_strategy FROM indexer.fee_strategy_changes`).Scan(&nft, &oldStrategy, &newStrategy); err != nil || nft != 42 || oldStrategy != 0 || newStrategy != 1 {
		t.Fatalf("strategy projection %d/%d/%d: %v", nft, oldStrategy, newStrategy, err)
	}
}

func TestTypedEventInputsRefuseInvalidNumbersAndStorage(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	p := processorFor(pool)
	for _, tc := range []struct{ event, key string }{{"FeeDistributed", "nftId"}, {"FeeDistributed", "strategy"}, {"FeeStrategyChanged", "nftId"}, {"FeeStrategyChanged", "oldStrategy"}, {"FeeStrategyChanged", "newStrategy"}} {
		t.Run(tc.event+"/"+tc.key, func(t *testing.T) {
			args := map[string]any{"nftId": "42", "strategy": "1", "oldStrategy": "0", "newStrategy": "1"}
			args[tc.key] = "invalid"
			webhook, err := p.processEvent(ctx, &decode.Event{EventName: tc.event, Args: args}, 123456, nil)
			if err == nil || webhook != nil {
				t.Fatalf("invalid %s %s accepted", tc.event, tc.key)
			}
		})
	}
	pool.Close()
	for _, event := range []string{"MarketCreated", "Swap", "PoolStateUpdated", "FeeRecorded", "FeeDistributed", "FeeStrategyChanged", "AirdropClaimed", "FeeFlow", "OpticalExecuted", "TokenForTokenSwap", "ConfigUpdated", "ChainLaunched", "ChainRevenueEpochOpened", "ChainRevenueClaimed", "LaunchedChainRetired"} {
		t.Run(event, func(t *testing.T) {
			args := map[string]any{"nftId": "42", "strategy": "1", "oldStrategy": "0", "newStrategy": "1", "kind": "4", "holder": airdropHolder, "pool": airdropPool, "amount": "7", "chain": "7"}
			webhook, err := p.processEvent(ctx, &decode.Event{EventName: event, Args: args}, 123456, nil)
			if err == nil || webhook != nil {
				t.Fatalf("%s acknowledged unavailable storage", event)
			}
		})
	}
	if _, err := rawSourceRecord(types.Log{BlockNumber: math.MaxUint64}); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("unbounded raw height %v", err)
	}
	if _, err := rawSourceRecord(types.Log{Removed: true}); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("removed raw log %v", err)
	}
}

func TestCanonicalBatchRefusesUnboundRangesAndProviders(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	for _, bounds := range [][2]int64{{-1, 1}, {2, 1}} {
		if err := h.Processor.ProcessBatch(ctx, bounds[0], bounds[1]); err == nil {
			t.Fatalf("invalid canonical range %v", bounds)
		}
	}
	for _, field := range []string{"stream", "owner"} {
		p := New(h.Processor.deps)
		if field == "stream" {
			p.deps.CanonicalStreamID = ""
		} else {
			p.deps.CanonicalOwner = ""
		}
		if err := p.ProcessBatch(ctx, 33963716, 33963716); err == nil {
			t.Fatalf("missing %s accepted", field)
		}
	}
	if err := h.Processor.ProcessBatch(ctx, 33963717, 33963717); !errors.Is(err, store.ErrWatermark) {
		t.Fatalf("discontinuous range %v", err)
	}
	h.inventory.batch(t, ctx, 33963716, 33963716)
	if err := h.Processor.ProcessBatch(ctx, 33963716, 33963716); err != nil {
		t.Fatalf("committed batch reconciliation %v", err)
	}
	h.Processor.pendingCanonical = &store.CanonicalBatch{ChainID: 125, StreamID: realStream, Owner: inventoryTestOwner, BatchID: "not-committed", Fence: h.Processor.readyFence.Load(), PreviousHeight: 33963716, Blocks: []store.BlockCompleteness{{Height: 33963717, Hash: "0x" + strings.Repeat("1", 64), FetchComplete: true, RequiredDecodeComplete: true}}}
	committed, err := h.Processor.reconcilePendingCanonical(ctx)
	if err != nil || committed || h.Processor.pendingCanonical != nil {
		t.Fatalf("uncommitted pending resolution %v: %v", committed, err)
	}
	h.Pool.Close()
	if err = h.Processor.ProcessBatch(ctx, 33963717, 33963717); err == nil || h.Processor.readyFence.Load() != 0 {
		t.Fatalf("unavailable lease ready: %v", err)
	}
	p := New(h.Processor.deps)
	if err = p.Run(ctx); err == nil {
		t.Fatal("unavailable canonical watermark started ingestion")
	}
}

func TestLegacyRunStopsOnContextAndStorage(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	workload := &recordedWorkload{result: source.FetchResult{BlockTimestamps: map[uint64]int64{}}}
	head := new(measuredHead)
	p := New(Deps{Store: store.New(pool), Source: workload, Head: head, Decoder: decode.NewDecoder(), ChainID: 125, PollInterval: time.Millisecond})
	if err := p.deps.Store.UpsertCursor(ctx, 125, 99); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := p.Run(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled live processor %v", err)
	}
	for _, failure := range []bool{false, true} {
		head.failure.Store(failure)
		bounded, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		err := p.Run(bounded)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("idle live loop failure=%v: %v", failure, err)
		}
	}
	pool.Close()
	if err := p.Run(ctx); err == nil {
		t.Fatal("unavailable cursor started live loop")
	}
}
