package l16

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestRebuiltHistoryCancellationPreservesPublishedSnapshot(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	snapshot, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	repository := l17.Repository{Pool: pool}
	summary, err := s.PublishRebuiltWallet(ctx, realMarketCreator, snapshot.Height, repository)
	if err != nil {
		t.Fatal(err)
	}
	var positionsRaw []byte
	if err = pool.QueryRow(ctx, `SELECT jsonb_agg(payload ORDER BY ordinal) FROM pnl.portfolio_snapshot_rows WHERE snapshot_id=$1`, summary.SnapshotID).Scan(&positionsRaw); err != nil {
		t.Fatal(err)
	}
	var positions []l17.Position
	if err = json.Unmarshal(positionsRaw, &positions); err != nil || len(positions) != 1 {
		t.Fatal(positions, err)
	}
	create := func(tx pgx.Tx, source string) (l17.Summary, error) {
		candidate := summary
		candidate.SnapshotID = ""
		candidate.Watermark.ProjectionRevision = "2"
		positions := append([]l17.Position(nil), positions...)
		for i := range positions {
			positions[i].SourceWatermark = candidate.Watermark
		}
		return repository.CommitSnapshotTx(ctx, tx, source, candidate, positions, time.Now().Add(time.Hour))
	}
	assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error {
		candidate, err := create(tx, "history-cancellation")
		if err != nil {
			return err
		}
		return s.commitRebuiltHistoryTx(ctx, tx, realMarketCreator, candidate.SnapshotID, "history-cancellation", []RebuildSnapshot{snapshot}, repository)
	})
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_wallet_ranges_v3 WHERE snapshot_id=$1`, summary.SnapshotID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	for _, kind := range []string{"invalid gas", "zero gas", "missing gas", "external sender"} {
		t.Run(kind, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
				t.Fatal(err)
			}
			records, err := s.readReplayArchiveTx(ctx, tx, realMarketCreator, snapshot)
			if err != nil || len(records) != 1 {
				t.Fatal(records, err)
			}
			record := &records[0]
			switch kind {
			case "invalid gas":
				record.GasPAXRaw = "bad"
			case "zero gas":
				record.GasPAXRaw = "0"
			case "missing gas":
				record.GasPAXRaw = ""
			case "external sender":
				record.Leg.Sender = "0x" + strings.Repeat("e", 40)
			}
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := record.Leg.Identity.Key()
			if err != nil {
				t.Fatal(err)
			}
			candidate := snapshot
			candidate.Generation = "history-variant"
			if _, err = tx.Exec(ctx, `INSERT INTO inventory_l16_archive_v2(generation,partition,identity,payload,payload_hash) VALUES($1,$2,$3,$4,$5)`, candidate.Generation, candidate.Partition, identity, raw, delivery.Hash(raw)); err != nil {
				t.Fatal(err)
			}
			all, err := json.Marshal(records)
			if err != nil {
				t.Fatal(err)
			}
			candidate.InputDigest = delivery.Hash(all)
			view, err := create(tx, "history-variant")
			if err != nil {
				t.Fatal(err)
			}
			err = s.commitRebuiltHistoryTx(ctx, tx, realMarketCreator, view.SnapshotID, "history-variant", []RebuildSnapshot{candidate}, repository)

			if kind == "invalid gas" {
				if err == nil {
					t.Fatal("invalid transaction gas accepted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	bad := snapshot
	bad.ObservedThrough = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if sameReplaySnapshot(bad, snapshot) {
		t.Fatal("unrepresentable snapshot compared equal")
	}
}

func TestParityQuarantineCancellationDoesNotPublish(t *testing.T) {
	ctx := context.Background()
	s, tracer := inventoryCancelableService(t)
	pool := s.authority.pool
	origin := repair63RecordedRPC(t)
	s.second, _ = inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool { return r.Method == "eth_call" }, func(out map[string]any) { out["result"] = "0x" + strings.Repeat("0", 64) })
	invoke := func() error { _, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733); return err }
	tracer.Start(0)
	err := invoke()
	queries := tracer.Stop()
	if !errors.Is(err, ErrParity) || len(queries) == 0 {
		t.Fatal(queries, err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM pnl.inventory_l16_quarantine_v2`); err != nil {
		t.Fatal(err)
	}
	for i, query := range queries {
		if strings.EqualFold(query, "rollback") {
			continue
		}
		tracer.Start(i + 1)
		err = invoke()
		observed := tracer.Stop()
		if !errors.Is(err, context.Canceled) || len(observed) != i+1 || observed[i] != query {
			t.Fatal(query, observed, err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_quarantine_v2`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
