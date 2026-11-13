package analytics_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
)

func TestRiskInputRowsFollowCanonicalRevision(t *testing.T) {
	authority := phase2Authority(t)
	ctx := context.Background()
	pool := "0x" + strings.Repeat("6", 40)
	wm := analytics.Watermark{ChainID: "125", Height: "10", BlockHash: "0x" + strings.Repeat("a", 64), ProjectionRevision: "1", ObservedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"}
	provider := analytics.PostgresRiskInputs{Authority: authority}
	read := func(watermark analytics.Watermark) (analytics.RiskInputs, error) {
		t.Helper()
		tx, err := authority.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		return provider.LoadRiskInputs(ctx, tx, pool, watermark)
	}
	missing, err := read(wm)
	if err != nil || missing.HolderCount != -1 || missing.UniqueTraders24h != -1 {
		t.Fatalf("unpublished inputs: %+v %v", missing, err)
	}
	inputs := analytics.RiskInputs{ProjectionRevision: "1", CreatedAt: time.Now().Unix() - 100, Top10Concentration: "9000", CreatorHoldingsPct: "5000", SupplyRaw: "100", UniqueTraders24h: 2, HolderCount: 3}
	write := func(watermark analytics.Watermark, value analytics.RiskInputs) error {
		t.Helper()
		tx, err := authority.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err = authority.PersistRiskInputsTx(ctx, tx, pool, watermark, value); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err := write(wm, inputs); err != nil {
		t.Fatal(err)
	}
	if got, err := read(wm); err != nil || got != inputs {
		t.Fatalf("committed inputs: %+v %v", got, err)
	}
	if err := write(wm, inputs); err != nil {
		t.Fatalf("idempotent inputs: %v", err)
	}
	changed := inputs
	changed.HolderCount = 4
	if err := write(wm, changed); !errors.Is(err, analytics.ErrConflict) {
		t.Fatalf("same revision changed: %v", err)
	}
	newer := wm
	newer.ProjectionRevision = "2"
	changed.ProjectionRevision = "2"
	if err := write(newer, changed); err != nil {
		t.Fatalf("newer inputs: %v", err)
	}
	if got, err := read(newer); err != nil || got != changed {
		t.Fatalf("newer committed inputs: %+v %v", got, err)
	}
	if _, err := read(wm); !errors.Is(err, analytics.ErrConflict) {
		t.Fatalf("stale read: %v", err)
	}
	if err := write(wm, inputs); !errors.Is(err, analytics.ErrConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if err := authority.PersistRiskInputsTx(ctx, nil, pool, newer, changed); err == nil {
		t.Fatal("missing transaction accepted")
	}
	if _, err := provider.LoadRiskInputs(ctx, nil, pool, newer); err == nil {
		t.Fatal("missing read transaction accepted")
	}
}
