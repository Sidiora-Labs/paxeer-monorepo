package devreads_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/devreads"
	inventory "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

// This committed, in-memory relational peer speaks the PostgreSQL simple-query
// protocol. The production Store writes its tables and the production Reader
// queries them through pgx. Each repeatable-read connection owns a frozen view;
// another connection can publish a fold while the first is paused at its cursor.
type repair1032Tables map[string]map[string]map[string]string

type repair1032Read struct {
	query       string
	transaction int
	repeatable  bool
}

type repair1032DB struct {
	mu              sync.Mutex
	tables          repair1032Tables
	reads           []repair1032Read
	nextTransaction int
	atCursor        chan struct{}
	releaseCursor   chan struct{}
}

func repair1032Copy(tables repair1032Tables) repair1032Tables {
	out := repair1032Tables{}
	for table, rows := range tables {
		out[table] = map[string]map[string]string{}
		for key, row := range rows {
			out[table][key] = map[string]string{}
			for column, value := range row {
				out[table][key][column] = value
			}
		}
	}
	return out
}

var repair1032Insert = regexp.MustCompile(`(?is)^INSERT INTO ([a-z_.]+)\s*\(([^)]+)\)\s*VALUES\s*\(([^)]+)\)`)
var repair1032Literal = regexp.MustCompile(`^'(?:[^']|'')*'$`)
var repair1032User = regexp.MustCompile(`(?:p\.|t\.)?user_address\s*=\s*'([^']*)'`)
var repair1032Pool = regexp.MustCompile(`pool_address\s*=\s*'([^']*)'`)

func repair1032Write(tables repair1032Tables, sql string) (string, error) {
	match := repair1032Insert.FindStringSubmatch(sql)
	if match == nil {
		return "", fmt.Errorf("unsupported write: %s", sql)
	}
	columns, values := strings.Split(match[2], ","), strings.Split(match[3], ",")
	if len(columns) != len(values) {
		return "", fmt.Errorf("column/value mismatch: %s", sql)
	}
	row := map[string]string{}
	for i, col := range columns {
		value := strings.TrimSpace(values[i])
		if repair1032Literal.MatchString(value) {
			value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		}
		row[strings.TrimSpace(col)] = value
	}
	var key string
	switch match[1] {
	case "pnl.user_trades":
		key = row["id"]
	case "pnl.user_positions":
		key = row["user_address"] + ":" + row["pool_address"]
	case "indexer.pools":
		key = row["pool_address"]
	case "indexer.cursors":
		key = row["chain_id"]
	default:
		return "", fmt.Errorf("table outside peer schema: %s", match[1])
	}
	if _, found := tables[match[1]][key]; found && strings.Contains(sql, "DO NOTHING") {
		return "INSERT 0 0", nil
	}
	tables[match[1]][key] = row
	return "INSERT 0 1", nil
}

func repair1032Select(tables repair1032Tables, sql string) ([]string, [][]string, error) {
	user := ""
	if m := repair1032User.FindStringSubmatch(sql); m != nil {
		user = m[1]
	}
	var table string
	var columns []string
	switch {
	case strings.Contains(sql, "SELECT pg_advisory_xact_lock("):
		return []string{"lock"}, [][]string{{""}}, nil
	case strings.Contains(sql, "SELECT last_processed_block FROM indexer.cursors"):
		row, found := tables["indexer.cursors"]["125"]
		if !found {
			return []string{"last_processed_block"}, nil, nil
		}
		return []string{"last_processed_block"}, [][]string{{row["last_processed_block"]}}, nil
	case strings.Contains(sql, "SELECT COALESCE(max(block_number), 0)"):
		var max int64
		for _, row := range tables["pnl.user_trades"] {
			n, _ := strconv.ParseInt(row["block_number"], 10, 64)
			if row["user_address"] == user && n > max {
				max = n
			}
		}
		return []string{"block_number"}, [][]string{{strconv.FormatInt(max, 10)}}, nil
	case strings.Contains(sql, "FROM pnl.user_positions p LEFT JOIN"):
		table = "pnl.user_positions"
		columns = []string{"resolved_token", "total_usdl_spent", "total_usdl_received", "realized_pnl_usdl", "current_holdings", "trade_count"}
	case strings.Contains(sql, "FROM pnl.user_trades t LEFT JOIN"):
		table = "pnl.user_trades"
		columns = []string{"id", "pool_address", "resolved_token", "is_buy", "usdl_amount", "token_amount", "block_number", "block_timestamp"}
	case strings.Contains(sql, "FROM pnl.user_positions WHERE"):
		table = "pnl.user_positions"
		columns = []string{"token_address", "total_usdl_spent", "total_tokens_bought", "total_usdl_received", "total_tokens_sold", "avg_cost_basis", "current_holdings", "realized_pnl_usdl", "first_buy_ts", "last_trade_ts", "trade_count"}
	default:
		return nil, nil, fmt.Errorf("query outside peer schema: %s", sql)
	}
	rows := []map[string]string{}
	for _, row := range tables[table] {
		if row["user_address"] != user {
			continue
		}
		if m := repair1032Pool.FindStringSubmatch(sql); m != nil && row["pool_address"] != m[1] {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if table == "pnl.user_trades" {
			a, _ := strconv.ParseInt(rows[i]["block_number"], 10, 64)
			b, _ := strconv.ParseInt(rows[j]["block_number"], 10, 64)
			if a != b {
				return a < b
			}
			return rows[i]["id"] < rows[j]["id"]
		}
		return rows[i]["pool_address"] < rows[j]["pool_address"]
	})
	out := [][]string{}
	for _, row := range rows {
		values := []string{}
		for _, col := range columns {
			value := row[col]
			if col == "resolved_token" {
				value = row["token_address"]
				if value == "" {
					value = tables["indexer.pools"][row["pool_address"]]["token_address"]
				}
				value = strings.ToLower(value)
			}
			values = append(values, value)
		}
		out = append(out, values)
	}
	return columns, out, nil
}

func (d *repair1032DB) serve(conn net.Conn) {
	defer conn.Close()
	b := pgproto3.NewBackend(conn, conn)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if b.Flush() != nil {
		return
	}
	var view repair1032Tables
	var readOnly, repeatable bool
	var transaction int
	status := byte('I')
	for {
		msg, err := b.Receive()
		if err != nil {
			return
		}
		if _, ok := msg.(*pgproto3.Terminate); ok {
			return
		}
		query, ok := msg.(*pgproto3.Query)
		if !ok {
			return
		}
		sql := strings.Join(strings.Fields(query.String), " ")
		tag := ""
		var columns []string
		var rows [][]string
		var queryErr error
		d.mu.Lock()
		tables := d.tables
		if view != nil {
			tables = view
		}
		var pause, release chan struct{}
		switch {
		case strings.HasPrefix(sql, "begin"):
			readOnly = strings.Contains(sql, "read only")
			repeatable = strings.Contains(sql, "repeatable read")
			view = repair1032Copy(d.tables)
			d.nextTransaction++
			transaction = d.nextTransaction
			status, tag = 'T', "BEGIN"
		case sql == "commit":
			if view != nil && !readOnly {
				d.tables = view
			}
			view = nil
			status, tag = 'I', "COMMIT"
		case sql == "rollback":
			view = nil
			status, tag = 'I', "ROLLBACK"
		case strings.HasPrefix(sql, "SAVEPOINT "):
			tag = "SAVEPOINT"
		case strings.HasPrefix(sql, "INSERT "):
			if readOnly && view != nil {
				queryErr = fmt.Errorf("write in read-only transaction")
			} else {
				tag, queryErr = repair1032Write(tables, sql)
			}
		case strings.HasPrefix(sql, "SELECT "):
			d.reads = append(d.reads, repair1032Read{sql, transaction, repeatable && readOnly && view != nil})
			columns, rows, queryErr = repair1032Select(tables, sql)
			tag = fmt.Sprintf("SELECT %d", len(rows))
			if strings.Contains(sql, "SELECT last_processed_block FROM indexer.cursors") && d.atCursor != nil {
				pause, release = d.atCursor, d.releaseCursor
				d.atCursor = nil
			}
		default:
			queryErr = fmt.Errorf("unsupported peer statement: %s", sql)
		}
		d.mu.Unlock()
		if pause != nil {
			close(pause)
			<-release
		}
		if queryErr != nil {
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: queryErr.Error()})
		} else {
			if len(columns) > 0 {
				fields := make([]pgproto3.FieldDescription, len(columns))
				for i, col := range columns {
					oid := uint32(25)
					switch col {
					case "block_number", "block_timestamp", "last_processed_block", "trade_count", "first_buy_ts", "last_trade_ts":
						oid = 20
					case "is_buy":
						oid = 16
					}
					fields[i] = pgproto3.FieldDescription{Name: []byte(col), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1}
				}
				b.Send(&pgproto3.RowDescription{Fields: fields})
				for _, row := range rows {
					values := make([][]byte, len(row))
					for i, value := range row {
						if value != "null" {
							values[i] = []byte(value)
						}
					}
					b.Send(&pgproto3.DataRow{Values: values})
				}
			}
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
		}
		b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		if b.Flush() != nil {
			return
		}
	}
}

func repair1032Peer(t *testing.T) (*repair1032DB, *pgxpool.Pool) {
	t.Helper()
	d := &repair1032DB{tables: repair1032Tables{}}
	for _, table := range []string{"pnl.user_positions", "pnl.user_trades", "indexer.pools", "indexer.cursors"} {
		d.tables[table] = map[string]map[string]string{}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() { defer wg.Done(); d.serve(conn) }()
		}
	}()
	config, err := pgxpool.ParseConfig("postgres://fixture@" + listener.Addr().String() + "/devreads?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.ConnConfig.ConnectTimeout = time.Second
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); listener.Close(); wg.Wait() })
	return d, pool
}

func repair1032Chain(t *testing.T) *devreads.Chain {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if req.Method != "eth_getBlockByNumber" || len(req.Params) != 2 {
			t.Errorf("unexpected RPC: %+v", req)
			w.WriteHeader(400)
			return
		}
		var tag string
		if err := json.Unmarshal(req.Params[0], &tag); err != nil {
			t.Error(err)
			return
		}
		height := uint64(20)
		if tag != "latest" {
			var err error
			height, err = strconv.ParseUint(strings.TrimPrefix(tag, "0x"), 16, 64)
			if err != nil {
				t.Error(err)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"number": fmt.Sprintf("0x%x", height), "hash": fmt.Sprintf("0x%064x", height), "timestamp": fmt.Sprintf("0x%x", int64(1_700_000_000)+int64(height)*12)}})
	}))
	t.Cleanup(server.Close)
	chain, err := devreads.DialChain(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(chain.Close)
	return chain
}

func repair1032Exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func repair1032Fold(t *testing.T, pool *pgxpool.Pool, sale bool) {
	t.Helper()
	in := store.TradeInput{ID: "0xbuy-0", UserAddress: trader, PoolAddress: poolA, IsBuy: true, UsdlAmount: "10000000", TokenAmount: "2000000000000000000", BlockNumber: 10, BlockTimestamp: 1_700_000_000, TxHash: "0x" + strings.Repeat("a", 64)}
	if sale {
		in.ID, in.IsBuy, in.UsdlAmount, in.TokenAmount, in.BlockNumber, in.BlockTimestamp = "0xsale-1", false, "8000000", "1000000000000000000", 11, 1_700_003_600
	}
	inserted, err := store.New(pool).FoldTrade(context.Background(), in)
	if err != nil || !inserted {
		t.Fatalf("real Store.FoldTrade: inserted=%v err=%v", inserted, err)
	}
}

func TestRepairM_1032_LatePoolMetadataChangesSnapshotIdentity(t *testing.T) {
	_, pool := repair1032Peer(t)
	repair1032Fold(t, pool, false)
	repair1032Exec(t, pool, `INSERT INTO indexer.cursors (chain_id, last_processed_block) VALUES (125, 10)`)
	reader := devreads.Reader{Pool: pool, Chain: repair1032Chain(t)}
	snapshot := func() devreads.Snapshot {
		t.Helper()
		out, err := reader.Snapshot(context.Background(), trader)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := snapshot()
	if len(before.Positions) != 0 || before.Revision != "2" || before.Watermark.Height != "10" {
		t.Fatalf("unresolved projection: %+v", before)
	}
	if again := snapshot(); again.ID != before.ID {
		t.Fatal("unchanged inputs changed snapshot identity")
	}
	repair1032Exec(t, pool, `INSERT INTO indexer.pools (pool_address, token_address) VALUES ($1,$2)`, poolA, tokenA)
	after := snapshot()
	if len(after.Positions) != 1 || after.Positions[0].TokenAddress != tokenA || after.Positions[0].KnownRemainingCostUsdlRaw != "10000000" {
		t.Fatalf("late metadata did not project real folded position: %+v", after)
	}
	if before.Revision != after.Revision || before.Watermark != after.Watermark {
		t.Fatal("fixture changed trade count or source watermark during metadata arrival")
	}
	if before.ID == after.ID {
		t.Fatal("finding 14: late pool metadata reused snapshot identity")
	}
	if again := snapshot(); again.ID != after.ID {
		t.Fatal("resolved projection identity is unstable")
	}
	repair1032Exec(t, pool, `INSERT INTO indexer.pools (pool_address, token_address) VALUES ($1,$2) ON CONFLICT (pool_address) DO UPDATE SET token_address = EXCLUDED.token_address`, poolA, tokenB)
	replaced := snapshot()
	if replaced.Positions[0].TokenAddress != tokenB || replaced.ID == after.ID {
		t.Fatal("resolved token identity is absent from snapshot identity")
	}
	repair1032Exec(t, pool, `INSERT INTO indexer.cursors (chain_id, last_processed_block) VALUES (125, 12) ON CONFLICT (chain_id) DO UPDATE SET last_processed_block = EXCLUDED.last_processed_block`)
	advanced := snapshot()
	if advanced.Watermark.Height != "12" || advanced.Revision != replaced.Revision || advanced.ID == replaced.ID {
		t.Fatal("source watermark is absent from snapshot identity")
	}
	router := chi.NewRouter()
	devreads.Register(router, devreads.Deps{Reader: reader, CursorSecret: "repair-m1032"})
	var body portfolioBody
	mustContract(t, "ConnectedPortfolioV3Response", get(t, router, "/v3/wallets/"+trader+"/portfolio", ""), &body)
	if body.Data.Summary.SnapshotID != advanced.ID || len(body.Data.Positions) != 1 || body.Data.Positions[0].TokenAddress != tokenB || body.Data.Summary.Watermark.Height != "12" {
		t.Fatalf("HTTP consumer differs from projection: %+v", body)
	}
}

func TestRepairM_1032_ConcurrentFoldStaysWithinHistoryWatermark(t *testing.T) {
	for _, route := range []string{"/v3/me/pnl/history", "/v2/me/portfolio/history"} {
		t.Run(route, func(t *testing.T) {
			d, pool := repair1032Peer(t)
			repair1032Fold(t, pool, false)
			repair1032Exec(t, pool, `INSERT INTO indexer.pools (pool_address, token_address) VALUES ($1,$2)`, poolA, tokenA)
			repair1032Exec(t, pool, `INSERT INTO indexer.cursors (chain_id, last_processed_block) VALUES (125, 10)`)
			reader := devreads.Reader{Pool: pool, Chain: repair1032Chain(t)}
			before, err := reader.Snapshot(context.Background(), trader)
			if err != nil {
				t.Fatal(err)
			}
			router := chi.NewRouter()
			devreads.Register(router, devreads.Deps{Reader: reader, Identity: auth.HTTPIdentityAuthority{Endpoint: identityGateway(t), ServiceToken: serviceToken, AllowPlainHTTP: true}, CursorSecret: "repair-m1032"})
			from := time.Unix(1_700_000_000, 0).UTC().Truncate(time.Hour)
			query := url.Values{"from": {from.Format("2006-01-02T15:04:05.000Z")}, "to": {from.Add(3 * time.Hour).Format("2006-01-02T15:04:05.000Z")}, "interval": {"1h"}, "limit": {"3"}}
			at, release := make(chan struct{}), make(chan struct{})
			d.mu.Lock()
			d.atCursor, d.releaseCursor = at, release
			d.mu.Unlock()
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			result := make(chan *httptest.ResponseRecorder, 1)
			go func() { result <- get(t, router, route+"?"+query.Encode(), traderBearer) }()
			select {
			case <-at:
			case <-time.After(2 * time.Second):
				t.Fatal("history did not reach source cursor")
			}
			repair1032Fold(t, pool, true)
			repair1032Exec(t, pool, `INSERT INTO indexer.cursors (chain_id, last_processed_block) VALUES (125, 11) ON CONFLICT (chain_id) DO UPDATE SET last_processed_block = EXCLUDED.last_processed_block`)
			unblock()
			var rec *httptest.ResponseRecorder
			select {
			case rec = <-result:
			case <-time.After(4 * time.Second):
				t.Fatal("history request did not finish")
			}
			if route == "/v3/me/pnl/history" {
				var series inventory.IntervalSeries
				mustContract(t, "ConnectedL16IntervalSeries", rec, &series)
				if series.SnapshotID != before.ID || series.Watermark != before.Watermark || len(series.Entries) != 3 {
					t.Fatalf("history anchor: %+v", series)
				}
				if series.Entries[0].TradeCount != 1 || series.Entries[0].SaleCount != 0 || series.Entries[1].TradeCount != 0 || series.Entries[1].SaleCount != 0 || series.Entries[1].KnownNetGainUsdlRaw != "0" {
					t.Fatalf("finding 15: concurrent sale entered earlier interval: %+v", series.Entries)
				}
			} else {
				var history struct {
					Data struct {
						Items []struct {
							Watermark l17.Watermark `json:"watermark"`
							Realized  string        `json:"knownRealizedSubtotalUsdlRaw"`
							Time      string        `json:"time"`
						} `json:"items"`
					} `json:"data"`
					Watermark l17.Watermark `json:"watermark"`
					Revision  string        `json:"revision"`
				}
				mustContract(t, "PnlHistoryResponse", rec, &history)
				if history.Watermark != before.Watermark || history.Revision != before.Revision || len(history.Data.Items) != 1 || history.Data.Items[0].Realized != "0" || history.Data.Items[0].Watermark != before.Watermark || history.Data.Items[0].Time != time.Unix(1_700_000_000, 0).UTC().Format("2006-01-02T15:04:05.000Z") {
					t.Fatalf("finding 15: concurrent sale entered earlier value history: %+v", history)
				}
			}
			d.mu.Lock()
			reads := append([]repair1032Read(nil), d.reads...)
			d.mu.Unlock()
			var positionTx, cursorTx, replayTx int
			for _, read := range reads {
				if strings.Contains(read.query, "FROM pnl.user_positions p LEFT JOIN") {
					positionTx = read.transaction
				}
				if strings.Contains(read.query, "SELECT last_processed_block FROM indexer.cursors") {
					cursorTx = read.transaction
				}
				if strings.Contains(read.query, "FROM pnl.user_trades t LEFT JOIN") {
					if !read.repeatable {
						t.Fatal("trade replay did not use a read-only repeatable-read view")
					}
					replayTx = read.transaction
				}
			}
			if replayTx == 0 || replayTx != positionTx || replayTx != cursorTx {
				t.Fatalf("projection/cursor/replay transactions differ: %d/%d/%d", positionTx, cursorTx, replayTx)
			}
			after, err := reader.Snapshot(context.Background(), trader)
			if err != nil {
				t.Fatal(err)
			}
			if after.Watermark.Height != "11" || after.Revision != "3" || after.ID == before.ID || len(after.Positions) != 1 || after.Positions[0].KnownRemainingCostUsdlRaw != "5000000" {
				t.Fatalf("concurrent production fold was not committed: %+v", after)
			}
			again := get(t, router, route+"?"+query.Encode(), traderBearer)
			if route == "/v3/me/pnl/history" {
				var series inventory.IntervalSeries
				mustContract(t, "ConnectedL16IntervalSeries", again, &series)
				if series.SnapshotID != after.ID || series.Watermark != after.Watermark || series.Entries[1].SaleCount != 1 || series.Entries[1].KnownNetGainUsdlRaw != "3000000" {
					t.Fatalf("next interval omitted committed fold: %+v", series)
				}
			} else {
				var history struct {
					Data struct {
						Items []struct {
							Realized string `json:"knownRealizedSubtotalUsdlRaw"`
						} `json:"items"`
					} `json:"data"`
					Watermark l17.Watermark `json:"watermark"`
				}
				mustContract(t, "PnlHistoryResponse", again, &history)
				if history.Watermark != after.Watermark || len(history.Data.Items) != 2 || history.Data.Items[1].Realized != "3000000" {
					t.Fatalf("next value history omitted committed fold: %+v", history)
				}
			}
		})
	}
}
