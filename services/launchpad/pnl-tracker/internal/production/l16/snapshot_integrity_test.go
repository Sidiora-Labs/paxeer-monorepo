package l16

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestActiveSnapshotAndPublicationRequireBoundPayload(t *testing.T) {
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	original, err := service.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil || original.Height != 33963733 {
		t.Fatal(original, err)
	}
	identity, _ := inventoryIdentityFixture(t, realMarketCreator)
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	originalRaw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := delivery.Hash(originalRaw)
	for _, tc := range []struct {
		name                 string
		change               func(*RebuildSnapshot)
		raw                  []byte
		digest               string
		activeError, publish bool
	}{
		{name: "hash binding", raw: []byte(`{}`), digest: strings.Repeat("0", 64), activeError: true, publish: true},
		{name: "json payload", raw: []byte(`{`), activeError: true, publish: true},
		{name: "invalid state", change: func(s *RebuildSnapshot) { s.State.Known = "invalid" }, activeError: true, publish: true},
		{name: "partition binding", change: func(s *RebuildSnapshot) { s.Partition = "invalid" }, activeError: true, publish: true},
		{name: "invalid algorithm", change: func(s *RebuildSnapshot) { s.Algorithm = "invalid" }, activeError: true},
		{name: "invalid generation", change: func(s *RebuildSnapshot) { s.Generation = "" }, activeError: true, publish: true},
		{name: "invalid height", change: func(s *RebuildSnapshot) { s.Height = 0 }, activeError: true, publish: true},
		{name: "invalid block hash", change: func(s *RebuildSnapshot) { s.BlockHash = "invalid" }, activeError: true, publish: true},
		{name: "release binding", change: func(s *RebuildSnapshot) { s.ReleaseDigest = strings.Repeat("e", 64) }, publish: true},
		{name: "generation binding", change: func(s *RebuildSnapshot) { s.Generation = "other" }, publish: true},
		{name: "history completeness", change: func(s *RebuildSnapshot) { s.Checkpoints = nil }, publish: true},
		{name: "history day", change: func(s *RebuildSnapshot) { s.Checkpoints[0].Day = "invalid" }, publish: true},
		{name: "archive input binding", change: func(s *RebuildSnapshot) { s.InputDigest = strings.Repeat("0", 64) }, publish: true},
		{name: "archive terminal identity", change: func(s *RebuildSnapshot) { s.LastIdentity = "invalid" }, publish: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var snapshot RebuildSnapshot
			if err := json.Unmarshal(originalRaw, &snapshot); err != nil {
				t.Fatal(err)
			}
			if tc.change != nil {
				tc.change(&snapshot)
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != nil {
				raw = tc.raw
			}
			digest := delivery.Hash(raw)
			if tc.digest != "" {
				digest = tc.digest
			}
			if _, err = pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_snapshots_v2(generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, original.Generation, original.Partition, digest, raw, original.Height, original.BlockHash, original.ReleaseDigest); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `UPDATE pnl.inventory_l16_active_v2 SET snapshot_digest=$1 WHERE partition=$2`, digest, original.Partition); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, `UPDATE pnl.inventory_l16_active_v2 SET snapshot_digest=$1 WHERE partition=$2`, originalDigest, original.Partition); err != nil {
					t.Error(err)
				}
			}()
			active, err := service.Active(ctx, realMarketCreator, realMarketToken)
			if (err != nil) != tc.activeError {
				t.Fatal(active, err)
			}
			if !tc.activeError && (active.InputDigest != snapshot.InputDigest || active.Generation != snapshot.Generation || active.LastIdentity != snapshot.LastIdentity) {
				t.Fatal(active, snapshot)
			}
			captureCase := InventoryMeasurementCase{ID: "snapshot-binding", Wallet: realMarketCreator, Token: realMarketToken, Height: original.Height, BlockHash: original.BlockHash, ExpectedState: &original.State}
			if _, err = captureInventoryMeasurement(ctx, worker, captureCase, service.release, 100000); err == nil {
				t.Fatal("unbound reconciliation captured")
			}
			if tc.publish {
				if _, err = service.PublishRebuiltWallet(ctx, realMarketCreator, original.Height, l17.Repository{Pool: pool}); err == nil {
					t.Fatal("unbound snapshot published")
				}
				var count int
				if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_snapshots`).Scan(&count); err != nil || count != 0 {
					t.Fatal(count, err)
				}
			}
		})
	}
	if _, err = service.HistoricalDay(ctx, realMarketCreator, realMarketToken, original.ObservedThrough.AddDate(0, 0, -3)); err == nil {
		t.Fatal("unrecorded day returned")
	}
	for _, tc := range []struct {
		wallet   string
		from, to time.Time
		interval string
		limit    int
	}{
		{"bad", time.Now(), time.Now(), "1h", 1},
		{realMarketCreator, time.Time{}, time.Now(), "1d", 1},
		{realMarketCreator, time.Now(), time.Now(), "other", 1},
	} {
		if _, err = service.IntervalHistory(ctx, tc.wallet, tc.from, tc.to, tc.interval, tc.limit); err == nil {
			t.Fatal(tc)
		}
	}
}
