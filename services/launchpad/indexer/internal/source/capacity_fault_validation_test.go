package source

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
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
)

func TestCapacityFaultAdmissionAndCanonicalProcessingFailures(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		faults     []CapacityFaultObservation
		change     func(*CapacityOptions, *canonicalCapacityWorkload)
	}{
		{name: "unknown endpoint", want: "capacity-fault-source", faults: []CapacityFaultObservation{{Kind: "endpoint_loss", SourceID: "absent", State: "injected"}}},
		{name: "stale observation", want: "capacity-fault-source", faults: []CapacityFaultObservation{{Kind: "throttle", State: "injected", ObservedAt: time.Unix(1, 0)}}},
		{name: "unquarantined recovery", want: "invalid", faults: []CapacityFaultObservation{{Kind: "recovery", State: "restored"}}},
		{name: "loss without recovery", want: "capacity-recovery", faults: []CapacityFaultObservation{{Kind: "endpoint_loss", State: "injected"}}},
		{name: "recovery before loss", want: "capacity-recovery-order", faults: []CapacityFaultObservation{{Kind: "recovery", State: "injected"}}},
		{name: "unknown fault", want: "capacity-fault-capture", faults: []CapacityFaultObservation{{Kind: "unknown", State: "injected"}}},
		{name: "processor lease", want: "fence", change: func(_ *CapacityOptions, w *canonicalCapacityWorkload) { w.fence++ }},
		{name: "historical range", want: "historical", change: func(o *CapacityOptions, _ *canonicalCapacityWorkload) { o.HistoricalFrom = 200; o.HistoricalTo = 201 }},
		{name: "sampled heap", want: "capacity", change: func(o *CapacityOptions, _ *canonicalCapacityWorkload) { o.MaxHeapBytes = 1 }},
		{name: "report exists", want: "file exists", change: func(o *CapacityOptions, _ *canonicalCapacityWorkload) {
			if err := os.WriteFile(filepath.Join(o.OutputDirectory, "source-capacity.report.json"), []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			node := newCapacityChain(t)
			live := repair997Open(t, fopts(node.server.URL))
			dsn, pool := canonicaltest.New(t)
			explorer := capacityExplorer(t, dsn, pool, live, 100)
			st := store.New(pool)
			fence, err := st.AcquireLease(ctx, 125, "capacity-fault", "capacity", 30*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'capacity-fault',100,100)`); err != nil {
				t.Fatal(err)
			}
			workload := &canonicalCapacityWorkload{source: live, store: st, fence: fence, stream: "capacity-fault"}
			providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live, Explorer: explorer}, Head: live, Processor: workload, Pool: pool}
			opts := capacityBoundOptions(t, live)
			opts.StreamID = "capacity-fault"
			opts.OutputDirectory = t.TempDir()
			opts.Duration = 800 * time.Millisecond
			opts.TargetEventsPerSecond = 1
			opts.Profile = "harness"
			opts.HistoricalFrom = 1
			opts.HistoricalTo = 2
			opts.ParityFrom = 1
			opts.ParityTo = 10000
			opts.MaxCaptureBytes = 16 << 20
			opts.MaxHeapBytes = 1 << 30
			opts.MutationApproved = true
			records := []string{`{"Kind":"event","Value":{"chainId":125}}`}
			for _, fault := range tc.faults {
				if fault.SourceID == "" {
					fault.SourceID = live.endpoints[0].id
				}
				raw, err := json.Marshal(capacityWorkloadRecord{Kind: "fault", Fault: &fault})
				if err != nil {
					t.Fatal(err)
				}
				records = append(records, string(raw))
			}
			opts.Workload = capacityObservedOutputCommand(t, strings.Join(records, "\n"))
			if tc.change != nil {
				tc.change(&opts, workload)
			}
			prepareCapacityMeasurement(t, opts, providers)
			node.advanceOnHead.Store(true)
			report, err := RunSourceCapacity(ctx, opts, providers)
			if err == nil || !strings.Contains(err.Error(), tc.want) || report.HarnessPassed || report.CapacityTargetQualified {
				t.Fatalf("fault admission %+v %v", report, err)
			}
			if tc.name == "processor lease" && (report.FailedAttempts != 1 || !errors.Is(err, store.ErrFenced)) {
				t.Fatalf("failed attempt count %+v %v", report, err)
			}
			if tc.name == "loss without recovery" || tc.name == "recovery before loss" {
				if report.InputEvents != 1 || report.DurableEvents != 1 || len(report.FaultObservations) != 1 {
					t.Fatalf("canonical fault observation %+v", report)
				}
			}
		})
	}
}
