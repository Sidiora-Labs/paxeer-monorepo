package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"math/big"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lib/pq"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
)

func TestRepair468_Migration(t *testing.T) {
	db := testDB(t)
	var empty bool
	if err := db.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'perps')`).Scan(&empty); err != nil || !empty {
		t.Fatalf("fixture must start without schema perps: empty=%v, error=%v", empty, err)
	}
	migrate(t, db)
	files := embeddedFiles(t)
	const file = "0016_spot_board_omissions.sql"
	if !slices.Contains(files, file) {
		t.Fatalf("production embed lacks %s: %v", file, files)
	}
	before := records(t, db)
	if len(before) != len(files) {
		t.Fatalf("migration records=%d, embedded files=%d", len(before), len(files))
	}
	for i, r := range before {
		if r.file != files[i] || !bytes.Equal(r.sum, embeddedSum(t, r.file)) {
			t.Fatalf("migration record %s does not match embedded file %s", r.file, files[i])
		}
	}
	for _, table := range []struct {
		name, key string
		columns   []column
	}{
		{"spot_board_omissions", "account", []column{{"account", "bytea"}, {"updated_block", "bigint"}}},
		{"spot_board_generation", "singleton", []column{{"singleton", "boolean"}, {"generated_at", "timestamp with time zone"}, {"omitted_accounts", "bigint"}}},
	} {
		if got := columns(t, db, table.name); !slices.Equal(got, table.columns) {
			t.Fatalf("%s columns=%v, want %v", table.name, got, table.columns)
		}
		if got := primaryKey(t, db, table.name); got != table.key {
			t.Fatalf("%s primary key=%s, want %s", table.name, got, table.key)
		}
		if got := queryStrings(t, db, `SELECT column_name FROM information_schema.columns
WHERE table_schema = 'perps' AND table_name = $1 AND is_nullable <> 'NO'`, table.name); len(got) != 0 {
			t.Fatalf("%s nullable columns: %v", table.name, got)
		}
	}
	var omissions, generations int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM perps.spot_board_omissions),
(SELECT count(*) FROM perps.spot_board_generation)`).Scan(&omissions, &generations); err != nil || omissions != 0 || generations != 0 {
		t.Fatalf("migration must not invent metadata: omissions=%d generations=%d error=%v", omissions, generations, err)
	}
	const omission = `INSERT INTO perps.spot_board_omissions(account, updated_block) VALUES ($1, $2)`
	const generation = `INSERT INTO perps.spot_board_generation(singleton, generated_at, omitted_accounts) VALUES ($1, $2, $3)`
	account := common.HexToAddress("0x4681").Bytes()
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	for _, accepted := range []struct {
		query string
		args  []any
	}{
		{omission, []any{account, int64(0)}},
		{generation, []any{true, at, int64(0)}},
		{generation, []any{true, at, int64(1)}},
	} {
		if err := execRolledBack(t, db, accepted.query, accepted.args...); err != nil {
			t.Fatalf("valid metadata rejected: %v", err)
		}
	}
	for _, rejected := range []struct {
		name, query, code, constraint string
		args                          []any
	}{
		{"short_address", omission, "23514", "spot_board_omissions_account_check", []any{account[:19], int64(1)}},
		{"long_address", omission, "23514", "spot_board_omissions_account_check", []any{append(slices.Clone(account), 0), int64(1)}},
		{"null_account", omission, "23502", "", []any{nil, int64(1)}},
		{"null_block", omission, "23502", "", []any{account, nil}},
		{"false_singleton", generation, "23514", "spot_board_generation_singleton_check", []any{false, at, int64(0)}},
		{"null_singleton", generation, "23502", "", []any{nil, at, int64(0)}},
		{"null_timestamp", generation, "23502", "", []any{true, nil, int64(0)}},
		{"null_count", generation, "23502", "", []any{true, at, nil}},
		{"negative_count", generation, "23514", "spot_board_generation_omitted_accounts_check", []any{true, at, int64(-1)}},
		{"duplicate_account", `INSERT INTO perps.spot_board_omissions VALUES ($1, 1), ($1, 2)`, "23505", "spot_board_omissions_pkey", []any{account}},
		{"duplicate_generation", `INSERT INTO perps.spot_board_generation VALUES (true, $1, 0), (true, $1, 1)`, "23505", "spot_board_generation_pkey", []any{at}},
	} {
		t.Run(rejected.name, func(t *testing.T) {
			err := execRolledBack(t, db, rejected.query, rejected.args...)
			var pg *pq.Error
			if !errors.As(err, &pg) || string(pg.Code) != rejected.code || pg.Constraint != rejected.constraint {
				t.Fatalf("got %v, want SQLSTATE %s constraint %q", err, rejected.code, rejected.constraint)
			}
		})
	}
	migrate(t, db)
	if after := records(t, db); !reflect.DeepEqual(before, after) {
		t.Fatalf("second migration changed records: before=%v after=%v", before, after)
	}
}

// Write committed typed facts through the same writer, projections and board
// fold used by the indexer; no fixture-only DDL or ranking inserts are used.
func repair468Write(t *testing.T, db *sql.DB, row decode.Row, event string) {
	t.Helper()
	f := row.Common()
	b := Block{Number: f.Block, Hash: f.BlockHash, Time: f.BlockTime, Proposer: strings.Repeat("AB", 20), TxCount: 1}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO perps.blocks(number, hash, parent_hash, time, proposer, tx_count)
VALUES ($1, $2, $3, $4, $5, $6)`, b.Number, b.Hash.Bytes(), b.ParentHash.Bytes(), b.Time, b.Proposer, b.TxCount); err != nil {
		t.Fatal(err)
	}
	result := &decode.Result{Decoded: []decode.Decoded{{Event: decode.Event{EventName: event}, Rows: []decode.Row{row}}}}
	if err := WriteBlock(context.Background(), tx, b, result); err != nil {
		t.Fatalf("WriteBlock at %d: %v", b.Number, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestRepair468_SpotBoard(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	ctx := context.Background()
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	good, refused, token := common.HexToAddress("0x4681"), common.HexToAddress("0x4682"), common.HexToAddress("0x468f")
	fact := func(n uint64, kind string, account common.Address) decode.Fact {
		hash := common.BigToHash(new(big.Int).SetUint64(n))
		return decode.Fact{Block: n, BlockHash: hash, TxHash: hash, BlockTime: at.Add(time.Duration(n) * time.Second),
			ReceiptKind: decode.ReceiptTx, Emitter: decode.SpotPoolAddress, Kind: kind, Account: account, TxFrom: &account}
	}
	repair468Write(t, db, decode.SpotTokenRegisteredRow{Fact: fact(10, "token_registered", good), Token: USDL, Decimals: 18, IsStablecoin: true}, "TokenRegistered")
	swap := func(n uint64, account common.Address, buy bool) {
		t.Helper()
		repair468Write(t, db, decode.SpotMarketOrderRow{Fact: fact(n, "market_order", account), User: account,
			Stablecoin: USDL, Token: token, StablecoinAmount: "100", TokenAmount: "10", IsBuy: buy}, "MarketOrderExecuted")
	}
	swap(11, good, true)
	swap(12, refused, false) // Selling without an attributed basis must persist an omission.
	swap(13, refused, false) // Exercise the omission upsert as well as its insert.
	swap(14, good, true)     // An unrelated update must retain the refusal.
	assertBoard := func() {
		t.Helper()
		var block, count int64
		var generated time.Time
		if err := db.QueryRow(`SELECT updated_block FROM perps.spot_board_omissions WHERE account = $1`, refused.Bytes()).Scan(&block); err != nil || block != 13 {
			t.Fatalf("persisted refusal block=%d, want 13: %v", block, err)
		}
		if err := db.QueryRow(`SELECT generated_at, omitted_accounts FROM perps.spot_board_generation WHERE singleton`).Scan(&generated, &count); err != nil || generated.IsZero() || count != 1 {
			t.Fatalf("published generation=%v omissions=%d, want timestamp and 1: %v", generated, count, err)
		}
		var volume string
		if err := db.QueryRow(`SELECT value::text FROM perps.spot_leaderboard WHERE kind = 'volume' AND account = $1`, good.Bytes()).Scan(&volume); err != nil || volume != "200" {
			t.Fatalf("production volume=%s, want 200: %v", volume, err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM perps.spot_leaderboard WHERE account = $1`, refused.Bytes()).Scan(&count); err != nil || count != 0 {
			t.Fatalf("refused account has %d ranks: %v", count, err)
		}
	}
	assertBoard()
	// The retained 828 fixture file must be safe to execute repeatedly after
	// production migrations and must preserve all published board data.
	fixture, err := os.ReadFile("../rest/repair_m828_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := func() []string {
		t.Helper()
		return queryStrings(t, db, `SELECT 'omission:' || row_to_json(o)::text FROM perps.spot_board_omissions o
UNION ALL SELECT 'generation:' || row_to_json(g)::text FROM perps.spot_board_generation g
UNION ALL SELECT 'rank:' || row_to_json(r)::text FROM perps.spot_leaderboard r
UNION ALL SELECT 'legs:' || row_to_json(l)::text FROM perps.spot_account_legs l ORDER BY 1`)
	}
	beforeRecords, beforeData := records(t, db), snapshot()
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(string(fixture)); err != nil {
			t.Fatalf("828 compatibility check: %v", err)
		}
	}
	migrate(t, db)
	if !reflect.DeepEqual(beforeRecords, records(t, db)) || !slices.Equal(beforeData, snapshot()) {
		t.Fatal("second migrator run or 828 compatibility check changed migration records or board data")
	}
	assertBoard()
	// Recovery reads the omission-account union and deletes a refusal whose
	// trade lies above the retained height, then republishes the generation.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := rebuildSpotBoard(ctx, tx, 11); err != nil {
		t.Fatalf("read and rebuild persisted board: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var omissions, count int64
	var volume string
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM perps.spot_board_omissions),
(SELECT omitted_accounts FROM perps.spot_board_generation WHERE singleton),
(SELECT value::text FROM perps.spot_leaderboard WHERE kind = 'volume' AND account = $1)`, good.Bytes()).Scan(&omissions, &count, &volume); err != nil || omissions != 0 || count != 0 || volume != "100" {
		t.Fatalf("recovered omissions=%d generation count=%d volume=%s, want 0/0/100: %v", omissions, count, volume, err)
	}
}
