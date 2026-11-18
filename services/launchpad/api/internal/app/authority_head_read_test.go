package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authorityHeadReadQueries struct {
	mu         sync.Mutex
	statements []string
}

func (q *authorityHeadReadQueries) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.statements = append(q.statements, data.SQL)
	return ctx
}

func (*authorityHeadReadQueries) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func authorityHeadReadState(t *testing.T) (*networkAuthority, *authorityStateReader) {
	t.Helper()
	a, reader, _ := newAuthorityReadinessState(t)
	commit := productionAuthorityHeads(t, a)
	internaltest.ApplyReadMigrations(t, a.pool, "60005", "60006")
	commit(reader.header, "healthy")
	a.origins = map[string]bool{webOrigin: true}
	return a, reader
}

func authorityHeadReadMany(t *testing.T, a *networkAuthority, header *types.Header) {
	t.Helper()
	if _, err := a.pool.Exec(t.Context(), "INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) SELECT 125,'authority',$1::numeric+n,'0x'||lpad(to_hex(n),64,'0'),'healthy','chain' FROM generate_series(1,6000) AS n", header.Number.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.pool.Exec(t.Context(), "ANALYZE indexer.indexer_committed_heads"); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorityHeadReadPlan(t *testing.T) {
	a, reader := authorityHeadReadState(t)
	authorityHeadReadMany(t, a, reader.header)
	queries := &authorityHeadReadQueries{}
	cfg := a.pool.Config()
	cfg.ConnConfig.Tracer = queries
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	a.pool = pool
	assertProductionPreflight(t, productionAuthorityRouter(t, a), productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash()), reader.header.Hash())
	queries.mu.Lock()
	statements := append([]string(nil), queries.statements...)
	queries.mu.Unlock()
	if len(statements) != 1 {
		t.Fatalf("head acceptance statements = %v", statements)
	}
	statement := statements[0]
	aggregate := regexp.MustCompile(`(?i)\b(avg|count|max|min|sum|array_agg|bool_and|bool_or|every|json_agg|jsonb_agg|json_object_agg|jsonb_object_agg|string_agg|bit_and|bit_or|bit_xor|range_agg|range_intersect_agg|corr|covar_pop|covar_samp|regr_[a-z_]+|stddev|stddev_pop|stddev_samp|variance|var_pop|var_samp)\s*\(`)
	if aggregate.MatchString(statement) {
		t.Fatalf("head acceptance contains an aggregate: %s", statement)
	}
	var raw []byte
	if err := pool.QueryRow(t.Context(), "EXPLAIN (FORMAT JSON) "+statement, reader.header.Number.String(), reader.header.Hash().Hex()).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	type planNode struct {
		NodeType  string            `json:"Node Type"`
		IndexCond string            `json:"Index Cond"`
		Plans     []json.RawMessage `json:"Plans"`
	}
	var plan []struct {
		Plan json.RawMessage `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("head acceptance plan = %s, %v", raw, err)
	}
	keyed := false
	var inspect func(json.RawMessage)
	inspect = func(raw json.RawMessage) {
		var node planNode
		if err := json.Unmarshal(raw, &node); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(node.NodeType, "Aggregate") {
			t.Fatalf("head acceptance has an Aggregate node: %s", raw)
		}
		if strings.Contains(node.IndexCond, "chain_id = 125") && strings.Contains(node.IndexCond, "height =") {
			keyed = true
		}
		for _, child := range node.Plans {
			inspect(child)
		}
	}
	inspect(plan[0].Plan)
	if !keyed {
		t.Fatalf("head acceptance has no index lookup by chain and height: %s", raw)
	}
}

func TestAuthorityHeadReadManyHeads(t *testing.T) {
	a, reader := authorityHeadReadState(t)
	authorityHeadReadMany(t, a, reader.header)
	router := productionAuthorityRouter(t, a)
	assertProductionPreflight(t, router, productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash()), reader.header.Hash())
	reader.header.Number.Add(reader.header.Number, big.NewInt(6001))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash()))
	assertAPIError(t, response.Code, response.Body.Bytes(), 409, "REVISION_CONFLICT")
}

type authorityHeadReadDelay struct {
	deployment.Reader
	delay time.Duration
}

func (r authorityHeadReadDelay) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	timer := time.NewTimer(r.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return r.Reader.HeaderByHash(ctx, hash)
	}
}

func TestAuthorityHeadReadBounds(t *testing.T) {
	if authorityHeadHeaderTimeout < 2*time.Second || authorityHeadRecordTimeout < 300*time.Millisecond {
		t.Fatal("head read bounds are below the authority policy")
	}
	a, reader := authorityHeadReadState(t)
	router := productionAuthorityRouter(t, a)
	for _, tc := range []struct {
		name                  string
		delay, requestTimeout time.Duration
		status                int
		elapsed               time.Duration
	}{
		{"slow header", 400 * time.Millisecond, 0, 200, 400 * time.Millisecond},
		{"header deadline", authorityHeadHeaderTimeout + time.Second, 0, 503, authorityHeadHeaderTimeout},
		{"request deadline", time.Second, 50 * time.Millisecond, 503, 50 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a.reader = authorityHeadReadDelay{Reader: reader, delay: tc.delay}
			r := productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash())
			start := time.Now()
			if tc.requestTimeout > 0 {
				ctx, cancel := context.WithTimeout(r.Context(), tc.requestTimeout)
				defer cancel()
				r = r.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, r)
			elapsed := time.Since(start)
			if elapsed < tc.elapsed || elapsed > tc.elapsed+time.Second {
				t.Fatalf("header read elapsed = %v, want %v within one second", elapsed, tc.elapsed)
			}
			if tc.status == 200 {
				if response.Code != 200 {
					t.Fatalf("slow header = %d %s", response.Code, response.Body)
				}
				if _, err := contractPreflight(response.Body.Bytes()); err != nil {
					t.Fatal(err)
				}
			} else {
				assertAPIError(t, response.Code, response.Body.Bytes(), 503, "DEPENDENCY_UNAVAILABLE")
			}
		})
	}
	t.Run("record deadline", func(t *testing.T) {
		a.reader = authorityHeadReadDelay{Reader: reader, delay: 400 * time.Millisecond}
		lock, err := a.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Rollback(context.Background())
		if _, err := lock.Exec(t.Context(), "LOCK TABLE indexer.indexer_committed_heads IN ACCESS EXCLUSIVE MODE"); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		start := time.Now()
		router.ServeHTTP(response, productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash()))
		elapsed := time.Since(start)
		if elapsed < 400*time.Millisecond+authorityHeadRecordTimeout || elapsed > 400*time.Millisecond+authorityHeadRecordTimeout+time.Second {
			t.Fatalf("record read elapsed = %v", elapsed)
		}
		assertAPIError(t, response.Code, response.Body.Bytes(), 503, "DEPENDENCY_UNAVAILABLE")
	})
}

func TestAuthorityHeadReadRevision(t *testing.T) {
	source, manifest := authorityEVMSource(t)
	_, client := forkChain(t, source)
	reader, err := capabilities.NewPinnedRPCReader(client)
	if err != nil {
		t.Fatal(err)
	}
	header, err := reader.HeaderByNumber(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if wait := time.Until(time.Unix(int64(header.Time), 0)); wait > 0 {
		time.Sleep(wait)
	}
	engine, err := capabilities.NewPreflightEngine(reader, manifest, developmentPoolRuntime, developmentSerialInterface, authorityMaxAge, authorityAttestationTimeout)
	if err != nil {
		t.Fatal(err)
	}
	a := &networkAuthority{reader: reader, engine: engine, origins: map[string]bool{webOrigin: true}}
	commit := productionAuthorityHeads(t, a)
	internaltest.ApplyReadMigrations(t, a.pool, "60005", "60006")
	commit(header, "healthy")
	indexerCursors(t, a.pool, header.Number.Uint64())
	dev := &developmentAuthority{store: store.New(a.pool, nil), rpc: client, reader: reader, engine: engine, origins: a.origins, logger: slog.Default()}
	admission, err := rest.NewAdmission(64, 64, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	devRouter := chi.NewRouter()
	dev.Register(devRouter, admission)
	for name, router := range map[string]http.Handler{"production": productionAuthorityRouter(t, a), "development": devRouter} {
		t.Run(name, func(t *testing.T) {
			for path, family := range map[string]capabilities.Family{"create": capabilities.Creation, "quote": capabilities.PlainQuote, "trade": capabilities.Trading, "claim": capabilities.NFTFees} {
				t.Run(path, func(t *testing.T) {
					for _, revision := range []string{strings.Repeat("0", 64), a.engine.Revision().RevisionSHA256} {
						r := productionPreflightRequest(t, a, path, family, header.Hash())
						var input capabilities.PreflightRequest
						if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
							t.Fatal(err)
						}
						input.ExpectedRevision = revision
						raw, err := json.Marshal(input)
						if err != nil {
							t.Fatal(err)
						}
						r.Body = io.NopCloser(bytes.NewReader(raw))
						if revision == a.engine.Revision().RevisionSHA256 {
							assertProductionPreflight(t, router, r, header.Hash())
						} else {
							response := httptest.NewRecorder()
							router.ServeHTTP(response, r)
							assertAPIError(t, response.Code, response.Body.Bytes(), 409, "REVISION_CONFLICT")
						}
					}
				})
			}
		})
	}
}

func TestAuthorityHeadReadProjection(t *testing.T) {
	a, reader, _ := newAuthorityReadinessState(t)
	reader.header.Number.SetUint64(9007199254740992)
	r := productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash())
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	var reports []error
	servePreflight(response, r, a.engine, capabilities.Trading, raw, func(err error) { reports = append(reports, err) })
	assertAPIError(t, response.Code, response.Body.Bytes(), 500, "INTERNAL_ERROR")
	if len(reports) != 1 || reports[0] == nil {
		t.Fatalf("projection reports = %v", reports)
	}
}
