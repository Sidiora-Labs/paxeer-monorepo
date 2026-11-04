package search_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The fixture owns only the persistence peer. Projection, delivery.Store, the
// envelope codec, generation activation, Store.Search and the HTTP handler are
// production components. Index rows are accepted only from their SQL writes.
type repair1044Rows struct {
	names  []string
	oids   []uint32
	values [][]any
	tag    string
}

func repair1044Result(names []string, oids []uint32, values ...[]any) repair1044Rows {
	return repair1044Rows{names: names, oids: oids, values: values, tag: fmt.Sprintf("SELECT %d", len(values))}
}

type repair1044Generation struct {
	State     string
	Watermark int64
	Observed  time.Time
	Validated *time.Time
	Rows      int64
	Digest    string
}
type repair1044Document struct {
	Raw      json.RawMessage
	Revision int64
}
type repair1044Visibility struct {
	Revision          int64
	Public, Tombstone bool
}
type repair1044Outbox struct {
	Subject string
	Wire    []byte
}
type repair1044Data struct {
	Generation        string
	Revision, Privacy int64
	Generations       map[string]repair1044Generation
	Source            map[string]json.RawMessage
	Documents         map[string]map[string]repair1044Document
	Authority         map[string]repair1044Visibility
	Creations         map[string]json.RawMessage
	Events            map[string]json.RawMessage
	Inbox             map[string]string
	Outbox            map[string]repair1044Outbox
}

func repair1044Clone(d repair1044Data) repair1044Data {
	raw, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	var copy repair1044Data
	if err = json.Unmarshal(raw, &copy); err != nil {
		panic(err)
	}
	return copy
}

type repair1044Database struct {
	t        *testing.T
	mu       sync.Mutex
	data     repair1044Data
	listener net.Listener
	done     chan struct{}
}

func newRepair1044Database(t *testing.T) *repair1044Database {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	d := &repair1044Database{t: t, done: make(chan struct{}), data: repair1044Data{Generation: "bootstrap", Revision: 1, Privacy: 1, Generations: map[string]repair1044Generation{"bootstrap": {State: "active", Watermark: 1, Observed: now}}, Source: map[string]json.RawMessage{}, Documents: map[string]map[string]repair1044Document{"bootstrap": {}}, Authority: map[string]repair1044Visibility{}, Creations: map[string]json.RawMessage{}, Events: map[string]json.RawMessage{}, Inbox: map[string]string{}, Outbox: map[string]repair1044Outbox{}}}
	var err error
	d.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(d.done)
		for {
			c, e := d.listener.Accept()
			if e != nil {
				return
			}
			go d.serve(c)
		}
	}()
	t.Cleanup(func() { d.listener.Close(); <-d.done })
	return d
}
func (d *repair1044Database) pool() *pgxpool.Pool {
	d.t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + d.listener.Addr().String() + "/search?sslmode=disable")
	if err != nil {
		d.t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(pool.Close)
	return pool
}
func repair1044Text(v any) []byte {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return []byte("\\x" + hex.EncodeToString(x))
	case json.RawMessage:
		return []byte(x)
	case time.Time:
		return []byte(x.Format("2006-01-02 15:04:05.999999999Z07:00"))
	case bool:
		if x {
			return []byte("t")
		}
		return []byte("f")
	default:
		return []byte(fmt.Sprint(x))
	}
}
func (d *repair1044Database) serve(c net.Conn) {
	defer c.Close()
	b := pgproto3.NewBackend(c, c)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	for k, v := range map[string]string{"server_version": "16.0", "client_encoding": "UTF8", "standard_conforming_strings": "on", "DateStyle": "ISO, MDY", "TimeZone": "UTC", "integer_datetimes": "on"} {
		b.Send(&pgproto3.ParameterStatus{Name: k, Value: v})
	}
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if b.Flush() != nil {
		return
	}
	var sql string
	var args []any
	var result repair1044Rows
	var queryErr error
	var snapshot *repair1044Data
	status := byte('I')
	execute := func(q string, a []any) (repair1044Rows, error) {
		normalized := strings.ToLower(strings.TrimSpace(q))
		if strings.HasPrefix(normalized, "begin") {
			d.mu.Lock()
			copy := repair1044Clone(d.data)
			snapshot = &copy
			status = 'T'
			return repair1044Rows{tag: "BEGIN"}, nil
		}
		if normalized == "commit" {
			snapshot = nil
			status = 'I'
			d.mu.Unlock()
			return repair1044Rows{tag: "COMMIT"}, nil
		}
		if normalized == "rollback" {
			if snapshot != nil {
				d.data = *snapshot
				snapshot = nil
				d.mu.Unlock()
			}
			status = 'I'
			return repair1044Rows{tag: "ROLLBACK"}, nil
		}
		if snapshot == nil {
			d.mu.Lock()
			defer d.mu.Unlock()
		}
		return d.execute(q, a)
	}
	defer func() {
		if snapshot != nil {
			d.data = *snapshot
			d.mu.Unlock()
		}
	}()
	describe := func(r repair1044Rows) {
		if len(r.names) == 0 {
			b.Send(&pgproto3.NoData{})
			return
		}
		fields := make([]pgproto3.FieldDescription, len(r.names))
		for i, name := range r.names {
			fields[i] = pgproto3.FieldDescription{Name: []byte(name), DataTypeOID: r.oids[i], DataTypeSize: -1, TypeModifier: -1}
		}
		b.Send(&pgproto3.RowDescription{Fields: fields})
	}
	output := func(r repair1044Rows, e error) {
		if e != nil {
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: e.Error()})
			return
		}
		for _, row := range r.values {
			values := make([][]byte, len(row))
			for i, v := range row {
				values[i] = repair1044Text(v)
			}
			b.Send(&pgproto3.DataRow{Values: values})
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(r.tag)})
	}
	for {
		m, err := b.Receive()
		if err != nil {
			if err != io.EOF {
				d.t.Errorf("persistence peer: %v", err)
			}
			return
		}
		switch m := m.(type) {
		case *pgproto3.Parse:
			sql = m.Query
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			args = make([]any, len(m.Parameters))
			tm := pgtype.NewMap()
			for i, raw := range m.Parameters {
				if raw == nil {
					continue
				}
				format := int16(0)
				if len(m.ParameterFormatCodes) == 1 {
					format = m.ParameterFormatCodes[0]
				} else if len(m.ParameterFormatCodes) > 1 {
					format = m.ParameterFormatCodes[i]
				}
				if format != pgtype.TextFormatCode {
					d.t.Errorf("exec-mode persistence peer requires text parameters, got %d", format)
					return
				}
				// QueryExecModeExec leaves OIDs unspecified and sends text. Decode
				// PostgreSQL bytea, boolean and timestamp values; all other SQL
				// scalar values retain their text representation.
				switch {
				case bytes.HasPrefix(raw, []byte(`\x`)):
					args[i], err = hex.DecodeString(string(raw[2:]))
				case bytes.Equal(raw, []byte("t")):
					args[i] = true
				case bytes.Equal(raw, []byte("f")):
					args[i] = false
				case len(raw) > 10 && raw[4] == '-' && raw[7] == '-' && raw[10] == ' ':
					codec := &pgtype.TimestamptzCodec{ScanLocation: time.UTC}
					args[i], err = codec.DecodeValue(tm, pgtype.TimestamptzOID, format, raw)
				default:
					args[i] = string(raw)
				}
				if err != nil {
					d.t.Errorf("parameter: %v", err)
					return
				}
			}
			result, queryErr = execute(sql, args)
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			if queryErr == nil {
				describe(result)
			} else {
				b.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			output(result, queryErr)
		case *pgproto3.Sync:
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Query:
			r, e := execute(m.String, nil)
			if len(r.names) > 0 {
				describe(r)
			}
			output(r, e)
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Terminate:
			return
		default:
			d.t.Errorf("unsupported peer protocol message %T", m)
			return
		}
		if b.Flush() != nil {
			return
		}
	}
}
func repair1044String(v any) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}
func repair1044Int(v any) int64 {
	n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}
func repair1044JSON(v any) json.RawMessage {
	if b, ok := v.([]byte); ok {
		return append(json.RawMessage(nil), b...)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
func (d *repair1044Database) execute(sql string, a []any) (repair1044Rows, error) {
	s := &d.data
	q := strings.Join(strings.Fields(sql), " ")
	command := func(tag string) repair1044Rows { return repair1044Rows{tag: tag} }
	text := func(v string) repair1044Rows { return repair1044Result([]string{"value"}, []uint32{25}, []any{v}) }
	boolean := func(v bool) repair1044Rows { return repair1044Result([]string{"value"}, []uint32{16}, []any{v}) }
	payload := func(raw json.RawMessage) repair1044Rows {
		r := repair1044Result([]string{"payload"}, []uint32{3802})
		if raw != nil {
			r.values = [][]any{{raw}}
		}
		return r
	}
	switch {
	case strings.HasPrefix(q, "SET LOCAL "):
		return command("SET"), nil
	case strings.HasPrefix(q, "INSERT INTO indexer.indexer_canonical_creations("):
		s.Creations[repair1044String(a[0])] = repair1044JSON(a[1])
		return command("INSERT 0 1"), nil
	case strings.HasPrefix(q, "INSERT INTO repair_source.events "):
		s.Events[repair1044String(a[0])] = repair1044JSON(a[1])
		return command("INSERT 0 1"), nil
	case strings.HasPrefix(q, `INSERT INTO "repair_delivery".delivery_outbox`):
		id := repair1044String(a[0])
		if _, ok := s.Outbox[id]; ok {
			return command("INSERT 0 0"), nil
		}
		s.Outbox[id] = repair1044Outbox{Subject: repair1044String(a[2]), Wire: []byte(repair1044String(a[4]))}
		return command("INSERT 0 1"), nil
	case strings.HasPrefix(q, `SELECT subject,envelope FROM "repair_delivery".delivery_outbox WHERE event_id=$1`):
		r := repair1044Result([]string{"subject", "envelope"}, []uint32{25, 17})
		if row, ok := s.Outbox[repair1044String(a[0])]; ok {
			r.values = [][]any{{row.Subject, row.Wire}}
		}
		return r, nil
	case strings.HasPrefix(q, `INSERT INTO "repair_delivery".delivery_inbox`):
		key := fmt.Sprint(a[:3])
		if _, ok := s.Inbox[key]; ok {
			return command("INSERT 0 0"), nil
		}
		s.Inbox[key] = repair1044String(a[3])
		return command("INSERT 0 1"), nil
	case strings.HasPrefix(q, `SELECT envelope_hash FROM "repair_delivery".delivery_inbox`):
		return text(s.Inbox[fmt.Sprint(a[:3])]), nil
	case q == "SELECT payload=$2::jsonb FROM indexer.indexer_canonical_creations_v1 WHERE pool=$1":
		var x, y any
		err := json.Unmarshal(s.Creations[repair1044String(a[0])], &x)
		if err != nil {
			return repair1044Rows{}, err
		}
		if err = json.Unmarshal([]byte(repair1044String(a[1])), &y); err != nil {
			return repair1044Rows{}, err
		}
		bx, _ := json.Marshal(x)
		by, _ := json.Marshal(y)
		return boolean(bytes.Equal(bx, by)), nil
	case q == `SELECT payload FROM "repair_source"."events_v2" WHERE event_id=$1`:
		return payload(s.Events[repair1044String(a[0])]), nil
	case q == "SELECT revision FROM stats_search.active_v2 WHERE singleton FOR UPDATE":
		return repair1044Result([]string{"revision"}, []uint32{20}, []any{s.Revision}), nil
	case q == "SELECT payload FROM stats_search.source_documents_v2 WHERE pool_address=$1 FOR UPDATE" || q == "SELECT payload FROM stats_search.source_documents_v2 WHERE pool_address=$1":
		return payload(s.Source[repair1044String(a[0])]), nil
	case strings.HasPrefix(q, "INSERT INTO stats_search.source_documents_v2(pool_address,payload)"):
		s.Source[repair1044String(a[0])] = repair1044JSON(a[1])
		return command("INSERT 0 1"), nil
	case q == "SELECT generation FROM stats_search.generations_v2 WHERE state IN ('active','building') ORDER BY generation FOR UPDATE":
		r := repair1044Result([]string{"generation"}, []uint32{25})
		keys := []string{}
		for g, v := range s.Generations {
			if v.State == "active" || v.State == "building" {
				keys = append(keys, g)
			}
		}
		sort.Strings(keys)
		for _, g := range keys {
			r.values = append(r.values, []any{g})
		}
		return r, nil
	case strings.HasPrefix(q, "INSERT INTO stats_search.documents_v2(generation,pool_address,token_address,"):
		g, p := repair1044String(a[0]), repair1044String(a[1])
		s.Revision++
		s.Documents[g][p] = repair1044Document{Raw: repair1044JSON(a[16]), Revision: s.Revision}
		return command("INSERT 0 1"), nil
	case strings.HasPrefix(q, "INSERT INTO stats_search.authority_v2(pool_address,visibility_revision,public,tombstone)"):
		p := repair1044String(a[0])
		revision := repair1044Int(a[1])
		if old, ok := s.Authority[p]; ok && old.Revision >= revision {
			return command("INSERT 0 0"), nil
		}
		s.Authority[p] = repair1044Visibility{revision, a[2].(bool), a[3].(bool)}
		return command("INSERT 0 1"), nil
	case q == "UPDATE stats_search.active_v2 SET revision=nextval('stats_search.revision_v2'),privacy_revision=privacy_revision+1 WHERE singleton RETURNING revision":
		s.Revision++
		s.Privacy++
		return repair1044Result([]string{"revision"}, []uint32{20}, []any{s.Revision}), nil
	case q == "SELECT generation,revision FROM stats_search.active_v2 WHERE singleton":
		return repair1044Result([]string{"generation", "revision"}, []uint32{25, 20}, []any{s.Generation, s.Revision}), nil
	case q == "SELECT generation FROM stats_search.active_v2 WHERE singleton FOR UPDATE":
		return text(s.Generation), nil
	case q == "INSERT INTO stats_search.generations_v2(generation,state,source_watermark,source_observed_at) VALUES($1,'building',$2,$3)":
		g := repair1044String(a[0])
		if _, ok := s.Generations[g]; ok {
			return repair1044Rows{}, fmt.Errorf("duplicate generation")
		}
		s.Generations[g] = repair1044Generation{State: "building", Watermark: repair1044Int(a[1]), Observed: a[2].(time.Time)}
		s.Documents[g] = map[string]repair1044Document{}
		return command("INSERT 0 1"), nil
	case strings.HasPrefix(q, "INSERT INTO stats_search.documents_v2 SELECT $1,pool_address,payload->>'tokenAddress'"):
		g := repair1044String(a[0])
		for p, raw := range s.Source {
			s.Revision++
			s.Documents[g][p] = repair1044Document{raw, s.Revision}
		}
		return command(fmt.Sprintf("INSERT 0 %d", len(s.Source))), nil
	case q == "SELECT state,source_watermark,source_observed_at,validated_at FROM stats_search.generations_v2 WHERE generation=$1 FOR UPDATE":
		g := s.Generations[repair1044String(a[0])]
		var validated any
		if g.Validated != nil {
			validated = *g.Validated
		}
		return repair1044Result([]string{"state", "source_watermark", "source_observed_at", "validated_at"}, []uint32{25, 20, 1184, 1184}, []any{g.State, g.Watermark, g.Observed, validated}), nil
	case strings.HasPrefix(q, "SELECT EXISTS((SELECT pool_address,payload FROM stats_search.documents_v2 WHERE generation=$1 EXCEPT"):
		docs := s.Documents[repair1044String(a[0])]
		mismatch := len(docs) != len(s.Source)
		for p, raw := range s.Source {
			mismatch = mismatch || !bytes.Equal(docs[p].Raw, raw)
		}
		return boolean(mismatch), nil
	case q == "SELECT pool_address,payload::text FROM stats_search.documents_v2 WHERE generation=$1 ORDER BY pool_address":
		r := repair1044Result([]string{"pool_address", "payload"}, []uint32{25, 25})
		docs := s.Documents[repair1044String(a[0])]
		keys := []string{}
		for p := range docs {
			keys = append(keys, p)
		}
		sort.Strings(keys)
		for _, p := range keys {
			r.values = append(r.values, []any{p, string(docs[p].Raw)})
		}
		return r, nil
	case q == "UPDATE stats_search.generations_v2 SET state='retained' WHERE generation=$1":
		g := repair1044String(a[0])
		v := s.Generations[g]
		v.State = "retained"
		s.Generations[g] = v
		return command("UPDATE 1"), nil
	case q == "UPDATE stats_search.generations_v2 SET state='active',row_count=$2,digest=$3,validated_at=clock_timestamp(),activated_at=clock_timestamp() WHERE generation=$1":
		g := repair1044String(a[0])
		v := s.Generations[g]
		v.State = "active"
		v.Rows = repair1044Int(a[1])
		v.Digest = repair1044String(a[2])
		now := time.Now().UTC()
		v.Validated = &now
		s.Generations[g] = v
		return command("UPDATE 1"), nil
	case q == "UPDATE stats_search.active_v2 SET generation=$1,revision=nextval('stats_search.revision_v2'),privacy_revision=privacy_revision+1 WHERE singleton":
		s.Generation = repair1044String(a[0])
		s.Revision++
		s.Privacy++
		return command("UPDATE 1"), nil
	case strings.HasPrefix(q, "SELECT d.payload FROM stats_search.documents_v2 d JOIN stats_search.authority_v2 v ON v.pool_address=d.pool_address WHERE "):
		if !strings.Contains(q, "v.public AND NOT v.tombstone AND v.visibility_revision>=d.visibility_revision") || !strings.Contains(q, "d.revision<=$3") || !strings.Contains(q, "ORDER BY (d.pool_address=$6) DESC,d.pool_address ASC LIMIT $7") {
			return repair1044Rows{}, fmt.Errorf("public reader lost authority, snapshot or order predicate")
		}
		r := repair1044Result([]string{"payload"}, []uint32{3802})
		docs := s.Documents[repair1044String(a[0])]
		pattern := repair1044String(a[1])
		after, identity := repair1044String(a[4]), repair1044String(a[5])
		matches := []string{}
		for p, row := range docs {
			var doc search.IndexedDocument
			if err := json.Unmarshal(row.Raw, &doc); err != nil {
				return r, err
			}
			authority, exists := s.Authority[p]
			if !exists || !authority.Public || authority.Tombstone || authority.Revision < doc.VisibilityRevision || row.Revision > repair1044Int(a[2]) {
				continue
			}
			if state := repair1044String(a[3]); state != "" && doc.PoolStatus != state {
				continue
			}
			match := false
			switch {
			case strings.Contains(q, "d.pool_address=$2 OR d.token_address=$2"):
				match = p == pattern || doc.TokenAddress == pattern
			case strings.Contains(q, "d.pool_address LIKE $2"):
				prefix := strings.TrimSuffix(pattern, "%")
				match = strings.HasPrefix(p, prefix) || strings.HasPrefix(doc.TokenAddress, prefix)
			case strings.Contains(q, "d.name_key LIKE $2"):
				needle := strings.Trim(pattern, "%")
				match = strings.Contains(doc.NameKey, needle) || strings.Contains(strings.ToLower(doc.CanonicalTicker), needle)
			default:
				return r, fmt.Errorf("unsupported search predicate")
			}
			less := func(x, y string) bool {
				if x == identity {
					return y != identity
				}
				if y == identity {
					return false
				}
				return x < y
			}
			if match && (after == "" || less(after, p)) {
				matches = append(matches, p)
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i] == identity {
				return true
			}
			if matches[j] == identity {
				return false
			}
			return matches[i] < matches[j]
		})
		limit := int(repair1044Int(a[6]))
		if len(matches) > limit {
			matches = matches[:limit]
		}
		for _, p := range matches {
			r.values = append(r.values, []any{docs[p].Raw})
		}
		return r, nil
	default:
		return repair1044Rows{}, fmt.Errorf("persistence fixture has no contract for SQL %q", sql)
	}
}

type repair1044Delivery struct{ acknowledged bool }

func (d *repair1044Delivery) Ack(context.Context) error { d.acknowledged = true; return nil }

type repair1044Path struct {
	t          *testing.T
	db         *repair1044Database
	pool       *pgxpool.Pool
	producer   *delivery.Store
	projection *search.Projection
	server     *httptest.Server
	sequence   int
}

func repair1044Policy() delivery.Policy {
	return delivery.Policy{Subjects: map[string]bool{"repair1044.creation": true, "repair1044.metadata": true, "repair1044.visibility": true, "repair1044.invalidation": true}, EventTypes: map[string]bool{"creation": true, "metadata": true, "visibility": true, "invalidation": true}}
}
func newRepair1044Path(t *testing.T) *repair1044Path {
	t.Helper()
	p := &repair1044Path{t: t, db: newRepair1044Database(t)}
	p.open()
	t.Cleanup(func() { p.server.Close() })
	return p
}
func (p *repair1044Path) open() {
	p.t.Helper()
	p.pool = p.db.pool()
	var err error
	p.producer, err = delivery.NewStore(p.pool, "repair_delivery", repair1044Policy())
	if err != nil {
		p.t.Fatal(err)
	}
	p.projection = &search.Projection{Delivery: p.producer, ConsumerID: "repair1044", InboxGeneration: "one", Sources: map[string]search.SourceRelation{"metadata": {Schema: "repair_source", View: "events_v2"}, "visibility": {Schema: "repair_source", View: "events_v2"}}, InvalidationSubject: "repair1044.invalidation", InvalidationEventType: "invalidation"}
	router := chi.NewRouter()
	httpapi.RegisterCanonicalSearch(router, search.Store{Pool: p.pool})
	p.server = httptest.NewServer(router)
}
func (p *repair1044Path) restart() {
	p.server.Close()
	p.pool.Close()
	p.projection = nil
	p.producer = nil
	p.open()
}
func repair1044Marshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func (p *repair1044Path) deliver(kind string, value any) delivery.Envelope {
	p.t.Helper()
	p.sequence++
	raw := repair1044Marshal(p.t, value)
	ctx := context.Background()
	subject := "repair1044." + kind
	e := delivery.Envelope{EventID: fmt.Sprintf("repair1044_%d", p.sequence), EventType: kind, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "1", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "repair1044", PayloadHash: delivery.Hash(raw), Payload: raw}
	if err := p.producer.Produce(ctx, []delivery.Pending{{Subject: subject, Envelope: e}}, func(ctx context.Context, tx pgx.Tx) error {
		if kind == "creation" {
			fact := value.(search.CanonicalCreation)
			_, err := tx.Exec(ctx, "INSERT INTO indexer.indexer_canonical_creations(pool,payload) VALUES($1,$2)", fact.Pool, raw)
			return err
		}
		_, err := tx.Exec(ctx, "INSERT INTO repair_source.events VALUES($1,$2)", e.EventID, raw)
		return err
	}); err != nil {
		p.t.Fatal(err)
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		p.t.Fatal(err)
	}
	storedSubject, stored, err := p.producer.OriginalEnvelopeTx(ctx, tx, e.EventID)
	if err != nil {
		tx.Rollback(ctx)
		p.t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		p.t.Fatal(err)
	}
	wire, err := stored.Encode(repair1044Policy(), storedSubject)
	if err != nil {
		p.t.Fatal(err)
	}
	want, err := e.Encode(repair1044Policy(), subject)
	if err != nil || !bytes.Equal(wire, want) || storedSubject != subject {
		p.t.Fatal("durable transport changed producer bytes", err)
	}
	p.consume(kind, subject, wire)
	return e
}
func (p *repair1044Path) consume(kind, subject string, wire []byte) {
	p.t.Helper()
	message := &repair1044Delivery{}
	var err error
	if kind == "creation" {
		err = p.projection.ConsumeCanonicalCreation(context.Background(), subject, wire, message)
	} else {
		err = p.projection.Consume(context.Background(), subject, wire, message)
	}
	if err != nil {
		p.t.Fatal(err)
	}
	if !message.acknowledged {
		p.t.Fatal("committed delivery was not acknowledged")
	}
	p.db.mu.Lock()
	defer p.db.mu.Unlock()
	event, err := delivery.Decode(wire, repair1044Policy(), subject)
	if err != nil {
		p.t.Fatal(err)
	}
	if p.db.data.Inbox[fmt.Sprint([]any{"repair1044", "one", event.EventID})] != delivery.Hash(wire) {
		p.t.Fatal("ack without durable inbox identity")
	}
}
func repair1044Address(n int) string { return fmt.Sprintf("0x%040x", n) }
func (p *repair1044Path) create(n int) search.IndexedDocument {
	p.t.Helper()
	fact := search.CanonicalCreation{Pool: repair1044Address(n), Token: repair1044Address(99), Creator: repair1044Address(999), BlockTimestampSeconds: "1700000000"}
	fact.Position.ChainID = "125"
	fact.Position.Height = "1"
	fact.Position.TransactionIndex = "0"
	fact.Position.LogIndex = "0"
	fact.Position.TransactionHash = fmt.Sprintf("0x%064x", n)
	fact.Position.BlockHash = "0x" + strings.Repeat("1", 64)
	fact.Metadata = &struct {
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	}{"Canonical Café", "COIN"}
	p.deliver("creation", fact)
	p.deliver("visibility", search.Change{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: fact.Pool, Public: true})
	return p.document(fact.Pool)
}
func (p *repair1044Path) document(pool string) search.IndexedDocument {
	p.t.Helper()
	var raw []byte
	if err := p.pool.QueryRow(context.Background(), "SELECT payload FROM stats_search.source_documents_v2 WHERE pool_address=$1", pool).Scan(&raw); err != nil {
		p.t.Fatal(err)
	}
	var d search.IndexedDocument
	if err := json.Unmarshal(raw, &d); err != nil {
		p.t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		p.t.Fatal(err)
	}
	return d
}
func (p *repair1044Path) activate(generation string) {
	p.t.Helper()
	ctx := context.Background()
	store := search.Store{Pool: p.pool}
	observed := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.BeginGeneration(ctx, generation, 1, observed); err != nil {
		p.t.Fatal(err)
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		p.t.Fatal(err)
	}
	count, digest, err := search.GenerationDigest(ctx, tx, generation)
	if err != nil {
		tx.Rollback(ctx)
		p.t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		p.t.Fatal(err)
	}
	if err = store.ActivateGeneration(ctx, search.GenerationProof{Generation: generation, SourceWatermark: 1, SourceObservedAt: observed, Rows: count, Digest: digest}); err != nil {
		p.t.Fatal(err)
	}
}

type repair1044Response struct {
	Data struct {
		Items      []search.Hit `json:"items"`
		Generation string       `json:"generation"`
		Status     string       `json:"status"`
	} `json:"data"`
	Revision string  `json:"revision"`
	Next     *string `json:"nextCursor"`
}

func (p *repair1044Path) get(values url.Values, status int) repair1044Response {
	p.t.Helper()
	res, err := p.server.Client().Get(p.server.URL + "/v2/search?" + values.Encode())
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		p.t.Fatal(err)
	}
	if res.StatusCode != status {
		p.t.Fatalf("%v: HTTP %d want %d: %s", values, res.StatusCode, status, raw)
	}
	schema := "SearchResponse"
	if status != 200 {
		schema = "Error"
	}
	if err = v2.ValidateJSON(schema, raw); err != nil {
		p.t.Fatalf("frozen %s: %v: %s", schema, err, raw)
	}
	if res.Header.Get("X-Schema-Version") != "2" || res.Header.Get("Cache-Control") != "no-store" {
		p.t.Fatal(res.Header)
	}
	var out repair1044Response
	if status == 200 {
		if err = json.Unmarshal(raw, &out); err != nil {
			p.t.Fatal(err)
		}
	} else if !bytes.Contains(raw, []byte(`"code":"CURSOR_INVALID"`)) {
		p.t.Fatalf("wrong cursor error: %s", raw)
	}
	return out
}
func repair1044Query(q, kind string, limit int) url.Values {
	return url.Values{"q": {q}, "type": {kind}, "limit": {strconv.Itoa(limit)}}
}
func repair1044IDs(t *testing.T, page repair1044Response, want ...int) {
	t.Helper()
	if len(page.Data.Items) != len(want) {
		t.Fatalf("items %+v want %v", page.Data.Items, want)
	}
	for i, n := range want {
		hit := page.Data.Items[i]
		if hit.ID != "125:"+repair1044Address(n) || hit.PoolAddress == nil || *hit.PoolAddress != repair1044Address(n) || hit.TokenAddress == nil || *hit.TokenAddress != repair1044Address(99) || hit.Wallet != nil || hit.Score != "1" {
			t.Fatalf("hit %+v want pool %d", hit, n)
		}
	}
}

func TestRepairM_1044_CanonicalDeliveryGenerationAndPublicDTO(t *testing.T) {
	p := newRepair1044Path(t)
	d := p.create(1)
	p.create(2)
	d.Name = "Updated Café"
	d.NameKey = "updated café"
	d.MetadataRevision = 2
	p.deliver("metadata", search.Change{Version: 2, Kind: "metadata", Revision: 2, PoolAddress: d.PoolAddress, Document: &d})
	p.activate("canonical_generation")
	page := p.get(repair1044Query("Updated Cafe\u0301", "pool", 25), http.StatusOK)
	repair1044IDs(t, page, 1)
	if page.Data.Generation != "canonical_generation" || page.Data.Items[0].Label != "Updated Café" || page.Revision == "" || page.Next != nil {
		t.Fatal(page)
	}
	repair1044IDs(t, p.get(repair1044Query(repair1044Address(99), "token", 25), 200), 1, 2)
	repair1044IDs(t, p.get(repair1044Query(repair1044Address(1), "pool", 25), 200), 1)
	p.db.mu.Lock()
	defer p.db.mu.Unlock()
	if len(p.db.data.Source) != 2 || len(p.db.data.Documents["canonical_generation"]) != 2 || len(p.db.data.Inbox) != 5 || len(p.db.data.Outbox) != 10 {
		t.Fatalf("producer/consumer cardinality: %+v", p.db.data)
	}
}
func TestRepairM_1044_RestartCursorContinuityAndVisibilityWithdrawal(t *testing.T) {
	p := newRepair1044Path(t)
	p.create(1)
	p.create(2)
	p.create(3)
	p.activate("restart_generation")
	query := repair1044Query("coin", "pool", 1)
	first := p.get(query, 200)
	repair1044IDs(t, first, 1)
	if first.Next == nil {
		t.Fatal("missing first cursor")
	}
	p.restart()
	query.Set("cursor", *first.Next)
	second := p.get(query, 200)
	repair1044IDs(t, second, 2)
	if second.Next == nil || second.Revision != first.Revision || second.Data.Generation != first.Data.Generation {
		t.Fatal("restart changed snapshot", first, second)
	}
	query.Set("cursor", *second.Next)
	repair1044IDs(t, p.get(query, 200), 3)
	p.deliver("visibility", search.Change{Version: 2, Kind: "visibility", Revision: 2, PoolAddress: repair1044Address(2), Public: false, Tombstone: true})
	p.get(query, 400)
	repair1044IDs(t, p.get(repair1044Query("coin", "pool", 25), 200), 1, 3)
	hidden := p.get(repair1044Query(repair1044Address(2), "pool", 25), 200)
	repair1044IDs(t, hidden)
	if hidden.Data.Status != "not-indexed" {
		t.Fatal(hidden)
	}
	p.restart()
	p.activate("withdrawn_generation")
	repair1044IDs(t, p.get(repair1044Query("coin", "pool", 25), 200), 1, 3)
	// A newer source/index row cannot override current independent moderation.
	d := p.document(repair1044Address(2))
	d.Name = "Updated Hidden"
	d.NameKey = "updated hidden"
	d.MetadataRevision = 3
	p.deliver("metadata", search.Change{Version: 2, Kind: "metadata", Revision: 3, PoolAddress: d.PoolAddress, Document: &d})
	repair1044IDs(t, p.get(repair1044Query("hidden", "pool", 25), 200))
}
func TestRepairM_1044_RedeliveryAfterRestartDoesNotChangeSnapshot(t *testing.T) {
	p := newRepair1044Path(t)
	p.create(1)
	p.create(2)
	p.activate("redelivery_generation")
	q := repair1044Query("coin", "pool", 1)
	before := p.get(q, 200)
	if before.Next == nil {
		t.Fatal("missing cursor")
	}
	p.db.mu.Lock()
	row := p.db.data.Outbox["repair1044_1"]
	inbox := len(p.db.data.Inbox)
	outbox := len(p.db.data.Outbox)
	p.db.mu.Unlock()
	p.restart()
	p.consume("creation", row.Subject, row.Wire)
	q.Set("cursor", *before.Next)
	after := p.get(q, 200)
	repair1044IDs(t, after, 2)
	if after.Revision != before.Revision {
		t.Fatal("redelivery advanced public revision", before, after)
	}
	p.db.mu.Lock()
	defer p.db.mu.Unlock()
	if len(p.db.data.Inbox) != inbox || len(p.db.data.Outbox) != outbox || len(p.db.data.Source) != 2 {
		t.Fatal("redelivery duplicated durable effects")
	}
}
