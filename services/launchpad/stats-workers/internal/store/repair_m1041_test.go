package store_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/migrations"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"modernc.org/sqlite"
)

var repairM1041Register sync.Once
var repairM1041Casts = regexp.MustCompile(`(?i)::(?:text|int|bigint|numeric|varchar\(66\))`)

func repairM1041Translate(q string) string {
	q = strings.ReplaceAll(q, "stats.", "")
	q = strings.ReplaceAll(q, `"stats".`, "")
	q = strings.ReplaceAll(q, "balance::numeric", "raw_numeric(balance)")
	q = repairM1041Casts.ReplaceAllString(q, "")
	// pgx simple protocol encodes booleans as PostgreSQL's 't'/'f' literals.
	// SQLite BOOLEAN affinity needs integer boolean literals for CASE and equality.
	q = strings.ReplaceAll(q, "'t'", "TRUE")
	q = strings.ReplaceAll(q, "'f'", "FALSE")
	q = strings.ReplaceAll(q, " FOR UPDATE", "")
	return q
}

// repairM1041SQL exposes a real file-backed SQLite database over the PostgreSQL
// simple-query protocol. Production pgx, Store, projection and holder arithmetic
// run unchanged. Only schema qualification and lock syntax are translated:
// BEGIN IMMEDIATE supplies a database-wide writer lock instead of advisory locks.
// Savepoints, conflicts, reads, writes and crash recovery execute in SQLite.
// This fixture does not certify PostgreSQL-specific concurrency behaviour.
type repairM1041SQL struct {
	db          *sql.DB
	listener    net.Listener
	accepted    chan struct{}
	connections sync.WaitGroup
}

func repairM1041OpenSQL(t *testing.T, path string) *repairM1041SQL {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	s := &repairM1041SQL{db: db, listener: listener, accepted: make(chan struct{})}
	go func() {
		defer close(s.accepted)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.connections.Add(1)
			go func() { defer s.connections.Done(); s.serve(conn) }()
		}
	}()
	return s
}

func (s *repairM1041SQL) close() {
	_ = s.listener.Close()
	<-s.accepted
	s.connections.Wait()
	_ = s.db.Close()
}

func (s *repairM1041SQL) serve(conn net.Conn) {
	defer conn.Close()
	backend := pgproto3.NewBackend(conn, conn)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	for key, value := range map[string]string{"client_encoding": "UTF8", "server_encoding": "UTF8", "standard_conforming_strings": "on", "server_version": "16.0"} {
		backend.Send(&pgproto3.ParameterStatus{Name: key, Value: value})
	}
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := backend.Flush(); err != nil {
		return
	}
	ctx := context.Background()
	dbconn, err := s.db.Conn(ctx)
	if err != nil {
		return
	}
	defer dbconn.Close()
	defer func() { _, _ = dbconn.ExecContext(ctx, "ROLLBACK") }()
	status := byte('I')
	for {
		message, err := backend.Receive()
		if err != nil {
			return
		}
		switch message := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			query := strings.TrimSpace(message.String)
			normalized := strings.ToUpper(strings.Join(strings.Fields(query), " "))
			translated := repairM1041Translate(query)
			if normalized == "BEGIN" {
				translated = "BEGIN IMMEDIATE"
			}
			if strings.HasPrefix(normalized, "SELECT PG_ADVISORY_XACT_LOCK(") {
				translated = "SELECT 1"
			}
			tag := ""
			if status == 'E' && !strings.HasPrefix(normalized, "ROLLBACK") {
				err = fmt.Errorf("current transaction is aborted")
			} else if strings.HasPrefix(strings.ToUpper(translated), "SELECT") {
				err = repairM1041Rows(ctx, backend, dbconn, translated)
			} else {
				var result sql.Result
				result, err = dbconn.ExecContext(ctx, translated)
				if err == nil {
					affected, countErr := result.RowsAffected()
					if countErr != nil {
						return
					}
					verb := strings.Fields(normalized)[0]
					switch verb {
					case "INSERT":
						tag = fmt.Sprintf("INSERT 0 %d", affected)
					case "UPDATE", "DELETE":
						tag = fmt.Sprintf("%s %d", verb, affected)
					default:
						tag = verb
					}
				}
			}
			if err != nil {
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: err.Error()})
				if status == 'T' {
					status = 'E'
				}
			} else {
				if normalized == "BEGIN" {
					status = 'T'
				}
				if normalized == "COMMIT" || normalized == "ROLLBACK" {
					status = 'I'
				}
				if strings.HasPrefix(normalized, "ROLLBACK TO") {
					status = 'T'
				}

				if tag != "" {
					backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
				}
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if err := backend.Flush(); err != nil {
				return
			}
		default:
			return // the client explicitly selects simple-query mode
		}
	}
}

func repairM1041Rows(ctx context.Context, backend *pgproto3.Backend, conn *sql.Conn, query string) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	fields := make([]pgproto3.FieldDescription, len(columns))
	for i, column := range columns {
		oid := uint32(pgtype.TextOID)
		typeName := strings.ToUpper(column.DatabaseTypeName())
		if strings.Contains(typeName, "INT") {
			oid = pgtype.Int8OID
		}
		if typeName == "BOOLEAN" {
			oid = pgtype.BoolOID
		}
		fields[i] = pgproto3.FieldDescription{Name: []byte(column.Name()), DataTypeOID: oid, DataTypeSize: -1}
	}
	var data [][][]byte
	for rows.Next() {
		values := make([]any, len(fields))
		dest := make([]any, len(fields))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		wire := make([][]byte, len(fields))
		for i, value := range values {
			if value == nil {
				continue
			}
			switch value := value.(type) {
			case int64:
				if fields[i].DataTypeOID == pgtype.BoolOID {
					switch value {
					case 0:
						wire[i] = []byte("f")
					case 1:
						wire[i] = []byte("t")
					default:
						return fmt.Errorf("invalid SQL boolean value: %d", value)
					}
				} else {
					fields[i].DataTypeOID = pgtype.Int8OID
					wire[i] = []byte(strconv.FormatInt(value, 10))
				}
			case bool:
				fields[i].DataTypeOID = pgtype.BoolOID
				if value {
					wire[i] = []byte("t")
				} else {
					wire[i] = []byte("f")
				}
			case []byte:
				wire[i] = value
			default:
				wire[i] = []byte(fmt.Sprint(value))
			}
		}
		data = append(data, wire)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	backend.Send(&pgproto3.RowDescription{Fields: fields})
	for _, row := range data {
		backend.Send(&pgproto3.DataRow{Values: row})
	}
	backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(fmt.Sprintf("SELECT %d", len(data)))})
	return nil
}

// SQLite's NUMERIC affinity is not arbitrary precision. This deterministic SQL
// key translates PostgreSQL's exact numeric balance cast without using float.
// Every tested ORDER BY and expression index still executes in the SQL engine.
func repairM1041RegisterNumeric() {
	repairM1041Register.Do(func() {
		sqlite.MustRegisterDeterministicScalarFunction("raw_numeric", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			raw, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("numeric text required")
			}
			n, err := accounting.Uint(raw)
			if err != nil {
				return nil, err
			}
			value := n.String()
			return strings.Repeat("0", 78-len(value)) + value, nil
		})
	})
}

type repairM1041Fixture struct {
	path  string
	sql   *repairM1041SQL
	pool  *pgxpool.Pool
	store *store.Store
	redis *goredis.Client
}

const repairM1041Version = "stats-market-v1"

func newRepairM1041Fixture(t *testing.T) *repairM1041Fixture {
	t.Helper()
	repairM1041RegisterNumeric()
	f := &repairM1041Fixture{path: filepath.Join(t.TempDir(), "stats.sqlite")}
	f.open(t)
	t.Cleanup(f.close)
	ddl, err := migrations.FS.ReadFile("00001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.Split(string(ddl), "-- +goose Down")[0], "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "CREATE SCHEMA") {
			continue
		}
		statement = strings.ReplaceAll(repairM1041Translate(statement), " USING btree", "")
		if _, err = f.sql.db.Exec(statement); err != nil {
			t.Fatalf("registered migration: %v: %s", err, statement)
		}
	}
	// The canonical peer holds actual codec output, in the production commit shape.
	f.exec(t, `CREATE TABLE accounting_market_commits(publication_id TEXT PRIMARY KEY,source_fill_id TEXT NOT NULL,source_fact_digest TEXT NOT NULL,projection_version TEXT NOT NULL,pool TEXT NOT NULL,fill TEXT NOT NULL,UNIQUE(source_fill_id,projection_version))`)
	if err = f.store.EnsureHolderBalanceIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *repairM1041Fixture) open(t *testing.T) {
	t.Helper()
	f.sql = repairM1041OpenSQL(t, f.path)
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + f.sql.listener.Addr().String() + "/stats?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	cfg.MaxConns = 4
	f.pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	f.store = store.New(f.pool)
	if err = f.store.SetAccountingProjection(repairM1041Version); err != nil {
		t.Fatal(err)
	}
	f.redis = goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	if err = f.redis.Close(); err != nil {
		t.Fatal(err)
	}
}
func (f *repairM1041Fixture) close() {
	f.pool.Close()
	f.sql.close()
}
func (f *repairM1041Fixture) restart(t *testing.T) {
	t.Helper()
	f.close()
	f.open(t)
}
func (f *repairM1041Fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := f.sql.db.Exec(q, args...); err != nil {
		t.Fatalf("SQL fixture: %v: %s", err, q)
	}
}
func (f *repairM1041Fixture) seedPool(t *testing.T, pool, token string, at int64) {
	t.Helper()
	_, err := f.store.InsertInitialPoolStats(context.Background(), store.InitialPoolStats{PoolAddress: pool, TokenAddress: token, Price: "1000", MarketCap: "1000", High24h: "1000", Low24h: "1000", CreatedAt: at, UpdatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
}

// Decode the committed chain-125 receipts through the canonical fill codec and
// the live bridge's ABI binding before the actual SwapConsumer writes Store.
type repairM1041Receipt struct {
	Fill    accounting.FillV1
	Emitter string
	Topics  []string
	Data    []byte
}

func repairM1041Receipts(t *testing.T) []repairM1041Receipt {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "projectionstats", "testdata", "real_pool_125.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tape struct {
		Pool, Token, USDL, EventEmitter string
		TokenDecimals                   uint8
		Blocks                          map[string]struct{ Timestamp int64 }
		Transactions                    []struct {
			Hash, From, BlockHash         string
			BlockNumber, TransactionIndex int64
			Logs                          []struct {
				LogIndex      int64
				Address, Data string
				Topics        []string
			}
		}
	}
	if err = json.Unmarshal(raw, &tape); err != nil {
		t.Fatal(err)
	}
	out := []repairM1041Receipt{}
	for _, tx := range tape.Transactions {
		for _, log := range tx.Logs {
			if log.Address != tape.EventEmitter || len(log.Topics) != 3 || log.Topics[0] != "0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc" {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			word := func(i int) string { return new(big.Int).SetBytes(data[32*i : 32*(i+1)]).String() }
			fill := accounting.FillV1{
				Version:       accounting.FillVersion,
				Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(tx.BlockNumber, 10), TransactionIndex: strconv.FormatInt(tx.TransactionIndex, 10), LogIndex: strconv.FormatInt(log.LogIndex, 10), TransactionHash: tx.Hash, BlockHash: tx.BlockHash},
				ActualEmitter: log.Address, PoolAddress: tape.Pool, Token: accounting.Asset{Address: tape.Token, Decimals: tape.TokenDecimals}, Settlement: accounting.Asset{Address: tape.USDL, Decimals: 6},
				PaidInputRaw: word(1), FeeRaw: word(3), InputDerivationQuality: "receipt_incomplete",
				Attribution:           accounting.Attribution{TransactionSender: tx.From, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
				Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
				Provenance:            accounting.Provenance{DeploymentID: "paxeer125-a35ff7c6f532d2f9", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "event-emitter-1", PoolCodeEpoch: "pool-1", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)},
				BlockTimestampSeconds: strconv.FormatInt(tape.Blocks[strconv.FormatInt(tx.BlockNumber, 10)].Timestamp, 10),
			}
			if word(0) == "1" {
				fill.Side, fill.TokenAmountRaw, fill.USDLAmountRaw, fill.FeeDisposition = "buy", word(2), word(1), "distributed_usdl"
			} else {
				fill.Side, fill.TokenAmountRaw, fill.USDLAmountRaw, fill.FeeDisposition = "sell", word(1), word(2), "retained_token_reserve"
			}
			fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
			out = append(out, repairM1041Receipt{fill, log.Address, log.Topics, data})
		}
	}
	if len(out) != 2 || out[0].Fill.Side != "buy" || out[1].Fill.Side != "sell" {
		t.Fatalf("recorded buy/sell missing: %+v", out)
	}
	return out
}

// The producer emits a validated canonical publication; the SQL peer stores its
// immutable fill and digest. The bridge then supplies the real swap projection.
func (f *repairM1041Fixture) project(t *testing.T, r repairM1041Receipt) accounting.FillV1 {
	t.Helper()
	fill := r.Fill
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	partition := accounting.PartitionPosition{ProjectionVersion: repairM1041Version, PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, Sequence: "1", SourceEventID: fill.SourceFillID, PayloadDigest: digest, SourceOrder: fill.Position}
	partition.EffectIdentity = accounting.EffectIdentity(fill.SourceFillID, "market", repairM1041Version, "", "market", partition.PartitionKey)
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{partition})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := accounting.DecodePublicationV1(wire)
	if err != nil {
		t.Fatal(err)
	}
	fill = *decoded.Fill
	raw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,fill) VALUES(?,?,?,?,?,?)`, decoded.PublicationID, fill.SourceFillID, string(decoded.SourceFactDigest), repairM1041Version, fill.PoolAddress, string(raw))
	event, err := consumer.LegacySwapFromCanonical(fill, r.Emitter, r.Topics, r.Data)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	swap := consumer.NewSwapConsumer(f.store, f.redis, logger)
	// No seeded pool is needed for transaction/bucket writes. Tests seed one to
	// exercise pool and route reads, and assert the explicit post-commit cache error.
	err = swap.ProcessEvent(context.Background(), event)
	if err != nil && !strings.Contains(err.Error(), "consumer: cache pool stats: redis: client is closed") {
		t.Fatal(err)
	}
	return fill
}
func (f *repairM1041Fixture) get(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	router := chi.NewRouter()
	httpapi.RegisterPlatform(router, f.store, f.redis)
	httpapi.RegisterPoolStats(router, f.store, f.redis)
	httpapi.RegisterPressure(router, f.store, f.redis)
	httpapi.RegisterPoolHolders(router, f.store)
	server := httptest.NewServer(router)
	defer server.Close()
	response, err := server.Client().Get(server.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d: %s", path, response.StatusCode, body)
	}
	var payload map[string]json.RawMessage
	if err = json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
func repairM1041At(r repairM1041Receipt, at int64) repairM1041Receipt {
	r.Data = append([]byte(nil), r.Data...)
	r.Topics = append([]string(nil), r.Topics...)
	r.Fill.BlockTimestampSeconds = strconv.FormatInt(at, 10)
	big.NewInt(at).FillBytes(r.Data[5*32 : 6*32])
	return r
}

func repairM1041DifferentScale(r repairM1041Receipt) repairM1041Receipt {
	r = repairM1041At(r, time.Now().Unix()-10)
	r.Fill.PoolAddress = "0x" + strings.Repeat("7", 40)
	r.Fill.Token = accounting.Asset{Address: "0x" + strings.Repeat("8", 40), Decimals: 18}
	r.Fill.Attribution.TransactionSender = "0x" + strings.Repeat("6", 40)
	r.Topics[2] = "0x" + strings.Repeat("0", 24) + strings.Repeat("6", 40)
	r.Fill.Position.TransactionHash = "0x" + strings.Repeat("9", 64)
	r.Fill.SourceFillID = accounting.SourceFillIdentity(r.Fill.Position, r.Fill.Provenance.DeploymentID)
	r.Fill.TokenAmountRaw = "1000000000000000000"
	r.Fill.PaidInputRaw = r.Fill.TokenAmountRaw
	r.Fill.USDLAmountRaw = "70000000"
	r.Fill.FeeRaw = "3000000000000000"
	for index, raw := range map[int]string{1: r.Fill.PaidInputRaw, 2: r.Fill.USDLAmountRaw, 3: r.Fill.FeeRaw} {
		value, _ := new(big.Int).SetString(raw, 10)
		value.FillBytes(r.Data[index*32 : (index+1)*32])
	}
	r.Topics[1] = "0x" + strings.Repeat("0", 24) + r.Fill.PoolAddress[2:]
	return r
}

func repairM1041FeeContract(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	f := newRepairM1041Fixture(t)
	now := time.Now().Unix()
	receipts := repairM1041Receipts(t)
	buy := repairM1041At(receipts[0], now-30)
	sell := repairM1041At(receipts[1], now-20)
	other := repairM1041DifferentScale(sell)
	f.seedPool(t, buy.Fill.PoolAddress, buy.Fill.Token.Address, now-60)
	f.seedPool(t, other.Fill.PoolAddress, other.Fill.Token.Address, now-90000)
	f.project(t, buy)
	f.project(t, sell)
	f.project(t, other)
	if err := f.store.InsertCrossTokenSwap(ctx, crossSwap("current-cross", "0xw", "0xa", "0xb", now-100)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.InsertCrossTokenSwap(ctx, crossSwap("old-cross", "0xw", "0xa", "0xb", now-90000)); err != nil {
		t.Fatal(err)
	}
	metrics, err := f.store.PlatformMetrics(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []store.PlatformAssetFee{
		{Asset: sell.Fill.Token.Address, Decimals: 6, Disposition: "retained_token_reserve", Raw: "144102208576"},
		{Asset: other.Fill.Token.Address, Decimals: 18, Disposition: "retained_token_reserve", Raw: "3000000000000000"},
		{Asset: buy.Fill.Settlement.Address, Decimals: 6, Disposition: "distributed_usdl", Raw: "3000000"},
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Asset < want[j].Asset })
	if !reflect.DeepEqual(metrics.TotalFees24h, want) || metrics.FeeSchema != "asset-fees-v1" {
		t.Fatalf("canonical asset fees=%+v schema=%s, want %+v", metrics.TotalFees24h, metrics.FeeSchema, want)
	}
	wantCap, err := shareddb.ComputeMarketCap(new(big.Int).SetBytes(sell.Data[4*32 : 5*32]).String())
	if err != nil {
		t.Fatal(err)
	}
	wantCap, err = shareddb.BigintAdd(wantCap, wantCap)
	if err != nil || metrics.TotalMarketCap != wantCap {
		t.Fatalf("market cap=%s want=%s err=%v", metrics.TotalMarketCap, wantCap, err)
	}
	if metrics.TotalVolume24h != "217278912" || metrics.TotalVolume1h != "217278912" || metrics.TotalTransactions24h != 3 || metrics.TotalTransactions1h != 3 || metrics.TotalBuys24h != 1 || metrics.TotalSells24h != 2 || metrics.UniqueTraders24h != 2 || metrics.TotalTokensLaunched != 2 || metrics.NewTokens24h != 1 || metrics.CrossTokenSwaps24h != 1 || metrics.UpdatedAt != now {
		t.Fatalf("canonical platform totals: %+v", metrics)
	}
	payload := f.get(t, "/stats/platform")
	var fees []store.PlatformAssetFee
	if err = json.Unmarshal(payload["totalFees24h"], &fees); err != nil {
		t.Fatalf("fees must be asset-separated JSON: %v", err)
	}
	if !reflect.DeepEqual(fees, want) || string(payload["feeSchema"]) != `"asset-fees-v1"` || string(payload["totalVolume24h"]) != `"217278912"` {
		t.Fatalf("HTTP fee contract: %s", payload)
	}
	// A duplicate accounting projection cannot double the fee total.
	f.exec(t, `INSERT INTO accounting_market_commits SELECT publication_id||'-v2',source_fill_id,source_fact_digest,'another-version',pool,fill FROM accounting_market_commits`)
	again, err := f.store.PlatformMetrics(ctx, now)
	if err != nil || !reflect.DeepEqual(again.TotalFees24h, want) {
		t.Fatalf("projection-qualified totals: %+v %v", again, err)
	}
	f.restart(t)
	recovered, err := f.store.PlatformMetrics(ctx, now)
	if err != nil || !reflect.DeepEqual(recovered.TotalFees24h, want) {
		t.Fatalf("recovered fees: %+v %v", recovered, err)
	}
}

func TestRepairM_1041_PlatformFeeUnits(t *testing.T) { repairM1041FeeContract(t) }

func TestRepairM_1041_FeeFixtureWrongUnits(t *testing.T) {
	repairM1041FeeContract(t)
	for _, mutation := range []string{"missing_commit", "missing_conversion_evidence", "invalid_scale", "digest_mismatch", "legacy_mismatch"} {
		t.Run(mutation, func(t *testing.T) {
			f := newRepairM1041Fixture(t)
			now := time.Now().Unix()
			receipt := repairM1041At(repairM1041Receipts(t)[0], now-10)
			f.seedPool(t, receipt.Fill.PoolAddress, receipt.Fill.Token.Address, now-20)
			fill := f.project(t, receipt)
			switch mutation {
			case "missing_commit":
				f.exec(t, `DELETE FROM accounting_market_commits`)
			case "digest_mismatch":
				f.exec(t, `UPDATE accounting_market_commits SET source_fact_digest=?`, strings.Repeat("0", 64))
			case "legacy_mismatch":
				f.exec(t, `UPDATE pool_transactions SET fee='1'`)
			default:
				if mutation == "missing_conversion_evidence" {
					// Marking a quote exact without priced/net/execution evidence is invalid;
					// the legacy fee cannot be accepted as an implicit converted USDL value.
					fill.InputDerivationQuality = "exact"
				} else {
					fill.Settlement.Decimals = 18
				}
				raw, err := json.Marshal(fill)
				if err != nil {
					t.Fatal(err)
				}
				f.exec(t, `UPDATE accounting_market_commits SET fill=?`, string(raw))
			}
			if _, err := f.store.PlatformMetrics(context.Background(), now); err == nil {
				t.Fatal("incomplete or mismatched canonical evidence accepted")
			}
			router := chi.NewRouter()
			httpapi.RegisterPlatform(router, f.store, f.redis)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest("GET", "/stats/platform", nil))
			if recorder.Code != 500 {
				t.Fatalf("incomplete evidence HTTP status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func repairM1041PressureContract(t *testing.T) {
	t.Helper()
	f := newRepairM1041Fixture(t)
	now := time.Now().Unix()
	receipts := repairM1041Receipts(t)
	buy := repairM1041At(receipts[0], now-90)
	sell := repairM1041At(receipts[1], now-80)
	f.seedPool(t, buy.Fill.PoolAddress, buy.Fill.Token.Address, now-100)
	f.project(t, buy)
	f.project(t, sell)
	// Full canonical buys outside 1h, and future fills, must not enter the window.
	old := repairM1041At(buy, now-7200)
	old.Fill.Position.TransactionHash = "0x" + strings.Repeat("c", 64)
	old.Fill.SourceFillID = accounting.SourceFillIdentity(old.Fill.Position, old.Fill.Provenance.DeploymentID)
	f.project(t, old)
	future := repairM1041At(buy, now+7200)
	future.Fill.Position.TransactionHash = "0x" + strings.Repeat("d", 64)
	future.Fill.SourceFillID = accounting.SourceFillIdentity(future.Fill.Position, future.Fill.Provenance.DeploymentID)
	// Record the future transaction without advancing the current pool projection.
	ev, err := consumer.LegacySwapFromCanonical(future.Fill, future.Emitter, future.Topics, future.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.InsertTransaction(context.Background(), store.TransactionRow{ID: ev.TxHash + "-" + strconv.Itoa(ev.LogIndex), PoolAddress: ev.PoolAddress, Sender: ev.Sender, IsBuy: ev.IsBuy, AmountIn: ev.AmountIn, AmountOut: ev.AmountOut, Price: ev.Price, Fee: ev.Fee, BlockTimestamp: ev.BlockTimestamp, TxHash: ev.TxHash}); err != nil {
		t.Fatal(err)
	}
	ps, found, err := f.store.PressureStats(context.Background(), buy.Fill.PoolAddress, now-3600)
	if err != nil || !found {
		t.Fatalf("pressure found=%v err=%v", found, err)
	}
	if ps.BuyVolume1h != "100000000" || ps.SellVolume1h != "47278912" || ps.BuyCount1h != 1 || ps.SellCount1h != 1 || ps.BuyCount24h != 2 || ps.SellCount24h != 1 || ps.Volume24h != "247278912" || ps.Volume1h != "147278912" {
		t.Fatalf("canonical settlement pressure: %+v", ps)
	}
	if ps.SellVolume1h == sell.Fill.TokenAmountRaw || sell.Fill.TokenAmountRaw == sell.Fill.USDLAmountRaw {
		t.Fatal("sell fixture failed to distinguish the token and USDL legs")
	}
	payload := f.get(t, "/stats/"+buy.Fill.PoolAddress+"/pressure")
	var hour struct {
		BuyVolume, SellVolume string
		BuyCount, SellCount   int
		BuyPct, SellPct       float64
		Direction             string
	}
	if err = json.Unmarshal(payload["1h"], &hour); err != nil {
		t.Fatal(err)
	}
	if hour.BuyVolume != ps.BuyVolume1h || hour.SellVolume != ps.SellVolume1h || hour.BuyCount != 1 || hour.SellCount != 1 || hour.BuyPct != 50 || hour.SellPct != 50 || hour.Direction != "neutral" {
		t.Fatalf("HTTP/store pressure disagree: %+v vs %+v", hour, ps)
	}
	if _, found, err = f.store.PressureStats(context.Background(), "0xunknown", now-3600); err != nil || found {
		t.Fatalf("unknown pool found=%v err=%v", found, err)
	}
}
func TestRepairM_1041_PressureSellUnits(t *testing.T)       { repairM1041PressureContract(t) }
func TestRepairM_1041_PressureFixtureWrongLeg(t *testing.T) { repairM1041PressureContract(t) }
func TestRepairM_1041_LegacyIdleWindows(t *testing.T) {
	ctx := context.Background()
	f := newRepairM1041Fixture(t)
	// No wall-clock sleeps: the scheduler's explicit clock crosses every cutoff.
	last := (time.Now().Unix()/60)*60 - 60
	receipts := repairM1041Receipts(t)
	buy := repairM1041At(receipts[0], last)
	sell := repairM1041At(receipts[1], last+1)
	pool := buy.Fill.PoolAddress
	f.seedPool(t, pool, buy.Fill.Token.Address, last-100)
	f.seedPool(t, "0xz-idle", "0xz-token", last-100)
	f.project(t, buy)
	f.project(t, sell)
	// A pre-swap extreme must expire too, rather than retaining an all-time high.
	if err := f.store.UpsertPriceSnapshot(ctx, pool, last-60, "90000000000000"); err != nil {
		t.Fatal(err)
	}
	// Simulate the stale compatibility projection this job must repair.
	f.exec(t, `UPDATE pool_stats SET price_change_1m='5',price_change_5m='5',price_change_15m='5',price_change_1h='5',price_change_24h='5',price_change_dollar_1m='7',price_change_dollar_5m='7',price_change_dollar_15m='7',price_change_dollar_1h='7',price_change_dollar_24h='7' WHERE pool_address=?`, pool)
	for _, tc := range []struct {
		offset      int64
		v5, v1, v24 string
		buys, sells int
	}{
		{299, "147278912", "147278912", "147278912", 1, 1},
		{300, "147278912", "147278912", "147278912", 1, 1},
		{301, "47278912", "147278912", "147278912", 1, 1},
		{302, "0", "147278912", "147278912", 1, 1},
		{3601, "0", "47278912", "147278912", 1, 1},
		{3602, "0", "0", "147278912", 1, 1},
		{86401, "0", "0", "47278912", 0, 1},
		{86581, "0", "0", "0", 0, 0},
	} {
		at := last + tc.offset
		next, more, err := f.store.RefreshIdleWindows(ctx, at, "", 1)
		if err != nil || !more || next != pool {
			t.Fatalf("bounded first batch next=%s more=%v err=%v", next, more, err)
		}
		end, more, err := f.store.RefreshIdleWindows(ctx, at, next, 1)
		if err != nil || !more || end != "0xz-idle" {
			t.Fatalf("bounded second batch next=%s more=%v err=%v", end, more, err)
		}
		end, more, err = f.store.RefreshIdleWindows(ctx, at, end, 1)
		if err != nil || more || end != "" {
			t.Fatalf("sweep completion next=%s more=%v err=%v", end, more, err)
		}
		row, err := f.store.GetPoolStats(ctx, pool)
		if err != nil || row == nil {
			t.Fatalf("idle row: %+v %v", row, err)
		}
		if row.Volume5m != tc.v5 || row.Volume1h != tc.v1 || row.Volume24h != tc.v24 || row.BuyCount24h != tc.buys || row.SellCount24h != tc.sells || row.UpdatedAt != at {
			t.Fatalf("idle offset %d: %+v", tc.offset, row)
		}
		wantTraders := 1
		if tc.buys+tc.sells == 0 {
			wantTraders = 0
		}
		if row.UniqueTraders24h != wantTraders {
			t.Fatalf("idle traders=%d want %d", row.UniqueTraders24h, wantTraders)
		}
		metrics, err := f.store.PlatformMetrics(ctx, at)
		if err != nil || metrics.TotalVolume1h != tc.v1 || metrics.TotalVolume24h != tc.v24 || metrics.TotalBuys24h != tc.buys || metrics.TotalSells24h != tc.sells || metrics.UniqueTraders24h != wantTraders {
			t.Fatalf("idle platform offset=%d: %+v %v", tc.offset, metrics, err)
		}
		if tc.offset == 86581 {
			if row.High24h != row.Price || row.Low24h != row.Price || row.PriceChange1m != "0" || row.PriceChange5m != "0" || row.PriceChange15m != "0" || row.PriceChange1h != "0" || row.PriceChange24h != "0" || row.PriceChangeDollar1m != "0" || row.PriceChangeDollar5m != "0" || row.PriceChangeDollar15m != "0" || row.PriceChangeDollar1h != "0" || row.PriceChangeDollar24h != "0" || len(metrics.TotalFees24h) != 0 {
				t.Fatalf("expired price/fee windows: %+v fees=%+v", row, metrics.TotalFees24h)
			}
		}
		f.restart(t)
		recovered, err := f.store.GetPoolStats(ctx, pool)
		if err != nil || !reflect.DeepEqual(recovered, row) {
			t.Fatalf("recovered idle state=%+v want %+v err=%v", recovered, row, err)
		}
	}
	// Timestamp-bounded platform sums cannot inherit deliberately stale pool sums.
	f.exec(t, `UPDATE pool_stats SET volume_24h='999',volume_1h='999',buy_count_24h=99,sell_count_24h=99`)
	metrics, err := f.store.PlatformMetrics(ctx, last+90000)
	if err != nil || metrics.TotalVolume24h != "0" || metrics.TotalVolume1h != "0" || metrics.TotalTransactions24h != 0 || metrics.TotalBuys24h != 0 || metrics.TotalSells24h != 0 {
		t.Fatalf("stale pool values leaked into fresh platform timestamp: %+v %v", metrics, err)
	}
	cursor := ""
	more, err := app.RefreshLegacyWindows(ctx, f.store, last+90000, &cursor)
	if err != nil || more || cursor != "" {
		t.Fatalf("live application idle pass: more=%v cursor=%s err=%v", more, cursor, err)
	}
	payload := f.get(t, "/stats/"+pool)
	if string(payload["volume5m"]) != `"0"` || string(payload["volume1h"]) != `"0"` || string(payload["volume24h"]) != `"0"` || string(payload["updatedAt"]) != strconv.FormatInt(last+90000, 10) {
		t.Fatalf("application/store/HTTP idle path: %s", payload)
	}
	if _, _, err = f.store.RefreshIdleWindows(ctx, last, "", 101); err == nil {
		t.Fatal("unbounded idle batch accepted")
	}
	if _, _, err = f.store.RefreshIdleWindows(ctx, last, "", 1); err == nil {
		t.Fatal("regressing idle clock accepted")
	}
}

func TestRepairM_1041_HolderTextOrder(t *testing.T) {
	ctx := context.Background()
	f := newRepairM1041Fixture(t)
	const pool = "0xholders"
	now := time.Now().Unix()
	f.seedPool(t, pool, "0xtoken", now-1000000)
	balances := []string{"9", "100", "1000", "0", "1000", "100000000000000", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9"}
	canonical := make([]store.HolderRow, 0, len(balances))
	for i, balance := range balances {
		holder := fmt.Sprintf("0x%040x", len(balances)-i)
		pct, err := shareddb.BigintMulDiv(balance, "10000", shareddb.TotalSupplyRaw)
		if err != nil {
			t.Fatal(err)
		}
		if balance == "0" {
			f.exec(t, `INSERT INTO pool_holders(pool_address,holder_address,balance,pct_of_supply,last_updated) VALUES(?,?,?,?,?)`, pool, holder, balance, pct, now)
		} else if applied, err := f.store.ApplyHolderDelta(ctx, pool, holder, true, "50000000", balance, now); err != nil || !applied {
			t.Fatalf("holder producer: applied=%v err=%v", applied, err)
		}
		canonical = append(canonical, store.HolderRow{PoolAddress: pool, HolderAddress: holder, Balance: balance, PctOfSupply: pct, LastUpdated: now})
	}
	sort.Slice(canonical, func(i, j int) bool {
		a, _ := new(big.Int).SetString(canonical[i].Balance, 10)
		b, _ := new(big.Int).SetString(canonical[j].Balance, 10)
		if compare := a.Cmp(b); compare != 0 {
			return compare > 0
		}
		return canonical[i].HolderAddress < canonical[j].HolderAddress
	})
	all, err := f.store.ListHoldersByBalance(ctx, pool)
	if err != nil || !reflect.DeepEqual(all, canonical) {
		t.Fatalf("numeric ranks=%+v want=%+v err=%v", all, canonical, err)
	}
	paged := []store.HolderRow{}
	for offset := 0; offset < len(canonical); offset += 3 {
		page, err := f.store.ListHolders(ctx, pool, 3, offset)
		if err != nil {
			t.Fatal(err)
		}
		paged = append(paged, page...)
		payload := f.get(t, fmt.Sprintf("/stats/%s/holders?limit=3&offset=%d", pool, offset))
		var holders []struct {
			store.HolderRow
			Rank int
		}
		if err = json.Unmarshal(payload["holders"], &holders); err != nil {
			t.Fatal(err)
		}
		if len(holders) != len(page) || string(payload["total"]) != "16" {
			t.Fatalf("holder page: %s", payload)
		}
		for i, holder := range holders {
			if holder.Rank != offset+i+1 || !reflect.DeepEqual(holder.HolderRow, canonical[offset+i]) {
				t.Fatalf("route rank=%+v want=%+v", holder, canonical[offset+i])
			}
		}
	}
	if !reflect.DeepEqual(paged, canonical) {
		t.Fatalf("unstable tie-break across pages: %+v", paged)
	}
	if err = f.store.RefreshPoolHolderStats(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	row, err := f.store.GetPoolStats(ctx, pool)
	if err != nil || row == nil {
		t.Fatal(err)
	}
	total := new(big.Int)
	for _, holder := range canonical[:10] {
		value, _ := new(big.Int).SetString(holder.Balance, 10)
		total.Add(total, value)
	}
	total.Mul(total, big.NewInt(10000))
	supply, _ := new(big.Int).SetString(shareddb.TotalSupplyRaw, 10)
	total.Quo(total, supply)
	if row.Top10Concentration != total.String() || row.Top10Concentration != "1000" || row.HolderCount != len(canonical) {
		t.Fatalf("canonical concentration: %+v want=%s", row, total)
	}
	// Verify that the actual expression index serves exact balance/address order.
	plan, err := f.sql.db.Query(`EXPLAIN QUERY PLAN SELECT balance FROM pool_holders WHERE pool_address=? ORDER BY raw_numeric(balance) DESC,holder_address ASC LIMIT 10`, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	indexed := false
	for plan.Next() {
		var id, parent, unused int
		var detail string
		if err = plan.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "idx_ph_balance_numeric") {
			indexed = true
		}
		if strings.Contains(detail, "TEMP B-TREE") {
			t.Fatalf("holder order sorted outside its index: %s", detail)
		}
	}
	if err = plan.Err(); err != nil {
		t.Fatal(err)
	}
	plan.Close()
	if !indexed {
		t.Fatal("numeric holder order did not use its expression index")
	}
	// Adjacent balances beyond float64 and uint64 precision retain their order.
	for i, balance := range []string{"100000000000000000000000000001", "100000000000000000000000000000"} {
		f.exec(t, `INSERT INTO pool_holders VALUES(?,?,?,?,?)`, "0xlarge", fmt.Sprintf("0x%040x", i+1), balance, "0", now)
	}
	large, err := f.store.ListHoldersByBalance(ctx, "0xlarge")
	if err != nil || len(large) != 2 || large[0].HolderAddress != fmt.Sprintf("0x%040x", 1) {
		t.Fatalf("exact large balance order: %+v %v", large, err)
	}
	f.restart(t)
	all, err = f.store.ListHoldersByBalance(ctx, pool)
	if err != nil || !reflect.DeepEqual(all, canonical) {
		t.Fatalf("recovered holder ranks: %+v %v", all, err)
	}
}
