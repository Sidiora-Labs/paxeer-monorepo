package analytics_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
)

func TestRecordedCanonicalPublicationCommitsAndReplays(t *testing.T) {
	archive := repair1038Fixture(t)
	fill := archive.Source[0]
	envelope, _ := repair1038Envelope(t, fill, 1, time.Now().UTC())
	authority := phase2Authority(t)
	projector, err := analytics.NewProjector(authority, risk.ProjectionModelV1{}, analytics.PostgresRiskInputs{Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	apply := func() error {
		t.Helper()
		tx, err := authority.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := projector.Effect(ctx, tx, envelope); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err := apply(); err != nil {
		t.Fatalf("recorded market publication: %v", err)
	}
	first, err := authority.ReadSnapshot(ctx, fill.PoolAddress, "pressure-v1", time.Now().Unix())
	if err != nil || first.Snapshot.Watermark.ProjectionRevision != "1" {
		t.Fatalf("first pressure snapshot: %+v %v", first, err)
	}
	if err := apply(); err != nil {
		t.Fatalf("idempotent publication: %v", err)
	}
	var count int
	if err := authority.Pool.QueryRow(ctx, "SELECT count(*) FROM "+authority.Schema+".analytics_outbox").Scan(&count); err != nil || count != 2 {
		t.Fatalf("one publication pair: %d %v", count, err)
	}
	tx, err := authority.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.RecomputeExpiryTx(ctx, tx, fill.PoolAddress); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	second, err := authority.ReadSnapshot(ctx, fill.PoolAddress, "pressure-v1", time.Now().Unix())
	if err != nil || second.Snapshot.Watermark.ProjectionRevision != "2" || second.Snapshot.Watermark.BlockHash != first.Snapshot.Watermark.BlockHash {
		t.Fatalf("expiry pressure snapshot: %+v %v", second, err)
	}
	page, err := authority.ReplaySnapshots(ctx, fill.PoolAddress, "0", 10)
	if err != nil || len(page.Items) != 4 {
		t.Fatalf("durable snapshot replay: %+v %v", page, err)
	}
	changed := envelope
	changed.SourceOrder.Height = strconv.FormatInt(time.Now().Unix(), 10)
	wrongTx, err := authority.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.Effect(ctx, wrongTx, changed); !errors.Is(err, analytics.ErrConflict) {
		_ = wrongTx.Rollback(ctx)
		t.Fatalf("changed source position: %v", err)
	}
	if err := wrongTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
