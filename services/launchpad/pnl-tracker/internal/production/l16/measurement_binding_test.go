package l16

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestInventoryMeasurementBindsCheckpointsAndCanonicalNodes(t *testing.T) {
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	inventoryCommittedHead(t, ctx, pool)
	identity, principal := inventoryIdentityFixture(t, realMarketCreator)
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := worker.RebuildOwned(ctx, principal)
	if err != nil || summary.SnapshotID == "" {
		t.Fatal(summary, err)
	}
	original, err := service.Active(ctx, realMarketCreator, realMarketToken)
	if err != nil {
		t.Fatal(err)
	}
	originalRaw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := delivery.Hash(originalRaw)
	base := InventoryMeasurementCase{ID: "canonical-inventory", Wallet: realMarketCreator, Token: realMarketToken, Height: original.Height, BlockHash: original.BlockHash, ExpectedState: &original.State}
	if raw, err := captureInventoryMeasurement(ctx, worker, base, service.release, 100000); err != nil || !json.Valid(raw) {
		t.Fatal(string(raw), err)
	}
	if raw, err := captureInventoryMeasurement(ctx, worker, base, service.release, 0); err == nil || !strings.Contains(err.Error(), "archive exceeds") || len(raw) == 0 {
		t.Fatal(string(raw), err)
	}
	normalized := base
	normalized.ExpectedNormalizedAverage = str("0")
	if _, err := captureInventoryMeasurement(ctx, worker, normalized, service.release, 100000); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		change     func(*RebuildSnapshot)
		checkpoint string
		parity     bool
	}{
		{name: "checkpoint completeness", change: func(s *RebuildSnapshot) { s.Checkpoints[0].CompleteHistory = false }},
		{name: "checkpoint state", change: func(s *RebuildSnapshot) { s.Checkpoints[0].State.Cost = "1" }},
		{name: "checkpoint rows missing", change: func(s *RebuildSnapshot) { s.NodeIDs[0] = "unbound" }, checkpoint: "missing"},
		{name: "checkpoint digest", change: func(s *RebuildSnapshot) { s.NodeIDs[0] = "unbound" }, checkpoint: "digest"},
		{name: "checkpoint release", change: func(s *RebuildSnapshot) { s.NodeIDs[0] = "unbound" }, checkpoint: "release"},
		{name: "node identity", change: func(s *RebuildSnapshot) { s.NodeIDs[0] = "unbound" }, parity: true},
		{name: "node agreement", change: func(s *RebuildSnapshot) { s.NodeBalances[1] = "0" }, parity: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var snapshot RebuildSnapshot
			if err := json.Unmarshal(originalRaw, &snapshot); err != nil {
				t.Fatal(err)
			}
			tc.change(&snapshot)
			if tc.checkpoint != "" {
				snapshot.NodeIDs[0] = tc.name
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			digest := delivery.Hash(raw)
			if _, err = pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_snapshots_v2(generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, original.Generation, original.Partition, digest, raw, original.Height, original.BlockHash, original.ReleaseDigest); err != nil {
				t.Fatal(err)
			}
			// Each binding is stored under a distinct immutable snapshot digest.
			if tc.checkpoint != "missing" {
				rows, err := pool.Query(ctx, `SELECT day::text,checkpoint_digest,payload FROM pnl.inventory_l16_checkpoints_v2 WHERE generation=$1 AND partition=$2 AND snapshot_digest=$3 ORDER BY day`, original.Generation, original.Partition, originalDigest)
				if err != nil {
					t.Fatal(err)
				}
				type checkpointRow struct {
					day  string
					hash string
					raw  []byte
				}
				entries := []checkpointRow{}
				for rows.Next() {
					var day string
					var hash string
					var payload []byte
					if err = rows.Scan(&day, &hash, &payload); err != nil {
						t.Fatal(err)
					}
					entries = append(entries, checkpointRow{day, hash, payload})
				}
				rows.Close()
				if err = rows.Err(); err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if tc.checkpoint == "digest" {
						entry.hash = strings.Repeat("0", 64)
					}
					if tc.checkpoint == "release" {
						var value map[string]any
						if err = json.Unmarshal(entry.raw, &value); err != nil {
							t.Fatal(err)
						}
						value["releaseDigest"] = strings.Repeat("0", 64)
						entry.raw, err = json.Marshal(value)
						if err != nil {
							t.Fatal(err)
						}
						entry.hash = delivery.Hash(entry.raw)
					}
					if _, err = pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_checkpoints_v2(generation,partition,snapshot_digest,day,checkpoint_digest,payload) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, original.Generation, original.Partition, digest, entry.day, entry.hash, entry.raw); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err = pool.Exec(ctx, `UPDATE pnl.inventory_l16_active_v2 SET snapshot_digest=$1 WHERE partition=$2`, digest, original.Partition); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, `UPDATE pnl.inventory_l16_active_v2 SET snapshot_digest=$1 WHERE partition=$2`, originalDigest, original.Partition); err != nil {
					t.Error(err)
				}
			}()
			_, err = captureInventoryMeasurement(ctx, worker, base, service.release, 100000)
			want := delivery.ErrIntegrity
			if tc.parity {
				want = ErrParity
			}
			if !errors.Is(err, want) {
				t.Fatal(err, want)
			}
		})
	}
}
