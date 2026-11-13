package source

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func TestCapacityCompletesWithMeasuredFaultRecoveryAndCertifiedParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	node := newCapacityChain(t)
	node.head.Store(128)
	log := node.logs[0]
	log.BlockNumber = 129
	log.BlockHash = committedFixtureHash(129)
	node.logs = make([]ethtypes.Log, 100)
	for i := range node.logs {
		node.logs[i] = log
		node.logs[i].Index = uint(i)
	}
	var mode atomic.Value
	mode.Store("")
	faultNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load().(string) {
		case "throttle":
			w.WriteHeader(http.StatusTooManyRequests)
		case "truncate", "endpoint_loss":
			_, _ = io.WriteString(w, `{"jsonrpc":`)
		default:
			forward, finish := context.WithTimeout(context.Background(), time.Second)
			defer finish()
			request, err := http.NewRequestWithContext(forward, r.Method, node.server.URL, r.Body)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			response, err := node.server.Client().Do(request)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			defer response.Body.Close()
			w.WriteHeader(response.StatusCode)
			if _, err = io.Copy(w, response.Body); err != nil {
				t.Error(err)
			}
		}
	}))
	defer faultNode.Close()
	options := fopts(node.server.URL)
	options.Endpoints = map[string]string{"a-live": node.server.URL, "z-recovery": faultNode.URL}
	live := repair997Open(t, options)
	repair997Seed(t, live)
	var endpoint *committedEndpoint
	for _, candidate := range live.endpoints {
		if candidate.id == "z-recovery" {
			endpoint = candidate
		}
	}
	if endpoint == nil {
		t.Fatal("recovery endpoint absent")
	}
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind := strings.TrimPrefix(r.URL.Path, "/")
		if kind == "start" {
			node.head.Store(129)
			if err := json.NewEncoder(w).Encode(capacityWorkloadRecord{Kind: "event", Value: json.RawMessage(`{"height":129}`)}); err != nil {
				t.Error(err)
			}
			return
		}
		mode.Store(kind)
		var result string
		switch kind {
		case "throttle":
			if err := live.rpcOnce(r.Context(), endpoint, false, "eth_chainId", nil, &result, 0); err == nil {
				t.Error("throttle admitted")
			}
		case "truncate":
			if err := live.rpc(r.Context(), endpoint, false, "eth_chainId", nil, &result); err == nil {
				t.Error("truncated response admitted")
			}
		case "endpoint_loss":
			for i := 0; i < 3; i++ {
				if err := live.rpc(r.Context(), endpoint, false, "eth_chainId", nil, &result); err == nil {
					t.Error("lost endpoint admitted")
				}
			}
			live.mu.Lock()
			quarantined := endpoint.quarantined
			live.mu.Unlock()
			if !quarantined {
				t.Error("invalid endpoint remained eligible")
			}
		case "recovery":
			mode.Store("")
		default:
			w.WriteHeader(400)
			return
		}
		state := "injected"
		if kind == "recovery" {
			state = valueRestored
		}
		fault := CapacityFaultObservation{Kind: kind, SourceID: endpoint.id, State: state, ObservedAt: time.Now().UTC()}
		if err := json.NewEncoder(w).Encode(capacityWorkloadRecord{Kind: "fault", Fault: &fault}); err != nil {
			t.Error(err)
		}
	}))
	defer control.Close()
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 10000, node.logs...)
	st := store.New(pool)
	const stream = "capacity-complete"
	fence, err := st.AcquireLease(ctx, 125, stream, "capacity", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,128,128)`, stream); err != nil {
		t.Fatal(err)
	}
	workload := &canonicalCapacityWorkload{source: live, store: st, fence: fence, stream: stream}
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live, Explorer: explorer}, Head: live, Processor: workload, Pool: pool}
	opts := capacityBoundOptions(t, live)
	opts.StreamID = stream
	opts.OutputDirectory = t.TempDir()
	opts.Duration = 2 * time.Second
	opts.TargetEventsPerSecond = 1
	opts.Profile = valueHarness
	opts.HistoricalFrom = 1
	opts.HistoricalTo = 2
	opts.ParityFrom = 1
	opts.ParityTo = 10000
	opts.MaxCaptureBytes = 64 << 20
	opts.MaxHeapBytes = 1 << 30
	opts.MutationApproved = true
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	python, err = filepath.EvalSymlinks(python)
	if err != nil {
		t.Fatal(err)
	}
	const script = "import sys,time,urllib.request\nwith urllib.request.urlopen(sys.argv[1]+'/start',timeout=10) as response:\n print(response.read().decode().strip(),flush=True)\ntime.sleep(0.4)\nfor kind in ('throttle','truncate','endpoint_loss','recovery'):\n with urllib.request.urlopen(sys.argv[1]+'/'+kind,timeout=10) as response:\n  print(response.read().decode().strip(),flush=True)\n"
	opts.Workload = CapacityCommand{Executable: python, SHA256: capacityFileDigest(t, python), Args: []string{"-c", script, control.URL}}
	prepareCapacityMeasurement(t, opts, providers)
	report, err := RunSourceCapacity(ctx, opts, providers)
	if err != nil || !report.HarnessPassed || report.CapacityTargetQualified || report.SharedSoakQualified || report.InputEvents != 100 || report.DurableEvents != 100 || report.Batches != 1 || report.HistoricalRanges == 0 || report.Throttles == 0 || report.InvalidResponses == 0 || len(report.FaultObservations) != 4 || report.Parity == nil || !report.Parity.Certified || report.Parity.HighLogBlockCount != 1 || report.Parity.CompleteBlockCount != 10000 {
		t.Fatalf("capacity completion passed=%v input=%d durable=%d batches=%d historical=%d throttle=%d invalid=%d faults=%v parity=%+v: %v", report.HarnessPassed, report.InputEvents, report.DurableEvents, report.Batches, report.HistoricalRanges, report.Throttles, report.InvalidResponses, report.FaultObservations, report.Parity, err)
	}
	for _, health := range live.EndpointHealth() {
		if health.SourceID == endpoint.id && (health.QuarantineReason != "" || health.RecoveryChecks != 128) {
			t.Fatalf("recovery health %+v", health)
		}
	}
	explorerOptions := explorer.options
	explorerOptions.ParityAdmission = report.Parity
	history, err := CreateHistorical(ctx, live.opts, explorer.pool.Config().ConnString(), &explorerOptions)
	if err != nil {
		t.Fatal(err)
	}
	historicalResult, err := history.FetchHistorical(ctx, FetchOptions{FromBlock: 129, ToBlock: 129, Historical: true}, 129)
	if err != nil || !historicalResult.Complete || len(historicalResult.Logs) != 100 || historicalResult.Explorer == nil {
		t.Fatalf("certified historical source: %+v %v", historicalResult, err)
	}
	if err = history.Close(); err != nil {
		t.Fatal(err)
	}
	if history, err = CreateHistorical(ctx, live.opts, "://invalid", &explorerOptions); err == nil || history != nil {
		t.Fatalf("invalid certified explorer DSN admitted: %v", err)
	}
	stored, err := os.ReadFile(filepath.Join(opts.OutputDirectory, "source-capacity.report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted CapacityReport
	if err = json.Unmarshal(stored, &persisted); err != nil || persisted.ReportID != report.ReportID || !persisted.HarnessPassed {
		t.Fatalf("retained capacity result: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(opts.OutputDirectory, "source-capacity.raw.jsonl"))
	if err != nil || !bytes.Contains(raw, []byte("durable_raw")) || !bytes.Contains(raw, []byte("workload_provider")) {
		t.Fatalf("retained source observations: %v", err)
	}
}
