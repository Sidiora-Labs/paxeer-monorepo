package coordinatortest

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func assertConnectedInterfaces(t *testing.T, db Database, admin *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	connect := func(t *testing.T, role string) *pgxpool.Pool {
		t.Helper()
		pool, err := pgxpool.New(ctx, db.Runtimes[role].DSN)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	t.Run("source lease holds its row lock", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'portfolio-source','indexer-owner',7,clock_timestamp()+interval '1 minute'),(125,'expired-source','expired-owner',8,clock_timestamp()-interval '1 minute')`); err != nil {
			t.Fatal(err)
		}
		pool := connect(t, "pnl_runtime")
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		var owner string
		var fence int64
		if err = tx.QueryRow(ctx, `SELECT owner_instance,fence_token FROM indexer.lock_ingest_lease_v1($1)`, "portfolio-source").Scan(&owner, &fence); err != nil || owner != "indexer-owner" || fence != 7 {
			t.Fatalf("source lease identity: %s %d: %v", owner, fence, err)
		}
		var expired int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM indexer.lock_ingest_lease_v1($1)`, "expired-source").Scan(&expired); err != nil || expired != 0 {
			t.Fatalf("expired source lease count: %d: %v", expired, err)
		}
		writer, err := admin.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback(ctx)
		if _, err = writer.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
			t.Fatal(err)
		}
		_, err = writer.Exec(ctx, `SELECT 1 FROM indexer.indexer_ingest_leases WHERE chain_id=125 AND stream_id='portfolio-source' FOR UPDATE`)
		var refused *pgconn.PgError
		if !errors.As(err, &refused) || refused.Code != "55P03" {
			t.Fatalf("source lease must hold its shared lock: %v", err)
		}
		if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_leases_v1 SET fence_token=9 WHERE stream_id='portfolio-source'`); !errors.As(err, &refused) || refused.Code != "42501" {
			t.Fatalf("portfolio cannot mutate source leases: %v", err)
		}
	})
	t.Run("operator journal appends through its owner", func(t *testing.T) {
		pool := connect(t, "indexer_runtime")
		if _, err := pool.Exec(ctx, `SELECT security.record_authorization_decision_v1($1,$2,$3,$4,$5)`, "operator", "inspect", "request", "policy", true); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := admin.QueryRow(ctx, `SELECT count(*) FROM security.authorization_decisions WHERE actor='operator' AND scope='inspect' AND request_id='request' AND policy_version='policy' AND allowed`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("operator decision journal: %d: %v", count, err)
		}
		var refused *pgconn.PgError
		if _, err := pool.Exec(ctx, `INSERT INTO security.authorization_decisions(actor,scope,request_id,policy_version,allowed) VALUES('operator','inspect','direct','policy',true)`); !errors.As(err, &refused) || refused.Code != "42501" {
			t.Fatalf("operator journal requires its append interface: %v", err)
		}
		if _, err := pool.Exec(ctx, `SELECT * FROM security.operator_sessions`); !errors.As(err, &refused) || refused.Code != "42501" {
			t.Fatalf("operator sessions require their read interface: %v", err)
		}
	})
}
