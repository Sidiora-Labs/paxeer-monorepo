package source

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBatchReadBoundsAndOrder(t *testing.T) {
	ctx := context.Background()
	for _, input := range [][2]int{{-1, 1}, {1, 0}} {
		values, err := ParallelBatchRead(ctx, input[0], input[1], func(_ context.Context, index int) (int, error) { return index, nil })
		var se *SourceError
		if values != nil || !errors.As(err, &se) || se.Kind != InvalidError {
			t.Fatalf("batch bounds %v: %v, %v", input, values, err)
		}
	}
	if values, err := ParallelBatchRead[int](ctx, 1, 1, nil); values != nil || err == nil {
		t.Fatalf("missing reader: %v, %v", values, err)
	}
	values, err := ParallelBatchRead(ctx, 0, 1, func(_ context.Context, index int) (int, error) {
		t.Fatal("empty batch invoked reader")
		return index, nil
	})
	if err != nil || values == nil || len(values) != 0 {
		t.Fatalf("empty batch: %v, %v", values, err)
	}
	values, err = ParallelBatchRead(ctx, 3, 64, func(_ context.Context, index int) (int, error) { return index * index, nil })
	if err != nil || !reflect.DeepEqual(values, []int{0, 1, 4}) {
		t.Fatalf("ordered batch: %v, %v", values, err)
	}
}

func TestCommittedArchiveConcurrencyAndPruningAdmission(t *testing.T) {
	var absent *CommittedSource
	if absent.HistoricalConcurrency() != defaultHistoricalConcurrency || new(CommittedSource).HistoricalConcurrency() != defaultHistoricalConcurrency {
		t.Fatal("archive concurrency default changed")
	}
	opts := fopts(fixture(t, "0x7d").URL)
	opts.HistoricalConcurrency = 7
	live := sourceRecoveryOpen(t, opts)
	if got := live.HistoricalConcurrency(); got != 7 {
		t.Fatalf("archive concurrency = %d", got)
	}
	for _, concurrency := range []int{-1, maxHistoricalConcurrency + 1} {
		invalid := opts
		invalid.HistoricalConcurrency = concurrency
		if source, err := NewCommittedSource(context.Background(), invalid); source != nil || err == nil {
			t.Fatalf("archive concurrency %d: %v, %v", concurrency, source, err)
		}
	}
	endpoint := live.endpoints[0]
	for _, message := range []string{"pruned", "pruned; earliest available is 18446744073709551616", "pruned; earliest available is 100", "pruned; earliest available is 90"} {
		live.learnFloor(endpoint, sourceError(RPCError, endpoint.id, errors.New(message)))
	}
	if endpoint.floor != 100 || !live.prunedFor(endpoint, 99) || live.prunedFor(endpoint, 100) {
		t.Fatalf("pruning floor = %d", endpoint.floor)
	}
	if err := live.PollHeads(context.Background(), nil, nil); err == nil {
		t.Fatal("polling admitted missing callbacks")
	}
	if err := live.ReportLiveLag(time.Now(), -time.Second); err == nil {
		t.Fatal("negative lag admitted")
	}
}

func TestCommittedCachedIdentityConflictIsQuarantined(t *testing.T) {
	live := sourceRecoveryOpen(t, fopts(fixture(t, "0x7d").URL))
	result, err := live.FetchLogs(context.Background(), FetchOptions{FromBlock: 100, ToBlock: 100})
	if err != nil {
		t.Fatal(err)
	}
	original := result.Blocks[100]
	changed := original
	changed.TimestampSeconds++
	err = live.retainBlock(live.endpoints[0], changed)
	var se *SourceError
	if !errors.As(err, &se) || se.Kind != IntegrityError {
		t.Fatalf("changed cached block: %v", err)
	}
	if live.cache[original.Hash] != original || !live.endpoints[0].quarantined || len(live.IntegrityIncidents()) != 1 {
		t.Fatal("cached identity conflict did not retain provenance and quarantine endpoint")
	}
}

func TestShellReceiptRejectsAmbiguousTransactionMembership(t *testing.T) {
	for _, name := range []string{"oversized list", "zero listed hash", "duplicate listed hash", "zero shell hash", "listed shell hash", "duplicate shell", "unlisted receipt", "listed index"} {
		t.Run(name, func(t *testing.T) {
			block := chainfixture.NewShellBlock(t, true)
			hashes := []common.Hash{block.Transactions[0].Hash()}
			switch name {
			case "oversized list":
				hashes = make([]common.Hash, 4097)
			case "zero listed hash":
				hashes[0] = common.Hash{}
			case "duplicate listed hash":
				hashes = append(hashes, hashes[0])
			case "zero shell hash":
				block.Receipts[1].TxHash = common.Hash{}
			case "listed shell hash":
				block.Receipts[1].TxHash = hashes[0]
			case "duplicate shell":
				block.Receipts = append(block.Receipts, block.Receipts[1])
			case "unlisted receipt":
				hashes[0] = common.HexToHash("0x1234")
			case "listed index":
				block.Receipts[0].TransactionIndex = 1
				for _, log := range block.Receipts[0].Logs {
					log.TxIndex = 1
				}
			}
			listed, err := separateShellReceipts(block.Receipts, hashes, block.Hash, block.Header.Number.Uint64(), hexutil.Encode(block.Header.Bloom[:]), block.Header.ReceiptHash.Hex(), true)
			var se *SourceError
			want := IntegrityError
			if name == "zero listed hash" {
				want = IncompleteError
			}
			if listed != nil || !errors.As(err, &se) || se.Kind != want {
				t.Fatalf("receipt membership: %v, %v", listed, err)
			}
		})
	}
}

func TestCapacityProfileAndWatermarkAdmission(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	live := sourceRecoveryOpen(t, fopts(node.server.URL))
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 100)
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live, Explorer: explorer}, Head: live, Processor: &canonicalCapacityWorkload{source: live, store: store.New(pool), stream: "capacity-profile"}, Pool: pool}
	base := capacityBoundOptions(t, live)
	base.StreamID = "capacity-profile"
	base.OutputDirectory = t.TempDir()
	base.Duration = time.Second
	base.HistoricalFrom, base.HistoricalTo = 1, 90
	base.ParityFrom, base.ParityTo = 1, 10000
	base.MaxCaptureBytes, base.MaxHeapBytes = 1<<20, 1<<30
	base.MutationApproved = true
	for _, tc := range []struct {
		profile string
		rate    uint64
		want    string
	}{{"harness", 0, "capacity-rate"}, {"peak", 2000, "capacity-peak"}, {"burst", 10000, "capacity-burst"}, {"unissued", 1, "capacity-profile"}, {"harness", 1, "capacity-watermark"}} {
		t.Run(tc.want, func(t *testing.T) {
			opts := base
			opts.Profile, opts.TargetEventsPerSecond = tc.profile, tc.rate
			report, err := RunSourceCapacity(ctx, opts, providers)
			var se *SourceError
			if !errors.As(err, &se) || se.SourceID != tc.want || report.HarnessPassed || report.CapacityTargetQualified || !report.StartedAt.IsZero() {
				t.Fatalf("capacity admission: %+v, %v", report, err)
			}
		})
	}
}

func TestCapacityCaptureRejectsUnsupportedValuesAndEmptyWorkload(t *testing.T) {
	if digest, err := capacityHash(math.Inf(1)); digest != "" || err == nil {
		t.Fatalf("nonfinite digest: %q, %v", digest, err)
	}
	capture, err := newCapacityCapture(filepath.Join(t.TempDir(), "capture.jsonl"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, _, err := capture.finish(); err != nil {
			t.Error(err)
		}
	}()
	if err := capture.write("sample", math.Inf(1)); err == nil || capture.used != 0 {
		t.Fatalf("nonfinite capture changed byte count: %d, %v", capture.used, err)
	}
	command := capacityOutputCommand(t, "")
	command.Args = []string{""}
	err = capacityWorkload(context.Background(), command, capture, make(chan CapacityFaultObservation))
	var se *SourceError
	if !errors.As(err, &se) || se.Kind != IncompleteError || se.SourceID != "capacity-workload-capture" {
		t.Fatalf("empty workload: %v", err)
	}
}

func TestCapacityDurableRejectsUnavailableRelations(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	for _, table := range []string{"indexer_raw_logs", "indexer_block_completeness"} {
		t.Run(table, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "ALTER TABLE indexer."+table+" RENAME TO unavailable_capacity_relation"); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, "ALTER TABLE indexer.unavailable_capacity_relation RENAME TO "+table); err != nil {
					t.Error(err)
				}
			}()
			at, count, err := capacityDurable(ctx, pool, "capacity-relations", 101, 101, FetchResult{}, nil, 0, map[int64]uint64{}, nil)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42P01" || !at.IsZero() || count != 0 {
				t.Fatalf("unavailable relation: %v, %d, %v", at, count, err)
			}
		})
	}
}
