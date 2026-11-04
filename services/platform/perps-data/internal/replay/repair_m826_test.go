package replay

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
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/lib/pq"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

var (
	repair826A     = common.HexToAddress("0x8261")
	repair826B     = common.HexToAddress("0x8262")
	repair826C     = common.HexToAddress("0x8263")
	repair826D     = common.HexToAddress("0x8264")
	repair826Token = common.HexToAddress("0x826f")
)

func TestRepairM_826_ProjectionInventory(t *testing.T) {
	want, got := slices.Clone(expectedProjectionTables), slices.Clone(projectionTables)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("reported projections %v, want independent inventory %v", got, want)
	}
	for name, want := range map[string]string{
		"spot_account_legs": `SELECT * FROM perps."spot_account_legs" WHERE account IN (SELECT key FROM replay_spot_accounts)`,
		"spot_leaderboard":  `SELECT * FROM perps."spot_leaderboard"`,
	} {
		got, err := projectionQuery(name)
		if err != nil || got != want {
			t.Errorf("%s scope = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := projectionQuery("unregistered_projection"); err == nil {
		t.Fatal("unregistered projection accepted without an explicit scope")
	}
}

// This source is an in-memory transcript of ABI-encoded chain events. Both
// indexing and replay consume it through the production registry and writer.
type repair826Transcript struct {
	registry *decode.Registry
	blocks   map[uint64]store.Block
	logs     map[uint64][]decode.Log
}

func (s *repair826Transcript) Read(ctx context.Context, q decode.Querier, height uint64) (store.Block, *decode.Result, error) {
	b, ok := s.blocks[height]
	if !ok {
		return store.Block{}, nil, fmt.Errorf("unrecorded height %d", height)
	}
	rows, err := s.registry.DecodeBlock(ctx, q, decode.BlockInput{
		Number: height, Hash: b.Hash, Time: b.Time, Logs: s.logs[height],
	})
	if err == nil && (len(rows.Raw) != 0 || len(rows.Decoded) != len(s.logs[height])) {
		return b, rows, fmt.Errorf("transcript height %d: decoded %d raw %d, want %d decoded events", height, len(rows.Decoded), len(rows.Raw), len(s.logs[height]))
	}
	return b, rows, err
}

func repair826Registry(t *testing.T) *decode.Registry {
	t.Helper()
	metrics, err := telemetry.New(config.Replay)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"),
		PricefeedModule: decode.ModuleAddress("pricefeed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func repair826Event(t *testing.T, load func() (abi.ABI, error), name string, emitter common.Address, values ...any) decode.Log {
	t.Helper()
	var topics []common.Hash
	var data []byte
	var err error
	if emitter == decode.PerpsAddress {
		topics, data, err = perpsabi.PackEvent(name, values...)
	} else {
		var contract abi.ABI
		contract, err = load()
		if err != nil {
			t.Fatal(err)
		}
		event, ok := contract.Events[name]
		if !ok || len(values) != len(event.Inputs) {
			t.Fatalf("invalid fixture event %s", name)
		}
		topics = []common.Hash{event.ID}
		var plain []any
		for i, input := range event.Inputs {
			if input.Indexed {
				words, err := abi.MakeTopics([]any{values[i]})
				if err != nil {
					t.Fatal(err)
				}
				topics = append(topics, words[0][0])
			} else {
				plain = append(plain, values[i])
			}
		}
		data, err = event.Inputs.NonIndexed().Pack(plain...)
	}
	if err != nil {
		t.Fatal(err)
	}
	return decode.Log{Log: ethtypes.Log{Address: emitter, Topics: topics, Data: data},
		ReceiptKind: decode.ReceiptTx, TxFrom: &repair826A}
}

func repair826Trade(t *testing.T, account common.Address, buy bool, stable, tokens int64) decode.Log {
	t.Helper()
	return repair826Event(t, spotabi.PECOR, "MarketOrderExecuted", decode.SpotPoolAddress,
		account, store.USDL, repair826Token, big.NewInt(stable), big.NewInt(tokens), buy)
}

func repair826Source(t *testing.T, variant string) *repair826Transcript {
	t.Helper()
	s := &repair826Transcript{registry: repair826Registry(t), blocks: map[uint64]store.Block{}, logs: map[uint64][]decode.Log{}}
	for h := uint64(1); h <= 3; h++ {
		s.blocks[h] = store.Block{Number: h, Hash: common.BigToHash(new(big.Int).SetUint64(h)),
			ParentHash: common.BigToHash(new(big.Int).SetUint64(h - 1)),
			Time:       time.Date(2026, 3, 2, 10, 0, int(h), 0, time.UTC), Proposer: strings.Repeat("AB", 20), TxCount: 1}
	}
	perps := func(name string, values ...any) decode.Log {
		return repair826Event(t, perpsabi.ABI, name, decode.PerpsAddress, values...)
	}
	s.logs[1] = []decode.Log{
		repair826Event(t, spotabi.Vault, "TokenRegistered", decode.SpotVaultAddress, store.USDL, uint8(18), true),
		repair826Event(t, spotabi.Vault, "TokenRegistered", decode.SpotVaultAddress, repair826Token, uint8(18), false),
		perps("CollateralDeposited", repair826A, big.NewInt(1000), big.NewInt(1000)),
		perps("PositionOpened", big.NewInt(1), repair826A, big.NewInt(0), true, big.NewInt(1000), big.NewInt(10), big.NewInt(60000), store.USDL, big.NewInt(100)),
		repair826Trade(t, repair826A, true, 100, 10),
		repair826Trade(t, repair826B, true, 200, 10), repair826Trade(t, repair826B, false, 300, 10),
		repair826Trade(t, repair826D, true, 50, 10), repair826Trade(t, repair826D, false, 75, 10),
	}
	s.logs[2] = []decode.Log{perps("PositionModified", big.NewInt(1), big.NewInt(800), big.NewInt(80000000000000), big.NewInt(80))}
	switch variant {
	case "original":
		s.logs[2] = append(s.logs[2], repair826Trade(t, repair826A, false, 120, 10),
			repair826Trade(t, repair826C, true, 500, 10), repair826Trade(t, repair826C, false, 600, 10))
	case "corrected":
		s.logs[2] = append(s.logs[2], repair826Trade(t, repair826A, false, 50, 10))
	case "removed":
	case "replacement_owner":
		s.logs[2] = append(s.logs[2], repair826Trade(t, repair826D, true, 10, 1))
	default:
		t.Fatalf("unknown transcript %s", variant)
	}
	s.logs[3] = []decode.Log{repair826Trade(t, repair826A, true, 10, 1)}
	for h, logs := range s.logs {
		for i := range logs {
			b := s.blocks[h]
			logs[i].BlockNumber, logs[i].BlockHash, logs[i].BlockTime = h, b.Hash, b.Time
			logs[i].Index, logs[i].TxHash = uint(i), common.BigToHash(new(big.Int).SetUint64(100+h))
			// Corrected facts are newly admitted source logs. Previously emitted
			// outbox identities must remain immutable when old facts disappear.
			if h == 2 && i > 0 && variant != "original" {
				logs[i].Index += 10
			}
		}
	}
	return s
}

type repair826Rank struct {
	Rank    int    `json:"rank"`
	Address string `json:"address"`
	Value   string `json:"value"`
}

func repair826Boards(t *testing.T, db *sql.DB) map[string][]repair826Rank {
	t.Helper()
	// These routes read only projections. An in-process node peer fails any
	// accidental chain read instead of allowing a hidden live dependency.
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("projected board unexpectedly queried the chain")
		http.Error(w, "unexpected chain read", http.StatusBadRequest)
	}))
	defer node.Close()
	client, err := ethclient.Dial(node.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	users, err := rest.NewSpotUsers(db, client)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	users.Register(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	out := map[string][]repair826Rank{}
	for _, kind := range []string{"pnl", "volume", "win-rate"} {
		response, err := server.Client().Get(server.URL + "/v1/leaderboards/" + kind)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Kind  string          `json:"kind"`
			Items []repair826Rank `json:"items"`
		}
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || body.Kind != kind {
			t.Fatalf("%s response: status %d body %+v error %v", kind, response.StatusCode, body, err)
		}
		out[kind] = body.Items
	}
	return out
}

func repair826ProjectionState(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	all := dump(t, db, 1, 3)
	out := map[string][]string{}
	for _, table := range expectedProjectionTables {
		out["perps."+table] = all["perps."+table]
	}
	out["perps.keeper_cycles"] = all["perps.keeper_cycles"]
	return out
}

// Production queries explicitly name perps, so isolate its schema in a unique
// database, as the store's repair21Database fixture does, rather than rewriting
// SQL or truncating the qualification database's shared schema.
func repair826Database(t *testing.T) (*sql.DB, string) {
	t.Helper()
	raw := requireEnv(t, testDatabaseEnv)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := store.Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(); err != nil {
			t.Error(err)
		}
	})
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "repair826_" + hex.EncodeToString(suffix[:])
	if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE ` + pq.QuoteIdentifier(name) + ` WITH (FORCE)`); err != nil {
			t.Error(err)
		}
	})
	u.Path = "/" + name
	u.RawPath = ""
	dsn := u.String()
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db, dsn
}

func repair826Reopen(t *testing.T, db *sql.DB, dsn string) *sql.DB {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	return reopened
}

func repair826Difference(before, after []string) int64 {
	counts := map[string]int64{}
	for _, row := range before {
		counts[row]++
	}
	for _, row := range after {
		counts[row]--
	}
	var changed int64
	for _, count := range counts {
		if count < 0 {
			changed -= count
		} else {
			changed += count
		}
	}
	return changed
}

func repair826DryRun(t *testing.T, variant string) {
	t.Helper()
	ctx := context.Background()
	db, dsn := repair826Database(t)
	original, replacement := repair826Source(t, "original"), repair826Source(t, variant)
	index(t, db, original, 1, 3)
	before := dump(t, db, 1, 3)
	beforeBoards := repair826Boards(t, db)
	if n := count(t, db, `SELECT count(*) FROM perps.positions_current WHERE position_id = 1 AND size_usd = 800`); n != 1 {
		t.Fatal("initial producer did not modify the existing position")
	}
	report, err := runner(db, replacement).Run(ctx, Request{From: 2, To: 2, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Request != (Request{From: 2, To: 2, DryRun: true}) {
		t.Fatalf("report request %+v", report.Request)
	}
	sameDump(t, before, dump(t, db, 1, 3))
	if got := repair826Boards(t, db); !reflect.DeepEqual(got, beforeBoards) {
		t.Fatalf("dry run persisted boards: %+v", got)
	}
	db = repair826Reopen(t, db, dsn)
	sameDump(t, before, dump(t, db, 1, 3))
	if _, err := runner(db, replacement).Run(ctx, Request{From: 2, To: 2}); err != nil {
		t.Fatal(err)
	}
	db = repair826Reopen(t, db, dsn)
	after := dump(t, db, 1, 3)
	gotState, gotBoards := repair826ProjectionState(t, db), repair826Boards(t, db)
	if got := cursorHeight(t, db); got != 3 {
		t.Fatalf("cursor %d after middle-height replay", got)
	}
	if n := count(t, db, `SELECT count(*) FROM perps.spot_account_legs WHERE account = $1`, repair826C.Bytes()); n != 0 {
		t.Fatal("removed-only trader retained account legs")
	}
	if n := count(t, db, `SELECT count(*) FROM perps.positions_current WHERE position_id = 1 AND size_usd = 800`); n != 1 {
		t.Fatal("replay lost the modified surviving position")
	}
	counts := map[string]TableCount{}
	for _, c := range report.Tables {
		counts[c.Table] = c
	}
	for _, table := range []string{"perps.spot_account_legs", "perps.spot_leaderboard"} {
		want := repair826Difference(before[table], after[table])
		got, ok := counts[table]
		if !ok || want == 0 || got.Changed != want || got.Rows != int64(len(after[table])) {
			t.Errorf("%s dry-run report %+v present=%v; want rows=%d changed=%d", table, got, ok, len(after[table]), want)
		}
	}
	var beforeD, afterD int
	for _, row := range beforeBoards["pnl"] {
		if row.Address == strings.ToLower(repair826D.Hex()) {
			beforeD = row.Rank
		}
	}
	for _, row := range gotBoards["pnl"] {
		if row.Address == strings.ToLower(repair826D.Hex()) {
			afterD = row.Rank
		}
	}
	if beforeD == 0 || afterD == 0 || beforeD == afterD {
		t.Fatalf("untouched account did not move: %d -> %d", beforeD, afterD)
	}
	// A fresh database state comes exclusively from the public WriteBlock path,
	// independently of the replay inventory and recovery implementation.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, freshDSN := repair826Database(t)
	index(t, fresh, replacement, 1, 3)
	fresh = repair826Reopen(t, fresh, freshDSN)
	sameDump(t, repair826ProjectionState(t, fresh), gotState)
	if want := repair826Boards(t, fresh); !reflect.DeepEqual(gotBoards, want) {
		t.Fatalf("replay boards %+v; fresh boards %+v", gotBoards, want)
	}
	if variant == "corrected" {
		for kind, values := range map[string][]string{"pnl": {"100", "25", "-50"}, "volume": {"500", "160", "125"}, "win-rate": {"100.0000", "100.0000", "0.0000"}} {
			if len(gotBoards[kind]) != len(values) {
				t.Fatalf("%s board: %+v", kind, gotBoards[kind])
			}
			for i, value := range values {
				if row := gotBoards[kind][i]; row.Rank != i+1 || row.Value != value {
					t.Errorf("%s rank %d: %+v; want %s", kind, i+1, row, value)
				}
			}
		}
	}
}

func TestRepairM_826_CorrectedSpotDryRun(t *testing.T)    { repair826DryRun(t, "corrected") }
func TestRepairM_826_RemovedSpotDryRun(t *testing.T)      { repair826DryRun(t, "removed") }
func TestRepairM_826_ReplacementOwnerDryRun(t *testing.T) { repair826DryRun(t, "replacement_owner") }

// Recorded RPC/Comet exchanges are committed peers, not an alternate follower.
func repair826RPC(t *testing.T) string {
	t.Helper()
	wire, err := os.ReadFile("../indexer/testdata/repair_209_rpc.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript []struct {
		Method string          `json:"method"`
		Params any             `json:"params"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(wire, &transcript); err != nil {
		t.Fatal(err)
	}
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			Params any             `json:"params"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid JSON", 400)
			return
		}
		for _, exchange := range transcript {
			if exchange.Method == request.Method && reflect.DeepEqual(exchange.Params, request.Params) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(struct {
					Version string          `json:"jsonrpc"`
					ID      json.RawMessage `json:"id"`
					Result  json.RawMessage `json:"result"`
				}{"2.0", request.ID, exchange.Result}); err != nil {
					t.Error(err)
				}
				return
			}
		}
		t.Errorf("unrecorded request %s %v", request.Method, request.Params)
		http.Error(w, "unrecorded request", 400)
	}))
	t.Cleanup(node.Close)
	return node.URL
}

func TestRepairM_826_FollowerRecoveryReopen(t *testing.T) {
	ctx := context.Background()
	db, dsn := repair826Database(t)
	src := repair826Source(t, "original")
	// The recorded node retains height 1 and serves a different height 2.
	b := src.blocks[2]
	b.Hash = common.HexToHash("0xf826")
	src.blocks[2] = b
	for i := range src.logs[2] {
		src.logs[2][i].BlockHash = b.Hash
	}
	index(t, db, src, 1, 1)
	wantState, wantBoards := repair826ProjectionState(t, db), repair826Boards(t, db)
	index(t, db, src, 2, 2)
	if n := count(t, db, `SELECT count(*) FROM perps.positions_current WHERE position_id = 1 AND size_usd = 800`); n != 1 {
		t.Fatal("fork did not modify the opening position")
	}
	url := repair826RPC(t)
	newFollower := func(db *sql.DB) (*source.Follower, *source.RPC) {
		rpc, err := source.NewRPC(ctx, source.RPCOptions{URLs: []string{url}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(rpc.Close)
		comet, err := source.NewComet([]string{url}, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		outbox, err := delivery.NewStore(db, "perps", store.Policy(125))
		if err != nil {
			t.Fatal(err)
		}
		follower, err := source.NewFollower(ctx, source.FollowerOptions{
			DB: db, RPC: rpc, Comet: comet, Outbox: outbox, ChainID: 125, StartHeight: 1,
			Write: func(ctx context.Context, tx *sql.Tx, b *source.Block) error {
				rows, err := src.registry.DecodeBlock(ctx, tx, decode.BlockInput{Number: b.Number, Hash: b.Hash, Time: b.Time, Logs: b.Logs, Events: b.FinalizeBlockEvents})
				if err != nil {
					return err
				}
				return store.WriteBlock(ctx, tx, store.Block{Number: b.Number, Hash: b.Hash, ParentHash: b.ParentHash, Time: b.Time, Proposer: b.Proposer, TxCount: b.TxCount}, rows)
			},
			Rebuild: func(ctx context.Context, tx *sql.Tx, a source.Affected) error {
				if a.From != 2 || !slices.Contains(a.Accounts, repair826A) || !slices.Contains(a.Accounts, repair826C) || !a.FromTime.Equal(b.Time) {
					t.Errorf("follower lost captured recovery scope: %+v", a)
				}
				return store.Rebuild(ctx, tx, int64(a.From), int64(a.From), store.Recovery{Accounts: a.Accounts, Markets: a.Markets, FromTime: a.FromTime})
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return follower, rpc
	}
	follower, rpc := newFollower(db)
	if n, err := follower.Step(ctx); err != nil || n != 0 {
		t.Fatalf("rewind step: blocks %d, error %v", n, err)
	}
	if got := cursorHeight(t, db); got != 1 {
		t.Fatalf("rewind cursor %d, want 1", got)
	}
	if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE subject = 'perps.125.chain.rewind'`); n != 1 {
		t.Fatalf("rewind envelopes %d", n)
	}
	rpc.Close()
	db = repair826Reopen(t, db, dsn)
	_, reopenedRPC := newFollower(db)
	defer reopenedRPC.Close()
	sameDump(t, wantState, repair826ProjectionState(t, db))
	if got := repair826Boards(t, db); !reflect.DeepEqual(got, wantBoards) {
		t.Fatalf("recovered boards %+v, want %+v", got, wantBoards)
	}
	if n := count(t, db, `SELECT count(*) FROM perps.positions_current WHERE position_id = 1 AND size_usd = 1000 AND collateral_amount = 100 AND opened_block = 1 AND status = 'open'`); n != 1 {
		t.Fatal("reopened follower did not recover the surviving opening position")
	}
	if n := count(t, db, `SELECT count(*) FROM perps.spot_account_legs WHERE account = $1`, repair826C.Bytes()); n != 0 {
		t.Fatal("fork-only spot trader survived restart")
	}
}
