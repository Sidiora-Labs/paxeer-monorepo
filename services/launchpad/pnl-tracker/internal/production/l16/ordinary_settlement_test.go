package l16

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
)

func TestOrdinaryReplaySettlesRecordedBuyAndSale(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	first, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil || first.State.Known == "0" {
		t.Fatal(first, err)
	}
	later, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963749)
	if err != nil || later.Height != 33963749 || later.State == first.State {
		t.Fatal(later, err)
	}
	summary, err := s.PublishRebuiltWallet(ctx, realMarketCreator, later.Height, l17.Repository{Pool: pool})
	if err != nil || summary.SnapshotID == "" || summary.PositionCount != 1 {
		t.Fatal(summary, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_history_rows WHERE snapshot_id=$1`, summary.SnapshotID).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
