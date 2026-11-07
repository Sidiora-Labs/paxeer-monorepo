package store_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/lib/pq"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

var (
	repair21A     = common.HexToAddress("0x0000000000000000000000000000000000000021")
	repair21B     = common.HexToAddress("0x0000000000000000000000000000000000000022")
	repair21C     = common.HexToAddress("0x0000000000000000000000000000000000000023")
	repair21Token = common.HexToAddress("0x00000000000000000000000000000000000000fa")
	repair21Time  = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
)

type repair21DB struct {
	db  *sql.DB
	dsn string
}

func repair21Database(t *testing.T) *repair21DB {
	t.Helper()
	raw := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := store.Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "repair21_" + hex.EncodeToString(suffix[:])
	if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := store.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	out := &repair21DB{db: db, dsn: u.String()}
	t.Cleanup(func() {
		if err := out.db.Close(); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(`DROP DATABASE ` + pq.QuoteIdentifier(name) + ` WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return out
}

func (d *repair21DB) reopen(t *testing.T) {
	t.Helper()
	if err := d.db.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	d.db, err = store.Open(d.dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background(), d.db); err != nil {
		t.Fatal(err)
	}
}

func repair21Tx(t *testing.T, db *sql.DB, work func(*sql.Tx)) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	work(tx)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func repair21Exec(t *testing.T, tx *sql.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func repair21Header(n uint64) store.Block {
	return store.Block{Number: n, Hash: crypto.Keccak256Hash([]byte(fmt.Sprint("repair21/", n))),
		ParentHash: crypto.Keccak256Hash([]byte(fmt.Sprint("repair21/", n-1))),
		Time:       repair21Time.Add(time.Duration(n) * time.Minute), Proposer: strings.Repeat("AB", 20), TxCount: 1}
}

func repair21Registry(t *testing.T) *decode.Registry {
	t.Helper()
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{PerpsModule: decode.ModuleAddress("perps"),
		SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed")})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func repair21Event(t *testing.T, load func() (abi.ABI, error), name string, emitter common.Address, values ...any) decode.Log {
	t.Helper()
	if emitter == decode.PerpsAddress {
		topics, data, err := perpsabi.PackEvent(name, values...)
		if err != nil {
			t.Fatal(err)
		}
		return decode.Log{Log: ethtypes.Log{Address: emitter, Topics: topics, Data: data}, ReceiptKind: decode.ReceiptTx, TxFrom: &repair21A}
	}
	a, err := load()
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := a.Events[name]
	if !ok || len(values) != len(ev.Inputs) {
		t.Fatalf("event %s arguments", name)
	}
	topics := []common.Hash{ev.ID}
	var data []any
	for i, input := range ev.Inputs {
		if input.Indexed {
			words, err := abi.MakeTopics([]any{values[i]})
			if err != nil {
				t.Fatal(err)
			}
			topics = append(topics, words[0][0])
		} else {
			data = append(data, values[i])
		}
	}
	packed, err := ev.Inputs.NonIndexed().Pack(data...)
	if err != nil {
		t.Fatal(err)
	}
	return decode.Log{Log: ethtypes.Log{Address: emitter, Topics: topics, Data: packed}, ReceiptKind: decode.ReceiptTx, TxFrom: &repair21A}
}

func repair21Trade(t *testing.T, account common.Address, buy bool, stable, tokens int64) decode.Log {
	return repair21Event(t, spotabi.PECOR, "MarketOrderExecuted", decode.SpotPoolAddress,
		account, store.USDL, repair21Token, big.NewInt(stable), big.NewInt(tokens), buy)
}

func repair21Decode(t *testing.T, tx *sql.Tx, registry *decode.Registry, b store.Block, firstLog uint, logs []decode.Log) *decode.Result {
	t.Helper()
	logs = slices.Clone(logs)
	for i := range logs {
		logs[i].BlockNumber, logs[i].BlockHash, logs[i].BlockTime = b.Number, b.Hash, b.Time
		logs[i].Index, logs[i].TxHash = firstLog+uint(i), crypto.Keccak256Hash(b.Hash.Bytes())
	}
	rows, err := registry.Decode(context.Background(), tx, logs)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Raw) != 0 || len(rows.Decoded) != len(logs) {
		t.Fatalf("decoder did not admit every event: decoded %d raw %d logs %d", len(rows.Decoded), len(rows.Raw), len(logs))
	}
	return rows
}

func repair21Write(t *testing.T, db *sql.DB, registry *decode.Registry, height uint64, logs ...decode.Log) {
	t.Helper()
	repair21WriteAt(t, db, registry, height, 0, logs...)
}

func repair21WriteAt(t *testing.T, db *sql.DB, registry *decode.Registry, height uint64, firstLog uint, logs ...decode.Log) {
	t.Helper()
	repair21Tx(t, db, func(tx *sql.Tx) {
		b := repair21Header(height)
		repair21Exec(t, tx, `INSERT INTO perps.blocks(number, hash, parent_hash, time, proposer, tx_count) VALUES ($1,$2,$3,$4,$5,$6)`,
			b.Number, b.Hash.Bytes(), b.ParentHash.Bytes(), b.Time, b.Proposer, b.TxCount)
		rows := repair21Decode(t, tx, registry, b, firstLog, logs)
		if err := store.WriteBlock(context.Background(), tx, b, rows); err != nil {
			t.Fatal(err)
		}
	})
}

func repair21Outbox(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT event_id, encode(envelope, 'hex') FROM perps.delivery_outbox`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, wire string
		if err := rows.Scan(&id, &wire); err != nil {
			t.Fatal(err)
		}
		out[id] = wire
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func repair21RetainedOutbox(t *testing.T, db *sql.DB, before map[string]string) {
	t.Helper()
	after := repair21Outbox(t, db)
	if len(after) <= len(before) {
		t.Fatal("replay did not enqueue newly discovered facts")
	}
	for id, wire := range before {
		if after[id] != wire {
			t.Fatalf("replay changed immutable delivery %s", id)
		}
	}
}

// Delete exactly the block-keyed rows the replay/follower consumers discard.
func repair21Delete(t *testing.T, tx *sql.Tx, from, to int64, blocks bool) {
	t.Helper()
	rows, err := tx.Query(`SELECT c.table_name FROM information_schema.columns c
JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
WHERE c.table_schema = 'perps' AND c.column_name = 'block' AND t.table_type = 'BASE TABLE' ORDER BY c.table_name`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range tables {
		repair21Exec(t, tx, `DELETE FROM perps.`+pq.QuoteIdentifier(name)+` WHERE block BETWEEN $1 AND $2`, from, to)
	}
	if blocks {
		repair21Exec(t, tx, `DELETE FROM perps.blocks WHERE number BETWEEN $1 AND $2`, from, to)
	}
}

type repair21Rank struct {
	Rank    int    `json:"rank"`
	Address string `json:"address"`
	Value   string `json:"value"`
}

func repair21Boards(t *testing.T, db *sql.DB) map[string][]repair21Rank {
	t.Helper()
	// Leaderboard requests use only the database; the real node client stays idle.
	node, err := ethclient.Dial("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	users, err := rest.NewSpotUsers(db, node)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	users.Register(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	out := map[string][]repair21Rank{}
	for _, kind := range []string{"pnl", "volume", "win-rate"} {
		response, err := server.Client().Get(server.URL + "/v1/leaderboards/" + kind)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Kind  string         `json:"kind"`
			Items []repair21Rank `json:"items"`
		}
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK || body.Kind != kind {
			t.Fatalf("%s response = %d %+v", kind, response.StatusCode, body)
		}
		out[kind] = body.Items
	}
	return out
}

func TestRepair_21_SpotReplayReopen(t *testing.T) {
	ctx := context.Background()
	indexed, fresh := repair21Database(t), repair21Database(t)
	registry := repair21Registry(t)
	initial := []decode.Log{
		repair21Event(t, spotabi.Vault, "TokenRegistered", decode.SpotVaultAddress, store.USDL, uint8(18), true),
		repair21Event(t, spotabi.Vault, "TokenRegistered", decode.SpotVaultAddress, repair21Token, uint8(18), false),
		repair21Trade(t, repair21A, true, 100, 10), repair21Trade(t, repair21B, true, 200, 10),
	}
	corrected := []decode.Log{repair21Trade(t, repair21A, false, 50, 10), repair21Trade(t, repair21B, false, 300, 10)}
	for _, db := range []*sql.DB{indexed.db, fresh.db} {
		repair21Write(t, db, registry, 10, initial...)
	}
	repair21Write(t, indexed.db, registry, 11, repair21Trade(t, repair21A, false, 120, 10),
		repair21Trade(t, repair21B, false, 300, 10), repair21Trade(t, repair21C, true, 500, 10), repair21Trade(t, repair21C, false, 600, 10))
	// Replay discovers previously unindexed source logs; an existing delivery
	// identity cannot carry a different payload. Earlier misattributed facts
	// disappear while the newly admitted logs keep their own source positions.
	repair21WriteAt(t, fresh.db, registry, 11, 4, corrected...)
	for _, db := range []*sql.DB{indexed.db, fresh.db} {
		repair21Write(t, db, registry, 12, repair21Trade(t, repair21A, true, 10, 1), repair21Trade(t, repair21B, true, 10, 1))
	}
	before, want := repair21Boards(t, indexed.db), repair21Boards(t, fresh.db)
	spotTables := []string{"spot_account_legs", "spot_leaderboard"}
	wantState := repair21ProjectionState(t, fresh.db, spotTables)
	if reflect.DeepEqual(before, want) {
		t.Fatal("correction must change the served boards")
	}
	outbox := repair21Outbox(t, indexed.db)
	repair21Tx(t, indexed.db, func(tx *sql.Tx) {
		old, err := store.CaptureRecovery(ctx, tx, 11, 11)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(old.Accounts, repair21C) {
			t.Fatal("capture lost removed-only account")
		}
		repair21Delete(t, tx, 11, 11, false)
		b := repair21Header(11)
		rows := repair21Decode(t, tx, registry, b, 4, corrected)
		if _, err := store.WriteFacts(ctx, tx, b, rows); err != nil {
			t.Fatal(err)
		}
		if err := store.Rebuild(ctx, tx, 11, 11, old); err != nil {
			t.Fatal(err)
		}
	})
	indexed.reopen(t)
	repair21RetainedOutbox(t, indexed.db, outbox)
	got := repair21Boards(t, indexed.db)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("replay/reopen differs from live fold: got %+v want %+v", got, want)
	}
	if state := repair21ProjectionState(t, indexed.db, spotTables); !slices.Equal(state, wantState) {
		t.Fatalf("spot projections differ from live fold\n got %q\nwant %q", state, wantState)
	}
	for _, board := range got {
		if len(board) != 2 {
			t.Fatalf("removed account still ranked: %+v", board)
		}
		for _, rank := range board {
			if rank.Address == strings.ToLower(repair21C.Hex()) {
				t.Fatal("removed account served")
			}
		}
	}
	for kind, values := range map[string][]string{"pnl": {"100", "-50"}, "volume": {"510", "160"}, "win-rate": {"100.0000", "0.0000"}} {
		for i, value := range values {
			if got[kind][i].Rank != i+1 || got[kind][i].Value != value {
				t.Fatalf("%s rank %d: %+v", kind, i+1, got[kind][i])
			}
		}
	}
	var legs int
	if err := indexed.db.QueryRow(`SELECT count(*) FROM perps.spot_account_legs WHERE account = $1`, repair21C.Bytes()).Scan(&legs); err != nil {
		t.Fatal(err)
	}
	if legs != 0 {
		t.Fatal("removed-only account legs survived")
	}
	// Repeating recovery is deterministic and can still fold past the replay range.
	repair21Tx(t, indexed.db, func(tx *sql.Tx) {
		if err := store.Rebuild(ctx, tx, 11, 11); err != nil {
			t.Fatal(err)
		}
	})
	if got := repair21Boards(t, indexed.db); !reflect.DeepEqual(got, want) {
		t.Fatalf("repeat recovery: %+v", got)
	}
	repair21Tx(t, indexed.db, func(tx *sql.Tx) {
		old, err := store.CaptureRecovery(ctx, tx, 10, 12)
		if err != nil {
			t.Fatal(err)
		}
		repair21Delete(t, tx, 10, 12, true)
		if err := store.Rebuild(ctx, tx, 10, 12, old); err != nil {
			t.Fatal(err)
		}
	})
	indexed.reopen(t)
	for _, board := range repair21Boards(t, indexed.db) {
		if len(board) != 0 {
			t.Fatalf("spot ranking survived empty head: %+v", board)
		}
	}
	for _, state := range repair21ProjectionState(t, indexed.db, spotTables) {
		if !strings.HasSuffix(state, ":") {
			t.Fatalf("spot projection survived empty head: %s", state)
		}
	}
}

func repair21AccountState(t *testing.T, db *sql.DB) []string {
	t.Helper()
	return repair21ProjectionState(t, db, []string{"positions_current", "orders", "account_margin", "user_stats", "user_pnl_daily", "market_stats_rolling", "leaderboard"})
}

func repair21ProjectionState(t *testing.T, db *sql.DB, tables []string) []string {
	t.Helper()
	var out []string
	for _, table := range tables {
		var state string
		if err := db.QueryRow(`SELECT coalesce(string_agg(to_jsonb(p)::text, ',' ORDER BY to_jsonb(p)::text), '') FROM perps.` + table + ` p`).Scan(&state); err != nil {
			t.Fatal(err)
		}
		out = append(out, table+":"+state)
	}
	return out
}

func TestRepair_21_RewindReopen(t *testing.T) {
	ctx := context.Background()
	db := repair21Database(t)
	registry := repair21Registry(t)
	perps := func(name string, values ...any) decode.Log {
		return repair21Event(t, perpsabi.ABI, name, decode.PerpsAddress, values...)
	}
	repair21Write(t, db.db, registry, 20,
		perps("CollateralDeposited", repair21A, big.NewInt(1000), big.NewInt(1000)),
		perps("PositionOpened", big.NewInt(1), repair21A, big.NewInt(0), true, big.NewInt(1000), big.NewInt(10), big.NewInt(60000), store.USDL, big.NewInt(100)))
	want := repair21AccountState(t, db.db)
	repair21Write(t, db.db, registry, 21,
		perps("PositionModified", big.NewInt(1), big.NewInt(800), big.NewInt(80000000000000), big.NewInt(80)),
		perps("CollateralDeposited", repair21A, big.NewInt(500), big.NewInt(1500)),
		perps("CollateralDeposited", repair21B, big.NewInt(200), big.NewInt(200)),
		perps("PositionOpened", big.NewInt(2), repair21B, big.NewInt(9), true, big.NewInt(200), big.NewInt(2), big.NewInt(500), store.USDL, big.NewInt(100)))
	if got := repair21AccountState(t, db.db); slices.Equal(got, want) {
		t.Fatal("fork did not modify projections")
	}
	repair21Tx(t, db.db, func(tx *sql.Tx) {
		old, err := store.CaptureRecovery(ctx, tx, 21, 21)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(old.Accounts, repair21A) || !slices.Contains(old.Accounts, repair21B) || !slices.Contains(old.Markets, "0") || !slices.Contains(old.Markets, "9") || !old.FromTime.Equal(repair21Header(21).Time) {
			t.Fatalf("incomplete recovery keys: %+v", old)
		}
		repair21Delete(t, tx, 21, 21, true)
		// The follower clears these projections before invoking its rebuild callback.
		repair21Exec(t, tx, `DELETE FROM perps.positions_current WHERE opened_block >= 21 OR closed_block >= 21`)
		repair21Exec(t, tx, `DELETE FROM perps.account_margin WHERE updated_block >= 21`)
		repair21Exec(t, tx, `DELETE FROM perps.user_stats WHERE updated_block >= 21`)
		if err := store.Rebuild(ctx, tx, 21, 21, old); err != nil {
			t.Fatal(err)
		}
	})
	db.reopen(t)
	if got := repair21AccountState(t, db.db); !slices.Equal(got, want) {
		t.Fatalf("rewind/reopen differs from pre-fork fold\n got %q\nwant %q", got, want)
	}
	// Losing the entire retained chain must clear keys rather than returning early.
	repair21Tx(t, db.db, func(tx *sql.Tx) {
		old, err := store.CaptureRecovery(ctx, tx, 20, 20)
		if err != nil {
			t.Fatal(err)
		}
		repair21Delete(t, tx, 20, 20, true)
		if err := store.Rebuild(ctx, tx, 20, 20, old); err != nil {
			t.Fatal(err)
		}
	})
	db.reopen(t)
	for _, state := range repair21AccountState(t, db.db) {
		if !strings.HasSuffix(state, ":") {
			t.Fatalf("projection survived empty head: %s", state)
		}
	}
}

func TestRepair_21_ReplayKeyUnionReopen(t *testing.T) {
	ctx := context.Background()
	indexed, fresh := repair21Database(t), repair21Database(t)
	registry := repair21Registry(t)
	perps := func(name string, values ...any) decode.Log {
		return repair21Event(t, perpsabi.ABI, name, decode.PerpsAddress, values...)
	}
	opened := func(id int64, account common.Address, market int64) decode.Log {
		return perps("PositionOpened", big.NewInt(id), account, big.NewInt(market), true, big.NewInt(200), big.NewInt(2), big.NewInt(500), store.USDL, big.NewInt(100))
	}
	for _, db := range []*sql.DB{indexed.db, fresh.db} {
		repair21Write(t, db, registry, 30, perps("CollateralDeposited", repair21A, big.NewInt(1000), big.NewInt(1000)), opened(1, repair21A, 0))
	}
	repair21Write(t, indexed.db, registry, 31, perps("CollateralDeposited", repair21B, big.NewInt(200), big.NewInt(200)), opened(2, repair21B, 9))
	replacement := []decode.Log{perps("CollateralDeposited", repair21C, big.NewInt(300), big.NewInt(300)), opened(3, repair21C, 0)}
	repair21WriteAt(t, fresh.db, registry, 31, 2, replacement...)
	// A surviving event beyond the corrected range must also be folded.
	for _, db := range []*sql.DB{indexed.db, fresh.db} {
		repair21Write(t, db, registry, 32, perps("PositionModified", big.NewInt(1), big.NewInt(150), big.NewInt(90000000000000), big.NewInt(90)))
	}
	want := repair21AccountState(t, fresh.db)
	outbox := repair21Outbox(t, indexed.db)
	repair21Tx(t, indexed.db, func(tx *sql.Tx) {
		old, err := store.CaptureRecovery(ctx, tx, 31, 31)
		if err != nil {
			t.Fatal(err)
		}
		repair21Delete(t, tx, 31, 31, false)
		b := repair21Header(31)
		rows := repair21Decode(t, tx, registry, b, 2, replacement)
		if _, err := store.WriteFacts(ctx, tx, b, rows); err != nil {
			t.Fatal(err)
		}
		if err := store.Rebuild(ctx, tx, 31, 31, old); err != nil {
			t.Fatal(err)
		}
	})
	indexed.reopen(t)
	repair21RetainedOutbox(t, indexed.db, outbox)
	if got := repair21AccountState(t, indexed.db); !slices.Equal(got, want) {
		t.Fatalf("replacement key union differs from live fold\n got %q\nwant %q", got, want)
	}
}
