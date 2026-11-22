package internaltest

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestAnvilCommitsDistinctBlocksAndAdvancesTime(t *testing.T) {
	chain := NewAnvil(t)
	client := chain.Client()
	if client == nil || chain.URL == "" {
		t.Fatal("chain client or endpoint missing")
	}
	ctx := t.Context()
	id, err := client.ChainID(ctx)
	if err != nil || id.String() != "125" {
		t.Fatalf("chain identity: %v, %v", id, err)
	}
	before, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := chain.Commit()
	after, err := client.HeaderByHash(ctx, hash)
	if err != nil || hash == (common.Hash{}) || after.Number.Uint64() != before.Number.Uint64()+1 || after.ParentHash != before.Hash() {
		t.Fatalf("committed block: %v, %v", after, err)
	}
	if err := chain.AdjustTime(2 * time.Hour); err != nil {
		t.Fatal(err)
	}
	later, err := client.HeaderByNumber(ctx, nil)
	if err != nil || later.Number.Uint64() != after.Number.Uint64()+1 || later.ParentHash != after.Hash() || later.Time < after.Time+7200 {
		t.Fatalf("advanced chain time: %v, %v", later, err)
	}
}

func TestIndexerTableFixtureAppliesWriterSchema(t *testing.T) {
	db := NewPostgres(t)
	ApplyIndexerTables(t, db, "cursors", "swaps")
	ctx := t.Context()
	if _, err := db.Exec(ctx, `INSERT INTO indexer.cursors(chain_id,last_processed_block) VALUES(125,42)`); err != nil {
		t.Fatal(err)
	}
	var height int64
	if err := db.QueryRow(ctx, `SELECT last_processed_block FROM indexer.cursors WHERE chain_id=125`).Scan(&height); err != nil || height != 42 {
		t.Fatalf("indexed cursor: %d, %v", height, err)
	}
	var nullable string
	if err := db.QueryRow(ctx, `SELECT is_nullable FROM information_schema.columns WHERE table_schema='indexer' AND table_name='swaps' AND column_name='router'`).Scan(&nullable); err != nil || nullable != "YES" {
		t.Fatalf("swap router column: %q, %v", nullable, err)
	}
	var required int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='indexer' AND table_name='swaps' AND column_name IN ('block_number','log_index','tx_hash') AND is_nullable='NO'`).Scan(&required); err != nil || required != 3 {
		t.Fatalf("required swap position columns: %d, %v", required, err)
	}
}
