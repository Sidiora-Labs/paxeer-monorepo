package l16

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLiveWalletSourceLeaseRenewsDuringReplay(t *testing.T) {
	f := newLiveOwnerSessionFixture(t)
	p := f.principalWithProofAge(t, time.Minute)
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	blocker, err := f.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(blocker)
	var blockerPID int
	if err = blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}
	if _, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "125:"+f.chain.Wallet+":"+f.chain.Token); err != nil {
		t.Fatal(err)
	}
	type result struct {
		summary l17.Summary
		err     error
	}
	results := make(chan result, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		summary, err := f.worker.RebuildOwned(ctx, p)
		results <- result{summary, err}
	}()
	t.Cleanup(func() {
		cancel()
		inventoryRollback(blocker)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("wallet rebuild did not stop")
		}
	})
	wait := func(ready func() bool) {
		t.Helper()
		deadline := time.NewTimer(12 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for !ready() {
			select {
			case actual := <-results:
				t.Fatalf("wallet rebuild completed before replay resumed: %+v, %v", actual.summary, actual.err)
			case <-deadline.C:
				t.Fatal("wallet replay or producer renewal did not advance")
			case <-ticker.C:
			}
		}
	}
	wait(func() bool {
		var blocked bool
		if err = f.admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, blockerPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		return blocked
	})
	var owner string
	var epoch int64
	var expires time.Time
	if err = f.admin.QueryRow(ctx, `SELECT owner_instance,fence_token,expires_at FROM indexer.indexer_ingest_leases_v1 WHERE chain_id=125 AND stream_id=$1`, f.chain.Stream).Scan(&owner, &epoch, &expires); err != nil {
		t.Fatal(err)
	}
	wait(func() bool {
		var renewed bool
		if err = f.admin.QueryRow(ctx, `SELECT owner_instance=$2 AND fence_token=$3 AND expires_at>$4 AND expires_at>clock_timestamp() FROM indexer.indexer_ingest_leases_v1 WHERE chain_id=125 AND stream_id=$1`, f.chain.Stream, owner, epoch, expires).Scan(&renewed); err != nil {
			t.Fatal(err)
		}
		return renewed
	})
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	actual := <-results
	if actual.err != nil || actual.summary.SnapshotID == "" {
		t.Fatal(actual.summary, actual.err)
	}
	var commits, history int
	if err = f.admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.inventory_l16_live_commits_v1 WHERE snapshot_id=$1 AND source_owner=$2 AND source_epoch=$3),(SELECT count(*) FROM pnl.portfolio_history_rows WHERE snapshot_id=$1)`, actual.summary.SnapshotID, owner, epoch).Scan(&commits, &history); err != nil || commits != 1 || history != 1 {
		t.Fatal(commits, history, err)
	}
}

func TestLiveSourceLeaseFenceRejectsChangedAuthority(t *testing.T) {
	ctx := context.Background()
	s, admin := recordedRebuildService(t, ctx)
	for _, change := range []string{
		`owner_instance='replacement'`,
		`fence_token=fence_token+1`,
		`expires_at=clock_timestamp()-interval '1 second'`,
	} {
		t.Run(change, func(t *testing.T) {
			tx, err := s.authority.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			owner, epoch, err := s.authority.observeSourceLease(ctx, tx)
			if err != nil {
				t.Fatal(err)
			}
			writer, err := admin.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(writer)
			if _, err = writer.Exec(ctx, `SET LOCAL lock_timeout='1s'`); err != nil {
				t.Fatal(err)
			}
			if _, err = writer.Exec(ctx, `UPDATE indexer.indexer_ingest_leases SET `+change+` WHERE chain_id=125 AND stream_id=$1`, s.authority.sourceStream); err != nil {
				t.Fatal(err)
			}
			if err = writer.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = s.authority.fenceSourceLease(ctx, tx, owner, epoch); !errors.Is(err, delivery.ErrLease) {
				t.Fatal(err)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err = admin.Exec(ctx, `UPDATE indexer.indexer_ingest_leases SET owner_instance=$2,fence_token=$3,expires_at=clock_timestamp()+interval '30 seconds' WHERE chain_id=125 AND stream_id=$1`, s.authority.sourceStream, owner, epoch); err != nil {
				t.Fatal(err)
			}
		})
	}
	tx, err := s.authority.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	owner, epoch, err := s.authority.observeSourceLease(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.authority.fenceSourceLease(ctx, tx, owner, epoch); err != nil {
		t.Fatal(err)
	}
	writer, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(writer)
	_, err = writer.Exec(ctx, `SELECT 1 FROM indexer.indexer_ingest_leases WHERE chain_id=125 AND stream_id=$1 FOR UPDATE NOWAIT`, s.authority.sourceStream)
	var refused *pgconn.PgError
	if !errors.As(err, &refused) || refused.Code != "55P03" {
		t.Fatal(err)
	}
}
