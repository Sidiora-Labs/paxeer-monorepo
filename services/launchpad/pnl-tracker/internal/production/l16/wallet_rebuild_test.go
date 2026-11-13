package l16

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
)

func TestWalletRebuildDiscoversTransferredTokensAndReusesArchive(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	repository := l17.Repository{Pool: pool}
	var snapshot string
	for i := 0; i < 2; i++ {
		summary, err := s.RebuildWallet(ctx, realMarketCreator, 33963733, repository)
		if err != nil || summary.PositionCount != 1 || summary.Wallet != realMarketCreator || summary.SnapshotID == "" {
			t.Fatal(summary, err)
		}
		if i == 0 {
			snapshot = summary.SnapshotID
		} else if summary.SnapshotID != snapshot {
			t.Fatal("unchanged archive published a different snapshot", summary.SnapshotID, snapshot)
		}
	}
	active, err := s.Active(ctx, realMarketCreator, realMarketToken)
	if err != nil || active.Height != 33963733 || active.State.Known == "0" {
		t.Fatal(active, err)
	}
}
