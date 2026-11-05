package metadata

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

// The database and object peers implement their wire protocols and persist
// committed state to disk. Every keeper, codec, SQL query and handler under
// repair is production code. Unknown queries fail explicitly.
type repair1007Work struct {
	Token, State    string
	Staged, Retired []string
	Old             bool
}
type repair1007State struct {
	Visibility                  repair59State
	Canonical                   []byte
	Meta                        *sqlcdb.UpsertTokenMetadataParams
	Images                      map[string]sqlcdb.UpsertTokenImageParams
	Work                        map[string]repair1007Work
	Asset                       AssetRecord
	AssetState, Mime            string
	Width, Height               int
	Variants                    json.RawMessage
	Revision                    int
	NonceOperation, NonceDigest string
}
type repair1007Database struct {
	mu          sync.Mutex
	connections sync.WaitGroup
	state       repair1007State
	fixture     creationFixture
	path        string
	t           *testing.T
	fail        string
	delayInsert time.Duration
}
type repair1007Fixture struct {
	t                      *testing.T
	path                   string
	creation               creationFixture
	db                     *repair1007Database
	pool                   *pgxpool.Pool
	store                  *ProductionStore
	objects                *s3.Client
	legacy                 *storage.Client
	objectServer           *httptest.Server
	objectDir              string
	failBanner, failDelete atomic.Bool
	puts                   atomic.Int64
}

func repair1007Key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, e := crypto.HexToECDSA(strings.Repeat("0", 63) + "1")
	if e != nil {
		t.Fatal(e)
	}
	return key
}

// The signing fixture rebinds only the recorded event's indexed creator to
// the committed local signing key, keeping both event topics coherent.
func repair1007Creator(f creationFixture, key *ecdsa.PrivateKey) creationFixture {
	f.Creator = strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	f.Market.Topics = append([]string(nil), f.Market.Topics...)
	f.Deployed.Topics = append([]string(nil), f.Deployed.Topics...)
	topic := common.BytesToHash(common.HexToAddress(f.Creator).Bytes()).Hex()
	f.Market.Topics[3], f.Deployed.Topics[3] = topic, topic
	return f
}

func newRepair1007Fixture(t *testing.T) *repair1007Fixture {
	t.Helper()
	dir := t.TempDir()
	f := &repair1007Fixture{t: t, path: filepath.Join(dir, "database.json"), objectDir: filepath.Join(dir, "objects"), creation: loadCreationFixture(t)}
	if e := os.Mkdir(f.objectDir, 0700); e != nil {
		t.Fatal(e)
	}
	initial := repair1007State{Images: map[string]sqlcdb.UpsertTokenImageParams{}, Work: map[string]repair1007Work{}, Visibility: repair59State{AuthorityAvailable: true}}
	raw, e := json.Marshal(initial)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(f.path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	f.objectServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		if key == r.URL.Path || strings.Contains(key, "..") {
			http.NotFound(w, r)
			return
		}
		digest := sha256.Sum256([]byte(key))
		path := filepath.Join(f.objectDir, hex.EncodeToString(digest[:]))
		switch r.Method {
		case http.MethodPut:
			if f.failBanner.Load() && strings.Contains(key, "/banner/") {
				w.WriteHeader(400)
				return
			}
			body, e := io.ReadAll(r.Body)
			if e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			if r.Header.Get("If-None-Match") == "*" {
				if _, e = os.Stat(path); e == nil {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(412)
					fmt.Fprint(w, "<Error><Code>PreconditionFailed</Code></Error>")
					return
				}
			}
			if e = os.WriteFile(path, body, 0600); e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			f.puts.Add(1)
			w.WriteHeader(200)
		case http.MethodGet, http.MethodHead:
			body, e := os.ReadFile(path)
			if os.IsNotExist(e) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(404)
				fmt.Fprint(w, "<Error><Code>NoSuchKey</Code></Error>")
				return
			}
			if e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", http.DetectContentType(body))
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			if r.Method == http.MethodGet {
				_, _ = w.Write(body)
			}
		case http.MethodDelete:
			if f.failDelete.Load() {
				w.WriteHeader(400)
				return
			}
			if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected object method %s", r.Method)
			w.WriteHeader(405)
		}
	}))
	f.objects = s3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("fixture", "fixture-only", ""), HTTPClient: f.objectServer.Client()}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(f.objectServer.URL)
		o.UsePathStyle = true
		o.RetryMaxAttempts = 1
	})
	f.legacy = storage.NewFromS3(f.objects, "bucket")
	f.open()
	t.Cleanup(func() { f.stop(); f.objectServer.Close() })
	return f
}
func (f *repair1007Fixture) stop() {
	if f.pool != nil {
		f.pool.Close()
		f.pool = nil
	}
	if f.db != nil {
		f.db.connections.Wait()
	}
}
func (f *repair1007Fixture) open() {
	t := f.t
	t.Helper()
	db := &repair1007Database{fixture: f.creation, path: f.path, t: t}
	raw, e := os.ReadFile(f.path)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &db.state); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig("postgres://fixture@localhost/fixture?sslmode=disable")
	if e != nil {
		t.Fatal(e)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		db.connections.Add(1)
		go func() { defer db.connections.Done(); db.serve(server) }()
		return client, nil
	}
	f.pool, e = pgxpool.NewWithConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	f.store, e = NewProductionStore(f.pool, "repair59_metadata", nil)
	if e != nil {
		t.Fatal(e)
	}
	f.db = db
}
func (f *repair1007Fixture) reopen() { f.stop(); f.open() }
func (d *repair1007Database) snapshot() repair1007State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return repair1007Clone(d.state)
}
func repair1007Clone(s repair1007State) repair1007State {
	raw, e := json.Marshal(s)
	if e != nil {
		panic(e)
	}
	var out repair1007State
	if e = json.Unmarshal(raw, &out); e != nil {
		panic(e)
	}
	return out
}
func (d *repair1007Database) save(s repair1007State) error {
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	file, e := os.OpenFile(d.path+".new", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	_, e = file.Write(raw)
	if e == nil {
		e = file.Sync()
	}
	closeErr := file.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(d.path+".new", d.path)
}
func (d *repair1007Database) serve(conn net.Conn) {
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
	var transaction *repair1007State
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
					snapshot := repair1007Clone(d.state)
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
				if !errors.Is(err, errRepair1007Injected) {
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
							if row[i] != repair1007Null {
								values[i] = []byte(row[i])
							}
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

var errRepair1007Injected = errors.New("injected dependency outage")

const repair1007Null = "\x00"

func repair1007Literals(sql string) []string {
	matches := regexp.MustCompile(`'((?:[^']|'')*)'`).FindAllStringSubmatch(sql, -1)
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = strings.ReplaceAll(m[1], "''", "'")
	}
	return out
}
func repair1007Int(v string) int64 {
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil {
		panic(e)
	}
	return n
}
func repair1007Nullable(v bool, s string) string {
	if !v {
		return repair1007Null
	}
	return s
}
func (d *repair1007Database) query(sql string, state *repair1007State) (repair59Result, error) {
	if d.fail != "" && strings.Contains(sql, d.fail) {
		return repair59Result{}, errRepair1007Injected
	}
	if strings.HasPrefix(sql, "-- name:") {
		sql = strings.TrimSpace(sql[strings.Index(sql, "\n")+1:])
	}
	l := repair1007Literals(sql)
	text := uint32(25)
	row := func(values ...string) repair59Result {
		oids := make([]uint32, len(values))
		for i := range oids {
			oids[i] = text
		}
		return repair59Row(oids, values...)
	}
	empty := func(n int) repair59Result { r := row(make([]string, n)...); r.rows = nil; r.tag = "SELECT 0"; return r }
	tag := func(value string) (repair59Result, error) { return repair59Result{tag: value}, nil }
	f := d.fixture
	switch {
	case strings.HasPrefix(sql, "SELECT pg_advisory_lock") || strings.HasPrefix(sql, "SELECT pg_advisory_unlock"):
		return row(""), nil
	case strings.HasPrefix(sql, "SELECT pg_try_advisory_xact_lock"):
		return repair59Row([]uint32{16}, "t"), nil
	case strings.HasPrefix(sql, `SELECT token_json FROM "repair59_metadata"."development_canonical_tokens"`):
		if len(state.Canonical) == 0 {
			return empty(1), nil
		}
		return repair59Row([]uint32{114}, string(state.Canonical)), nil
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."development_canonical_tokens"`):
		if d.delayInsert > 0 {
			time.Sleep(d.delayInsert)
		}
		if len(l) != 5 || l[0] != f.Token || l[1] != f.Pool || l[2] != f.Creator || l[3] != f.TxHash {
			return repair59Result{}, fmt.Errorf("canonical write binding: %s", sql)
		}
		state.Canonical = json.RawMessage(l[4])
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, "SELECT lower(pool_address)"):
		return repair59Row([]uint32{text, text, text, text, 20}, f.Pool, f.Token, f.Creator, f.TxHash, strconv.FormatUint(f.Height, 10)), nil
	case strings.HasPrefix(sql, "SELECT pool_address, token_address, creator"):
		return row(f.Pool, f.Token, f.Creator), nil
	case strings.HasPrefix(sql, "INSERT INTO metadata.legacy_publications"):
		var keys []string
		if len(l) != 4 || json.Unmarshal([]byte(l[3]), &keys) != nil {
			return repair59Result{}, fmt.Errorf("invalid intent %s", sql)
		}
		state.Work[l[0]] = repair1007Work{Token: l[1], State: l[2], Staged: keys}
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, "UPDATE metadata.legacy_publications SET state='published'"):
		work, ok := state.Work[l[2]]
		if !ok || work.State != "staging" {
			return tag("UPDATE 0")
		}
		work.State = "published"
		if e := json.Unmarshal([]byte(l[1]), &work.Retired); e != nil {
			return repair59Result{}, e
		}
		state.Work[l[2]] = work
		return tag("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT operation::text,token FROM metadata.legacy_publications"):
		r := empty(2)
		for op, w := range state.Work {
			if w.State == "staging" || w.Old {
				r.rows = append(r.rows, []string{op, w.Token})
			}
		}
		return r, nil
	case strings.HasPrefix(sql, "UPDATE metadata.legacy_publications SET created_at="):
		for op, w := range state.Work {
			w.Old = true
			state.Work[op] = w
		}
		return tag("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT CASE WHEN state='staging' THEN staged ELSE retired END"):
		w, ok := state.Work[l[1]]
		if !ok || w.State != "staging" && !w.Old {
			return empty(1), nil
		}
		keys := w.Retired
		if w.State == "staging" {
			keys = w.Staged
		}
		raw, e := json.Marshal(keys)
		return repair59Row([]uint32{114}, string(raw)), e
	case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM metadata.token_images WHERE storage_key="):
		for _, img := range state.Images {
			if img.StorageKey == l[0] {
				return repair59Row([]uint32{16}, "t"), nil
			}
		}
		return repair59Row([]uint32{16}, "f"), nil
	case strings.HasPrefix(sql, "DELETE FROM metadata.legacy_publications"):
		delete(state.Work, l[0])
		return tag("DELETE 1")
	case strings.HasPrefix(sql, "SELECT id, token_address, image_type, storage_key"):
		r := empty(7)
		r.oids = []uint32{text, text, text, text, text, 23, 20}
		for _, img := range state.Images {
			if strings.Contains(sql, "AND image_type") && img.ImageType != l[1] {
				continue
			}
			r.rows = append(r.rows, []string{img.ID, img.TokenAddress, img.ImageType, img.StorageKey, img.MimeType, strconv.FormatInt(int64(img.SizeBytes), 10), strconv.FormatInt(img.UploadedAt, 10)})
		}
		return r, nil
	case strings.HasPrefix(sql, "INSERT INTO metadata.token_images"):
		if len(l) != 7 {
			return repair59Result{}, fmt.Errorf("image params: %s", sql)
		}
		state.Images[l[2]] = sqlcdb.UpsertTokenImageParams{ID: l[0], TokenAddress: l[1], ImageType: l[2], StorageKey: l[3], MimeType: l[4], SizeBytes: int32(repair1007Int(l[5])), UploadedAt: repair1007Int(l[6])}
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, "SELECT token_address, pool_address, name, symbol"):
		if state.Meta == nil {
			return empty(14), nil
		}
		m := state.Meta
		return repair59Row([]uint32{text, text, text, text, text, text, text, text, text, text, text, 20, 20, 23}, m.TokenAddress, m.PoolAddress, repair1007Nullable(m.Name.Valid, m.Name.String), repair1007Nullable(m.Symbol.Valid, m.Symbol.String), repair1007Nullable(m.Description.Valid, m.Description.String), repair1007Nullable(m.Website.Valid, m.Website.String), repair1007Nullable(m.Twitter.Valid, m.Twitter.String), repair1007Nullable(m.Telegram.Valid, m.Telegram.String), repair1007Nullable(m.Discord.Valid, m.Discord.String), repair1007Nullable(m.CustomTags.Valid, m.CustomTags.String), m.CreatedBy, strconv.FormatInt(m.CreatedAt, 10), strconv.FormatInt(m.UpdatedAt, 10), strconv.FormatInt(int64(m.Decimals), 10)), nil
	case strings.HasPrefix(sql, "INSERT INTO metadata.token_metadata"):
		// SQL NULL is intentionally absent from the literal list, so parse the
		// VALUES tuple separately to preserve nullable pgtype fields.
		start := strings.Index(sql, "VALUES (") + len("VALUES (")
		end := strings.Index(sql[start:], ")") + start
		parts := regexp.MustCompile(`(?i)'(?:[^']|'')*'|null`).FindAllString(sql[start:end], -1)
		if len(parts) != 14 {
			return repair59Result{}, fmt.Errorf("metadata values: %s", sql)
		}
		vals := make([]string, 14)
		for i, p := range parts {
			if !strings.EqualFold(p, "null") {
				vals[i] = strings.ReplaceAll(p[1:len(p)-1], "''", "'")
			}
		}
		m := &sqlcdb.UpsertTokenMetadataParams{TokenAddress: vals[0], PoolAddress: vals[1], Name: strToText(vals[2]), Symbol: strToText(vals[3]), Description: strToText(vals[4]), Website: strToText(vals[5]), Twitter: strToText(vals[6]), Telegram: strToText(vals[7]), Discord: strToText(vals[8]), CustomTags: strToText(vals[9]), Decimals: int32(repair1007Int(vals[10])), CreatedBy: vals[11], CreatedAt: repair1007Int(vals[12]), UpdatedAt: repair1007Int(vals[13])}
		if state.Meta != nil {
			m.CreatedAt = state.Meta.CreatedAt
			m.CreatedBy = state.Meta.CreatedBy
			m.PoolAddress = state.Meta.PoolAddress
		}
		state.Meta = m
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."assets"`):
		state.Asset = AssetRecord{ID: l[0], Pool: l[1], Hash: l[3], Size: repair1007Int(l[4]), ObjectKey: l[5]}
		state.AssetState = "CREATED"
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, `SELECT pool,hash,size FROM "repair59_metadata"."assets"`):
		return repair59Row([]uint32{text, text, 20}, state.Asset.Pool, state.Asset.Hash, strconv.FormatInt(state.Asset.Size, 10)), nil
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."asset_details_v2"`):
		state.Mime = l[1]
		state.Width = int(repair1007Int(l[2]))
		state.Height = int(repair1007Int(l[3]))
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, `SELECT mime,width,height FROM "repair59_metadata"."asset_details_v2"`):
		return repair59Row([]uint32{text, 23, 23}, state.Mime, strconv.Itoa(state.Width), strconv.Itoa(state.Height)), nil
	case strings.HasPrefix(sql, `UPDATE "repair59_metadata"."assets" SET state='APPROVED'`):
		state.AssetState = "APPROVED"
		state.Asset.ObjectKey = l[1]
		state.Asset.Engine = l[2]
		state.Asset.Signatures = l[3]
		return tag("UPDATE 1")
	case strings.HasPrefix(sql, `UPDATE "repair59_metadata"."assets" SET state=`):
		if state.Asset.ID != l[1] || state.AssetState != l[2] {
			return tag("UPDATE 0")
		}
		state.AssetState = l[0]
		return tag("UPDATE 1")
	case strings.HasPrefix(sql, `SELECT pool FROM "repair59_metadata"."assets"`):
		return row(state.Asset.Pool), nil
	case strings.HasPrefix(sql, `SELECT pool,state,hash,size FROM "repair59_metadata"."assets"`):
		return repair59Row([]uint32{text, text, text, 20}, state.Asset.Pool, state.AssetState, state.Asset.Hash, strconv.FormatInt(state.Asset.Size, 10)), nil
	case strings.HasPrefix(sql, `SELECT blocked FROM "repair59_metadata"."documents"`):
		value := "f"
		if state.Visibility.Blocked {
			value = "t"
		}
		return repair59Row([]uint32{16}, value), nil
	case strings.HasPrefix(sql, "SELECT to_regclass("):
		return repair59Row([]uint32{16}, "t"), nil
	case strings.HasPrefix(sql, `UPDATE "repair59_metadata"."asset_details_v2" SET variants=`):
		state.Variants = json.RawMessage(l[0])
		return tag("UPDATE 1")
	case strings.HasPrefix(sql, `SELECT pool,creator FROM "repair59_metadata"."pools"`):
		return row(f.Pool, f.Creator), nil
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."v2_nonces"`):
		state.NonceOperation, state.NonceDigest = l[2], l[3]
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, `SELECT operation,digest FROM "repair59_metadata"."v2_nonces"`):
		return row(state.NonceOperation, state.NonceDigest), nil
	case strings.HasPrefix(sql, `SELECT pool,actor,body_hash,result FROM "repair59_metadata"."v2_operations"`):
		return empty(4), nil
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."v2_documents"`):
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, `SELECT revision::text FROM "repair59_metadata"."v2_documents"`):
		return row(strconv.Itoa(state.Revision)), nil
	case strings.HasPrefix(sql, `SELECT state FROM "repair59_metadata"."assets"`):
		return row(state.AssetState), nil
	case strings.HasPrefix(sql, `UPDATE "repair59_metadata"."v2_documents" SET revision=`):
		var patch map[string]any
		if e := json.Unmarshal([]byte(l[0]), &patch); e != nil {
			return repair59Result{}, e
		}
		var doc map[string]any
		if len(state.Visibility.Document) > 0 {
			if e := json.Unmarshal(state.Visibility.Document, &doc); e != nil {
				return repair59Result{}, e
			}
		}
		if doc == nil {
			doc = map[string]any{}
		}
		for k, v := range patch {
			doc[k] = v
		}
		raw, e := json.Marshal(doc)
		if e != nil {
			return repair59Result{}, e
		}
		state.Visibility.Document = raw
		state.Revision++
		state.Visibility.AssetID = l[1]
		return tag("UPDATE 1")
	case strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."v2_operations"`), strings.HasPrefix(sql, `INSERT INTO "repair59_metadata"."v2_outbox"`):
		return tag("INSERT 0 1")
	case strings.HasPrefix(sql, `SELECT p.pool,p.token,d.revision::text,d.document,d.updated_at,d.asset_id,COALESCE(m.blocked,false)`):
		if state.Revision == 0 {
			return empty(7), nil
		}
		blocked := "f"
		if state.Visibility.Blocked {
			blocked = "t"
		}
		return repair59Row([]uint32{text, text, text, 114, 1184, text, 16}, f.Pool, f.Token, strconv.Itoa(state.Revision), string(state.Visibility.Document), "2026-10-13 09:00:00+00", state.Visibility.AssetID, blocked), nil
	case strings.HasPrefix(sql, `SELECT a.hash,d.variants FROM "repair59_metadata"."assets"`):
		if state.AssetState != "APPROVED" {
			return empty(2), nil
		}
		return repair59Row([]uint32{text, 114}, state.Asset.Hash, string(state.Variants)), nil
	}
	base := repair59Database{fixture: f}
	return base.query(sql, &state.Visibility)
}

func repair1007Scanner(t *testing.T) sharedhttp.ScanOptions {
	t.Helper()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				command, e := reader.ReadString(0)
				if e != nil {
					return
				}
				switch command {
				case "zVERSION\x00":
					_, _ = io.WriteString(conn, "ClamAV 1.4.3/27792/fixture\x00")
				case "zINSTREAM\x00":
					var content bytes.Buffer
					for {
						var n uint32
						if e := binary.Read(reader, binary.BigEndian, &n); e != nil {
							return
						}
						if n == 0 {
							break
						}
						if n > 65536 {
							return
						}
						if _, e := io.CopyN(&content, reader, int64(n)); e != nil {
							return
						}
					}
					if !bytes.HasPrefix(content.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
						_, _ = io.WriteString(conn, "stream: ERROR\x00")
						return
					}
					_, _ = io.WriteString(conn, "stream: OK\x00")
				default:
					t.Errorf("unknown scanner command %q", command)
				}
			}()
		}
	}()
	return sharedhttp.ScanOptions{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Timeout: time.Second}
}

func repair1007PublishApproved(t *testing.T, store *ProductionStore, objects S3AssetObjects, f creationFixture, key *ecdsa.PrivateKey, original []byte, asset string, document map[string]any) raster.Raster {
	t.Helper()
	ctx := context.Background()
	decoded, e := raster.Reencode(ctx, original)
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(original)
	a := AssetRecord{ID: asset, Pool: f.Pool, Hash: hex.EncodeToString(digest[:]), Size: int64(len(original)), ObjectKey: "quarantine/" + asset}
	if e = store.CreateConnectedAsset(ctx, a, "image/png", decoded.Width, decoded.Height); e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{"CREATED", "UPLOADING"}, {"UPLOADING", "QUARANTINED"}, {"QUARANTINED", "VALIDATING"}} {
		if e = store.TransitionAsset(ctx, asset, pair[0], pair[1]); e != nil {
			t.Fatal(e)
		}
	}
	store.RasterWorker = fixtureRasterWorker{}
	scan := sharedhttp.ScanVerifiedBuffer(ctx, original, repair1007Scanner(t))
	if !scan.Clean {
		t.Fatalf("scan: %+v", scan)
	}
	if e = store.Promote(ctx, asset, original, decoded.Variants, scan, objects); e != nil {
		t.Fatal(e)
	}
	patch := map[string]any{"expectedRevision": "0", "logoAssetId": asset}
	for k, v := range document {
		patch[k] = v
	}
	raw, e := json.Marshal(patch)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	store.Verifier = V2SignedVerifier{Store: store, Audience: "https://metadata.example", Clock: func() time.Time { return now }}
	digest = sha256.Sum256(raw)
	action := VerifiedAction{Wallet: f.Creator, OperationID: uuid.NewString(), Method: "PATCH", Path: "/v2/tokens/" + f.Token + "/metadata", BodyHash: hex.EncodeToString(digest[:]), Nonce: strings.Repeat("a", 64), ExpiresAt: strconv.FormatInt(now.Add(time.Minute).Unix(), 10), Audience: "https://metadata.example", ChainID: 125}
	signature, e := crypto.Sign(accounts.TextHash([]byte(V2SigningMessage(action))), key)
	if e != nil {
		t.Fatal(e)
	}
	router := chi.NewRouter()
	V2Handlers{Store: store, Clock: func() time.Time { return now }}.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	req, e := http.NewRequest(http.MethodPatch, server.URL+action.Path, bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", action.OperationID)
	req.Header.Set("X-Action-Wallet", action.Wallet)
	req.Header.Set("X-Action-Signature", "0x"+hex.EncodeToString(signature))
	req.Header.Set("X-Action-Nonce", action.Nonce)
	req.Header.Set("X-Action-Expires", action.ExpiresAt)
	req.Header.Set("X-Action-Audience", action.Audience)
	response, e := server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	body, e := io.ReadAll(response.Body)
	if e != nil {
		t.Fatal(e)
	}
	if response.StatusCode != 200 {
		t.Fatalf("publish: %d %s", response.StatusCode, body)
	}
	var published WireResponse[V2Data]
	if e = json.Unmarshal(body, &published); e != nil {
		t.Fatal(e)
	}
	if published.Data.Revision != "1" || published.Data.LogoAssetID == nil || *published.Data.LogoAssetID != asset {
		t.Fatalf("publication result: %s", body)
	}
	return decoded
}

func (f *repair1007Fixture) media() ConnectedMedia {
	return ConnectedMedia{Metadata: V2ContractHandlers{Store: f.store, Canonical: PostgresCanonicalSource{Schema: "repair59_indexer", CanonicalStreamID: "paxeer125-canonical"}}, Objects: S3AssetObjects{Client: f.objects, PublicBucket: "bucket", PublicOrigin: "https://metadata.example"}, Moderation: PostgresVisibilityAuthority{Pool: f.pool, Schema: "repair59_visibility"}}
}

func TestRepairM_1007_ModerationPublishedContent(t *testing.T) {
	for _, first := range []string{"single", "batch", "logo"} {
		t.Run(first, func(t *testing.T) {
			f := newRepair1007Fixture(t)
			f.stop()
			key := repair1007Key(t)
			f.creation = repair1007Creator(f.creation, key)
			f.open()
			document := map[string]any{"name": "Creator override", "symbol": "EDIT", "description": "Creator secret description", "tags": []string{"creator-tag"}, "socials": []V2Social{{Label: "Site", URL: "https://creator.example"}}}
			decoded := repair1007PublishApproved(t, f.store, f.media().Objects, f.creation, key, pngBytes(t, 640, 480), "11111111-1111-4111-8111-111111111111", document)
			state := f.db.snapshot()
			if state.AssetState != "APPROVED" || state.Revision != 1 || state.Visibility.Blocked {
				t.Fatalf("not published: %+v", state)
			}
			initialRouter := chi.NewRouter()
			NewProductionLegacyReads(f.media(), "https://metadata.example").RegisterRoutes(initialRouter)
			initialServer := httptest.NewServer(initialRouter)
			initialReader := &repair59Fixture{creation: f.creation, logo: decoded.Variants[512], server: initialServer}
			for _, route := range []string{"single", "batch", "logo"} {
				initialReader.read(t, route, false)
			}
			initialServer.Close()
			// The authority changes while the token is otherwise idle. No connected
			// read or local documents write is allowed between removal and restart.
			if _, e := f.pool.Exec(context.Background(), `INSERT INTO "repair59_visibility"."safety_visibility"(target_type,target_id,revision,status) VALUES('token',$1,$2,$3) ON CONFLICT(target_type,target_id) DO UPDATE SET revision=EXCLUDED.revision,status=EXCLUDED.status`, f.creation.Pool, int64(17), "removed"); e != nil {
				t.Fatal(e)
			}
			if f.db.snapshot().Visibility.Blocked {
				t.Fatal("authority write changed metadata locally")
			}
			f.reopen()
			media := f.media()
			router := chi.NewRouter()
			NewProductionLegacyReads(media, "https://metadata.example").RegisterRoutes(router)
			server := httptest.NewServer(router)
			defer server.Close()
			readFixture := &repair59Fixture{creation: f.creation, logo: decoded.Variants[512], server: server}
			readFixture.read(t, first, true)
			for _, route := range []string{"single", "batch", "logo"} {
				readFixture.read(t, route, true)
			}
			if state := f.db.snapshot(); !state.Visibility.Blocked || state.Visibility.ModerationRevision != 17 {
				t.Fatalf("visibility not durable: %+v", state.Visibility)
			}
			f.reopen()
			router = chi.NewRouter()
			NewProductionLegacyReads(f.media(), "https://metadata.example").RegisterRoutes(router)
			restarted := httptest.NewServer(router)
			defer restarted.Close()
			readFixture.server = restarted
			for _, route := range []string{"single", "batch", "logo"} {
				readFixture.read(t, route, true)
			}
		})
	}
}

func TestRepairM_1007_CanonicalRetentionRestart(t *testing.T) {
	f := newRepair1007Fixture(t)
	raw, e := os.ReadFile("testdata/paxeer125-creation-0x0f92fdd0.json")
	if e != nil {
		t.Fatal(e)
	}
	var record struct {
		Receipt json.RawMessage `json:"receipt"`
	}
	if e = json.Unmarshal(raw, &record); e != nil {
		t.Fatal(e)
	}
	var pruned atomic.Bool
	var calls atomic.Int64
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params []string        `json:"params"`
		}
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
			t.Error(e)
			w.WriteHeader(400)
			return
		}
		if request.Method != "eth_getTransactionReceipt" || len(request.Params) != 1 || request.Params[0] != f.creation.TxHash {
			t.Errorf("unexpected RPC request: %+v", request)
			w.WriteHeader(400)
			return
		}
		calls.Add(1)
		result := record.Receipt
		if pruned.Load() {
			result = json.RawMessage("null")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer rpc.Close()
	chain, e := ethclient.Dial(rpc.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer chain.Close()
	reader := func() *DevelopmentV2Reads {
		d := NewDevelopmentV2Reads(f.pool, chain, nil)
		media := f.media()
		d.Connected = &media
		return d
	}
	resolve := func(d *DevelopmentV2Reads) (string, error) {
		row, e := d.poolRow(context.Background(), f.creation.Token)
		if e != nil {
			return "", e
		}
		token, e := d.chainToken(context.Background(), row)
		if e != nil {
			return "", e
		}
		encoded, e := json.Marshal(token)
		return string(encoded), e
	}
	f.db.fail = "INSERT INTO \"repair59_metadata\".\"development_canonical_tokens\""
	d := reader()
	if _, e = resolve(d); e == nil {
		t.Fatal("failed insert reported reusable success")
	}
	if len(d.tokens) != 0 || len(f.db.snapshot().Canonical) != 0 {
		t.Fatal("failed retention entered a cache or persisted")
	}
	f.reopen()
	d = reader()
	retained, e := resolve(d)
	if e != nil {
		t.Fatal(e)
	}
	if len(f.db.snapshot().Canonical) == 0 {
		t.Fatal("identity not durably retained")
	}
	before := calls.Load()
	pruned.Store(true)
	f.reopen()
	recovered, e := resolve(reader())
	if e != nil {
		t.Fatal(e)
	}
	if recovered != retained || calls.Load() != before {
		t.Fatalf("pruned restart recovery differs: %s != %s, RPC %d -> %d", recovered, retained, before, calls.Load())
	}
	// A cancelled caller must not leave an unbounded detached persistence write.
	row := developmentPoolRow{Token: f.creation.Token, Pool: f.creation.Pool, Creator: f.creation.Creator, TxHash: f.creation.TxHash}
	token, found, e := reader().persistedToken(context.Background(), row)
	if e != nil || !found {
		t.Fatal(found, e)
	}
	f.db.delayInsert = 4 * time.Second
	start := time.Now()
	e = reader().persistToken(context.Background(), row, token)
	if e == nil || time.Since(start) > 3500*time.Millisecond {
		t.Fatalf("persistence not bounded: %v after %v", e, time.Since(start))
	}
}

func (f *repair1007Fixture) handler() (*Handlers, *httptest.Server) {
	t := f.t
	t.Helper()
	authority := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request sharedauth.IdentityAuthenticationRequest
		if r.URL.Path != "/internal/v2/identity/request:authenticate" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) || json.NewDecoder(r.Body).Decode(&request) != nil || request.Authorization != "Bearer fixture-session" {
			w.WriteHeader(401)
			return
		}
		now := time.Now()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sharedauth.IdentityPrincipal{AccountID: "11111111-1111-4111-8111-111111111111", SessionID: "22222222-2222-4222-8222-222222222222", Wallet: f.creation.Creator, AuthEpoch: 1, ProofAt: now, AccessExpiresAt: now.Add(time.Hour)})
	}))
	t.Cleanup(authority.Close)
	options := repair1007Scanner(t)
	h := New(Deps{Queries: sqlcdb.New(f.pool), LegacyDB: f.pool, Store: f.legacy, PublicURL: "https://metadata.example", PoolLookupAttempts: 1, Identity: sharedauth.HTTPIdentityAuthority{Endpoint: authority.URL, ServiceToken: strings.Repeat("s", 32), AllowPlainHTTP: true}, Scan: func(ctx context.Context, b []byte) sharedhttp.ScanResult {
		return sharedhttp.ScanVerifiedBuffer(ctx, b, options)
	}})
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return h, server
}

func repair1007Upload(t *testing.T, server *httptest.Server, token, name string, logo, banner []byte) int {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	raw, e := json.Marshal(map[string]string{"name": name, "symbol": "EDIT", "description": name + " description"})
	if e != nil {
		t.Fatal(e)
	}
	if e = writer.WriteField("metadata", string(raw)); e != nil {
		t.Fatal(e)
	}
	for _, file := range []struct {
		name string
		body []byte
	}{{"logo", logo}, {"banner", banner}} {
		if file.body == nil {
			continue
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, file.name, file.name+".png"))
		header.Set("Content-Type", "image/png")
		part, e := writer.CreatePart(header)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = part.Write(file.body); e != nil {
			t.Fatal(e)
		}
	}
	if e = writer.Close(); e != nil {
		t.Fatal(e)
	}
	req, e := http.NewRequest(http.MethodPost, server.URL+"/metadata/"+token, &body)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer fixture-session")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, e := server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	raw, e = io.ReadAll(response.Body)
	if e != nil {
		t.Fatal(e)
	}
	if response.StatusCode == 200 {
		var result types.UploadResponse
		if json.Unmarshal(raw, &result) != nil || !result.Success || !result.MetadataUpdated || result.LogoURL == nil || result.BannerURL == nil {
			t.Fatalf("invalid publication result: %s", raw)
		}
	}
	return response.StatusCode
}

func repair1007ReadLegacy(t *testing.T, server *httptest.Server, f *repair1007Fixture, name string, logo, banner []byte) {
	t.Helper()
	for _, path := range []string{"/metadata/" + f.creation.Token, "/metadata/batch?addresses=" + f.creation.Token} {
		status, header, body := getBody(t, server.URL+path)
		if status != 200 || header.Get("Cache-Control") != "no-store" {
			t.Fatalf("read %s: %d %s", path, status, body)
		}
		var result types.PublicMetadata
		if strings.Contains(path, "batch") {
			var batch map[string]types.PublicMetadata
			if e := json.Unmarshal(body, &batch); e != nil {
				t.Fatal(e)
			}
			result = batch[f.creation.Token]
		} else if e := json.Unmarshal(body, &result); e != nil {
			t.Fatal(e)
		}
		if result.Name == nil || *result.Name != name || result.Description == nil || *result.Description != name+" description" || result.Images.Logo == nil || result.Images.Banner == nil {
			t.Fatalf("mixed publication: %s", body)
		}
	}
	for _, image := range []struct {
		kind string
		body []byte
	}{{"logo", logo}, {"banner", banner}} {
		status, header, body := getBody(t, server.URL+"/"+image.kind+"/"+f.creation.Token+".png")
		if status != 200 || header.Get("Cache-Control") != "no-store" || !bytes.Equal(body, image.body) {
			t.Fatalf("mixed %s bytes: %d %x", image.kind, status, sha256.Sum256(body))
		}
	}
}

func TestRepairM_1007_LegacyAtomicPublicationRecovery(t *testing.T) {
	for _, failure := range []string{"database-after-objects", "second-image"} {
		t.Run(failure, func(t *testing.T) {
			f := newRepair1007Fixture(t)
			h, server := f.handler()
			oldLogo, oldBanner := pngBytes(t, 32, 24), pngBytes(t, 48, 24)
			newLogo, newBanner := pngBytes(t, 40, 30), pngBytes(t, 60, 30)
			if status := repair1007Upload(t, server, f.creation.Token, "Old", oldLogo, oldBanner); status != 200 {
				t.Fatalf("initial publication: %d", status)
			}
			repair1007ReadLegacy(t, server, f, "Old", oldLogo, oldBanner)
			before := f.db.snapshot()
			puts := f.puts.Load()
			if failure == "database-after-objects" {
				f.db.mu.Lock()
				f.db.fail = "INSERT INTO metadata.token_metadata"
				f.db.mu.Unlock()
			} else {
				f.failBanner.Store(true)
			}
			if status := repair1007Upload(t, server, f.creation.Token, "New", newLogo, newBanner); status != 500 {
				t.Fatalf("failed update status: %d", status)
			}
			expectedPuts := int64(2)
			if failure == "second-image" {
				expectedPuts = 1
			}
			if f.puts.Load()-puts != expectedPuts {
				t.Fatalf("failure did not follow object success: %d puts", f.puts.Load()-puts)
			}
			after := f.db.snapshot()
			if after.Meta.Name != before.Meta.Name || after.Images["logo"].StorageKey != before.Images["logo"].StorageKey || after.Images["banner"].StorageKey != before.Images["banner"].StorageKey {
				t.Fatalf("partial pointers escaped failed transaction: %+v", after)
			}
			pending := []string{}
			for _, work := range after.Work {
				if work.State == "staging" {
					pending = append(pending, work.Staged...)
				}
			}
			if len(pending) != 2 {
				t.Fatalf("cleanup intent not durable: %+v", after.Work)
			}
			f.db.mu.Lock()
			f.db.fail = ""
			f.db.mu.Unlock()
			f.failBanner.Store(false)
			repair1007ReadLegacy(t, server, f, "Old", oldLogo, oldBanner)
			// Close the handler transport and database, then reconstruct both from
			// durable fixture files. No in-memory intent or pointer can survive.
			server.Close()
			f.reopen()
			h, server = f.handler()
			repair1007ReadLegacy(t, server, f, "Old", oldLogo, oldBanner)
			f.failDelete.Store(true)
			if e := h.RecoverLegacyPublications(context.Background()); e == nil {
				t.Fatal("cleanup failure was hidden")
			}
			found := false
			for _, w := range f.db.snapshot().Work {
				found = found || w.State == "staging"
			}
			if !found {
				t.Fatal("failed cleanup lost its durable retry")
			}
			f.failDelete.Store(false)
			server.Close()
			f.reopen()
			h, server = f.handler()
			if e := h.RecoverLegacyPublications(context.Background()); e != nil {
				t.Fatal(e)
			}
			for _, key := range pending {
				if obj, e := f.legacy.Get(context.Background(), key); !errors.Is(e, storage.ErrNotFound) {
					if obj != nil {
						obj.Body.Close()
					}
					t.Fatalf("abandoned stage survived recovery: %s %v", key, e)
				}
			}
			for _, w := range f.db.snapshot().Work {
				if w.State == "staging" {
					t.Fatal("staging operation survived successful cleanup")
				}
			}
			repair1007ReadLegacy(t, server, f, "Old", oldLogo, oldBanner)
			if status := repair1007Upload(t, server, f.creation.Token, "New", newLogo, newBanner); status != 200 {
				t.Fatalf("retry publication: %d", status)
			}
			current := f.db.snapshot()
			for kind, old := range before.Images {
				if current.Images[kind].StorageKey == old.StorageKey || !strings.HasPrefix(current.Images[kind].StorageKey, "legacy/") {
					t.Fatal("publication reused mutable object key")
				}
				obj, e := f.legacy.Get(context.Background(), old.StorageKey)
				if e != nil {
					t.Fatal("old reader object retired before grace period", e)
				}
				obj.Body.Close()
			}
			server.Close()
			f.reopen()
			h, server = f.handler()
			repair1007ReadLegacy(t, server, f, "New", newLogo, newBanner)
			if _, e := f.pool.Exec(context.Background(), "UPDATE metadata.legacy_publications SET created_at=clock_timestamp()-interval '2 hours'"); e != nil {
				t.Fatal(e)
			}
			if e := h.RecoverLegacyPublications(context.Background()); e != nil {
				t.Fatal(e)
			}
			if len(f.db.snapshot().Work) != 0 {
				t.Fatal("completed cleanup was not retired")
			}
			for _, old := range before.Images {
				if obj, e := f.legacy.Get(context.Background(), old.StorageKey); !errors.Is(e, storage.ErrNotFound) {
					if obj != nil {
						obj.Body.Close()
					}
					t.Fatalf("replaced object survived cleanup: %v", e)
				}
			}
			repair1007ReadLegacy(t, server, f, "New", newLogo, newBanner)
		})
	}
}

func TestRepairM_1007_LegacyMissingTransactionFailsClosed(t *testing.T) {
	f := newRepair1007Fixture(t)
	h, _ := f.handler()
	h.db = nil
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/metadata/"+f.creation.Token, strings.NewReader("not processed")))
	if response.Code != 503 || len(f.db.snapshot().Work) != 0 || f.puts.Load() != 0 {
		t.Fatalf("unwired transaction did not fail closed: %d", response.Code)
	}
}
