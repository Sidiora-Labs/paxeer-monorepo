package l16

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestReplayPersistenceRetainsImmutableArchiveAndMonotonicHead(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	snapshot, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	records, err := s.readReplayArchiveTx(ctx, tx, realMarketCreator, snapshot)
	if err != nil || len(records) != 1 || records[0].State != snapshot.State {
		t.Fatal(records, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "live"}[live], func(t *testing.T) {
			persist := func(tx pgx.Tx, snapshot RebuildSnapshot, records []EconomicRecord) error {
				if live {
					return s.livePersistReplayTx(ctx, tx, snapshot, records)
				}
				return s.persistReplayTx(ctx, tx, snapshot, records)
			}
			for _, tc := range []struct {
				name   string
				change func(pgx.Tx, *RebuildSnapshot, *[]EconomicRecord)
				want   string
			}{
				{name: "same archive"},
				{name: "older head", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) { v.Height-- }, want: "integrity"},
				{name: "archive conflict", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) {
					(*r)[0].SourceDigest = strings.Repeat("0", 64)
				}, want: "integrity"},
				{name: "identity", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) { (*r)[0].Leg.Identity.Chain = "1" }, want: "identity"},
				{name: "record time", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) {
					(*r)[0].CommittedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
				}, want: "MarshalJSON"},
				{name: "snapshot time", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) {
					v.ObservedThrough = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
				}, want: "MarshalJSON"},
				{name: "prior digest", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) {
					v.InputDigest = strings.Repeat("e", 64)
					raw := []byte(`{}`)
					digest := strings.Repeat("0", 64)
					if _, err := tx.Exec(ctx, `INSERT INTO pnl.inventory_l16_snapshots_v2(generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.Generation, v.Partition, digest, raw, v.Height, v.BlockHash, v.ReleaseDigest); err != nil {
						t.Fatal(err)
					}
					if _, err := tx.Exec(ctx, `UPDATE pnl.inventory_l16_active_v2 SET snapshot_digest=$1 WHERE partition=$2`, digest, v.Partition); err != nil {
						t.Fatal(err)
					}
				}, want: "integrity"},
				{name: "prior json", change: func(tx pgx.Tx, v *RebuildSnapshot, r *[]EconomicRecord) {
					v.InputDigest = strings.Repeat("e", 64)
					raw := []byte(`{`)
					digest := delivery.Hash(raw)
					if _, err := tx.Exec(ctx, `INSERT INTO pnl.inventory_l16_snapshots_v2(generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.Generation, v.Partition, digest, raw, v.Height, v.BlockHash, v.ReleaseDigest); err != nil {
						t.Fatal(err)
					}
					if _, err := tx.Exec(ctx, `UPDATE pnl.inventory_l16_active_v2 SET snapshot_digest=$1 WHERE partition=$2`, digest, v.Partition); err != nil {
						t.Fatal(err)
					}
				}, want: "error"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer inventoryRollback(tx)
					candidate := snapshot
					inputs := append([]EconomicRecord(nil), records...)
					if tc.change != nil {
						tc.change(tx, &candidate, &inputs)
					}
					err = persist(tx, candidate, inputs)
					if tc.want == "" {
						if err != nil {
							t.Fatal(err)
						}
						var count int
						if err = tx.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_archive_v2`).Scan(&count); err != nil || count != len(records) {
							t.Fatal(count, err)
						}
					} else if err == nil || tc.want == "integrity" && !errors.Is(err, delivery.ErrIntegrity) || tc.want != "integrity" && tc.want != "error" && !strings.Contains(err.Error(), tc.want) {
						t.Fatal(err)
					}
				})
			}
			assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error { return persist(tx, snapshot, records) })
		})
	}
}

func TestReplayArchiveRejectsUnboundStoredRecords(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	snapshot, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		digest bool
	}{
		{name: "digest"},
		{name: "time schema", change: func(r map[string]any) { r["CommittedAt"] = "invalid" }, digest: true},
		{name: "wallet binding", change: func(r map[string]any) { r["Leg"].(map[string]any)["Wallet"] = "0x" + strings.Repeat("e", 40) }, digest: true},
		{name: "state binding", change: func(r map[string]any) { r["state"].(map[string]any)["knownQuantityRaw"] = "0" }, digest: true},
		{name: "allocation binding", change: func(r map[string]any) { r["allocation"] = map[string]any{} }, digest: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			var raw []byte
			var identity string
			if err = tx.QueryRow(ctx, `SELECT identity,payload FROM pnl.inventory_l16_archive_v2 WHERE partition=$1 LIMIT 1`, snapshot.Partition).Scan(&identity, &raw); err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err = json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			if tc.change != nil {
				tc.change(document)
			}
			raw, err = json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			digest := strings.Repeat("0", 64)
			if tc.digest {
				digest = delivery.Hash(raw)
			}
			candidate := snapshot
			candidate.Generation = "g2"
			if _, err = tx.Exec(ctx, `INSERT INTO pnl.inventory_l16_archive_v2(generation,partition,identity,payload_hash,payload) VALUES($1,$2,$3,$4,$5)`, candidate.Generation, candidate.Partition, identity, digest, raw); err != nil {
				t.Fatal(err)
			}
			if _, err = s.readReplayArchiveTx(ctx, tx, realMarketCreator, candidate); err == nil {
				t.Fatal("unbound archive accepted")
			}
		})
	}
	if _, err = s.readReplayArchiveTx(ctx, nil, realMarketCreator, snapshot); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
}
