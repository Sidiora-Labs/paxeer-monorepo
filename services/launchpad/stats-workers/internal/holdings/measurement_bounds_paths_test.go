package holdings

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhase4PlanRejectsIncompleteCapacityInputs(t *testing.T) {
	base := phase4TestPlan(t, "http://127.0.0.1:8080")
	for _, tc := range []struct {
		name string
		edit func(*Phase4Plan)
	}{
		{"origin", func(p *Phase4Plan) { p.Origin = "http://example.com" }},
		{"holder schema", func(p *Phase4Plan) { p.HolderSchema = "invalid-schema" }},
		{"source count", func(p *Phase4Plan) {
			for i := 0; i < 2049; i++ {
				p.Sources[strings.Repeat("x", i+1)] = "a"
			}
		}},
		{"mix size", func(p *Phase4Plan) { delete(p.GlobalMix, "chart") }},
		{"phase count", func(p *Phase4Plan) { p.Phases = p.Phases[:3] }},
		{"duration", func(p *Phase4Plan) { p.Phases[3].Seconds = 61 }},
		{"phase rate", func(p *Phase4Plan) { p.Phases[0].RPS = 0 }},
		{"token identity", func(p *Phase4Plan) { p.Pools[0].Token = Zero }},
		{"metrics origin", func(p *Phase4Plan) { p.Hosts[0].MetricsURL = "http://example.com" }},
		{"host role", func(p *Phase4Plan) { p.Hosts = p.Hosts[:3] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var plan Phase4Plan
			if err := json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			tc.edit(&plan)
			if err := plan.Validate(); err == nil {
				t.Fatal("invalid capacity input accepted")
			}
		})
	}
}

func TestPhase4TelemetryRejectsUnavailableAndOversizedResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       string
	}{
		{"unavailable", "", 503, "host telemetry unavailable"},
		{"missing metric", "cpu -1\n", 200, "required measured host metric absent"},
		{"oversized metric", strings.Repeat("x", 1048577), 200, "HTTP byte budget exceeded"},
		{"oversized line", strings.Repeat("x", 1048576), 200, "token too long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			plan := Phase4Plan{Hosts: []Phase4Host{{ID: "worker", Role: "worker", MetricsURL: server.URL, RequiredMetrics: []string{"cpu"}}}}
			a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
			defer a.close()
			if err := phase4Hosts(context.Background(), server.Client(), plan, a, "normal"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("host input: %v", err)
			}
		})
	}
	if _, _, _, err := phase4HTTP(context.Background(), http.DefaultClient, ":"); err == nil {
		t.Fatal("invalid HTTP URL accepted")
	}
	plan := phase4TestPlan(t, "http://127.0.0.1:8080")
	plan.MaxGeneratorHeapBytes = 1
	a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	defer a.close()
	if err := phase4Generator(plan, a, "normal"); err == nil || !strings.Contains(err.Error(), "heap") {
		t.Fatalf("heap bound: %v", err)
	}
	plan.MaxGeneratorHeapBytes = math.MaxUint64
	plan.MaxGeneratorRSSBytes = 1
	if err := phase4Generator(plan, a, "normal"); err == nil || !strings.Contains(err.Error(), "RSS") {
		t.Fatalf("RSS bound: %v", err)
	}
}

func TestPhase4BudgetRejectsUnboundVendorAmounts(t *testing.T) {
	plan := phase4TestPlan(t, "http://127.0.0.1:8080")
	quote := filepath.Join(t.TempDir(), "quote.json")
	if err := os.WriteFile(quote, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"expired", func(b map[string]any) { b["validUntil"] = time.Now().Add(-time.Hour).Format(time.RFC3339) }, "candidate-bound"},
		{"approved integer", func(b map[string]any) { b["approvedMonthlyMinor"] = "invalid" }, "invalid approved"},
		{"modeled integer", func(b map[string]any) { b["modeledMonthlyMinor"] = "0" }, "invalid modeled"},
		{"vendor", func(b map[string]any) {
			b["quotes"] = []map[string]string{{"path": quote, "sha256": phase4Hash([]byte(`{}`)), "vendor": ""}}
		}, "vendor quote authority absent"},
		{"quote digest", func(b map[string]any) {
			b["quotes"] = []map[string]string{{"path": quote, "sha256": strings.Repeat("0", 64), "vendor": "database"}}
		}, "checksum mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := map[string]any{"candidate": plan.Candidate, "topology": plan.Topology, "approvedBy": "operator", "currency": "USD", "approvedMonthlyMinor": "130", "modeledMonthlyMinor": "100", "contingencyBps": 3000, "validUntil": time.Now().Add(time.Hour).Format(time.RFC3339), "quotes": []map[string]string{{"path": quote, "sha256": phase4Hash([]byte(`{}`)), "vendor": "database"}}}
			tc.edit(b)
			raw, err := json.Marshal(b)
			if err != nil {
				t.Fatal(err)
			}
			p := plan
			p.BudgetPath = filepath.Join(t.TempDir(), "budget.json")
			p.BudgetSHA256 = phase4Hash(raw)
			if err := os.WriteFile(p.BudgetPath, raw, 0600); err != nil {
				t.Fatal(err)
			}
			a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
			defer a.close()
			if err := phase4Budget(p, a); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("budget authority: %v", err)
			}
		})
	}
}

func TestPhase4FeederEnforcesReceiptAndEnvironmentBounds(t *testing.T) {
	executable, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("L14_PHASE4_FEED_ADMITTED", "YES")
	for _, tc := range []struct {
		name string
		edit func(*Phase4Plan)
		want string
	}{
		{"executable binding", func(p *Phase4Plan) { p.Feeder.SHA256 = strings.Repeat("0", 64) }, "checksum mismatch"},
		{"existing receipt", func(p *Phase4Plan) { p.Feeder.Output = p.Feeder.Args[0] }, "fresh real feeder receipt"},
		{"invalid environment", func(p *Phase4Plan) { p.Feeder.Environment = map[string]string{"bad key": "value"} }, "invalid explicit feeder environment"},
		{"command rejection", func(p *Phase4Plan) { p.Feeder.Args = []string{"--invalid-option"} }, "actual source feeder failed"},
		{"receipt size", func(p *Phase4Plan) { p.MaxReceiptBytes = 1 }, "storage envelope"},
		{"receipt integer", func(p *Phase4Plan) { p.MaxReceiptBytes = math.MaxUint64 }, "storage envelope invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			output := filepath.Join(dir, "receipt")
			if err := os.WriteFile(source, []byte("source receipt\n"), 0600); err != nil {
				t.Fatal(err)
			}
			p := Phase4Plan{SourceRoot: dir, MaxReceiptBytes: 1024, Feeder: Phase4Command{Executable: executable, SHA256: phase4Hash(raw), Args: []string{source, output}, Environment: map[string]string{"LANG": "C"}, Output: output}}
			tc.edit(&p)
			a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
			defer a.close()
			if err := phase4Feeder(context.Background(), p, a); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("feeder bounds: %v", err)
			}
		})
	}
}
