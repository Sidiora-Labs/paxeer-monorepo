package analytics_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestAnalyticsFillRequiresObservedCanonicalOrder(t *testing.T) {
	archive := repair1038Fixture(t)
	if len(archive.Source) == 0 {
		t.Fatal("canonical accounting source absent")
	}
	fill := archive.Source[0]
	a := phase2Authority(t)
	projector, err := analytics.NewProjector(a, risk.ProjectionModelV1{}, analytics.PostgresRiskInputs{Authority: a})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		at   time.Time
	}{
		{"missing observation", time.Time{}},
		{"local observation", time.Now().In(time.FixedZone("offset", 3600))},
		{"future event", time.Unix(1, 0).UTC()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := a.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := projector.ApplyFillTx(ctx, tx, fill, tc.at); err == nil {
				_ = tx.Rollback(ctx)
				t.Fatal("unqualified observation accepted")
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, priorDigest string
		want              error
	}{
		{"identical replay", string(digest), nil},
		{"changed fact", strings.Repeat("0", 64), analytics.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.Pool.Exec(ctx, "DELETE FROM "+a.Schema+".analytics_trades"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.Pool.Exec(ctx, "INSERT INTO "+a.Schema+".analytics_trades(source_id,pool,height,transaction_index,log_index,block_hash,event_time,side,usdl_raw,digest) VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,$9)", fill.SourceFillID, fill.PoolAddress, fill.Position.Height, fill.Position.TransactionIndex, fill.Position.LogIndex, fill.Position.BlockHash, fill.Side, fill.USDLAmountRaw, tc.priorDigest); err != nil {
				t.Fatal(err)
			}
			tx, err := a.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC())
			if !errors.Is(err, tc.want) {
				_ = tx.Rollback(ctx)
				t.Fatalf("replayed fill: %v, want %v", err, tc.want)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
