package l16

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func acceptedSnapshotFixture(t *testing.T) (*LiveTransferWorker, RebuildSnapshot) {
	t.Helper()
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	inventoryCommittedHead(t, ctx, pool)
	worker, principal := incrementalWorker(t, service, realMarketCreator)
	summary, err := worker.RebuildOwned(ctx, principal)
	if err != nil || summary.SnapshotID == "" {
		t.Fatalf("wallet summary = %+v, error = %v", summary, err)
	}
	snapshot, err := service.Active(ctx, principal.Wallet, realMarketToken)
	if err != nil || snapshot.State.Known == "0" || len(snapshot.Checkpoints) == 0 {
		t.Fatalf("accepted snapshot = %+v, error = %v", snapshot, err)
	}
	return worker, snapshot
}

func TestAcceptedSnapshotRecoveryFromCommittedRows(t *testing.T) {
	ctx := context.Background()
	worker, snapshot := acceptedSnapshotFixture(t)
	service := worker.service
	pool := service.authority.pool
	wallet := strings.Split(snapshot.Partition, ":")[1]
	var height int64
	var blockHash, snapshotID, source string
	if err := pool.QueryRow(ctx, `SELECT accepted_height,accepted_hash,accepted_snapshot::text,source_digest FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, wallet).Scan(&height, &blockHash, &snapshotID, &source); err != nil {
		t.Fatal(err)
	}
	if height != snapshot.Height || blockHash != snapshot.BlockHash {
		t.Fatalf("accepted frontier = %d %s, snapshot = %d %s", height, blockHash, snapshot.Height, snapshot.BlockHash)
	}
	restarted, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, worker.identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := restarted.processWallet(ctx, wallet); err != nil || !changed {
		t.Fatalf("restore accepted wallet: changed = %v, error = %v", changed, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	seeds, err := restarted.acceptedSnapshotsTx(ctx, tx, wallet, height, &blockHash, &snapshotID, &source)
	if err != nil || len(seeds) != 1 || !reflect.DeepEqual(seeds[realMarketToken], snapshot) {
		t.Fatalf("restored snapshots = %+v, error = %v", seeds, err)
	}
	raw, err := restarted.readAcceptedSnapshotInputTx(ctx, tx, wallet, height, blockHash, snapshotID, source)
	if err != nil || delivery.Hash(raw) != source {
		t.Fatalf("restored input digest = %s, want %s, error = %v", delivery.Hash(raw), source, err)
	}
	var committed []byte
	if err = tx.QueryRow(ctx, `SELECT payload FROM pnl.inventory_l16_snapshots_v2 WHERE generation=$1 AND partition=$2 AND snapshot_digest=$3`, snapshot.Generation, "live-wallet:"+wallet, source).Scan(&committed); err != nil || string(raw) != string(committed) {
		t.Fatalf("reconstructed input differs from the committed wallet input: %v", err)
	}
	var input liveWalletInput
	if err = json.Unmarshal(raw, &input); err != nil || input.Release != service.release || input.Wallet != wallet || len(input.Snapshots) != 1 {
		t.Fatalf("restored input = %+v, error = %v", input, err)
	}
	if _, err = restarted.readAcceptedSnapshotInputTx(ctx, tx, wallet, height, blockHash, snapshotID, strings.Repeat("0", 64)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("uncommitted input digest: %v", err)
	}
	if _, err = restarted.readAcceptedSnapshotInputTx(ctx, tx, wallet, height+1, blockHash, snapshotID, source); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("uncommitted snapshot height: %v", err)
	}
}

func TestLiveReplayResumesAcceptedArchive(t *testing.T) {
	ctx := context.Background()
	worker, snapshot := acceptedSnapshotFixture(t)
	service := worker.service
	pool := service.authority.pool
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	progress, key, err := service.readLiveProgressTx(ctx, tx, snapshot.Partition, "accepted", &snapshot)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	day, err := time.Parse("2006-01-02", snapshot.Checkpoints[0].Day)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(progress.Snapshot, snapshot) || len(progress.Records) == 0 || progress.RecordCount != len(progress.Records) || progress.CreationTime != uint64(day.Unix()) || progress.BaseDigest != delivery.Hash(raw) || len(progress.DayHashState) == 0 || len(progress.RecordHashState) == 0 || len(progress.OpenPageMoves) == 0 || key != "live-enrollment:"+snapshot.Partition+":accepted" {
		t.Fatalf("resumed progress = %+v, key = %q", progress, key)
	}
	if progress.Records[len(progress.Records)-1].State != snapshot.State {
		t.Fatal("resumed archive does not reach the accepted inventory")
	}
	if err = service.writeLiveProgressTx(ctx, tx, key, progress); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	restored, restoredKey, err := service.readLiveProgressTx(ctx, tx, snapshot.Partition, "accepted", &snapshot)
	if err != nil || restoredKey != key || !reflect.DeepEqual(restored, progress) {
		t.Fatalf("durable progress = %+v, key = %q, error = %v", restored, restoredKey, err)
	}
	for _, tc := range []struct {
		name       string
		change     func(*RebuildSnapshot)
		parseError bool
	}{
		{"partition", func(s *RebuildSnapshot) { s.Partition = "invalid" }, false},
		{"checkpoints", func(s *RebuildSnapshot) { s.Checkpoints = nil }, false},
		{"checkpoint date", func(s *RebuildSnapshot) { s.Checkpoints[0].Day = "invalid" }, true},
		{"creation epoch", func(s *RebuildSnapshot) { s.Checkpoints[0].Day = "1969-12-31" }, false},
		{"checkpoint binding", func(s *RebuildSnapshot) { s.Checkpoints[0].InputDigest = strings.Repeat("0", 64) }, false},
		{"archive binding", func(s *RebuildSnapshot) { s.InputDigest = strings.Repeat("0", 64) }, false},
		{"movement binding", func(s *RebuildSnapshot) { s.MovementDigest = strings.Repeat("0", 64) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := snapshot
			changed.Checkpoints = append([]Checkpoint(nil), snapshot.Checkpoints...)
			tc.change(&changed)
			_, err := service.resumeLiveSnapshotTx(ctx, tx, changed)
			var parseError *time.ParseError
			if tc.parseError {
				if !errors.As(err, &parseError) {
					t.Fatalf("checkpoint date error = %v", err)
				}
			} else if !errors.Is(err, delivery.ErrIntegrity) {
				t.Fatalf("changed snapshot error = %v", err)
			}
		})
	}
}
