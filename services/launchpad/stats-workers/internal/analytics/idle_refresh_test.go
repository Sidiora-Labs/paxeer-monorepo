package analytics_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func analyticsIdleRefreshFixture(t *testing.T) (*analytics.Projector, string, string) {
	t.Helper()
	source, fill, _ := canonicalRiskPostgresFixture(t)
	projector, err := analytics.NewProjector(source.Authority, risk.ProjectionModelV1{}, source)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tx, err := source.Authority.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return projector, fill.Token.Address, fill.PoolAddress
}

func TestAnalyticsIdleRefreshRetriesBlockedSnapshot(t *testing.T) {
	projector, token, pool := analyticsIdleRefreshFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := projector.Authority.Pool
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", projector.Authority.Schema+":analytics:"+pool); err != nil {
		t.Fatal(err)
	}
	blocker := tx.Conn().PgConn().PID()
	done := make(chan error, 1)
	go func() { done <- projector.RefreshTokenInputs(ctx, token) }()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)) AND wait_event='advisory')`, blocker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("refresh ended before waiting for pool lock: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	var expiryRevision int
	if err := tx.QueryRow(ctx, "UPDATE "+projector.Authority.Schema+".analytics_progress SET revision=revision+1 WHERE pool=$1 RETURNING revision::integer", pool).Scan(&expiryRevision); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("refresh after concurrent expiry: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var revision, outbox int
	if err := db.QueryRow(ctx, "SELECT revision::integer,(SELECT count(*) FROM "+projector.Authority.Schema+".analytics_outbox WHERE pool=$1) FROM "+projector.Authority.Schema+".analytics_progress WHERE pool=$1", pool).Scan(&revision, &outbox); err != nil || revision != expiryRevision+1 || outbox != 4 {
		t.Fatalf("revision=%d expiry=%d outbox=%d: %v", revision, expiryRevision, outbox, err)
	}
	for _, model := range []string{"pressure-v1", projector.Model.Version()} {
		result, err := projector.Authority.ReadSnapshot(ctx, pool, model, time.Now().Unix())
		if err != nil || result.Snapshot.Watermark.ProjectionRevision != strconv.Itoa(revision) {
			t.Fatalf("refreshed %s snapshot: %+v %v", model, result, err)
		}
	}
}

func TestAnalyticsIdleRefreshRetryBounds(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		attempts   int
		cancel     bool
	}{
		{name: "serialization", code: "40001", attempts: 4},
		{name: "deadlock", code: "40P01", attempts: 4},
		{name: "constraint", code: "23514", attempts: 1},
		{name: "cancellation", code: "40001", attempts: 1, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projector, token, pool := analyticsIdleRefreshFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			transactions := map[string]bool{}
			attempts := 0
			analytics.SetTokenInputsBeforeCommit(projector, func(ctx context.Context, tx pgx.Tx) error {
				attempts++
				var id, isolation string
				if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id()::text,current_setting('transaction_isolation')").Scan(&id, &isolation); err != nil {
					return err
				}
				if transactions[id] || isolation != "repeatable read" {
					t.Errorf("refresh transaction=%s isolation=%s reused=%t", id, isolation, transactions[id])
				}
				transactions[id] = true
				_, err := tx.Exec(ctx, fmt.Sprintf("DO $$ BEGIN RAISE EXCEPTION 'refresh transaction conflict' USING ERRCODE = '%s'; END $$", tc.code))
				if tc.cancel {
					cancel()
				}
				return err
			})
			err := projector.RefreshTokenInputs(ctx, token)
			var pgerr *pgconn.PgError
			if tc.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("refresh cancellation: %v", err)
				}
			} else if !errors.As(err, &pgerr) || pgerr.Code != tc.code {
				t.Fatalf("refresh SQLSTATE=%s: %v", tc.code, err)
			}
			if attempts != tc.attempts || len(transactions) != tc.attempts {
				t.Fatalf("refresh attempts=%d transactions=%d, want %d", attempts, len(transactions), tc.attempts)
			}
			var revision, outbox int
			if err := projector.Authority.Pool.QueryRow(context.Background(), "SELECT revision::integer,(SELECT count(*) FROM "+projector.Authority.Schema+".analytics_outbox WHERE pool=$1) FROM "+projector.Authority.Schema+".analytics_progress WHERE pool=$1", pool).Scan(&revision, &outbox); err != nil || revision != 1 || outbox != 2 {
				t.Fatalf("failed refresh changed progress or outbox: revision=%d outbox=%d: %v", revision, outbox, err)
			}
			for _, model := range []string{"pressure-v1", projector.Model.Version()} {
				result, err := projector.Authority.ReadSnapshot(context.Background(), pool, model, time.Now().Unix())
				if err != nil || result.Snapshot.Watermark.ProjectionRevision != "1" {
					t.Fatalf("failed refresh changed %s snapshot: %+v %v", model, result, err)
				}
			}
			analytics.SetTokenInputsBeforeCommit(projector, nil)
			if err := projector.RefreshTokenInputs(context.Background(), token); err != nil {
				t.Fatalf("refresh recovery: %v", err)
			}
		})
	}
}
