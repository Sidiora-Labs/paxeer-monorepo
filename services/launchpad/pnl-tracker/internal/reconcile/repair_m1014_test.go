package reconcile_test

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/reconcile"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
)

// The peers speak PostgreSQL and RESP over owned connections. Store.FoldTrade,
// pgx transactions, pnlmath, Reconciler, position.Consumer and go-redis are the
// production implementations. Only storage/transport peers are fixtures; no
// accounting or refresh behavior is implemented by them.
type repair1014State struct {
	Swaps     []store.IndexerSwap
	Trades    map[string][][]byte
	Positions map[string][][]byte
	Cursor    store.Cursor
}
type repair1014DB struct {
	mu            sync.Mutex
	path          string
	state         repair1014State
	cursorFailure bool
}

func repair1014Swap(id, user string, index int64) store.IndexerSwap {
	return store.IndexerSwap{ID: id, Sender: user, PoolAddress: "0xpool", TokenAddress: "0xtoken", IsBuy: true,
		AmountIn: "1000000", AmountOut: oneToken, Price: "0", Fee: "0", BlockNumber: 100,
		BlockTimestamp: 100, LogIndex: index, TxHash: strings.TrimSuffix(id, "-0")}
}
func repair1014Open(t *testing.T, path string, swaps []store.IndexerSwap) (*repair1014DB, *store.Store) {
	t.Helper()
	db := &repair1014DB{path: path, state: repair1014State{Swaps: swaps, Trades: map[string][][]byte{}, Positions: map[string][][]byte{}, Cursor: store.Cursor{LastLogIndex: -1}}}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &db.state); err != nil {
			t.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig("postgres://fixture@localhost:1/pnl?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.LookupFunc = func(context.Context, string) ([]string, error) { return []string{"127.0.0.1"}, nil }
	cfg.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) {
		a, b := net.Pipe()
		go db.serve(t, b)
		return a, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return db, store.New(pool)
}
func repair1014Describe(oids []uint32) *pgproto3.RowDescription {
	d := &pgproto3.RowDescription{}
	for i, oid := range oids {
		d.Fields = append(d.Fields, pgproto3.FieldDescription{Name: []byte(fmt.Sprint(i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1})
	}
	return d
}
func repair1014OIDs(q string) []uint32 {
	switch {
	case strings.HasPrefix(q, "SELECT last_block"):
		return []uint32{20, 20, 20}
	case strings.Contains(q, "FROM indexer.swaps"):
		return []uint32{25, 25, 25, 25, 16, 25, 25, 25, 25, 20, 20, 20, 25}
	case strings.HasPrefix(q, "SELECT token_address, total_usdl_spent"):
		return []uint32{25, 25, 25, 25, 25, 25, 25, 25, 20, 20, 23}
	case strings.HasPrefix(q, "SELECT token_address FROM indexer.pools"):
		return []uint32{25}
	case strings.Contains(q, "FROM pnl.user_positions p LEFT JOIN"):
		return []uint32{25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 20, 20, 23, 25, 25, 25, 25}
	case strings.Contains(q, "FROM pnl.user_positions"):
		return []uint32{25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 20, 20, 23}
	case strings.Contains(q, "FROM pnl.user_trades"):
		return []uint32{25, 25, 25, 25, 16, 25, 25, 25, 25, 20, 20, 25}
	default:
		return nil
	}
}
func (db *repair1014DB) serve(t *testing.T, conn net.Conn) {
	defer conn.Close()
	b := pgproto3.NewBackend(conn, conn)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := b.Flush(); err != nil {
		return
	}
	var q string
	var args [][]byte
	var oids []uint32
	var tx *repair1014State
	status := byte('I')
	result := func(rows [][][]byte, tag string, err error) {
		if err != nil {
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "P0001", Message: err.Error()})
			return
		}
		for _, row := range rows {
			b.Send(&pgproto3.DataRow{Values: row})
		}
		if tag == "" {
			tag = fmt.Sprintf("SELECT %d", len(rows))
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	}
	for {
		msg, err := b.Receive()
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Parse:
			q = strings.Join(strings.Fields(m.Query), " ")
			oids = m.ParameterOIDs
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			args = make([][]byte, len(m.Parameters))
			for i, v := range m.Parameters {
				if v == nil {
					continue
				}
				format := int16(0)
				if len(m.ParameterFormatCodes) == 1 {
					format = m.ParameterFormatCodes[0]
				} else if i < len(m.ParameterFormatCodes) {
					format = m.ParameterFormatCodes[i]
				}
				args[i] = append([]byte(nil), v...)
				if format == 1 {
					switch oids[i] {
					case 20:
						args[i] = []byte(strconv.FormatInt(int64(binary.BigEndian.Uint64(v)), 10))
					case 23:
						args[i] = []byte(strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(v))), 10))
					case 16:
						args[i] = []byte(strconv.FormatBool(v[0] != 0))
					default:
						t.Errorf("unsupported binary PostgreSQL parameter oid=%d", oids[i])
						return
					}
				}
			}
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			if cols := repair1014OIDs(q); len(cols) > 0 {
				b.Send(repair1014Describe(cols))
			} else {
				b.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			rows, tag, e := db.query(q, args, tx)
			result(rows, tag, e)
		case *pgproto3.Query:
			switch strings.ToLower(strings.TrimSpace(m.String)) {
			case "begin":
				db.mu.Lock()
				raw, e := json.Marshal(db.state)
				tx = &repair1014State{}
				if e == nil {
					e = json.Unmarshal(raw, tx)
				}
				db.mu.Unlock()
				status = 'T'
				result(nil, "BEGIN", e)
			case "commit":
				db.mu.Lock()
				raw, e := json.Marshal(tx)
				if e == nil {
					e = os.WriteFile(db.path, raw, 0600)
				}
				if e == nil {
					db.state = *tx
				}
				db.mu.Unlock()
				tx = nil
				status = 'I'
				result(nil, "COMMIT", e)
			case "rollback":
				tx = nil
				status = 'I'
				result(nil, "ROLLBACK", nil)
			default:
				t.Errorf("unexpected PostgreSQL simple query %q", m.String)
				result(nil, "", fmt.Errorf("unsupported query"))
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Sync:
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Flush:
		case *pgproto3.Close:
			b.Send(&pgproto3.CloseComplete{})
		default:
			t.Errorf("unsupported PostgreSQL protocol message %T", m)
			return
		}
		switch msg.(type) {
		case *pgproto3.Sync, *pgproto3.Query, *pgproto3.Flush:
			if e := b.Flush(); e != nil {
				return
			}
		}
	}
}
func repair1014Bytes(values ...any) [][]byte {
	row := make([][]byte, len(values))
	for i, v := range values {
		if v != nil {
			row[i] = []byte(fmt.Sprint(v))
		}
	}
	return row
}
func (db *repair1014DB) query(q string, a [][]byte, tx *repair1014State) ([][][]byte, string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	s := tx
	if s == nil {
		s = &db.state
	}
	val := func(i int) string { return string(a[i]) }
	num := func(i int) int64 { n, _ := strconv.ParseInt(val(i), 10, 64); return n }
	switch {
	case strings.Contains(q, "pg_advisory_xact_lock"):
		return nil, "SELECT 1", nil
	case strings.HasPrefix(q, "SELECT last_block"):
		return [][][]byte{repair1014Bytes(s.Cursor.LastBlock, s.Cursor.LastLogIndex, s.Cursor.UpdatedAt)}, "", nil
	case strings.Contains(q, "FROM indexer.swaps"):
		var rows [][][]byte
		for _, sw := range s.Swaps {
			if (sw.BlockNumber > num(0) || sw.BlockNumber == num(0) && sw.LogIndex > num(1)) && int64(len(rows)) < num(2) {
				rows = append(rows, repair1014Bytes(sw.ID, sw.PoolAddress, sw.TokenAddress, sw.Sender, sw.IsBuy, sw.AmountIn, sw.AmountOut, sw.Fee, sw.Price, sw.BlockNumber, sw.BlockTimestamp, sw.LogIndex, sw.TxHash))
			}
		}
		return rows, "", nil
	case strings.HasPrefix(q, "UPDATE pnl.reconciler_cursor"):
		if db.cursorFailure {
			return nil, "", errors.New("fixture cursor write refused")
		}
		s.Cursor = store.Cursor{LastBlock: num(0), LastLogIndex: num(1), UpdatedAt: num(3)}
		raw, e := json.Marshal(s)
		if e == nil {
			e = os.WriteFile(db.path, raw, 0600)
		}
		return nil, "UPDATE 1", e
	case strings.HasPrefix(q, "INSERT INTO pnl.user_trades"):
		if tx == nil {
			return nil, "", errors.New("trade outside transaction")
		}
		if _, ok := s.Trades[val(0)]; ok {
			return nil, "INSERT 0 0", nil
		}
		s.Trades[val(0)] = a
		return nil, "INSERT 0 1", nil
	case strings.HasPrefix(q, "INSERT INTO pnl.user_positions"):
		if tx == nil {
			return nil, "", errors.New("position outside transaction")
		}
		s.Positions[val(0)+"|"+val(1)] = a
		return nil, "INSERT 0 1", nil
	case strings.HasPrefix(q, "SELECT token_address FROM indexer.pools"):
		return [][][]byte{repair1014Bytes("0xtoken")}, "", nil
	case strings.Contains(q, "FROM pnl.user_positions"):
		var rows [][][]byte
		for _, p := range s.Positions {
			if string(p[0]) != val(0) || len(a) > 1 && string(p[1]) != val(1) {
				continue
			}
			if strings.HasPrefix(q, "SELECT token_address,") {
				rows = append(rows, p[2:])
			} else if strings.Contains(q, "FROM pnl.user_positions p LEFT JOIN") {
				row := append([][]byte(nil), p...)
				row = append(row, nil, nil, nil, nil)
				rows = append(rows, row)
			} else {
				rows = append(rows, p)
			}
		}
		return rows, "", nil
	case strings.Contains(q, "FROM pnl.user_trades"):
		var rows [][][]byte
		for _, trade := range s.Trades {
			if string(trade[1]) == val(0) {
				rows = append(rows, trade)
			}
		}
		return rows, "", nil
	default:
		return nil, "", fmt.Errorf("unsupported storage transcript query: %s", q)
	}
}

// A small RESP2 peer with real key, WATCH, MULTI/EXEC and pub/sub semantics.
// Failure injection drops EXEC's connection before or after atomic application,
// covering both possible outcomes of an ambiguous Redis transaction response.
type repair1014Redis struct {
	mu                    sync.Mutex
	values                map[string]string
	versions              map[string]int
	subs                  map[string]map[*repair1014Conn]bool
	conns                 map[net.Conn]bool
	dropBefore, dropAfter bool
	published             int
	listener              net.Listener
}
type repair1014Conn struct {
	net.Conn
	mu sync.Mutex
}

func (c *repair1014Conn) send(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, e := io.WriteString(c.Conn, s)
	return e
}
func repair1014Bulk(s string) string { return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s) }
func repair1014Array(v ...string) string {
	s := fmt.Sprintf("*%d\r\n", len(v))
	for _, x := range v {
		s += repair1014Bulk(x)
	}
	return s
}
func repair1014Read(r *bufio.Reader) ([]string, error) {
	line, e := r.ReadString('\n')
	if e != nil {
		return nil, e
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("RESP command is not array: %q", line)
	}
	n, e := strconv.Atoi(strings.TrimSpace(line[1:]))
	if e != nil || n < 1 || n > 100 {
		return nil, fmt.Errorf("invalid RESP array")
	}
	args := make([]string, n)
	for i := range args {
		line, e = r.ReadString('\n')
		if e != nil {
			return nil, e
		}
		if !strings.HasPrefix(line, "$") {
			return nil, errors.New("expected bulk string")
		}
		size, e := strconv.Atoi(strings.TrimSpace(line[1:]))
		if e != nil || size < 0 || size > 1<<20 {
			return nil, errors.New("invalid bulk size")
		}
		raw := make([]byte, size+2)
		if _, e = io.ReadFull(r, raw); e != nil {
			return nil, e
		}
		args[i] = string(raw[:size])
	}
	return args, nil
}
func repair1014StartRedis(t *testing.T) *repair1014Redis {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s := &repair1014Redis{values: map[string]string{}, versions: map[string]int{}, subs: map[string]map[*repair1014Conn]bool{}, conns: map[net.Conn]bool{}, listener: l}
	t.Cleanup(func() {
		l.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		for c := range s.conns {
			c.Close()
		}
	})
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			s.mu.Lock()
			s.conns[c] = true
			s.mu.Unlock()
			go s.serve(c)
		}
	}()
	return s
}
func (s *repair1014Redis) client(t *testing.T) *goredis.Client {
	c := goredis.NewClient(&goredis.Options{Addr: s.listener.Addr().String(), Protocol: 2, MaxRetries: -1, DisableIdentity: true, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	t.Cleanup(func() { c.Close() })
	return c
}
func (s *repair1014Redis) serve(conn net.Conn) {
	c := &repair1014Conn{Conn: conn}
	defer func() {
		conn.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.conns, conn)
		for _, subs := range s.subs {
			delete(subs, c)
		}
	}()
	reader := bufio.NewReader(conn)
	watches := map[string]int{}
	var queued [][]string
	multi := false
	for {
		a, e := repair1014Read(reader)
		if e != nil {
			return
		}
		cmd := strings.ToLower(a[0])
		s.mu.Lock()
		if multi && cmd != "exec" && cmd != "discard" {
			queued = append(queued, a)
			s.mu.Unlock()
			if c.send("+QUEUED\r\n") != nil {
				return
			}
			continue
		}
		response := ""
		switch cmd {
		case "hello":
			response = "-ERR unknown command 'hello'\r\n"
		case "client":
			response = "+OK\r\n"
		case "watch":
			for _, k := range a[1:] {
				watches[k] = s.versions[k]
			}
			response = "+OK\r\n"
		case "unwatch":
			watches = map[string]int{}
			response = "+OK\r\n"
		case "multi":
			multi = true
			queued = nil
			response = "+OK\r\n"
		case "discard":
			multi = false
			queued = nil
			watches = map[string]int{}
			response = "+OK\r\n"
		case "exec":
			if s.dropBefore {
				s.dropBefore = false
				s.mu.Unlock()
				return
			}
			conflict := false
			for k, v := range watches {
				if s.versions[k] != v {
					conflict = true
				}
			}
			if conflict {
				response = "*-1\r\n"
			} else {
				response = fmt.Sprintf("*%d\r\n", len(queued))
				for _, q := range queued {
					response += s.apply(q)
				}
			}
			queued = nil
			multi = false
			watches = map[string]int{}
			if s.dropAfter {
				s.dropAfter = false
				s.mu.Unlock()
				return
			}
		case "subscribe":
			for i, ch := range a[1:] {
				if s.subs[ch] == nil {
					s.subs[ch] = map[*repair1014Conn]bool{}
				}
				s.subs[ch][c] = true
				response += fmt.Sprintf("*3\r\n%s%s:%d\r\n", repair1014Bulk("subscribe"), repair1014Bulk(ch), i+1)
			}
		case "unsubscribe":
			for _, ch := range a[1:] {
				delete(s.subs[ch], c)
				response += fmt.Sprintf("*3\r\n%s%s:0\r\n", repair1014Bulk("unsubscribe"), repair1014Bulk(ch))
			}
		default:
			response = s.apply(a)
		}
		s.mu.Unlock()
		if c.send(response) != nil {
			return
		}
	}
}
func (s *repair1014Redis) apply(a []string) string {
	switch strings.ToLower(a[0]) {
	case "ping":
		return "+PONG\r\n"
	case "get":
		if v, ok := s.values[a[1]]; ok {
			return repair1014Bulk(v)
		}
		return "$-1\r\n"
	case "set":
		s.values[a[1]] = a[2]
		s.versions[a[1]]++
		return "+OK\r\n"
	case "exists":
		n := 0
		for _, k := range a[1:] {
			if _, ok := s.values[k]; ok {
				n++
			}
		}
		return fmt.Sprintf(":%d\r\n", n)
	case "del":
		n := 0
		for _, k := range a[1:] {
			if _, ok := s.values[k]; ok {
				delete(s.values, k)
				s.versions[k]++
				n++
			}
		}
		return fmt.Sprintf(":%d\r\n", n)
	case "publish":
		s.published++
		for c := range s.subs[a[1]] {
			c.send(repair1014Array("message", a[1], a[2]))
		}
		return fmt.Sprintf(":%d\r\n", len(s.subs[a[1]]))
	default:
		return "-ERR unsupported fixture command " + a[0] + "\r\n"
	}
}

func repair1014Cache(t *testing.T, c *goredis.Client, user string) {
	t.Helper()
	for _, key := range []string{pnlcache.KeyPositions(user), pnlcache.KeyPortfolio(user)} {
		if e := c.Set(context.Background(), key, "stale", 0).Err(); e != nil {
			t.Fatal(e)
		}
	}
}
func repair1014AssertFresh(t *testing.T, c *goredis.Client, user string) {
	t.Helper()
	for _, key := range []string{pnlcache.KeyPositions(user), pnlcache.KeyPortfolio(user)} {
		if _, e := c.Get(context.Background(), key).Result(); !errors.Is(e, goredis.Nil) {
			t.Fatalf("stale cache %s remains: %v", key, e)
		}
	}
}
func repair1014AssertState(t *testing.T, st *store.Store, wantBlock, wantIndex int64) {
	t.Helper()
	ctx := context.Background()
	p, e := st.GetPosition(ctx, "0xuser", "0xpool")
	if e != nil || p == nil || p.TradeCount != 1 || p.TotalUsdlSpent != "1000000" || p.CurrentHoldings != oneToken || p.TokenAddress != "0xtoken" {
		t.Fatalf("position=%+v err=%v", p, e)
	}
	trades, e := st.ListTrades(ctx, "0xuser", store.TradeFilter{})
	if e != nil || len(trades) != 1 || trades[0].ID != "0xswap-0" {
		t.Fatalf("trades=%+v err=%v", trades, e)
	}
	poisonTrades, e := st.ListTrades(ctx, "0xother", store.TradeFilter{})
	if e != nil || len(poisonTrades) != 0 {
		t.Fatalf("poison trade escaped rollback: %+v %v", poisonTrades, e)
	}
	cur, e := st.GetCursor(ctx)
	if e != nil || cur.LastBlock != wantBlock || cur.LastLogIndex != wantIndex {
		t.Fatalf("cursor=%+v err=%v want=(%d,%d)", cur, e, wantBlock, wantIndex)
	}
}
func repair1014Subscribe(t *testing.T, c *goredis.Client) *goredis.PubSub {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sub := c.Subscribe(ctx, constants.ChannelPnlUpdate)
	t.Cleanup(func() { sub.Close() })
	if _, e := sub.Receive(ctx); e != nil {
		t.Fatal(e)
	}
	return sub
}
func repair1014Notification(t *testing.T, sub *goredis.PubSub, st *store.Store, c *goredis.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	m, e := sub.ReceiveMessage(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var signal map[string]string
	if e = json.Unmarshal([]byte(m.Payload), &signal); e != nil || m.Channel != constants.ChannelPnlUpdate || len(signal) != 1 || signal["userAddress"] != "0xuser" {
		t.Fatalf("refresh signal=%+v payload=%s err=%v", m, m.Payload, e)
	}
	router := chi.NewRouter()
	httpapi.RegisterReads(router, st, c)
	for _, route := range []string{"positions", "portfolio"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/users/"+signal["userAddress"]+"/"+route, nil)
		router.ServeHTTP(response, request)
		var body struct {
			User      string              `json:"user"`
			Positions []store.PositionRow `json:"positions"`
			Total     string              `json:"totalValueUsdl"`
		}
		if e = json.Unmarshal(response.Body.Bytes(), &body); e != nil || response.Code != http.StatusOK || body.User != "0xuser" || len(body.Positions) != 1 || body.Positions[0].TradeCount != 1 || body.Positions[0].TotalUsdlSpent != "1000000" || body.Positions[0].CurrentHoldings != oneToken {
			t.Fatalf("subscriber %s refresh status=%d body=%s err=%v", route, response.Code, response.Body.String(), e)
		}
		key := pnlcache.KeyPositions(body.User)
		if route == "portfolio" {
			key = pnlcache.KeyPortfolio(body.User)
			if body.Total != "0" {
				t.Fatalf("unpriced portfolio total=%q", body.Total)
			}
		}
		cached, e := c.Get(ctx, key).Result()
		if e != nil || cached != strings.TrimSpace(response.Body.String()) {
			t.Fatalf("refreshed cache %s=%q err=%v", key, cached, e)
		}
	}
}

func TestRepairM_1014_PoisonPrefixRecovery(t *testing.T) {
	ctx := context.Background()
	good := repair1014Swap("0xswap-0", "0xUsEr", 0)
	poison := repair1014Swap("0xpoison-1", "0xother", 1)
	poison.AmountIn = "poison"
	path := filepath.Join(t.TempDir(), "postgres.json")
	_, st := repair1014Open(t, path, []store.IndexerSwap{good, poison})
	peer := repair1014StartRedis(t)
	c := peer.client(t)
	sub := repair1014Subscribe(t, peer.client(t))
	repair1014Cache(t, c, "0xuser")
	repair1014Cache(t, c, "0xunrelated")
	worker := reconcile.New(st, c, discardLogger(), 500)
	n, e := worker.RunOnce(ctx)
	if n != 1 || e == nil || !strings.Contains(e.Error(), poison.ID) {
		t.Fatalf("prefix=%d err=%v", n, e)
	}
	repair1014AssertState(t, st, 100, 0)
	repair1014AssertFresh(t, c, "0xuser")
	repair1014Notification(t, sub, st, c)
	if v, e := c.Get(ctx, pnlcache.KeyPortfolio("0xunrelated")).Result(); e != nil || v != "stale" {
		t.Fatalf("unrelated user's cache changed: %q %v", v, e)
	}
	st.Pool().Close()
	c.Close()
	_, st = repair1014Open(t, path, nil)
	c = peer.client(t)
	worker = reconcile.New(st, c, discardLogger(), 500)
	n, e = worker.RunOnce(ctx)
	if n != 0 || e == nil || !strings.Contains(e.Error(), poison.ID) {
		t.Fatalf("reopened poison=%d err=%v", n, e)
	}
	repair1014AssertState(t, st, 100, 0)
	peer.mu.Lock()
	count := peer.published
	peer.mu.Unlock()
	if count != 1 {
		t.Fatalf("prefix notifications=%d", count)
	}
}

func TestRepairM_1014_CursorFailureReplayDeduplicates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "postgres.json")
	db, st := repair1014Open(t, path, []store.IndexerSwap{repair1014Swap("0xswap-0", "0xUsEr", 0)})
	db.cursorFailure = true
	peer := repair1014StartRedis(t)
	c := peer.client(t)
	sub := repair1014Subscribe(t, peer.client(t))
	repair1014Cache(t, c, "0xuser")
	n, e := reconcile.New(st, c, discardLogger(), 1).RunOnce(ctx)
	if n != 1 || e == nil || !strings.Contains(e.Error(), "cursor write refused") {
		t.Fatalf("cursor failure=%d %v", n, e)
	}
	repair1014AssertState(t, st, 0, -1)
	repair1014AssertFresh(t, c, "0xuser")
	repair1014Notification(t, sub, st, c)
	st.Pool().Close()
	c.Close()
	_, st = repair1014Open(t, path, nil)
	c = peer.client(t)
	repair1014Cache(t, c, "0xuser")
	n, e = reconcile.New(st, c, discardLogger(), 1).RunOnce(ctx)
	if n != 1 || e != nil {
		t.Fatalf("replay=%d %v", n, e)
	}
	repair1014AssertState(t, st, 100, 0)
	repair1014AssertFresh(t, c, "0xuser")
	peer.mu.Lock()
	count := peer.published
	peer.mu.Unlock()
	if count != 1 {
		t.Fatalf("replay published duplicate signals: %d", count)
	}
}

func TestRepairM_1014_RestartRefreshObligation(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis_committed_%v", after), func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "postgres.json")
			_, st := repair1014Open(t, path, []store.IndexerSwap{repair1014Swap("0xswap-0", "0xUsEr", 0)})
			peer := repair1014StartRedis(t)
			c := peer.client(t)
			sub := repair1014Subscribe(t, peer.client(t))
			repair1014Cache(t, c, "0xuser")
			peer.mu.Lock()
			peer.dropBefore = !after
			peer.dropAfter = after
			peer.mu.Unlock()
			n, e := reconcile.New(st, c, discardLogger(), 500).RunOnce(ctx)
			if n != 1 || e == nil {
				t.Fatalf("interrupted refresh=%d %v", n, e)
			}
			repair1014AssertState(t, st, 0, -1)
			if !after {
				if v, e := c.Get(ctx, pnlcache.KeyPortfolio("0xuser")).Result(); e != nil || v != "stale" {
					t.Fatalf("uncommitted refresh changed cache: %q %v", v, e)
				}
			} else {
				repair1014AssertFresh(t, c, "0xuser")
			}
			// Reopen all participant handles from committed peer state; no in-memory
			// Reconciler touched-users set can survive this point.
			st.Pool().Close()
			c.Close()
			_, st = repair1014Open(t, path, nil)
			c = peer.client(t)
			consumer := position.NewConsumer(st, c, discardLogger())
			if e = consumer.ProcessEvent(ctx, position.SwapEvent{PoolAddress: "0xpool", Sender: "0xUsEr", IsBuy: true, AmountIn: "1000000", AmountOut: oneToken, Price: "0", Fee: "0", BlockNumber: 100, BlockTimestamp: 100, TxHash: "0xswap", LogIndex: 0}); e != nil {
				t.Fatal(e)
			}
			worker := reconcile.New(st, c, discardLogger(), 500)
			n, e = worker.RunOnce(ctx)
			if n != 1 || e != nil {
				t.Fatalf("recovery=%d %v", n, e)
			}
			repair1014AssertState(t, st, 100, 0)
			repair1014AssertFresh(t, c, "0xuser")
			repair1014Notification(t, sub, st, c)
			if n, e = worker.RunOnce(ctx); n != 0 || e != nil {
				t.Fatalf("idle=%d %v", n, e)
			}
			peer.mu.Lock()
			count := peer.published
			peer.mu.Unlock()
			if count != 1 {
				t.Fatalf("notifications across restart=%d want=1", count)
			}
		})
	}
}

func TestRepairM_1014_PoisonCursorFailureRecovery(t *testing.T) {
	ctx := context.Background()
	good := repair1014Swap("0xswap-0", "0xUsEr", 0)
	poison := repair1014Swap("0xpoison-1", "0xother", 1)
	poison.AmountIn = "poison"
	path := filepath.Join(t.TempDir(), "postgres.json")
	db, st := repair1014Open(t, path, []store.IndexerSwap{good, poison})
	db.cursorFailure = true
	peer := repair1014StartRedis(t)
	c := peer.client(t)
	sub := repair1014Subscribe(t, peer.client(t))
	repair1014Cache(t, c, "0xuser")
	n, e := reconcile.New(st, c, discardLogger(), 500).RunOnce(ctx)
	if n != 1 || e == nil || !strings.Contains(e.Error(), poison.ID) || !strings.Contains(e.Error(), "cursor write refused") {
		t.Fatalf("poison plus cursor failure=%d %v", n, e)
	}
	repair1014AssertState(t, st, 0, -1)
	repair1014AssertFresh(t, c, "0xuser")
	repair1014Notification(t, sub, st, c)
	st.Pool().Close()
	c.Close()
	_, st = repair1014Open(t, path, nil)
	c = peer.client(t)
	repair1014Cache(t, c, "0xuser")
	n, e = reconcile.New(st, c, discardLogger(), 500).RunOnce(ctx)
	if n != 1 || e == nil || !strings.Contains(e.Error(), poison.ID) {
		t.Fatalf("recovery prefix=%d %v", n, e)
	}
	repair1014AssertState(t, st, 100, 0)
	repair1014AssertFresh(t, c, "0xuser")
	peer.mu.Lock()
	count := peer.published
	peer.mu.Unlock()
	if count != 1 {
		t.Fatalf("poison-path cursor replay duplicated refresh: %d", count)
	}
}
