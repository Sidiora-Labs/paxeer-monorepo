package holdings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func phase4TestPlan(t *testing.T, origin string) Phase4Plan {
	t.Helper()
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, name := range append(append([]string{}, phase4RequiredSources...), "tools/launchpad/production/load/calibration.py", "tools/launchpad/production/load/scheduler.py") {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = phase4Hash(raw)
	}
	return Phase4Plan{
		Candidate: strings.Repeat("a", 64), Interface: strings.Repeat("b", 64), Generator: strings.Repeat("c", 64),
		Topology: sources[phase4RequiredSources[0]], TopologyPath: filepath.Join(root, phase4RequiredSources[0]),
		SourceRoot: root, Sources: sources, Origin: origin, SourceStream: "canonical", HolderSchema: "holdings", BrokerStream: "events", BrokerConsumer: "holder",
		GlobalDAU:       500000,
		GlobalMix:       map[string]int{"chart": 800, "portfolio": 800, "search": 600, "quotes": 2000, "writes": 2000, "market": 1000, "rankings": 600, "identity_profile": 500, "metadata_media": 500, "notifications_social": 800, "capabilities": 400},
		Pools:           []Phase4Pool{{Pool: addr(1), Token: addr(2)}, {Pool: addr(3), Token: addr(4), Creator: addr(5)}},
		Phases:          []Phase4Phase{{Name: "normal", Seconds: 60, RPS: 1, HotBPS: 6000, SourceEventsRPS: 2000}, {Name: "hot", Seconds: 60, RPS: 1, HotBPS: 8000, SourceEventsRPS: 2000}, {Name: "burst", Seconds: 300, RPS: 1, HotBPS: 8000, SourceEventsRPS: 10000}, {Name: "recovery", Seconds: 60, RPS: 1, HotBPS: 8000, SourceEventsRPS: 2000}},
		Hosts:           []Phase4Host{{ID: "api", Role: "api", MetricsURL: origin, RequiredMetrics: []string{"cpu", "memory"}}, {ID: "database", Role: "postgres", MetricsURL: origin, RequiredMetrics: []string{"cpu", "memory"}}, {ID: "broker", Role: "broker", MetricsURL: origin, RequiredMetrics: []string{"cpu", "memory"}}, {ID: "worker", Role: "worker", MetricsURL: origin, RequiredMetrics: []string{"cpu", "memory"}}},
		GeneratorHostID: "generator", GeneratorNIC: "lo", GeneratorNICBitsPerSecond: 1000000000,
		MaxGeneratorHeapBytes: 1 << 40, MaxGeneratorRSSBytes: 1 << 40, MaxReceiptBytes: 1 << 20,
		MaxBrokerBacklog: 100, MaxProjectionLagMS: 100, RecoverySeconds: 60, Workers: 2, WarmupSeconds: 1,
		Feeder: Phase4Command{Executable: "/bin/sh", SHA256: strings.Repeat("d", 64), Args: []string{"-c", "true"}, Environment: map[string]string{"PATH": "/usr/bin"}, Output: filepath.Join(t.TempDir(), "receipt")},
	}
}

func TestPhase4PlanAndSourceBindings(t *testing.T) {
	p := phase4TestPlan(t, "http://127.0.0.1:8080")
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := phase4SourceBindings(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	changes := []struct {
		name string
		edit func(*Phase4Plan)
	}{
		{"mix", func(p *Phase4Plan) { p.GlobalMix["market"]-- }},
		{"source", func(p *Phase4Plan) { p.SourceStream = "bad stream" }},
		{"phase", func(p *Phase4Plan) { p.Phases[2].SourceEventsRPS-- }},
		{"pool", func(p *Phase4Plan) { p.Pools[1].Pool = p.Pools[0].Pool }},
		{"creator", func(p *Phase4Plan) { p.Pools[1].Creator = "invalid" }},
		{"host", func(p *Phase4Plan) { p.Hosts[1].ID = p.Hosts[0].ID }},
		{"generator", func(p *Phase4Plan) { p.GeneratorHostID = p.Hosts[0].ID }},
		{"feeder", func(p *Phase4Plan) { p.Feeder.Output = "relative" }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			copy := p
			copy.GlobalMix = make(map[string]int, len(p.GlobalMix))
			for key, value := range p.GlobalMix {
				copy.GlobalMix[key] = value
			}
			copy.Phases = append([]Phase4Phase(nil), p.Phases...)
			copy.Pools = append([]Phase4Pool(nil), p.Pools...)
			copy.Hosts = append([]Phase4Host(nil), p.Hosts...)
			tc.edit(&copy)
			if err := copy.Validate(); err == nil {
				t.Fatal("changed plan accepted")
			}
		})
	}
	p.Sources[phase4RequiredSources[0]] = strings.Repeat("0", 64)
	if err := phase4SourceBindings(context.Background(), p); err == nil {
		t.Fatal("changed source accepted")
	}
}

func TestPhase4ReadAndHostTelemetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			_, _ = w.Write([]byte("# host\ncpu 0.5\nmemory 1024\n"))
			return
		}
		w.Header().Set("Server-Timing", "origin;dur=1")
		_, _ = w.Write([]byte(`{"quality":"complete","data":{"coverage":"complete"}}`))
	}))
	defer server.Close()
	p := phase4TestPlan(t, server.URL)
	a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	defer func() {
		if err := a.close(); err != nil {
			t.Error(err)
		}
	}()
	for i := range p.Hosts {
		p.Hosts[i].MetricsURL = server.URL + "/metrics"
	}
	if err := phase4Hosts(context.Background(), server.Client(), p, a, "normal"); err != nil {
		t.Fatal(err)
	}
	if err := phase4Generator(p, a, "normal"); err != nil {
		t.Fatal(err)
	}
	phase := Phase4Phase{Name: "normal", Seconds: 1, RPS: 1, HotBPS: 6000}
	summary, err := phase4ReadPhase(context.Background(), server.Client(), p, a, phase)
	if err != nil || summary.Scheduled != 1 || summary.Completed != 1 || summary.Failed != 0 || len(summary.Errors) != 0 {
		t.Fatalf("read summary: %+v, %v", summary, err)
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hosts.ndjson", "generator.ndjson", "http.ndjson"} {
		raw, err := os.ReadFile(filepath.Join(a.directory, name))
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestPhase4BudgetAndArtifactWriting(t *testing.T) {
	p := phase4TestPlan(t, "http://127.0.0.1:8080")
	dir := t.TempDir()
	quote := filepath.Join(dir, "quote.json")
	if err := os.WriteFile(quote, []byte(`{"monthlyMinor":"100"}`), 0600); err != nil {
		t.Fatal(err)
	}
	quoteRaw, err := os.ReadFile(quote)
	if err != nil {
		t.Fatal(err)
	}
	budget := map[string]any{"candidate": p.Candidate, "topology": p.Topology, "currency": "USD", "approvedBy": "operator", "approvedMonthlyMinor": "130", "modeledMonthlyMinor": "100", "contingencyBps": 3000, "validUntil": time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "quotes": []map[string]string{{"path": quote, "sha256": phase4Hash(quoteRaw), "vendor": "database"}}}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	p.BudgetPath = filepath.Join(dir, "budget.json")
	p.BudgetSHA256 = phase4Hash(raw)
	if err := os.WriteFile(p.BudgetPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if err := phase4Budget(p, a); err != nil {
		t.Fatal(err)
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(filepath.Join(a.directory, "budget.ndjson"))
	if err != nil || !strings.Contains(string(result), `"modeledWithContingencyMinor":"130"`) {
		t.Fatalf("budget artifact: %s, %v", result, err)
	}
	t.Setenv("L14_PHASE4_APPROVED_CANDIDATE", p.Candidate)
	t.Setenv("L14_PHASE4_APPROVED_INTERFACE", p.Interface)
	t.Setenv("L14_PHASE4_APPROVED_TOPOLOGY", p.Topology)
	planBytes, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, planBytes, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "evidence")
	if err := RunPhase4Measurement(context.Background(), planPath, phase4Hash(planBytes), output); err == nil {
		t.Fatal("calibration authority missing from plan")
	}
	var measurement map[string]any
	result, err = os.ReadFile(filepath.Join(output, "result.json"))
	if err != nil || json.Unmarshal(result, &measurement) != nil || measurement["failureStage"] != "calibration" {
		t.Fatalf("measurement result: %s %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(output, "artifact-sha256.json")); err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	p.PythonExecutable, err = filepath.EvalSymlinks(python)
	if err != nil {
		t.Fatal(err)
	}
	pythonBytes, err := os.ReadFile(p.PythonExecutable)
	if err != nil {
		t.Fatal(err)
	}
	p.PythonSHA256 = phase4Hash(pythonBytes)
	p.CalibrationPath = filepath.Join(dir, "calibration.json")
	calibration := []byte(`[]`)
	p.CalibrationSHA256 = phase4Hash(calibration)
	if err := os.WriteFile(p.CalibrationPath, calibration, 0600); err != nil {
		t.Fatal(err)
	}
	planBytes, err = json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, planBytes, 0600); err != nil {
		t.Fatal(err)
	}
	output = filepath.Join(dir, "uncalibrated")
	if err := RunPhase4Measurement(context.Background(), planPath, phase4Hash(planBytes), output); err == nil {
		t.Fatal("uncalibrated report accepted")
	}
	calibrationResult, err := os.ReadFile(filepath.Join(output, "calibration.ndjson"))
	if err != nil || !strings.Contains(string(calibrationResult), `"exitSuccess":false`) {
		t.Fatalf("calibration artifact: %s %v", calibrationResult, err)
	}
	budget["approvedMonthlyMinor"] = "129"
	raw, err = json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	p.BudgetSHA256 = phase4Hash(raw)
	if err := os.WriteFile(p.BudgetPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := phase4Budget(p, a); err == nil {
		t.Fatal("over-budget plan accepted")
	}
}
