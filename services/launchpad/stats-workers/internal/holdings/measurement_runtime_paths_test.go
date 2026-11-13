package holdings

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func phase4RuntimePlan(t *testing.T, origin string) Phase4Plan {
	t.Helper()
	p := phase4TestPlan(t, origin)
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	p.PythonExecutable, err = filepath.EvalSymlinks(python)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p.PythonExecutable)
	if err != nil {
		t.Fatal(err)
	}
	p.PythonSHA256 = phase4Hash(raw)
	directory := t.TempDir()
	p.CalibrationPath = filepath.Join(directory, "calibration.json")
	program := `import sys,json,time,hashlib
sys.path.insert(0,sys.argv[1])
from scheduler import Sample,summarize
end=time.time_ns()-1000000000
start=end-60000000000
raw=[dict(id=i,scheduled_ns=1000000000+i*1000000000,attempted_ns=1000000001+i*1000000000,completed_ns=1000000002+i*1000000000,status="received",receiver_monotonic_ns=1000000001+i*1000000000,receiver_utc_ns=start+i*1000000000) for i in range(60)]
report=dict(raw_samples=raw,raw_samples_sha256=hashlib.sha256(json.dumps(raw,sort_keys=True,separators=(",",":")).encode()).hexdigest(),duration_ns=60000000000,started_utc_ns=start,ended_utc_ns=end,rate=1,cpu_before=[100,50],cpu_after=[200,150],cpu_fraction=0,nic_fraction=0,nic_counters=dict(tx_before=0,rx_before=0,tx_after=0,rx_after=0,duration_ns=60000000000,speed_bps=1000000000),candidate=sys.argv[3],topology=sys.argv[4],generator=sys.argv[5],outside_application_cluster=True,clock_offset_ms=0)
report.update(summarize([Sample(**s) for s in raw]))
json.dump([dict(report,host_id="calibration-"+str(i)) for i in range(4)],open(sys.argv[2],"w"))
`
	if out, err := exec.Command(p.PythonExecutable, "-c", program, filepath.Join(p.SourceRoot, "tools/launchpad/production/load"), p.CalibrationPath, p.Candidate, p.Topology, p.Generator).CombinedOutput(); err != nil {
		t.Fatalf("calibration inputs: %s %v", out, err)
	}
	raw, err = os.ReadFile(p.CalibrationPath)
	if err != nil {
		t.Fatal(err)
	}
	p.CalibrationSHA256 = phase4Hash(raw)
	quote := filepath.Join(directory, "quote.json")
	if err := os.WriteFile(quote, []byte(`{"monthlyMinor":"100"}`), 0600); err != nil {
		t.Fatal(err)
	}
	budget := map[string]any{"candidate": p.Candidate, "topology": p.Topology, "currency": "USD", "approvedBy": "operator", "approvedMonthlyMinor": "130", "modeledMonthlyMinor": "100", "contingencyBps": 3000, "validUntil": time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "quotes": []map[string]string{{"path": quote, "sha256": phase4Hash([]byte(`{"monthlyMinor":"100"}`)), "vendor": "database"}}}
	raw, err = json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	p.BudgetPath = filepath.Join(directory, "budget.json")
	p.BudgetSHA256 = phase4Hash(raw)
	if err := os.WriteFile(p.BudgetPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func phase4RunPlan(t *testing.T, ctx context.Context, p Phase4Plan) (string, error) {
	t.Helper()
	for k, v := range map[string]string{"L14_PHASE4_APPROVED_CANDIDATE": p.Candidate, "L14_PHASE4_APPROVED_INTERFACE": p.Interface, "L14_PHASE4_APPROVED_TOPOLOGY": p.Topology} {
		t.Setenv(k, v)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "plan.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "measurement")
	return output, RunPhase4Measurement(ctx, path, phase4Hash(raw), output)
}

func TestPhase4RuntimeProviderBoundaries(t *testing.T) {
	p := phase4RuntimePlan(t, "http://127.0.0.1:8080")
	for _, tc := range []struct{ name, dsn, broker, want string }{
		{"missing primary", "", "", "actual primary PostgreSQL"},
		{"invalid primary", "postgres://%", "nats://127.0.0.1:1", "invalid private PostgreSQL"},
		{"closed primary", "postgres://127.0.0.1:1/stats?sslmode=disable&connect_timeout=1", "nats://127.0.0.1:1", "connect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("L14_PHASE4_POSTGRES_DSN", tc.dsn)
			t.Setenv("L14_PHASE4_NATS_URL", tc.broker)
			output, err := phase4RunPlan(t, context.Background(), p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("provider admission: %v", err)
			}
			var result struct {
				Stage  string `json:"failureStage"`
				Passed bool   `json:"laneMeasurementPassed"`
				Global bool   `json:"globalCapacityQualified"`
			}
			raw, readErr := os.ReadFile(filepath.Join(output, "result.json"))
			if readErr != nil || json.Unmarshal(raw, &result) != nil || result.Stage != "providers" || result.Passed || result.Global {
				t.Fatalf("provider result: %s %v", raw, readErr)
			}
			raw, readErr = os.ReadFile(filepath.Join(output, "calibration.ndjson"))
			if readErr != nil || !strings.Contains(string(raw), `"exitSuccess":true`) {
				t.Fatalf("calibration result: %s %v", raw, readErr)
			}
		})
	}
}

func TestPhase4RuntimeMeasuresAcceptedRequests(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	var timestamps string
	if err := pool.QueryRow(ctx, "SHOW track_commit_timestamp").Scan(&timestamps); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			_, _ = w.Write([]byte("cpu 0.1\nmemory 1024\n"))
			return
		}
		w.Header().Set("Server-Timing", "origin;dur=1")
		_, _ = w.Write([]byte(`{"quality":"complete","data":{"coverage":"complete"}}`))
	}))
	defer server.Close()
	p := phase4RuntimePlan(t, server.URL)
	const schema = "measurement_holders"
	ddl, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(mustPhase4Migration(t, "60001"))); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'canonical',10,$1,'healthy','indexer')`, mint.BlockHash); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); err != nil || batch.Applied != 1 {
		t.Fatalf("projected mint: %+v %v", batch, err)
	}
	p.HolderSchema = schema
	p.Pools[0] = Phase4Pool{Pool: addr(83), Token: mint.Token}
	for i := range p.Hosts {
		p.Hosts[i].MetricsURL = server.URL + "/metrics"
	}
	p.GeneratorNICBitsPerSecond = 1 << 60
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	p.BrokerStream = "MEASURE_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	p.BrokerConsumer = "measure"
	if _, err := js.AddStream(&nats.StreamConfig{Name: p.BrokerStream, Subjects: []string{p.BrokerStream + ".events"}, Storage: nats.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(p.BrokerStream); err != nil {
			t.Error(err)
		}
	})
	if _, err := js.AddConsumer(p.BrokerStream, &nats.ConsumerConfig{Durable: p.BrokerConsumer, AckPolicy: nats.AckExplicitPolicy, FilterSubject: p.BrokerStream + ".events"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("L14_PHASE4_POSTGRES_DSN", pool.Config().ConnConfig.ConnString())
	t.Setenv("L14_PHASE4_NATS_URL", os.Getenv("L20_NATS_URL"))
	output, err := phase4RunPlan(t, ctx, p)
	var result struct {
		Stage     string                   `json:"failureStage"`
		Passed    bool                     `json:"laneMeasurementPassed"`
		Summaries map[string]Phase4Summary `json:"summaries"`
	}
	raw, readErr := os.ReadFile(filepath.Join(output, "result.json"))
	if readErr != nil || json.Unmarshal(raw, &result) != nil || result.Passed {
		t.Fatalf("measurement result: %s %v", raw, readErr)
	}
	if timestamps != "on" {
		if err == nil || !strings.Contains(err.Error(), "commit timestamp enabled primary") {
			t.Fatalf("commit authority: %v", err)
		}
		return
	}
	intervals, intervalErr := os.ReadFile(filepath.Join(output, "generator-intervals.ndjson"))
	if intervalErr != nil {
		t.Fatal(intervalErr)
	}
	scanner := bufio.NewScanner(bytes.NewReader(intervals))
	saturated, intervalCount := false, 0
	for scanner.Scan() {
		var interval struct {
			CPU    string `json:"cpuFraction"`
			NIC    string `json:"nicFraction"`
			Passed bool   `json:"targetPassed"`
		}
		if e := json.Unmarshal(scanner.Bytes(), &interval); e != nil {
			t.Fatal(e)
		}
		cpu, cpuOK := new(big.Rat).SetString(interval.CPU)
		nic, nicOK := new(big.Rat).SetString(interval.NIC)
		if !cpuOK || !nicOK {
			t.Fatalf("invalid utilization interval: %s", scanner.Bytes())
		}
		exceeds := cpu.Cmp(big.NewRat(7, 10)) > 0 || nic.Cmp(big.NewRat(7, 10)) > 0
		if interval.Passed == exceeds {
			t.Fatalf("utilization admission differs from counters: %s", scanner.Bytes())
		}
		saturated = saturated || exceeds
		intervalCount++
	}
	if scanner.Err() != nil || intervalCount == 0 {
		t.Fatalf("generator interval samples: %d %v", intervalCount, scanner.Err())
	}
	// The arrival records determine whether the generator met its scheduling bound.
	arrivals, readHTTPError := os.ReadFile(filepath.Join(output, "http.ndjson"))
	if readHTTPError != nil {
		t.Fatal(readHTTPError)
	}
	delays := []int64{}
	arrivalsScanner := bufio.NewScanner(bytes.NewReader(arrivals))
	for arrivalsScanner.Scan() {
		var sample Phase4Sample
		if e := json.Unmarshal(arrivalsScanner.Bytes(), &sample); e != nil {
			t.Fatal(e)
		}
		if sample.Status != 200 || sample.Error != "" || sample.OriginNS != 1000000 || sample.AttemptedOffsetNS < sample.ScheduledOffsetNS {
			t.Fatalf("accepted arrival: %+v", sample)
		}
		delays = append(delays, sample.AttemptedOffsetNS-sample.ScheduledOffsetNS)
	}
	if arrivalsScanner.Err() != nil || len(delays) != 60 {
		t.Fatalf("arrival evidence: %d %v", len(delays), arrivalsScanner.Err())
	}
	sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })
	summary := result.Summaries["normal"]
	if summary.Completed != 60 || summary.Scheduled != 60 || summary.Failed != 0 || summary.Missed != 0 || summary.OriginP95NS != 1000000 || summary.OriginP99NS != 1000000 || summary.SchedulerP99NS != delays[59] {
		t.Fatalf("normal reads: %+v", summary)
	}
	wantFailure := "actual projection lag samples missing from phase"
	if saturated {
		wantFailure = "actual generator CPU/NIC saturation exceeds70percent"
	}
	if delays[59] > 10000000 {
		wantFailure = "generator saturation"
		if len(summary.Errors) != 1 || summary.Errors[0] != wantFailure {
			t.Fatalf("scheduler admission: %+v", summary)
		}
	} else if len(summary.Errors) != 0 {
		t.Fatalf("scheduler within bound: %+v", summary)
	}
	if err == nil || err.Error() != wantFailure || result.Stage != "measurement_normal" {
		t.Fatalf("measurement admission: %s %v; want %s", raw, err, wantFailure)
	}
	for _, name := range []string{"database.ndjson", "broker.ndjson", "hosts.ndjson", "generator.ndjson", "generator-intervals.ndjson", "http.ndjson", "artifact-sha256.json"} {
		raw, err := os.ReadFile(filepath.Join(output, name))
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func mustPhase4Migration(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../../../../migrations", name+".sql"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPhase4RuntimeRejectsUnboundPlan(t *testing.T) {
	p := phase4TestPlan(t, "http://127.0.0.1:8080")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := phase4SourceBindings(ctx, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled source binding: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*Phase4Plan)
		want string
	}{
		{"source missing", func(p *Phase4Plan) { delete(p.Sources, phase4RequiredSources[0]) }, "source missing"},
		{"source path", func(p *Phase4Plan) { p.Sources["../escape"] = strings.Repeat("a", 64) }, "escapes"},
		{"source absent", func(p *Phase4Plan) { p.Sources["absent.go"] = strings.Repeat("a", 64) }, "path mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := p
			copy.Sources = map[string]string{}
			for k, v := range p.Sources {
				copy.Sources[k] = v
			}
			tc.edit(&copy)
			if err := phase4SourceBindings(context.Background(), copy); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("source binding: %v", err)
			}
		})
	}
}
