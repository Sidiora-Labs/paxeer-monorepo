package store

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestRebuildPayloadRequiresRepresentableTimesAndCompleteness(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	position := accounting.SourcePosition{Height: "1", TransactionIndex: "0", LogIndex: "0"}
	base := rebuildSnapshot{
		Interval: "1m", Start: 60, IntervalEnd: 120,
		AlgorithmVersion: "qualified-exact-v2", Provenance: "actual_trade", ActorSemantics: "canonical_swap_actor_v1",
		Trades: 1, Revision: 1, DisplayRevision: 1, CompletenessStatus: "live_interval",
		FirstPosition: rebuildOrder{Height: 1}, LastPosition: rebuildOrder{Height: 1},
		OpenSource: rebuildPriceSource{Position: position}, CloseSource: rebuildPriceSource{Position: position},
	}
	for _, tc := range []struct {
		name string
		edit func(*rebuildSnapshot)
	}{
		{"start beyond calendar range", func(v *rebuildSnapshot) { v.Start, v.IntervalEnd = 253402300800, 253402300860 }},
		{"end beyond calendar range", func(v *rebuildSnapshot) { v.Start, v.IntervalEnd = 253402300740, 253402300800 }},
		{"unknown completeness", func(v *rebuildSnapshot) { v.CompletenessStatus = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bar := base
			tc.edit(&bar)
			payload, err := rebuildCanonicalPayload(ctx, tx, "candles", bar)
			if payload != nil || !errors.Is(err, ErrIntegrity) {
				t.Fatalf("canonical rebuild payload: %s, %v", payload, err)
			}
		})
	}
}
