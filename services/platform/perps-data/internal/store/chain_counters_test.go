package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestChainCounters(t *testing.T) {
	insertRows := func(t *testing.T, db *sql.DB) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO perps.blocks (number, hash, parent_hash, time, proposer, tx_count)
SELECT n, decode(repeat(lpad(n::text, 2, '0'), 32), 'hex'), decode(repeat('00', 32), 'hex'),
    now(), repeat('AB', 20), 0 FROM generate_series(1, 2) AS n`); err != nil {
			t.Fatalf("insert two blocks: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO perps.tx_actions
    (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account, tx_from, action)
SELECT 2, decode(repeat('02', 32), 'hex'), n, 0, 0, decode(repeat(lpad(n::text, 2, '0'), 32), 'hex'),
    'tx', now(), decode(repeat('11', 20), 'hex'), 'action', decode(repeat('22', 20), 'hex'),
    decode(repeat('22', 20), 'hex'), 'place_order' FROM generate_series(0, 2) AS n`); err != nil {
			t.Fatalf("insert three tx_actions: %v", err)
		}
	}
	assertCounters := func(t *testing.T, db *sql.DB, wantBlocks, wantActions int64) {
		t.Helper()
		var blocks, blockRows, actions, actionRows, counters int64
		if err := db.QueryRow(`SELECT
    (SELECT value FROM perps.chain_counters WHERE name = 'blocks'),
    (SELECT count(*) FROM perps.blocks),
    (SELECT value FROM perps.chain_counters WHERE name = 'tx_actions'),
    (SELECT count(*) FROM perps.tx_actions),
    (SELECT count(*) FROM perps.chain_counters)`).Scan(&blocks, &blockRows, &actions, &actionRows, &counters); err != nil {
			t.Fatalf("read chain counters and table counts: %v", err)
		}
		if blocks != wantBlocks || blockRows != wantBlocks || actions != wantActions || actionRows != wantActions || counters != 2 {
			t.Fatalf("counters blocks=%d tx_actions=%d, table counts blocks=%d tx_actions=%d, counter rows=%d; want %d, %d, and exactly two counter rows",
				blocks, actions, blockRows, actionRows, counters, wantBlocks, wantActions)
		}
	}

	t.Run("inserts_and_deletes", func(t *testing.T) {
		db := testDB(t)
		migrate(t, db)
		assertCounters(t, db, 0, 0)
		insertRows(t, db)
		assertCounters(t, db, 2, 3)
		if _, err := db.Exec(`DELETE FROM perps.blocks WHERE number = 1`); err != nil {
			t.Fatalf("delete one block: %v", err)
		}
		assertCounters(t, db, 1, 3)
		if err := execRolledBack(t, db, `DELETE FROM perps.blocks; DELETE FROM perps.tx_actions`); err != nil {
			t.Fatalf("rolled-back deletes: %v", err)
		}
		assertCounters(t, db, 1, 3)
		if _, err := db.Exec(`DELETE FROM perps.tx_actions WHERE tx_index = 0`); err != nil {
			t.Fatalf("delete one tx_action: %v", err)
		}
		assertCounters(t, db, 1, 2)
		if _, err := db.Exec(`DELETE FROM perps.blocks; DELETE FROM perps.tx_actions`); err != nil {
			t.Fatalf("delete remaining rows: %v", err)
		}
		assertCounters(t, db, 0, 0)
		wantCheckViolation(t, execRolledBack(t, db, `UPDATE perps.chain_counters SET value = -1 WHERE name = 'blocks'`), "chain_counters_value_check")
		assertCounters(t, db, 0, 0)
	})

	t.Run("seeds_existing_rows", func(t *testing.T) {
		db := testDB(t)
		ctx := context.Background()
		if _, err := db.Exec(bootstrapSQL); err != nil {
			t.Fatalf("bootstrap migrations: %v", err)
		}
		files, err := embedded()
		if err != nil {
			t.Fatalf("read embedded migrations: %v", err)
		}
		for _, m := range files {
			if m.file >= "0013_chain_counters.sql" {
				break
			}
			if err := inTx(ctx, db, func(tx *sql.Tx) error { return apply(ctx, tx, m) }); err != nil {
				t.Fatalf("apply %s before chain counters: %v", m.file, err)
			}
		}
		insertRows(t, db)
		migrate(t, db)
		assertCounters(t, db, 2, 3)
		migrate(t, db)
		assertCounters(t, db, 2, 3)
		if _, err := db.Exec(`DELETE FROM perps.blocks WHERE number = 1`); err != nil {
			t.Fatalf("delete one seeded block: %v", err)
		}
		assertCounters(t, db, 1, 3)
	})
}
