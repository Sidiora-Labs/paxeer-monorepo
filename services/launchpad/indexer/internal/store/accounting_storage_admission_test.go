package store

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/jackc/pgx/v5"
)

type transactionOwner struct {
	sqlcdb.DBTX
	owner interface {
		Begin(context.Context) (pgx.Tx, error)
	}
}

func (s transactionOwner) Begin(ctx context.Context) (pgx.Tx, error) { return s.owner.Begin(ctx) }

func TestAccountingStorageAndSnapshotAdmissionPreserveCanonicalState(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	fill := accountingLifecycleFill()
	fence, err := st.AcquireLease(ctx, 125, "accounting-admission", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'accounting-admission',100,100)`); err != nil {
		t.Fatal(err)
	}
	b := CanonicalBatch{ChainID: 125, StreamID: "accounting-admission", Owner: "worker", Fence: fence, BatchID: "source", PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}}
	if err = st.CommitCanonical(ctx, b, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"raw unavailable", `ALTER TABLE indexer.indexer_raw_logs RENAME TO raw_unavailable`},
		{"fill unavailable", `ALTER TABLE indexer.indexer_accounting_fills RENAME TO fills_unavailable`},
		{"fill unacknowledged", `CREATE FUNCTION pg_temp.suppress_fill() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NULL; END'; CREATE TRIGGER suppress_fill BEFORE INSERT ON indexer.indexer_accounting_fills FOR EACH ROW EXECUTE FUNCTION pg_temp.suppress_fill()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, e := pool.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			if _, e = tx.Exec(ctx, tc.sql); e != nil {
				t.Fatal(e)
			}
			if digest, e := (CanonicalTx{Tx: tx, Store: st}).InsertAccountingFill(ctx, fill); e == nil || digest != "" {
				t.Fatalf("unacknowledged accounting fill %s: %v", digest, e)
			}
		})
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	owned := NewFromDBTX(transactionOwner{DBTX: conn.Conn(), owner: conn.Conn()})
	if matched, err := owned.ReconcileCanonicalCommit(ctx, b); err != nil || !matched {
		t.Fatalf("owned snapshot %v: %v", matched, err)
	}
	outer, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Rollback(ctx)
	nested := NewFromDBTX(transactionOwner{DBTX: outer, owner: outer})
	if matched, err := nested.ReconcileCanonicalCommit(ctx, b); err == nil || matched {
		t.Fatalf("nested isolation admission %v: %v", matched, err)
	}
	if err = outer.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_fills`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unacknowledged fill rows %d: %v", count, err)
	}
}
