package delivery

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The bounded gate runs the production pgx and S3 transports against loopback
// peers. This transactional in-memory SQL fixture implements only the committed
// delivery query contract; unsupported SQL fails rather than returning success.
// It is not evidence that a PostgreSQL deployment was qualified.
type m962Build struct {
	Consumer, Generation, Manifest, Status, Owner, Projection string
	Next, Count, Revision, Epoch                              int64
	Lease                                                     time.Time
}
type m962Alias struct {
	Generation, Prior string
	Revision          int64
}
type m962Observation struct {
	Range, Path, Subject string
	Envelope             Envelope
}
type m962Proof struct {
	Manifest, Digest, Subjects, Consumer, Generation string
	Count                                            int64
}
type m962Outbox struct {
	Subject  string
	Wire     []byte
	Envelope Envelope
}
type m962Data struct {
	Source       []ArchiveRaw
	Frontier     int64
	Manifests    map[string][]byte
	Builds       map[string]*m962Build
	Aliases      map[string]m962Alias
	Paths        map[string]string
	Observations map[string]m962Observation
	Proofs       map[string]m962Proof
	Outbox       map[string]m962Outbox
	Projection   map[string][][]byte
	History      []m962Alias
}
type m962Database struct {
	mu        sync.Mutex
	data      m962Data
	failApply bool
}

func m962NewData() m962Data {
	return m962Data{Frontier: 123, Manifests: map[string][]byte{}, Builds: map[string]*m962Build{}, Aliases: map[string]m962Alias{}, Paths: map[string]string{}, Observations: map[string]m962Observation{}, Proofs: map[string]m962Proof{}, Outbox: map[string]m962Outbox{}, Projection: map[string][][]byte{}}
}
func (d *m962Database) edit(f func(*m962Data)) { d.mu.Lock(); defer d.mu.Unlock(); f(&d.data) }
func (d *m962Database) snapshot() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	b, _ := json.Marshal(d.data)
	return string(b)
}
func m962Pair(a, b string) string { return a + "/" + b }
func m962Int(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}
func m962Bytes(s string) []byte {
	if !strings.HasPrefix(s, `\x`) {
		return []byte(s)
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil {
		panic(err)
	}
	return b
}
func m962InArray(array, value string) bool {
	// All fixture subjects are plain tokens without PostgreSQL array escapes.
	for _, s := range strings.Split(strings.Trim(array, "{}"), ",") {
		if strings.Trim(s, `"`) == value {
			return true
		}
	}
	return false
}

type m962Result struct {
	rows [][]any
	tag  string
	err  error
}

func m962Rows(rows ...[]any) m962Result {
	return m962Result{rows: rows, tag: fmt.Sprintf("SELECT %d", len(rows))}
}
func m962Tag(kind string, n int) m962Result { return m962Result{tag: fmt.Sprintf("%s %d", kind, n)} }
func (db *m962Database) eval(d *m962Data, sql string, a []string) m962Result {
	q := strings.Join(strings.Fields(strings.ReplaceAll(sql, `"m962".`, "")), " ")
	has := func(s string) bool { return strings.Contains(q, s) }
	key := ""
	if len(a) >= 2 {
		key = m962Pair(a[0], a[1])
	}
	b := d.Builds[key]
	live := b != nil && len(a) > 2 && b.Owner == a[2] && b.Lease.After(time.Now())
	switch {
	case strings.HasPrefix(q, "SELECT block_hash,raw_count FROM indexer."):
		h := m962Int(a[1])
		var block string
		var n int64
		for _, r := range d.Source {
			if r.Height == h {
				block = r.BlockHash
				n++
			}
		}
		if block == "" {
			return m962Rows()
		}
		return m962Rows([]any{block, n})
	case strings.HasPrefix(q, "SELECT transaction_hash,block_hash,emitter,height,"):
		var rows [][]any
		for _, r := range d.Source {
			if r.Height == m962Int(a[0]) {
				topics, _ := json.Marshal(r.Topics)
				rows = append(rows, []any{r.TransactionHash, r.BlockHash, r.Emitter, r.Height, int64(r.TransactionIndex), int64(r.LogIndex), topics, r.Data})
			}
		}
		return m962Rows(rows...)
	case strings.HasPrefix(q, "SELECT typed_complete_height FROM indexer."):
		return m962Rows([]any{d.Frontier})
	case strings.HasPrefix(q, "SELECT count(*) FROM generate_series"):
		var missing int64
		for h := m962Int(a[1]) - m962Int(a[2]); h <= m962Int(a[1]); h++ {
			found := false
			for _, r := range d.Source {
				if r.Height == h {
					found = true
				}
			}
			if !found {
				missing++
			}
		}
		return m962Rows([]any{missing})
	case strings.HasPrefix(q, "INSERT INTO delivery_archive_manifests"):
		if _, ok := d.Manifests[a[0]]; ok {
			return m962Tag("INSERT 0", 0)
		}
		d.Manifests[a[0]] = m962Bytes(a[6])
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT manifest FROM delivery_archive_manifests"):
		raw, ok := d.Manifests[a[0]]
		if !ok {
			return m962Rows()
		}
		return m962Rows([]any{raw})
	case strings.HasPrefix(q, "SELECT generation FROM delivery_read_aliases"):
		alias, ok := d.Aliases[a[0]]
		if !ok {
			return m962Rows()
		}
		return m962Rows([]any{alias.Generation})
	case strings.HasPrefix(q, "SELECT b.manifest_digest,b.status,b.next_height,b.raw_count"):
		if b == nil || d.Aliases[a[0]].Generation != a[1] {
			return m962Rows()
		}
		return m962Rows([]any{b.Manifest, b.Status, b.Next, b.Count})
	case strings.HasPrefix(q, "INSERT INTO delivery_rebuilds"):
		if b != nil {
			return m962Tag("INSERT 0", 0)
		}
		d.Builds[key] = &m962Build{Consumer: a[0], Generation: a[1], Manifest: a[2], Next: m962Int(a[3]), Status: "building", Owner: a[4], Epoch: 1, Lease: time.Now().Add(time.Duration(m962Int(a[5])) * time.Millisecond)}
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT manifest_digest,status FROM delivery_rebuilds"):
		if b == nil {
			return m962Rows()
		}
		return m962Rows([]any{b.Manifest, b.Status})
	case strings.HasPrefix(q, "UPDATE delivery_rebuilds SET owner=$3,epoch=epoch+"):
		if b == nil || b.Owner != a[2] && b.Lease.After(time.Now()) || b.Status != "building" && b.Status != "verified" {
			return m962Tag("UPDATE", 0)
		}
		if b.Owner != a[2] {
			b.Epoch++
		}
		b.Owner = a[2]
		b.Lease = time.Now().Add(time.Duration(m962Int(a[3])) * time.Millisecond)
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "INSERT INTO delivery_generation_paths"):
		if _, ok := d.Paths[key]; ok {
			return m962Tag("INSERT 0", 0)
		}
		d.Paths[key] = a[2]
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT authoritative_path FROM delivery_generation_paths"):
		v, ok := d.Paths[key]
		if !ok {
			return m962Rows()
		}
		return m962Rows([]any{v})
	case strings.HasPrefix(q, "SELECT manifest_digest,next_height FROM delivery_rebuilds"):
		if b == nil || b.Status != "building" {
			return m962Rows()
		}
		return m962Rows([]any{b.Manifest, b.Next})
	case strings.HasPrefix(q, "SELECT epoch,next_height,owner FROM delivery_rebuilds"):
		if b == nil || b.Status != "building" || b.Owner != a[2] && b.Lease.After(time.Now()) {
			return m962Rows()
		}
		return m962Rows([]any{b.Epoch, b.Next, b.Owner})
	case strings.HasPrefix(q, "UPDATE delivery_rebuilds SET owner=$3,epoch=$4"):
		if b == nil {
			return m962Tag("UPDATE", 0)
		}
		b.Owner = a[2]
		b.Epoch = m962Int(a[3])
		b.Lease = time.Now().Add(time.Duration(m962Int(a[4])) * time.Millisecond)
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "INSERT INTO repair_41_projection"):
		if db.failApply {
			db.failApply = false
			return m962Result{err: errors.New("interrupted projection transaction")}
		}
		d.Projection[a[0]] = append(d.Projection[a[0]], [][]byte{m962Bytes(a[4])}...)
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT body FROM repair_41_projection"):
		var rows [][]any
		for _, body := range d.Projection[a[0]] {
			rows = append(rows, []any{body})
		}
		return m962Rows(rows...)
	case strings.HasPrefix(q, "UPDATE delivery_rebuilds SET next_height=$5"):
		if !live || b.Epoch != m962Int(a[3]) {
			return m962Tag("UPDATE", 0)
		}
		b.Next = m962Int(a[4])
		b.Count += m962Int(a[5])
		b.Revision++
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "SELECT next_height,raw_count,manifest_digest,epoch FROM delivery_rebuilds"):
		if !live || b.Status != "building" {
			return m962Rows()
		}
		return m962Rows([]any{b.Next, b.Count, b.Manifest, b.Epoch})
	case strings.HasPrefix(q, "UPDATE delivery_rebuilds SET status='verified'"):
		if !live || b.Epoch != m962Int(a[3]) {
			return m962Tag("UPDATE", 0)
		}
		b.Status = "verified"
		b.Projection = a[4]
		b.Revision++
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "SELECT manifest_digest,status,epoch FROM delivery_rebuilds"):
		if !live {
			return m962Rows()
		}
		return m962Rows([]any{b.Manifest, b.Status, b.Epoch})
	case strings.HasPrefix(q, "SELECT next_height,raw_count,projection_digest FROM delivery_rebuilds"):
		if b == nil {
			return m962Rows()
		}
		return m962Rows([]any{b.Next, b.Count, b.Projection})
	case strings.HasPrefix(q, "SELECT status,next_height,raw_count,revision FROM delivery_rebuilds"):
		if b == nil {
			return m962Rows()
		}
		return m962Rows([]any{b.Status, b.Next, b.Count, b.Revision})
	case strings.HasPrefix(q, "SELECT epoch FROM delivery_rebuilds"):
		if !live || b.Status != "active" {
			return m962Rows()
		}
		return m962Rows([]any{b.Epoch})
	case strings.HasPrefix(q, "SELECT status,manifest_digest,next_height,raw_count,projection_digest FROM delivery_rebuilds"):
		if b == nil {
			return m962Rows()
		}
		return m962Rows([]any{b.Status, b.Manifest, b.Next, b.Count, b.Projection})
	case strings.HasPrefix(q, "UPDATE delivery_rebuilds SET epoch=epoch"):
		if !live || b.Status != "active" || b.Epoch != m962Int(a[3]) {
			return m962Tag("UPDATE", 0)
		}
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "UPDATE delivery_rebuilds SET status='active'"):
		if b == nil || has("owner=$3") && (!live || b.Epoch != m962Int(a[3])) {
			return m962Tag("UPDATE", 0)
		}
		b.Status = "active"
		b.Revision++
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "SELECT generation,revision FROM delivery_read_aliases"):
		v, ok := d.Aliases[a[0]]
		if !ok {
			return m962Rows()
		}
		return m962Rows([]any{v.Generation, v.Revision})
	case strings.HasPrefix(q, "SELECT generation,COALESCE(prior_generation,''),revision FROM delivery_read_aliases"):
		v, ok := d.Aliases[a[0]]
		if !ok {
			return m962Rows()
		}
		return m962Rows([]any{v.Generation, v.Prior, v.Revision})
	case strings.HasPrefix(q, "INSERT INTO delivery_read_aliases"):
		d.Aliases[a[0]] = m962Alias{a[1], a[2], m962Int(a[3])}
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "UPDATE delivery_read_aliases SET generation=$2"):
		v, ok := d.Aliases[a[0]]
		if !ok {
			return m962Tag("UPDATE", 0)
		}
		d.Aliases[a[0]] = m962Alias{a[1], a[2], v.Revision + 1}
		return m962Tag("UPDATE", 1)
	case strings.HasPrefix(q, "INSERT INTO delivery_cutover_history"):
		for _, v := range d.History {
			if v.Revision == m962Int(a[1]) {
				return m962Result{err: errors.New("duplicate cutover revision")}
			}
		}
		d.History = append(d.History, m962Alias{a[2], a[3], m962Int(a[1])})
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "INSERT INTO delivery_outbox"):
		if _, ok := d.Outbox[a[0]]; ok {
			return m962Tag("INSERT 0", 0)
		}
		wire := m962Bytes(a[4])
		var e Envelope
		if err := json.Unmarshal(wire, &e); err != nil {
			return m962Result{err: err}
		}
		d.Outbox[a[0]] = m962Outbox{a[2], wire, e}
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT subject,envelope FROM delivery_outbox"):
		v, ok := d.Outbox[a[0]]
		if !ok {
			return m962Rows()
		}
		return m962Rows([]any{v.Subject, v.Wire})
	case strings.HasPrefix(q, "INSERT INTO delivery_shadow_observations"):
		k := key + "/" + a[2]
		if _, ok := d.Observations[k]; ok {
			return m962Tag("INSERT 0", 0)
		}
		var order SourceOrder
		if err := json.Unmarshal([]byte(a[4]), &order); err != nil {
			return m962Result{err: err}
		}
		d.Observations[k] = m962Observation{a[0], a[1], a[5], Envelope{EventID: a[2], PayloadHash: a[3], SourceOrder: order}}
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT payload_hash,source_order,subject FROM delivery_shadow_observations"):
		v, ok := d.Observations[key+"/"+a[2]]
		if !ok {
			return m962Rows()
		}
		raw, _ := json.Marshal(v.Envelope.SourceOrder)
		return m962Rows([]any{v.Envelope.PayloadHash, raw, v.Subject})
	case strings.HasPrefix(q, "SELECT path,event_id,payload_hash,source_order FROM delivery_shadow_observations"):
		keys := make([]string, 0, len(d.Observations))
		for k := range d.Observations {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var rows [][]any
		for _, k := range keys {
			v := d.Observations[k]
			if v.Range == a[0] && (!has("subject=ANY($2)") || m962InArray(a[1], v.Subject)) {
				raw, _ := json.Marshal(v.Envelope.SourceOrder)
				rows = append(rows, []any{v.Path, v.Envelope.EventID, v.Envelope.PayloadHash, raw})
			}
		}
		return m962Rows(rows...)
	case strings.HasPrefix(q, "SELECT event_id,payload_hash FROM delivery_outbox"):
		ids := make([]string, 0, len(d.Outbox))
		for k := range d.Outbox {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		var rows [][]any
		for _, id := range ids {
			v := d.Outbox[id]
			h := m962Int(v.Envelope.SourceOrder.Height)
			if h >= m962Int(a[0]) && h <= m962Int(a[1]) && m962InArray(a[2], v.Subject) {
				rows = append(rows, []any{id, v.Envelope.PayloadHash})
			}
		}
		return m962Rows(rows...)
	case strings.HasPrefix(q, "INSERT INTO delivery_shadow_comparisons"):
		p := m962Proof{Manifest: a[0], Digest: a[1], Count: m962Int(a[2]), Subjects: a[3], Consumer: a[4], Generation: a[5]}
		d.Proofs[a[0]+"/"+a[4]+"/"+a[5]+"/"+a[3]] = p
		return m962Tag("INSERT 0", 1)
	case strings.HasPrefix(q, "SELECT event_digest,event_count FROM delivery_shadow_comparisons"):
		v, ok := d.Proofs[a[0]+"/"+a[1]+"/"+a[2]+"/"+a[3]]
		if !ok || v.Count <= 0 {
			return m962Rows()
		}
		return m962Rows([]any{v.Digest, v.Count})
	}
	return m962Result{err: fmt.Errorf("unsupported committed SQL contract: %s", q)}
}

func m962Fields(rows [][]any) []pgproto3.FieldDescription {
	if len(rows) == 0 {
		return nil
	}
	fields := make([]pgproto3.FieldDescription, len(rows[0]))
	for i, v := range rows[0] {
		oid := uint32(25)
		switch v.(type) {
		case int64, int:
			oid = 20
		case []byte:
			oid = 17
		case bool:
			oid = 16
		}
		fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("c%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1}
	}
	return fields
}
func m962Value(v any) []byte {
	switch x := v.(type) {
	case []byte:
		return []byte(`\x` + hex.EncodeToString(x))
	case bool:
		if x {
			return []byte("t")
		}
		return []byte("f")
	default:
		return []byte(fmt.Sprint(v))
	}
}
func (db *m962Database) serve(conn net.Conn) {
	defer conn.Close()
	wire := pgproto3.NewBackend(conn, conn)
	if _, err := wire.ReceiveStartupMessage(); err != nil {
		return
	}
	wire.Send(&pgproto3.AuthenticationOk{})
	wire.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	wire.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	wire.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	wire.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if wire.Flush() != nil {
		return
	}
	var tx *m962Data
	defer func() {
		if tx != nil {
			db.mu.Unlock()
		}
	}()
	execute := func(q string, a []string) m962Result {
		lower := strings.ToLower(q)
		if strings.HasPrefix(lower, "begin") {
			if tx != nil {
				return m962Result{err: errors.New("nested transaction")}
			}
			db.mu.Lock()
			raw, _ := json.Marshal(db.data)
			var copy m962Data
			if err := json.Unmarshal(raw, &copy); err != nil {
				panic(err)
			}
			tx = &copy
			return m962Result{tag: "BEGIN"}
		}
		if lower == "commit" {
			if tx != nil {
				db.data = *tx
				tx = nil
				db.mu.Unlock()
			}
			return m962Result{tag: "COMMIT"}
		}
		if lower == "rollback" {
			if tx != nil {
				tx = nil
				db.mu.Unlock()
			}
			return m962Result{tag: "ROLLBACK"}
		}
		if tx != nil {
			return db.eval(tx, q, a)
		}
		db.mu.Lock()
		defer db.mu.Unlock()
		return db.eval(&db.data, q, a)
	}
	sendRows := func(result m962Result, fields bool) {
		if result.err != nil {
			wire.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: result.err.Error()})
			return
		}
		if fields {
			if f := m962Fields(result.rows); len(f) > 0 {
				wire.Send(&pgproto3.RowDescription{Fields: f})
			} else {
				wire.Send(&pgproto3.NoData{})
			}
		}
		for _, row := range result.rows {
			values := make([][]byte, len(row))
			for i, v := range row {
				values[i] = m962Value(v)
			}
			wire.Send(&pgproto3.DataRow{Values: values})
		}
		wire.Send(&pgproto3.CommandComplete{CommandTag: []byte(result.tag)})
	}
	var query string
	var args []string
	var result *m962Result
	for {
		msg, err := wire.Receive()
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case *pgproto3.Parse:
			query = m.Query
			result = nil
			wire.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			args = make([]string, len(m.Parameters))
			for i, p := range m.Parameters {
				args[i] = string(p)
			}
			wire.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			r := execute(query, args)
			result = &r
			if r.err != nil {
				wire.Send(&pgproto3.NoData{})
			} else if f := m962Fields(r.rows); len(f) > 0 {
				wire.Send(&pgproto3.RowDescription{Fields: f})
			} else {
				wire.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			if result == nil {
				r := execute(query, args)
				result = &r
			}
			sendRows(*result, false)
		case *pgproto3.Sync:
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			wire.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if wire.Flush() != nil {
				return
			}
		case *pgproto3.Query:
			r := execute(m.String, nil)
			sendRows(r, len(r.rows) > 0)
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			wire.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if wire.Flush() != nil {
				return
			}
		case *pgproto3.Terminate:
			return
		default:
			wire.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "0A000", Message: fmt.Sprintf("unsupported protocol %T", m)})
			if wire.Flush() != nil {
				return
			}
		}
	}
}

// Source rows are a committed deterministic chain fixture. Manifests, segments,
// observations and comparison evidence are emitted by the real producers.
func m962Source() []ArchiveRaw {
	return []ArchiveRaw{{TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64), Emitter: "0x" + strings.Repeat("3", 40), Height: 123, TransactionIndex: 4, LogIndex: 5, Topics: []string{"0x" + strings.Repeat("4", 64)}, Data: []byte{0, 1, 2, 3}}}
}

type m962Fixture struct {
	ctx        context.Context
	db         *m962Database
	address    string
	objects    *storage.Client
	objectMu   sync.Mutex
	objectData map[string][]byte
	pool       *pgxpool.Pool
	archive    *Archive
	engine     *ReplayEngine
	manifest   ArchiveManifest
	digest     string
	authority  bool
}

func m962FixtureNew(t *testing.T) *m962Fixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	f := &m962Fixture{ctx: ctx, db: &m962Database{data: m962NewData()}, objectData: map[string][]byte{}, authority: true}
	f.db.data.Source = m962Source()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	f.address = listener.Addr().String()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go f.db.serve(conn)
		}
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.objectMu.Lock()
		defer f.objectMu.Unlock()
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			f.objectData[r.URL.Path] = body
			w.Header().Set("ETag", `"`+Hash(body)+`"`)
			w.WriteHeader(200)
		case http.MethodGet, http.MethodHead:
			body, ok := f.objectData[r.URL.Path]
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(404)
				_, _ = io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("Content-Type", "application/octet-stream")
			if r.Method == http.MethodGet {
				_, _ = w.Write(body)
			}
		case http.MethodDelete:
			delete(f.objectData, r.URL.Path)
			w.WriteHeader(204)
		default:
			http.Error(w, "unsupported object operation", 405)
		}
	}))
	t.Cleanup(server.Close)
	client := s3.New(s3.Options{Region: "test", BaseEndpoint: aws.String(server.URL), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider("fixture-key", "fixture-secret", "")})
	f.objects = storage.NewFromS3(client, "m962")
	f.open(t, "g1", "worker")
	f.manifest, f.digest, err = f.archive.Capture(ctx, 123, 123)
	if err != nil {
		t.Fatal("capture through object transport", err)
	}
	return f
}
func (f *m962Fixture) open(t *testing.T, generation, owner string) {
	t.Helper()
	if f.pool != nil {
		f.pool.Close()
	}
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + f.address + "/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		conn.TypeMap().RegisterDefaultPgType(SourceOrder{}, "jsonb")
		return nil
	}
	pool, err := pgxpool.NewWithConfig(f.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f.pool = pool
	policy := fixturePolicy()
	policy.Subjects["kindle.chain125.other"] = true
	store, err := NewStoreWithEpochGuard(pool, "m962", policy, func(context.Context, pgx.Tx) error {
		if !f.authority {
			return ErrLease
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pin := strings.Repeat("a", 64)
	archive, err := NewArchive(store, f.objects, "m962", "canonical", "deployment", pin, pin, pin)
	if err != nil {
		t.Fatal(err)
	}
	f.archive = archive
	f.engine, err = NewReplayEngine(archive, "reader", generation, owner, repair41Projection{archive}, func(ctx context.Context, action string) error {
		allowed, _ := ctx.Value(m962AdminKey{}).(string)
		if allowed != action {
			return ErrAuthorization
		}
		return nil
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
}

type m962AdminKey struct{}

func (f *m962Fixture) admin(action string) context.Context {
	return context.WithValue(f.ctx, m962AdminKey{}, action)
}
func (f *m962Fixture) observe(t *testing.T, subject string, e Envelope, paths ...string) {
	t.Helper()
	if err := f.archive.store.Produce(f.ctx, []Pending{{subject, e}}, func(ctx context.Context, tx pgx.Tx) error {
		for _, path := range paths {
			if err := f.archive.store.RecordShadowTx(ctx, tx, f.digest, path, subject, e); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func (f *m962Fixture) event(id string) Envelope {
	e := fixtureEnvelope(id)
	e.SourceOrder = SourceOrder{"123", "4", "5"}
	return e
}
func (f *m962Fixture) evidence(t *testing.T, subjects []string, consumer, generation string) {
	t.Helper()
	if _, err := f.archive.store.CompareShadowGeneration(f.ctx, f.digest, subjects, consumer, generation); err != nil {
		t.Fatal("persist scoped comparison", err)
	}
}
func (f *m962Fixture) build(t *testing.T) {
	t.Helper()
	if err := f.engine.Start(f.ctx, f.digest); err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= len(f.manifest.Blocks); i++ {
		if err := f.engine.RunBlock(f.ctx); err != nil {
			t.Fatal("run archive block", err)
		}
	}
	status, err := f.engine.Status(f.ctx, 0)
	if err != nil || status.Status != "verified" || !status.Ready {
		t.Fatal(status, err)
	}
}
func (f *m962Fixture) promote(t *testing.T, revision int64) {
	t.Helper()
	if err := f.engine.Promote(f.admin("replay.promote"), revision); err != nil {
		t.Fatal("promote", err)
	}
}
func m962AllSubjects() []string { return []string{fixtureSubject, "kindle.chain125.other"} }
func (f *m962Fixture) bothGenerations(t *testing.T) {
	t.Helper()
	f.observe(t, fixtureSubject, f.event("event"), "legacy", "durable")
	f.build(t)
	f.evidence(t, m962AllSubjects(), "reader", "g1")
	f.promote(t, 0)
	f.open(t, "g2", "worker")
	f.build(t)
	f.evidence(t, m962AllSubjects(), "reader", "g2")
	f.promote(t, 1)
}

func TestRepairM_962_ReplayLifecycle(t *testing.T) {
	f := m962FixtureNew(t)
	f.observe(t, fixtureSubject, f.event("event"), "legacy", "durable")
	if err := f.engine.Start(f.ctx, f.digest); err != nil {
		t.Fatal(err)
	}
	// An interrupted apply must roll back both projected bytes and progress.
	f.db.mu.Lock()
	f.db.failApply = true
	f.db.mu.Unlock()
	before := f.db.snapshot()
	if err := f.engine.RunBlock(f.ctx); err == nil {
		t.Fatal("interrupted apply accepted")
	}
	if f.db.snapshot() != before {
		t.Fatal("crash left partial progress or projection")
	}
	f.open(t, "g1", "other-worker")
	if err := f.engine.Start(f.ctx, f.digest); !errors.Is(err, ErrLease) {
		t.Fatal("live lease stolen", err)
	}
	f.db.edit(func(d *m962Data) { d.Builds["reader/g1"].Lease = time.Now().Add(-time.Second) })
	if err := f.engine.Start(f.ctx, f.digest); err != nil {
		t.Fatal("expired lease recovery", err)
	}
	stale := *f.engine
	stale.owner = "worker"
	if err := stale.RunBlock(f.ctx); !errors.Is(err, ErrLease) {
		t.Fatal("stale worker replay accepted", err)
	}
	for i := 0; i < 2; i++ {
		if err := f.engine.RunBlock(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	f.evidence(t, m962AllSubjects(), "reader", "g1")
	f.promote(t, 0)
	f.db.edit(func(d *m962Data) { d.Builds["reader/g1"].Lease = time.Now().Add(-time.Second) })
	before = f.db.snapshot()
	f.open(t, "g1", "restart")
	budget, err := NewWorkBudget(1, 10000, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.engine.Rebuild(f.ctx, f.digest, budget, true, 0); err != nil {
		t.Fatal("duplicate startup after promotion", err)
	}
	if f.db.snapshot() != before {
		t.Fatal("duplicate startup changed durable state")
	}
	status, err := f.engine.Status(f.ctx, 0)
	if err != nil || !status.Ready || status.Status != "active" {
		t.Fatal(status, err)
	}
	f.db.mu.Lock()
	defer f.db.mu.Unlock()
	if len(f.db.data.Projection["g1"]) != 1 || len(f.db.data.History) != 1 || f.db.data.Aliases["reader"].Revision != 1 {
		t.Fatal("restart duplicated projected records or cutover")
	}
}

func TestRepairM_962_ChangedCanonical(t *testing.T) {
	for _, phase := range []string{"apply", "verify", "promote"} {
		t.Run(phase, func(t *testing.T) {
			f := m962FixtureNew(t)
			f.observe(t, fixtureSubject, f.event("event"), "legacy", "durable")
			if err := f.engine.Start(f.ctx, f.digest); err != nil {
				t.Fatal(err)
			}
			if phase != "apply" {
				if err := f.engine.RunBlock(f.ctx); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "promote" {
				if err := f.engine.RunBlock(f.ctx); err != nil {
					t.Fatal(err)
				}
				f.evidence(t, m962AllSubjects(), "reader", "g1")
			}
			f.db.edit(func(d *m962Data) { d.Source[0].Data = []byte("changed canonical bytes") })
			before := f.db.snapshot()
			var err error
			if phase == "promote" {
				err = f.engine.Promote(f.admin("replay.promote"), 0)
			} else {
				err = f.engine.RunBlock(f.ctx)
			}
			if !errors.Is(err, ErrIntegrity) {
				t.Fatal("changed canonical data accepted", err)
			}
			if before != f.db.snapshot() {
				t.Fatal("failed integrity check committed state")
			}
		})
	}
}

func TestRepairM_962_RollbackFreshness(t *testing.T) {
	for _, scenario := range []string{"fresh", "retired", "verified", "source_advanced", "expired_segment", "expired_manifest", "changed_projection", "incomplete_progress", "stale_worker", "expired_lease", "recovery_epoch", "missing_guard", "inactive_current", "alias_revision", "unauthorized"} {
		t.Run(scenario, func(t *testing.T) {
			f := m962FixtureNew(t)
			f.bothGenerations(t)
			ctx := f.admin("replay.rollback")
			revision := int64(2)
			switch scenario {
			case "retired", "verified":
				f.db.edit(func(d *m962Data) { d.Builds["reader/g1"].Status = scenario })
			case "source_advanced":
				f.db.edit(func(d *m962Data) { d.Frontier++ })
			case "expired_segment":
				if err := f.objects.Delete(f.ctx, f.archive.key("segments", f.manifest.Blocks[0].SegmentHash)); err != nil {
					t.Fatal(err)
				}
			case "expired_manifest":
				if err := f.objects.Delete(f.ctx, f.archive.key("manifests", f.digest)); err != nil {
					t.Fatal(err)
				}
			case "changed_projection":
				f.db.edit(func(d *m962Data) { d.Projection["g1"][0] = []byte(`{"changed":true}`) })
			case "incomplete_progress":
				f.db.edit(func(d *m962Data) { d.Builds["reader/g1"].Next-- })
			case "stale_worker":
				f.engine.owner = "stale-worker"
			case "expired_lease":
				f.db.edit(func(d *m962Data) { d.Builds["reader/g2"].Lease = time.Now().Add(-time.Second) })
			case "recovery_epoch":
				f.authority = false
			case "missing_guard":
				f.archive.store.epochGuard = nil
			case "inactive_current":
				f.db.edit(func(d *m962Data) { d.Builds["reader/g2"].Status = "building" })
			case "alias_revision":
				revision = 1
			case "unauthorized":
				ctx = f.ctx
			}
			before := f.db.snapshot()
			err := f.engine.RollbackAlias(ctx, revision)
			if scenario == "fresh" || scenario == "retired" || scenario == "verified" {
				if err != nil {
					t.Fatal("fresh retained rollback refused", err)
				}
				f.db.mu.Lock()
				alias := f.db.data.Aliases["reader"]
				n := len(f.db.data.History)
				f.db.mu.Unlock()
				if alias.Generation != "g1" || alias.Prior != "g2" || alias.Revision != 3 || n != 3 {
					t.Fatal("rollback alias/history incorrect", alias, n)
				}
				f.open(t, "g1", "restart")
				if err = f.engine.Start(f.ctx, f.digest); err != nil {
					t.Fatal("rollback did not survive reopen", err)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe rollback accepted", scenario)
				}
				expected := error(nil)
				switch scenario {
				case "source_advanced", "incomplete_progress":
					expected = ErrGap
				case "changed_projection", "alias_revision":
					expected = ErrIntegrity
				case "stale_worker", "expired_lease", "recovery_epoch", "inactive_current":
					expected = ErrLease
				}
				if expected != nil && !errors.Is(err, expected) {
					t.Fatal("rollback failed for wrong reason", scenario, err)
				}
				if (scenario == "expired_manifest" || scenario == "expired_segment") && err.Error() != "delivery: archive object unavailable" {
					t.Fatal("retention failure not exercised", err)
				}
				if scenario == "missing_guard" && err.Error() != "delivery: recovery epoch guard required" {
					t.Fatal("missing recovery authority not exercised", err)
				}
				if scenario == "unauthorized" && err.Error() != "delivery: administrator authorization required" {
					t.Fatal("authorization failure not exercised", err)
				}
				if before != f.db.snapshot() {
					t.Fatal("refused rollback changed state")
				}
			}
		})
	}
}

func TestRepairM_962_ShadowProofScope(t *testing.T) {
	for _, scope := range []string{"subset", "consumer", "generation", "unbound", "canonical"} {
		t.Run(scope, func(t *testing.T) {
			f := m962FixtureNew(t)
			f.observe(t, fixtureSubject, f.event("event"), "legacy", "durable")
			f.build(t)
			subjects := m962AllSubjects()
			consumer, generation := "reader", "g1"
			switch scope {
			case "subset":
				subjects = []string{fixtureSubject}
			case "consumer":
				consumer = "different-reader"
			case "generation":
				generation = "g2"
			case "canonical":
				subjects = []string{"kindle.chain125.other", fixtureSubject, fixtureSubject}
			}
			if scope == "unbound" {
				if _, err := f.archive.store.CompareShadowRange(f.ctx, f.digest, subjects); err != nil {
					t.Fatal(err)
				}
			} else {
				f.evidence(t, subjects, consumer, generation)
			}
			before := f.db.snapshot()
			err := f.engine.Promote(f.admin("replay.promote"), 0)
			if scope == "canonical" {
				if err != nil {
					t.Fatal("canonical set refused", err)
				}
			} else {
				if !errors.Is(err, ErrIntegrity) {
					t.Fatal("wrong scope promoted", scope, err)
				}
				if before != f.db.snapshot() {
					t.Fatal("refused promotion changed state")
				}
			}
		})
	}
}

func TestRepairM_962_ShadowDurable(t *testing.T) {
	for _, fault := range []string{"exact", "missing", "extra", "both_extra", "changed", "both_changed", "wrong_range", "outbox_missing", "retry_conflict", "subject_retry_conflict", "late_extra", "changed_after_proof"} {
		t.Run(fault, func(t *testing.T) {
			f := m962FixtureNew(t)
			e := f.event("event")
			paths := []string{"legacy", "durable"}
			if fault == "missing" {
				paths = []string{"legacy"}
			}
			if fault == "wrong_range" {
				e.SourceOrder.Height = "124"
			}
			f.observe(t, fixtureSubject, e, paths...)
			if fault == "exact" {
				f.observe(t, fixtureSubject, e, paths...)
			}
			if fault == "retry_conflict" || fault == "subject_retry_conflict" {
				bad := e
				subject := fixtureSubject
				if fault == "retry_conflict" {
					bad.SourceOrder.LogIndex = "6"
				} else {
					subject = "kindle.chain125.other"
				}
				before := f.db.snapshot()
				err := f.archive.store.mutate(f.ctx, func(tx pgx.Tx) error {
					return f.archive.store.RecordShadowTx(f.ctx, tx, f.digest, "durable", subject, bad)
				})
				if !errors.Is(err, ErrIntegrity) || before != f.db.snapshot() {
					t.Fatal("conflicting durable retry accepted", err)
				}
				return
			}
			if fault == "changed" || fault == "both_changed" {
				bad := e
				bad.Payload = json.RawMessage(`{"changed":true}`)
				bad.PayloadHash = Hash(bad.Payload)
				paths := []string{"durable"}
				if fault == "both_changed" {
					paths = append(paths, "legacy")
				}
				for _, path := range paths {
					f.db.edit(func(d *m962Data) { delete(d.Observations, f.digest+"/"+path+"/"+e.EventID) })
					if err := f.archive.store.mutate(f.ctx, func(tx pgx.Tx) error {
						return f.archive.store.RecordShadowTx(f.ctx, tx, f.digest, path, fixtureSubject, bad)
					}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if fault == "extra" || fault == "both_extra" {
				extra := f.event("extra")
				paths := []string{"durable"}
				if fault == "both_extra" {
					paths = append(paths, "legacy")
				}
				for _, path := range paths {
					if err := f.archive.store.mutate(f.ctx, func(tx pgx.Tx) error {
						return f.archive.store.RecordShadowTx(f.ctx, tx, f.digest, path, fixtureSubject, extra)
					}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if fault == "outbox_missing" {
				f.db.edit(func(d *m962Data) { delete(d.Outbox, e.EventID) })
			}
			if fault == "changed_after_proof" {
				f.build(t)
				f.evidence(t, m962AllSubjects(), "reader", "g1")
				f.observe(t, fixtureSubject, f.event("later"), "legacy", "durable")
				before := f.db.snapshot()
				if err := f.engine.Promote(f.admin("replay.promote"), 0); !errors.Is(err, ErrIntegrity) {
					t.Fatal("changed evidence digest accepted", err)
				}
				if before != f.db.snapshot() {
					t.Fatal("failed digest validation committed")
				}
				return
			}
			if fault == "late_extra" {
				f.build(t)
				f.evidence(t, m962AllSubjects(), "reader", "g1")
				extra := f.event("late-extra")
				if err := f.archive.store.mutate(f.ctx, func(tx pgx.Tx) error {
					return f.archive.store.RecordShadowTx(f.ctx, tx, f.digest, "durable", fixtureSubject, extra)
				}); err != nil {
					t.Fatal(err)
				}
				before := f.db.snapshot()
				if err := f.engine.Promote(f.admin("replay.promote"), 0); !errors.Is(err, ErrIntegrity) {
					t.Fatal("stale persisted evidence accepted", err)
				}
				if before != f.db.snapshot() {
					t.Fatal("failed proof revalidation committed")
				}
				return
			}
			_, err := f.archive.store.CompareShadowGeneration(f.ctx, f.digest, m962AllSubjects(), "reader", "g1")
			f.db.mu.Lock()
			proofs := len(f.db.data.Proofs)
			f.db.mu.Unlock()
			if fault == "exact" {
				if err != nil || proofs != 1 {
					t.Fatal("exact durable observations not persisted", proofs, err)
				}
				before := f.db.snapshot()
				f.open(t, "g1", "reopened")
				f.evidence(t, m962AllSubjects(), "reader", "g1")
				if before != f.db.snapshot() {
					t.Fatal("retry after reopen changed durable proof")
				}
			}
			if fault != "exact" && (!errors.Is(err, ErrIntegrity) || proofs != 0) {
				t.Fatal("bad comparison persisted evidence", fault, proofs, err)
			}
		})
	}
}

// Assert that the fixture projection hashes the exact raw
// bytes the production archive emitted, including its record delimiters.
func TestRepairM_962_ArchiveTransport(t *testing.T) {
	f := m962FixtureNew(t)
	block, records, err := f.archive.ReadBlock(f.ctx, f.digest, 123)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	for _, r := range records {
		body, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		raw.Write(body)
		raw.WriteByte('\n')
	}
	if len(records) != 1 || Hash(raw.Bytes()) != block.SegmentHash || block.RawCount != 1 || int64(raw.Len()) != block.SegmentBytes {
		t.Fatal("producer/archive/consumer bytes differ")
	}
}
