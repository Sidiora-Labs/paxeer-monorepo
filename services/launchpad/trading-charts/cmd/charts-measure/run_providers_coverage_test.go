package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func measurementCommitTimestampDatabase(t *testing.T) string {
	t.Helper()
	binary, err := exec.LookPath("pg_virtualenv")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	environmentPath, stopPath := filepath.Join(directory, "database.env"), filepath.Join(directory, "stop")
	output, err := os.OpenFile(filepath.Join(directory, "database.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-t", "-o", "track_commit_timestamp=on", "sh", "-c", `printf '%s\n%s\n%s\n%s\n' "$PGHOST" "$PGPORT" "$PGUSER" "$PGPASSWORD" > "$1"; while [ ! -f "$2" ]; do sleep 0.05; done`, "charts-database", environmentPath, stopPath)
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(stopPath, nil, 0600); err != nil {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			t.Error(err)
		}
		_ = output.Close()
	})
	var fields []string
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		raw, err := os.ReadFile(environmentPath)
		if err == nil {
			fields = strings.Split(strings.TrimSpace(string(raw)), "\n")
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(fields) != 4 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[3] == "" {
		t.Fatalf("database connection authority: %d fields", len(fields))
	}
	dsnURL := url.URL{Scheme: "postgres", User: url.UserPassword(fields[2], fields[3]), Host: net.JoinHostPort(fields[0], fields[1]), Path: "/postgres"}
	dsn := dsnURL.String() + "?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var value string
	if err := pool.QueryRow(context.Background(), `SELECT current_setting('track_commit_timestamp')`).Scan(&value); err != nil || value != "on" {
		t.Fatalf("commit timestamp database: %q, %v", value, err)
	}
	return dsn
}

func TestRunConnectsOwnedProvidersBeforeArchiveAdmission(t *testing.T) {
	dsn := measurementCommitTimestampDatabase(t)
	chain := internaltest.NewMarketChainAt(t, 1703980800, 9*24*time.Hour)
	first, last, rpcURL := chain.Headers[1], chain.Headers[2], chain.RPCURL
	if first.Time >= last.Time || last.Time < 1704672000 {
		t.Fatalf("archive chain bounds: %+v %+v", first, last)
	}
	weekStart, err := engine.BucketStart(time.Now().Unix(), "1w")
	if err != nil {
		t.Fatal(err)
	}
	sourceHeight := chain.Swaps[1].BlockNumber.Int64()
	_, brokerURL := measurementBrokerWithURL(t, context.Background())
	digest := strings.Repeat("a", 64)
	previousDigest, previousArgs, previousFlags := candidateDigest, os.Args, flag.CommandLine
	candidateDigest = digest
	t.Cleanup(func() {
		candidateDigest, os.Args, flag.CommandLine = previousDigest, previousArgs, previousFlags
	})
	poolA, poolB := strings.ToLower(chain.Pool.Hex()), "0x"+strings.Repeat("b", 40)
	profile := config{
		CandidateDigest: digest, InterfaceDigest: digest, Schema: "candles", SourceStream: "canonical",
		NATSStream: "CHART_MEASURE", NATSDurable: "observer", Applications: []string{"charts"},
		Pools: []string{poolA, poolB}, HotPool: poolA, From: weekStart, To: weekStart + 604800,
		SampleMillis: 1000, MaximumBars: 1000, RawArtifactBudgetBytes: 1048576,
		StartAt: time.Unix(1704067200, 0), BurstOffsetSeconds: 60, RecoverySeconds: 60,
	}
	for name, value := range map[string]string{"DATABASE_URL": dsn, "RPC_URL": rpcURL, "NATS_URL": brokerURL, "S3_ENDPOINT": " ", "S3_BUCKET": "charts", "S3_ACCESS_KEY_ID": "local", "S3_SECRET_ACCESS_KEY": "local"} {
		t.Setenv(name, value)
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	output := filepath.Join(t.TempDir(), "measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
	if err := run(); err == nil || !strings.Contains(err.Error(), "actual archive object provider unavailable") {
		t.Fatalf("archive provider admission: %v", err)
	}
	for _, name := range []string{"samples.ndjson", "latencies.ndjson", "locks.ndjson"} {
		info, err := os.Stat(filepath.Join(output, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private measurement artifact %s: %+v, %v", name, info, err)
		}
	}
	objects, objectConfig := internaltest.NewObjectStoreWithConfig(t)
	t.Setenv("S3_ENDPOINT", objectConfig.Endpoint)
	t.Setenv("S3_BUCKET", objectConfig.Bucket)
	t.Setenv("S3_ACCESS_KEY_ID", objectConfig.AccessKeyID)
	t.Setenv("S3_SECRET_ACCESS_KEY", objectConfig.SecretAccessKey)
	database, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL, store.RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(context.Background(), ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(context.Background(), `CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_archive_segments_v2(object_key text PRIMARY KEY,manifest jsonb NOT NULL,manifest_digest text NOT NULL,verified_at timestamptz NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	blocks := []store.ArchiveBlock{{Height: 1, Hash: first.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}, {Height: 2, Hash: last.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}
	segment := store.ArchiveSegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, Blocks: blocks, Records: []store.ArchiveRaw{}, Events: []store.ArchiveEvent{}}
	segmentWire, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	segmentDigest := sha256.Sum256(segmentWire)
	manifest := store.ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, ByteLength: int64(len(segmentWire)), SHA256: hex.EncodeToString(segmentDigest[:]), Bucket: objects.Bucket(), ObjectKey: "indexer/raw/v2/canonical/" + hex.EncodeToString(segmentDigest[:]) + ".json", Blocks: blocks}
	manifestWire, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestWire)
	if err := objects.Put(context.Background(), manifest.ObjectKey, bytes.NewReader(segmentWire), int64(len(segmentWire)), "application/json"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO indexer.indexer_archive_segments_v2 VALUES($1,$2,$3,clock_timestamp())`, manifest.ObjectKey, manifestWire, hex.EncodeToString(manifestDigest[:])); err != nil {
		t.Fatal(err)
	}
	manifestList, err := json.Marshal([]string{hex.EncodeToString(manifestDigest[:])})
	if err != nil {
		t.Fatal(err)
	}
	listDigest := sha256.Sum256(manifestList)
	retentionFrom, retentionTo := int64(1704067200), int64(1704672000)
	if _, err := database.Exec(context.Background(), `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,$2,$3,$4)`, poolA, retentionFrom, retentionTo, hex.EncodeToString(listDigest[:])); err != nil {
		t.Fatal(err)
	}
	manifestsWire, err := json.Marshal([]store.ArchiveManifest{manifest})
	if err != nil {
		t.Fatal(err)
	}
	manifestsPath := filepath.Join(t.TempDir(), "manifests.json")
	if err := os.WriteFile(manifestsPath, manifestsWire, 0600); err != nil {
		t.Fatal(err)
	}
	manifestsDigest := sha256.Sum256(manifestsWire)
	profile.Retention = retentionConfig{Pool: poolA, Interval: "1m", From: retentionFrom, To: retentionTo, ManifestsPath: manifestsPath, ManifestsSHA256: hex.EncodeToString(manifestsDigest[:])}
	raw, err = json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(raw)
	output = filepath.Join(t.TempDir(), "archive-measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
	if err := run(); err == nil || !strings.Contains(err.Error(), "indexer_ingest_watermarks_v1") {
		t.Fatalf("canonical source observation: %v", err)
	}
	if _, err := database.Exec(context.Background(), `CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint,stream_id text,raw_complete_height bigint,typed_complete_height bigint);
CREATE TABLE indexer.indexer_ingest_leases_v1(chain_id bigint,stream_id text,expires_at timestamptz);
INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'canonical',`+strconv.FormatInt(sourceHeight, 10)+`,`+strconv.FormatInt(sourceHeight, 10)+`);
INSERT INTO indexer.indexer_ingest_leases_v1 VALUES(125,'canonical',clock_timestamp()+interval '1 hour')`); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{poolA, poolB} {
		if _, err := database.Exec(context.Background(), `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,$2,$3,false)`, address, sourceHeight, profile.To); err != nil {
			t.Fatal(err)
		}
	}
	output = filepath.Join(t.TempDir(), "source-measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
	if err := run(); err == nil || !strings.Contains(err.Error(), "indexer_accounting_fills") {
		t.Fatalf("canonical fill reconciliation: %v", err)
	}
	samples, err := os.ReadFile(filepath.Join(output, "samples.ndjson"))
	if err != nil || !bytes.Contains(samples, []byte(`"sourceHeight":`+strconv.FormatInt(sourceHeight, 10))) {
		t.Fatalf("captured canonical source: %s, %v", samples, err)
	}
	if _, err := database.Exec(context.Background(), `CREATE TABLE indexer.indexer_accounting_fills(chain_id bigint,transaction_hash text,log_index bigint,pool_address text,payload jsonb,source_fact_digest text);
CREATE TABLE indexer.indexer_raw_logs_v1(chain_id bigint,transaction_hash text,log_index bigint,height bigint,block_hash text,transaction_index bigint);
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint,stream_id text,height bigint,block_hash text,fetch_complete boolean,required_decode_complete boolean)`); err != nil {
		t.Fatal(err)
	}
	receipt := chain.Swaps[0]
	log := receipt.Logs[0]
	receiptWire, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	headerWire, err := json.Marshal(chain.Headers[int64(log.BlockNumber)])
	if err != nil {
		t.Fatal(err)
	}
	receiptDigest := sha256.Sum256(receiptWire)
	headerDigest := sha256.Sum256(headerWire)
	fill := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatUint(log.BlockNumber, 10), TransactionIndex: strconv.FormatUint(uint64(log.TxIndex), 10), LogIndex: strconv.FormatUint(uint64(log.Index), 10), TransactionHash: log.TxHash.Hex(), BlockHash: log.BlockHash.Hex()},
		ActualEmitter: strings.ToLower(log.Address.Hex()), PoolAddress: poolA,
		Token:      accounting.Asset{Address: strings.ToLower(chain.Token.Hex()), Decimals: 18},
		Settlement: accounting.Asset{Address: strings.ToLower(chain.Settlement.Hex()), Decimals: 6},
		Side:       "buy", TokenAmountRaw: "2000000", USDLAmountRaw: "1000000", PaidInputRaw: "1000000", FeeRaw: "1000",
		FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete", BlockTimestampSeconds: strconv.FormatInt(chain.Headers[int64(log.BlockNumber)].Time, 10),
		Attribution: accounting.Attribution{TransactionSender: strings.ToLower(chain.Sender.Hex()), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:  accounting.Provenance{DeploymentID: "measurement-local", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: hex.EncodeToString(receiptDigest[:]), HeaderSHA256: hex.EncodeToString(headerDigest[:])},
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	if err := accounting.ValidateFillV1(fill); err != nil {
		t.Fatal(err)
	}
	fillWire, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	fillDigest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO indexer.indexer_accounting_fills VALUES(125,$1,$2,$3,$4,$5)`, fill.Position.TransactionHash, int64(log.Index), poolA, fillWire, string(fillDigest)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO indexer.indexer_raw_logs_v1 VALUES(125,$1,$2,$3,$4,$5)`, fill.Position.TransactionHash, int64(log.Index), int64(log.BlockNumber), log.BlockHash.Hex(), int64(log.TxIndex)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'canonical',$1,$2,true,true)`, int64(log.BlockNumber), log.BlockHash.Hex()); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(database, "candles")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := database.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.ApplySeriesTx(context.Background(), tx, exact, fill, 1, engine.SeriesCheckpoint{Height: uint64(sourceHeight), Timestamp: profile.To}); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if completed, err := engine.RecomputeRollups(context.Background(), exact, 64); err != nil || completed != 6 {
		t.Fatalf("completed parent series: %d, %v", completed, err)
	}
	if _, err := database.Exec(context.Background(), `UPDATE candles.chart_outbox SET published_at=clock_timestamp() WHERE published_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	output = filepath.Join(t.TempDir(), "parity-measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
	if err := run(); err != nil {
		t.Fatalf("reconciled measurement: %v", err)
	}
	resultWire, err := os.ReadFile(filepath.Join(output, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Parity struct {
			AcceptedUniqueFills int    `json:"acceptedUniqueFills"`
			ReferenceDigest     string `json:"referenceDigest"`
			ActualDigest        string `json:"actualDigest"`
			ExactParity         bool   `json:"exactParity"`
		} `json:"parity"`
	}
	if err := json.Unmarshal(resultWire, &result); err != nil || result.Parity.AcceptedUniqueFills != 1 || !result.Parity.ExactParity || result.Parity.ReferenceDigest == "" || result.Parity.ReferenceDigest != result.Parity.ActualDigest {
		t.Fatalf("exact measurement parity: %+v, %v", result.Parity, err)
	}
	runVariant := func(t *testing.T, variant config, release bool, parent context.Context, want string) {
		t.Helper()
		wire, err := json.Marshal(variant)
		if err != nil {
			t.Fatal(err)
		}
		variantPath := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(variantPath, wire, 0600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(wire)
		flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
		flag.CommandLine.SetOutput(io.Discard)
		os.Args = []string{"charts-measure", "-config", variantPath, "-config-sha256", hex.EncodeToString(digest[:]), "-output", filepath.Join(t.TempDir(), "measurement")}
		if release {
			os.Args = append(os.Args, "-release")
		}
		if err := runWithContext(parent, 100*time.Millisecond); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("provider observation: %v", err)
		}
	}
	for _, tc := range []struct {
		name, environment, value, want string
		change                         func(*config)
	}{
		{"unavailable chain RPC", "RPC_URL", "not-a-rpc-url", "actual canonical RPC unavailable", nil},
		{"unavailable broker", "NATS_URL", "nats://127.0.0.1:1", "actual NATS unavailable", nil},
		{"missing broker stream", "", "", "stream", func(c *config) { c.NATSStream = "CHART_UNAVAILABLE" }},
		{"missing broker consumer", "", "", "consumer", func(c *config) { c.NATSDurable = "unavailable" }},
		{"missing archive manifest document", "", "", "bounded regular input required", func(c *config) { c.Retention.ManifestsPath = filepath.Join(t.TempDir(), "missing.json") }},
		{"unsupported retention interval", "", "", "unsupported_resolution", func(c *config) { c.Retention.Interval = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.environment != "" {
				t.Setenv(tc.environment, tc.value)
			}
			variant := profile
			if tc.change != nil {
				tc.change(&variant)
			}
			runVariant(t, variant, false, context.Background(), tc.want)
		})
	}
	t.Run("stale release start", func(t *testing.T) {
		runVariant(t, profile, true, context.Background(), "fresh coordinated release start")
	})
	t.Run("release start context", func(t *testing.T) {
		variant := profile
		variant.StartAt = time.Now().Add(3 * time.Second)
		bounded, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		runVariant(t, variant, true, bounded, "context deadline exceeded")
	})
	t.Run("sampling context", func(t *testing.T) {
		bounded, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		runVariant(t, profile, false, bounded, "context deadline exceeded")
	})
	t.Run("recovery budget", func(t *testing.T) {
		if _, err := database.Exec(context.Background(), `UPDATE candles.chart_series_clock SET watermark_height=$1 WHERE chain_id='125'`, sourceHeight-1); err != nil {
			t.Fatal(err)
		}
		variant := profile
		variant.RecoverySeconds = 1
		runVariant(t, variant, false, context.Background(), "did not recover")
		if _, err := database.Exec(context.Background(), `UPDATE candles.chart_series_clock SET watermark_height=$1 WHERE chain_id='125'`, sourceHeight); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unlisted release archive mutation", func(t *testing.T) {
		variant := profile
		variant.StartAt = time.Now().Add(time.Second)
		variant.Retention.StagingDeletionAdmitted = true
		variant.Retention.DeleteStagingObject = "indexer/raw/v2/canonical/unlisted.json"
		runVariant(t, variant, true, context.Background(), "exact verified staging object")
	})
	releaseProfile := profile
	releaseProfile.StartAt = time.Now().Add(4 * time.Second)
	releaseWire, err := json.Marshal(releaseProfile)
	if err != nil {
		t.Fatal(err)
	}
	releasePath := filepath.Join(t.TempDir(), "release-profile.json")
	if err := os.WriteFile(releasePath, releaseWire, 0600); err != nil {
		t.Fatal(err)
	}
	releaseDigest := sha256.Sum256(releaseWire)
	releaseOutput := filepath.Join(t.TempDir(), "release-observation")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", releasePath, "-config-sha256", hex.EncodeToString(releaseDigest[:]), "-output", releaseOutput, "-release"}
	if err := runWithContext(context.Background(), 1200*time.Millisecond); err != nil {
		t.Fatalf("coordinated chart observation: %v", err)
	}
	releaseResult, err := os.ReadFile(filepath.Join(releaseOutput, "result.json"))
	if err != nil || !bytes.Contains(releaseResult, []byte(`"profile":"24_hour_chart_observer"`)) {
		t.Fatalf("coordinated observation profile: %s, %v", releaseResult, err)
	}
	latencyFile, err := os.CreateTemp(t.TempDir(), "latencies-*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = latencyFile.Close() })
	latencyMeasurements := measurements{Latencies: boundedArtifact{file: latencyFile, budget: &rawBudget{maximum: 1 << 20}}}
	assertReconcile := func(label, want string) {
		t.Helper()
		got, err := reconcile(context.Background(), database, profile, latencyMeasurements, sourceHeight)
		if got != nil || err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s reconciliation authority: %+v, %v", label, got, err)
		}
	}
	profile.MaximumBars = 0
	assertReconcile("missing reference capacity", "measurement_reference_bar_budget_required")
	profile.MaximumBars = 6
	assertReconcile("canonical reference capacity", "measurement_reference_bar_budget_exceeded")
	profile.MaximumBars = 1000
	if _, err := database.Exec(context.Background(), `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) SELECT chain_id,$1,interval,start,snapshot FROM candles.chart_exact WHERE pool=$2 AND interval='1m' LIMIT 1`, poolB, poolA); err != nil {
		t.Fatal(err)
	}
	profile.MaximumBars = 7
	assertReconcile("actual candle count", "actual bar budget exceeded")
	profile.MaximumBars = 1000
	if _, err := database.Exec(context.Background(), `DELETE FROM candles.chart_exact WHERE pool=$1`, poolB); err != nil {
		t.Fatal(err)
	}
	limited := measurements{Latencies: boundedArtifact{file: latencyFile, budget: &rawBudget{maximum: 0}}}
	if got, err := reconcile(context.Background(), database, profile, limited, sourceHeight); got != nil || err == nil || !strings.Contains(err.Error(), "raw artifact byte budget exceeded") {
		t.Fatalf("bounded commit latency artifact: %+v, %v", got, err)
	}
	var savedSnapshot []byte
	if err = database.QueryRow(context.Background(), `SELECT snapshot FROM candles.chart_exact WHERE pool=$1 AND interval='1m' LIMIT 1`, poolA).Scan(&savedSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_exact SET snapshot='"invalid"'::jsonb WHERE pool=$1 AND interval='1m'`, poolA); err != nil {
		t.Fatal(err)
	}
	assertReconcile("unparseable candle", "unmarshal")
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_exact SET snapshot=$2 WHERE pool=$1 AND interval='1m'`, poolA, savedSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_inbox SET fill='{}'::jsonb WHERE pool=$1`, poolA); err != nil {
		t.Fatal(err)
	}
	assertReconcile("changed chart inbox", "canonical fill missing or changed")
	t.Run("reconciled source disagreement", func(t *testing.T) {
		runVariant(t, profile, false, context.Background(), "canonical fill missing or changed")
	})
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_inbox SET fill=$2 WHERE pool=$1`, poolA, fillWire); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE indexer.indexer_accounting_fills SET payload=payload-'token' WHERE transaction_hash=$1`, fill.Position.TransactionHash); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_inbox SET fill=fill-'token' WHERE pool=$1`, poolA); err != nil {
		t.Fatal(err)
	}
	assertReconcile("invalid canonical source fill", "invalid")
	if _, err = database.Exec(context.Background(), `UPDATE indexer.indexer_accounting_fills SET payload=$2 WHERE transaction_hash=$1`, fill.Position.TransactionHash, fillWire); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_inbox SET fill=$2 WHERE pool=$1`, poolA, fillWire); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_exact SET snapshot=jsonb_set(snapshot,'{synthetic}','true'::jsonb) WHERE pool=$1 AND interval='1m'`, poolA); err != nil {
		t.Fatal(err)
	}
	assertReconcile("invalid exact amount", "invalid")
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_exact SET snapshot=$2 WHERE pool=$1 AND interval='1m'`, poolA, savedSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `ALTER TABLE candles.chart_exact RENAME TO chart_exact_held`); err != nil {
		t.Fatal(err)
	}
	assertReconcile("unavailable exact candles", "chart_exact")
	if _, err = database.Exec(context.Background(), `ALTER TABLE candles.chart_exact_held RENAME TO chart_exact`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `ALTER TABLE indexer.indexer_accounting_fills RENAME TO indexer_accounting_fills_held`); err != nil {
		t.Fatal(err)
	}
	assertReconcile("unavailable canonical fills", "indexer_accounting_fills")
	if _, err = database.Exec(context.Background(), `ALTER TABLE indexer.indexer_accounting_fills_held RENAME TO indexer_accounting_fills`); err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{"indexer_raw_logs_v1", "indexer_block_completeness_v1"} {
		if _, err = database.Exec(context.Background(), `ALTER TABLE indexer.`+relation+` RENAME TO `+relation+`_held`); err != nil {
			t.Fatal(err)
		}
		assertReconcile("unavailable "+relation, relation)
		if _, err = database.Exec(context.Background(), `ALTER TABLE indexer.`+relation+`_held RENAME TO `+relation); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_exact SET snapshot=jsonb_set(snapshot,'{volumeUsdlRaw}','"999"'::jsonb) WHERE pool=$1 AND interval='1m'`, poolA); err != nil {
		t.Fatal(err)
	}
	assertReconcile("economic candle", "independent rational reference differs")
	if _, err = database.Exec(context.Background(), `UPDATE candles.chart_exact SET snapshot=$2 WHERE pool=$1 AND interval='1m'`, poolA, savedSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `INSERT INTO candles.chart_inbox(chain_id,emitter,code_epoch,tx_hash,log_index,pool,digest,fill,algorithm) SELECT chain_id,emitter,code_epoch,tx_hash||'-extra',log_index+1,pool,digest,fill,algorithm FROM candles.chart_inbox LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	assertReconcile("extra chart identity", "extra or missing accepted chart identity")
	if _, err = database.Exec(context.Background(), `DELETE FROM candles.chart_inbox WHERE tx_hash LIKE '%-extra'`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=$2 WHERE transaction_hash=$1`, fill.Position.TransactionHash, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	assertReconcile("source digest", "source fact digest disagreement")
	if _, err = database.Exec(context.Background(), `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=$2 WHERE transaction_hash=$1`, fill.Position.TransactionHash, string(fillDigest)); err != nil {
		t.Fatal(err)
	}
	assertReconcile("source commit latency", "commit latency authority absent or regressing")
	if _, err = database.Exec(context.Background(), `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=NULL WHERE transaction_hash=$1`, fill.Position.TransactionHash); err != nil {
		t.Fatal(err)
	}
	assertReconcile("unreadable source digest", "cannot scan NULL")
	if _, err = database.Exec(context.Background(), `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=$2 WHERE transaction_hash=$1`, fill.Position.TransactionHash, string(fillDigest)); err != nil {
		t.Fatal(err)
	}
	output = filepath.Join(t.TempDir(), "release-measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output, "-release"}
	if err := run(); err == nil || !strings.Contains(err.Error(), "fresh coordinated release start") {
		t.Fatalf("coordinated release bound: %v", err)
	}
	runAt := func(output, want string) {
		t.Helper()
		flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
		flag.CommandLine.SetOutput(io.Discard)
		os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
		if err := run(); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("measurement admission at %s: %v", output, err)
		}
	}
	runAt("relative-output", "absolute private output directory required")
	runAt(filepath.Join(t.TempDir(), "missing", "output"), "no such file or directory")
	runAt(output, "file exists")
	for _, file := range []string{"latencies.ndjson", "locks.ndjson"} {
		dir := filepath.Join(t.TempDir(), "measurement")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte("occupied"), 0600); err != nil {
			t.Fatal(err)
		}
		runAt(dir, "file exists")
	}
	t.Setenv("DATABASE_URL", "://")
	runAt(filepath.Join(t.TempDir(), "database"), "actual database unavailable")
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("RPC_URL", "not-a-rpc-url")
	runAt(filepath.Join(t.TempDir(), "rpc"), "actual canonical RPC unavailable")
	t.Setenv("RPC_URL", rpcURL)
	t.Setenv("NATS_URL", "nats://127.0.0.1:1")
	runAt(filepath.Join(t.TempDir(), "broker"), "actual NATS unavailable")
	t.Setenv("NATS_URL", brokerURL)
	nc, err := nats.Connect(brokerURL, nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := js.Consumer(context.Background(), profile.NATSStream, profile.NATSDurable)
	if err != nil {
		t.Fatal(err)
	}
	sampleFile, err := os.CreateTemp(t.TempDir(), "captures-*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sampleFile.Close() })
	lockFile, err := os.CreateTemp(t.TempDir(), "locks-*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lockFile.Close() })
	budget := &rawBudget{maximum: 1 << 20}
	captureOutput := measurements{Samples: boundedArtifact{file: sampleFile, budget: budget}, Locks: boundedArtifact{file: lockFile, budget: budget}}
	holder, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close(context.Background()) })
	if _, err := holder.Exec(context.Background(), `SELECT pg_advisory_lock(2239482)`); err != nil {
		t.Fatal(err)
	}
	waitConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if waitConfig.RuntimeParams == nil {
		waitConfig.RuntimeParams = map[string]string{}
	}
	waitConfig.RuntimeParams["application_name"] = "charts"
	waiter, err := pgx.ConnectConfig(context.Background(), waitConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = waiter.Close(context.Background()) })
	t.Cleanup(func() { _, _ = holder.Exec(context.Background(), `SELECT pg_advisory_unlock(2239482)`) })
	waitResult := make(chan error, 1)
	go func() {
		_, waitErr := waiter.Exec(context.Background(), `SELECT pg_advisory_lock(2239482)`)
		waitResult <- waitErr
	}()
	seenLock := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		var waiting int
		if err := database.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE application_name='charts' AND wait_event_type='Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			seenLock = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !seenLock {
		t.Fatal("database did not report the owned lock waiter")
	}
	lockedObservation, err := capture(context.Background(), database, profile, broker, 0, captureOutput)
	if err != nil || lockedObservation.WaitingBackends < 1 {
		t.Fatalf("observed database lock wait: %+v, %v", lockedObservation, err)
	}
	lockRows, err := os.ReadFile(lockFile.Name())
	var lockEvidence struct {
		Application string `json:"application"`
		WaitType    string `json:"waitType"`
	}
	if err != nil || json.Unmarshal(bytes.TrimSpace(lockRows), &lockEvidence) != nil || lockEvidence.Application != "charts" || lockEvidence.WaitType != "Lock" {
		t.Fatalf("captured database lock evidence: %s, %v", lockRows, err)
	}
	boundedLocks := measurements{Samples: captureOutput.Samples, Locks: boundedArtifact{file: lockFile, budget: &rawBudget{maximum: 0}}}
	if _, err := capture(context.Background(), database, profile, broker, 0, boundedLocks); err == nil || !strings.Contains(err.Error(), "raw artifact byte budget exceeded") {
		t.Fatalf("bounded database lock artifact: %v", err)
	}
	var released bool
	if err := holder.QueryRow(context.Background(), `SELECT pg_advisory_unlock(2239482)`).Scan(&released); err != nil || !released {
		t.Fatalf("released database lock: %t, %v", released, err)
	}
	select {
	case err := <-waitResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owned lock waiter did not resume")
	}
	checkCapture := func(want string) {
		t.Helper()
		observation, captureErr := capture(context.Background(), database, profile, broker, 0, captureOutput)
		if captureErr == nil || !strings.Contains(captureErr.Error(), want) || observation.SourceHeight != sourceHeight {
			t.Fatalf("canonical capture authority: %+v, %v", observation, captureErr)
		}
	}
	if observed, err := capture(context.Background(), database, profile, broker, 0, captureOutput); err != nil || observed.SourceHeight != sourceHeight || observed.ChartWatermark != sourceHeight || observed.AcceptedFills < 1 {
		t.Fatalf("complete canonical capture: %+v, %v", observed, err)
	}
	if _, err := database.Exec(context.Background(), `UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=$1 WHERE stream_id='canonical'`, sourceHeight+1); err != nil {
		t.Fatal(err)
	}
	checkCapture("complete canonical source and live fence required")
	if _, err := database.Exec(context.Background(), `UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=$1 WHERE stream_id='canonical'`, sourceHeight); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `UPDATE indexer.indexer_ingest_leases_v1 SET expires_at=clock_timestamp()-interval '1 second' WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	checkCapture("complete canonical source and live fence required")
	if _, err := database.Exec(context.Background(), `UPDATE indexer.indexer_ingest_leases_v1 SET expires_at=clock_timestamp()+interval '1 hour' WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	otherPool := "0x" + strings.Repeat("c", 40)
	if _, err := database.Exec(context.Background(), `UPDATE candles.chart_series_clock SET pool=$2 WHERE pool=$1`, poolB, otherPool); err != nil {
		t.Fatal(err)
	}
	checkCapture("all observed series clocks must exist")
	if _, err := database.Exec(context.Background(), `UPDATE candles.chart_series_clock SET pool=$2 WHERE pool=$1`, otherPool, poolB); err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{"chart_series_clock", "chart_inbox", "chart_stream_sources", "chart_outbox", "chart_rollup_pending", "chart_pool_fences", "chart_delivery_attempts"} {
		if _, err := database.Exec(context.Background(), `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
			t.Fatal(err)
		}
		checkCapture(relation)
		if _, err := database.Exec(context.Background(), `ALTER TABLE candles.`+relation+`_held RENAME TO `+relation); err != nil {
			t.Fatal(err)
		}
	}
	tooSmall := measurements{Samples: boundedArtifact{file: sampleFile, budget: &rawBudget{}}, Locks: captureOutput.Locks}
	if _, err := capture(context.Background(), database, profile, broker, 0, tooSmall); err == nil || !strings.Contains(err.Error(), "raw artifact byte budget exceeded") {
		t.Fatalf("capture artifact bound: %v", err)
	}
	archiveReader := store.ArchiveReader{Objects: objects, Bucket: objects.Bucket(), Exact: exact, BlockTime: func(ctx context.Context, height int64, expected string) (int64, error) {
		block, err := (chainread.Reader{RPC: chain.RPC}).Block(ctx, height)
		if err != nil {
			return 0, err
		}
		if block.Hash != expected {
			return 0, store.ErrIntegrity
		}
		return block.Time, nil
	}}
	staging := profile.Retention
	staging.StagingDeletionAdmitted = true
	staging.DeleteStagingObject = manifest.ObjectKey
	if removed, err := verifyStagingArchive(context.Background(), false, staging, objects, archiveReader, exact, []store.ArchiveManifest{manifest}); err != nil || removed {
		t.Fatalf("unrequested archive mutation: removed=%t err=%v", removed, err)
	}
	if removed, err := verifyStagingArchive(context.Background(), true, profile.Retention, objects, archiveReader, exact, []store.ArchiveManifest{manifest}); err != nil || removed {
		t.Fatalf("unadmitted archive mutation: removed=%t err=%v", removed, err)
	}
	missing := staging
	missing.DeleteStagingObject = ""
	if removed, err := verifyStagingArchive(context.Background(), true, missing, objects, archiveReader, exact, []store.ArchiveManifest{manifest}); err == nil || removed || !strings.Contains(err.Error(), "mutation authority") {
		t.Fatalf("archive deletion without object identity: removed=%t err=%v", removed, err)
	}
	missing.DeleteStagingObject = "indexer/raw/v2/canonical/unlisted.json"
	if removed, err := verifyStagingArchive(context.Background(), true, missing, objects, archiveReader, exact, []store.ArchiveManifest{manifest}); err == nil || removed || !strings.Contains(err.Error(), "verified staging object") {
		t.Fatalf("unlisted archive deletion: removed=%t err=%v", removed, err)
	}
	if removed, err := verifyStagingArchive(context.Background(), true, staging, objects, archiveReader, exact, []store.ArchiveManifest{manifest}); err != nil || !removed {
		t.Fatalf("retention after staging object deletion: removed=%t err=%v", removed, err)
	}
	if object, err := objects.Get(context.Background(), manifest.ObjectKey); !errors.Is(err, storage.ErrNotFound) || object != nil {
		t.Fatalf("deleted staging archive read: object=%v err=%v", object, err)
	}
	if _, err := database.Exec(context.Background(), `DELETE FROM indexer.indexer_accounting_fills WHERE transaction_hash=$1`, fill.Position.TransactionHash); err != nil {
		t.Fatal(err)
	}
	assertReconcile("empty canonical range", "actual canonical fills required")
	nc.Close()
	if _, err := capture(context.Background(), database, profile, broker, 0, captureOutput); err == nil || !strings.Contains(err.Error(), "connection closed") {
		t.Fatalf("closed broker observation: %v", err)
	}
	database.Close()
	if result, err := reconcile(context.Background(), database, profile, latencyMeasurements, sourceHeight); result != nil || err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("closed canonical measurement: %+v, %v", result, err)
	}
}
