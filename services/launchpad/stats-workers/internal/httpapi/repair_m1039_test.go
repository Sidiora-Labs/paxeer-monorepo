package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

// The database peer is a committed, file-backed in-memory table fixture exposed
// through PostgreSQL's wire protocol. Store, pgx, HTTP routing, operation ledger,
// signature codec and reaction authority are production implementations. Only
// the peer executes these explicitly supported transcript statements; this is
// not PostgreSQL execution-plan or production capacity evidence.
type repair1039Tables struct {
	Transactions  map[string]store.TransactionRow
	Cross         map[string]store.CrossTokenSwapRow
	Pools         map[string]store.PoolStatsRow
	Holders       []store.HolderRow
	Votes         map[string]analytics.ReactionResult
	Nonces        map[string][]string
	Ledger        map[string][]string
	PoolRevision  int64
	WalletChanges map[string]time.Time
	Outbox        []string
	Audit         []string
}
type repair1039DB struct {
	t            *testing.T
	mu           sync.Mutex
	Tables       repair1039Tables
	Now          time.Time
	File         string
	Listener     net.Listener
	Connections  sync.WaitGroup
	RowsReturned int
	WalletVisits int
	Canonical    []holdings.Balance
	Distribution holdings.Distribution
	FailLedger   bool
	Queries      []string
}

func repair1039Database(t *testing.T) (*repair1039DB, *pgxpool.Pool) {
	t.Helper()
	d := &repair1039DB{t: t, File: filepath.Join(t.TempDir(), "tables.json"), Now: time.Now().UTC()}
	d.Tables = repair1039Tables{Transactions: map[string]store.TransactionRow{}, Cross: map[string]store.CrossTokenSwapRow{}, Pools: map[string]store.PoolStatsRow{}, Votes: map[string]analytics.ReactionResult{}, Nonces: map[string][]string{}, Ledger: map[string][]string{}, WalletChanges: map[string]time.Time{}}
	d.start()
	pool := d.openPool()
	t.Cleanup(func() { pool.Close(); d.Listener.Close(); d.Connections.Wait() })
	return d, pool
}
func (d *repair1039DB) start() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		d.t.Fatal(err)
	}
	d.Listener = listener
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			d.Connections.Add(1)
			go d.connection(connection)
		}
	}()
}
func (d *repair1039DB) openPool() *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig("postgres://fixture:fixture@" + d.Listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		d.t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		d.t.Fatal(err)
	}
	return pool
}
func (d *repair1039DB) restart(pool *pgxpool.Pool) *pgxpool.Pool {
	pool.Close()
	d.Listener.Close()
	d.Connections.Wait()
	raw, err := os.ReadFile(d.File)
	if err != nil {
		d.t.Fatal(err)
	}
	d.Tables = repair1039Tables{}
	if err = json.Unmarshal(raw, &d.Tables); err != nil {
		d.t.Fatal(err)
	}
	d.start()
	next := d.openPool()
	d.t.Cleanup(next.Close)
	return next
}
func (d *repair1039DB) save() {
	body, err := json.Marshal(d.Tables)
	if err != nil {
		d.t.Error(err)
		return
	}
	if err = os.WriteFile(d.File, body, 0600); err != nil {
		d.t.Error(err)
	}
}

type repair1039Result struct {
	rows    [][]any
	columns []uint32
	tag     string
	err     string
}

func repair1039Rows(rows ...[]any) repair1039Result {
	return repair1039Result{rows: rows, tag: fmt.Sprintf("SELECT %d", len(rows))}
}
func repair1039Fields(result repair1039Result) []pgproto3.FieldDescription {
	count := len(result.columns)
	if len(result.rows) > 0 {
		count = len(result.rows[0])
	}
	fields := make([]pgproto3.FieldDescription, count)
	for i := range fields {
		oid := uint32(25)
		if len(result.columns) > i {
			oid = result.columns[i]
		} else {
			for _, row := range result.rows {
				if row[i] == nil {
					continue
				}
				switch row[i].(type) {
				case int, int64, uint64:
					oid = 20
				case bool:
					oid = 16
				case []byte:
					oid = 17
				case time.Time:
					oid = 1184
				}
				break
			}
		}
		fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("v%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1, Format: 0}
	}
	return fields
}
func repair1039Wire(value any) []byte {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return []byte("\\x" + hex.EncodeToString(v))
	case time.Time:
		return []byte(v.Format("2006-01-02 15:04:05.999999999Z07:00"))
	case bool:
		if v {
			return []byte("t")
		}
		return []byte("f")
	}
	return []byte(fmt.Sprint(value))
}
func (d *repair1039DB) connection(connection net.Conn) {
	defer d.Connections.Done()
	defer connection.Close()
	backend := pgproto3.NewBackend(connection, connection)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if backend.Flush() != nil {
		return
	}
	statements := map[string]*pgproto3.Parse{}
	var result repair1039Result
	locked := false
	var rollback repair1039Tables
	defer func() {
		if locked {
			d.Tables = rollback
			d.mu.Unlock()
		}
	}()
	execute := func(sql string, args []string) repair1039Result {
		lower := strings.ToLower(strings.TrimSpace(sql))
		if strings.HasPrefix(lower, "begin") {
			d.mu.Lock()
			locked = true
			raw, _ := json.Marshal(d.Tables)
			rollback = repair1039Tables{}
			_ = json.Unmarshal(raw, &rollback)
			return repair1039Result{tag: "BEGIN"}
		}
		if lower == "commit" {
			d.save()
			locked = false
			d.mu.Unlock()
			return repair1039Result{tag: "COMMIT"}
		}
		if lower == "rollback" {
			d.Tables = rollback
			locked = false
			d.mu.Unlock()
			return repair1039Result{tag: "ROLLBACK"}
		}
		if !locked {
			d.mu.Lock()
			defer d.mu.Unlock()
		}
		return d.query(sql, args)
	}
	for {
		message, err := backend.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Parse:
			copy := *m
			statements[m.Name] = &copy
			backend.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			statement := statements[m.PreparedStatement]
			if statement == nil {
				return
			}
			args := make([]string, len(m.Parameters))
			for i, raw := range m.Parameters {
				format := int16(0)
				if len(m.ParameterFormatCodes) == 1 {
					format = m.ParameterFormatCodes[0]
				} else if len(m.ParameterFormatCodes) > i {
					format = m.ParameterFormatCodes[i]
				}
				oid := uint32(25)
				if len(statement.ParameterOIDs) > i {
					oid = statement.ParameterOIDs[i]
				}
				var value any
				if err := pgtype.NewMap().Scan(oid, format, raw, &value); err != nil {
					d.t.Errorf("decode parameter %d OID %d: %v", i, oid, err)
					return
				}
				if b, ok := value.([]byte); ok {
					args[i] = string(b)
				} else if value != nil {
					args[i] = fmt.Sprint(value)
				}
			}
			result = execute(statement.Query, args)
			backend.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			if fields := repair1039Fields(result); len(fields) > 0 {
				backend.Send(&pgproto3.RowDescription{Fields: fields})
			} else {
				backend.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			if result.err != "" {
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: result.err})
			} else {
				for _, row := range result.rows {
					values := make([][]byte, len(row))
					for i, v := range row {
						values[i] = repair1039Wire(v)
					}
					backend.Send(&pgproto3.DataRow{Values: values})
				}
				tag := result.tag
				if tag == "" {
					tag = "INSERT 0 1"
				}
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
			}
		case *pgproto3.Query:
			result = execute(m.String, nil)
			if result.err != "" {
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: result.err})
			} else {
				if fields := repair1039Fields(result); len(fields) > 0 {
					backend.Send(&pgproto3.RowDescription{Fields: fields})
				}
				for _, row := range result.rows {
					values := make([][]byte, len(row))
					for i, v := range row {
						values[i] = repair1039Wire(v)
					}
					backend.Send(&pgproto3.DataRow{Values: values})
				}
				tag := result.tag
				if tag == "" {
					tag = "SELECT 1"
				}
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: func() byte {
				if locked {
					return 'T'
				}
				return 'I'
			}()})
		case *pgproto3.Sync:
			status := byte('I')
			if locked {
				status = 'T'
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Terminate:
			return
		default:
			d.t.Errorf("unsupported PostgreSQL transcript message %T", message)
			return
		}
		if backend.Flush() != nil {
			return
		}
	}
}
func repair1039Int(s string) int64 { v, _ := strconv.ParseInt(s, 10, 64); return v }
func repair1039JSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
func repair1039TxValues(v store.TransactionRow) []any {
	return []any{v.ID, v.PoolAddress, v.Sender, v.IsBuy, v.AmountIn, v.AmountOut, v.Price, v.Fee, v.BlockTimestamp, v.TxHash}
}
func repair1039CrossValues(v store.CrossTokenSwapRow) []any {
	return []any{v.ID, v.Sender, v.TokenIn, v.TokenOut, v.PoolIn, v.PoolOut, v.AmountIn, v.IntermediateUsdl, v.AmountOut, v.FeeIn, v.FeeOut, v.BlockTimestamp, v.TxHash}
}
func repair1039Before(ts int64, id string, boundTS int64, boundID string) bool {
	return ts < boundTS || ts == boundTS && id < boundID
}

func (d *repair1039DB) query(sql string, a []string) repair1039Result {
	q := strings.Join(strings.Fields(sql), " ")
	d.Queries = append(d.Queries, q)
	if q == "-- ping" {
		return repair1039Result{tag: "SELECT 0"}
	}
	if strings.Contains(q, "pg_advisory_xact_lock") {
		return repair1039Result{tag: "SELECT 1"}
	}
	if q == "SELECT clock_timestamp()" {
		return repair1039Rows([]any{d.Now})
	}
	if strings.Contains(q, "INSERT INTO stats.pool_transactions") {
		d.Tables.Transactions[a[0]] = store.TransactionRow{ID: a[0], PoolAddress: a[1], Sender: a[2], IsBuy: repair1039Bool(a[3]), AmountIn: a[4], AmountOut: a[5], Price: a[6], Fee: a[7], BlockTimestamp: repair1039Int(a[8]), TxHash: a[9]}
		d.save()
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "INSERT INTO stats.cross_token_swaps") {
		d.Tables.Cross[a[0]] = store.CrossTokenSwapRow{ID: a[0], Sender: a[1], TokenIn: a[2], TokenOut: a[3], PoolIn: a[4], PoolOut: a[5], AmountIn: a[6], IntermediateUsdl: a[7], AmountOut: a[8], FeeIn: a[9], FeeOut: a[10], BlockTimestamp: repair1039Int(a[11]), TxHash: a[12]}
		d.save()
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "INSERT INTO stats.pool_stats") {
		creator := a[2]
		d.Tables.Pools[a[0]] = store.PoolStatsRow{PoolAddress: a[0], TokenAddress: a[1], CreatorAddress: &creator, CreatorHoldingsPct: "0"}
		d.save()
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "FROM stats.pool_stats WHERE pool_address") {
		row, ok := d.Tables.Pools[a[0]]
		if !ok {
			return repair1039Rows()
		}
		values := make([]any, 32)
		for i := range values {
			values[i] = "0"
		}
		values[0], values[1] = row.PoolAddress, row.TokenAddress
		for _, i := range []int{19, 20, 21, 22, 25, 28, 29} {
			values[i] = int64(0)
		}
		values[26] = nil
		if row.CreatorAddress != nil {
			values[27] = *row.CreatorAddress
		} else {
			values[27] = nil
		}
		values = values[:30]
		return repair1039Rows(values)
	}
	if strings.Contains(q, "FROM stats.pool_holders") {
		if strings.Contains(q, "COUNT(*)") {
			return repair1039Rows([]any{int64(len(d.Tables.Holders))})
		}
		if strings.HasPrefix(q, "SELECT balance FROM") {
			return repair1039Rows()
		}
		if !strings.Contains(q, "LIMIT $2 OFFSET $3") {
			return repair1039Result{err: "unbounded holder read refused by fixture"}
		}
		limit, offset := int(repair1039Int(a[1])), int(repair1039Int(a[2]))
		var rows [][]any
		for i := offset; i < len(d.Tables.Holders) && len(rows) < limit; i++ {
			v := d.Tables.Holders[i]
			rows = append(rows, []any{v.PoolAddress, v.HolderAddress, v.Balance, v.PctOfSupply, v.LastUpdated})
		}
		d.RowsReturned += len(rows)
		return repair1039Rows(rows...)
	}
	if strings.Contains(q, "WITH summary AS") {
		if !strings.Contains(q, "LIMIT $8 OFFSET $3") || !strings.Contains(q, "sum(amount_out::numeric)") || !strings.Contains(q, "block_timestamp,id") {
			return repair1039Result{err: "creator SQL contract missing"}
		}
		all := []store.TransactionRow{}
		for _, v := range d.Tables.Transactions {
			if v.PoolAddress == a[0] && v.Sender == a[1] && (a[6] == "" || !repair1039Before(repair1039Int(a[5]), a[6], v.BlockTimestamp, v.ID)) {
				all = append(all, v)
			}
		}
		sort.Slice(all, func(i, j int) bool {
			return repair1039Before(all[j].BlockTimestamp, all[j].ID, all[i].BlockTimestamp, all[i].ID)
		})
		bought, sold := new(big.Int), new(big.Int)
		var buys, sells int64
		for _, v := range all {
			if v.IsBuy {
				buys++
				n, _ := new(big.Int).SetString(v.AmountOut, 10)
				bought.Add(bought, n)
			} else {
				sells++
				n, _ := new(big.Int).SetString(v.AmountIn, 10)
				sold.Add(sold, n)
			}
		}
		page := []store.TransactionRow{}
		skip := int(repair1039Int(a[2]))
		limit := int(repair1039Int(a[7]))
		for _, v := range all {
			if a[4] != "" && !repair1039Before(v.BlockTimestamp, v.ID, repair1039Int(a[3]), a[4]) {
				continue
			}
			if skip > 0 {
				skip--
				continue
			}
			if len(page) < limit {
				page = append(page, v)
			}
		}
		var topTime, topID any
		if len(all) > 0 {
			topTime, topID = all[0].BlockTimestamp, all[0].ID
		}
		d.RowsReturned += len(page)
		return repair1039Rows([]any{buys, sells, bought.String(), sold.String(), new(big.Int).Sub(bought, sold).String(), topTime, topID, repair1039JSON(page)})
	}
	if strings.Contains(q, "FROM stats.pool_transactions") {
		if !strings.Contains(q, "ORDER BY block_timestamp DESC, id DESC LIMIT $2 OFFSET $3") {
			return repair1039Result{err: "transaction SQL order/bound missing"}
		}
		data := []store.TransactionRow{}
		for _, v := range d.Tables.Transactions {
			if v.PoolAddress == a[0] && (!strings.Contains(q, "is_buy = true") || v.IsBuy) && (!strings.Contains(q, "is_buy = false") || !v.IsBuy) {
				data = append(data, v)
			}
		}
		sort.Slice(data, func(i, j int) bool {
			return repair1039Before(data[j].BlockTimestamp, data[j].ID, data[i].BlockTimestamp, data[i].ID)
		})
		var rows [][]any
		for i := int(repair1039Int(a[2])); i < len(data) && len(rows) < int(repair1039Int(a[1])); i++ {
			rows = append(rows, repair1039TxValues(data[i]))
		}
		d.RowsReturned += len(rows)
		return repair1039Rows(rows...)
	}
	if strings.Contains(q, "FROM stats.cross_token_swaps") {
		if !strings.Contains(q, "ORDER BY block_timestamp DESC,id DESC LIMIT $2 OFFSET $3") || !strings.Contains(q, "(block_timestamp,id)<($4,$5)") {
			return repair1039Result{err: "cross SQL cursor/order/bound missing"}
		}
		data := []store.CrossTokenSwapRow{}
		for _, v := range d.Tables.Cross {
			match := v.Sender == a[0]
			if strings.Contains(q, "token_in=$1") {
				match = v.TokenIn == a[0] || v.TokenOut == a[0]
			}
			if match && (a[4] == "" || repair1039Before(v.BlockTimestamp, v.ID, repair1039Int(a[3]), a[4])) {
				data = append(data, v)
			}
		}
		sort.Slice(data, func(i, j int) bool {
			return repair1039Before(data[j].BlockTimestamp, data[j].ID, data[i].BlockTimestamp, data[i].ID)
		})
		var rows [][]any
		for i := int(repair1039Int(a[2])); i < len(data) && len(rows) < int(repair1039Int(a[1])); i++ {
			rows = append(rows, repair1039CrossValues(data[i]))
		}
		d.RowsReturned += len(rows)
		return repair1039Rows(rows...)
	}
	if strings.Contains(q, `.tokens WHERE token=$1`) {
		return repair1039Rows([]any{d.Distribution.Supply, "complete", int64(7)})
	}
	if strings.Contains(q, `.aggregates WHERE token=$1`) {
		var rows [][]any
		for band, v := range d.Distribution.Bands {
			if v.Count > 0 {
				rows = append(rows, []any{"external_wallet", int64(band), v.Count, v.Raw})
			}
		}
		d.RowsReturned += len(rows)
		return repair1039Rows(rows...)
	}
	if strings.Contains(q, "COALESCE(sum(balance),0)") {
		if !strings.Contains(q, "ORDER BY balance DESC,address ASC LIMIT 10") {
			return repair1039Result{err: "unbounded leader query"}
		}
		sum := new(big.Int)
		for i := 0; i < 10 && i < len(d.Canonical); i++ {
			n, _ := new(big.Int).SetString(d.Canonical[i].Raw, 10)
			sum.Add(sum, n)
		}
		return repair1039Rows([]any{sum.String()})
	}
	if strings.HasPrefix(q, "SELECT balance::text,category FROM") {
		for _, v := range d.Canonical {
			if v.Address == a[1] {
				return repair1039Rows([]any{v.Raw, v.Category})
			}
		}
		return repair1039Rows()
	}
	if strings.Contains(q, `.balances WHERE token=$1`) {
		if !strings.Contains(q, "ORDER BY balance DESC,address ASC LIMIT $6 OFFSET $7") || !strings.Contains(q, "balance>=$2::numeric") {
			return repair1039Result{err: "indexed wallet query missing"}
		}
		limit, offset := int(repair1039Int(a[5])), int(repair1039Int(a[6]))
		threshold, _ := new(big.Int).SetString(a[1], 10)
		bound, _ := new(big.Int).SetString(a[2], 10)
		start := 0
		if !repair1039Bool(a[4]) {
			start = sort.Search(len(d.Canonical), func(i int) bool {
				v, _ := new(big.Int).SetString(d.Canonical[i].Raw, 10)
				c := v.Cmp(bound)
				return c < 0 || c == 0 && d.Canonical[i].Address > a[3]
			})
		}
		var rows [][]any
		for i := start + offset; i < len(d.Canonical) && len(rows) < limit; i++ {
			d.WalletVisits++
			v := d.Canonical[i]
			n, _ := new(big.Int).SetString(v.Raw, 10)
			if n.Cmp(threshold) < 0 {
				break
			}
			rows = append(rows, []any{v.Address, v.Raw})
		}
		d.RowsReturned += len(rows)
		return repair1039Rows(rows...)
	}
	// The reaction transcript persists every committed ledger/vote/outbox operation.
	if strings.Contains(q, `INSERT INTO "stats"."reaction_wallets"`) {
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "SELECT last_change,clock_timestamp()") {
		var last any
		if v, ok := d.Tables.WalletChanges[a[0]]; ok {
			last = v
		}
		return repair1039Rows([]any{last, d.Now})
	}
	if strings.Contains(q, "SELECT envelope_digest,body_digest,response,response_digest FROM operation_ledger") {
		if v, ok := d.Tables.Ledger[a[0]+a[1]+a[2]]; ok {
			return repair1039Rows([]any{v[3], v[4], []byte(v[6]), v[7]})
		}
		return repair1039Rows()
	}
	if strings.HasPrefix(q, "INSERT INTO operation_ledger") {
		if d.FailLedger {
			d.FailLedger = false
			return repair1039Result{err: "injected failure before durable ledger commit"}
		}
		response, decodeErr := hex.DecodeString(strings.TrimPrefix(a[6], "\\x"))
		if !strings.HasPrefix(a[6], "\\x") || decodeErr != nil {
			return repair1039Result{err: "invalid bytea transcript parameter"}
		}
		saved := append([]string(nil), a...)
		saved[6] = string(response)
		d.Tables.Ledger[a[0]+a[1]+a[2]] = saved
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "INSERT INTO security_audit_outbox") {
		d.Tables.Audit = append(d.Tables.Audit, strings.Join(a, "|"))
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "SELECT digest,result FROM") {
		if v, ok := d.Tables.Nonces[a[0]+a[1]]; ok {
			return repair1039Rows([]any{v[2], []byte(v[3])})
		}
		return repair1039Rows()
	}
	if strings.Contains(q, "SELECT revision FROM") && strings.Contains(q, "reaction_votes") {
		if v, ok := d.Tables.Votes[a[0]+a[1]]; ok {
			return repair1039Rows([]any{v.Revision})
		}
		return repair1039Rows()
	}
	if strings.Contains(q, "INSERT INTO") && strings.Contains(q, "reaction_pools") {
		d.Tables.PoolRevision++
		return repair1039Rows([]any{d.Tables.PoolRevision})
	}
	if strings.Contains(q, "INSERT INTO") && strings.Contains(q, "reaction_votes") {
		d.Tables.Votes[a[0]+a[1]] = analytics.ReactionResult{Vote: a[2], Revision: repair1039Int(a[3])}
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "SELECT reaction,count(*)") {
		counts := map[string]int64{}
		for key, v := range d.Tables.Votes {
			if strings.HasPrefix(key, a[0]) && v.Vote != "clear" {
				counts[v.Vote]++
			}
		}
		rows := [][]any{}
		for k, n := range counts {
			rows = append(rows, []any{k, n})
		}
		return repair1039Rows(rows...)
	}
	if strings.Contains(q, "INSERT INTO") && strings.Contains(q, "reaction_nonces") {
		d.Tables.Nonces[a[0]+a[1]] = append([]string(nil), a...)
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "INSERT INTO") && strings.Contains(q, "reaction_outbox") {
		d.Tables.Outbox = append(d.Tables.Outbox, a[1])
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.Contains(q, "UPDATE") && strings.Contains(q, "reaction_wallets") {
		d.Tables.WalletChanges[a[0]] = d.Now
		return repair1039Result{tag: "UPDATE 1"}
	}
	if strings.HasPrefix(q, "SELECT revision::text") && strings.Contains(q, "reaction_pools") {
		return repair1039Rows([]any{strconv.FormatInt(d.Tables.PoolRevision, 10)})
	}
	d.t.Errorf("unexpected database transcript: %s args=%q", q, a)
	return repair1039Result{err: "unexpected database transcript"}
}

func repair1039Pool(t *testing.T, st *store.Store, pool string, creator *string) {
	t.Helper()
	if _, err := st.InsertInitialPoolStats(context.Background(), store.InitialPoolStats{PoolAddress: pool, TokenAddress: "0x" + strings.Repeat("3", 40), CreatorAddress: creator, Price: "1", MarketCap: "1", High24h: "1", Low24h: "1"}); err != nil {
		t.Fatal(err)
	}
}
func repair1039Get(t *testing.T, router http.Handler, path string) map[string]json.RawMessage {
	t.Helper()
	response := serve(t, router, "GET", path, nil, nil)
	if response.Code != 200 {
		t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
	}
	var result map[string]json.RawMessage
	decode(t, response, &result)
	return result
}
func repair1039Next(t *testing.T, response map[string]json.RawMessage) string {
	t.Helper()
	var next *string
	if err := json.Unmarshal(response["nextCursor"], &next); err != nil {
		t.Fatal(err)
	}
	if next == nil {
		return ""
	}
	return *next
}
func repair1039InvalidPages(t *testing.T, router http.Handler, path string, maximum int) {
	t.Helper()
	for _, q := range []string{"limit=-1", "offset=-1", "limit=" + strconv.Itoa(maximum+1), "limit=NaN", "offset=no", "limit=1&limit=2", "offset=10001", "cursor=garbage", "limit=99999999999999999999999999999999999", "limit="} {
		response := serve(t, router, "GET", path+"?"+q, nil, nil)
		if response.Code != 400 {
			t.Fatalf("%s?%s: %d %s", path, q, response.Code, response.Body.String())
		}
	}
}

func TestRepairM_1039_CrossHistoryPageBound(t *testing.T) {
	d, pool := repair1039Database(t)
	st := store.New(pool)
	router := chi.NewRouter()
	httpapi.RegisterCrossTokenSwaps(router, st)
	wallet, token := "0xwallet", "0xtoken"
	for i := 0; i < 1003; i++ {
		v := store.CrossTokenSwapRow{ID: fmt.Sprintf("swap-%04d", i), Sender: wallet, TokenIn: token, TokenOut: "other", AmountIn: "1", IntermediateUsdl: "1", AmountOut: "1", FeeIn: "0", FeeOut: "0", BlockTimestamp: int64(i / 3), TxHash: "hash"}
		if err := st.InsertCrossTokenSwap(context.Background(), v); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/stats/cross-token-swaps/0xWALLET", "/stats/cross-token-swaps/token/0xTOKEN"} {
		repair1039InvalidPages(t, router, path, 500)
		seen := map[string]bool{}
		cursor := ""
		last := int64(1 << 62)
		lastID := ""
		for page := 0; ; page++ {
			target := path + "?limit=500"
			if cursor != "" {
				target += "&cursor=" + url.QueryEscape(cursor)
			}
			response := repair1039Get(t, router, target)
			var swaps []store.CrossTokenSwapRow
			if err := json.Unmarshal(response["swaps"], &swaps); err != nil {
				t.Fatal(err)
			}
			expected := 500
			if page == 2 {
				expected = 3
			}
			if len(swaps) != expected {
				t.Fatalf("exact page boundary: page %d size %d", page, len(swaps))
			}
			for _, v := range swaps {
				if seen[v.ID] || v.BlockTimestamp > last || (v.BlockTimestamp == last && v.ID >= lastID) {
					t.Fatal("unstable cross-history ordering", v.ID)
				}
				seen[v.ID] = true
				last, lastID = v.BlockTimestamp, v.ID
			}
			cursor = repair1039Next(t, response)
			if cursor == "" {
				break
			}
		}
		if len(seen) != 1003 {
			t.Fatal("cross-history loss", len(seen))
		}
	}
	if d.RowsReturned > 2010 {
		t.Fatal("cross history exceeded bounded transport", d.RowsReturned)
	}
}

func TestRepairM_1039_TransactionPageBound(t *testing.T) {
	_, pool := repair1039Database(t)
	st := store.New(pool)
	router := chi.NewRouter()
	httpapi.RegisterPoolTransactions(router, st)
	for i := 0; i < 1003; i++ {
		insertTx(t, context.Background(), st, fmt.Sprintf("tx-%04d", i), "0xpool", "wallet", i%2 == 0, "4", "10", int64(i/3))
	}
	path := "/stats/0xPOOL/transactions"
	repair1039InvalidPages(t, router, path, 500)
	for _, offset := range []int{0, 500, 1000, 1003} {
		result := repair1039Get(t, router, fmt.Sprintf("%s?limit=500&offset=%d", path, offset))
		var rows []store.TransactionRow
		if err := json.Unmarshal(result["transactions"], &rows); err != nil {
			t.Fatal(err)
		}
		expected := 500
		if offset == 1000 {
			expected = 3
		}
		if offset == 1003 {
			expected = 0
		}
		if len(rows) != expected {
			t.Fatal("transaction page", offset, len(rows))
		}
		for i := 1; i < len(rows); i++ {
			if !repair1039Before(rows[i].BlockTimestamp, rows[i].ID, rows[i-1].BlockTimestamp, rows[i-1].ID) {
				t.Fatal("transaction tie ordering")
			}
		}
	}
}

func TestRepairM_1039_CreatorHistoryUnbounded(t *testing.T) {
	d, pool := repair1039Database(t)
	st := store.New(pool)
	router := chi.NewRouter()
	httpapi.RegisterPoolAnalytics(router, st)
	creator := "0xcreator"
	repair1039Pool(t, st, "0xpool", &creator)
	for i := 0; i < 307; i++ {
		insertTx(t, context.Background(), st, fmt.Sprintf("tx-%04d", i), "0xpool", creator, i%2 == 0, "4", "10", int64(i/3))
	}
	path := "/stats/0xpool/creator-activity"
	repair1039InvalidPages(t, router, path, 100)
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	var last store.TransactionRow
	for {
		target := path + "?limit=37"
		if cursor != "" {
			target += "&cursor=" + url.QueryEscape(cursor)
		}
		result := repair1039Get(t, router, target)
		var rows []store.TransactionRow
		var summary store.CreatorSummary
		if err := json.Unmarshal(result["transactions"], &rows); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(result["summary"], &summary); err != nil {
			t.Fatal(err)
		}
		if summary.BuyCount != 154 || summary.SellCount != 153 || summary.TotalBoughtTokens != "1540" || summary.TotalSoldTokens != "612" || summary.NetTokenBalance != "928" || !summary.HasSold {
			t.Fatalf("summary lost snapshot boundary: %+v", summary)
		}
		for _, v := range rows {
			if seen[v.ID] {
				t.Fatal("duplicate creator transaction")
			}
			if last.ID != "" && !repair1039Before(v.BlockTimestamp, v.ID, last.BlockTimestamp, last.ID) {
				t.Fatal("creator ordering")
			}
			seen[v.ID] = true
			last = v
		}
		pages++
		cursor = repair1039Next(t, result)
		if pages == 1 {
			done := make(chan struct{})
			go func() {
				defer close(done)
				err := st.InsertTransaction(context.Background(), store.TransactionRow{ID: "canonical-append", PoolAddress: "0xpool", Sender: creator, IsBuy: true, AmountIn: "0", AmountOut: "20", Price: "1", Fee: "0", BlockTimestamp: 1000, TxHash: "append"})
				if err != nil {
					t.Error(err)
				}
			}()
			<-done
		}
		if cursor == "" {
			break
		}
	}
	if len(seen) != 307 || pages != 9 {
		t.Fatal("creator page boundary", len(seen), pages)
	}
	result := repair1039Get(t, router, path+"?limit=100")
	var summary store.CreatorSummary
	_ = json.Unmarshal(result["summary"], &summary)
	if summary.BuyCount != 155 || summary.TotalBoughtTokens != "1560" {
		t.Fatal("fresh history missed append", summary)
	}
	if d.RowsReturned > 430 {
		t.Fatal("unbounded creator transport", d.RowsReturned)
	}
}

func TestRepairM_1039_LegacyHolderPageBound(t *testing.T) {
	d, pool := repair1039Database(t)
	st := store.New(pool)
	router := chi.NewRouter()
	httpapi.RegisterPoolHolders(router, st)
	// The peer is a populated table fixture, never a replacement handler/store.
	for i := 0; i < 12003; i++ {
		d.Tables.Holders = append(d.Tables.Holders, store.HolderRow{PoolAddress: "pool", HolderAddress: fmt.Sprintf("wallet-%05d", i), Balance: strconv.Itoa(999999 - i), PctOfSupply: "1", LastUpdated: 1})
	}
	path := "/stats/pool/holders"
	repair1039InvalidPages(t, router, path, 500)
	for _, offset := range []int{0, 500, 10000} {
		result := repair1039Get(t, router, fmt.Sprintf("%s?limit=500&offset=%d", path, offset))
		var holders []struct {
			store.HolderRow
			Rank int `json:"rank"`
		}
		if err := json.Unmarshal(result["holders"], &holders); err != nil {
			t.Fatal(err)
		}
		if len(holders) != 500 || holders[0].Rank != offset+1 || holders[0].HolderAddress != fmt.Sprintf("wallet-%05d", offset) {
			t.Fatal("holder boundary", offset, holders[0])
		}
	}
	canonical := chi.NewRouter()
	httpapi.RegisterTransferHolders(canonical, nil, nil)
	response := serve(t, canonical, "GET", "/v1/pools/pool/holders", nil, nil)
	if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatal("canonical holder route changed")
	}
}

func repair1039Distribution(t *testing.T) {
	t.Helper()
	d, pool := repair1039Database(t)
	st := store.New(pool)
	creator := "0x" + strings.Repeat("4", 40)
	repair1039Pool(t, st, "0xpool", &creator)
	const population = 1000000
	accumulator, err := holdings.NewAccumulator(strconv.Itoa(population))
	if err != nil {
		t.Fatal(err)
	}
	d.Canonical = make([]holdings.Balance, 0, population)
	for i := 0; i < population; i++ {
		v := holdings.Balance{Address: fmt.Sprintf("0x%040x", i+1), Raw: "1", Category: "external_wallet"}
		if err := accumulator.Add(v, false); err != nil {
			t.Fatal(err)
		}
		d.Canonical = append(d.Canonical, v)
	}
	d.Distribution, err = accumulator.Finish()
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	httpapi.RegisterPoolHolders(router, st)
	path := "/stats/0xpool/holders/distribution"
	repair1039InvalidPages(t, router, path, 500)
	before := d.WalletVisits
	first := repair1039Get(t, router, path+"?limit=7")
	var wallets []struct {
		Address string `json:"address"`
		Rank    int    `json:"rank"`
	}
	_ = json.Unmarshal(first["walletMap"], &wallets)
	var aggregate holdings.Distribution
	_ = json.Unmarshal(first["distribution"], &aggregate)
	if len(wallets) != 7 || aggregate.AllPositiveCount != population || aggregate.Bands[0].Count != population || aggregate.Bands[0].Raw != "1000000" || aggregate.Top10.BasisPoints == nil || *aggregate.Top10.BasisPoints != "0" {
		t.Fatalf("canonical distribution: wallets=%d aggregate=%+v", len(wallets), aggregate)
	}
	cursor := repair1039Next(t, first)
	if cursor == "" {
		t.Fatal("missing distribution cursor")
	}
	second := repair1039Get(t, router, path+"?limit=7&cursor="+url.QueryEscape(cursor))
	_ = json.Unmarshal(second["walletMap"], &wallets)
	if len(wallets) != 7 || wallets[0].Rank != 8 || wallets[0].Address != fmt.Sprintf("0x%040x", 8) {
		t.Fatal("numeric wallet cursor continuity", wallets)
	}
	if work := d.WalletVisits - before; work != 16 {
		t.Fatalf("small pages visited %d of %d source wallets; want 16 including lookahead", work, population)
	}
	t.Logf("million-wallet fixture: two limit=7 pages, %d wallet visits including lookahead; production SQL plan/capacity remains an owning-domain qualification", d.WalletVisits-before)
	// Changing a revision invalidates continuation instead of mixing snapshots.
	raw, _ := base64DecodeRepair1039(cursor)
	var c map[string]any
	_ = json.Unmarshal(raw, &c)
	c["revision"] = float64(6)
	bad := encodeRepair1039(repair1039JSON(c))
	response := serve(t, router, "GET", path+"?cursor="+bad, nil, nil)
	if response.Code != 409 {
		t.Fatal("ownership cursor mixed revisions", response.Code)
	}
	zero := repair1039Get(t, router, path+"?limit=0")
	_ = json.Unmarshal(zero["walletMap"], &wallets)
	if len(wallets) != 0 {
		t.Fatal("zero wallet page not empty")
	}
}
func TestRepairM_1039_DistributionUnbounded(t *testing.T) { repair1039Distribution(t) }
func repair1039Whales(t *testing.T) {
	t.Helper()
	d, pool := repair1039Database(t)
	st := store.New(pool)
	creator := "0x" + strings.Repeat("1", 40)
	repair1039Pool(t, st, "0xpool", &creator)
	d.Canonical = []holdings.Balance{{Address: creator, Raw: "700", Category: "external_wallet"}, {Address: "0x" + strings.Repeat("2", 40), Raw: "299", Category: "external_wallet"}, {Address: "0x" + strings.Repeat("4", 40), Raw: "1", Category: "external_wallet"}}
	d.Distribution.Supply = "1000"
	router := chi.NewRouter()
	httpapi.RegisterPoolAnalytics(router, st)
	path := "/stats/0xpool/whales"
	repair1039InvalidPages(t, router, path, 100)
	first := repair1039Get(t, router, path+"?limit=1")
	var whales []struct {
		HolderAddress string `json:"holderAddress"`
		IsCreator     bool   `json:"isCreator"`
		PctOfSupply   int64  `json:"pctOfSupply"`
	}
	_ = json.Unmarshal(first["whales"], &whales)
	if len(whales) != 1 || whales[0].HolderAddress != creator || !whales[0].IsCreator || whales[0].PctOfSupply != 7000 {
		t.Fatal("canonical whale/creator", whales)
	}
	cursor := repair1039Next(t, first)
	if cursor == "" {
		t.Fatal("missing whale cursor")
	}
	second := repair1039Get(t, router, path+"?limit=1&cursor="+url.QueryEscape(cursor))
	_ = json.Unmarshal(second["whales"], &whales)
	if len(whales) != 1 || whales[0].PctOfSupply != 2990 || repair1039Next(t, second) != "" {
		t.Fatal("whale threshold or page bound", whales)
	}
}
func TestRepairM_1039_WhalePageBound(t *testing.T) { repair1039Whales(t) }

// Admission uses a recorded RESP peer; Redis never supplies vote/tally state.
func repair1039Admission(t *testing.T) *redis.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var connections sync.WaitGroup
	client := redis.NewClient(&redis.Options{Addr: listener.Addr().String(), Protocol: 2, DisableIdentity: true, MaxRetries: -1})
	t.Cleanup(func() { client.Close(); listener.Close(); connections.Wait() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer conn.Close()
				r := bufio.NewReader(conn)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
					if err != nil {
						return
					}
					args := make([]string, n)
					for i := range args {
						line, err = r.ReadString('\n')
						if err != nil {
							return
						}
						size, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "$")))
						raw := make([]byte, size+2)
						if _, err = io.ReadFull(r, raw); err != nil {
							return
						}
						args[i] = string(raw[:size])
					}
					switch strings.ToLower(args[0]) {
					case "hello":
						_, err = io.WriteString(conn, "-ERR unknown command 'hello'\r\n")
					case "client":
						_, err = io.WriteString(conn, "+OK\r\n")
					case "eval":
						_, err = io.WriteString(conn, ":1\r\n")
					case "ping":
						_, err = io.WriteString(conn, "+PONG\r\n")
					default:
						t.Errorf("vote state unexpectedly reached admission/cache peer: %q", args)
						_, err = io.WriteString(conn, "-ERR unsupported transcript\r\n")
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	return client
}

type repair1039Reactions struct {
	d         *repair1039DB
	pool      *pgxpool.Pool
	key       *ecdsa.PrivateKey
	wallet    string
	identity  auth.HTTPIdentityAuthority
	verifier  auth.WalletOperationVerifier
	admission *httpapi.ReactionAdmission
	router    http.Handler
}

func repair1039ReactionPath(t *testing.T) *repair1039Reactions {
	t.Helper()
	d, pool := repair1039Database(t)
	key, wallet := newWallet(t)
	wallet = strings.ToLower(wallet)
	principal := auth.IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: wallet, AuthEpoch: 1, ProofAt: time.Now().Add(-time.Minute), AccessExpiresAt: time.Now().Add(time.Hour)}
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) {
			w.WriteHeader(403)
			return
		}
		if r.URL.Path == "/internal/v2/identity/request:authenticate" {
			var in auth.IdentityAuthenticationRequest
			if json.NewDecoder(r.Body).Decode(&in) != nil || in.Authorization != "Bearer wallet-session" {
				w.WriteHeader(403)
				return
			}
			_ = json.NewEncoder(w).Encode(principal)
			return
		}
		if r.URL.Path == "/internal/v2/identity/session:check" {
			var in auth.StreamPrincipal
			if json.NewDecoder(r.Body).Decode(&in) != nil || in != principal.StreamPrincipal() {
				w.WriteHeader(403)
				return
			}
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(identity.Close)
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			w.WriteHeader(400)
			return
		}
		var result any
		switch in.Method {
		case "eth_chainId":
			result = "0x7d"
		case "eth_getCode":
			result = "0x"
		case "eth_getBlockByNumber":
			header := &types.Header{Number: big.NewInt(100), Difficulty: big.NewInt(1), GasLimit: 30000000, Extra: []byte{}}
			raw, _ := json.Marshal(header)
			var block map[string]any
			_ = json.Unmarshal(raw, &block)
			block["hash"] = "0x" + strings.Repeat("a", 64)
			result = block
		default:
			t.Errorf("unexpected RPC transcript %s", in.Method)
			w.WriteHeader(500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": in.ID, "result": result})
	}))
	t.Cleanup(rpcServer.Close)
	rpcClient, err := rpc.Dial(rpcServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpcClient.Close)
	path := &repair1039Reactions{d: d, pool: pool, key: key, wallet: wallet, identity: auth.HTTPIdentityAuthority{Endpoint: identity.URL, ServiceToken: strings.Repeat("s", 32)}, verifier: auth.WalletOperationVerifier{Backend: auth.NewReportedHashBackend(rpcClient)}, admission: &httpapi.ReactionAdmission{Redis: repair1039Admission(t), Namespace: "repair1039", WalletPerMinute: 12, IPPerMinute: 100}}
	path.register(t)
	return path
}
func (p *repair1039Reactions) register(t *testing.T) {
	router := chi.NewRouter()
	httpapi.RegisterReactions(router, nil)
	policies := auth.PolicyRegistry{Version: "repair1039", Actions: map[string]auth.Policy{"reaction.replace": {Scope: "wallet", OwnerRequired: true, RouteTemplate: "/v2/stats/{poolAddress}/reactions"}}}
	if err := httpapi.RegisterDurableReactions(router, httpapi.DurableReactionDeps{Authority: analytics.ReactionAuthority{Pool: p.pool, Schema: "stats"}, Ledger: auth.Ledger{Pool: p.pool, Environment: "repair1039", AuditKey: bytes.Repeat([]byte{1}, 32)}, Identity: p.identity, Verifier: p.verifier, Policies: policies, Admission: p.admission}); err != nil {
		t.Fatal(err)
	}
	p.router = router
}

const repair1039ReactionPool = "0x1111111111111111111111111111111111111111"

func (p *repair1039Reactions) operation(t *testing.T, reaction string, revision int64, nonceByte byte) (analytics.Intent, auth.OperationEnvelope, string) {
	t.Helper()
	intent := analytics.Intent{ChainID: "125", Pool: repair1039ReactionPool, Wallet: p.wallet, Reaction: reaction, Nonce: strings.Repeat(string(nonceByte), 64), Expiry: time.Now().Add(4 * time.Minute).Unix(), ExpectedRevision: revision}
	body := repair1039JSON(intent)
	hash, err := auth.BodyDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	envelope := auth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "repair1039", Actor: p.wallet, Action: "reaction.replace", Method: "POST", Path: "/v2/stats/" + intent.Pool + "/reactions", BodyHash: hash, Nonce: intent.Nonce, IssuedAt: time.Now().Unix(), ExpiresAt: intent.Expiry}
	message, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	return intent, envelope, signWallet(t, p.key, message)
}
func (p *repair1039Reactions) send(t *testing.T, method, path string, intent analytics.Intent, envelope auth.OperationEnvelope, signature string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, p.router, method, path, repair1039JSON(intent), map[string]string{"Content-Type": "application/json", "Authorization": "Bearer wallet-session", "X-Operation-Envelope": string(repair1039JSON(envelope)), "X-Operation-Signature": signature})
}
func repair1039RequireStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status %d want %d: %s", response.Code, status, response.Body.String())
	}
}
func repair1039LegacyRetirement(t *testing.T) {
	t.Helper()
	router := chi.NewRouter()
	httpapi.RegisterReactions(router, nil)
	key, wallet := newWallet(t)
	for _, reaction := range []string{"bullish", "hot", "clear"} {
		intent := analytics.Intent{ChainID: "125", Pool: repair1039ReactionPool, Wallet: strings.ToLower(wallet), Reaction: reaction, Nonce: strings.Repeat("a", 64), Expiry: time.Now().Add(time.Minute).Unix()}
		raw, err := intent.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		message := "KindleLaunch reaction v1\n" + string(raw)
		response := serve(t, router, "POST", "/stats/"+repair1039ReactionPool+"/reactions", mustJSON(t, map[string]string{"walletAddress": wallet, "reaction": reaction, "message": message, "signature": signWallet(t, key, message)}), nil)
		repair1039RequireStatus(t, response, 410)
		if response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Header().Get("Link"), "/v2/stats/") || !strings.Contains(response.Body.String(), "fresh signed") {
			t.Fatal("missing explicit signed-operation migration", response.Body.String())
		}
	}
	for _, method := range []string{"GET", "POST"} {
		repair1039RequireStatus(t, serve(t, router, method, "/stats/"+repair1039ReactionPool+"/reactions", nil, nil), 410)
	}
}
func TestRepairM_1039_LegacyReactionSignature(t *testing.T) {
	repair1039LegacyRetirement(t)
	p := repair1039ReactionPath(t)
	intent, envelope, signature := p.operation(t, "bullish", 0, 'a')
	path := envelope.Path
	repair1039RequireStatus(t, p.send(t, "POST", path, intent, envelope, signature), 200)
	for _, field := range []string{"pool", "reaction", "method", "expiry", "nonce", "environment"} {
		t.Run(field, func(t *testing.T) {
			changed, wrapped, target, method := intent, envelope, path, "POST"
			switch field {
			case "pool":
				changed.Pool = "0x" + strings.Repeat("2", 40)
				target = "/v2/stats/" + changed.Pool + "/reactions"
			case "reaction":
				changed.Reaction = "hot"
			case "method":
				wrapped.Method = "DELETE"
			case "expiry":
				wrapped.ExpiresAt--
			case "nonce":
				wrapped.Nonce = strings.Repeat("b", 64)
			case "environment":
				wrapped.Environment = "other"
			}
			repair1039RequireStatus(t, p.send(t, method, target, changed, wrapped, signature), 403)
		})
	}
	repair1039RequireStatus(t, p.send(t, "DELETE", path, intent, envelope, signature), 405)
	response := p.send(t, "POST", path, intent, envelope, signature)
	repair1039RequireStatus(t, response, 200)
	if response.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("identical replay mutated vote")
	}
}

func repair1039DurableReplacement(t *testing.T) {
	t.Helper()
	p := repair1039ReactionPath(t)
	intent, envelope, signature := p.operation(t, "bullish", 0, 'a')
	first := p.send(t, "POST", envelope.Path, intent, envelope, signature)
	repair1039RequireStatus(t, first, 200)
	// Signed replacements contend through the real authority and ledger. The
	// committed table fixture serializes transactions and injects commit failure.
	p.d.mu.Lock()
	p.d.Now = p.d.Now.Add(6 * time.Second)
	p.d.mu.Unlock()
	hot, hotEnv, hotSig := p.operation(t, "hot", 1, 'b')
	diamond, diamondEnv, diamondSig := p.operation(t, "diamond", 1, 'c')
	results := make(chan *httptest.ResponseRecorder, 2)
	go func() { results <- p.send(t, "POST", hotEnv.Path, hot, hotEnv, hotSig) }()
	go func() { results <- p.send(t, "POST", diamondEnv.Path, diamond, diamondEnv, diamondSig) }()
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.Code == 200 {
			success++
		} else if r.Code == 429 {
			conflict++
		} else {
			t.Fatal("replacement result", r.Code, r.Body.String())
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("simultaneous replacement", success, conflict)
	}
	p.d.mu.Lock()
	prior := p.d.Tables.Votes[intent.Pool+p.wallet]
	p.d.Now = p.d.Now.Add(6 * time.Second)
	p.d.mu.Unlock()
	stale, staleEnv, staleSig := p.operation(t, "trash", 1, 'e')
	repair1039RequireStatus(t, p.send(t, "POST", staleEnv.Path, stale, staleEnv, staleSig), 409)
	p.d.mu.Lock()
	p.d.FailLedger = true
	p.d.mu.Unlock()
	clear, clearEnv, clearSig := p.operation(t, "clear", 2, 'd')
	repair1039RequireStatus(t, p.send(t, "POST", clearEnv.Path, clear, clearEnv, clearSig), 503)
	p.d.mu.Lock()
	after := p.d.Tables.Votes[intent.Pool+p.wallet]
	p.d.mu.Unlock()
	if after.Vote != prior.Vote || after.Revision != prior.Revision {
		t.Fatal("failed ledger transaction leaked a vote")
	}
	p.d.mu.Lock()
	outboxCount, auditCount, ledgerCount := len(p.d.Tables.Outbox), len(p.d.Tables.Audit), len(p.d.Tables.Ledger)
	p.d.mu.Unlock()
	if outboxCount != 2 || auditCount != 2 || ledgerCount != 2 {
		t.Fatal("failed transaction leaked durable publication/audit", outboxCount, auditCount, ledgerCount)
	}
	// Simulate lost publication/response by retaining the signed operation while
	// dropping every HTTP/pgx component. Redis contains admission only.
	p.pool = p.d.restart(p.pool)
	p.register(t)
	if len(p.d.Tables.Outbox) != 2 || len(p.d.Tables.Audit) != 2 || len(p.d.Tables.Ledger) != 2 {
		t.Fatal("outbox, audit or ledger lost across reopen")
	}
	read := repair1039Get(t, p.router, envelope.Path)
	var counts map[string]int64
	_ = json.Unmarshal(read["reactions"], &counts)
	total := int64(0)
	for _, n := range counts {
		total += n
	}
	if total != 1 || counts[prior.Vote] != 1 {
		t.Fatal("recovered tally disagrees with durable vote", counts, prior)
	}
	replay := p.send(t, "POST", envelope.Path, intent, envelope, signature)
	repair1039RequireStatus(t, replay, 200)
	if !bytes.Equal(replay.Body.Bytes(), first.Body.Bytes()) || replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("operation replay lost after reopen", replay.Body.String())
	}
	repair1039RequireStatus(t, p.send(t, "POST", clearEnv.Path, clear, clearEnv, clearSig), 200)
	final := repair1039Get(t, p.router, envelope.Path)
	_ = json.Unmarshal(final["reactions"], &counts)
	for _, n := range counts {
		if n != 0 {
			t.Fatal("clear did not remove tally")
		}
	}
	// A changed intent with the old signature never authorizes replacement.
	clear.Reaction = "hot"
	repair1039RequireStatus(t, p.send(t, "POST", clearEnv.Path, clear, clearEnv, clearSig), 403)
}
func TestRepairM_1039_LegacyReactionState(t *testing.T) {
	repair1039LegacyRetirement(t)
	repair1039DurableReplacement(t)
}
func TestRepairM_1039_ReactionTestWrongIntent(t *testing.T) { repair1039DurableReplacement(t) }

func base64DecodeRepair1039(raw string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(raw)
}
func encodeRepair1039(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

func repair1039Bool(raw string) bool {
	value, err := strconv.ParseBool(raw)
	if err != nil {
		panic(fmt.Sprintf("invalid PostgreSQL boolean transcript %q", raw))
	}
	return value
}
