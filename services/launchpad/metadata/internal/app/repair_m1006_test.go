package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	m1006Schema       = "repair_m1006"
	m1006Operation    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	m1006Session      = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	m1006Account      = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	m1006Request      = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	m1006Expired      = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	m1006AccountAsset = "ffffffff-ffff-4fff-8fff-ffffffffffff"
)

type m1006Evidence struct {
	Receipt types.Receipt
	Block   json.RawMessage
}

func m1006LoadEvidence(t *testing.T) (m1006Evidence, map[string][]map[string]json.RawMessage) {
	t.Helper()
	var evidence m1006Evidence
	raw, err := os.ReadFile("testdata/repair_m1006_factory.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/repair_m1006_indexer.json")
	if err != nil {
		t.Fatal(err)
	}
	var indexed map[string][]map[string]json.RawMessage
	if err = json.Unmarshal(raw, &indexed); err != nil {
		t.Fatal(err)
	}
	if len(indexed["indexer.indexer_canonical_creations"]) != 1 || len(indexed["indexer.pools"]) != 1 || len(evidence.Receipt.Logs) != 15 || len(indexed["indexer.indexer_raw_logs"]) != 14 {
		t.Fatal("incomplete tracked factory/indexer transcript")
	}
	// The receipt also has the router's final event, which is not in this
	// indexer stream. Every retained raw log must still match its producer.
	logs := make(map[string]*types.Log, len(evidence.Receipt.Logs))
	for _, log := range evidence.Receipt.Logs {
		logs[strconv.FormatUint(uint64(log.Index), 10)] = log
	}
	for _, row := range indexed["indexer.indexer_raw_logs"] {
		log := logs[m1006String(row["log_index"])]
		if log == nil || strings.ToLower(log.Address.Hex()) != m1006String(row["emitter"]) || log.TxHash.Hex() != m1006String(row["transaction_hash"]) || log.BlockHash.Hex() != m1006String(row["block_hash"]) || strconv.FormatUint(log.BlockNumber, 10) != m1006String(row["height"]) || strconv.FormatUint(uint64(log.TxIndex), 10) != m1006String(row["transaction_index"]) {
			t.Fatal("indexer retained log position differs from factory receipt")
		}
		data, err := hex.DecodeString(strings.TrimPrefix(m1006String(row["data"]), "\\x"))
		if err != nil || !bytes.Equal(data, log.Data) {
			t.Fatal("indexer retained data differs from factory receipt")
		}
		var topics []string
		if json.Unmarshal(row["topics"], &topics) != nil || len(topics) != len(log.Topics) {
			t.Fatal("indexer retained topic arity differs from factory receipt")
		}
		for i, topic := range topics {
			if topic != log.Topics[i].Hex() {
				t.Fatal("indexer retained topic differs from factory receipt")
			}
		}
	}
	return evidence, indexed
}

func m1006String(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func m1006Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func m1006PNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// Only the transport peers are fixtures. Production pgx, ProductionStore,
// scanner protocol, raster decoder, S3 client and all maintenance workers run.
type m1006Asset struct {
	ID, Pool, Hash, Key, State, MIME string
	Size                             int64
	Until                            time.Time
	CreatedAt                        time.Time
	Variants                         json.RawMessage
}

type m1006State struct {
	Assets                                   map[string]m1006Asset
	AccountState, PublicationState, Revision string
	Document                                 json.RawMessage
	Outbox                                   int
}

type m1006Database struct {
	t                                                                 *testing.T
	mu                                                                sync.Mutex
	txMu                                                              sync.Mutex
	state                                                             m1006State
	path                                                              string
	now                                                               time.Time
	indexed                                                           map[string][]map[string]json.RawMessage
	body, patch, envelope, admission                                  []byte
	bodyHash, contentHash, envelopeHash, token, pool, creator, txHash string
	published, accountExpired                                         chan struct{}
	publishOnce, accountOnce                                          sync.Once
}

func m1006NewDatabase(t *testing.T) *m1006Database {
	t.Helper()
	evidence, indexed := m1006LoadEvidence(t)
	c := indexed["indexer.indexer_canonical_creations"][0]
	d := &m1006Database{t: t, indexed: indexed, now: time.Now().UTC(), path: filepath.Join(t.TempDir(), "state.json"), published: make(chan struct{}), accountExpired: make(chan struct{})}
	d.token, d.pool, d.creator, d.txHash = m1006String(c["token"]), m1006String(c["pool"]), m1006String(c["creator"]), evidence.Receipt.TxHash.Hex()
	d.patch = []byte("{\"expectedRevision\":\"0\",\"displayName\":\"Repaired publication\",\"description\":\"Accepted before the corrupt upload\",\"tags\":[],\"socialLinks\":[]}")
	d.body, _ = json.Marshal(metadata.PublicationBody{DraftID: m1006Operation, OperationID: m1006Operation, CreationTxHash: d.txHash, Metadata: d.patch, UploadRefs: []string{}})
	if _, _, err := metadata.ParsePublicationBody(d.body); err != nil {
		t.Fatal(err)
	}
	envelope := sharedauth.OperationEnvelope{Version: 2, ChainID: "125", Environment: "development", Actor: d.creator, Action: metadata.PublicationAction, Method: "POST", Path: "/v3/tokens/" + d.token + "/metadata-publications", BodyHash: m1006Hash(d.body), Nonce: strings.Repeat("a", 64), IssuedAt: d.now.Unix(), ExpiresAt: d.now.Add(time.Minute).Unix()}
	statement, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	d.envelope, _ = json.Marshal(envelope)
	d.envelopeHash, d.bodyHash, d.contentHash = m1006Hash([]byte(statement)), m1006Hash(d.body), m1006Hash(d.patch)
	d.admission = []byte("{\"accepted\":true}")
	d.state = m1006State{Assets: map[string]m1006Asset{}, AccountState: "REJECTED", PublicationState: "metadata_pending", Revision: "0", Document: json.RawMessage("{}")}
	d.save(d.state)
	return d
}

func (d *m1006Database) save(state m1006State) {
	d.t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		d.t.Fatal(err)
	}
	if err = os.WriteFile(d.path+".new", raw, 0600); err != nil {
		d.t.Fatal(err)
	}
	if err = os.Rename(d.path+".new", d.path); err != nil {
		d.t.Fatal(err)
	}
}

type m1006Result struct {
	rows [][]any
	tag  string
}

func m1006Row(values ...any) m1006Result { return m1006Result{rows: [][]any{values}, tag: "SELECT 1"} }
func m1006NoRows() m1006Result           { return m1006Result{tag: "SELECT 0"} }

func m1006WireValue(value any) (uint32, []byte) {
	switch v := value.(type) {
	case nil:
		return 25, nil
	case string:
		return 25, []byte(v)
	case bool:
		if v {
			return 16, []byte("t")
		}
		return 16, []byte("f")
	case int:
		return 20, []byte(strconv.Itoa(v))
	case int64:
		return 20, []byte(strconv.FormatInt(v, 10))
	case uint64:
		return 20, []byte(strconv.FormatUint(v, 10))
	case time.Time:
		return 1184, []byte(v.UTC().Format("2006-01-02 15:04:05.999999999") + "+00")
	case json.RawMessage:
		return 114, []byte(v)
	case []byte:
		return 17, []byte("\\x" + hex.EncodeToString(v))
	default:
		panic(fmt.Sprintf("unsupported wire value %T", value))
	}
}

func (d *m1006Database) open() (*pgxpool.Pool, func()) {
	d.t.Helper()
	raw, err := os.ReadFile(d.path)
	if err != nil {
		d.t.Fatal(err)
	}
	d.mu.Lock()
	d.state = m1006State{}
	if err = json.Unmarshal(raw, &d.state); err != nil {
		d.t.Fatal(err)
	}
	d.mu.Unlock()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		d.t.Fatal(err)
	}
	var workers sync.WaitGroup
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() { defer workers.Done(); d.serve(conn) }()
		}
	}()
	cfg, err := pgxpool.ParseConfig("postgres://fixture:fixture@" + listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		d.t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.RuntimeParams["search_path"] = m1006Schema + ",pg_catalog"
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		d.t.Fatal(err)
	}
	var once sync.Once
	stop := func() { once.Do(func() { pool.Close(); listener.Close(); workers.Wait() }) }
	return pool, stop
}

func (d *m1006Database) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	wire := pgproto3.NewBackend(conn, conn)
	if _, err := wire.ReceiveStartupMessage(); err != nil {
		return
	}
	wire.Send(&pgproto3.AuthenticationOk{})
	for k, v := range map[string]string{"server_version": "16.0", "client_encoding": "UTF8", "standard_conforming_strings": "on", "integer_datetimes": "on"} {
		wire.Send(&pgproto3.ParameterStatus{Name: k, Value: v})
	}
	wire.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if wire.Flush() != nil {
		return
	}
	var tx *m1006State
	defer func() {
		if tx != nil {
			d.txMu.Unlock()
		}
	}()
	for {
		message, err := wire.Receive()
		if err != nil {
			return
		}
		switch q := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			sql := strings.TrimSpace(q.String)
			var result m1006Result
			var queryErr error
			switch {
			case strings.HasPrefix(sql, "begin"):
				d.txMu.Lock()
				d.mu.Lock()
				raw, _ := json.Marshal(d.state)
				tx = &m1006State{}
				_ = json.Unmarshal(raw, tx)
				d.mu.Unlock()
				result.tag = "BEGIN"
			case sql == "commit":
				if tx == nil {
					queryErr = fmt.Errorf("commit without transaction")
				} else {
					d.mu.Lock()
					d.state = *tx
					d.save(d.state)
					d.mu.Unlock()
					tx = nil
					d.txMu.Unlock()
					result.tag = "COMMIT"
				}
			case sql == "rollback":
				if tx != nil {
					tx = nil
					d.txMu.Unlock()
				}
				result.tag = "ROLLBACK"
			default:
				if tx == nil {
					d.txMu.Lock()
				}
				d.mu.Lock()
				state := &d.state
				if tx != nil {
					state = tx
				}
				result, queryErr = d.query(sql, state)
				if queryErr == nil && tx == nil {
					d.save(d.state)
				}
				d.mu.Unlock()
				if tx == nil {
					d.txMu.Unlock()
				}
			}
			if queryErr != nil {
				d.t.Errorf("fixture rejected SQL: %v", queryErr)
				wire.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: queryErr.Error()})
			} else {
				if len(result.rows) > 0 {
					fields := make([]pgproto3.FieldDescription, len(result.rows[0]))
					for i, v := range result.rows[0] {
						oid, _ := m1006WireValue(v)
						fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("c%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1}
					}
					wire.Send(&pgproto3.RowDescription{Fields: fields})
					for _, row := range result.rows {
						values := make([][]byte, len(row))
						for i, v := range row {
							_, values[i] = m1006WireValue(v)
						}
						wire.Send(&pgproto3.DataRow{Values: values})
					}
				}
				wire.Send(&pgproto3.CommandComplete{CommandTag: []byte(result.tag)})
			}
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			wire.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if wire.Flush() != nil {
				return
			}
		default:
			d.t.Errorf("unsupported peer protocol %T", q)
			return
		}
	}
}

func (d *m1006Database) query(sql string, state *m1006State) (m1006Result, error) {
	table := func(n string) string { return pgx.Identifier{m1006Schema, n}.Sanitize() }
	has := func(v string) bool { return strings.Contains(sql, "'"+v+"'") }
	var asset m1006Asset
	for id, a := range state.Assets {
		if has(id) {
			asset = a
			break
		}
	}
	mutation := func(tag string) (m1006Result, error) { return m1006Result{tag: tag}, nil }
	quotedJSON := func(marker string) (json.RawMessage, error) {
		i := strings.Index(sql, marker)
		if i < 0 {
			return nil, fmt.Errorf("missing %s", marker)
		}
		literal := regexp.MustCompile("'((?:[^']|'')*)'").FindStringSubmatch(sql[i+len(marker):])
		if len(literal) != 2 {
			return nil, fmt.Errorf("missing JSON value")
		}
		raw := json.RawMessage(strings.ReplaceAll(literal[1], "''", "'"))
		if !json.Valid(raw) {
			return nil, fmt.Errorf("invalid JSON: %s", raw)
		}
		return raw, nil
	}
	switch {
	case sql == "-- ping":
		return m1006NoRows(), nil
	case strings.Contains(sql, "pg_try_advisory_lock") || strings.Contains(sql, "pg_advisory_unlock"):
		return m1006Row(true), nil
	case strings.HasPrefix(sql, "SELECT a.id,a.pool,a.hash,a.size,a.object_key,a.state,d.mime"):
		if !strings.Contains(sql, "d.rejected_until<=clock_timestamp()") || !strings.Contains(sql, "LIMIT 10") {
			return m1006Result{}, fmt.Errorf("scan must bound the batch and filter persisted retry times")
		}
		var ids []string
		for id, a := range state.Assets {
			pending := a.State == "UPLOADING" || a.State == "QUARANTINED" || a.State == "VALIDATING" || a.State == "APPROVED" && string(a.Variants) == "{}"
			if pending && (a.Until.IsZero() || !a.Until.After(d.now)) {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			a, b := state.Assets[ids[i]], state.Assets[ids[j]]
			if a.Until.IsZero() != b.Until.IsZero() {
				return a.Until.IsZero()
			}
			if !a.Until.Equal(b.Until) {
				return a.Until.Before(b.Until)
			}
			if !a.CreatedAt.Equal(b.CreatedAt) {
				return a.CreatedAt.Before(b.CreatedAt)
			}
			return ids[i] < ids[j]
		})
		if len(ids) > 10 {
			ids = ids[:10]
		}
		r := m1006NoRows()
		for _, id := range ids {
			a := state.Assets[id]
			r.rows = append(r.rows, []any{a.ID, a.Pool, a.Hash, a.Size, a.Key, a.State, a.MIME})
		}
		r.tag = fmt.Sprintf("SELECT %d", len(r.rows))
		return r, nil
	case strings.HasPrefix(sql, "UPDATE "+table("asset_details_v2")+" SET rejected_until=clock_timestamp()+interval '30 seconds'"):
		if asset.ID == "" || asset.Until.After(d.now) {
			return mutation("UPDATE 0")
		}
		asset.Until = d.now.Add(30 * time.Second)
		state.Assets[asset.ID] = asset
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "WITH rejected AS(UPDATE "+table("assets")):
		if asset.ID == "" {
			return mutation("UPDATE 0")
		}
		if !strings.Contains(sql, "state='REJECTED'") || !strings.Contains(sql, "interval '24 hours'") {
			return m1006Result{}, fmt.Errorf("rejection lost terminal state or retention")
		}
		asset.State = "REJECTED"
		asset.Until = d.now.Add(24 * time.Hour)
		state.Assets[asset.ID] = asset
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT state FROM "+table("assets")):
		if asset.ID == "" {
			return m1006NoRows(), nil
		}
		return m1006Row(asset.State), nil
	case strings.HasPrefix(sql, "SELECT pool,state,hash,size FROM "+table("assets")):
		return m1006Row(asset.Pool, asset.State, asset.Hash, asset.Size), nil
	case strings.HasPrefix(sql, "SELECT pool FROM "+table("assets")):
		return m1006Row(asset.Pool), nil
	case strings.HasPrefix(sql, "SELECT creator FROM "+table("pools")):
		if !has(d.pool) {
			return m1006Result{}, fmt.Errorf("wrong creator pool")
		}
		return m1006Row(d.creator), nil
	case strings.HasPrefix(sql, "INSERT INTO "+table("documents")):
		if !has(d.pool) {
			return m1006Result{}, fmt.Errorf("wrong moderation pool")
		}
		return mutation("INSERT 0 1")
	case strings.HasPrefix(sql, "SELECT blocked FROM "+table("documents")):
		return m1006Row(false), nil
	case strings.HasPrefix(sql, "UPDATE "+table("assets")+" SET state="):
		if asset.ID == "" {
			return mutation("UPDATE 0")
		}
		m := regexp.MustCompile("SET state=\\s*'([A-Z]+)'").FindStringSubmatch(sql)
		if len(m) != 2 {
			return m1006Result{}, fmt.Errorf("missing asset transition: %s", sql)
		}
		asset.State = m[1]
		state.Assets[asset.ID] = asset
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT to_regclass("):
		return m1006Row(true), nil
	case strings.HasPrefix(sql, "UPDATE "+table("asset_details_v2")+" SET variants="):
		raw, err := quotedJSON("SET variants=")
		if err != nil {
			return m1006Result{}, err
		}
		if asset.ID == "" {
			return mutation("UPDATE 0")
		}
		asset.Variants = raw
		if strings.Contains(sql, "rejected_until=NULL") {
			asset.Until = time.Time{}
		}
		state.Assets[asset.ID] = asset
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT a.id,a.object_key FROM "+table("assets")):
		r := m1006NoRows()
		for _, a := range state.Assets {
			if a.State == "REJECTED" && !a.Until.IsZero() && !a.Until.After(d.now) {
				r.rows = append(r.rows, []any{a.ID, a.Key})
			}
		}
		r.tag = fmt.Sprintf("SELECT %d", len(r.rows))
		return r, nil
	case strings.HasPrefix(sql, "SELECT id::text FROM "+table("account_assets_v2")):
		if strings.Contains(sql, "state='REJECTED'") && state.AccountState == "REJECTED" {
			return m1006Row(m1006AccountAsset), nil
		}
		return m1006NoRows(), nil
	case strings.HasPrefix(sql, "UPDATE "+table("account_assets_v2")+" SET state='DELETED'"):
		if !has(m1006AccountAsset) {
			return m1006Result{}, fmt.Errorf("wrong expired account asset")
		}
		state.AccountState = "DELETED"
		d.accountOnce.Do(func() { close(d.accountExpired) })
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "UPDATE "+table("metadata_publication_jobs_v3")+" j SET state='rejected'"):
		return mutation("UPDATE 0")
	case strings.HasPrefix(sql, "WITH due AS (SELECT j.operation FROM "+table("metadata_publication_jobs_v3")):
		if state.PublicationState != "metadata_pending" {
			return m1006NoRows(), nil
		}
		state.PublicationState = "publishing"
		return m1006Row(m1006Operation, int64(1), 1, d.pool), nil
	case strings.HasPrefix(sql, "SELECT lease_owner="):
		return m1006Row(state.PublicationState == "publishing"), nil
	case strings.HasPrefix(sql, "SELECT i.body,i.metadata,g.envelope,g.response"):
		return m1006Row(d.body, d.patch, d.envelope, d.admission, d.token, d.pool, d.creator, d.txHash, d.contentHash, d.bodyHash, d.envelopeHash, strings.Repeat("a", 64), m1006Request, m1006Account, m1006Session, int64(1)), nil
	case strings.HasPrefix(sql, "SELECT response,envelope_digest,body_digest,request_id::text"):
		return m1006Row(d.admission, d.envelopeHash, d.bodyHash, m1006Request), nil
	case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM "+table("security_audit_outbox")):
		if !has(d.envelopeHash) || !has(m1006Request) {
			return m1006Result{}, fmt.Errorf("publication lost admission binding")
		}
		return m1006Row(true), nil
	case strings.HasPrefix(sql, "SELECT account_id::text,session_id::text,wallet,auth_epoch,proof_at,access_expires_at"):
		if !has(m1006Session) || !has(d.creator) {
			return m1006Result{}, fmt.Errorf("publication identity mismatch")
		}
		return m1006Row(m1006Account, m1006Session, d.creator, int64(1), time.Now().Add(-time.Minute), time.Now().Add(time.Hour)), nil
	case strings.HasPrefix(sql, "SELECT token,pool,creator,factory,optical,nft_id::text"):
		c := d.indexed["indexer.indexer_canonical_creations"][0]
		if !has(d.token) {
			return m1006NoRows(), nil
		}
		return m1006Row(m1006String(c["token"]), m1006String(c["pool"]), m1006String(c["creator"]), m1006String(c["factory"]), m1006String(c["optical"]), m1006String(c["nft_id"]), m1006String(c["height"]), m1006String(c["block_hash"]), m1006String(c["transaction_hash"]), m1006String(c["transaction_index"]), m1006String(c["log_index"]), m1006String(c["block_timestamp"]), c["provenance"], c["payload"]), nil
	case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM "+pgx.Identifier{"indexer", "indexer_block_completeness_v1"}.Sanitize()):
		c := d.indexed["indexer.indexer_block_completeness"][0]
		return m1006Row(has(m1006String(c["height"])) && has(m1006String(c["block_hash"])) && has(m1006String(c["stream_id"]))), nil
	case strings.HasPrefix(sql, "SELECT height::text,transaction_index::text,block_hash,emitter,topics,data"):
		for _, l := range d.indexed["indexer.indexer_raw_logs"] {
			if !has(m1006String(l["transaction_hash"])) || !regexp.MustCompile("log_index=\\s*'"+m1006String(l["log_index"])+"'\\s*::bigint").MatchString(sql) {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(m1006String(l["data"]), "\\x"))
			if err != nil {
				return m1006Result{}, err
			}
			return m1006Row(m1006String(l["height"]), m1006String(l["transaction_index"]), m1006String(l["block_hash"]), m1006String(l["emitter"]), l["topics"], data), nil
		}
		return m1006NoRows(), nil
	case strings.HasPrefix(sql, "SELECT pool,creator FROM "+table("pools")):
		return m1006Row(d.pool, d.creator), nil
	case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM "+table("documents")):
		return m1006Row(false), nil
	case strings.HasPrefix(sql, "SELECT pool,actor,body_hash,result FROM "+table("v2_operations")):
		return m1006NoRows(), nil
	case strings.HasPrefix(sql, "INSERT INTO "+table("v2_documents")):
		return mutation("INSERT 0 1")
	case strings.HasPrefix(sql, "SELECT revision::text FROM "+table("v2_documents")):
		return m1006Row(state.Revision), nil
	case strings.HasPrefix(sql, "UPDATE "+table("v2_documents")+" SET revision=revision+1"):
		if state.Revision != "0" {
			return m1006Result{}, fmt.Errorf("publication applied twice")
		}
		raw, err := quotedJSON("document=document||")
		if err != nil {
			return m1006Result{}, err
		}
		state.Document = raw
		state.Revision = "1"
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT p.pool,p.token,d.revision::text,d.document"):
		return m1006Row(d.pool, d.token, state.Revision, state.Document, time.Now().UTC(), nil, false), nil
	case strings.HasPrefix(sql, "UPDATE "+table("v2_documents")+" SET search_document="):
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "INSERT INTO "+table("v2_operations")):
		return mutation("INSERT 0 1")
	case strings.HasPrefix(sql, "INSERT INTO "+table("v2_outbox")):
		state.Outbox++
		return mutation("INSERT 0 1")
	case strings.HasPrefix(sql, "UPDATE "+table("metadata_publication_jobs_v3")+" SET state='published'"):
		state.PublicationState = "published"
		d.publishOnce.Do(func() { close(d.published) })
		return mutation("UPDATE 1")
	case strings.HasPrefix(sql, "SELECT lower(pool_address),lower(token_address),lower(creator),lower(tx_hash),created_block FROM indexer.pools"):
		if !has(d.token) {
			return m1006NoRows(), nil
		}
		p := d.indexed["indexer.pools"][0]
		height, _ := strconv.ParseInt(m1006String(p["created_block"]), 10, 64)
		return m1006Row(m1006String(p["pool_address"]), m1006String(p["token_address"]), m1006String(p["creator"]), m1006String(p["tx_hash"]), height), nil
	case strings.Contains(sql, "FROM metadata.token_metadata"):
		return m1006NoRows(), nil
	default:
		return m1006Result{}, fmt.Errorf("unsupported SQL: %s", sql)
	}
}

type m1006Objects struct {
	t              *testing.T
	server         *httptest.Server
	mu             sync.Mutex
	data           map[string][]byte
	gets           map[string]int
	mode           string
	unavailable    string
	stalled        string
	stallEntered   chan struct{}
	release        chan struct{}
	stallOnce      sync.Once
	knownAnonymous int
}

func m1006NewObjects(t *testing.T) *m1006Objects {
	o := &m1006Objects{t: t, data: map[string][]byte{}, gets: map[string]int{}, mode: "private", stallEntered: make(chan struct{}), release: make(chan struct{})}
	o.server = httptest.NewServer(http.HandlerFunc(o.serve))
	t.Cleanup(o.server.Close)
	return o
}
func (o *m1006Objects) client() *s3.Client {
	return s3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("fixture", "fixture-secret", ""), HTTPClient: o.server.Client(), RetryMaxAttempts: 1}, func(c *s3.Options) { c.BaseEndpoint = aws.String(o.server.URL); c.UsePathStyle = true })
}
func (o *m1006Objects) serve(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")
	signed := strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ")
	if !signed {
		o.mu.Lock()
		body, exists := o.data[key]
		readable := o.mode == "readable" || o.mode == "account-readable" && strings.Contains(key, "/quarantine/account/")
		mode := o.mode
		if exists {
			o.knownAnonymous++
		}
		o.mu.Unlock()
		if mode == "redirect" {
			http.Redirect(w, r, "/quarantine-bucket", http.StatusTemporaryRedirect)
			return
		}
		if mode == "listing" && r.URL.Query().Get("list-type") != "" {
			w.WriteHeader(200)
			return
		}
		if r.Method == http.MethodGet && exists && readable {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Write(body)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(200)
		return
	}
	if r.URL.Query().Has("acl") {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "<AccessControlPolicy><Owner><ID>private-owner</ID></Owner><AccessControlList><Grant><Grantee><ID>private-owner</ID></Grantee><Permission>FULL_CONTROL</Permission></Grant></AccessControlList></AccessControlPolicy>")
		return
	}
	if r.URL.Query().Has("policyStatus") {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "<PolicyStatus><IsPublic>false</IsPublic></PolicyStatus>")
		return
	}
	switch r.Method {
	case http.MethodPut:
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			o.t.Error(err)
			w.WriteHeader(400)
			return
		}
		o.mu.Lock()
		if _, exists := o.data[key]; exists && r.Header.Get("If-None-Match") == "*" {
			o.mu.Unlock()
			w.WriteHeader(412)
			return
		}
		o.data[key] = bytes.Clone(raw)
		o.mu.Unlock()
		w.WriteHeader(200)
	case http.MethodGet:
		o.mu.Lock()
		o.gets[key]++
		body, exists := o.data[key]
		stalled, unavailable := key == o.stalled, key == o.unavailable
		o.mu.Unlock()
		if stalled {
			o.stallOnce.Do(func() { close(o.stallEntered) })
			select {
			case <-o.release:
			case <-r.Context().Done():
				return
			}
		}
		if unavailable {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(503)
			io.WriteString(w, "<Error><Code>ServiceUnavailable</Code></Error>")
			return
		}
		if !exists {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(404)
			io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Write(body)
	case http.MethodDelete:
		o.mu.Lock()
		delete(o.data, key)
		o.mu.Unlock()
		w.WriteHeader(204)
	default:
		o.t.Errorf("unsupported S3 peer request %s %s", r.Method, r.URL)
		w.WriteHeader(405)
	}
}
func (o *m1006Objects) put(t *testing.T, bucket, key string, body []byte) {
	t.Helper()
	_, err := o.client().PutObject(context.Background(), &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader(body), ContentLength: aws.Int64(int64(len(body))), ContentType: aws.String("image/png")})
	if err != nil {
		t.Fatal(err)
	}
}

func m1006Scanner(t *testing.T, original []byte) sharedhttp.ScanOptions {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	t.Cleanup(func() { listener.Close(); workers.Wait() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				command, err := reader.ReadString(0)
				if err != nil {
					return
				}
				switch command {
				case "zVERSION\x00":
					io.WriteString(conn, "ClamAV 1.4.2/12345/fixture signatures\x00")
				case "zINSTREAM\x00":
					var body bytes.Buffer
					for {
						var length uint32
						if binary.Read(reader, binary.BigEndian, &length) != nil {
							return
						}
						if length == 0 {
							break
						}
						if length > 65536 {
							t.Error("scanner received unbounded frame")
							return
						}
						if _, err := io.CopyN(&body, reader, int64(length)); err != nil {
							return
						}
					}
					if !bytes.Equal(body.Bytes(), original) {
						t.Error("scanner did not receive the producer's original")
					}
					io.WriteString(conn, "stream: OK\x00")
				default:
					t.Errorf("unknown scanner command %q", command)
				}
			}()
		}
	}()
	addr := listener.Addr().(*net.TCPAddr)
	return sharedhttp.ScanOptions{Host: "127.0.0.1", Port: addr.Port, Timeout: time.Second}
}

// The chain ID comes from a real in-process geth node, while the factory
// receipt is a recorded producer transcript rather than custom LOG bytecode.
func m1006Chain(t *testing.T, id int64) (*gethrpc.Client, *atomic.Int64) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(id)
	backend := simulated.NewBackend(types.GenesisAlloc{}, func(n *node.Config, e *ethconfig.Config) {
		n.HTTPHost = "127.0.0.1"
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
		e.Genesis.Config = &chainConfig
		e.NetworkId = uint64(id)
	})
	t.Cleanup(func() { backend.Close() })
	actual, err := ethclient.Dial("http://127.0.0.1:" + strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(actual.Close)
	evidence, _ := m1006LoadEvidence(t)
	var receiptCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var result any
		switch call.Method {
		case "eth_chainId":
			chain, err := actual.ChainID(r.Context())
			if err != nil {
				t.Error(err)
				w.WriteHeader(503)
				return
			}
			result = fmt.Sprintf("0x%x", chain)
		case "eth_getTransactionReceipt":
			receiptCalls.Add(1)
			var hash string
			if len(call.Params) == 1 {
				_ = json.Unmarshal(call.Params[0], &hash)
			}
			if hash == evidence.Receipt.TxHash.Hex() {
				result = &evidence.Receipt
			}
		default:
			t.Errorf("unexpected RPC peer method %s", call.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result})
	}))
	t.Cleanup(server.Close)
	client, err := gethrpc.DialContext(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client, &receiptCalls
}

func m1006Media(t *testing.T, pool *pgxpool.Pool, objects *m1006Objects, original []byte, chain *gethrpc.Client) (metadata.ConnectedMedia, metadata.PublicationService) {
	t.Helper()
	backend := sharedauth.NewReportedHashBackend(chain)
	v2, err := metadata.NewV2ProductionHandlers(pool, m1006Schema, "metadata-development", backend, metadata.PostgresCanonicalSource{Schema: "indexer", CanonicalStreamID: "chain125-canonical"}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	media := metadata.ConnectedMedia{
		Metadata: v2, Identity: sharedauth.HTTPIdentityAuthority{Endpoint: "http://127.0.0.1", ServiceToken: strings.Repeat("i", 32), AllowPlainHTTP: true},
		Objects: metadata.S3AssetObjects{Client: objects.client(), QuarantineBucket: "quarantine-bucket", PublicBucket: "approved-bucket", PublicOrigin: "https://metadata.example"},
		Scanner: m1006Scanner(t, original), Worker: raster.InProcessWorker{}, Moderation: metadata.DevelopmentVisibility{},
	}
	publication, err := metadata.NewPublicationService(media, sharedauth.OperationAuthorizer{
		Ledger:    sharedauth.Ledger{Pool: pool, Environment: "development", AuditKey: bytes.Repeat([]byte{1}, 32)},
		Verifier:  sharedauth.WalletOperationVerifier{Backend: backend},
		Transport: sharedauth.OperationTransport{EnvelopeHeader: "X-Kindle-Operation", SignatureHeader: "X-Kindle-Operation-Signature"},
	}, sharedauth.PostgresIdentityResolver{Pool: pool, GuardFunction: "users.identity_guard_v2"})
	if err != nil {
		t.Fatal(err)
	}
	return media, publication
}

func m1006SeedAssets(t *testing.T, d *m1006Database, o *m1006Objects, original []byte) []string {
	t.Helper()
	ids := []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
		"44444444-4444-4444-8444-444444444444",
	}
	bodies := [][]byte{[]byte("corrupt object"), original[:33], append(bytes.Clone(original), 0), original}
	for i, id := range ids {
		body := bodies[i]
		a := m1006Asset{ID: id, Pool: d.pool, Hash: m1006Hash(body), Size: int64(len(body)), Key: "quarantine/" + id, State: "QUARANTINED", MIME: http.DetectContentType(body), CreatedAt: d.now.Add(time.Duration(i) * time.Second), Variants: json.RawMessage("{}")}
		if i == 0 {
			a.Hash = strings.Repeat("0", 64)
		}
		d.state.Assets[id] = a
		o.put(t, "quarantine-bucket", a.Key, body)
	}
	d.state.Assets[m1006Expired] = m1006Asset{ID: m1006Expired, Pool: d.pool, Hash: m1006Hash([]byte("expired")), Size: 7, Key: "quarantine/" + m1006Expired, State: "REJECTED", MIME: "image/png", Until: d.now.Add(-time.Hour), Variants: json.RawMessage("{}")}
	o.put(t, "quarantine-bucket", "quarantine/"+m1006Expired, []byte("expired"))
	o.put(t, "quarantine-bucket", "quarantine/account/"+m1006AccountAsset, []byte("expired account"))
	o.unavailable = "quarantine-bucket/quarantine/" + ids[2]
	d.save(d.state)
	return ids
}

func m1006AssertPublished(t *testing.T, d *m1006Database, media metadata.ConnectedMedia) {
	t.Helper()
	d.mu.Lock()
	state := d.state
	d.mu.Unlock()
	if state.PublicationState != "published" || state.Revision != "1" || state.Outbox != 1 || state.AccountState != "DELETED" || state.Assets[m1006Expired].State != "DELETED" {
		t.Fatalf("maintenance starved unrelated work: %+v", state)
	}
	router := chi.NewRouter()
	media.Metadata.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/v2/tokens/" + d.token + "/metadata")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || iface.ValidateJSON("MetadataGetResponse", raw) != nil {
		t.Fatalf("published HTTP consumer: %d %s", response.StatusCode, raw)
	}
	var got iface.MetadataGetResponse
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.DisplayName == nil || *got.Data.DisplayName != "Repaired publication" || string(got.Data.Revision) != "1" || got.Data.Token.SourcePosition == nil || got.Data.Token.SourcePosition.TransactionHash != d.txHash {
		t.Fatalf("accepted publication was not delivered: %+v", got.Data)
	}
}

func TestRepairM_1006_CorruptOldestRecovery(t *testing.T) {
	d := m1006NewDatabase(t)
	objects := m1006NewObjects(t)
	original := m1006PNG(t)
	ids := m1006SeedAssets(t, d, objects, original)
	chain, _ := m1006Chain(t, 125)
	pool, stop := d.open()
	t.Cleanup(stop)
	media, publication := m1006Media(t, pool, objects, original, chain)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := runMediaMaintenance(ctx, pool, m1006Schema, media, publication, true)
	if err == nil || !strings.Contains(err.Error(), "metadata_quarantine_unavailable") {
		stop()
		t.Fatalf("temporary object failure must remain observable: %v", err)
	}
	d.mu.Lock()
	first, decode, retry, healthy := d.state.Assets[ids[0]], d.state.Assets[ids[1]], d.state.Assets[ids[2]], d.state.Assets[ids[3]]
	d.mu.Unlock()
	if first.State != "REJECTED" || decode.State != "REJECTED" || first.Until.Sub(d.now) != 24*time.Hour || decode.Until.Sub(d.now) != 24*time.Hour {
		stop()
		t.Fatalf("terminal integrity/decode failures not durably rejected: %+v %+v", first, decode)
	}
	if retry.State != "VALIDATING" || retry.Until.Sub(d.now) != 30*time.Second || healthy.State != "APPROVED" || !healthy.Until.IsZero() {
		stop()
		t.Fatalf("retry/healthy upload: %+v %+v", retry, healthy)
	}
	var variants map[string]string
	if json.Unmarshal(healthy.Variants, &variants) != nil || len(variants) != 3 {
		stop()
		t.Fatalf("healthy variants: %s", healthy.Variants)
	}
	for _, key := range variants {
		objects.mu.Lock()
		body := bytes.Clone(objects.data["approved-bucket/"+key])
		objects.mu.Unlock()
		if !strings.HasSuffix(key, "/"+m1006Hash(body)+".png") || len(body) == 0 {
			stop()
			t.Fatal("promotion did not deliver hash-bound bytes")
		}
	}
	m1006AssertPublished(t, d, media)
	stop()

	// Destroy the process-local state and reopen the storage transport and
	// production component. Only the committed file can restore these rows.
	d.mu.Lock()
	d.state = m1006State{}
	d.mu.Unlock()
	pool, stop = d.open()
	defer stop()
	media, publication = m1006Media(t, pool, objects, original, chain)
	objects.mu.Lock()
	beforeBad, beforeRetry := objects.gets["quarantine-bucket/quarantine/"+ids[0]], objects.gets["quarantine-bucket/quarantine/"+ids[2]]
	objects.mu.Unlock()
	if err = runMediaMaintenance(ctx, pool, m1006Schema, media, publication, true); err != nil {
		t.Fatalf("restart before retry: %v", err)
	}
	objects.mu.Lock()
	afterBad, afterRetry := objects.gets["quarantine-bucket/quarantine/"+ids[0]], objects.gets["quarantine-bucket/quarantine/"+ids[2]]
	objects.mu.Unlock()
	if beforeBad != afterBad || beforeRetry != afterRetry {
		t.Fatal("restart forgot rejection or per-item backoff")
	}
	m1006AssertPublished(t, d, media)
	d.mu.Lock()
	d.now = d.now.Add(31 * time.Second)
	d.mu.Unlock()
	err = runMediaMaintenance(ctx, pool, m1006Schema, media, publication, false)
	if err == nil || !strings.Contains(err.Error(), "metadata_quarantine_unavailable") {
		t.Fatalf("due retry not attempted: %v", err)
	}
	d.mu.Lock()
	retry = d.state.Assets[ids[2]]
	d.mu.Unlock()
	objects.mu.Lock()
	afterRetry = objects.gets["quarantine-bucket/quarantine/"+ids[2]]
	objects.mu.Unlock()
	if retry.Until.Sub(d.now) != 30*time.Second || afterRetry != beforeRetry+1 {
		t.Fatalf("retry not bounded and durable: %+v attempts %d", retry, afterRetry)
	}
}

func TestRepairM_1006_IndependentWorkClasses(t *testing.T) {
	d := m1006NewDatabase(t)
	objects := m1006NewObjects(t)
	original := m1006PNG(t)
	ids := m1006SeedAssets(t, d, objects, original)
	objects.stalled = "quarantine-bucket/quarantine/" + ids[0]
	objects.unavailable = ""
	pool, stop := d.open()
	defer stop()
	chain, _ := m1006Chain(t, 125)
	media, publication := m1006Media(t, pool, objects, original, chain)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- runMediaMaintenance(ctx, pool, m1006Schema, media, publication, true) }()
	released := false
	defer func() {
		if !released {
			close(objects.release)
		}
	}()
	select {
	case <-objects.stallEntered:
	case <-ctx.Done():
		t.Fatal("scan did not enter the real object transport")
	}
	// Both consumers must finish while the oldest object's read is blocked.
	for _, done := range []<-chan struct{}{d.published, d.accountExpired} {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("stalled scan consumed another work class's budget")
		}
	}
	// The unrelated retry fixture is deliberately unavailable, so only the
	// oldest corrupt row was stalled; never process the alternate raster.
	objects.mu.Lock()
	objects.unavailable = "quarantine-bucket/quarantine/" + ids[2]
	objects.mu.Unlock()
	close(objects.release)
	released = true
	if err := <-result; err == nil || !strings.Contains(err.Error(), "metadata_quarantine_unavailable") {
		t.Fatalf("maintenance failure lost: %v", err)
	}
	m1006AssertPublished(t, d, media)
}

func TestRepairM_1006_QuarantinePrivacy(t *testing.T) {
	for _, mode := range []string{"private", "readable", "account-readable", "listing", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			objects := m1006NewObjects(t)
			objects.mode = mode
			cfg := config.Config{S3Endpoint: objects.server.URL, S3Region: "us-east-1", S3AccessKeyID: "fixture", S3SecretAccessKey: "fixture-secret", S3Bucket: "approved-bucket", QuarantineBucket: "quarantine-bucket", PublicURL: "https://metadata.example", IdentityServiceURL: "http://127.0.0.1", IdentityServiceToken: strings.Repeat("i", 32)}
			o, err := newDevelopmentObjects(context.Background(), cfg)
			if mode == "private" {
				if err != nil || o.Ready(context.Background()) != nil {
					t.Fatalf("private bucket refused: %v", err)
				}
				objects.mu.Lock()
				probes, remaining := objects.knownAnonymous, len(objects.data)
				objects.mu.Unlock()
				if probes != 4 || remaining != 0 {
					t.Fatalf("known-key probes or cleanup missing: probes %d remaining %d", probes, remaining)
				}
				for _, bucket := range []string{"", "approved-bucket"} {
					cfg.QuarantineBucket = bucket
					if _, err := newDevelopmentObjects(context.Background(), cfg); err == nil {
						t.Fatalf("unsafe quarantine configuration %q admitted", bucket)
					}
				}
				return
			}
			if err == nil {
				t.Fatalf("anonymous %s quarantine admitted", mode)
			}
			objects.mu.Lock()
			remaining := len(objects.data)
			objects.mu.Unlock()
			if remaining != 0 {
				t.Fatalf("privacy probes leaked %d objects", remaining)
			}
			if mode == "readable" || mode == "account-readable" {
				key := "quarantine/original"
				if mode == "account-readable" {
					key = "quarantine/account/original"
				}
				body := []byte("unscanned private original")
				objects.put(t, "quarantine-bucket", key, body)
				response, getErr := objects.server.Client().Get(objects.server.URL + "/quarantine-bucket/" + key)
				if getErr != nil {
					t.Fatal(getErr)
				}
				raw, _ := io.ReadAll(response.Body)
				response.Body.Close()
				if response.StatusCode != 200 || !bytes.Equal(raw, body) {
					t.Fatal("fixture did not actually permit anonymous original reads")
				}
				router := chi.NewRouter()
				(metadata.ConnectedMedia{Objects: metadata.S3AssetObjects{Client: objects.client(), QuarantineBucket: "quarantine-bucket", PublicBucket: "approved-bucket"}}).RegisterDevelopmentApprovedObjects(router)
				denied := httptest.NewRecorder()
				router.ServeHTTP(denied, httptest.NewRequest("GET", "/"+key, nil))
				if denied.Code != 404 {
					t.Fatalf("original unexpectedly routed: %d", denied.Code)
				}
				chain, _ := m1006Chain(t, 125)
				if media, startErr := newDevelopmentMedia(context.Background(), cfg, nil, nil, chain); startErr == nil || media != nil || startErr.Error() != "metadata_private_quarantine_unavailable" {
					t.Fatalf("public bucket did not refuse development startup before DB access: %v", startErr)
				}
			}
		})
	}
}

func TestRepairM_1006_FactoryReceiptChainBinding(t *testing.T) {
	d := m1006NewDatabase(t)
	pool, stop := d.open()
	defer stop()
	for _, chainID := range []int64{125, 1337} {
		t.Run(strconv.FormatInt(chainID, 10), func(t *testing.T) {
			chain, calls := m1006Chain(t, chainID)
			client := ethclient.NewClient(chain)
			admitted := requireMetadataChain(context.Background(), client)
			if chainID == 125 && admitted != nil {
				t.Fatalf("chain 125 refused: %v", admitted)
			}
			if chainID == 1337 && admitted == nil {
				t.Fatal("real chain 1337 endpoint admitted as chain 125")
			}
			reads := metadata.NewDevelopmentV2Reads(pool, developmentChainReceipts{client: client}, time.Now)
			router := chi.NewRouter()
			reads.RegisterRoutes(router)
			server := httptest.NewServer(router)
			defer server.Close()
			response, err := server.Client().Get(server.URL + "/v2/tokens/" + d.token + "/metadata")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if chainID == 1337 {
				if response.StatusCode != 503 || !bytes.Contains(raw, []byte("\"code\":\"unavailable\"")) || calls.Load() != 0 {
					t.Fatalf("chain mismatch relabelled receipt: %d calls %d %s", response.StatusCode, calls.Load(), raw)
				}
				return
			}
			if response.StatusCode != 200 || iface.ValidateJSON("MetadataGetResponse", raw) != nil || calls.Load() != 1 {
				t.Fatalf("tracked factory/indexer consumer: %d calls %d %s", response.StatusCode, calls.Load(), raw)
			}
			var result iface.MetadataGetResponse
			if err = json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			token := result.Data.Token
			if token.Address != d.token || token.Name != "LiveTest549002" || token.Symbol != "LT549002" || token.Decimals == nil || *token.Decimals != 6 || token.TotalSupplyRaw == nil || string(*token.TotalSupplyRaw) != "1000000000000000" || token.SourcePosition == nil || token.SourcePosition.ChainId != "125" || token.SourcePosition.TransactionHash != d.txHash || string(token.SourcePosition.LogIndex) != "11" {
				t.Fatalf("factory identity or source position lost: %+v", token)
			}
		})
	}
}
