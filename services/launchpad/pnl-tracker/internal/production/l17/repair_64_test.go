package l17

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepair_64_HistoryRowInsertGrant(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../../migrations/grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Roles           []production.RoleManifest `json:"roles"`
		OwnerMembership bool                      `json:"owner_membership_for_runtime"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.OwnerMembership {
		t.Fatal("runtime may not inherit schema ownership")
	}
	var role production.RoleManifest
	for _, candidate := range manifest.Roles {
		if candidate.RuntimeRole == "pnl_runtime" {
			role = candidate
		}
	}
	if role.Service != "core/pnl-tracker" || role.Schema != "pnl" || role.OwnerRole == role.RuntimeRole {
		t.Fatal("wrong production runtime", role)
	}
	sql, err := role.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	wantGrant := `GRANT SELECT, INSERT ON TABLE "pnl"."portfolio_history_rows" TO "pnl_runtime"`
	count := 0
	for _, statement := range sql {
		if strings.Contains(statement, `"portfolio_history_rows"`) {
			count++
			if statement != wantGrant {
				t.Fatalf("history grant must be minimal: %s", statement)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected one exact history rows grant, got %d", count)
	}
	prior := role
	prior.Grants = append([]production.Grant(nil), role.Grants...)
	for i := range prior.Grants {
		if prior.Grants[i].Schema == "pnl" && prior.Grants[i].Object == "portfolio_history_rows" {
			prior.Grants[i].Privileges = []string{"SELECT"}
		}
	}
	priorSQL, err := prior.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p := fixture()
	snapshot := "64646464-6464-4646-8646-646464646464"
	source := "repair-64:canonical-wallet-projection"
	summary := Summary{Wallet: wallet, SnapshotID: snapshot, Watermark: p.SourceWatermark}
	expires := time.Now().UTC().Add(time.Hour)
	records := []HistoryRecord{
		{EventID: "125:trade:64:0", Kind: "trade", Asset: p.MarkPrice.BaseAsset, AmountRaw: "9007199254740993", QuoteAmountUsdlRaw: ptr("12345678901234567890"), BasisQuality: "complete", Position: *p.MarkPrice.Source.Position},
		{EventID: "125:transfer:64:1", Kind: "transfer_in", Asset: p.MarkPrice.BaseAsset, AmountRaw: "18446744073709551616", BasisQuality: "unknown", Position: *p.MarkPrice.Source.Position},
	}
	records[1].Position.LogIndex = "2"

	// The peer is the database boundary, not a replacement Repository or pgx.Tx.
	// All persisted JSON and COPY bytes below are emitted by production methods.
	commit := func(repo Repository) (Summary, error) {
		tx, e := repo.Pool.Begin(ctx)
		if e != nil {
			return Summary{}, e
		}
		defer rollbackPortfolio(tx)
		saved, e := repo.CommitSnapshotTx(ctx, tx, source, summary, []Position{p}, expires)
		if e != nil {
			return saved, e
		}
		if e = repo.CommitHistoryTx(ctx, tx, wallet, saved.SnapshotID, source, records); e != nil {
			return saved, e
		}
		return saved, tx.Commit(ctx)
	}

	t.Run("select_only_rejects_copy_and_rolls_back_projection", func(t *testing.T) {
		peer := repair64NewPeer(t, priorSQL)
		repo := peer.repository(t)
		_, e := commit(repo)
		var denied *pgconn.PgError
		if !errors.As(e, &denied) || denied.Code != "42501" {
			t.Fatalf("missing INSERT must fail at real COPY, got %v", e)
		}
		repo.Pool.Close()
		peer.mu.Lock()
		defer peer.mu.Unlock()
		if !peer.copyDenied || len(peer.tables) != 0 {
			t.Fatalf("denied history leaked transaction: copyDenied=%v tables=%v", peer.copyDenied, peer.tables)
		}
	})

	peer := repair64NewPeer(t, sql)
	repo := peer.repository(t)
	saved, err := commit(repo)
	if err != nil {
		t.Fatal("runtime projection must commit nonempty history", err)
	}
	// A new pool and service have no request, transaction or repository state.
	repo.Pool.Close()
	repo = peer.repository(t)
	page, err := repo.HistoryPage(ctx, wallet, wallet, snapshot, -1, 1, time.Now())
	if err != nil || page.Rows != 2 || len(page.Records) != 1 || !reflect.DeepEqual(page.Records[0], records[0]) {
		t.Fatalf("recovered trade history: %+v %v", page, err)
	}
	page, err = repo.HistoryPage(ctx, wallet, wallet, snapshot, 0, 1, time.Now())
	if err != nil || page.Rows != 2 || len(page.Records) != 1 || !reflect.DeepEqual(page.Records[0], records[1]) {
		t.Fatalf("recovered transfer history: %+v %v", page, err)
	}
	if _, err = repo.HistoryPage(ctx, wallet, token, snapshot, -1, 2, time.Now()); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("another owner read private rows", err)
	}
	if _, err = commit(repo); err != nil {
		t.Fatal("idempotent participant replay", err)
	}
	peer.mu.Lock()
	if len(peer.tables["portfolio_snapshots"]) != 1 || len(peer.tables["portfolio_history_snapshots"]) != 1 || len(peer.tables["portfolio_history_rows"]) != 2 || len(peer.tables["portfolio_outbox"]) != 1 || len(peer.tables["portfolio_history_outbox"]) != 1 || peer.copied != 2 {
		t.Errorf("projection, history and notices did not commit exactly once: %+v copied=%d", peer.tables, peer.copied)
	}
	var notice struct {
		Snapshot string `json:"snapshotId"`
		Wallet   string `json:"wallet"`
		Rows     int    `json:"rows"`
	}
	err = json.Unmarshal(peer.tables["portfolio_history_outbox"][0][1], &notice)
	peer.mu.Unlock()
	if err != nil || notice.Snapshot != snapshot || notice.Wallet != wallet || notice.Rows != 2 {
		t.Fatal("history notification lost binding", notice, err)
	}

	secret := strings.Repeat("s", 32)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req auth.IdentityAuthenticationRequest
		if r.Method != "POST" || r.URL.Path != "/internal/v2/identity/request:authenticate" || r.Header.Get("Authorization") != "Bearer "+secret || json.NewDecoder(r.Body).Decode(&req) != nil || req.Method != "GET" {
			http.Error(w, "invalid authentication transport", 400)
			return
		}
		owner := wallet
		switch req.Authorization {
		case "Bearer repair-64-owner":
		case "Bearer repair-64-other":
			owner = token
		default:
			http.Error(w, "invalid credentials", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(auth.IdentityPrincipal{AccountID: "64646464-6464-4646-8646-646464646401", SessionID: "64646464-6464-4646-8646-646464646402", Wallet: owner, ProofAt: time.Now().Add(-time.Minute), AccessExpiresAt: time.Now().Add(time.Hour)})
	}))
	defer identity.Close()
	service := Service{Repository: repo, Authority: auth.HTTPIdentityAuthority{Endpoint: identity.URL, ServiceToken: secret, Client: identity.Client()}}
	router := chi.NewRouter()
	service.RegisterHTTP(router)
	origin := httptest.NewServer(router)
	defer origin.Close()
	target := origin.URL + "/v2/me/portfolio/history?from=2026-10-10T00:00:00.000Z&to=2026-10-10T01:00:00.000Z&interval=1h&limit=1"
	get := func(bearer string) (int, []byte) {
		t.Helper()
		request, e := http.NewRequestWithContext(ctx, "GET", target, nil)
		if e != nil {
			t.Fatal(e)
		}
		if bearer != "" {
			request.Header.Set("Authorization", "Bearer "+bearer)
		}
		response, e := origin.Client().Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		body, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("private history cache policy lost")
		}
		return response.StatusCode, body
	}
	status, body := get("repair-64-owner")
	if status != http.StatusOK {
		t.Fatalf("authenticated history status=%d body=%s", status, body)
	}
	if err = interfaces.ValidateJSON("PnlHistoryResponse", body); err != nil {
		t.Fatal("history wire contract", err)
	}
	var response interfaces.PnlHistoryResponse
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Items) != 1 || response.Revision == nil || string(*response.Revision) != saved.Watermark.ProjectionRevision || response.Watermark == nil || string(response.Watermark.BlockHash) != saved.Watermark.BlockHash {
		t.Fatalf("reopened HTTP history lost committed projection: %s", body)
	}
	expected, _ := json.Marshal(saved.PricedSubtotalUsdlRaw)
	if !bytes.Contains(body, append([]byte(`"pricedSubtotalUsdlRaw":`), expected...)) {
		t.Fatalf("history value differs from producer: %s", body)
	}
	status, body = get("repair-64-other")
	if status != http.StatusOK {
		t.Fatalf("other authenticated wallet: %d %s", status, body)
	}
	if err = json.Unmarshal(body, &response); err != nil || len(response.Data.Items) != 0 || response.Revision != nil {
		t.Fatalf("private history crossed owners: %s %v", body, err)
	}
	status, body = get("")
	if status != http.StatusForbidden {
		t.Fatalf("unauthenticated history: %d %s", status, body)
	}
}

// repair64Peer is a strict in-memory database peer for the production pgx client.
// It accepts only the participant's SQL, enforces generated runtime grants, and
// stores the producer's original values transactionally across client restarts.
// It is not evidence of a PostgreSQL coordinator bootstrap or catalog check.
type repair64Row [][]byte
type repair64Peer struct {
	t          *testing.T
	listener   net.Listener
	mu         sync.Mutex
	tables     map[string][]repair64Row
	grants     map[string]map[string]bool
	copied     int
	copyDenied bool
	wg         sync.WaitGroup
}

func repair64NewPeer(t *testing.T, grants []string) *repair64Peer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	peer := &repair64Peer{t: t, listener: listener, tables: map[string][]repair64Row{}, grants: map[string]map[string]bool{}}
	grantPattern := regexp.MustCompile(`^GRANT ([A-Z, ]+) ON TABLE "pnl"\."([a-z0-9_]+)" TO "pnl_runtime"$`)
	for _, statement := range grants {
		if match := grantPattern.FindStringSubmatch(statement); match != nil {
			peer.grants[match[2]] = map[string]bool{}
			for _, privilege := range strings.Split(match[1], ", ") {
				peer.grants[match[2]][privilege] = true
			}
		}
	}
	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			peer.wg.Add(1)
			go func() { defer peer.wg.Done(); defer conn.Close(); peer.serve(conn) }()
		}
	}()
	t.Cleanup(func() { listener.Close(); peer.wg.Wait() })
	return peer
}

func (p *repair64Peer) repository(t *testing.T) Repository {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://pnl_runtime@" + p.listener.Addr().String() + "/pnl?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	config.ConnConfig.ConnectTimeout = 2 * time.Second
	config.ConnConfig.RuntimeParams["search_path"] = "pnl"
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return Repository{Pool: pool}
}

func repair64Fields(query string) []pgproto3.FieldDescription {
	var names []string
	var oids []uint32
	switch {
	case strings.HasPrefix(query, `select "snapshot_id"`):
		names = []string{"snapshot_id", "ordinal", "payload"}
		oids = []uint32{2950, 23, 3802}
	case strings.HasPrefix(query, "SELECT digest,summary"):
		names = []string{"digest", "summary"}
		oids = []uint32{25, 3802}
	case strings.HasPrefix(query, "SELECT h.digest"):
		names = []string{"digest", "id", "wallet"}
		oids = []uint32{25, 25, 25}
	case strings.HasPrefix(query, "SELECT h.row_count"):
		names = []string{"row_count", "expires_at"}
		oids = []uint32{23, 1184}
	case strings.HasPrefix(query, "SELECT summary"), strings.HasPrefix(query, "SELECT chosen.summary"):
		names = []string{"summary"}
		oids = []uint32{3802}
	case strings.HasPrefix(query, "SELECT payload"), strings.HasPrefix(query, "SELECT m.payload"):
		names = []string{"payload"}
		oids = []uint32{3802}
	}
	fields := make([]pgproto3.FieldDescription, len(names))
	for i := range names {
		fields[i] = pgproto3.FieldDescription{Name: []byte(names[i]), DataTypeOID: oids[i], DataTypeSize: -1, TypeModifier: -1}
	}
	return fields
}

const repair64Timestamp = "2006-01-02 15:04:05.999999999Z07:00"

func repair64Params(query string) []uint32 {
	switch {
	case strings.HasPrefix(query, "SELECT pg_advisory_xact_lock("), strings.HasPrefix(query, "SELECT digest,summary"), strings.HasPrefix(query, "SELECT h.digest"):
		return []uint32{25}
	case strings.HasPrefix(query, "INSERT INTO portfolio_snapshots("):
		return []uint32{2950, 25, 25, 25, 1700, 1184, 3802, 1184}
	case strings.HasPrefix(query, "INSERT INTO portfolio_snapshot_rows "):
		return []uint32{2950, 23, 3802}
	case strings.HasPrefix(query, "INSERT INTO portfolio_outbox("), strings.HasPrefix(query, "INSERT INTO portfolio_history_outbox("):
		return []uint32{2950, 3802}
	case strings.HasPrefix(query, "INSERT INTO portfolio_history_snapshots "):
		return []uint32{2950, 25, 25, 23}
	case strings.HasPrefix(query, "SELECT summary FROM"), strings.HasPrefix(query, "SELECT h.row_count"), strings.HasPrefix(query, "SELECT m.payload"):
		return []uint32{2950, 25}
	case strings.HasPrefix(query, "SELECT payload FROM portfolio_history_rows"):
		return []uint32{2950, 23, 23}
	case strings.HasPrefix(query, "SELECT chosen.summary"):
		return []uint32{25, 1184, 1184, 1186, 23}
	default:
		return nil
	}
}

// Preserve JSON exactly; use pgx's real scalar codecs for binary parameters.
func repair64Parameter(oid uint32, format int16, raw []byte) ([]byte, error) {
	if format == 0 {
		return bytes.Clone(raw), nil
	}
	types := pgtype.NewMap()
	dataType, ok := types.TypeForOID(oid)
	if !ok {
		return nil, fmt.Errorf("unrecorded parameter OID %d", oid)
	}
	value, err := dataType.Codec.DecodeValue(types, oid, format, raw)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case int32:
		return []byte(strconv.FormatInt(int64(v), 10)), nil
	case time.Time:
		return []byte(v.Format(repair64Timestamp)), nil
	default:
		return nil, fmt.Errorf("unrecorded binary parameter type %T", value)
	}
}

func repair64Result(oid uint32, format int16, raw []byte) ([]byte, error) {
	if format == 0 {
		return raw, nil
	}
	var value any
	switch oid {
	case 23:
		n, err := strconv.ParseInt(string(raw), 10, 32)
		if err != nil {
			return nil, err
		}
		value = int32(n)
	case 1184:
		instant, err := time.Parse(repair64Timestamp, string(raw))
		if err != nil {
			return nil, err
		}
		value = instant
	default:
		return nil, fmt.Errorf("unrecorded binary result OID %d", oid)
	}
	return pgtype.NewMap().Encode(oid, format, value, nil)
}

func (p *repair64Peer) serve(conn net.Conn) {
	backend := pgproto3.NewBackend(conn, conn)
	startup, err := backend.ReceiveStartupMessage()
	if err != nil {
		p.t.Error(err)
		return
	}
	request, ok := startup.(*pgproto3.StartupMessage)
	if !ok || request.Parameters["user"] != "pnl_runtime" || request.Parameters["search_path"] != "pnl" {
		p.t.Error("connection escaped runtime identity", startup)
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	backend.Send(&pgproto3.ParameterStatus{Name: "DateStyle", Value: "ISO, MDY"})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if backend.Flush() != nil {
		return
	}
	statements := map[string]string{}
	query := ""
	var args repair64Row
	var formats []int16
	var transaction map[string][]repair64Row
	status := byte('I')
	failed := false
	var copied bytes.Buffer
	copying := false
	sendError := func(e error) {
		code := "XX000"
		var postgres *pgconn.PgError
		if errors.As(e, &postgres) {
			code = postgres.Code
		}
		backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: code, Message: e.Error()})
		if status == 'T' {
			status = 'E'
		}
		failed = true
	}
	execute := func(sql string, parameters repair64Row) {
		switch {
		case sql == "begin":
			p.mu.Lock()
			transaction = map[string][]repair64Row{}
			for table, rows := range p.tables {
				transaction[table] = append([]repair64Row(nil), rows...)
			}
			p.mu.Unlock()
			status = 'T'
			backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("BEGIN")})
		case sql == "commit":
			if status != 'T' {
				sendError(errors.New("cannot commit aborted transaction"))
				return
			}
			p.mu.Lock()
			p.tables = transaction
			p.mu.Unlock()
			transaction = nil
			status = 'I'
			backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("COMMIT")})
		case sql == "rollback":
			transaction = nil
			status = 'I'
			backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("ROLLBACK")})
		default:
			p.mu.Lock()
			tables := transaction
			if tables == nil {
				tables = p.tables
			}
			rows, e := p.execute(tables, sql, parameters)
			p.mu.Unlock()
			if e != nil {
				sendError(e)
				return
			}
			for _, row := range rows {
				encoded := make(repair64Row, len(row))
				fields := repair64Fields(sql)
				for i := range row {
					format := int16(0)
					if len(formats) > 0 {
						format = formats[i%len(formats)]
					}
					encoded[i], e = repair64Result(fields[i].DataTypeOID, format, row[i])
					if e != nil {
						sendError(e)
						return
					}
				}
				backend.Send(&pgproto3.DataRow{Values: encoded})
			}
			backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT " + strconv.Itoa(len(rows)))})
		}
	}
	for {
		message, e := backend.Receive()
		if e != nil {
			if !errors.Is(e, io.EOF) && !errors.Is(e, net.ErrClosed) {
				p.t.Error(e)
			}
			return
		}
		switch m := message.(type) {
		case *pgproto3.Parse:
			statements[m.Name] = m.Query
			backend.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Describe:
			if m.ObjectType == 'S' {
				query = statements[m.Name]
				backend.Send(&pgproto3.ParameterDescription{ParameterOIDs: repair64Params(query)})
			}
			fields := repair64Fields(query)
			if m.ObjectType == 'P' {
				for i := range fields {
					if len(formats) > 0 {
						fields[i].Format = formats[i%len(formats)]
					}
				}
			}
			if len(fields) == 0 {
				backend.Send(&pgproto3.NoData{})
			} else {
				backend.Send(&pgproto3.RowDescription{Fields: fields})
			}
		case *pgproto3.Bind:
			query = statements[m.PreparedStatement]
			formats = append([]int16(nil), m.ResultFormatCodes...)
			oids := repair64Params(query)
			if len(oids) != len(m.Parameters) {
				p.t.Errorf("parameter contract mismatch: %s", query)
				return
			}
			args = make(repair64Row, len(m.Parameters))
			for i := range args {
				format := int16(0)
				if len(m.ParameterFormatCodes) > 0 {
					format = m.ParameterFormatCodes[i%len(m.ParameterFormatCodes)]
				}
				args[i], err = repair64Parameter(oids[i], format, m.Parameters[i])
				if err != nil {
					p.t.Error(err)
					return
				}
			}
			backend.Send(&pgproto3.BindComplete{})
		case *pgproto3.Execute:
			if !failed {
				execute(query, args)
			}
		case *pgproto3.Sync:
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			failed = false
			if backend.Flush() != nil {
				return
			}
		case *pgproto3.Query:
			if strings.HasPrefix(m.String, `copy "portfolio_history_rows"`) {
				if m.String != `copy "portfolio_history_rows" ( "snapshot_id", "ordinal", "payload" ) from stdin binary;` {
					sendError(fmt.Errorf("unexpected COPY contract: %s", m.String))
				} else if !p.grants["portfolio_history_rows"]["INSERT"] {
					p.mu.Lock()
					p.copyDenied = true
					p.mu.Unlock()
					sendError(&pgconn.PgError{Code: "42501", Message: "permission denied for table portfolio_history_rows"})
				} else if status != 'T' {
					sendError(errors.New("COPY outside participant transaction"))
				} else {
					copying = true
					copied.Reset()
					backend.Send(&pgproto3.CopyInResponse{OverallFormat: 1, ColumnFormatCodes: []uint16{1, 1, 1}})
				}
			} else {
				execute(m.String, nil)
			}
			if !copying {
				backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
				failed = false
			}
			if backend.Flush() != nil {
				return
			}
		case *pgproto3.CopyData:
			if !copying && status == 'E' {
				continue
			} // PostgreSQL discards COPY messages after denial.
			if !copying {
				p.t.Error("COPY data outside COPY")
				return
			}
			copied.Write(m.Data)
		case *pgproto3.CopyDone:
			if !copying && status == 'E' {
				continue
			}
			// CopyDone supplies the end marker for pgx's protocol stream;
			// unlike a binary COPY file, its CopyData has no -1 trailer.
			rows, e := repair64DecodeCopy(append(copied.Bytes(), 0xff, 0xff))
			if e != nil {
				sendError(e)
			} else {
				transaction["portfolio_history_rows"] = append(transaction["portfolio_history_rows"], rows...)
				p.mu.Lock()
				p.copied += len(rows)
				p.mu.Unlock()
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(fmt.Sprintf("COPY %d", len(rows)))})
			}
			copying = false
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			failed = false
			if backend.Flush() != nil {
				return
			}
		case *pgproto3.Terminate:
			return
		default:
			p.t.Errorf("unrecorded frontend message %T", message)
			return
		}
	}
}

func (p *repair64Peer) execute(tables map[string][]repair64Row, query string, args repair64Row) ([]repair64Row, error) {
	permission := func(table, privilege string) error {
		if !p.grants[table][privilege] {
			return &pgconn.PgError{Code: "42501", Message: "permission denied for table " + table}
		}
		return nil
	}
	value := func(row repair64Row, i int) string { return string(row[i]) }
	switch {
	case strings.HasPrefix(query, "SELECT pg_advisory_xact_lock("):
		return nil, nil
	case strings.HasPrefix(query, `select "snapshot_id"`):
		return nil, permission("portfolio_history_rows", "SELECT")
	case strings.HasPrefix(query, "INSERT INTO "):
		table := strings.Fields(query)[2]
		table = strings.Split(table, "(")[0]
		counts := map[string]int{"portfolio_snapshots": 8, "portfolio_snapshot_rows": 3, "portfolio_outbox": 2, "portfolio_history_snapshots": 4, "portfolio_history_outbox": 2}
		if counts[table] == 0 || len(args) != counts[table] {
			return nil, fmt.Errorf("unrecorded insert %q with %d parameters", query, len(args))
		}
		if e := permission(table, "INSERT"); e != nil {
			return nil, e
		}
		tables[table] = append(tables[table], args)
		return nil, nil
	case strings.HasPrefix(query, "SELECT digest,summary FROM portfolio_snapshots"):
		if e := permission("portfolio_snapshots", "SELECT"); e != nil {
			return nil, e
		}
		for _, row := range tables["portfolio_snapshots"] {
			if value(row, 2) == value(args, 0) {
				return []repair64Row{{row[3], row[6]}}, nil
			}
		}
		return nil, nil
	case strings.HasPrefix(query, "SELECT summary FROM portfolio_snapshots"):
		if e := permission("portfolio_snapshots", "SELECT"); e != nil {
			return nil, e
		}
		for _, row := range tables["portfolio_snapshots"] {
			if value(row, 0) == value(args, 0) && value(row, 1) == value(args, 1) {
				return []repair64Row{{row[6]}}, nil
			}
		}
		return nil, nil
	case strings.HasPrefix(query, "SELECT h.digest"), strings.HasPrefix(query, "SELECT h.row_count"):
		if e := permission("portfolio_history_snapshots", "SELECT"); e != nil {
			return nil, e
		}
		if e := permission("portfolio_snapshots", "SELECT"); e != nil {
			return nil, e
		}
		for _, history := range tables["portfolio_history_snapshots"] {
			for _, snapshot := range tables["portfolio_snapshots"] {
				if value(history, 0) != value(snapshot, 0) {
					continue
				}
				if strings.HasPrefix(query, "SELECT h.digest") && value(history, 1) == value(args, 0) {
					return []repair64Row{{history[2], history[0], snapshot[1]}}, nil
				}
				if strings.HasPrefix(query, "SELECT h.row_count") && value(history, 0) == value(args, 0) && value(snapshot, 1) == value(args, 1) {
					return []repair64Row{{history[3], snapshot[5]}}, nil
				}
			}
		}
		return nil, nil
	case strings.HasPrefix(query, "SELECT payload FROM portfolio_history_rows"):
		if e := permission("portfolio_history_rows", "SELECT"); e != nil {
			return nil, e
		}
		after, e := strconv.Atoi(value(args, 1))
		if e != nil {
			return nil, e
		}
		limit, e := strconv.Atoi(value(args, 2))
		if e != nil {
			return nil, e
		}
		var rows []repair64Row
		for _, row := range tables["portfolio_history_rows"] {
			ordinal, e := strconv.Atoi(value(row, 1))
			if e != nil {
				return nil, e
			}
			if value(row, 0) == value(args, 0) && ordinal > after && len(rows) < limit {
				rows = append(rows, repair64Row{row[2]})
			}
		}
		return rows, nil
	case strings.HasPrefix(query, "SELECT chosen.summary FROM generate_series("):
		if e := permission("portfolio_snapshots", "SELECT"); e != nil {
			return nil, e
		}
		// This committed fixture contains one snapshot in one hour bucket.
		from, e := time.Parse(repair64Timestamp, value(args, 2))
		if e != nil {
			return nil, e
		}
		to, e := time.Parse(repair64Timestamp, value(args, 1))
		if e != nil {
			return nil, e
		}
		var rows []repair64Row
		for _, row := range tables["portfolio_snapshots"] {
			observed, e := time.Parse(repair64Timestamp, value(row, 7))
			if e != nil {
				return nil, e
			}
			if value(row, 1) == value(args, 0) && !observed.Before(from) && observed.Before(to) {
				rows = append(rows, repair64Row{row[6]})
			}
		}
		return rows, nil
	case strings.HasPrefix(query, "SELECT m.payload FROM portfolio_snapshot_history_metadata"):
		if e := permission("portfolio_snapshot_history_metadata", "SELECT"); e != nil {
			return nil, e
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unrecorded SQL: %s", query)
	}
}

func repair64DecodeCopy(raw []byte) ([]repair64Row, error) {
	reader := bytes.NewReader(raw)
	header := make([]byte, 19)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if !bytes.Equal(header[:11], []byte("PGCOPY\n\xff\r\n\x00")) || binary.BigEndian.Uint32(header[11:15]) != 0 || binary.BigEndian.Uint32(header[15:19]) != 0 {
		return nil, errors.New("invalid binary COPY header")
	}
	var rows []repair64Row
	for {
		var count int16
		if err := binary.Read(reader, binary.BigEndian, &count); err != nil {
			return nil, err
		}
		if count == -1 {
			if reader.Len() != 0 {
				return nil, errors.New("trailing COPY bytes")
			}
			return rows, nil
		}
		if count != 3 {
			return nil, fmt.Errorf("COPY must have snapshot, ordinal, payload; got %d", count)
		}
		fields := make([][]byte, 3)
		for i := range fields {
			var size int32
			if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
				return nil, err
			}
			if size < 0 || int64(size) > int64(reader.Len()) {
				return nil, errors.New("invalid COPY field length")
			}
			fields[i] = make([]byte, size)
			if _, err := io.ReadFull(reader, fields[i]); err != nil {
				return nil, err
			}
		}
		if len(fields[0]) != 16 || len(fields[1]) != 4 || len(fields[2]) < 2 || fields[2][0] != 1 || !json.Valid(fields[2][1:]) {
			return nil, errors.New("COPY field codec mismatch")
		}
		id := hex.EncodeToString(fields[0])
		id = id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
		ordinal := int32(binary.BigEndian.Uint32(fields[1]))
		if int(ordinal) != len(rows) {
			return nil, errors.New("COPY history ordinal is not contiguous")
		}
		rows = append(rows, repair64Row{[]byte(id), []byte(strconv.Itoa(int(ordinal))), fields[2][1:]})
	}
}
