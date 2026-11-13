package processor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMeasurementRejectsAbsentActualProviders(t *testing.T) {
	repairM1000MeasurementRefusals(t)
}
func TestMeasurementRejectsChangedCandidate(t *testing.T) {
	repairM1000ChangedCandidate(t)
}
func TestMeasurementStorageAndTraceUseActualPostgres(t *testing.T) {
	ctx, dsn, pool := replayPostgres(t)
	batch := prepare(t, pool)
	ddl, err := delivery.SchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	trace := new(IngestSQLMeasurementTracer)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Tracer = trace
	observed, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer observed.Close()
	before, err := store.New(observed).MeasureIngestStorage(ctx, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	if err = process(ctx, observed, batch); err != nil {
		t.Fatal(err)
	}
	after, err := store.New(observed).MeasureIngestStorage(ctx, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	wal, err := store.WALBytesBetween(before.WALInsertLSN, after.WALInsertLSN)
	if err != nil || wal == 0 {
		t.Fatalf("actual PostgreSQL WAL not measured: %d %v", wal, err)
	}
	counters := trace.Snapshot()
	if counters.QueryCalls == 0 || counters.RawScalarInsertQueries != 4 || counters.CopyCalls != 0 {
		t.Fatalf("actual scalar ingest trace: %+v", counters)
	}
	digest, err := store.New(observed).MeasureCanonicalRange(ctx, "live", 101, 101)
	if err != nil || digest.RawCount != 4 || digest.TypedCount != 3 || digest.ProjectionCount != 3 || len(digest.ProjectionSHA256) != 64 || digest.BlockCount != 1 || len(digest.RawSHA256) != 64 || len(digest.TypedSHA256) != 64 {
		t.Fatalf("actual raw/typed reconciliation: %+v %v", digest, err)
	}
	fresh, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	restarted, err := store.New(fresh).MeasureCanonicalRange(ctx, "live", 101, 101)
	if err != nil || restarted != digest {
		t.Fatalf("fresh database connection digest changed: %+v %v", restarted, err)
	}
	plans, err := store.New(fresh).MeasureIngestPlans(ctx, "live", batch.Records[0], "0x"+strings.Repeat("2", 40))
	if err != nil || len(plans) != 5 {
		t.Fatalf("actual query plans missing: %d %v", len(plans), err)
	}
	for _, plan := range plans {
		if len(plan.Plan) == 0 {
			t.Fatal("empty PostgreSQL EXPLAIN plan")
		}
	}
	if after.RawPartitioned {
		t.Fatal("unpartitioned actual fixture falsely reported partition support")
	}
}
func TestMeasurementHistogramUsesEventWeightsAndTail(t *testing.T) {
	var h measurementHistogram
	h.add(400*time.Millisecond, 99)
	h.add(1200*time.Millisecond, 2)
	if h.p99() != 1200 {
		t.Fatalf("weighted tail hidden: %d", h.p99())
	}
	h.add(time.Hour, 1000)
	if h.p99() != 60001 {
		t.Fatal("overflow tail lost")
	}
	if _, err := store.WALBytesBetween("1/100", "0/FFFF"); err == nil {
		t.Fatal("restored WAL timeline regression counted as throughput")
	}
}
func TestRestoreCheckpointRejectsUnboundOrMalformedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(path, []byte(`{"CandidateSHA256":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRestoreCheckpoint(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong checkpoint digest accepted")
	}
	if _, err := (&IngestMeasurementHarness{}).Reconstruct(context.Background(), IngestReconstructionSpec{}); err == nil {
		t.Fatal("absent archive/restore provider accepted")
	}
}

func TestMeasurementCaptureBindsWrittenRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.ndjson")
	if _, err := openMeasurementCapture(path, 0); err == nil {
		t.Fatal("capture accepted an absent storage budget")
	}
	capture, err := openMeasurementCapture(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.write("sample", map[string]int{"height": 101}); err != nil {
		t.Fatal(err)
	}
	if err := capture.write("large", strings.Repeat("x", 100)); err == nil {
		t.Fatal("capture accepted a record beyond the storage budget")
	}
	if err := capture.file.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Kind  string
		Value map[string]int
	}
	if err := json.Unmarshal(body, &row); err != nil || row.Kind != "sample" || row.Value["height"] != 101 {
		t.Fatalf("captured record: %+v %v", row, err)
	}
	if digest := hex.EncodeToString(capture.hash.(interface{ Sum([]byte) []byte }).Sum(nil)); digest != delivery.Hash(body) {
		t.Fatal("capture digest differs from persisted records")
	}
	if _, err := openMeasurementCapture(path, 100); !os.IsExist(err) {
		t.Fatalf("capture replaced a prior record: %v", err)
	}
}

func TestMeasurementArtifactsRequireExactBindings(t *testing.T) {
	spec := repairM1000MeasurementSpec(t)
	if err := validateMeasurementSource(spec); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*IngestMeasurementSpec)
	}{
		{"topology region", func(s *IngestMeasurementSpec) { s.Region = "different" }},
		{"candidate digest", func(s *IngestMeasurementSpec) { s.CandidateSHA256 = strings.Repeat("0", 64) }},
		{"steady duration", func(s *IngestMeasurementSpec) { s.Duration = time.Second }},
		{"burst duration", func(s *IngestMeasurementSpec) { s.Profile = "burst"; s.Duration = 4 * time.Minute }},
		{"soak duration", func(s *IngestMeasurementSpec) { s.Profile = "soak" }},
		{"unknown profile", func(s *IngestMeasurementSpec) { s.Profile = "other" }},
		{"missing source", func(s *IngestMeasurementSpec) { s.SourceFiles = nil }},
		{"missing topology", func(s *IngestMeasurementSpec) { s.TopologyPath = "" }},
		{"capture budget", func(s *IngestMeasurementSpec) { s.CaptureMaxBytes = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := spec
			tc.edit(&changed)
			if err := validateMeasurementSource(changed); err == nil {
				t.Fatal("unbound measurement input accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	checkpoint := IngestRestoreCheckpoint{CandidateSHA256: spec.CandidateSHA256, CapturedAt: time.Now().UTC(), Storage: store.IngestStorageSample{Database: "indexer"}, Canonical: store.CanonicalRangeDigest{RawSHA256: strings.Repeat("1", 64), TypedSHA256: strings.Repeat("2", 64)}}
	body, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadRestoreCheckpoint(path, delivery.Hash(body))
	if err != nil || loaded.CandidateSHA256 != checkpoint.CandidateSHA256 || loaded.Storage.Database != checkpoint.Storage.Database {
		t.Fatalf("checkpoint binding: %+v %v", loaded, err)
	}
	if _, err := loadRestoreCheckpoint(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("changed checkpoint digest accepted")
	}
	if err := os.WriteFile(path, append(body, []byte("{}")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRestoreCheckpoint(path, delivery.Hash(append(body, []byte("{}")...))); err == nil {
		t.Fatal("trailing checkpoint value accepted")
	}
}

func TestMeasurementSQLCopyTraceCountsRowsAndFailures(t *testing.T) {
	trace := new(IngestSQLMeasurementTracer)
	ctx := trace.TraceCopyFromStart(context.Background(), nil, pgx.TraceCopyFromStartData{})
	trace.TraceCopyFromEnd(ctx, nil, pgx.TraceCopyFromEndData{CommandTag: pgconn.NewCommandTag("COPY 3")})
	trace.TraceCopyFromEnd(ctx, nil, pgx.TraceCopyFromEndData{CommandTag: pgconn.NewCommandTag("COPY 2")})
	trace.TraceCopyFromEnd(ctx, nil, pgx.TraceCopyFromEndData{Err: errors.New("copy rejected")})
	got := trace.Snapshot()
	if got.CopyCalls != 1 || got.CopyRows != 5 || got.MaxCopyRows != 3 || got.QueryFailures != 1 {
		t.Fatalf("copy trace counters: %+v", got)
	}
}
