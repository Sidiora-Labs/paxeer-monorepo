package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type measuredHead struct {
	height  atomic.Int64
	failure atomic.Bool
}

func (h *measuredHead) Head(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if h.failure.Load() {
		return 0, store.ErrIntegrity
	}
	return h.height.Load(), nil
}
func (h *measuredHead) Name() string { return "committed-input" }
func (h *measuredHead) Close() error { return nil }

type lifecycleCoverageHarness struct {
	*IngestMeasurementHarness
	inventory inventoryHarness
	head      *measuredHead
}

func newLifecycleCoverageHarness(t *testing.T) lifecycleCoverageHarness {
	t.Helper()
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	inventory := newInventoryHarnessOnPool(t, ctx, 0, pool, 33963749)
	trace := new(IngestSQLMeasurementTracer)
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = trace
	observed, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(observed.Close)
	guard, err := queue.NewEpochGuard("recovery", 1, "railway-prod")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewLifecycleStore(observed, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{ReplaySubjectV2: true}, EventTypes: map[string]bool{ReplaySubjectV2: true}}
	outbox, err := delivery.NewStoreWithEpochGuard(observed, "indexer", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	inventory.proc.deps.Store = st
	head := new(measuredHead)
	head.height.Store(33963749)
	inventory.proc.deps.Head = head
	inventory.proc.deps.PollInterval = time.Millisecond
	live := inventory.proc.deps.QualifiedBatchSource.(*source.LiveQualifiedSource)
	historical := &source.HistoricalQualifiedSource{History: &source.HistoricalSource{RPC: live.RPC}, Authority: live.Authority, DurableHeight: inventory.height.Load}
	inventory.proc.deps.QualifiedBatchSource = historical
	runner := &ReplayRunner{Store: st, Source: historical, Deliveries: outbox, Owner: inventoryTestOwner, DeliverySchema: "indexer", BatchSize: 17}
	return lifecycleCoverageHarness{IngestMeasurementHarness: &IngestMeasurementHarness{SQLTrace: trace, Processor: inventory.proc, Pool: observed, Store: st, Replay: runner}, inventory: inventory, head: head}
}
func (h lifecycleCoverageHarness) spec(t *testing.T) IngestMeasurementSpec {
	t.Helper()
	spec := repairM1000MeasurementSpec(t)
	var database string
	if err := h.Pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&database); err != nil {
		t.Fatal(err)
	}
	topology := IngestTopologyEvidence{Region: spec.Region, Database: database, ApprovedAuthorityID: "committed-test-topology", CapturedAt: time.Now().UTC(), Zones: []string{"a", "b", "c"}, NodeIDs: []string{"n1", "n2", "n3"}}
	body, err := json.Marshal(topology)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(spec.TopologyPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	spec.TopologySHA256 = delivery.Hash(body)
	spec.HotPool = realMarketPool
	spec.BatchBlocks = 17
	spec.MaxDatabaseConnections = 6
	return spec
}
func authorizeCoverageReplay(t *testing.T, h lifecycleCoverageHarness, mode, id string, from, to int64) store.ReplayRequest {
	t.Helper()
	ctx := context.Background()
	batch, _, err := h.Replay.Source.FetchQualifiedBatch(ctx, uint64(to), uint64(to))
	if err != nil {
		t.Fatal(err)
	}
	request := store.ReplayRequest{JobID: id, SourceStream: realStream, Target: "charts", Generation: "g1", ProjectionRevision: "revision2", Mode: mode, Requester: "operator", AuthorizationID: id, FromHeight: from, ToHeight: to, CheckpointHeight: to, CheckpointHash: batch.Blocks[0].Hash}
	if _, err = h.Pool.Exec(ctx, `INSERT INTO indexer.indexer_replay_authorizations VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,clock_timestamp()+interval '1 hour',NULL)`, id, request.Requester, strings.Repeat("a", 64), mode, request.Target, request.Generation, request.ProjectionRevision, from, to); err != nil {
		t.Fatal(err)
	}
	return request
}
func archiveCoverageObjects(t *testing.T) *storage.Client {
	t.Helper()
	directory := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relative := strings.TrimPrefix(r.URL.Path, "/")
		if strings.Contains(relative, "..") {
			http.Error(w, "invalid object key", http.StatusBadRequest)
			return
		}
		path := filepath.Join(directory, filepath.FromSlash(relative))
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20+1))
			if err != nil || len(body) > 64<<20 {
				http.Error(w, "bounded object required", http.StatusBadRequest)
				return
			}
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Error(err)
				http.Error(w, "storage unavailable", 500)
				return
			}
			if err = os.WriteFile(path, body, 0600); err != nil {
				t.Error(err)
				http.Error(w, "storage unavailable", 500)
				return
			}
			w.Header().Set("ETag", `"`+delivery.Hash(body)+`"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet, http.MethodHead:
			body, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			if err != nil {
				t.Error(err)
				http.Error(w, "storage unavailable", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("ETag", `"`+delivery.Hash(body)+`"`)
			if r.Method == http.MethodGet {
				if _, err = w.Write(body); err != nil {
					t.Error(err)
				}
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	client, err := storage.New(context.Background(), storage.Config{Endpoint: server.URL, Region: "auto", Bucket: "archive", AccessKeyID: uuid.NewString(), SecretAccessKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestMeasurementCapturesQualifiedBatchesAndShutdown(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	spec := h.spec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	report, err := h.Run(ctx, spec)
	if !errors.Is(err, context.DeadlineExceeded) || report.Transactions != 2 || report.CommittedBlocks != 34 || report.TotalFills != 2 || report.HotPoolFills != 2 || report.CommittedRaw != report.OfferedRaw || report.CommittedRaw == 0 || report.PeakGoHeapBytes == 0 || report.PeakRSSBytes == 0 || report.SQLTrace.CopyCalls != 0 || report.SQLTrace.RawScalarInsertQueries != uint64(report.CommittedRaw) || report.CaptureSHA256 == "" || report.FinishedAt.IsZero() || report.StartedAt.IsZero() {
		t.Fatalf("qualified measurement: %+v %v", report, err)
	}
	if !slices.Contains(report.Violations, "actual ingest executed no COPY staging") || !slices.Contains(report.Violations, "actual ingest used one scalar raw insert per event") {
		t.Fatalf("scalar ingestion was not reported: %v", report.Violations)
	}
	body, err := os.ReadFile(spec.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"input", "source-batch", "durable-ack", "resources", "query-plans", "report"} {
		if !bytes.Contains(body, []byte(`"kind":"`+kind+`"`)) {
			t.Fatalf("capture lacks %s", kind)
		}
	}
	if ready := h.Processor.Ready(context.Background()); ready == nil || h.Processor.readyFence.Load() != 0 {
		t.Fatalf("released measurement remains ready: %v", ready)
	}
	watermark, err := h.Store.GetCanonicalWatermark(context.Background(), 125, realStream)
	if err != nil || watermark != 33963749 {
		t.Fatalf("measurement watermark %d: %v", watermark, err)
	}
}

func TestArchiveReplayAndIndependentReconstruction(t *testing.T) {
	ctx := context.Background()
	original := newLifecycleCoverageHarness(t)
	original.inventory.batch(t, ctx, 33963716, 33963716)
	spec := original.spec(t)
	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	checkpoint, err := original.CaptureRestoreCheckpoint(ctx, spec, 33963716, 33963716, checkpointPath)
	if err != nil || checkpoint.Canonical.RawCount == 0 || checkpoint.Storage.Database == "" {
		t.Fatalf("source checkpoint: %+v %v", checkpoint, err)
	}
	checkpointBody, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = original.CaptureRestoreCheckpoint(ctx, spec, 33963716, 33963716, checkpointPath); !os.IsExist(err) {
		t.Fatalf("checkpoint replaced: %v", err)
	}
	original.inventory.batch(t, ctx, 33963717, 33963733)
	original.inventory.batch(t, ctx, 33963734, 33963749)
	objects := archiveCoverageObjects(t)
	archive := &Archive{Store: original.Store, Source: original.Replay.Source, Objects: objects}
	manifest, err := archive.Produce(ctx, realStream, 33963717, 33963749)
	if err != nil || manifest.TypedCount < 2 || manifest.EventCount < 2 {
		t.Fatalf("archive production: %+v %v", manifest, err)
	}
	segment, err := archive.Restore(ctx, realStream, 33963717, 33963749)
	if err != nil || len(segment.Records) != manifest.EventCount || len(segment.Events) != manifest.TypedCount {
		t.Fatalf("archive restore: %+v %v", segment, err)
	}
	for _, bounds := range [][2]int64{{33963717, 33963733}, {33963734, 33963749}} {
		subset, err := archive.Restore(ctx, realStream, bounds[0], bounds[1])
		if err != nil || subset.FromHeight != bounds[0] || subset.ToHeight != bounds[1] || len(subset.Blocks) != int(bounds[1]-bounds[0]+1) {
			t.Fatalf("archive subset: %+v %v", subset, err)
		}
	}
	restored := newLifecycleCoverageHarness(t)
	restored.inventory.batch(t, ctx, 33963716, 33963716)
	restored.Archive = &Archive{Store: restored.Store, Source: restored.Replay.Source, Objects: objects}
	restored.Replay.Archive = restored.Archive
	var manifestBody []byte
	var manifestDigest, objectKey string
	if err = original.Pool.QueryRow(ctx, `SELECT manifest,manifest_digest,object_key FROM indexer.indexer_archive_segments WHERE from_height=33963717`).Scan(&manifestBody, &manifestDigest, &objectKey); err != nil {
		t.Fatal(err)
	}
	if _, err = restored.Pool.Exec(ctx, `INSERT INTO indexer.indexer_archive_segments(segment_id,chain_id,source_stream,from_height,to_height,manifest,manifest_digest,object_key) VALUES($1,125,$2,33963717,33963749,$3,$1,$4)`, manifestDigest, realStream, manifestBody, objectKey); err != nil {
		t.Fatal(err)
	}
	request := authorizeCoverageReplay(t, restored, "reconstruct", "restore", 33963717, 33963749)
	duplicate := authorizeCoverageReplay(t, restored, "reconstruct", "restore-again", 33963717, 33963749)
	reconstruction := IngestReconstructionSpec{RunID: "archive-restore", CandidateSHA256: spec.CandidateSHA256, OutputPath: filepath.Join(t.TempDir(), "restore.ndjson"), RecordedCheckpointPath: checkpointPath, RecordedCheckpointSHA256: delivery.Hash(checkpointBody), DeliverySchema: "indexer", CaptureMaxBytes: 16 << 20, Request: request, DuplicateRequest: duplicate}
	for _, tc := range []struct {
		name string
		edit func(*IngestMeasurementHarness, *IngestReconstructionSpec, *IngestRestoreCheckpoint)
		want string
	}{
		{"runner owner", func(h *IngestMeasurementHarness, _ *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			h.Replay.Owner = ""
		}, "bounded owner"},
		{"outbox schema", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.DeliverySchema = "other"
		}, "schema differs"},
		{"request", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.Request.SourceStream = ""
		}, ""},
		{"duplicate request", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.DuplicateRequest.SourceStream = ""
		}, ""},
		{"mode", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.Request.Mode = "replay"
		}, "distinct authorized"},
		{"same job", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.DuplicateRequest.JobID = s.Request.JobID
		}, "distinct authorized"},
		{"generation", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.DuplicateRequest.Generation += "-other"
		}, "identical original facts"},
		{"candidate", func(_ *IngestMeasurementHarness, _ *IngestReconstructionSpec, c *IngestRestoreCheckpoint) {
			c.CandidateSHA256 = strings.Repeat("1", 64)
		}, "bind this candidate"},
		{"same database", func(h *IngestMeasurementHarness, _ *IngestReconstructionSpec, c *IngestRestoreCheckpoint) {
			if e := h.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&c.Storage.Database); e != nil {
				t.Fatal(e)
			}
		}, "independently restored"},
		{"canonical digest", func(_ *IngestMeasurementHarness, _ *IngestReconstructionSpec, c *IngestRestoreCheckpoint) {
			c.Canonical.RawSHA256 = strings.Repeat("1", 64)
		}, ""},
		{"output parent", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.OutputPath = filepath.Join(t.TempDir(), "missing", "capture")
		}, ""},
		{"capture input budget", func(_ *IngestMeasurementHarness, s *IngestReconstructionSpec, _ *IngestRestoreCheckpoint) {
			s.CaptureMaxBytes = 1
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := *restored.IngestMeasurementHarness
			runner := *h.Replay
			h.Replay = &runner
			s := reconstruction
			s.OutputPath = filepath.Join(t.TempDir(), "capture.ndjson")
			c := checkpoint
			tc.edit(&h, &s, &c)
			body, e := json.Marshal(c)
			if e != nil {
				t.Fatal(e)
			}
			s.RecordedCheckpointPath = filepath.Join(t.TempDir(), "checkpoint.json")
			if e = os.WriteFile(s.RecordedCheckpointPath, body, 0600); e != nil {
				t.Fatal(e)
			}
			s.RecordedCheckpointSHA256 = delivery.Hash(body)
			r, e := h.Reconstruct(ctx, s)
			if e == nil || (tc.want != "" && !strings.Contains(e.Error(), tc.want)) || r.DuplicateReplayMatched {
				t.Fatalf("reconstruction admission %+v: %v", r, e)
			}
		})
	}
	report, err := restored.Reconstruct(ctx, reconstruction)
	if err != nil || !report.CheckpointMatched || !report.ArchiveOlderThan72Hours || !report.DuplicateReplayMatched || report.RestoredRaw != int64(manifest.EventCount) || report.RestoredTyped != int64(manifest.TypedCount) || len(report.SegmentDigests) != 2 || report.FinishedAt.IsZero() || report.InfrastructureRPOOrRTOCertified || report.TrafficActivationAuthorized {
		t.Fatalf("archive reconstruction: %+v %v", report, err)
	}
	if err = restored.verifyReconstructionOutbox(ctx, request, segment); err != nil {
		t.Fatal(err)
	}
	digest, err := restored.reconstructionEffects(ctx, request)
	if err != nil || len(digest) != 64 {
		t.Fatalf("effective projection digest %q: %v", digest, err)
	}
	if err = restored.Replay.Run(ctx, request); err != nil {
		t.Fatalf("completed reconstruction: %v", err)
	}
	var status string
	var effects int
	if err = restored.Pool.QueryRow(ctx, `SELECT status,(SELECT count(*) FROM indexer.indexer_replay_effects) FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status, &effects); err != nil || status != "completed" || effects != manifest.TypedCount {
		t.Fatalf("reconstruction rows %s/%d: %v", status, effects, err)
	}
}

func TestReplayGenerationMatchesQualifiedSource(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	h.inventory.batch(t, ctx, 33963717, 33963733)
	h.inventory.batch(t, ctx, 33963734, 33963749)
	request := authorizeCoverageReplay(t, h, "reconstruct", "generation", 33963733, 33963733)
	projection, err := NewGenerationReplayProjection(h.Replay, request)
	if err != nil {
		t.Fatal(err)
	}
	batch, receipts, err := h.Replay.Source.FetchQualifiedBatch(ctx, 33963733, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	segment, err := qualifiedSegment(ctx, request.SourceStream, batch, receipts)
	if err != nil || len(segment.Events) == 0 {
		t.Fatalf("qualified replay segment: %+v %v", segment, err)
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, raw := range segment.Records {
		archiveRaw := delivery.ArchiveRaw{Height: raw.Height, TransactionHash: raw.TransactionHash, BlockHash: raw.BlockHash, Emitter: raw.Emitter, TransactionIndex: raw.TransactionIndex, LogIndex: raw.LogIndex, Topics: raw.Topics, Data: raw.Data}
		if err = projection.ApplyRawTx(ctx, tx, request.Generation, archiveRaw); err != nil {
			t.Fatal(err)
		}
		if err = projection.ApplyRawTx(ctx, tx, request.Generation, archiveRaw); err != nil {
			t.Fatalf("idempotent generation row: %v", err)
		}
	}
	actual, err := projection.ProjectionDigestTx(ctx, tx, request.Generation)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := generationDigest(generationFacts(segment))
	if err != nil || actual != expected {
		t.Fatalf("source/projection digest %s/%s: %v", actual, expected, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"backfill", "replay"} {
		r := authorizeCoverageReplay(t, h, mode, "job-"+mode, 33963733, 33963733)
		var runErr error
		if mode == "backfill" {
			h.Processor.deps.HistoricalReplay = h.Replay
			h.Processor.deps.HistoricalRequest = &r
			from := r.FromHeight
			runErr = NewBackfill(h.Processor, &from, 17).Run(ctx)
		} else if mode == "replay" {
			runErr = h.Replay.RecoverMissedRange(ctx, r)
		} else {
			runErr = h.Replay.Run(ctx, r)
		}
		if runErr != nil {
			t.Fatalf("%s replay: %v", mode, runErr)
		}
		var status string
		if err = h.Pool.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, r.JobID).Scan(&status); err != nil || status != "completed" {
			t.Fatalf("%s replay state %s: %v", mode, status, err)
		}
	}
}

func TestInfrastructureObservationPinsCaptureAndReads(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	spec := h.spec(t)
	path := filepath.Join(t.TempDir(), "observation.json")
	body := []byte(`{"event":"node shutdown"}`)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	observation, err := h.ObserveInfrastructureFailure(ctx, spec, path, delivery.Hash(body), 33963716, 33963716)
	if err != nil || !observation.FailureDriverEvidenceExternallyAsserted || !observation.Ready || observation.Storage == nil || observation.Canonical == nil || len(observation.ActualReadErrors) != 0 || observation.InfrastructureRPOOrRTOCertified {
		t.Fatalf("infrastructure observation: %+v %v", observation, err)
	}
	if _, err = h.ObserveInfrastructureFailure(ctx, spec, path, strings.Repeat("0", 64), 33963716, 33963716); err == nil {
		t.Fatal("changed observation digest accepted")
	}
	observation, err = h.ObserveInfrastructureFailure(ctx, spec, path, delivery.Hash(body), 33963715, 33963716)
	if err != nil || len(observation.ActualReadErrors) != 1 || observation.Canonical != nil {
		t.Fatalf("incomplete canonical observation: %+v %v", observation, err)
	}
	h.Pool.Close()
	observation, err = h.ObserveInfrastructureFailure(ctx, spec, path, delivery.Hash(body), 33963716, 33963716)
	if err != nil || observation.Ready || len(observation.ActualReadErrors) != 2 || observation.ReadinessError == "" {
		t.Fatalf("unavailable database observation: %+v %v", observation, err)
	}
}

func TestProcessorValueAndInputBounds(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want int64
	}{{"42", 42}, {int64(42), 42}, {int(42), 42}, {uint8(42), 42}, {uint16(42), 42}, {uint32(42), 42}, {uint64(42), 42}, {big.NewInt(42), 42}} {
		got, err := asInt64(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("coercion %T: %d %v", tc.in, got, err)
		}
	}
	if _, err := asInt64(struct{}{}); err == nil {
		t.Fatal("unsupported integer accepted")
	}
	for _, tc := range []struct {
		in   any
		want bool
	}{{true, true}, {false, false}, {big.NewInt(1), true}, {big.NewInt(0), false}, {"true", false}} {
		if got := asBool(tc.in); got != tc.want {
			t.Fatalf("boolean %v: %v", tc.in, got)
		}
	}
	for _, n := range []any{-1, uint64(1) << 32, "bad"} {
		if _, err := chainLaunchNumber(map[string]any{"chain": n}); err == nil {
			t.Fatalf("registry number %v accepted", n)
		}
	}
	for _, name := range []string{"ChainLaunched", "ChainRevenueEpochOpened", "ChainRevenueClaimed", "LaunchedChainRetired"} {
		p := New(Deps{})
		event, err := p.processEvent(context.Background(), &decode.Event{EventName: name, Args: map[string]any{"chain": "bad"}}, 1, nil)
		if err == nil || event != nil {
			t.Fatalf("%s accepted invalid chain", name)
		}
	}
	if actors := unknownLegacyActors("invalid"); actors.EventCaller != nil || actors.AttributionConfidence != "unknown" {
		t.Fatalf("legacy actors %+v", actors)
	}
	if err := new(Processor).Ready(context.Background()); err == nil {
		t.Fatal("unconfigured ingestion ready")
	}
	if _, err := qualifiedSegment(context.Background(), "canonical", store.CanonicalBatch{}, nil); err == nil {
		t.Fatal("missing complete blocks accepted")
	}
	if _, err := new(Archive).Produce(context.Background(), "canonical", 0, 1); err == nil {
		t.Fatal("missing archive providers accepted")
	}
	if _, err := new(Archive).Restore(context.Background(), "canonical", 0, 1); err == nil {
		t.Fatal("missing restore providers accepted")
	}
	if _, err := new(Archive).readVerified(context.Background(), store.ArchiveManifest{}); err == nil {
		t.Fatal("missing archive reader accepted")
	}
	if err := NewBackfill(nil, nil, 0).Run(context.Background()); err == nil {
		t.Fatal("missing historical replay accepted")
	}
	if err := new(ReplayRunner).RecoverMissedRange(context.Background(), store.ReplayRequest{}); err == nil {
		t.Fatal("implicit missed-range operation accepted")
	}
	if _, err := NewGenerationReplayProjection(nil, store.ReplayRequest{}); err == nil {
		t.Fatal("missing generation providers accepted")
	}
	var histogram measurementHistogram
	histogram.add(-time.Second, 3)
	histogram.add(time.Second, 0)
	if histogram.p99() != 0 {
		t.Fatal("negative duration escaped first bin")
	}
}

func TestTopologyAndCheckpointInputsAreStrict(t *testing.T) {
	spec := repairM1000MeasurementSpec(t)
	for _, tc := range []struct {
		name string
		body []byte
	}{{"unknown", []byte(`{"Other":1}`)}, {"trailing", []byte(`{} {}`)}, {"missing", []byte(`{}`)}, {"malformed", []byte(`{`)}, {"duplicate zones", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			if body == nil {
				body, _ = json.Marshal(IngestTopologyEvidence{Region: "local", Database: "indexer", ApprovedAuthorityID: "test", CapturedAt: time.Now(), Zones: []string{"a", "a", "b"}, NodeIDs: []string{"n1", "n2", "n3"}})
			}
			path := filepath.Join(t.TempDir(), "topology.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readTopology(path, delivery.Hash(body)); err == nil {
				t.Fatal("invalid topology accepted")
			}
		})
	}
	for _, profile := range []struct {
		name     string
		duration time.Duration
	}{{"burst", 5 * time.Minute}, {"soak", 24 * time.Hour}} {
		changed := spec
		changed.Profile = profile.name
		changed.Duration = profile.duration
		if err := validateMeasurementSource(changed); err != nil {
			t.Fatalf("%s bounds: %v", profile.name, err)
		}
	}
	directory := t.TempDir()
	if err := validateCandidateFiles(map[string]string{directory: strings.Repeat("1", 64)}, strings.Repeat("2", 64)); err == nil {
		t.Fatal("directory accepted as source")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleep(ctx, time.Hour) {
		t.Fatal("cancelled wait completed")
	}
	if !sleep(context.Background(), time.Nanosecond) {
		t.Fatal("elapsed wait cancelled")
	}
}

func TestReplayPayloadPreservesLargeIntegers(t *testing.T) {
	raw := fixtureBatch(t).Records[0]
	digest, err := store.RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"value":1606938044258990275541962092341162602522202993782792835301377}`)
	event := store.ReplayEvent{RawDigest: digest, Raw: raw, Kind: "Swap", Timestamp: 1700000000, Payload: payload}
	wrapper := ReplayEnvelopeV2{Version: valueIndexerReplayV2, Mode: valueHistorical, Target: "charts", Generation: "g1", ProjectionRevision: "r1", SourceStream: realStream, RawDigest: digest, Raw: raw, OriginalTimestampSeconds: "1700000000", EventKind: event.Kind, Event: payload}
	body, err := canonicalReplayPayload(wrapper)
	if err != nil || !bytes.Contains(body, payload) {
		t.Fatalf("exact replay payload: %s %v", body, err)
	}
	segment := store.ReplaySegment{Records: []store.RawRecord{raw}, Events: []store.ReplayEvent{event}}
	facts := generationFacts(segment)
	if len(facts) != 1 || facts[0].RawDigest != digest || !reflect.DeepEqual(facts[0].Events, segment.Events) {
		t.Fatalf("raw generation facts: %+v", facts)
	}
	first, err := generationDigest(facts)
	if err != nil {
		t.Fatal(err)
	}
	facts[0].Events[0].Timestamp++
	second, err := generationDigest(facts)
	if err != nil || first == second {
		t.Fatalf("generation digest ignores original timestamp: %s/%s %v", first, second, err)
	}
}

func TestReplayAdmissionAndCheckpointRefusals(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	request := authorizeCoverageReplay(t, h, "backfill", "paused-source", 33963717, 33963733)
	original := h.Replay.Source
	h.Replay.Source = &source.HistoricalQualifiedSource{}
	if err := h.Replay.Run(ctx, request); err == nil {
		t.Fatal("unavailable source completed replay")
	}
	var status string
	if err := h.Pool.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status); err != nil || status != "paused" {
		t.Fatalf("unavailable replay state %s: %v", status, err)
	}
	h.Replay.Source = original
	if err := h.Store.ResumeReplay(ctx, request.Scope(), request.JobID); err != nil {
		t.Fatal(err)
	}
	if err := h.Replay.Run(ctx, request); err != nil {
		t.Fatalf("resumed source replay: %v", err)
	}
	changed := authorizeCoverageReplay(t, h, "replay", "checkpoint-conflict", 33963717, 33963733)
	changed.CheckpointHash = "0x" + strings.Repeat("1", 64)
	if err := h.Replay.Run(ctx, changed); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed checkpoint: %v", err)
	}
	if err := h.Pool.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, changed.JobID).Scan(&status); err != nil || status != "paused" {
		t.Fatalf("checkpoint conflict state %s: %v", status, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Backfill)
	}{
		{"mode", func(b *Backfill) {
			r := *b.p.deps.HistoricalRequest
			r.Mode = "replay"
			b.p.deps.HistoricalRequest = &r
		}},
		{"range", func(b *Backfill) { v := int64(1); b.fromBlock = &v }},
		{"batch", func(b *Backfill) { b.batchSize = 1001 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := New(h.Processor.deps)
			p.deps.HistoricalReplay = h.Replay
			p.deps.HistoricalRequest = &request
			b := NewBackfill(p, nil, 17)
			tc.change(b)
			if err := b.Run(ctx); err == nil {
				t.Fatal("unbound historical operation accepted")
			}
		})
	}
	invalid := request
	invalid.Mode = "backfill"
	if _, err := NewGenerationReplayProjection(h.Replay, invalid); err == nil {
		t.Fatal("live target accepted as isolated reconstruction")
	}
	invalid = request
	invalid.JobID = ""
	if _, err := NewGenerationReplayProjection(h.Replay, invalid); err == nil {
		t.Fatal("unnamed generation operation accepted")
	}
	if _, err := h.Replay.fetch(ctx, request, 33963733, 33963734); err != nil {
		t.Fatalf("qualified historical range: %v", err)
	}
}

func TestMeasurementHeadObservationsBindMonotonicHeights(t *testing.T) {
	head := new(measuredHead)
	head.height.Store(101)
	h := &IngestMeasurementHarness{Processor: New(Deps{Head: head, PollInterval: time.Millisecond})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates, failures, done := h.observeCommittedHeads(ctx, 100)
	select {
	case observed := <-updates:
		if observed.Height != 101 || observed.ObservedAt.IsZero() {
			t.Fatalf("head observation %+v", observed)
		}
	case err := <-failures:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("head observation unavailable")
	}
	head.height.Store(99)
	select {
	case err := <-failures:
		if !errors.Is(err, store.ErrIntegrity) {
			t.Fatalf("head regression %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("head regression accepted")
	}
	<-done
	head.height.Store(101)
	head.failure.Store(true)
	_, failures, done = h.observeCommittedHeads(ctx, 100)
	select {
	case err := <-failures:
		if !errors.Is(err, store.ErrIntegrity) {
			t.Fatalf("head provider refusal %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("head error dropped")
	}
	<-done
	head.failure.Store(false)
	_, _, done = h.observeCommittedHeads(ctx, 101)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("head observer ignored cancellation")
	}
}

func TestArchiveReaderRejectsChangedObjectAndAuthority(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	archive := &Archive{Store: h.Store, Source: h.Replay.Source, Objects: archiveCoverageObjects(t)}
	manifest, err := archive.Produce(ctx, realStream, 33963716, 33963716)
	if err != nil {
		t.Fatal(err)
	}
	body, err := archive.readVerified(ctx, manifest)
	if err != nil || delivery.Hash(body) != manifest.SHA256 {
		t.Fatalf("archive bytes %v", err)
	}
	changed := manifest
	changed.Bucket = "different"
	if _, err = archive.readVerified(ctx, changed); err == nil {
		t.Fatal("different archive authority accepted")
	}
	changed = manifest
	changed.ByteLength++
	if _, err = archive.readVerified(ctx, changed); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed archive byte count %v", err)
	}
	changed = manifest
	changed.FormatVersion = "unknown"
	if _, err = archive.readVerified(ctx, changed); err == nil {
		t.Fatal("unknown archive format accepted")
	}
	changed = manifest
	changed.SHA256 = strings.Repeat("0", 64)
	changed.ObjectKey = "indexer/raw/v2/" + realStream + "/" + changed.SHA256 + ".json"
	if _, err = archive.readVerified(ctx, changed); err == nil {
		t.Fatal("missing archive object accepted")
	}
	corrupt := append([]byte(nil), body...)
	corrupt[len(corrupt)/2] ^= 1
	if err = archive.Objects.Put(ctx, manifest.ObjectKey, bytes.NewReader(corrupt), int64(len(corrupt)), "application/json"); err != nil {
		t.Fatal(err)
	}
	if _, err = archive.Restore(ctx, realStream, 33963716, 33963716); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed archive bytes accepted: %v", err)
	}
	if _, err = archive.Restore(ctx, realStream, 33963717, 33963717); err == nil {
		t.Fatal("unarchived range accepted")
	}
	for _, body := range [][]byte{[]byte(`{`), []byte(`{} {}`), []byte(`{"unknown":1}`)} {
		changed := manifest
		changed.ByteLength = int64(len(body))
		changed.SHA256 = delivery.Hash(body)
		changed.ObjectKey = "indexer/raw/v2/" + realStream + "/" + changed.SHA256 + ".json"
		if _, err = ValidateArchivedSegment(body, changed); err == nil {
			t.Fatal("invalid archive JSON accepted")
		}
	}
}

func TestGenerationDigestRequiresEveryEffectAndOutbox(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	request := authorizeCoverageReplay(t, h, "reconstruct", "generation-checks", 33963733, 33963733)
	projection, err := NewGenerationReplayProjection(h.Replay, request)
	if err != nil {
		t.Fatal(err)
	}
	segment, err := h.Replay.fetch(ctx, request, 33963733, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, raw := range segment.Records {
		if err = projection.ApplyRawTx(ctx, tx, request.Generation, delivery.ArchiveRaw{Height: raw.Height, TransactionHash: raw.TransactionHash, BlockHash: raw.BlockHash, Emitter: raw.Emitter, TransactionIndex: raw.TransactionIndex, LogIndex: raw.LogIndex, Topics: raw.Topics, Data: raw.Data}); err != nil {
			t.Fatal(err)
		}
	}
	manifest := delivery.ArchiveManifest{Version: 1, ChainID: 125, Complete: true, SourceStream: request.SourceStream, Deployment: "fixture", DecoderDigest: strings.Repeat("1", 64), ABIDigest: strings.Repeat("2", 64), CodeDigest: strings.Repeat("3", 64), FromHeight: 33963733, ToHeight: 33963733, RawCount: int64(len(segment.Records)), Blocks: []delivery.ArchiveBlock{{Height: 33963733, BlockHash: segment.Blocks[0].Hash, RawCount: int64(len(segment.Records)), SegmentHash: strings.Repeat("4", 64), SegmentBytes: 1}}}
	canonical, err := projection.CanonicalDigestTx(ctx, tx, manifest)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := projection.ProjectionDigestTx(ctx, tx, request.Generation)
	if err != nil || actual != canonical {
		t.Fatalf("canonical generation %s/%s: %v", actual, canonical, err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"effect payload", `UPDATE indexer.indexer_replay_effects SET payload='{}'::jsonb`},
		{"missing effect", `DELETE FROM indexer.indexer_replay_effects`},
		{"outbox binding", `WITH old AS (DELETE FROM indexer.delivery_outbox RETURNING *) INSERT INTO indexer.delivery_outbox(event_id,schema_version,subject,payload_hash,envelope) SELECT event_id,schema_version,subject,repeat('0',64),envelope FROM old`},
		{"missing outbox", `DELETE FROM indexer.delivery_outbox`},
		{"generation payload", `UPDATE indexer.indexer_generation_raw SET payload='[]'::jsonb`},
		{"generation count", `UPDATE indexer.indexer_generation_raw SET payload='{}'::jsonb`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tx.Exec(ctx, "SAVEPOINT generation_case"); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if digest, err := projection.ProjectionDigestTx(ctx, tx, request.Generation); err == nil {
				t.Fatalf("changed generation accepted: %s", digest)
			}
			if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT generation_case"); err != nil {
				t.Fatal(err)
			}
		})
	}
	changed := manifest
	changed.Blocks = append([]delivery.ArchiveBlock(nil), manifest.Blocks...)
	changed.Blocks[0].BlockHash = "0x" + strings.Repeat("0", 64)
	if _, err = projection.CanonicalDigestTx(ctx, tx, changed); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed canonical generation block %v", err)
	}
	if _, err = projection.ProjectionDigestTx(ctx, tx, "other"); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed generation name %v", err)
	}
	if err = projection.ApplyRawTx(ctx, tx, "other", delivery.ArchiveRaw{Height: 33963733}); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed raw generation %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMeasurementReconcilesCompletedObservationWindow(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	spec := h.spec(t)
	request := authorizeCoverageReplay(t, h, "backfill", "measured-source-history", 33963716, 33963716)
	spec.Backfill = &request
	ctx, cancel := context.WithTimeout(context.Background(), spec.Duration+time.Minute)
	defer cancel()
	report, err := h.Run(ctx, spec)
	if err == nil || err.Error() != "ingestion measurement has observed violations" || report.FinalCanonicalRange == nil || report.CommittedRaw != 32 || report.CommittedBlocks != 34 || report.Transactions != 2 || report.TotalFills != 2 || report.HotPoolFills != 2 || report.WALScope == "" || report.FinalCanonicalRange.RawCount != report.CommittedRaw || report.FinalCanonicalRange.AccountingFillCount != report.TotalFills {
		t.Fatalf("completed observation: %+v %v", report, err)
	}
	if report.FinishedAt.Sub(report.StartedAt) < spec.Duration || report.CaptureSHA256 == "" || !report.FinalSharedAcceptanceDeferred || !slices.Contains(report.Violations, "actual ingest executed no COPY staging") {
		t.Fatalf("measurement bounds and trace: %+v", report)
	}
	var status string
	if err := h.Pool.QueryRow(context.Background(), `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("concurrent historical job %s: %v", status, err)
	}
	body, err := os.ReadFile(spec.OutputPath)
	if err != nil || !bytes.Contains(body, []byte(`"kind":"canonical-identity-reconciliation"`)) || !bytes.Contains(body, []byte(`"kind":"storage-finish"`)) {
		t.Fatalf("completed capture: %v", err)
	}
}

func TestMeasurementRunsAuthorizedBackfillDuringIngest(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, context.Background(), 33963716, 33963716)
	if err := h.Store.ReleaseLease(context.Background(), 125, realStream, inventoryTestOwner, h.Processor.canonicalFence); err != nil {
		t.Fatal(err)
	}
	h.Processor.canonicalFence = 0
	h.Processor.readyFence.Store(0)
	request := authorizeCoverageReplay(t, h, "backfill", "measured-backfill", 33963716, 33963716)
	spec := h.spec(t)
	spec.Backfill = &request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	report, err := h.Run(ctx, spec)
	if !errors.Is(err, context.DeadlineExceeded) || report.CommittedBlocks != 33 || report.CommittedRaw == 0 || report.CaptureSHA256 == "" {
		t.Fatalf("concurrent backfill measurement: %+v %v", report, err)
	}
	var status string
	if err := h.Pool.QueryRow(context.Background(), `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("measured backfill state %s: %v", status, err)
	}
	for _, mode := range []string{"replay", "backfill"} {
		next := spec
		next.OutputPath = filepath.Join(t.TempDir(), "input.ndjson")
		changed := request
		changed.Mode = mode
		next.Backfill = &changed
		if mode == "backfill" {
			h.Replay = nil
		}
		observed, err := h.Run(context.Background(), next)
		if err == nil || observed.CaptureSHA256 == "" || observed.Transactions != 0 {
			t.Fatalf("unbound concurrent replay accepted: %+v %v", observed, err)
		}
	}
}
