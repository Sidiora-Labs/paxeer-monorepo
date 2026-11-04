package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This database peer holds the committed bootstrap inputs and durable rows.
// It speaks pgwire to the real coordinator and ledger; it is not a SQL engine.
// Unrecorded queries fail. DDL is admitted only as exact registered source bytes,
// as pnl_owner under the coordinator's locks and transaction timeout.
type repairM917Table struct {
	Columns []string
	Types   []uint32
	Key     []string
	Rights  map[string]bool
	Rows    [][]string
}
type repairM917State struct {
	Tables   map[string]*repairM917Table
	Applied  map[int64]string
	History  []int64
	Attempts map[string]string
	DDL      []string
	Legacy   map[string]string
}
type repairM917Peer struct {
	mu       sync.Mutex
	State    repairM917State
	file     string
	sources  map[string]string
	listener net.Listener
	wg       sync.WaitGroup
}
type repairM917Session struct {
	user, role string
	state      *repairM917State
	timeout    bool
	locked     bool
}
type repairM917Result struct {
	names []string
	types []uint32
	rows  [][]string
	tag   string
}

func repairM917Rows(types []uint32, rows ...[]string) repairM917Result {
	return repairM917Result{types: types, rows: rows, tag: fmt.Sprintf("SELECT %d", len(rows))}
}
func repairM917Copy(state repairM917State) repairM917State {
	raw, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	var copy repairM917State
	if err := json.Unmarshal(raw, &copy); err != nil {
		panic(err)
	}
	return copy
}
func repairM917Database(t *testing.T, file string, sources map[string]string) *repairM917Peer {
	t.Helper()
	p := &repairM917Peer{file: file, sources: sources, State: repairM917State{Tables: map[string]*repairM917Table{}, Applied: map[int64]string{}, Attempts: map[string]string{}, Legacy: map[string]string{}}}
	raw, err := os.ReadFile(file)
	if err == nil {
		if err = json.Unmarshal(raw, &p.State); err != nil {
			t.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	p.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			c, err := p.listener.Accept()
			if err != nil {
				return
			}
			p.wg.Add(1)
			go func() { defer p.wg.Done(); defer c.Close(); p.serve(c) }()
		}
	}()
	t.Cleanup(p.stop)
	return p
}
func (p *repairM917Peer) stop() { p.listener.Close(); p.wg.Wait() }
func (p *repairM917Peer) pool(t *testing.T, user string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://" + user + "@" + p.listener.Addr().String() + "/reward?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	cfg.ConnConfig.ConnectTimeout = time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func (p *repairM917Peer) snapshot() repairM917State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return repairM917Copy(p.State)
}
func (p *repairM917Peer) persist() error {
	raw, err := json.Marshal(p.State)
	if err != nil {
		return err
	}
	if err = os.WriteFile(p.file+".next", raw, 0600); err != nil {
		return err
	}
	return os.Rename(p.file+".next", p.file)
}
func repairM917Split(s string) []string {
	var out []string
	depth, start := 0, 0
	quoted := false
	for i, c := range s {
		if c == '\'' {
			quoted = !quoted
		}
		if quoted {
			continue
		}
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

var repairM917Create = regexp.MustCompile(`CREATE TABLE (?:"pnl"\.)?([a-z][a-z0-9_]*)\s*\(`)
var repairM917Type = map[string]uint32{"text": 25, "text[]": 1009, "uuid": 2950, "bytea": 17, "timestamptz": 1184, "jsonb": 3802, "bigint": 20, "bigserial": 20, "integer": 23, "boolean": 16, "numeric": 1700, "varchar": 1043, "date": 1082}

func repairM917Catalog(state *repairM917State, sql string) error {
	for _, loc := range repairM917Create.FindAllStringSubmatchIndex(sql, -1) {
		name := sql[loc[2]:loc[3]]
		start := loc[1]
		depth, end := 1, start
		for ; end < len(sql) && depth > 0; end++ {
			switch sql[end] {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		if depth != 0 {
			return fmt.Errorf("unclosed table %s", name)
		}
		if state.Tables[name] != nil {
			return fmt.Errorf("duplicate table %s", name)
		}
		table := &repairM917Table{Rights: map[string]bool{}}
		for _, column := range repairM917Split(sql[start : end-1]) {
			f := strings.Fields(column)
			if strings.HasPrefix(column, "PRIMARY KEY(") {
				table.Key = strings.Split(strings.TrimSuffix(strings.TrimPrefix(column, "PRIMARY KEY("), ")"), ",")
				continue
			}
			if len(f) > 0 && (f[0] == "CHECK" || f[0] == "FOREIGN" || f[0] == "UNIQUE" || strings.HasPrefix(f[0], "UNIQUE(") || strings.HasPrefix(f[0], "CHECK(")) {
				continue
			}
			if len(f) < 2 {
				return fmt.Errorf("invalid column %s", column)
			}
			typ := strings.Split(f[1], "(")[0]
			oid, ok := repairM917Type[typ]
			if !ok {
				return fmt.Errorf("unrecorded type %s", typ)
			}
			table.Columns = append(table.Columns, f[0])
			table.Types = append(table.Types, oid)
			if strings.Contains(column, "PRIMARY KEY") {
				table.Key = []string{f[0]}
			}
		}
		state.Tables[name] = table
	}
	return nil
}
func repairM917Signature(sql string) ([]uint32, []uint32) {
	switch {
	case strings.HasPrefix(sql, "SELECT pg_try_advisory"), strings.HasPrefix(sql, "SELECT pg_advisory_unlock"):
		return []uint32{20}, []uint32{16}
	case strings.HasPrefix(sql, "SELECT migration_id,checksum"):
		return nil, []uint32{20, 25}
	case strings.HasPrefix(sql, "INSERT INTO migration_control.migration_attempts"):
		return []uint32{20, 25, 25, 20}, []uint32{20}
	case strings.HasPrefix(sql, "UPDATE migration_control.migration_attempts"):
		return []uint32{20, 25, 20, 25}, nil
	case strings.HasPrefix(sql, "INSERT INTO migration_control.applied_migrations"):
		return []uint32{20, 25, 25, 25, 25}, nil
	case strings.HasPrefix(sql, "INSERT INTO migration_control.pnl_tracker_versions"):
		return []uint32{20}, nil
	case strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock"):
		return []uint32{25}, []uint32{2278}
	case strings.Contains(sql, `"recovery".assert_epoch`):
		return []uint32{20, 25}, []uint32{2278}
	case strings.Contains(sql, "has_table_privilege"):
		return []uint32{25, 1009}, []uint32{16, 16}
	case strings.HasPrefix(sql, "SELECT digest,response,expires_at,release_digest,interface_digest"):
		return []uint32{25, 2950}, []uint32{25, 17, 1184, 25, 25}
	case strings.HasPrefix(sql, "SELECT creator,pool"):
		return []uint32{1700}, []uint32{25, 25}
	case strings.HasPrefix(sql, "SELECT nft_id::text"):
		return []uint32{1700}, []uint32{25, 25, 25, 20, 25, 25, 25, 25}
	case strings.HasPrefix(sql, "INSERT INTO l18_requests"):
		return []uint32{25, 2950, 25, 17, 1184, 25, 25}, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_audit"):
		return []uint32{25, 25, 3802}, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_outbox"):
		return []uint32{25, 25, 3802}, nil
	}
	return nil, nil
}
func (p *repairM917Peer) query(s *repairM917Session, sql string, args []string) (repairM917Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := &p.State
	if s.state != nil {
		state = s.state
	}
	ok := repairM917Result{tag: "SELECT 0"}
	normalized := strings.Join(strings.Fields(sql), " ")
	switch {
	case sql == production.MigrationControlDDL() || sql == production.OnlineIndexControlDDL() || sql == "CREATE ROLE pnl_owner NOLOGIN; GRANT USAGE, CREATE ON SCHEMA pnl TO pnl_owner":
		if s.user != "migration_coordinator" || !s.locked {
			return ok, fmt.Errorf("bootstrap escaped coordinator")
		}
		return repairM917Result{tag: "CREATE TABLE"}, nil
	case strings.HasPrefix(sql, "SET LOCAL ROLE migration_control_owner; CREATE TABLE "):
		history, err := production.HistoryDDL("core/pnl-tracker", "pnl")
		if err != nil || history != sql || s.state == nil || s.user != "migration_coordinator" {
			return ok, fmt.Errorf("unrecorded history bootstrap")
		}
		return repairM917Result{tag: "CREATE TABLE"}, nil
	case normalized == "begin" || normalized == "begin read only":
		if s.state != nil {
			return ok, fmt.Errorf("nested transaction")
		}
		copy := repairM917Copy(p.State)
		s.state = &copy
		s.timeout = false
		return repairM917Result{tag: "BEGIN"}, nil
	case normalized == "commit":
		if s.state == nil {
			return ok, fmt.Errorf("commit outside transaction")
		}
		p.State = *s.state
		s.state = nil
		s.role = ""
		s.timeout = false
		return repairM917Result{tag: "COMMIT"}, p.persist()
	case normalized == "rollback":
		s.state = nil
		s.role = ""
		s.timeout = false
		return repairM917Result{tag: "ROLLBACK"}, nil
	case normalized == "SET LOCAL lock_timeout='2s'":
		if s.state == nil {
			return ok, fmt.Errorf("timeout outside transaction")
		}
		s.timeout = true
		return repairM917Result{tag: "SET"}, nil
	case normalized == `SET LOCAL ROLE "pnl_owner"`:
		if s.user != "migration_coordinator" || s.state == nil {
			return ok, fmt.Errorf("owner outside coordinator transaction")
		}
		s.role = "pnl_owner"
		return repairM917Result{tag: "SET"}, nil
	case normalized == "RESET ROLE":
		s.role = ""
		return repairM917Result{tag: "RESET"}, nil
	case normalized == `SET LOCAL search_path TO "pnl"`:
		return repairM917Result{tag: "SET"}, nil
	case strings.HasPrefix(sql, "SELECT pg_try_advisory_lock"):
		if s.user != "migration_coordinator" {
			return ok, fmt.Errorf("runtime coordinator lock")
		}
		s.locked = true
		return repairM917Rows([]uint32{16}, []string{"t"}), nil
	case strings.HasPrefix(sql, "SELECT pg_try_advisory_xact_lock"):
		return repairM917Rows([]uint32{16}, []string{strconv.FormatBool(s.locked && s.state != nil)}), nil
	case strings.HasPrefix(sql, "SELECT pg_advisory_unlock"):
		locked := s.locked
		s.locked = false
		return repairM917Rows([]uint32{16}, []string{strconv.FormatBool(locked)}), nil
	case strings.HasPrefix(sql, "SELECT migration_id,checksum FROM migration_control.applied_migrations"):
		r := repairM917Rows([]uint32{20, 25})
		ids := []int64{}
		for id := range state.Applied {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			r.rows = append(r.rows, []string{strconv.FormatInt(id, 10), state.Applied[id]})
		}
		r.tag = fmt.Sprintf("SELECT %d", len(r.rows))
		return r, nil
	case strings.HasPrefix(sql, "INSERT INTO migration_control.migration_attempts"):
		if !s.locked || s.user != "migration_coordinator" {
			return ok, fmt.Errorf("attempt without coordinator lock")
		}
		id := strconv.Itoa(len(p.State.Attempts) + 1)
		p.State.Attempts[id] = "started"
		return repairM917Rows([]uint32{20}, []string{id}), p.persist()
	case strings.HasPrefix(sql, "UPDATE migration_control.migration_attempts"):
		if p.State.Attempts[args[0]] != "started" {
			return ok, fmt.Errorf("attempt not started")
		}
		p.State.Attempts[args[0]] = args[1]
		return repairM917Result{tag: "UPDATE 1"}, p.persist()
	case strings.HasPrefix(sql, "INSERT INTO migration_control.pnl_tracker_versions"):
		id, e := strconv.ParseInt(args[0], 10, 64)
		if e != nil {
			return ok, e
		}
		state.History = append(state.History, id)
		return repairM917Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, "INSERT INTO migration_control.applied_migrations"):
		if s.state == nil || s.role != "" {
			return ok, fmt.Errorf("migration history outside reset owner transaction")
		}
		id, e := strconv.ParseInt(args[0], 10, 64)
		if e != nil {
			return ok, e
		}
		if state.Applied[id] != "" {
			return ok, fmt.Errorf("duplicate migration")
		}
		state.Applied[id] = args[3]
		return repairM917Result{tag: "INSERT 0 1"}, nil
	}
	if source, registered := p.sources[sql]; registered {
		if s.user != "migration_coordinator" || s.state == nil || s.role != "pnl_owner" || !s.timeout || !s.locked {
			return ok, fmt.Errorf("DDL escaped coordinated owner transaction")
		}
		if e := repairM917Catalog(state, sql); e != nil {
			return ok, e
		}
		state.DDL = append(state.DDL, source)
		return repairM917Result{tag: "CREATE TABLE"}, nil
	}
	if s.user != "pnl_runtime" {
		return ok, fmt.Errorf("unrecorded coordinator SQL: %s", normalized)
	}
	switch {
	case strings.Contains(sql, `"recovery".assert_epoch`):
		if s.state == nil || len(args) != 2 || args[0] != "1" || args[1] != "fixture-loopback" {
			return ok, fmt.Errorf("epoch fence differs")
		}
		return repairM917Rows([]uint32{2278}, []string{""}), nil
	case strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock"):
		if s.state == nil {
			return ok, fmt.Errorf("request lock outside transaction")
		}
		return repairM917Rows([]uint32{2278}, []string{""}), nil
	case strings.Contains(sql, "has_table_privilege"):
		name := strings.TrimPrefix(strings.ReplaceAll(args[0], `"`, ""), "pnl.")
		table := state.Tables[name]
		if table == nil {
			return ok, fmt.Errorf("missing table %s", name)
		}
		var wanted []string
		if err := pgtype.NewMap().Scan(1009, 0, []byte(args[1]), &wanted); err != nil {
			return ok, err
		}
		return repairM917Rows([]uint32{16, 16}, []string{strconv.FormatBool(table.Rights["SELECT"] && table.Rights["INSERT"]), strconv.FormatBool(strings.Join(table.Key, ",") == strings.Join(wanted, ","))}), nil
	case strings.HasSuffix(normalized, " LIMIT 0"):
		re := regexp.MustCompile(`^SELECT (.+) FROM "pnl"\."([a-z0-9_]+)" LIMIT 0$`)
		m := re.FindStringSubmatch(normalized)
		if m == nil {
			return ok, fmt.Errorf("unrecorded catalog projection %s", sql)
		}
		table := state.Tables[m[2]]
		if table == nil {
			return ok, fmt.Errorf("relation %s does not exist", m[2])
		}
		r := repairM917Rows(nil)
		r.names = strings.Split(strings.ReplaceAll(m[1], `"`, ""), ",")
		for _, name := range r.names {
			found := false
			for i, c := range table.Columns {
				if c == name {
					r.types = append(r.types, table.Types[i])
					found = true
					break
				}
			}
			if !found {
				return ok, fmt.Errorf("missing column %s", name)
			}
		}
		return r, nil
	case strings.HasPrefix(sql, "SELECT digest,response,expires_at,release_digest,interface_digest FROM l18_requests"):
		table := state.Tables["l18_requests"]
		if table == nil || !table.Rights["SELECT"] {
			return ok, fmt.Errorf("request storage unavailable")
		}
		r := repairM917Rows([]uint32{25, 17, 1184, 25, 25})
		for _, row := range table.Rows {
			if row[0] == args[0] && row[1] == args[1] {
				r.rows = append(r.rows, []string{row[2], row[5], row[6], row[7], row[8]})
			}
		}
		r.tag = fmt.Sprintf("SELECT %d", len(r.rows))
		return r, nil
	case strings.Contains(sql, "FROM indexer.indexer_canonical_creations_v1 WHERE nft_id=$1::numeric"):
		if args[0] != "1" {
			return repairM917Rows([]uint32{25, 25}), nil
		}
		creator, pool := state.Legacy["creator"], state.Legacy["pool"]
		if strings.HasPrefix(sql, "SELECT creator,pool") {
			return repairM917Rows([]uint32{25, 25}, []string{creator, pool}), nil
		}
		return repairM917Rows([]uint32{25, 25, 25, 20, 25, 25, 25, 25}, []string{"1", pool, creator, "10", fmt.Sprintf("0x%064x", 10), fmt.Sprintf("0x%064x", 1000001), "0", "1"}), nil
	case strings.HasPrefix(sql, "INSERT INTO l18_requests"), strings.HasPrefix(sql, "INSERT INTO l18_outbox"), strings.HasPrefix(sql, "INSERT INTO l18_audit"):
		if s.state == nil {
			return ok, fmt.Errorf("write outside transaction")
		}
		name := strings.Split(strings.Fields(sql)[2], "(")[0]
		table := state.Tables[name]
		if table == nil || !table.Rights["INSERT"] {
			return ok, fmt.Errorf("unwritable %s", name)
		}
		row := append([]string(nil), args...)
		if name == "l18_requests" {
			row = []string{args[0], args[1], args[2], "POST", "/v2/me/rewards:prepare-claim", args[3], args[4], args[5], args[6]}
		} else if name == "l18_audit" {
			row = []string{args[0], args[1], "prepare_claim", "wallet signs and broadcasts", args[2]}
		}
		for _, old := range table.Rows {
			if old[0] == row[0] && (name != "l18_requests" || old[1] == row[1]) {
				return ok, fmt.Errorf("duplicate %s identity", name)
			}
		}
		table.Rows = append(table.Rows, row)
		return repairM917Result{tag: "INSERT 0 1"}, nil
	}
	return ok, fmt.Errorf("unrecorded runtime SQL: %s", normalized)
}

func (p *repairM917Peer) serve(conn net.Conn) {
	b := pgproto3.NewBackend(conn, conn)
	raw, err := b.ReceiveStartupMessage()
	if err != nil {
		return
	}
	hello, ok := raw.(*pgproto3.StartupMessage)
	if !ok {
		return
	}
	s := &repairM917Session{user: hello.Parameters["user"]}
	if s.user != "migration_coordinator" && s.user != "pnl_runtime" {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if b.Flush() != nil {
		return
	}
	typeMap := pgtype.NewMap()
	statements := map[string]string{}
	var sql string
	var args []string
	var formats []int16
	var params, fields []uint32
	failed := false
	formatAt := func(codes []int16, i int) int16 {
		if len(codes) == 0 {
			return 0
		}
		if len(codes) == 1 {
			return codes[0]
		}
		return codes[i]
	}
	sendError := func(err error) {
		b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: err.Error()})
		failed = true
	}
	describe := func(types []uint32, names []string) {
		if len(types) == 0 {
			b.Send(&pgproto3.NoData{})
			return
		}
		desc := make([]pgproto3.FieldDescription, len(types))
		for i, oid := range types {
			name := fmt.Sprintf("column%d", i)
			if len(names) > i {
				name = names[i]
			}
			desc[i] = pgproto3.FieldDescription{Name: []byte(name), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1, Format: formatAt(formats, i)}
		}
		b.Send(&pgproto3.RowDescription{Fields: desc})
	}
	sendRows := func(r repairM917Result) {
		for _, row := range r.rows {
			values := make([][]byte, len(row))
			for i, v := range row {
				format := formatAt(formats, i)
				if format == 0 {
					values[i] = []byte(v)
					continue
				}
				var value any
				if err := typeMap.Scan(r.types[i], 0, []byte(v), &value); err != nil {
					sendError(err)
					return
				}
				encoded, err := typeMap.Encode(r.types[i], format, value, nil)
				if err != nil {
					sendError(err)
					return
				}
				values[i] = encoded
			}
			b.Send(&pgproto3.DataRow{Values: values})
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(r.tag)})
	}
	ready := func() {
		status := byte('I')
		if s.state != nil {
			status = 'T'
			if failed {
				status = 'E'
			}
		}
		b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
	}
	for {
		message, err := b.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
		switch m := message.(type) {
		case *pgproto3.Parse:
			statements[m.Name] = m.Query
			sql = m.Query
			params, fields = repairM917Signature(sql)
			formats = nil
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Describe:
			if m.ObjectType == 'S' {
				sql = statements[m.Name]
				params, fields = repairM917Signature(sql)
				if strings.HasSuffix(sql, " LIMIT 0") {
					r, err := p.query(s, sql, nil)
					if err != nil {
						sendError(err)
						break
					}
					fields = r.types
					b.Send(&pgproto3.ParameterDescription{ParameterOIDs: params})
					describe(fields, r.names)
				} else {
					b.Send(&pgproto3.ParameterDescription{ParameterOIDs: params})
					describe(fields, nil)
				}
			} else {
				describe(fields, nil)
			}
		case *pgproto3.Bind:
			sql = statements[m.PreparedStatement]
			params, fields = repairM917Signature(sql)
			formats = append([]int16(nil), m.ResultFormatCodes...)
			args = make([]string, len(m.Parameters))
			if len(params) != len(args) {
				sendError(fmt.Errorf("parameter signature mismatch for %s", sql))
				break
			}
			for i, raw := range m.Parameters {
				var value any
				if err := typeMap.Scan(params[i], formatAt(m.ParameterFormatCodes, i), raw, &value); err != nil {
					sendError(err)
					break
				}
				text, err := typeMap.Encode(params[i], 0, value, nil)
				if err != nil {
					sendError(err)
					break
				}
				args[i] = string(text)
			}
			if !failed {
				b.Send(&pgproto3.BindComplete{})
			}
		case *pgproto3.Execute:
			if !failed {
				r, err := p.query(s, sql, args)
				if err != nil {
					sendError(err)
				} else {
					sendRows(r)
				}
			}
		case *pgproto3.Sync:
			ready()
			failed = false
		case *pgproto3.Query:
			formats = nil
			r, err := p.query(s, m.String, nil)
			if err != nil {
				sendError(err)
			} else {
				if len(r.types) > 0 {
					describe(r.types, r.names)
				}
				sendRows(r)
			}
			ready()
			failed = false
		case *pgproto3.Close:
			delete(statements, m.Name)
			b.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Terminate:
			return
		default:
			sendError(fmt.Errorf("unrecorded pgwire message %T", m))
		}
		if b.Flush() != nil {
			return
		}
	}
}

func repairM917Grant(t *testing.T, p *repairM917Peer, manifest production.RoleManifest) {
	t.Helper()
	statements, err := manifest.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^GRANT ([A-Z, ]+) ON TABLE "pnl"\."([a-z0-9_]+)" TO "pnl_runtime"$`)
	wanted := map[string]bool{"l18_requests": false, "l18_audit": false, "l18_outbox": false}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, statement := range statements {
		if m := pattern.FindStringSubmatch(statement); m != nil {
			if _, needed := wanted[m[2]]; !needed {
				continue
			}
			table := p.State.Tables[m[2]]
			if table == nil {
				t.Fatalf("runtime grant references absent reward table %s", m[2])
			}
			for _, right := range strings.Split(m[1], ", ") {
				table.Rights[right] = true
			}
			wanted[m[2]] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Fatalf("runtime manifest omits reward privileges for %s", name)
		}
	}
	if err := p.persist(); err != nil {
		t.Fatal(err)
	}
}
func repairM917EqualJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xx, _ := json.Marshal(x)
	yy, _ := json.Marshal(y)
	return bytes.Equal(xx, yy)
}
