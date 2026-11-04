package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types"
)

type repair59Fixture struct {
	runtime                                     *pgxpool.Pool
	store                                       *ProductionStore
	media                                       ConnectedMedia
	server                                      *httptest.Server
	creation                                    creationFixture
	metadataSchema, visibilitySchema, statePath string
	logo                                        []byte
	objectReads                                 atomic.Int64
}

func newRepair59Fixture(t *testing.T) *repair59Fixture {
	t.Helper()
	f := &repair59Fixture{
		creation: loadCreationFixture(t), metadataSchema: "repair59_metadata",
		visibilitySchema: "repair59_visibility", statePath: filepath.Join(t.TempDir(), "store.json"),
		logo: pngBytes(t, 512, 384),
	}
	originalHash := sha256.Sum256(pngBytes(t, 640, 480))
	logoHash := sha256.Sum256(f.logo)
	hash := hex.EncodeToString(originalHash[:])
	key := fmt.Sprintf("approved/%s/512/%x.png", hash, logoHash)
	state := repair59State{
		AssetID: "11111111-1111-4111-8111-111111111111", Hash: hash, Key: key,
		Document:           json.RawMessage(`{"name":"Creator override","symbol":"EDIT","description":"Creator secret description","tags":["creator-tag"],"socials":[{"label":"Site","url":"https://creator.example"}]}`),
		AuthorityAvailable: true,
	}
	raw, e := json.Marshal(state)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(f.statePath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	var objectMu sync.Mutex
	var objectBody []byte
	objects := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata-repair59/"+key {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			body, e := io.ReadAll(r.Body)
			if e != nil || !bytes.Equal(body, f.logo) {
				t.Error("object producer sent different logo bytes")
				w.WriteHeader(400)
				return
			}
			objectMu.Lock()
			objectBody = bytes.Clone(body)
			objectMu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			f.objectReads.Add(1)
			objectMu.Lock()
			body := bytes.Clone(objectBody)
			objectMu.Unlock()
			if len(body) == 0 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected S3 method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(objects.Close)
	client := s3.NewFromConfig(aws.Config{
		Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("repair59", "local-only", ""),
		HTTPClient: objects.Client(),
	}, func(o *s3.Options) { o.BaseEndpoint = aws.String(objects.URL); o.UsePathStyle = true })
	if _, e = client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: aws.String("metadata-repair59"), Key: aws.String(key), Body: bytes.NewReader(f.logo), ContentType: aws.String("image/png")}); e != nil {
		t.Fatal(e)
	}
	f.media = ConnectedMedia{
		Metadata: V2ContractHandlers{Canonical: PostgresCanonicalSource{Schema: "repair59_indexer", CanonicalStreamID: "paxeer125-canonical"}},
		Objects:  S3AssetObjects{Client: client, PublicBucket: "metadata-repair59", PublicOrigin: "https://metadata.example"},
	}
	t.Cleanup(f.stop)
	f.open(t)
	return f
}

func (f *repair59Fixture) stop() {
	if f.server != nil {
		f.server.Close()
		f.server = nil
	}
	if f.runtime != nil {
		f.runtime.Close()
		f.runtime = nil
	}
	f.store = nil
}

func (f *repair59Fixture) open(t *testing.T) {
	t.Helper()
	// Each open loads a new database fixture from disk. No reader, connection,
	// moderation state or canonical cache survives a stop/open cycle.
	raw, e := os.ReadFile(f.statePath)
	if e != nil {
		t.Fatal(e)
	}
	db := &repair59Database{fixture: f.creation, path: f.statePath, t: t}
	if e = json.Unmarshal(raw, &db.state); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig("postgres://repair59@localhost/repair59?sslmode=disable")
	if e != nil {
		t.Fatal(e)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) {
		client, backend := net.Pipe()
		go db.serve(backend)
		return client, nil
	}
	f.runtime, e = pgxpool.NewWithConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	f.store, e = NewProductionStore(f.runtime, f.metadataSchema, nil)
	if e != nil {
		t.Fatal(e)
	}
	media := f.media
	media.Metadata.Store = f.store
	media.Moderation = PostgresVisibilityAuthority{Pool: f.runtime, Schema: f.visibilitySchema}
	router := chi.NewRouter()
	NewProductionLegacyReads(media, "https://metadata.example").RegisterRoutes(router)
	f.server = httptest.NewServer(router)
}

func (f *repair59Fixture) visibility(t *testing.T, revision int64, status string) {
	t.Helper()
	table := pgx.Identifier{f.visibilitySchema, "safety_visibility"}.Sanitize()
	if _, e := f.runtime.Exec(context.Background(), "INSERT INTO "+table+"(target_type,target_id,revision,status) VALUES('token',$1,$2,$3) ON CONFLICT(target_type,target_id) DO UPDATE SET revision=EXCLUDED.revision,status=EXCLUDED.status", f.creation.Pool, revision, status); e != nil {
		t.Fatal(e)
	}
}

func (f *repair59Fixture) path(route string) string {
	switch route {
	case "single":
		return "/metadata/" + f.creation.Token + ".json"
	case "batch":
		return "/metadata/batch?addresses=" + f.creation.Token + ",0x7777777777777777777777777777777777777777"
	default:
		return "/logo/" + f.creation.Token + ".png"
	}
}

func (f *repair59Fixture) read(t *testing.T, route string, removed bool) {
	t.Helper()
	objectReads := f.objectReads.Load()
	status, header, body := getBody(t, f.server.URL+f.path(route))
	if header.Get("Cache-Control") != "no-store" {
		t.Fatalf("%s cached: %q", route, header.Get("Cache-Control"))
	}
	if route == "logo" {
		if removed {
			if status != http.StatusNotFound || bytes.Equal(body, f.logo) || f.objectReads.Load() != objectReads {
				t.Fatalf("removed logo: %d %s", status, body)
			}
		} else if status != http.StatusOK || !bytes.Equal(body, f.logo) || header.Get("Content-Type") != "image/png" {
			t.Fatalf("approved logo: %d %s", status, body)
		}
		return
	}
	if status != http.StatusOK {
		t.Fatalf("%s: %d %s", route, status, body)
	}
	var record types.PublicMetadata
	if route == "batch" {
		var batch map[string]types.PublicMetadata
		if e := json.Unmarshal(body, &batch); e != nil {
			t.Fatal(e)
		}
		unknown, ok := batch["0x7777777777777777777777777777777777777777"]
		if len(batch) != 2 || !ok || unknown.Name != nil {
			t.Fatalf("batch shape: %s", body)
		}
		record = batch[f.creation.Token]
	} else if e := json.Unmarshal(body, &record); e != nil {
		t.Fatal(e)
	}
	if record.TokenAddress != f.creation.Token || record.PoolAddress == nil || *record.PoolAddress != f.creation.Pool || record.Creator == nil || *record.Creator != f.creation.Creator || record.Name == nil || record.Symbol == nil {
		t.Fatalf("canonical identity lost: %+v", record)
	}
	if removed {
		if *record.Name != f.creation.Name || *record.Symbol != f.creation.Symbol || record.Description != nil || len(record.Tags) != 0 || record.Socials.Website != nil || record.Images.Logo != nil || bytes.Contains(body, []byte("Creator secret")) || bytes.Contains(body, []byte("creator.example")) {
			t.Fatalf("creator content leaked: %s", body)
		}
	} else if *record.Name != "Creator override" || *record.Symbol != "EDIT" || record.Description == nil || *record.Description != "Creator secret description" || len(record.Tags) != 1 || record.Tags[0] != "creator-tag" || record.Socials.Website == nil || *record.Socials.Website != "https://creator.example" || record.Images.Logo == nil {
		t.Fatalf("approved content missing: %s", body)
	}
}

func (f *repair59Fixture) persisted(t *testing.T, revision string, blocked bool) {
	t.Helper()
	var actualRevision string
	var actualBlocked bool
	if e := f.runtime.QueryRow(context.Background(), "SELECT moderation_revision::text,blocked FROM "+f.store.table("documents")+" WHERE pool=$1", f.creation.Pool).Scan(&actualRevision, &actualBlocked); e != nil {
		t.Fatal(e)
	}
	if actualRevision != revision || actualBlocked != blocked {
		t.Fatalf("persisted visibility: %s %v, want %s %v", actualRevision, actualBlocked, revision, blocked)
	}
}

func TestRepair_59_RevocationLegacyRoutes(t *testing.T) {
	for _, first := range []string{"single", "batch", "logo"} {
		t.Run(first, func(t *testing.T) {
			f := newRepair59Fixture(t)
			for _, route := range []string{"single", "batch", "logo"} {
				f.read(t, route, false)
			}
			f.persisted(t, "0", false)
			f.visibility(t, 17, "removed")
			f.persisted(t, "0", false)
			f.read(t, first, true)
			f.persisted(t, "17", true)
			for _, route := range []string{"single", "batch", "logo"} {
				f.read(t, route, true)
			}
			f.visibility(t, 18, "restored")
			f.read(t, first, false)
			f.persisted(t, "18", false)
			for _, route := range []string{"single", "batch", "logo"} {
				f.read(t, route, false)
			}
		})
	}
}

func TestRepair_59_RevocationAfterRestart(t *testing.T) {
	f := newRepair59Fixture(t)
	for _, route := range []string{"single", "batch", "logo"} {
		f.read(t, route, false)
	}
	f.stop()
	f.open(t)
	f.visibility(t, 17, "removed")
	f.stop()
	f.open(t)
	f.read(t, "logo", true)
	f.persisted(t, "17", true)
	f.stop()
	f.open(t)
	if _, e := f.runtime.Exec(context.Background(), "DELETE FROM "+pgx.Identifier{f.visibilitySchema, "safety_visibility"}.Sanitize()+" WHERE target_id=$1", f.creation.Pool); e != nil {
		t.Fatal(e)
	}
	f.stop()
	f.open(t)
	for _, route := range []string{"single", "batch", "logo"} {
		f.read(t, route, true)
	}
	f.persisted(t, "17", true)
	f.visibility(t, 18, "restored")
	for _, route := range []string{"single", "batch", "logo"} {
		f.read(t, route, false)
	}
	f.persisted(t, "18", false)
}

func TestRepair_59_AuthorityUnavailable(t *testing.T) {
	f := newRepair59Fixture(t)
	for _, route := range []string{"single", "batch", "logo"} {
		f.read(t, route, false)
	}
	if _, e := f.runtime.Exec(context.Background(), "DROP VIEW "+pgx.Identifier{f.visibilitySchema, "safety_visibility_v2"}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	for _, route := range []string{"single", "batch", "logo"} {
		objectReads := f.objectReads.Load()
		status, header, body := getBody(t, f.server.URL+f.path(route))
		if status != http.StatusServiceUnavailable || header.Get("Cache-Control") != "no-store" || f.objectReads.Load() != objectReads || bytes.Contains(body, []byte("Creator")) || bytes.Equal(body, f.logo) {
			t.Fatalf("%s authority outage: %d %s", route, status, body)
		}
	}
	media := f.media
	media.Metadata.Store = f.store
	router := chi.NewRouter()
	NewProductionLegacyReads(media, "https://metadata.example").RegisterRoutes(router)
	for _, route := range []string{"single", "batch", "logo"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, f.path(route), nil))
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s missing authority: %d", route, response.Code)
		}
	}
}

// repair59Database supplies only the database transport for this repair proof.
// The real pgx pool, ProductionStore SQL, canonical event decoder, visibility
// authority and HTTP handlers run unchanged. Unknown SQL is an explicit failure.
// Committed rows live on disk so reopening cannot reuse an in-memory blocked bit.
type repair59State struct {
	AssetID, Hash, Key string
	Document           json.RawMessage
	ModerationRevision int64
	Blocked            bool
	AuthorityRevision  int64
	AuthorityStatus    string
	AuthorityAvailable bool
}

type repair59Database struct {
	mu      sync.Mutex
	state   repair59State
	fixture creationFixture
	path    string
	t       *testing.T
}

type repair59Result struct {
	oids []uint32
	rows [][]string
	tag  string
}

func repair59Row(oids []uint32, values ...string) repair59Result {
	return repair59Result{oids: oids, rows: [][]string{values}, tag: "SELECT 1"}
}

func (d *repair59Database) save(state repair59State) error {
	raw, e := json.Marshal(state)
	if e != nil {
		return e
	}
	if e = os.WriteFile(d.path+".new", raw, 0600); e != nil {
		return e
	}
	return os.Rename(d.path+".new", d.path)
}

func (d *repair59Database) serve(conn net.Conn) {
	defer conn.Close()
	if e := conn.SetDeadline(time.Now().Add(30 * time.Second)); e != nil {
		d.t.Error(e)
		return
	}
	backend := pgproto3.NewBackend(conn, conn)
	if _, e := backend.ReceiveStartupMessage(); e != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	for name, value := range map[string]string{"server_version": "16.0", "client_encoding": "UTF8", "standard_conforming_strings": "on", "integer_datetimes": "on"} {
		backend.Send(&pgproto3.ParameterStatus{Name: name, Value: value})
	}
	backend.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if e := backend.Flush(); e != nil {
		return
	}
	var transaction *repair59State
	for {
		message, e := backend.Receive()
		if e != nil {
			return
		}
		switch message := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			d.mu.Lock()
			var result repair59Result
			var err error
			sql := strings.TrimSpace(message.String)
			switch {
			case strings.HasPrefix(sql, "begin"):
				if transaction != nil {
					err = fmt.Errorf("nested transaction")
				} else {
					snapshot := d.state
					transaction = &snapshot
					result.tag = "BEGIN"
				}
			case sql == "commit":
				if transaction == nil {
					err = fmt.Errorf("commit without transaction")
				} else {
					err = d.save(*transaction)
					if err == nil {
						d.state = *transaction
					}
					transaction = nil
					result.tag = "COMMIT"
				}
			case sql == "rollback":
				transaction = nil
				result.tag = "ROLLBACK"
			case sql == "-- ping":
				result.tag = "SELECT 0"
			default:
				state := &d.state
				if transaction != nil {
					state = transaction
				}
				result, err = d.query(sql, state)
				if err == nil && transaction == nil {
					err = d.save(d.state)
				}
			}
			d.mu.Unlock()
			if err != nil {
				if err != errRepair59AuthorityUnavailable {
					d.t.Errorf("in-process store: %v", err)
				}
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: err.Error()})
			} else {
				if len(result.oids) > 0 {
					fields := make([]pgproto3.FieldDescription, len(result.oids))
					for i, oid := range result.oids {
						fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("column%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1, Format: 0}
					}
					backend.Send(&pgproto3.RowDescription{Fields: fields})
					for _, row := range result.rows {
						values := make([][]byte, len(row))
						for i := range row {
							values[i] = []byte(row[i])
						}
						backend.Send(&pgproto3.DataRow{Values: values})
					}
				}
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(result.tag)})
			}
			status := byte('I')
			if transaction != nil {
				status = 'T'
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if e = backend.Flush(); e != nil {
				return
			}
		default:
			d.t.Errorf("unsupported database protocol message %T", message)
			return
		}
	}
}

func (d *repair59Database) query(sql string, state *repair59State) (repair59Result, error) {
	f := d.fixture
	has := func(value string) bool { return strings.Contains(sql, "'"+value+"'") }
	text := uint32(25)
	boolean := func(v bool) string {
		if v {
			return "t"
		}
		return "f"
	}
	noRows := func(oids ...uint32) repair59Result { return repair59Result{oids: oids, tag: "SELECT 0"} }
	switch {
	case strings.HasPrefix(sql, `SELECT pool FROM "repair59_metadata"."pools" WHERE token=`):
		if !has(f.Token) {
			return noRows(text), nil
		}
		return repair59Row([]uint32{text}, f.Pool), nil
	case strings.HasPrefix(sql, `SELECT creator FROM "repair59_metadata"."pools" WHERE pool=`):
		if !has(f.Pool) {
			return noRows(text), nil
		}
		return repair59Row([]uint32{text}, f.Creator), nil
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."documents"(pool,moderation_revision,blocked)`):
		if !strings.Contains(sql, `WHERE "repair59_metadata"."documents".moderation_revision<EXCLUDED.moderation_revision`) {
			return repair59Result{}, fmt.Errorf("restriction lost monotonic revision predicate: %s", sql)
		}
		matches := regexp.MustCompile(`VALUES\(\s*'([^']+)'\s*,\s*'([0-9]+)'\s*::numeric\s*,\s*'(t|f)'\s*\)`).FindStringSubmatch(sql)
		if len(matches) != 4 || matches[1] != f.Pool {
			return repair59Result{}, fmt.Errorf("invalid restriction SQL: %s", sql)
		}
		revision, e := strconv.ParseInt(matches[2], 10, 64)
		if e != nil {
			return repair59Result{}, e
		}
		if revision > state.ModerationRevision {
			state.ModerationRevision, state.Blocked = revision, matches[3] == "t"
		}
		return repair59Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, `SELECT moderation_revision::text,blocked FROM "repair59_metadata"."documents" WHERE pool=`):
		if !has(f.Pool) {
			return noRows(text, 16), nil
		}
		return repair59Row([]uint32{text, 16}, strconv.FormatInt(state.ModerationRevision, 10), boolean(state.Blocked)), nil
	case strings.HasPrefix(sql, `SELECT revision,status FROM "repair59_visibility"."safety_visibility_v2" WHERE target_type=`):
		if !state.AuthorityAvailable {
			// A missing authority is a transport failure, not an allowed row.
			return repair59Result{}, errRepair59AuthorityUnavailable
		}
		if !has("token") || !has(f.Pool) {
			return repair59Result{}, fmt.Errorf("wrong authority target: %s", sql)
		}
		if state.AuthorityStatus == "" {
			return noRows(20, text), nil
		}
		return repair59Row([]uint32{20, text}, strconv.FormatInt(state.AuthorityRevision, 10), state.AuthorityStatus), nil
	case strings.HasPrefix(sql, `INSERT INTO "repair59_visibility"."safety_visibility"`):
		matches := regexp.MustCompile(`VALUES\('token',\s*'([^']+)'\s*,\s*'([0-9]+)'\s*,\s*'(removed|restored)'\s*\)`).FindStringSubmatch(sql)
		if len(matches) != 4 || matches[1] != f.Pool {
			return repair59Result{}, fmt.Errorf("invalid authority write: %s", sql)
		}
		revision, e := strconv.ParseInt(matches[2], 10, 64)
		if e != nil {
			return repair59Result{}, e
		}
		state.AuthorityRevision, state.AuthorityStatus = revision, matches[3]
		return repair59Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, `DELETE FROM "repair59_visibility"."safety_visibility" WHERE target_id=`):
		if !has(f.Pool) {
			return repair59Result{}, fmt.Errorf("wrong authority delete: %s", sql)
		}
		state.AuthorityRevision, state.AuthorityStatus = 0, ""
		return repair59Result{tag: "DELETE 1"}, nil
	case sql == `DROP VIEW "repair59_visibility"."safety_visibility_v2"`:
		state.AuthorityAvailable = false
		return repair59Result{tag: "DROP VIEW"}, nil
	case strings.HasPrefix(sql, `SELECT p.pool,p.token,d.revision::text,d.document,d.updated_at,d.asset_id,COALESCE(m.blocked,false)`):
		oids := []uint32{text, text, text, 114, 1184, text, 16}
		if !has(f.Token) {
			return noRows(oids...), nil
		}
		return repair59Row(oids, f.Pool, f.Token, "1", string(state.Document), "2026-01-01 00:00:00+00", state.AssetID, boolean(state.Blocked)), nil
	case strings.HasPrefix(sql, `SELECT a.hash,d.variants FROM "repair59_metadata"."assets"`):
		if !has(state.AssetID) || !has(f.Pool) || !has("APPROVED") {
			return noRows(text, 114), nil
		}
		variants, e := json.Marshal(map[string]string{"512": state.Key})
		if e != nil {
			return repair59Result{}, e
		}
		return repair59Row([]uint32{text, 114}, state.Hash, string(variants)), nil
	case strings.HasPrefix(sql, `SELECT token,pool,creator,factory,optical,nft_id::text`):
		oids := []uint32{text, text, text, text, text, text, text, text, text, text, text, text, 114, 114}
		if !has(f.Token) {
			return noRows(oids...), nil
		}
		receiptHash, headerHash := sha256.Sum256([]byte(f.TxHash)), sha256.Sum256([]byte(f.BlockHash))
		provenance := map[string]string{"deploymentId": "paxeer125-a35ff7c6f532d2f9", "abiRevision": "1", "emitterCodeEpoch": "1", "poolCodeEpoch": "1", "receiptSHA256": hex.EncodeToString(receiptHash[:]), "headerSHA256": hex.EncodeToString(headerHash[:])}
		payload := map[string]any{"token": f.Token, "pool": f.Pool, "creator": f.Creator, "factory": f.Factory, "optical": f.Optical, "nftId": f.NFTID, "position": f.position(f.Market), "blockTimestampSeconds": strconv.FormatUint(f.Timestamp, 10), "provenance": provenance,
			"metadata": map[string]any{"emitter": f.Emitter, "name": f.Name, "symbol": f.Symbol, "decimals": f.Decimals, "totalSupply": f.Supply, "position": f.position(f.Deployed), "provenance": provenance}}
		proof, e := json.Marshal(provenance)
		if e != nil {
			return repair59Result{}, e
		}
		raw, e := json.Marshal(payload)
		if e != nil {
			return repair59Result{}, e
		}
		return repair59Row(oids, f.Token, f.Pool, f.Creator, f.Factory, f.Optical, f.NFTID, strconv.FormatUint(f.Height, 10), f.BlockHash, f.TxHash, strconv.FormatUint(f.TxIndex, 10), strconv.FormatInt(mustHex(f.Market.LogIndex), 10), strconv.FormatUint(f.Timestamp, 10), string(proof), string(raw)), nil
	case strings.HasPrefix(sql, `SELECT EXISTS(SELECT 1 FROM "repair59_indexer"."indexer_block_completeness_v1"`):
		if !has(f.BlockHash) || !has("paxeer125-canonical") {
			return repair59Result{}, fmt.Errorf("wrong canonical completeness binding: %s", sql)
		}
		return repair59Row([]uint32{16}, "t"), nil
	case strings.HasPrefix(sql, `SELECT height::text,transaction_index::text,block_hash,emitter,topics,data FROM "repair59_indexer"."indexer_raw_logs_v1"`):
		if !has(f.TxHash) {
			return noRows(text, text, text, text, 114, 17), nil
		}
		for _, log := range []fixtureLog{f.Market, f.Deployed} {
			if !regexp.MustCompile(`log_index=\s*'` + strconv.FormatInt(mustHex(log.LogIndex), 10) + `'\s*::bigint`).MatchString(sql) {
				continue
			}
			topics, e := json.Marshal(log.Topics)
			if e != nil {
				return repair59Result{}, e
			}
			return repair59Row([]uint32{text, text, text, text, 114, 17}, strconv.FormatUint(f.Height, 10), strconv.FormatUint(f.TxIndex, 10), f.BlockHash, strings.ToLower(log.Address), string(topics), "\\x"+strings.TrimPrefix(log.Data, "0x")), nil
		}
		return repair59Result{}, fmt.Errorf("unknown canonical log: %s", sql)
	default:
		return repair59Result{}, fmt.Errorf("unsupported SQL: %s", sql)
	}
}

var errRepair59AuthorityUnavailable = fmt.Errorf("visibility authority view is unavailable")
