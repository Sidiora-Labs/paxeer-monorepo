package store_test

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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func repair32Input() ranker.InputSnapshot {
	cutoff := time.Now().UTC().Truncate(time.Second)
	in := ranker.InputSnapshot{
		RunID: uuid.NewString(), Complete: true, AlgorithmVersion: ranker.AlgorithmVersion,
		FeatureSchemaVersion: ranker.FeatureSchemaVersion, CutoffTimeUTC: cutoff.Unix(), CutoffHeight: 1,
		Candidates: []ranker.Candidate{},
		Provenance: &ranker.Provenance{
			Watermark:      v1.Watermark{ChainId: "125", Height: "1", BlockHash: "0x" + strings.Repeat("a", 64), ProjectionRevision: "1", ObservedAt: cutoff.Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"},
			UniverseDigest: ranker.CanonicalUniverse(nil), StatsRevision: "1", ModerationRevision: "1", ActivityRevision: "1", WindowStart: cutoff.Unix() - 86400, WindowEnd: cutoff.Unix(),
		},
	}
	return in
}

func TestRepair_32_RejectIncompleteOrMismatchedHeavyRuns(t *testing.T) {
	in := repair32Input()
	compute := func(input ranker.InputSnapshot, newOnly bool) []ranker.Snapshot {
		t.Helper()
		snapshots, err := ranker.ComputeRun(input, newOnly)
		if err != nil {
			t.Fatal(err)
		}
		return snapshots
	}
	heavy := compute(in, false)
	if len(heavy) != 7 || len(ranker.Categories) != 8 {
		t.Fatal("seven heavy and eight total categories required")
	}
	newest := compute(in, true)
	duplicate := append([]ranker.Snapshot(nil), heavy...)
	duplicate[len(duplicate)-1] = duplicate[0]
	withNew := append([]ranker.Snapshot(nil), heavy...)
	withNew[len(withNew)-1] = newest[0]
	otherInput := in
	otherInput.RunID = uuid.NewString()
	other := compute(otherInput, false)
	mixedRun := append([]ranker.Snapshot(nil), heavy...)
	mixedRun[len(mixedRun)-1] = other[len(other)-1]
	otherInput = in
	otherInput.Provenance = nil
	noProvenance := compute(otherInput, false)
	otherInput = in
	changed := *in.Provenance
	changed.StatsRevision = "2"
	otherInput.Provenance = &changed
	differentInput := compute(otherInput, false)
	mixedDigest := append([]ranker.Snapshot(nil), heavy...)
	mixedDigest[len(mixedDigest)-1] = differentInput[len(differentInput)-1]
	changed = *in.Provenance
	changed.WindowEnd--
	otherInput.Provenance = &changed
	invalidProvenance := compute(otherInput, false)
	cases := map[string][]ranker.Snapshot{
		"empty": nil, "legacy-five": heavy[:5], "missing-one": heavy[:6],
		"single-heavy": heavy[:1], "all-eight": append(append([]ranker.Snapshot(nil), heavy...), newest...),
		"duplicate": duplicate, "new-in-heavy": withNew, "mixed-run": mixedRun,
		"missing-provenance": noProvenance, "mixed-input": mixedDigest, "invalid-provenance": invalidProvenance,
	}
	for name, batch := range cases {
		t.Run(name, func(t *testing.T) {
			// No database handle: every malformed run must be refused before starting a transaction.
			publication := &store.PublicationStore{}
			if err := publication.Complete(context.Background(), batch); err == nil {
				t.Fatal("malformed run admitted")
			}
		})
	}
}

func TestRepair_32_ProducerRunsReachTransaction(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://repair:repair@localhost:1/repair?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	publication, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	for _, newOnly := range []bool{false, true} {
		snapshots, e := ranker.ComputeRun(repair32Input(), newOnly)
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// A real pool must see both producer shapes; cancellation occurs at transaction acquisition.
		if e = publication.Complete(ctx, snapshots); !errors.Is(e, context.Canceled) {
			t.Fatalf("producer run newOnly=%v rejected before transaction acquisition: %v", newOnly, e)
		}
	}
}

func TestRepair_32_CoordinatorDurableStreamAndHTTPAcrossRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "publication.json")
	var pool *pgxpool.Pool
	var publication *store.PublicationStore
	var server *httptest.Server
	t.Cleanup(func() {
		if server != nil {
			server.Close()
		}
		if pool != nil {
			pool.Close()
		}
	})
	open := func() {
		t.Helper()
		pool = repair32OpenPool(t, path)
		guard, e := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
		if e != nil {
			t.Fatal(e)
		}
		publication, e = store.NewPublicationStore(pool, "ranking", guard)
		if e != nil {
			t.Fatal(e)
		}
	}
	open()
	source := repair32Source{input: repair32PopulatedInput(t)}
	visibility := source
	coordinator, err := ranker.NewCoordinator(source, publication)
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Unix(source.input.CutoffTimeUTC, 0).UTC()
	heavy, err := coordinator.Refresh(ctx, uuid.NewString(), cutoff, false)
	if err != nil {
		t.Fatalf("coordinator heavy publication: %v", err)
	}
	if len(heavy) != 7 {
		t.Fatalf("heavy categories=%d, want 7", len(heavy))
	}
	newest, err := coordinator.Refresh(ctx, uuid.NewString(), cutoff, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(newest) != 1 || newest[0].Category() != "new" {
		t.Fatal("new run is not independent")
	}
	produced := append(append([]ranker.Snapshot(nil), heavy...), newest...)
	want := map[string]v1.RankingSnapshot{}
	nonempty := false
	for _, snapshot := range produced {
		category := snapshot.Category()
		header, e := publication.Current(ctx, category)
		if e != nil {
			t.Fatal(e)
		}
		if header.RunID != snapshot.RunID() || header.InputDigest != snapshot.InputDigest() || header.OutputDigest != snapshot.OutputDigest() || header.Revision != 1 {
			t.Fatalf("%s durable head changed producer identity: %+v", category, header)
		}
		page, e := publication.Page(ctx, header, 0, 100)
		if e != nil {
			t.Fatal(e)
		}
		entries := snapshot.Entries()
		expected := len(entries)
		if expected > 100 {
			expected = 100
		}
		if len(page.Items) != expected {
			t.Fatalf("%s page length differs", category)
		}
		for i, item := range page.Items {
			if item.Address != entries[i].Address || item.Score != strconv.FormatFloat(entries[i].Score, 'f', -1, 64) || item.Rank != int64(i+1) {
				t.Fatalf("%s entry %d differs", category, i)
			}
		}
		if category != "new" && len(entries) > 0 {
			nonempty = true
		}
		wire, e := store.WireSnapshot(header, page.Items)
		if e != nil {
			t.Fatal(e)
		}
		want[category] = wire
	}
	if len(want) != 8 || !nonempty {
		t.Fatal("actual source must exercise all eight categories with at least one nonempty heavy ranking")
	}
	assertCount := func(table string, expected int) {
		t.Helper()
		var count int
		if e := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil || count != expected {
			t.Fatalf("%s count=%d want=%d: %v", table, count, expected, e)
		}
	}
	assertDurable := func() {
		t.Helper()
		for _, table := range []string{"ranking.snapshots", "ranking.category_heads", "ranking.publication_outbox"} {
			assertCount(table, 8)
		}
	}
	serve := func() {
		t.Helper()
		codec, e := ranker.NewCursorCodec([]byte(strings.Repeat("repair32-cursor-key", 2)))
		if e != nil {
			t.Fatal(e)
		}
		handler, e := httpapi.NewSnapshots(publication.SnapshotStore, nil, codec, visibility, 8)
		if e != nil {
			t.Fatal(e)
		}
		router := chi.NewRouter()
		httpapi.RegisterSnapshots(router, handler)
		server = httptest.NewServer(router)
	}
	assertHTTP := func() {
		t.Helper()
		for _, category := range ranker.Categories {
			response, e := server.Client().Get(server.URL + "/rankings/" + category + "?limit=100")
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(io.LimitReader(response.Body, 1<<20))
			response.Body.Close()
			if e != nil || response.StatusCode != http.StatusOK {
				t.Fatalf("%s HTTP status=%d: %s %v", category, response.StatusCode, raw, e)
			}
			var body v1.RankingsResponse
			if e = json.Unmarshal(raw, &body); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(body.Data.Snapshot, want[category]) || response.Header.Get("X-Ranking-Snapshot") != want[category].SnapshotId || body.Quality != "complete" {
				t.Fatalf("%s HTTP consumption differs from durable producer snapshot", category)
			}
		}
	}
	assertDurable()
	assertCount("ranking.publication_outbox WHERE published_at IS NULL", 8)
	serve()
	assertHTTP()
	// Stop both the consumer and its database pool with every stream intent still pending.
	server.Close()
	server = nil
	pool.Close()
	pool = nil
	open()
	assertDurable()
	serve()
	assertHTTP()
	writer := delivery.StreamWriter{Schema: "api_stream"}
	for i := 0; i < 3; i++ {
		worked, e := publication.CommitStreamOnce(ctx, writer, 10)
		if e != nil || !worked {
			t.Fatalf("initial stream commit %d: %v", i, e)
		}
	}
	assertCount("api_stream.stream_delivery_outbox", 3)
	server.Close()
	server = nil
	pool.Close()
	pool = nil
	open()
	assertCount("ranking.publication_outbox WHERE published_at IS NULL", 5)
	for i := 0; i < 5; i++ {
		worked, e := publication.CommitStreamOnce(ctx, writer, 10)
		if e != nil || !worked {
			t.Fatalf("recovered stream commit %d: %v", i, e)
		}
	}
	if worked, e := publication.CommitStreamOnce(ctx, writer, 10); e != nil || worked {
		t.Fatalf("stream intent duplicated after drain: %v", e)
	}
	if e := publication.Complete(ctx, heavy); e != nil {
		t.Fatal("heavy retry", e)
	}
	if e := publication.Complete(ctx, newest); e != nil {
		t.Fatal("new retry", e)
	}
	assertDurable()
	assertCount("ranking.publication_outbox WHERE published_at IS NULL", 0)
	assertCount("api_stream.stream_delivery_outbox", 8)
	assertCount("api_stream.stream_projection", 8)
	for _, category := range ranker.Categories {
		var frame []byte
		var sequence int64
		if e := pool.QueryRow(ctx, "SELECT sequence,frame FROM api_stream.stream_projection WHERE topic=$1", "market.rankings."+category).Scan(&sequence, &frame); e != nil {
			t.Fatal(e)
		}
		if _, e := delivery.ParseCommittedStreamFrame(frame); e != nil {
			t.Fatal(e)
		}
		var event v1.RankingEvent
		if e := json.Unmarshal(frame, &event); e != nil {
			t.Fatal(e)
		}
		if sequence != 1 || event.Sequence != "1" || event.Topic != "market.rankings."+category || !reflect.DeepEqual(event.Payload, want[category]) {
			t.Fatalf("%s recovered stream differs from producer", category)
		}
	}
	serve()
	assertHTTP()
}

// Only the source facts, visibility fence and PostgreSQL storage boundary are
// doubled. Coordinator, PublicationStore, pgx, StreamWriter and HTTP are real.
type repair32Source struct{ input ranker.InputSnapshot }

func (s repair32Source) Freeze(_ context.Context, id string, cutoff time.Time) (ranker.InputSnapshot, error) {
	raw, err := json.Marshal(s.input)
	if err != nil {
		return ranker.InputSnapshot{}, err
	}
	var in ranker.InputSnapshot
	if err = json.Unmarshal(raw, &in); err != nil {
		return in, err
	}
	if cutoff.Unix() != in.CutoffTimeUTC {
		return in, ranker.ErrUnavailable
	}
	in.RunID = id
	return in, nil
}
func (s repair32Source) CurrentRankingRevision(context.Context) (string, time.Time, error) {
	return s.input.Provenance.ModerationRevision, time.Unix(s.input.CutoffTimeUTC, 0), nil
}
func repair32PopulatedInput(t *testing.T) ranker.InputSnapshot {
	t.Helper()
	in := repair32Input()
	in.CutoffHeight = 100
	in.Provenance.Watermark.Height = "100"
	for i := 1; i <= 60; i++ {
		activity, err := ranker.AdjustActivity("2400", "0", true, false, 0, in.CutoffTimeUTC)
		if err != nil {
			t.Fatal(err)
		}
		change := "25"
		if i%2 == 0 {
			change = "-25"
		}
		in.Candidates = append(in.Candidates, ranker.Candidate{
			Pool:           ranker.PoolStat{PoolAddress: fmt.Sprintf("0x%040x", i), Volume24h: "2400", Volume1h: fmt.Sprint(i), Volume5m: "10", MarketCap: "1000", PriceChange24h: change, UniqueTraders24h: 3, HolderCount: 4, BuyCount24h: 5, SellCount24h: 2},
			SourceRevision: "1", SourceObservedAt: in.CutoffTimeUTC, ModerationObservedAt: in.CutoffTimeUTC,
			Depth: "1000", SupplyVerified: true, Activity: activity, CreationHeight: uint64(i),
		})
	}
	in.Provenance.UniverseDigest = ranker.CanonicalUniverse(in.Candidates)
	return in
}

type repair32Stored struct {
	Artifact []byte
	Header   ranker.Header
	Items    []ranker.RankedItem
}
type repair32Intent struct {
	Key                        string
	Sequence, CategorySequence int64
	Published                  bool
	Attempts                   int
	BrokerStream               string
	BrokerSequence             int64
}
type repair32State struct {
	Snapshots      map[string]*repair32Stored
	Heads          map[string]string
	Outbox         []repair32Intent
	Frames         map[string]json.RawMessage
	RejectCategory string
}
type repair32Database struct {
	mu    sync.Mutex
	path  string
	state repair32State
	t     *testing.T
	reads []repair931Read
}

func repair32OpenPool(t *testing.T, path string, reads ...repair931Read) *pgxpool.Pool {
	t.Helper()
	db := &repair32Database{path: path, t: t, reads: reads, state: repair32State{Snapshots: map[string]*repair32Stored{}, Heads: map[string]string{}, Frames: map[string]json.RawMessage{}}}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &db.state); err != nil {
			t.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig("postgres://repair:repair@localhost:1/repair?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.LookupFunc = func(context.Context, string) ([]string, error) { return []string{"127.0.0.1"}, nil }
	cfg.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		go db.serve(server)
		return client, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

type repair32Result struct {
	oids []uint32
	rows [][]string
	tag  string
}

func repair32Columns(sql string) []uint32 {
	switch {
	case strings.HasPrefix(sql, "SELECT category,cutoff_time"):
		return []uint32{25, 20}
	case strings.HasPrefix(sql, "SELECT count(*),count(*) FILTER"):
		return []uint32{20, 20, 20}
	case strings.HasPrefix(sql, "SELECT w.raw_complete_height"):
		return []uint32{20, 20, 16}
	case strings.HasPrefix(sql, "SELECT block_hash,completed_at"):
		return []uint32{25, 1184, 16}
	case strings.HasPrefix(sql, "SELECT NOT EXISTS"):
		return []uint32{16}
	case strings.HasPrefix(sql, "WITH facts AS"):
		return []uint32{25, 20, 20, 20, 20, 25, 25, 25, 20, 20, 20, 25, 25, 25, 25, 25, 20, 25, 25, 20, 25, 25, 20}
	case strings.HasPrefix(sql, "select ") && strings.Contains(sql, "snapshot_entries"):
		return []uint32{pgtype.TextOID, pgtype.TextOID, pgtype.Int8OID, pgtype.TextOID, pgtype.TextOID}
	case strings.HasPrefix(sql, "SELECT s.run_id,s.category"):
		return []uint32{25, 25, 25, 25, 25, 20, 1184, 20, 20, 114}
	case strings.HasPrefix(sql, "SELECT run_id,cutoff_time,revision"):
		return []uint32{25, 20, 20}
	case strings.HasPrefix(sql, "SELECT o.sequence,o.run_id"):
		return []uint32{20, 25, 25, 20}
	case strings.HasPrefix(sql, "SELECT run_id FROM"):
		return []uint32{25}
	case strings.HasPrefix(sql, "SELECT artifact="):
		return []uint32{16}
	case strings.HasPrefix(sql, "SELECT pool_address,score,rank"):
		return []uint32{25, 25, 20}
	case strings.HasPrefix(sql, "SELECT count(*)"):
		return []uint32{20}
	case strings.HasPrefix(sql, "SELECT sequence,frame"):
		return []uint32{20, 17}
	default:
		return nil
	}
}
func repair32Description(oids []uint32) *pgproto3.RowDescription {
	desc := &pgproto3.RowDescription{}
	for i, oid := range oids {
		desc.Fields = append(desc.Fields, pgproto3.FieldDescription{Name: []byte(fmt.Sprint(i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1, Format: 0})
	}
	return desc
}

// A fail-closed, transactional storage double behind the actual pgx pool.
// It accepts only the SQL exercised here; unknown statements fail the test.
// Committed state is written to disk, and every reopen constructs a new server.
func (db *repair32Database) serve(conn net.Conn) {
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
	statements := map[string]*pgproto3.Parse{}
	var sql string
	var args []string
	var tx *repair32State
	var copyData []byte
	status := byte('I')
	sendResult := func(r repair32Result, err error) {
		if err != nil {
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "P0001", Message: err.Error()})
			return
		}
		if len(r.oids) > 0 {
			b.Send(repair32Description(r.oids))
		}
		for _, row := range r.rows {
			values := make([][]byte, len(row))
			for i := range row {
				values[i] = []byte(row[i])
			}
			b.Send(&pgproto3.DataRow{Values: values})
		}
		tag := r.tag
		if tag == "" {
			tag = fmt.Sprintf("SELECT %d", len(r.rows))
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	}
	for {
		message, err := b.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Parse:
			p := *m
			p.ParameterOIDs = append([]uint32(nil), m.ParameterOIDs...)
			statements[p.Name] = &p
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			p := statements[m.PreparedStatement]
			sql = p.Query
			args = make([]string, len(m.Parameters))
			for i, value := range m.Parameters {
				format := int16(0)
				if len(m.ParameterFormatCodes) == 1 {
					format = m.ParameterFormatCodes[0]
				} else if len(m.ParameterFormatCodes) > i {
					format = m.ParameterFormatCodes[i]
				}
				oid := uint32(0)
				if i < len(p.ParameterOIDs) {
					oid = p.ParameterOIDs[i]
				}
				switch {
				case format == 0 && bytes.HasPrefix(value, []byte(`\x`)):
					decoded, decodeErr := hex.DecodeString(string(value[2:]))
					if decodeErr != nil {
						db.t.Errorf("invalid bytea parameter: %v", decodeErr)
						return
					}
					args[i] = string(decoded)
				case format == 1 && oid == pgtype.Int8OID:
					args[i] = strconv.FormatInt(int64(binary.BigEndian.Uint64(value)), 10)
				case format == 1 && oid == pgtype.Int4OID:
					args[i] = strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(value))), 10)
				default:
					args[i] = string(value)
				}
			}
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			query := sql
			if m.ObjectType == 'S' {
				p := statements[m.Name]
				query = p.Query
				b.Send(&pgproto3.ParameterDescription{ParameterOIDs: p.ParameterOIDs})
			}
			if oids := repair32Columns(query); len(oids) > 0 {
				b.Send(repair32Description(oids))
			} else {
				b.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			// The portal description was already sent by Describe.
			r, e := db.query(sql, args, &tx)
			r.oids = nil
			sendResult(r, e)
		case *pgproto3.Query:
			query := strings.TrimSpace(m.String)
			switch {
			case strings.HasPrefix(query, "begin"):
				db.mu.Lock()
				raw, e := json.Marshal(db.state)
				if e == nil {
					tx = &repair32State{}
					e = json.Unmarshal(raw, tx)
				}
				db.mu.Unlock()
				status = 'T'
				sendResult(repair32Result{tag: "BEGIN"}, e)
			case query == "commit":
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
				sendResult(repair32Result{tag: "COMMIT"}, e)
			case query == "rollback":
				tx = nil
				status = 'I'
				sendResult(repair32Result{tag: "ROLLBACK"}, nil)
			case strings.HasPrefix(query, "copy "):
				copyData = nil
				b.Send(&pgproto3.CopyInResponse{OverallFormat: 1, ColumnFormatCodes: []uint16{1, 1, 1, 1, 1}})
				if e := b.Flush(); e != nil {
					return
				}
				continue
			default:
				r, e := db.query(query, nil, &tx)
				sendResult(r, e)
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.CopyData:
			copyData = append(copyData, m.Data...)
			continue
		case *pgproto3.CopyDone:
			n, e := repair32Copy(tx, copyData)
			sendResult(repair32Result{tag: fmt.Sprintf("COPY %d", n)}, e)
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Sync:
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Flush:
		case *pgproto3.Close:
			b.Send(&pgproto3.CloseComplete{})
		default:
			db.t.Errorf("unsupported PostgreSQL message %T", m)
			return
		}
		// Extended-protocol requests are pipelined on net.Pipe. Flush at Sync,
		// Flush, Query and CopyDone so neither endpoint blocks writing to the other.
		switch message.(type) {
		case *pgproto3.Sync, *pgproto3.Flush, *pgproto3.Query, *pgproto3.CopyDone:
			if err = b.Flush(); err != nil {
				return
			}
		}
	}
}

func repair32Copy(tx *repair32State, raw []byte) (int, error) {
	if tx == nil || len(raw) < 19 || !bytes.Equal(raw[:11], []byte("PGCOPY\n\xff\r\n\x00")) {
		return 0, fmt.Errorf("invalid binary COPY")
	}
	r := bytes.NewReader(raw[19:])
	n := 0
	for r.Len() > 0 {
		var columns int16
		if err := binary.Read(r, binary.BigEndian, &columns); err != nil {
			return n, err
		}
		if columns == -1 {
			return n, nil
		}
		if columns != 5 {
			return n, fmt.Errorf("COPY columns %d", columns)
		}
		fields := make([][]byte, 5)
		for i := range fields {
			var size int32
			if err := binary.Read(r, binary.BigEndian, &size); err != nil {
				return n, err
			}
			if size < 0 || int64(size) > int64(r.Len()) {
				return n, fmt.Errorf("invalid COPY field size")
			}
			fields[i] = make([]byte, size)
			if _, err := io.ReadFull(r, fields[i]); err != nil {
				return n, err
			}
		}
		key := string(fields[0]) + "/" + string(fields[1])
		stored := tx.Snapshots[key]
		if stored == nil || len(fields[2]) != 8 {
			return n, fmt.Errorf("COPY without snapshot")
		}
		rank := int64(binary.BigEndian.Uint64(fields[2]))
		if rank != int64(len(stored.Items)+1) {
			return n, fmt.Errorf("COPY rank gap")
		}
		stored.Items = append(stored.Items, ranker.RankedItem{Address: string(fields[3]), Score: string(fields[4]), Rank: rank})
		n++
	}
	return n, nil
}

func (db *repair32Database) query(sql string, args []string, tx **repair32State) (repair32Result, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	s := *tx
	if s == nil {
		s = &db.state
	}
	result := repair32Result{oids: repair32Columns(sql)}
	for _, read := range db.reads {
		if sql == read.SQL {
			if !reflect.DeepEqual(args, read.Args) {
				return result, fmt.Errorf("recorded query bindings differ: %v want %v", args, read.Args)
			}
			result.rows = read.Rows
			return result, nil
		}
	}
	integer := func(i int) int64 { n, _ := strconv.ParseInt(args[i], 10, 64); return n }
	key := func() string { return args[0] + "/" + args[1] }
	switch {
	case strings.HasPrefix(sql, "SET LOCAL "), strings.Contains(sql, "pg_advisory_xact_lock"), strings.Contains(sql, ".assert_epoch("):
		result.tag = "SELECT 1"
	case strings.HasPrefix(sql, "SELECT run_id,cutoff_time,revision"):
		if stored := s.Snapshots[s.Heads[args[0]]]; stored != nil {
			h := stored.Header
			result.rows = [][]string{{h.RunID, fmt.Sprint(h.CutoffTime), fmt.Sprint(h.Revision)}}
		}
	case strings.HasPrefix(sql, "INSERT INTO ") && strings.Contains(sql, `"snapshots"`):
		if s.Snapshots[key()] != nil {
			result.tag = "INSERT 0 0"
			break
		}
		var public struct {
			AlgorithmVersion string            `json:"algorithmVersion"`
			Cutoff           int64             `json:"cutoffTime"`
			Provenance       ranker.Provenance `json:"provenance"`
			Total            int64             `json:"total"`
		}
		if err := json.Unmarshal([]byte(args[5]), &public); err != nil {
			return result, err
		}
		h := ranker.Header{RunID: args[0], Category: args[1], InputDigest: args[2], OutputDigest: args[3], AlgorithmVersion: public.AlgorithmVersion, CutoffTime: public.Cutoff, CompletedAt: time.Now().UTC().Truncate(time.Millisecond), Total: public.Total, Provenance: public.Provenance}
		s.Snapshots[key()] = &repair32Stored{Artifact: []byte(args[4]), Header: h, Items: []ranker.RankedItem{}}
		result.tag = "INSERT 0 1"
	case strings.HasPrefix(sql, "SELECT artifact="):
		stored := s.Snapshots[key()]
		if stored == nil {
			break
		}
		result.rows = [][]string{{strconv.FormatBool(bytes.Equal(stored.Artifact, []byte(args[2])))}}
	case strings.HasPrefix(sql, "INSERT INTO ") && strings.Contains(sql, `"category_heads"`):
		k := args[1] + "/" + args[0]
		s.Heads[args[0]] = k
		s.Snapshots[k].Header.Revision = integer(3)
		result.tag = "INSERT 0 1"
	case strings.HasPrefix(sql, "INSERT INTO ") && strings.Contains(sql, `"publication_outbox"`):
		if args[1] == s.RejectCategory {
			return result, fmt.Errorf("publication outbox constraint rejected %s", args[1])
		}
		s.Outbox = append(s.Outbox, repair32Intent{Key: key(), Sequence: int64(len(s.Outbox) + 1), CategorySequence: integer(3)})
		result.tag = "INSERT 0 1"
	case strings.HasPrefix(sql, "SELECT run_id FROM"):
		if stored := s.Snapshots[s.Heads[args[0]]]; stored != nil {
			result.rows = [][]string{{stored.Header.RunID}}
		}
	case strings.HasPrefix(sql, "SELECT s.run_id,s.category"):
		if stored := s.Snapshots[key()]; stored != nil {
			h := stored.Header
			provenance, err := json.Marshal(h.Provenance)
			if err != nil {
				return result, err
			}
			result.rows = [][]string{{h.RunID, h.Category, h.InputDigest, h.OutputDigest, h.AlgorithmVersion, fmt.Sprint(h.CutoffTime), h.CompletedAt.Format("2006-01-02 15:04:05.999999999Z07:00"), fmt.Sprint(h.Revision), fmt.Sprint(h.Total), string(provenance)}}
		}
	case strings.HasPrefix(sql, "SELECT pool_address,score,rank"):
		if stored := s.Snapshots[key()]; stored != nil {
			offset, limit := int64(0), int64(100)
			if len(args) == 4 {
				offset = integer(2)
				limit = integer(3)
			}
			for _, item := range stored.Items {
				if item.Rank > offset && int64(len(result.rows)) < limit {
					result.rows = append(result.rows, []string{item.Address, item.Score, fmt.Sprint(item.Rank)})
				}
			}
		}
	case strings.HasPrefix(sql, "SELECT o.sequence,o.run_id"):
		for _, intent := range s.Outbox {
			if !intent.Published && intent.Attempts < int(integer(0)) {
				h := s.Snapshots[intent.Key].Header
				result.rows = [][]string{{fmt.Sprint(intent.Sequence), h.RunID, h.Category, fmt.Sprint(intent.CategorySequence)}}
				break
			}
		}
	case strings.HasPrefix(sql, `SELECT "api_stream"."append_ranking"`):
		if *tx == nil {
			return result, fmt.Errorf("stream append outside transaction")
		}
		var update delivery.StreamUpdate
		var proof delivery.StreamSourceProof
		if err := json.Unmarshal([]byte(args[0]), &update); err != nil {
			return result, err
		}
		if err := json.Unmarshal([]byte(args[1]), &proof); err != nil {
			return result, err
		}
		category := strings.TrimPrefix(update.Topic, "market.rankings.")
		stored := s.Snapshots[proof.SourceID+"/"+category]
		if stored == nil || proof.Kind != "ranking_commit" || stored.Header.OutputDigest != proof.SourceDigest || fmt.Sprint(stored.Header.Revision) != proof.Sequence {
			return result, fmt.Errorf("uncommitted stream source")
		}
		expected, err := store.WireSnapshot(stored.Header, stored.Items)
		if err != nil {
			return result, err
		}
		var payload v1.RankingSnapshot
		if err = json.Unmarshal(update.Payload, &payload); err != nil {
			return result, err
		}
		if !reflect.DeepEqual(expected, payload) {
			return result, fmt.Errorf("stream payload differs from committed source")
		}
		frame, err := json.Marshal(map[string]any{"eventId": update.EventID, "topic": update.Topic, "sequence": proof.Sequence, "revision": update.Revision, "occurredAt": update.OccurredAt, "watermark": update.Watermark, "payload": update.Payload, "coalesced": false, "sourceSequenceFirst": nil, "sourceSequenceLast": nil})
		if err != nil {
			return result, err
		}
		if _, exists := s.Frames[update.Topic]; exists {
			return result, fmt.Errorf("duplicate stream append")
		}
		s.Frames[update.Topic] = frame
		result.tag = "SELECT 1"
	case strings.HasPrefix(sql, "UPDATE ") && strings.Contains(sql, " SET published_at="):
		for i := range s.Outbox {
			if s.Outbox[i].Sequence == integer(0) {
				s.Outbox[i].Published = true
				s.Outbox[i].Attempts++
				if len(args) == 3 {
					s.Outbox[i].BrokerStream = args[1]
					s.Outbox[i].BrokerSequence = integer(2)
				}
			}
		}
		result.tag = "UPDATE 1"
	case strings.HasPrefix(sql, "UPDATE ") && strings.Contains(sql, " SET attempts=attempts+1"):
		for i := range s.Outbox {
			if s.Outbox[i].Sequence == integer(0) && !s.Outbox[i].Published && s.Outbox[i].Attempts < int(integer(1)) {
				s.Outbox[i].Attempts++
			}
		}
		result.tag = "UPDATE 1"
	case strings.HasPrefix(sql, "SELECT category,cutoff_time"):
		if len(args) != 2 || args[1] != strconv.Itoa(len(ranker.Categories)) {
			return result, fmt.Errorf("unbounded category health query: %v", args)
		}
		for _, category := range ranker.Categories {
			if stored := s.Snapshots[s.Heads[category]]; stored != nil {
				result.rows = append(result.rows, []string{category, fmt.Sprint(stored.Header.CutoffTime)})
			}
		}
	case strings.HasPrefix(sql, "SELECT count(*),count(*) FILTER"):
		n, exhausted := 0, 0
		for _, intent := range s.Outbox {
			if !intent.Published {
				n++
				if intent.Attempts >= int(integer(0)) {
					exhausted++
				}
			}
		}
		result.rows = [][]string{{fmt.Sprint(n), fmt.Sprint(exhausted), "0"}}
	case strings.HasPrefix(sql, "SELECT count(*)"):
		n := 0
		switch {
		case strings.Contains(sql, "api_stream."):
			n = len(s.Frames)
		case strings.Contains(sql, "ranking.snapshots"):
			n = len(s.Snapshots)
		case strings.Contains(sql, "ranking.category_heads"):
			n = len(s.Heads)
		case strings.Contains(sql, "ranking.publication_outbox"):
			for _, intent := range s.Outbox {
				if !strings.Contains(sql, "published_at IS NULL") || !intent.Published {
					n++
				}
			}
		default:
			return result, fmt.Errorf("unsupported count query: %s", sql)
		}
		result.rows = [][]string{{fmt.Sprint(n)}}
	case strings.HasPrefix(sql, "SELECT sequence,frame"):
		if frame, ok := s.Frames[args[0]]; ok {
			result.rows = [][]string{{"1", `\x` + fmt.Sprintf("%x", frame)}}
		}
	default:
		db.t.Errorf("unsupported storage query: %s", sql)
		return result, fmt.Errorf("unsupported storage query: %s", sql)
	}
	return result, nil
}

func TestRepair_32_HeavyRunRollbackAndEmptyCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "rollback.json")
	state := repair32State{Snapshots: map[string]*repair32Stored{}, Heads: map[string]string{}, Frames: map[string]json.RawMessage{}, RejectCategory: "volume"}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pool := repair32OpenPool(t, path)
	defer func() { pool.Close() }()
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	publication, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := ranker.ComputeRun(repair32PopulatedInput(t), false)
	if err != nil {
		t.Fatal(err)
	}
	// volume sorts last: the injected storage constraint fails after the other
	// categories have staged their snapshots, entries, heads and stream intents.
	if err = publication.Complete(ctx, snapshots); err == nil || !strings.Contains(err.Error(), "outbox constraint") {
		t.Fatalf("heavy run did not reach final outbox constraint: %v", err)
	}
	for _, table := range []string{"ranking.snapshots", "ranking.category_heads", "ranking.publication_outbox"} {
		var count int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial run persisted in %s: %d, %v", table, count, err)
		}
	}
	for _, category := range ranker.HeavyCategoryNames() {
		if _, err = publication.Current(ctx, category); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("rolled back %s head visible: %v", category, err)
		}
	}
	pool.Close()
	// Remove only the injected constraint and reopen the storage boundary.
	state.RejectCategory = ""
	raw, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pool = repair32OpenPool(t, path)
	publication, err = store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := ranker.ComputeRun(repair32Input(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = publication.Complete(ctx, empty); err != nil {
		t.Fatal(err)
	}
	if err = publication.Complete(ctx, empty); err != nil {
		t.Fatalf("empty heavy retry: %v", err)
	}
	pool.Close()
	pool = repair32OpenPool(t, path)
	publication, err = store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range ranker.HeavyCategoryNames() {
		header, err := publication.Current(ctx, category)
		if err != nil || header.Total != 0 || header.Revision != 1 || header.RunID != empty[0].RunID() {
			t.Fatalf("empty %s head not recovered: %+v %v", category, header, err)
		}
		page, err := publication.Page(ctx, header, 0, 100)
		if err != nil || page.Items == nil || len(page.Items) != 0 {
			t.Fatalf("empty %s page unavailable: %+v %v", category, page, err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NULL").Scan(&count); err != nil || count != len(ranker.HeavyCategoryNames()) {
		t.Fatalf("empty run intents: %d %v", count, err)
	}
}
