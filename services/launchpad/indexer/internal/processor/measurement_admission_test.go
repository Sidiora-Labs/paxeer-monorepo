package processor

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

func TestMeasurementRejectsUnavailableWatermarkLeaseAndHead(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	ctx := context.Background()
	for _, tc := range []struct{ name, table, want string }{
		{"watermark", "indexer_ingest_watermarks", "indexer_ingest_watermarks"},
		{"lease", "indexer_ingest_leases", "indexer_ingest_leases"},
		{"head", "", "immutable ingestion payload conflict"},
		{"qualified source", "", "qualified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := h.spec(t)
			if tc.table != "" {
				if _, err := h.Pool.Exec(ctx, `ALTER TABLE indexer.`+tc.table+` RENAME TO measurement_unavailable`); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := h.Pool.Exec(ctx, `ALTER TABLE indexer.measurement_unavailable RENAME TO `+tc.table); err != nil {
						t.Fatal(err)
					}
				}()
			}
			original := h.Processor.deps.QualifiedBatchSource
			if tc.name == "head" {
				h.head.failure.Store(true)
				defer h.head.failure.Store(false)
			}
			if tc.name == "qualified source" {
				h.Processor.deps.QualifiedBatchSource = &source.HistoricalQualifiedSource{}
				defer func() { h.Processor.deps.QualifiedBatchSource = original }()
			}
			bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			report, err := h.Run(bounded, s)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) || report.CommittedRaw != 0 || report.CaptureSHA256 == "" || report.FinishedAt.IsZero() {
				t.Fatalf("measurement input admission %+v: %v", report, err)
			}
			if h.Processor.readyFence.Load() != 0 {
				t.Fatal("failed measurement retained readiness")
			}
		})
	}
}

func TestMeasurementReportsActualMemoryAndConnectionBudgetViolations(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	ctx := context.Background()
	spec := h.spec(t)
	spec.MaxMemoryBytes = 1
	spec.MaxDatabaseConnections = 1
	held, err := h.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	heldSecond, err := h.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer heldSecond.Release()
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	report, err := h.Run(bounded, spec)
	if !errors.Is(err, context.DeadlineExceeded) || report.CommittedBlocks != 34 || report.CommittedRaw != 32 || report.PeakRSSBytes <= 1 || report.PeakDatabaseConnections <= 1 {
		t.Fatalf("resource observations %+v: %v", report, err)
	}
	for _, want := range []string{"measured process memory exceeds admitted budget", "database connections exceed admitted budget"} {
		if !slices.Contains(report.Violations, want) {
			t.Fatalf("missing resource violation %s: %v", want, report.Violations)
		}
	}
	next := spec
	next.OutputPath = filepath.Join(t.TempDir(), "capture.ndjson")
	next.DeliverySchema = "unavailable"
	if report, err = h.Run(ctx, next); err == nil || report.CommittedRaw != 0 {
		t.Fatalf("unavailable storage schema %+v: %v", report, err)
	}
}

func TestCanonicalProcessorDropsFencedReadinessAndRejectsUnavailableWatermarks(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	ctx := context.Background()
	p := h.Processor
	if err := p.ProcessBatch(ctx, 33963716, 33963716); err != nil {
		t.Fatal(err)
	}
	old := p.canonicalFence
	if err := h.Store.ReleaseLease(ctx, 125, realStream, inventoryTestOwner, old); err != nil {
		t.Fatal(err)
	}
	other, err := h.Store.AcquireLease(ctx, 125, realStream, "other-owner", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.ProcessBatch(ctx, 33963717, 33963733); !errors.Is(err, store.ErrFenced) || p.canonicalFence != 0 || p.readyFence.Load() != 0 {
		t.Fatalf("stale lease admission: %v", err)
	}
	if err = p.ProcessBatch(ctx, 33963717, 33963733); !errors.Is(err, store.ErrFenced) {
		t.Fatalf("occupied lease admission: %v", err)
	}
	if err = h.Store.ReleaseLease(ctx, 125, realStream, "other-owner", other); err != nil {
		t.Fatal(err)
	}
	p.canonicalFence, err = h.Store.AcquireLease(ctx, 125, realStream, inventoryTestOwner, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	p.readyFence.Store(p.canonicalFence)
	if _, err = h.Pool.Exec(ctx, `ALTER TABLE indexer.indexer_ingest_watermarks RENAME TO watermark_unavailable`); err != nil {
		t.Fatal(err)
	}
	if err = p.ProcessBatch(ctx, 33963717, 33963733); err == nil {
		t.Fatal("unavailable watermark admitted")
	}
	if _, err = h.Pool.Exec(ctx, `ALTER TABLE indexer.watermark_unavailable RENAME TO indexer_ingest_watermarks`); err != nil {
		t.Fatal(err)
	}
	p.canonicalRenewed = time.Now().Add(-11 * time.Second)
	if err = h.Store.ReleaseLease(ctx, 125, realStream, inventoryTestOwner, p.canonicalFence); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Store.AcquireLease(ctx, 125, realStream, "other-owner", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if err = p.Run(ctx); !errors.Is(err, store.ErrFenced) || p.canonicalFence != 0 || p.readyFence.Load() != 0 {
		t.Fatalf("loop retained stale lease: %v", err)
	}
}
