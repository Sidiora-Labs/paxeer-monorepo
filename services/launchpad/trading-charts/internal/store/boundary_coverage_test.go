package store

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestChartStorageRejectsUnboundedAuthorityInputs(t *testing.T) {
	for _, ddl := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
		if sql, err := ddl("invalid-schema"); sql != "" || err == nil {
			t.Fatalf("invalid schema name: %q %v", sql, err)
		}
	}
	if st, err := NewExact(nil, "candles"); st != nil || err == nil {
		t.Fatalf("store without owned pool: %+v %v", st, err)
	}
	address := "0x" + strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, ddl := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
		sql, err := ddl("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exact.DeleteRetainedRange(ctx, "invalid", "1m", 0, 60); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("retention pool: %v", err)
	}
	if err := exact.HoldReplayRange(ctx, "owned", address, 0, 60, time.Millisecond); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("short replay hold: %v", err)
	}
	if err := exact.RecordRecoverabilityIncident(ctx, address, "", digest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("incident without reason: %v", err)
	}
	if err := exact.RecordAttempt(ctx, "charts", "local", "publication", "bad", 1, "received", false); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unbound delivery digest: %v", err)
	}
	if err := exact.AuthorizeReprocess(ctx, "charts", "local", "publication", digest, digest, "decoder", "operator"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unisolated reprocess generation: %v", err)
	}
	if err := exact.CheckReprocess(ctx, "charts", "local", "publication", digest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unisolated reprocess check: %v", err)
	}
	if err := exact.StageRebuild(ctx, "invalid/id", address, 0, 60, digest, digest, digest, 1, nil, nil, nil); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("rebuild identity: %v", err)
	}
	if err := exact.FreezePoolsTx(ctx, nil, []string{address}, accounting.Digest(digest)); err == nil {
		t.Fatal("freeze without transaction admitted")
	}
	if published, err := exact.PublishNextCommitted(ctx, nil, "candle.updated.v2"); published || err == nil {
		t.Fatalf("relay without publisher: %t %v", published, err)
	}
	if _, err := exact.AdvanceSeriesTx(ctx, nil, address, 1, 1, false); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("clock without transaction: %v", err)
	}
	if err := exact.InvalidateRollupTx(ctx, nil, address, "5m", 0, math.MaxUint64); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unbounded rollup revision: %v", err)
	}
	if unresolved, err := exact.HasUnresolvedDeliveriesTx(ctx, nil); unresolved || !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unowned delivery query: %t %v", unresolved, err)
	}
	if raw, err := refreshSeriesSnapshot([]byte(`{"intervalEnd":0}`), 1, 1, false, time.Now()); raw != nil || !errors.Is(err, ErrIntegrity) {
		t.Fatalf("nonpositive candle end: %s %v", raw, err)
	}
	if raw, err := refreshSeriesSnapshot([]byte(`{"intervalEnd":"invalid"}`), 1, 1, false, time.Now()); raw != nil || err == nil {
		t.Fatalf("malformed candle end: %s %v", raw, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if err := exact.FreezePoolsTx(ctx, tx, []string{"invalid"}, accounting.Digest(digest)); err == nil || !strings.Contains(err.Error(), "invalid freeze pool") {
		t.Fatalf("noncanonical freeze pool: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
