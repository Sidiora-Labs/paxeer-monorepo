package processor

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestPendingCanonicalCommitResolvesBeforeSelectingAnotherRange(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	b := h.inventory.committedBatch(t, ctx, 33963716)
	p := h.Processor
	p.pendingCanonical = &b
	if err := p.ProcessBatch(ctx, 33963716, 33963716); err != nil || p.pendingCanonical != nil {
		t.Fatalf("committed response reconciliation %v", err)
	}
	p.pendingCanonical = &b
	if err := p.ProcessBatch(ctx, 33963717, 33963717); !errors.Is(err, store.ErrWatermark) || p.pendingCanonical != nil {
		t.Fatalf("different range reconciliation %v", err)
	}
	if _, err := h.Pool.Exec(ctx, `ALTER TABLE indexer.indexer_ingest_commits RENAME TO unavailable_commits`); err != nil {
		t.Fatal(err)
	}
	p.pendingCanonical = &b
	if err := p.ProcessBatch(ctx, 33963716, 33963716); err == nil || p.pendingCanonical != &b {
		t.Fatalf("unavailable acknowledgment forgot candidate %v", err)
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	err := p.Run(bounded)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || p.pendingCanonical != &b {
		t.Fatalf("pending retry shutdown %v", err)
	}
	if _, err = h.Pool.Exec(ctx, `ALTER TABLE indexer.unavailable_commits RENAME TO indexer_ingest_commits`); err != nil {
		t.Fatal(err)
	}
	if committed, err := h.Store.ReconcileCanonicalCommit(ctx, b); err != nil || !committed {
		t.Fatalf("restored canonical commit visibility %v: %v", committed, err)
	}
	h.head.height.Store(33963716)
	bounded, cancel = context.WithTimeout(ctx, 30*time.Millisecond)
	err = p.Run(bounded)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || p.pendingCanonical != nil {
		t.Fatalf("pending live reconciliation %v", err)
	}
}

func TestReserveSnapshotsPinLastSwapAndRejectInvalidPositions(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	_, receipts, err := h.Replay.Source.FetchQualifiedBatch(ctx, 33963733, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	var fills []accounting.FillV1
	for _, q := range receipts {
		f, err := q.Fills(ctx)
		if err != nil {
			t.Fatal(err)
		}
		fills = append(fills, f...)
	}
	if len(fills) != 1 {
		t.Fatalf("recorded buy fills %d", len(fills))
	}
	original := fills[0]
	index, err := strconv.ParseUint(original.Position.LogIndex, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	later := original
	later.Position.LogIndex = strconv.FormatUint(index+1, 10)
	p := h.Processor
	snapshots, err := p.readSwapReserves(ctx, []accounting.FillV1{original, later, original})
	if err != nil || len(snapshots) != 1 || snapshots[0].LogIndex != uint32(index+1) || snapshots[0].TransactionHash != original.Position.TransactionHash || snapshots[0].Reserves.Token == nil || snapshots[0].Reserves.Token.Sign() <= 0 {
		t.Fatalf("last swap reserves %+v %v", snapshots, err)
	}
	for _, tc := range []struct {
		name      string
		edit      func(*accounting.FillV1)
		duplicate bool
	}{
		{"pool", func(f *accounting.FillV1) { f.PoolAddress = "invalid" }, false},
		{"height", func(f *accounting.FillV1) { f.Position.Height = "invalid" }, false},
		{"index", func(f *accounting.FillV1) { f.Position.LogIndex = "invalid" }, false},
		{"comparison", func(f *accounting.FillV1) { f.Position.TransactionIndex = "invalid" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := original
			tc.edit(&f)
			input := []accounting.FillV1{f}
			if tc.duplicate {
				input = append(input, original)
			}
			if out, err := p.readSwapReserves(ctx, input); err == nil || out != nil {
				t.Fatalf("invalid snapshot %+v %v", out, err)
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := p.readSwapReserves(canceled, fills); err == nil || out != nil {
		t.Fatalf("cancelled reserve read %+v %v", out, err)
	}
	p.deps.QualifiedBatchSource = &source.HistoricalQualifiedSource{}
	if err := p.ProcessBatch(ctx, 33963717, 33963717); err == nil {
		t.Fatal("unavailable qualified source acknowledged")
	}
}
