package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repair70RawLog struct {
	Address, BlockHash, TransactionHash, Data string
	BlockNumber, TransactionIndex, Index      int64
	Topics                                    []string
	Removed                                   bool
}
type repair70Receipt struct {
	Label, Hash, BlockHash, From string
	BlockNumber, Index, Status   int64
	Logs                         []repair70RawLog
}
type repair70Fixture struct {
	dsn        string
	pool       *pgxpool.Pool
	projection *canonicalMultihopProjection
	cfg        config.Config
	document   statsRuntimeDocument
	logger     *slog.Logger
	guard      *queue.EpochGuard
	route      canonicalMultihopLog
	expected   store.CrossTokenSwapRow
}

func newRepair70Fixture(t *testing.T) *repair70Fixture {
	t.Helper()
	ctx := context.Background()
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	root := filepath.Join("..", "..", "..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "sdk", "launchpad", "protocol", "contracts", "v1", "chain", "golden-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != "b7538134b931cf24a04d3d18d8545d8865306f703ff5c421f7cff8424e99a993" {
		t.Fatal("real EVM producer receipt fixture changed")
	}
	var golden struct {
		ChainID        int
		ReplayVerified bool
		Addresses      map[string]string
		Receipts       []repair70Receipt
		DecodedLogs    []struct {
			Name, Address string
			BlockNumber   int64
			Args          []json.RawMessage
			Raw           repair70RawLog
		}
	}
	if err = json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.ChainID != 125 || !golden.ReplayVerified {
		t.Fatal("unqualified fixture")
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE SCHEMA indexer; CREATE SCHEMA recovery`)
	for _, id := range []string{"60000", "440000"} {
		ddl, err := os.ReadFile(filepath.Join(root, "launchpad", "spec", "production-system", "integration", "migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		exec(string(ddl))
	}
	partitionDDL, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	exec(partitionDDL)
	ddl, err := os.ReadFile(filepath.Join(root, "launchpad", "spec", "production-system", "integration", "migrations", "190001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(ddl))
	exec(canonicalMultihopSchemaSQL)
	exec(`INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,1,'fixture-region',1,repeat('0',64))`)
	var receipt repair70Receipt
	for _, r := range golden.Receipts {
		if r.Label == "multihop" {
			receipt = r
		}
	}
	if receipt.Status != 1 || receipt.BlockNumber != 68 || len(receipt.Logs) != 14 {
		t.Fatal("missing successful real multihop receipt")
	}
	for _, l := range receipt.Logs {
		if l.Removed || l.TransactionHash != receipt.Hash || l.BlockHash != receipt.BlockHash || l.BlockNumber != receipt.BlockNumber || l.TransactionIndex != receipt.Index {
			t.Fatal("receipt identity mismatch")
		}
		topics, err := json.Marshal(l.Topics)
		if err != nil {
			t.Fatal(err)
		}
		data, err := hex.DecodeString(strings.TrimPrefix(l.Data, "0x"))
		if err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, strings.ToLower(l.TransactionHash), l.Index, l.BlockNumber, l.TransactionIndex, strings.ToLower(l.BlockHash), strings.ToLower(l.Address), string(topics), data)
	}
	// The creation records come from the same real producer's factory receipts,
	// decoded in its frozen replay, rather than invented token/pool associations.
	for _, l := range golden.DecodedLogs {
		if l.Name != "MarketCreated" || !strings.EqualFold(l.Address, golden.Addresses["factory"]) || l.BlockNumber > receipt.BlockNumber {
			continue
		}
		args := make([]string, len(l.Args))
		for i, a := range l.Args {
			if err = json.Unmarshal(a, &args[i]); err != nil {
				t.Fatal(err)
			}
		}
		if len(args) != 5 {
			t.Fatal("factory creation shape")
		}
		exec(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1791504000,'{}',$12)`, strings.ToLower(args[0]), strings.ToLower(args[1]), strings.ToLower(args[2]), strings.ToLower(l.Address), strings.ToLower(args[4]), args[3], l.Raw.BlockNumber, strings.ToLower(l.Raw.BlockHash), strings.ToLower(l.Raw.TransactionHash), l.Raw.TransactionIndex, l.Raw.Index, string(rawJSON(t, args)))
	}
	exec(`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'chain125-canonical','real-indexer',1,clock_timestamp()+interval '1 hour')`)
	exec(`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'chain125-canonical',68,68)`)
	exec(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'chain125-canonical',68,$1,14,true,true)`, strings.ToLower(receipt.BlockHash))
	exec(`INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'chain125-canonical','evm-multihop',67,68,$1,14,0)`, strings.ToLower(receipt.BlockHash))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-region")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{RouterAddress: golden.Addresses["router"], EventEmitterAddress: golden.Addresses["eventEmitter"]}, LegacyBridgeBatch: 1, LegacyBridgeIntervalMS: 100}
	document := statsRuntimeDocument{SourceStream: "chain125-canonical"}
	projection := &canonicalMultihopProjection{pool: pool, stream: document.SourceStream, router: strings.ToLower(cfg.RouterAddress), emitter: strings.ToLower(cfg.EventEmitterAddress), batch: cfg.LegacyBridgeBatch, guard: guard, logger: logger}
	route, err := scanMultihopLog(pool.QueryRow(ctx, `SELECT `+multihopLogColumns+` FROM indexer.indexer_raw_logs_v1 WHERE log_index=12`))
	if err != nil {
		t.Fatal(err)
	}
	expected := store.CrossTokenSwapRow{ID: receipt.Hash + "-12", Sender: strings.ToLower(receipt.From), TokenIn: "0x82eeedbedd9e9f890b65924a9ca904726530d57a", TokenOut: "0xed9e01c0e704566017556794f9d89daf4363099d", PoolIn: "0x277100da3092ad2693701d36f7a659e834f80bf9", PoolOut: "0xa850aa7dea058da707b4c1d0668eef1f01a96aee", AmountIn: "14969972418576", IntermediateUsdl: "297812138", AmountOut: "28835684790084", FeeIn: "44909917255", FeeOut: "893436", BlockTimestamp: 1791504068, TxHash: receipt.Hash}
	return &repair70Fixture{dsn: dsn, pool: pool, projection: projection, cfg: cfg, document: document, logger: logger, guard: guard, route: route, expected: expected}
}

func rawJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func (f *repair70Fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}
func (f *repair70Fixture) counts(t *testing.T) (int, int) {
	t.Helper()
	var rows, checkpoints int
	if err := f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM stats.cross_token_swaps),(SELECT count(*) FROM stats.canonical_multihop_checkpoints)`).Scan(&rows, &checkpoints); err != nil {
		t.Fatal(err)
	}
	return rows, checkpoints
}
func (f *repair70Fixture) assertHistory(t *testing.T, want int) {
	t.Helper()
	router := chi.NewRouter()
	httpapi.RegisterCrossTokenSwaps(router, store.New(f.pool))
	response := httptest.NewRecorder()
	path := "/stats/cross-token-swaps/" + strings.ToUpper(f.expected.Sender)
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	var page struct {
		Swaps []store.CrossTokenSwapRow `json:"swaps"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Swaps) != want {
		t.Fatalf("wallet history: %d %s", response.Code, response.Body.String())
	}
	if want == 1 && page.Swaps[0] != f.expected {
		t.Fatalf("wallet row = %+v, want %+v", page.Swaps[0], f.expected)
	}
}
func (f *repair70Fixture) restart(t *testing.T) {
	t.Helper()
	f.pool.Close()
	pool, err := pgxpool.New(context.Background(), f.dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f.pool = pool
	old := *f.projection
	old.pool = pool
	f.projection = &old
}
func (f *repair70Fixture) runJob(t *testing.T) {
	t.Helper()
	health := &statsRuntimeHealth{failures: map[string]error{}}
	job, err := canonicalMultihopJob(f.pool, f.cfg, f.document, f.guard, f.logger, health)
	if err != nil {
		t.Fatal(err)
	}
	if health.check(context.Background()) == nil {
		t.Fatal("projection ready before first catchup")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- job.run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("stop production job: %v", err)
		}
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatalf("production multihop job did not catch up: %v", health.check(context.Background()))
		case <-ticker.C:
			if health.check(context.Background()) == nil {
				return
			}
		}
	}
}

func TestRepair_70_CrossWalletCatchupAndRestart(t *testing.T) {
	f := newRepair70Fixture(t)
	f.assertHistory(t, 0)
	f.runJob(t)
	f.assertHistory(t, 1)
	if rows, checkpoints := f.counts(t); rows != 1 || checkpoints != 1 {
		t.Fatal(rows, checkpoints)
	}
	var before time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT committed_at FROM stats.canonical_multihop_checkpoints`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	metrics, err := store.New(f.pool).PlatformMetrics(context.Background(), f.expected.BlockTimestamp+1)
	if err != nil || metrics.CrossTokenSwaps24h != 1 {
		t.Fatal(metrics, err)
	}
	f.restart(t)
	f.runJob(t)
	f.assertHistory(t, 1)
	if n, err := f.projection.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	var after time.Time
	if err = f.pool.QueryRow(context.Background(), `SELECT committed_at FROM stats.canonical_multihop_checkpoints`).Scan(&after); err != nil || !after.Equal(before) {
		t.Fatal(before, after, err)
	}
	// Redeliver the exact persisted route through the same transactional consumer.
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = f.projection.projectRoute(context.Background(), tx, f.route); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.assertHistory(t, 1)
	if rows, checkpoints := f.counts(t); rows != 1 || checkpoints != 1 {
		t.Fatal(rows, checkpoints)
	}
}

func TestRepair_70_CheckpointFailureRollsBackAndRecovers(t *testing.T) {
	f := newRepair70Fixture(t)
	f.exec(t, `ALTER TABLE stats.canonical_multihop_checkpoints ADD CONSTRAINT repair70_checkpoint_failure CHECK(log_index<>12)`)
	n, err := f.projection.RunOnce(context.Background())
	var failure *pgconn.PgError
	if n != 0 || !errors.As(err, &failure) || failure.Code != "23514" || failure.ConstraintName != "repair70_checkpoint_failure" {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if rows, checkpoints := f.counts(t); rows != 0 || checkpoints != 0 {
		t.Fatal("partial commit", rows, checkpoints)
	}
	f.assertHistory(t, 0)
	f.restart(t)
	f.exec(t, `ALTER TABLE stats.canonical_multihop_checkpoints DROP CONSTRAINT repair70_checkpoint_failure`)
	if n, err = f.projection.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
	if rows, checkpoints := f.counts(t); rows != 1 || checkpoints != 1 {
		t.Fatal(rows, checkpoints)
	}
}

func TestRepair_70_CheckpointAndWalletRowBecomeVisibleTogether(t *testing.T) {
	f := newRepair70Fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	blocker, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(ctx, `LOCK TABLE stats.canonical_multihop_checkpoints IN SHARE MODE`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() { n, err := f.projection.RunOnce(ctx); done <- result{n, err} }()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	waiting := false
	for !waiting {
		select {
		case r := <-done:
			t.Fatalf("projection ended before checkpoint lock: %+v", r)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			if err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'INSERT INTO stats.canonical_multihop_checkpoints%')`).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
		}
	}
	if rows, checkpoints := f.counts(t); rows != 0 || checkpoints != 0 {
		t.Fatal("visible before checkpoint", rows, checkpoints)
	}
	f.assertHistory(t, 0)
	// A second live participant cannot apply this uncommitted route.
	other := *f.projection
	if n, err := other.RunOnce(ctx); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err != nil || r.n != 1 {
		t.Fatal(r.n, r.err)
	}
	if rows, checkpoints := f.counts(t); rows != 1 || checkpoints != 1 {
		t.Fatal(rows, checkpoints)
	}
	f.assertHistory(t, 1)
}

func TestRepair_70_RejectsUnboundRouteEvidence(t *testing.T) {
	cases := []struct {
		name, sql   string
		unavailable bool
	}{
		{"missing leg", `DELETE FROM indexer.indexer_raw_logs WHERE log_index=2`, false},
		{"missing mirror", `DELETE FROM indexer.indexer_raw_logs WHERE log_index=13`, false},
		{"route amount", `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,31,1) WHERE log_index=12`, false},
		{"mirror amount", `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,95,1) WHERE log_index=13`, false},
		{"leg amount", `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,63,1) WHERE log_index=2`, false},
		{"leg wallet", `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{2}',to_jsonb('0x0000000000000000000000001111111111111111111111111111111111111111'::text)) WHERE log_index=11`, false},
		{"leg block identity", `UPDATE indexer.indexer_raw_logs SET block_hash='0x'||repeat('a',64) WHERE log_index=2`, false},
		{"wrong pool mapping", `UPDATE indexer.indexer_canonical_creations SET pool='0x1111111111111111111111111111111111111111' WHERE token='0xed9e01c0e704566017556794f9d89daf4363099d'`, false},
		{"incomplete block", `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`, false},
		{"missing source commit", `DELETE FROM indexer.indexer_ingest_commits`, false},
		{"uncommitted typed head", `UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=67`, true},
		{"expired source", `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second'`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepair70Fixture(t)
			f.exec(t, tc.sql)
			want := consumer.ErrLegacyBridgeIntegrity
			if tc.unavailable {
				want = consumer.ErrAccountingSourceUnavailable
			}
			if n, err := f.projection.RunOnce(context.Background()); n != 0 || !errors.Is(err, want) {
				t.Fatalf("n=%d err=%v want=%v", n, err, want)
			}
			if rows, checkpoints := f.counts(t); rows != 0 || checkpoints != 0 {
				t.Fatal(rows, checkpoints)
			}
			f.assertHistory(t, 0)
		})
	}
}

func TestRepair_70_DoesNotInventRoutesFromPoolFills(t *testing.T) {
	f := newRepair70Fixture(t)
	f.exec(t, `DELETE FROM indexer.indexer_raw_logs WHERE log_index IN (12,13)`)
	if n, err := f.projection.RunOnce(context.Background()); n != 0 || err != nil {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 0)
	if rows, checkpoints := f.counts(t); rows != 0 || checkpoints != 0 {
		t.Fatal(rows, checkpoints)
	}
}

func TestRepair_70_RedeliveryRejectsIdentityConflict(t *testing.T) {
	f := newRepair70Fixture(t)
	if n, err := f.projection.RunOnce(context.Background()); n != 1 || err != nil {
		t.Fatal(n, err)
	}
	// A changed price leaves the public row unchanged but changes source evidence.
	f.exec(t, `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,159,1) WHERE log_index=2`)
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = f.projection.projectRoute(context.Background(), tx, f.route); !errors.Is(err, consumer.ErrLegacyBridgeIntegrity) {
		t.Fatal(err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.assertHistory(t, 1)
	if rows, checkpoints := f.counts(t); rows != 1 || checkpoints != 1 {
		t.Fatal(rows, checkpoints)
	}
}

func TestRepair_70_RejectsExistingConflictingWalletRow(t *testing.T) {
	f := newRepair70Fixture(t)
	conflict := f.expected
	conflict.AmountOut = "1"
	if err := store.New(f.pool).InsertCrossTokenSwap(context.Background(), conflict); err != nil {
		t.Fatal(err)
	}
	if n, err := f.projection.RunOnce(context.Background()); n != 0 || !errors.Is(err, consumer.ErrLegacyBridgeIntegrity) {
		t.Fatal(n, err)
	}
	if rows, checkpoints := f.counts(t); rows != 1 || checkpoints != 0 {
		t.Fatal(rows, checkpoints)
	}
	got, err := store.New(f.pool).ListCrossTokenSwapsByWallet(context.Background(), conflict.Sender, 1, 0)
	if err != nil || len(got) != 1 || got[0] != conflict {
		t.Fatal(got, err)
	}
}

func TestRepair_70_RecoveryEpochFencesProjection(t *testing.T) {
	f := newRepair70Fixture(t)
	f.exec(t, `UPDATE recovery.recovery_control SET epoch=2`)
	if n, err := f.projection.RunOnce(context.Background()); n != 0 || err == nil {
		t.Fatal(n, err)
	}
	if rows, checkpoints := f.counts(t); rows != 0 || checkpoints != 0 {
		t.Fatal(rows, checkpoints)
	}
	f.assertHistory(t, 0)
}

func TestRepair_70_ProductionJobRequiresCanonicalDependencies(t *testing.T) {
	f := newRepair70Fixture(t)
	health := &statsRuntimeHealth{failures: map[string]error{}}
	bad := f.cfg
	bad.RouterAddress = ""
	if _, err := canonicalMultihopJob(f.pool, bad, f.document, f.guard, f.logger, health); err == nil {
		t.Fatal("missing router accepted")
	}
	if _, err := canonicalMultihopJob(f.pool, f.cfg, f.document, nil, f.logger, health); err == nil {
		t.Fatal("missing epoch accepted")
	}
	if _, err := canonicalMultihopJob(f.pool, f.cfg, statsRuntimeDocument{}, f.guard, f.logger, health); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err := canonicalMultihopJob(f.pool, f.cfg, f.document, f.guard, f.logger, health); err != nil {
		t.Fatal(fmt.Errorf("real production job: %w", err))
	}
}
