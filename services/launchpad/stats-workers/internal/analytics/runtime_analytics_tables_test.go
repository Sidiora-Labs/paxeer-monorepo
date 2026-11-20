package analytics_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRuntimeRoleAnalyticsTables(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_runtime")
	runtime := database.Runtimes["stats_runtime"]
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	want, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatalf("runtime readiness before analytics work: %v", err)
	}
	assertAnalyticsCatalog(t, ctx, pool)
	assertProjectionCatalog(t, ctx, database.AdminDSN)
	assertRuntimeStatements(t, ctx, pool)

	authority := analytics.ReactionAuthority{Pool: pool, Schema: "stats"}
	projector, err := analytics.NewProjector(authority, risk.ProjectionModelV1{}, analytics.PostgresRiskInputs{Authority: authority})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	poolAddress := "0x" + strings.Repeat("1", 40)
	blockHash := "0x" + strings.Repeat("b", 64)
	watermark := analytics.Watermark{
		ChainID:            "125",
		Height:             "100",
		BlockHash:          blockHash,
		ProjectionRevision: "1",
		ObservedAt:         now.Format("2006-01-02T15:04:05.000Z"),
		Integrity:          "healthy",
	}
	inputs := analytics.RiskInputs{CreatedAt: now.Unix() - 100, Top10Concentration: "0", CreatorHoldingsPct: "0", SupplyRaw: "100", UniqueTraders24h: 1, HolderCount: 1}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.PersistRiskInputsTx(ctx, tx, poolAddress, watermark, inputs); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("runtime risk-input write: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	f := runtimeAnalyticsFill(poolAddress, blockHash, now.Unix()-1)
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.ApplyFillTx(ctx, tx, f, now); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("runtime trade projection: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.RecomputeExpiryTx(ctx, tx, poolAddress); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("runtime analytics expiry: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	readTx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := (analytics.PostgresRiskInputs{Authority: authority}).LoadRiskInputs(ctx, readTx, poolAddress, watermark)
	if commitErr := readTx.Commit(ctx); err == nil {
		err = commitErr
	}
	if err != nil || loaded.SupplyRaw != inputs.SupplyRaw || loaded.HolderCount != inputs.HolderCount {
		t.Fatalf("runtime risk-input read: %+v %v", loaded, err)
	}
	page, err := authority.ReplaySnapshots(ctx, poolAddress, "0", 10)
	if err != nil || len(page.Items) != 4 {
		t.Fatalf("runtime outbox read: %d items: %v", len(page.Items), err)
	}
	var progress, trades, outbox, riskInputs int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats.analytics_progress WHERE pool=$1", poolAddress).Scan(&progress); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats.analytics_trades WHERE pool=$1", poolAddress).Scan(&trades); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats.analytics_outbox WHERE pool=$1", poolAddress).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats.analytics_risk_inputs WHERE pool=$1", poolAddress).Scan(&riskInputs); err != nil {
		t.Fatal(err)
	}
	if progress != 1 || trades != 1 || outbox != 4 || riskInputs != 1 {
		t.Fatalf("runtime analytics rows: progress=%d trades=%d outbox=%d risk_inputs=%d", progress, trades, outbox, riskInputs)
	}

	var revision, height, volume string
	if err := pool.QueryRow(ctx, "SELECT revision::text,height::text FROM stats.analytics_progress WHERE pool=$1", poolAddress).Scan(&revision, &height); err != nil || revision != "2" || height != "100" {
		t.Fatalf("analytics progress revision=%s height=%s: %v", revision, height, err)
	}
	if err := pool.QueryRow(ctx, "SELECT usdl_raw::text FROM stats.analytics_trades WHERE source_id=$1", f.SourceFillID).Scan(&volume); err != nil || volume != f.USDLAmountRaw {
		t.Fatalf("analytics trade volume=%s: %v", volume, err)
	}
	snapshot, err := authority.ReadSnapshot(ctx, poolAddress, "pressure-v1", now.Unix())
	if err != nil || snapshot.Snapshot.Watermark.ProjectionRevision != "2" || snapshot.Snapshot.Watermark.BlockHash != blockHash {
		t.Fatalf("expiry snapshot: %+v: %v", snapshot, err)
	}
	watermark.ProjectionRevision = "3"
	inputs.ProjectionRevision = "3"
	inputs.HolderCount = 2
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.PersistRiskInputsTx(ctx, tx, poolAddress, watermark, inputs); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("runtime risk-input update: %v", err)
	}
	loaded, err = (analytics.PostgresRiskInputs{Authority: authority}).LoadRiskInputs(ctx, tx, poolAddress, watermark)
	if err != nil || loaded != inputs {
		_ = tx.Rollback(ctx)
		t.Fatalf("updated risk-input rows: %+v: %v", loaded, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatalf("runtime readiness after analytics work: %v", err)
	}
}

func runtimeAnalyticsFill(pool, blockHash string, eventTime int64) accounting.FillV1 {
	position := accounting.SourcePosition{ChainID: "125", Height: "100", TransactionIndex: "0", LogIndex: "0", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: blockHash}
	f := accounting.FillV1{
		Version: accounting.FillVersion, Position: position, ActualEmitter: "0x" + strings.Repeat("2", 40), PoolAddress: pool,
		Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6},
		Side: "buy", TokenAmountRaw: "10", USDLAmountRaw: "100", PaidInputRaw: "100", FeeRaw: "1", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution:           accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:            accounting.Provenance{DeploymentID: "runtime-fixture", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)},
		BlockTimestampSeconds: fmt.Sprint(eventTime),
	}
	f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
	return f
}

func assertAnalyticsCatalog(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	expected := map[string][]string{
		"analytics_progress":    {"pool:text", "height:numeric(78,0)", "transaction_index:numeric(78,0)", "log_index:numeric(78,0)", "block_hash:text", "revision:numeric(78,0)", "observed_at:text"},
		"analytics_trades":      {"source_id:text", "pool:text", "height:numeric(78,0)", "transaction_index:numeric(78,0)", "log_index:numeric(78,0)", "block_hash:text", "event_time:bigint", "side:text", "usdl_raw:numeric(78,0)", "digest:text"},
		"analytics_outbox":      {"id:bigint", "pool:text", "model:text", "revision:numeric(78,0)", "payload:jsonb", "created_at:timestamp with time zone"},
		"analytics_risk_inputs": {"pool:text", "height:numeric(78,0)", "block_hash:text", "payload:jsonb"},
	}
	for table, want := range expected {
		rows, err := pool.Query(ctx, `SELECT a.attname,format_type(a.atttypid,a.atttypmod) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='stats' AND c.relname=$1 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, table)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var name, typ string
			if err := rows.Scan(&name, &typ); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			got = append(got, name+":"+typ)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("stats.%s columns: got %v want %v", table, got, want)
		}
	}
	keys := map[string]string{"analytics_progress": "pool", "analytics_trades": "source_id", "analytics_outbox": "id", "analytics_risk_inputs": "pool"}
	for table, column := range keys {
		var got string
		if err := pool.QueryRow(ctx, `SELECT a.attname FROM pg_constraint c JOIN pg_class r ON r.oid=c.conrelid JOIN pg_namespace n ON n.oid=r.relnamespace JOIN unnest(c.conkey) WITH ORDINALITY AS k(attnum,ord) ON true JOIN pg_attribute a ON a.attrelid=r.oid AND a.attnum=k.attnum WHERE n.nspname='stats' AND r.relname=$1 AND c.contype='p' ORDER BY k.ord`, table).Scan(&got); err != nil || got != column {
			t.Fatalf("stats.%s primary key: got %q want %q: %v", table, got, column, err)
		}
	}
	for _, index := range []string{"analytics_trades_pool_event_time_idx", "analytics_outbox_pool_id_idx"} {
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "stats."+index).Scan(&exists); err != nil || !exists {
			t.Fatalf("stats.%s index missing: %v", index, err)
		}
	}
	var sequence bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('stats.analytics_outbox_id_seq') IS NOT NULL").Scan(&sequence); err != nil || !sequence {
		t.Fatalf("analytics outbox identity sequence missing: %v", err)
	}
}
