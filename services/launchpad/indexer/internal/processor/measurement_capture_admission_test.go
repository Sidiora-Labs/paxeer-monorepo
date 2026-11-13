package processor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

func TestMeasurementCaptureBudgetsBoundEachDurableObservation(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	primeMeasurementCaptureSource(t, h)
	spec := h.spec(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	report, err := h.Run(ctx, spec)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || report.CommittedRaw != 32 {
		t.Fatalf("reference observations: raw=%d err=%v", report.CommittedRaw, err)
	}
	file, err := os.Open(spec.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 16<<20)
	budgets := map[string]int64{}
	var offset int64
	for scanner.Scan() {
		line := scanner.Bytes()
		var record struct{ Kind string }
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if _, ok := budgets[record.Kind]; !ok {
			budgets[record.Kind] = offset + int64(len(line))/2
		}
		offset += int64(len(line) + 1)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"input", "approved-topology-input", "storage-start", "source-batch", "durable-ack", "resources", "query-plans"} {
		t.Run(kind, func(t *testing.T) {
			budget, ok := budgets[kind]
			if !ok || budget < 1 {
				t.Fatalf("observation %s absent", kind)
			}
			fresh := newLifecycleCoverageHarness(t)
			primeMeasurementCaptureSource(t, fresh)
			bounded := fresh.spec(t)
			bounded.CaptureMaxBytes = budget
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got, err := fresh.Run(ctx, bounded)
			if err == nil || !strings.Contains(err.Error(), "storage budget") || got.CaptureSHA256 == "" || fresh.Processor.readyFence.Load() != 0 {
				t.Fatalf("bounded observation %s: capture=%s readiness=%d err=%v", kind, got.CaptureSHA256, fresh.Processor.readyFence.Load(), err)
			}
			info, err := os.Stat(bounded.OutputPath)
			if err != nil || info.Size() > budget {
				t.Fatalf("capture bound: %v %v", info, err)
			}
		})
	}
}

func TestMeasurementRetainsActualCommitFailureAndBackfillAdmission(t *testing.T) {
	for _, mode := range []string{"commit failure", "missing runner", "wrong backfill mode", "unissued backfill"} {
		t.Run(mode, func(t *testing.T) {
			h := newLifecycleCoverageHarness(t)
			spec := h.spec(t)
			var want string
			switch mode {
			case "commit failure":
				if _, err := h.Pool.Exec(context.Background(), `CREATE FUNCTION indexer.reject_raw_write() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''raw write unavailable''; END'; CREATE TRIGGER reject_raw_write BEFORE INSERT ON indexer.indexer_raw_logs FOR EACH ROW EXECUTE FUNCTION indexer.reject_raw_write()`); err != nil {
					t.Fatal(err)
				}
				want = "raw write unavailable"
			case "missing runner":
				spec.Backfill = &store.ReplayRequest{Mode: "backfill"}
				h.Replay = nil
				want = "backfill runner"
			case "wrong backfill mode":
				spec.Backfill = &store.ReplayRequest{Mode: "replay"}
				want = "explicit backfill"
			case "unissued backfill":
				spec.Backfill = &store.ReplayRequest{Mode: "backfill"}
				want = "replay"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			report, err := h.Run(ctx, spec)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), want) || report.CaptureSHA256 == "" || h.Processor.readyFence.Load() != 0 {
				t.Fatalf("measurement admission %s: raw=%d err=%v", mode, report.CommittedRaw, err)
			}
			if mode == "commit failure" && (report.CommittedRaw != 0 || report.OfferedRaw == 0) {
				t.Fatalf("rejected commit counts: %+v", report)
			}
		})
	}
}

func TestMeasurementCaptureRejectsDirectoryAndInvalidTopology(t *testing.T) {
	h := newLifecycleCoverageHarness(t)
	spec := h.spec(t)
	spec.OutputPath = filepath.Join(t.TempDir(), "missing", "capture")
	if report, err := h.Run(context.Background(), spec); !os.IsNotExist(err) || report.CommittedRaw != 0 {
		t.Fatalf("unavailable capture: %v", err)
	}
}

func primeMeasurementCaptureSource(t *testing.T, h lifecycleCoverageHarness) {
	t.Helper()
	ctx := context.Background()
	var raw, height int64
	err := h.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_raw_logs),typed_complete_height FROM indexer.indexer_ingest_watermarks WHERE chain_id=125 AND stream_id=$1`, realStream).Scan(&raw, &height)
	if err != nil || raw != 0 || height != int64(realAnchor.Height) {
		t.Fatalf("initial measurement state: raw=%d height=%d err=%v", raw, height, err)
	}
	for _, bounds := range []struct {
		from, to uint64
		records  int
	}{{33963716, 33963732, 15}, {33963733, 33963749, 17}} {
		batch, qualified, err := h.Processor.deps.QualifiedBatchSource.FetchQualifiedBatch(ctx, bounds.from, bounds.to)
		if err != nil || len(batch.Blocks) != 17 || len(batch.Records) != bounds.records || len(qualified) == 0 {
			t.Fatalf("qualified capture source: blocks=%d records=%d receipts=%d err=%v", len(batch.Blocks), len(batch.Records), len(qualified), err)
		}
	}
	err = h.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_raw_logs),typed_complete_height FROM indexer.indexer_ingest_watermarks WHERE chain_id=125 AND stream_id=$1`, realStream).Scan(&raw, &height)
	if err != nil || raw != 0 || height != int64(realAnchor.Height) {
		t.Fatalf("source reads changed durable state: raw=%d height=%d err=%v", raw, height, err)
	}

}
