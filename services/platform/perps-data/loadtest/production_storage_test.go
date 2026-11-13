package loadtest

// This is the storage peer of the load fixture, not a replacement for WriteBlock,
// Project, delivery.Store, publish.Publisher, feeds.Providers or gateway.Server.
// It implements the SQL operations used by the fill/order profile over in-memory
// tables. Reads come from committed rows, transactions serialize and undo every
// table mutation on rollback, and outbox claims enforce leases and fences. SQL
// outside this fixture's supported workload is an error, never a successful no-op.
// PostgreSQL migrations and PostgreSQL query planning remain outside this gate.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/markets"
	"github.com/lib/pq"
)

type profileStorageRow map[string]driver.Value
type profileStorageMemory struct {
	mu            sync.Mutex
	tables        map[string]map[string]profileStorageRow
	sequence      int64
	fillMarkets   map[string]map[string]profileStorageRow
	outboxPending map[string]profileStorageRow
}
type profileStorageConnector struct{ memory *profileStorageMemory }

func (c profileStorageConnector) Connect(context.Context) (driver.Conn, error) {
	return &profileStorageConn{memory: c.memory}, nil
}
func (c profileStorageConnector) Driver() driver.Driver { return profileStorageDriver{} }

type profileStorageDriver struct{}

func (profileStorageDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the fixture connector")
}

type profileStorageConn struct {
	memory *profileStorageMemory
	tx     bool
	undo   []func()
}

func (c *profileStorageConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fixture requires context queries")
}
func (c *profileStorageConn) Close() error { return nil }
func (c *profileStorageConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *profileStorageConn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.memory.mu.Lock()
	c.tx = true
	return c, nil
}
func (c *profileStorageConn) Commit() error {
	c.undo = nil
	c.tx = false
	c.memory.mu.Unlock()
	return nil
}
func (c *profileStorageConn) Rollback() error {
	for i := len(c.undo) - 1; i >= 0; i-- {
		c.undo[i]()
	}
	c.undo = nil
	c.tx = false
	c.memory.mu.Unlock()
	return nil
}
func (c *profileStorageConn) Ping(ctx context.Context) error { return ctx.Err() }
func (c *profileStorageConn) put(table, key string, row profileStorageRow) {
	t := c.memory.tables[table]
	if t == nil {
		t = map[string]profileStorageRow{}
		c.memory.tables[table] = t
	}
	old, exists := t[key]
	if c.tx {
		c.undo = append(c.undo, func() {
			if exists {
				t[key] = old
			} else {
				delete(t, key)
			}
			c.index(table, key, old)
		})
	}
	t[key] = row
	c.index(table, key, row)
}
func (c *profileStorageConn) index(table, key string, row profileStorageRow) {
	switch table {
	case "fills":
		for _, rows := range c.memory.fillMarkets {
			delete(rows, key)
		}
		if row != nil {
			market := profileStorageString(row["market_id"])
			if c.memory.fillMarkets[market] == nil {
				c.memory.fillMarkets[market] = map[string]profileStorageRow{}
			}
			c.memory.fillMarkets[market][key] = row
		}
	case "delivery_outbox":
		if row != nil && row["state"] != "published" {
			c.memory.outboxPending[key] = row
		} else {
			delete(c.memory.outboxPending, key)
		}
	}
}
func profileStorageCopy(row profileStorageRow) profileStorageRow {
	out := profileStorageRow{}
	for k, v := range row {
		out[k] = v
	}
	return out
}
func profileStorageString(v driver.Value) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}
func profileStorageInt(v driver.Value) int64 {
	if n, ok := v.(int64); ok {
		return n
	}
	n, _ := strconv.ParseInt(profileStorageString(v), 10, 64)
	return n
}
func profileStorageDecimal(v driver.Value, scale string) string {
	r, ok := new(big.Rat).SetString(profileStorageString(v))
	if !ok {
		return "0"
	}
	d, _ := new(big.Rat).SetString(scale)
	return r.Quo(r, d).FloatString(18)
}
func profileStorageAdd(a, b driver.Value) string {
	x, _ := new(big.Int).SetString(profileStorageString(a), 10)
	y, _ := new(big.Int).SetString(profileStorageString(b), 10)
	return new(big.Int).Add(x, y).String()
}
func profileStorageArgs(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}
func profileStorageSQL(q string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(q, `"`, "")), " ")
}

type profileStorageRows struct {
	columns []string
	values  [][]driver.Value
	at      int
}

func profileStorageResult(n int, values ...[]driver.Value) *profileStorageRows {
	cols := make([]string, n)
	for i := range cols {
		cols[i] = fmt.Sprint(i)
	}
	return &profileStorageRows{columns: cols, values: values}
}
func (r *profileStorageRows) Columns() []string { return r.columns }
func (r *profileStorageRows) Close() error      { return nil }
func (r *profileStorageRows) Next(dst []driver.Value) error {
	if r.at == len(r.values) {
		return io.EOF
	}
	copy(dst, r.values[r.at])
	r.at++
	return nil
}

var profileStorageInsert = regexp.MustCompile(`INSERT INTO perps\.([a-z_]+)\s*\(([^)]+)\) VALUES\s*(.*?)\s*(?:ON CONFLICT|RETURNING|$)`)
var profileStorageTuple = regexp.MustCompile(`\(([^()]*)\)`)
var profileStorageFactCTE = regexp.MustCompile(`f([0-9]+) AS \((INSERT INTO perps\.[a-z_]+ .*? RETURNING [0-9]+, tx_hash, log_index, sub)\)`)

func (c *profileStorageConn) insert(q string, args []driver.Value) (string, []profileStorageRow, error) {
	m := profileStorageInsert.FindStringSubmatch(q)
	if m == nil {
		return "", nil, fmt.Errorf("unsupported insert: %s", q)
	}
	table := m[1]
	columns := strings.Split(m[2], ",")
	var inserted []profileStorageRow
	for _, tuple := range profileStorageTuple.FindAllStringSubmatch(m[3], -1) {
		vals := strings.Split(tuple[1], ",")
		if len(vals) != len(columns) {
			return "", nil, errors.New("insert column count")
		}
		row := profileStorageRow{}
		for i, v := range vals {
			v = strings.TrimSpace(v)
			var value driver.Value
			switch {
			case strings.HasPrefix(v, "$"):
				p := strings.SplitN(v, "::", 2)[0]
				n, err := strconv.Atoi(p[1:])
				if err != nil || n < 1 || n > len(args) {
					return "", nil, errors.New("invalid bind")
				}
				value = args[n-1]
			case v == "NULL" || v == "DEFAULT":
				value = nil
			case v == "true":
				value = true
			case v == "false":
				value = false
			case strings.HasPrefix(v, "'"):
				value = strings.Trim(v, "'")
			default:
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return "", nil, fmt.Errorf("unsupported value %s", v)
				}
				value = n
			}
			row[strings.TrimSpace(columns[i])] = value
		}
		var key string
		switch table {
		case "blocks":
			key = profileStorageString(row["number"])
		case "cursors":
			key = profileStorageString(row["name"])
		case "orders":
			key = profileStorageString(row["order_id"])
		case "delivery_outbox":
			key = profileStorageString(row["event_id"])
		default:
			key = fmt.Sprintf("%x/%v/%v", row["tx_hash"], row["log_index"], row["sub"])
		}
		if _, ok := c.memory.tables[table][key]; ok {
			if strings.Contains(q, "DO NOTHING") {
				continue
			}
			return "", nil, fmt.Errorf("duplicate %s key", table)
		}
		if table == "delivery_outbox" {
			c.memory.sequence++
			row["perps_seq"] = c.memory.sequence
			row["state"] = "pending"
			row["attempts"] = int64(0)
			row["fence"] = int64(0)
			row["created_at"] = time.Now()
			row["available_at"] = time.Now()
		}
		c.put(table, key, row)
		inserted = append(inserted, row)
	}
	return table, inserted, nil
}

func (c *profileStorageConn) ExecContext(ctx context.Context, query string, named []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.tx {
		c.memory.mu.Lock()
		defer c.memory.mu.Unlock()
	}
	q := profileStorageSQL(query)
	a := profileStorageArgs(named)
	var n int64
	switch {
	case strings.HasPrefix(q, "SELECT pg_advisory_xact_lock("):
		if !c.tx {
			return nil, errors.New("advisory lock requires transaction")
		}
	case strings.HasPrefix(q, "UPDATE perps.delivery_outbox SET state='pending'"):
		for key, r := range c.memory.outboxPending {
			if r["state"] == "quarantined" && r["failure_class"] == "transient" && !r["available_at"].(time.Time).After(time.Now()) {
				v := profileStorageCopy(r)
				v["state"] = "pending"
				v["attempts"] = int64(0)
				c.put("delivery_outbox", key, v)
				n++
			}
		}
	case strings.HasPrefix(q, "UPDATE perps.delivery_outbox o SET acknowledgement_latency_ms="):
		ids, err := profileStorageArray(a[0])
		if err != nil {
			return nil, err
		}
		owners, err := profileStorageArray(a[1])
		if err != nil {
			return nil, err
		}
		fences, err := profileStorageArray(a[2])
		if err != nil {
			return nil, err
		}
		streams, err := profileStorageArray(a[3])
		if err != nil {
			return nil, err
		}
		seqs, err := profileStorageArray(a[4])
		if err != nil {
			return nil, err
		}
		for i, id := range ids {
			r := c.memory.tables["delivery_outbox"][id]
			if profileStorageLease(r, owners[i], fences[i]) {
				v := profileStorageCopy(r)
				v["state"] = "published"
				v["broker_stream"] = streams[i]
				v["broker_sequence"] = profileStorageInt(seqs[i])
				v["lease_owner"] = nil
				v["lease_until"] = nil
				c.put("delivery_outbox", id, v)
				n++
			}
		}
	case strings.HasPrefix(q, "UPDATE perps.cursors SET height ="):
		if r := c.memory.tables["cursors"]["head"]; r != nil {
			v := profileStorageCopy(r)
			v["height"], v["hash"] = a[0], a[1]
			c.put("cursors", "head", v)
			n = 1
		}
	case strings.HasPrefix(q, "UPDATE perps.orders SET remaining_size_usd = $2::numeric,"):
		key := profileStorageString(a[0])
		if r := c.memory.tables["orders"][key]; r != nil {
			v := profileStorageCopy(r)
			v["remaining_size_usd"] = a[1]
			if profileStorageString(a[1]) == "0" && (r["status"] == "active" || r["status"] == "triggered") {
				v["status"] = "filled"
				v["closed_block"] = a[2]
			}
			c.put("orders", key, v)
			n = 1
		}
	case strings.HasPrefix(q, "INSERT INTO perps.orders (") && strings.Contains(q, "FROM perps.projection_seeds"):
		if len(c.memory.tables["projection_seeds"]) != 0 {
			return nil, errors.New("fixture does not support state-seeded orders")
		}
	case strings.HasPrefix(q, "INSERT INTO perps.market_stats_rolling "):
		market := profileStorageString(a[0])
		at := a[1].(time.Time)
		fills := c.fills(market)
		v := profileStorageRow{"market_id": market, "volume_24h": "0", "volume_7d": "0", "volume_30d": "0", "trades_count": int64(len(fills)), "updated_block": a[2]}
		for _, r := range fills {
			age := at.Sub(r["block_time"].(time.Time))
			for name, span := range map[string]time.Duration{"volume_24h": 24 * time.Hour, "volume_7d": 7 * 24 * time.Hour, "volume_30d": 30 * 24 * time.Hour} {
				if age < span {
					v[name] = profileStorageAdd(v[name], r["size_usd"])
				}
			}
			v["last_price"] = r["price"]
			if age < 24*time.Hour {
				if v["open_24h"] == nil {
					v["open_24h"] = r["price"]
				}
			}
		}
		c.put("market_stats_rolling", market, v)
		n = 1
	case strings.HasPrefix(q, "INSERT INTO perps.market_snapshots "):
		if len(c.memory.tables["mark_prices"]) != 0 {
			return nil, errors.New("fixture snapshot does not support mark facts")
		}
	case strings.HasPrefix(q, "INSERT INTO perps.protocol_snapshots "):
		at := a[0].(time.Time)
		v := profileStorageRow{"hour": at, "total_volume_usd": "0", "trades_count": int64(0)}
		for _, r := range c.memory.tables["fills"] {
			if r["block_time"].(time.Time).Before(at) {
				v["total_volume_usd"] = profileStorageAdd(v["total_volume_usd"], r["size_usd"])
				v["trades_count"] = profileStorageInt(v["trades_count"]) + 1
			}
		}
		c.put("protocol_snapshots", at.String(), v)
		n = 1
	case strings.HasPrefix(q, "INSERT INTO perps."):
		_, rows, err := c.insert(q, a)
		if err != nil {
			return nil, err
		}
		n = int64(len(rows))
	default:
		return nil, fmt.Errorf("unsupported storage fixture exec: %s", q)
	}
	return driver.RowsAffected(n), nil
}
func profileStorageArray(value driver.Value) ([]string, error) {
	var a pq.StringArray
	err := a.Scan(value)
	return []string(a), err
}
func profileStorageLease(r profileStorageRow, owner, fence string) bool {
	if r == nil || r["state"] != "pending" || r["lease_owner"] != owner || profileStorageString(r["fence"]) != fence {
		return false
	}
	until, ok := r["lease_until"].(time.Time)
	return ok && until.After(time.Now())
}
func (c *profileStorageConn) fills(market string) []profileStorageRow {
	out := make([]profileStorageRow, 0, len(c.memory.fillMarkets[market]))
	for _, r := range c.memory.fillMarkets[market] {
		out = append(out, r)
	}
	slices.SortFunc(out, profileStorageOrder)
	return out
}
func profileStorageOrder(a, b profileStorageRow) int {
	for _, k := range []string{"block", "tx_index", "log_index", "sub"} {
		if x, y := profileStorageInt(a[k]), profileStorageInt(b[k]); x < y {
			return -1
		} else if x > y {
			return 1
		}
	}
	return 0
}
func (c *profileStorageConn) outbox() []profileStorageRow {
	out := make([]profileStorageRow, 0, len(c.memory.outboxPending))
	for _, r := range c.memory.outboxPending {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b profileStorageRow) int {
		return int(profileStorageInt(a["perps_seq"]) - profileStorageInt(b["perps_seq"]))
	})
	return out
}

func (c *profileStorageConn) QueryContext(ctx context.Context, query string, named []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.tx {
		c.memory.mu.Lock()
		defer c.memory.mu.Unlock()
	}
	q := profileStorageSQL(query)
	a := profileStorageArgs(named)
	switch {
	case strings.HasPrefix(q, "WITH f0 AS (INSERT INTO"):
		out := profileStorageResult(4)
		matches := profileStorageFactCTE.FindAllStringSubmatch(q, -1)
		if len(matches) == 0 {
			return nil, errors.New("fixture could not parse fact CTE")
		}
		for _, m := range matches {
			_, rows, err := c.insert(m[2], a)
			if err != nil {
				return nil, err
			}
			for _, r := range rows {
				out.values = append(out.values, []driver.Value{profileStorageInt(m[1]), r["tx_hash"], r["log_index"], r["sub"]})
			}
		}
		return out, nil
	case strings.HasPrefix(q, "INSERT INTO perps.delivery_outbox"):
		_, rows, err := c.insert(q, a)
		if err != nil {
			return nil, err
		}
		out := profileStorageResult(3)
		for _, r := range rows {
			out.values = append(out.values, []driver.Value{r["event_id"], r["subject"], nil})
		}
		return out, nil
	case strings.HasPrefix(q, "SELECT event_id, subject, envelope FROM perps.delivery_outbox"):
		ids, err := profileStorageArray(a[0])
		if err != nil {
			return nil, err
		}
		out := profileStorageResult(3)
		for _, id := range ids {
			if r := c.memory.tables["delivery_outbox"][id]; r != nil {
				out.values = append(out.values, []driver.Value{r["event_id"], r["subject"], r["envelope"]})
			}
		}
		return out, nil
	case strings.HasPrefix(q, "WITH barrier AS ("):
		out := profileStorageResult(7)
		now := time.Now()
		for _, r := range c.outbox() {
			if r["state"] != "pending" {
				continue
			}
			until, _ := r["lease_until"].(time.Time)
			if r["available_at"].(time.Time).After(now) || until.After(now) {
				break
			}
			if len(out.values) >= int(profileStorageInt(a[0])) {
				break
			}
			v := profileStorageCopy(r)
			v["lease_owner"] = a[1]
			v["lease_until"] = now.Add(time.Duration(profileStorageInt(a[2])) * time.Millisecond)
			v["fence"] = profileStorageInt(r["fence"]) + 1
			v["attempts"] = profileStorageInt(r["attempts"]) + 1
			c.put("delivery_outbox", profileStorageString(r["event_id"]), v)
			out.values = append(out.values, []driver.Value{v["event_id"], v["subject"], v["envelope"], v["perps_seq"], v["fence"], v["attempts"], false})
		}
		return out, nil
	case strings.HasPrefix(q, "UPDATE perps.delivery_outbox o SET state=CASE WHEN"):
		ids, err := profileStorageArray(a[0])
		if err != nil {
			return nil, err
		}
		owners, err := profileStorageArray(a[1])
		if err != nil {
			return nil, err
		}
		fences, err := profileStorageArray(a[2])
		if err != nil {
			return nil, err
		}
		out := profileStorageResult(4)
		for i, id := range ids {
			r := c.memory.tables["delivery_outbox"][id]
			if profileStorageLease(r, owners[i], fences[i]) {
				over := profileStorageInt(r["attempts"]) > profileStorageInt(a[3])
				class := ""
				if over {
					v := profileStorageCopy(r)
					v["state"] = "quarantined"
					v["failure_class"] = "exhausted"
					v["lease_owner"] = nil
					v["lease_until"] = nil
					c.put("delivery_outbox", id, v)
					class = "exhausted"
				}
				out.values = append(out.values, []driver.Value{r["perps_seq"], id, class, over})
			}
		}
		return out, nil
	case q == "SELECT count(*) FROM perps.delivery_outbox WHERE state <> 'published'":
		var n int64
		for _, r := range c.memory.tables["delivery_outbox"] {
			if r["state"] != "published" {
				n++
			}
		}
		return profileStorageResult(1, []driver.Value{n}), nil
	case strings.HasPrefix(q, "SELECT COALESCE(EXTRACT(EPOCH FROM clock_timestamp()-min(created_at))"):
		var oldest time.Time
		for _, r := range c.outbox() {
			if r["state"] == "pending" {
				oldest = r["created_at"].(time.Time)
				break
			}
		}
		ms := int64(0)
		if !oldest.IsZero() {
			ms = time.Since(oldest).Milliseconds()
		}
		return profileStorageResult(1, []driver.Value{ms}), nil
	case strings.HasPrefix(q, "SELECT symbol FROM perps.markets WHERE market_id ="):
		r := c.memory.tables["markets"][profileStorageString(a[0])]
		if r == nil {
			return profileStorageResult(1), nil
		}
		return profileStorageResult(1, []driver.Value{r["symbol"]}), nil
	case q == "SELECT market_id::text, symbol FROM perps.markets ORDER BY market_id" || q == "SELECT symbol, market_id::text FROM perps.markets":
		out := profileStorageResult(2)
		for _, m := range markets.Seed {
			r := c.memory.tables["markets"][strconv.FormatUint(m.ID, 10)]
			if r != nil {
				v := []driver.Value{r["market_id"], r["symbol"]}
				if strings.HasPrefix(q, "SELECT symbol,") {
					slices.Reverse(v)
				}
				out.values = append(out.values, v)
			}
		}
		return out, nil
	case strings.HasPrefix(q, "SELECT height FROM perps.cursors"):
		r := c.memory.tables["cursors"]["head"]
		if r == nil {
			return profileStorageResult(1), nil
		}
		return profileStorageResult(1, []driver.Value{r["height"]}), nil
	case strings.HasPrefix(q, "SELECT remaining_size_usd FROM perps.orders"):
		r := c.memory.tables["orders"][profileStorageString(a[0])]
		if r == nil {
			return profileStorageResult(1), nil
		}
		return profileStorageResult(1, []driver.Value{r["remaining_size_usd"]}), nil
	case strings.HasPrefix(q, "SELECT order_id::text, is_long,"):
		out := profileStorageResult(4)
		for _, r := range c.memory.tables["orders"] {
			if profileStorageString(r["market_id"]) == profileStorageString(a[0]) && r["status"] == "active" {
				out.values = append(out.values, []driver.Value{profileStorageString(r["order_id"]), r["is_long"], r["limit_price"], r["remaining_size_usd"]})
			}
		}
		return out, nil
	case strings.HasPrefix(q, "SELECT s.market_id FROM perps.market_stats_rolling"):
		out := profileStorageResult(1)
		for _, s := range c.memory.tables["market_stats_rolling"] {
			b := c.memory.tables["blocks"][profileStorageString(s["updated_block"])]
			if b != nil && b["time"].(time.Time).Before(a[0].(time.Time).Add(-24*time.Hour)) {
				out.values = append(out.values, []driver.Value{s["market_id"]})
			}
		}
		return out, nil
	case strings.HasPrefix(q, "WITH windows (window, since)"):
		if len(c.memory.tables["leaderboard"]) != 0 {
			return nil, errors.New("fixture only supports empty account leaderboard")
		}
		return profileStorageResult(1), nil
	case strings.HasPrefix(q, "SELECT time FROM perps.blocks WHERE number <"):
		var n int64 = -1
		var at driver.Value
		for _, b := range c.memory.tables["blocks"] {
			h := profileStorageInt(b["number"])
			if h < profileStorageInt(a[0]) && h > n {
				n = h
				at = b["time"]
			}
		}
		if at == nil {
			return profileStorageResult(1), nil
		}
		return profileStorageResult(1, []driver.Value{at}), nil
	case strings.HasPrefix(q, "WITH m AS ( SELECT mark_price"):
		if len(c.memory.tables["mark_prices"])+len(c.memory.tables["index_prices"]) != 0 {
			return nil, errors.New("fixture only supports unpriced markets")
		}
		return profileStorageResult(4), nil
	case strings.HasPrefix(q, "SELECT (SELECT long_oi /"):
		if len(c.memory.tables["open_interest"])+len(c.memory.tables["funding_settlements"]) != 0 {
			return nil, errors.New("fixture only supports unset OI/funding")
		}
		return profileStorageResult(4, []driver.Value{nil, nil, nil, nil}), nil
	case strings.HasPrefix(q, "SELECT (array_agg(f.price ORDER BY"):
		rows := c.fills(profileStorageString(a[0]))
		var first, hi, lo driver.Value
		sum := "0"
		for _, r := range rows {
			if r["block_time"].(time.Time).Before(a[1].(time.Time)) {
				continue
			}
			if first == nil {
				first = r["price"]
				hi = first
				lo = first
			}
			v, _ := new(big.Int).SetString(profileStorageString(r["price"]), 10)
			h, _ := new(big.Int).SetString(profileStorageString(hi), 10)
			l, _ := new(big.Int).SetString(profileStorageString(lo), 10)
			if v.Cmp(h) > 0 {
				hi = r["price"]
			}
			if v.Cmp(l) < 0 {
				lo = r["price"]
			}
			sum = profileStorageAdd(sum, r["size_usd"])
		}
		var f, h, l driver.Value
		if first != nil {
			f = profileStorageDecimal(first, profileStorageString(a[2]))
			h = profileStorageDecimal(hi, profileStorageString(a[2]))
			l = profileStorageDecimal(lo, profileStorageString(a[2]))
		}
		return profileStorageResult(5, []driver.Value{f, h, l, profileStorageDecimal(sum, profileStorageString(a[2])), "0"}), nil
	case strings.HasPrefix(q, "SELECT f.block, f.log_index, f.block_time, f.taker_is_long,"):
		rows := c.fills(profileStorageString(a[0]))
		slices.Reverse(rows)
		out := profileStorageResult(8)
		for _, r := range rows[:min(len(rows), int(profileStorageInt(a[1])))] {
			out.values = append(out.values, []driver.Value{r["block"], r["log_index"], r["block_time"], r["taker_is_long"], profileStorageDecimal(r["price"], profileStorageString(a[2])), profileStorageDecimal(r["size_usd"], profileStorageString(r["price"])), profileStorageDecimal(r["size_usd"], profileStorageString(a[2])), false})
		}
		return out, nil
	case strings.HasPrefix(q, "SELECT block, tx_index, log_index, sub, kind, order_id::text,"):
		out := profileStorageResult(14)
		for _, table := range []string{"order_events", "fills"} {
			for _, r := range c.memory.tables[table] {
				if profileStorageInt(r["block"]) != profileStorageInt(a[0]) || profileStorageInt(r["tx_index"]) != profileStorageInt(a[1]) || profileStorageInt(r["log_index"]) != profileStorageInt(a[2]) {
					continue
				}
				v := []driver.Value{r["block"], r["tx_index"], r["log_index"], r["sub"], r["kind"], "0", r["market_id"], nil, nil, nil, nil, nil, nil, nil}
				if table == "fills" {
					v[7] = r["taker_is_long"]
					v[10], v[11], v[12], v[13] = r["maker_order_id"], r["taker_order_id"], r["price"], r["size_usd"]
				} else {
					v[5], v[7], v[8], v[9] = r["order_id"], r["is_long"], r["size_usd"], r["limit_price"]
				}
				out.values = append(out.values, v)
			}
		}
		return out, nil
	case strings.HasPrefix(q, "SELECT m.market_id::text, m.symbol, m.name,"):
		return c.marketRecords(q, a), nil
	default:
		return nil, fmt.Errorf("unsupported storage fixture query: %s", q)
	}
}

func (c *profileStorageConn) marketRecords(q string, a []driver.Value) driver.Rows {
	rest := strings.Contains(q, "COALESCE(s.volume_24h, 0)")
	n := 12
	if rest {
		n = 20
	}
	out := profileStorageResult(n)
	for _, m := range markets.Seed {
		id := strconv.FormatUint(m.ID, 10)
		r := c.memory.tables["markets"][id]
		if r == nil {
			continue
		}
		if rest && a[2] != nil && a[2] != r["symbol"] {
			continue
		}
		s := c.memory.tables["market_stats_rolling"][id]
		vol24, vol7, vol30 := "0", "0", "0"
		trades := int64(0)
		var updated, change driver.Value
		if s != nil {
			vol24 = profileStorageDecimal(s["volume_24h"], profileStorageString(a[0]))
			vol7 = profileStorageDecimal(s["volume_7d"], profileStorageString(a[0]))
			vol30 = profileStorageDecimal(s["volume_30d"], profileStorageString(a[0]))
			trades = profileStorageInt(s["trades_count"])
			if b := c.memory.tables["blocks"][profileStorageString(s["updated_block"])]; b != nil {
				updated = b["time"]
			}
			if s["open_24h"] != nil {
				last, _ := new(big.Rat).SetString(profileStorageString(s["last_price"]))
				open, _ := new(big.Rat).SetString(profileStorageString(s["open_24h"]))
				change = last.Sub(last, open).Quo(last, open).FloatString(18)
			}
		}
		contracts := new(big.Rat)
		volume := "0"
		for _, f := range c.fills(id) {
			if f["block_time"].(time.Time).Before(a[1].(time.Time)) {
				continue
			}
			size, _ := new(big.Rat).SetString(profileStorageString(f["size_usd"]))
			price, _ := new(big.Rat).SetString(profileStorageString(f["price"]))
			contracts.Add(contracts, size.Quo(size, price))
			volume = profileStorageAdd(volume, f["size_usd"])
		}
		if rest {
			out.values = append(out.values, []driver.Value{id, r["symbol"], r["name"], change, vol24, vol7, vol30, trades, "0", "0", nil, nil, nil, nil, nil, nil, profileStorageDecimal(volume, profileStorageString(a[0])), contracts.FloatString(18), "0", "0"})
		} else {
			out.values = append(out.values, []driver.Value{id, r["symbol"], r["name"], vol7, vol30, trades, change, updated, contracts.FloatString(18), "0", nil, nil})
		}
	}
	return out
}

func profileStorageDatabase(t *testing.T) (*sql.DB, *profileStorageMemory) {
	t.Helper()
	m := &profileStorageMemory{tables: map[string]map[string]profileStorageRow{}, fillMarkets: map[string]map[string]profileStorageRow{}, outboxPending: map[string]profileStorageRow{}}
	m.tables["markets"] = map[string]profileStorageRow{}
	for _, market := range markets.Seed {
		id := strconv.FormatUint(market.ID, 10)
		m.tables["markets"][id] = profileStorageRow{"market_id": id, "symbol": market.Symbol, "name": market.Name}
	}
	db := sql.OpenDB(profileStorageConnector{m})
	db.SetMaxOpenConns(64)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, m
}
