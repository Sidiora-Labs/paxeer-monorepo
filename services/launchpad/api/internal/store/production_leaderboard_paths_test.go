package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestLeaderboardEmptyAccountingUsesCommittedHead(t *testing.T) {
	model, db := productionAccountingFixture(t)
	internaltest.ApplyReadMigrations(t, db, "60001")
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'canonical',900,$1,'healthy','canonical'),(125,'canonical',1001,$2,'healthy','canonical')`, productionBlockHash(900), productionBlockHash(1001)); err != nil {
		t.Fatal(err)
	}
	const end = int64(1700000001)
	before := time.Now().UTC().Truncate(time.Millisecond)
	empty := map[string]string{}
	for _, category := range []string{"trader", "creator", "volume"} {
		for _, window := range []string{"24h", "7d", "30d"} {
			t.Run(category+"/"+window, func(t *testing.T) {
				snapshot, err := model.Leaderboard(ctx, category, window, end)
				if err != nil {
					t.Fatal(err)
				}
				if snapshot.Category != category || snapshot.Window != window || snapshot.WindowEnd != end || snapshot.Entries == nil || len(snapshot.Entries) != 0 || snapshot.Quality != "complete" || snapshot.MethodologyVersion != store.LeaderboardMethodologyVersion || snapshot.Revision != "1001" {
					t.Fatalf("empty leaderboard: %+v", snapshot)
				}
				watermark := snapshot.Watermark
				observed, err := time.Parse("2006-01-02T15:04:05.000Z", watermark.ObservedAt)
				if err != nil || observed.Before(before) || observed.After(time.Now()) || watermark.ChainID != "125" || watermark.Height != "1001" || watermark.BlockHash != productionBlockHash(1001) || watermark.ProjectionRevision != "1001" || watermark.Integrity != "healthy" {
					t.Fatalf("committed head watermark: %+v, %v", watermark, err)
				}
				if _, err = uuid.Parse(snapshot.SnapshotID); err != nil {
					t.Fatal(err)
				}
				replay, err := model.Leaderboard(ctx, category, window, end)
				if err != nil || replay.SnapshotID != snapshot.SnapshotID {
					t.Fatalf("empty snapshot replay: %+v, %v", replay, err)
				}
				empty[category+"/"+window] = snapshot.SnapshotID
			})
		}
	}
	if len(empty) != 9 {
		t.Fatalf("empty scopes: %v", empty)
	}
	insertProductionFill(t, db, 1, "")
	for _, category := range []string{"trader", "creator", "volume"} {
		for _, window := range []string{"24h", "7d", "30d"} {
			snapshot, err := model.Leaderboard(ctx, category, window, end)
			if err != nil || snapshot.SnapshotID == empty[category+"/"+window] || snapshot.Revision != "1" {
				t.Fatalf("first fill snapshot: %+v, %v", snapshot, err)
			}
		}
	}
	config := productionReadConfig()
	config.ProjectionVersion = "next-market"
	next, err := store.NewAccountingReadModel(db, config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := next.Leaderboard(ctx, "volume", "24h", end)
	if err != nil || len(snapshot.Entries) != 0 || snapshot.Revision != "1001" || snapshot.SnapshotID != empty["volume/24h"] {
		t.Fatalf("empty projection version: %+v, %v", snapshot, err)
	}
}

func TestLeaderboardEmptyAccountingRequiresReadableHealthyHead(t *testing.T) {
	model, db := productionAccountingFixture(t)
	internaltest.ApplyReadMigrations(t, db, "60001")
	ctx := context.Background()
	if snapshot, err := model.Leaderboard(ctx, "volume", "24h", 1700000001); snapshot != nil || !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("missing committed head: %+v, %v", snapshot, err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'canonical',110,$1,'healthy','canonical'),(125,'canonical',111,$2,'quarantined','canonical')`, productionBlockHash(110), productionBlockHash(111)); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := model.Leaderboard(ctx, "volume", "24h", 1700000001); snapshot != nil || !errors.Is(err, store.ErrWatermarkQuarantined) {
		t.Fatalf("quarantined latest head: %+v, %v", snapshot, err)
	}
	for _, table := range []string{"stats.accounting_market_commits", "indexer.indexer_committed_heads"} {
		t.Run(table, func(t *testing.T) {
			lock, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(ctx)
			if _, err = lock.Exec(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE"); err != nil {
				t.Fatal(err)
			}
			snapshot, err := model.Leaderboard(ctx, "volume", "24h", 1700000001)
			var databaseError *pgconn.PgError
			if snapshot != nil || !errors.As(err, &databaseError) || databaseError.Code != "57014" {
				t.Fatalf("bounded accounting read: %+v, %v", snapshot, err)
			}
		})
	}
	for _, view := range []string{"indexer.indexer_committed_heads_v2", "stats.accounting_market_commits_v1"} {
		if _, err := db.Exec(ctx, "DROP VIEW "+view+" CASCADE"); err != nil {
			t.Fatal(err)
		}
		snapshot, err := model.Leaderboard(ctx, "volume", "24h", 1700000001)
		var databaseError *pgconn.PgError
		if snapshot != nil || !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
			t.Fatalf("unreadable accounting dependency: %+v, %v", snapshot, err)
		}
	}
}
