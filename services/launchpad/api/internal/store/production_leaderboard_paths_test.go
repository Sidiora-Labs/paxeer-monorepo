package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestProductionLeaderboardRanksCommittedVolumeAndCreatorFees(t *testing.T) {
	model, db := productionAccountingFixture(t)
	insertProductionFill(t, db, 1, "")
	const wallet = "0x5555555555555555555555555555555555555555"
	for _, check := range []struct {
		category, wantWallet, wantMetric string
	}{
		{"volume", wallet, "1000000"},
		{"creator", wallet, "3000"},
	} {
		t.Run(check.category, func(t *testing.T) {
			result, err := model.Leaderboard(context.Background(), check.category, "24h", 1700000001)
			if err != nil {
				t.Fatal(err)
			}
			if result.Category != check.category || result.WindowEnd != 1700000001 || len(result.Entries) != 1 ||
				result.Entries[0].Wallet != check.wantWallet || result.Entries[0].MetricRaw != check.wantMetric || result.Entries[0].Quality != "partial" ||
				result.Watermark.Height != "110" || result.Revision != "1" || result.SnapshotID == "" {
				t.Fatalf("committed leaderboard: %+v", result)
			}
		})
	}
	for _, check := range []struct {
		category, window string
		end              int64
	}{
		{"other", "24h", 1700000001},
		{"volume", "other", 1700000001},
		{"volume", "24h", -1},
	} {
		if _, err := model.Leaderboard(context.Background(), check.category, check.window, check.end); !errors.Is(err, store.ErrAccountingNotReady) {
			t.Fatalf("scope %+v: %v", check, err)
		}
	}
	outside, err := model.Leaderboard(context.Background(), "volume", "24h", 1700200000)
	if err != nil || len(outside.Entries) != 0 || !strings.HasPrefix(outside.Watermark.BlockHash, "0x") {
		t.Fatalf("empty window: %+v, %v", outside, err)
	}
}
