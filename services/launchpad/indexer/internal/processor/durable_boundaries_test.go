package processor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestMeasurementAdmissionAndUnavailableDurableInputs(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	spec := h.spec(t)
	for _, tc := range []struct {
		name string
		edit func(*IngestMeasurementSpec)
		want string
	}{
		{"burst source", func(s *IngestMeasurementSpec) { s.Profile = "burst"; s.Duration = time.Second }, "exactly five minutes"},
		{"pool identity", func(s *IngestMeasurementSpec) { s.HotPool = "pool" }, "hot pool"},
		{"output directory", func(s *IngestMeasurementSpec) { s.OutputPath = t.TempDir() }, "file exists"},
		{"capture bound", func(s *IngestMeasurementSpec) { s.CaptureMaxBytes = 1 }, "storage budget"},
		{"database binding", func(s *IngestMeasurementSpec) {
			var topology IngestTopologyEvidence
			body, err := os.ReadFile(s.TopologyPath)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &topology); err != nil {
				t.Fatal(err)
			}
			topology.Database = "another-database"
			body, err = json.Marshal(topology)
			if err != nil {
				t.Fatal(err)
			}
			s.TopologyPath = filepath.Join(t.TempDir(), "topology.json")
			if err = os.WriteFile(s.TopologyPath, body, 0600); err != nil {
				t.Fatal(err)
			}
			s.TopologySHA256 = delivery.Hash(body)
		}, "differs from approved topology"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec
			s.OutputPath = filepath.Join(t.TempDir(), "capture.jsonl")
			tc.edit(&s)
			report, err := h.Run(ctx, s)
			if err == nil || !strings.Contains(err.Error(), tc.want) || report.CommittedRaw != 0 {
				t.Fatalf("admission %+v %v", report, err)
			}
		})
	}
	if _, err := (*IngestMeasurementHarness)(nil).Run(ctx, spec); err == nil || !strings.Contains(err.Error(), "harness required") {
		t.Fatalf("nil harness %v", err)
	}
	invalid := spec
	invalid.SourceFiles = map[string]string{}
	if _, err := h.Run(ctx, invalid); err == nil {
		t.Fatal("empty candidate accepted")
	}
	originalStore := h.Store
	h.Store = nil
	if _, err := h.CaptureRestoreCheckpoint(ctx, spec, 33963716, 33963716, filepath.Join(t.TempDir(), "checkpoint.json")); err == nil {
		t.Fatal("unbound checkpoint accepted")
	}
	h.Store = originalStore
	h.Pool.Close()
	spec.OutputPath = filepath.Join(t.TempDir(), "capture.jsonl")
	if _, err := h.Run(ctx, spec); err == nil {
		t.Fatal("closed measurement pool accepted")
	}
	if _, err := h.CaptureRestoreCheckpoint(ctx, spec, 33963716, 33963716, filepath.Join(t.TempDir(), "checkpoint.json")); err == nil {
		t.Fatal("closed checkpoint pool accepted")
	}
}

func TestReplayPausesUnavailableArchivesAndCheckpointDisagreement(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	for _, tc := range []struct {
		name, mode string
		edit       func(*store.ReplayRequest)
		want       error
	}{
		{name: "archive absent", mode: "reconstruct"},
		{name: "checkpoint disagreement", mode: "replay", edit: func(r *store.ReplayRequest) { r.CheckpointHash = "0x" + strings.Repeat("1", 64) }, want: store.ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := strings.ReplaceAll(tc.name, " ", "-")
			r := authorizeCoverageReplay(t, h, tc.mode, id, 33963716, 33963716)
			if tc.edit != nil {
				tc.edit(&r)
			}
			err := h.Replay.Run(ctx, r)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("unavailable replay accepted: %v", err)
			}
			var status string
			if err = h.Pool.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, r.JobID).Scan(&status); err != nil || status != "paused" {
				t.Fatalf("replay state %s %v", status, err)
			}
		})
	}
}

func TestProcessorCanonicalRangeAdmissionAndLoopCancellation(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	p := h.Processor
	for _, bounds := range [][2]int64{{-1, 1}, {2, 1}} {
		if err := p.ProcessBatch(ctx, bounds[0], bounds[1]); err == nil {
			t.Fatalf("invalid range %v accepted", bounds)
		}
	}
	owner := p.deps.CanonicalOwner
	p.deps.CanonicalOwner = ""
	if err := p.ProcessBatch(ctx, 33963716, 33963716); err == nil {
		t.Fatal("ownerless batch accepted")
	}
	p.deps.CanonicalOwner = owner
	stream := p.deps.CanonicalStreamID
	p.deps.CanonicalStreamID = ""
	if err := p.Run(ctx); err == nil || !strings.Contains(err.Error(), "canonical stream") {
		t.Fatalf("streamless loop %v", err)
	}
	p.deps.CanonicalStreamID = stream
	if err := p.ProcessBatch(ctx, 33963718, 33963718); !errors.Is(err, store.ErrWatermark) {
		t.Fatalf("gap batch %v", err)
	}
	if err := p.deps.Store.ReleaseLease(ctx, 125, stream, owner, p.canonicalFence); err != nil {
		t.Fatal(err)
	}
	p.canonicalFence = 0
	// A head read failure keeps the processor waiting until its caller cancels.
	h.head.failure.Store(true)
	bounded, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	if err := p.Run(bounded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("head failure shutdown %v", err)
	}
	cancel()
	h.head.failure.Store(false)
	h.Pool.Close()
	if err := p.Run(ctx); err == nil || !strings.Contains(err.Error(), "canonical watermark") {
		t.Fatalf("closed canonical loop %v", err)
	}
	legacy := New(Deps{Store: h.Store, Source: &recordedWorkload{}, Head: h.head})
	if err := legacy.Run(ctx); err == nil || !strings.Contains(err.Error(), "load cursor") {
		t.Fatalf("closed legacy loop %v", err)
	}
}
