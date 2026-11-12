package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestLeaderboardRejectsCorruptCommittedHead(t *testing.T) {
	model, db := productionAccountingFixture(t)
	insertProductionFill(t, db, 1, "")
	ctx := context.Background()
	for _, check := range []struct {
		name, field, value string
	}{
		{"height", "{position,height}", `"-1"`},
		{"block hash", "{position,blockHash}", `"unavailable"`},
		{"timestamp", "{blockTimestampSeconds}", `"-1"`},
	} {
		t.Run(check.name, func(t *testing.T) {
			var original []byte
			if err := db.QueryRow(ctx, `SELECT fill FROM stats.accounting_market_commits WHERE pool=$1`, productionAddress).Scan(&original); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := db.Exec(ctx, `UPDATE stats.accounting_market_commits SET fill=$1 WHERE pool=$2`, original, productionAddress); err != nil {
					t.Error(err)
				}
			})
			if _, err := db.Exec(ctx, `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,$1,$2::jsonb) WHERE pool=$3`, check.field, check.value, productionAddress); err != nil {
				t.Fatal(err)
			}
			if board, err := model.Leaderboard(ctx, "volume", "24h", 1700000001); board != nil || !errors.Is(err, store.ErrDataIntegrity) {
				t.Fatalf("corrupt %s entered leaderboard: %+v %v", check.name, board, err)
			}
		})
	}
	if seconds, ok := store.LeaderboardWindowSeconds("7d"); !ok || seconds != 604800 {
		t.Fatalf("seven day leaderboard window: %d %t", seconds, ok)
	}
	if seconds, ok := store.LeaderboardWindowSeconds("7days"); ok || seconds != 0 {
		t.Fatalf("unknown leaderboard window: %d %t", seconds, ok)
	}
}
