package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func bindMeasurementPlan(t *testing.T, plan InventoryMeasurementPlan) InventoryMeasurementPlan {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := decodeInventoryMeasurementPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

func measurementPlanFixture(t *testing.T) (*LiveTransferWorker, InventoryMeasurementPlan) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	plan := InventoryMeasurementPlan{Version: 1, RunID: "inventory-plan", ReleaseDigest: strings.Repeat("c", 64), SourceRoot: root, Region: "local", APIOrigin: "http://localhost", RequestsPerSecond: 800, Concurrency: 4, RequestTimeoutMillis: 1000, MaxDurationSeconds: 10, MaxArtifactBytes: 32 * 1024 * 1024, MaxHeapAllocBytes: 1 << 40, MaxPoolAcquired: 32, MaxArchiveRecords: 1000000}
	for _, path := range requiredInventorySources() {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		digest := delivery.Hash(raw)
		plan.SourceFiles = append(plan.SourceFiles, InventoryMeasurementFile{Path: path, SHA256: digest})
		if path == inventoryFrozenAuthority {
			plan.InterfaceDigest = digest
		}
	}
	path := filepath.Join(t.TempDir(), "inventory.json")
	raw := []byte(`{"algorithm":"inventory-v1"}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	file := InventoryMeasurementFile{Path: path, SHA256: delivery.Hash(raw)}
	plan.CandidateManifest, plan.Topology, plan.Dependencies = file, file, file
	plan.CandidateDigest = file.SHA256
	t.Setenv("INVENTORY_PLAN_ACCESS", "owner-access")
	leg := testLeg(1)
	for i := 1; i <= 11; i++ {
		state := Empty()
		c := InventoryMeasurementCase{ID: fmt.Sprintf("case-%d", i), Scenario: i, Wallet: leg.Wallet, Token: leg.Token, AccessTokenEnv: "INVENTORY_PLAN_ACCESS", Operation: "active", Height: 100, BlockHash: "0x" + strings.Repeat("b", 64), ExpectedState: &state, Requests: 1}
		if i == 3 {
			c.ExpectedNormalizedAverage = str("0")
		}
		if i == 8 {
			c.Requests = 100
		}
		if i == 10 {
			c.Operation = operationHTTPPortfolio
			c.PricingQuality = statusUnpriced
		}
		plan.Cases = append(plan.Cases, c)
	}
	worker := &LiveTransferWorker{service: &Service{authority: &BaselineAuthority{}, release: plan.ReleaseDigest}, identity: auth.HTTPIdentityAuthority{}}
	return worker, bindMeasurementPlan(t, plan)
}

func TestInventoryMeasurementPlanBoundsAndBindings(t *testing.T) {
	worker, base := measurementPlanFixture(t)
	if err := validateInventoryMeasurementPlan(worker, base); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(*InventoryMeasurementPlan)
	}{
		{"version", func(p *InventoryMeasurementPlan) { p.Version = 2 }},
		{"run", func(p *InventoryMeasurementPlan) { p.RunID = "../run" }},
		{"release", func(p *InventoryMeasurementPlan) { p.ReleaseDigest = strings.Repeat("d", 64) }},
		{"root", func(p *InventoryMeasurementPlan) { p.SourceRoot = "relative" }},
		{"origin", func(p *InventoryMeasurementPlan) { p.APIOrigin = "http://example.org" }},
		{"origin credentials", func(p *InventoryMeasurementPlan) { p.APIOrigin = "https://user:password@localhost" }},
		{"origin query", func(p *InventoryMeasurementPlan) { p.APIOrigin = "https://localhost?token=x" }},
		{"source escape", func(p *InventoryMeasurementPlan) { p.SourceFiles[0].Path = "../source.go" }},
		{"source duplicate", func(p *InventoryMeasurementPlan) { p.SourceFiles = append(p.SourceFiles, p.SourceFiles[0]) }},
		{"source missing", func(p *InventoryMeasurementPlan) { p.SourceFiles = p.SourceFiles[1:] }},
		{"source digest", func(p *InventoryMeasurementPlan) { p.SourceFiles[0].SHA256 = strings.Repeat("0", 64) }},
		{"interface", func(p *InventoryMeasurementPlan) { p.InterfaceDigest = strings.Repeat("e", 64) }},
		{"candidate", func(p *InventoryMeasurementPlan) { p.CandidateDigest = strings.Repeat("e", 64) }},
		{"dependency", func(p *InventoryMeasurementPlan) { p.Dependencies.SHA256 = strings.Repeat("e", 64) }},
		{"duplicate case", func(p *InventoryMeasurementPlan) { p.Cases[1].ID = p.Cases[0].ID }},
		{"state", func(p *InventoryMeasurementPlan) { p.Cases[0].ExpectedState = &Inventory{Known: "-1"} }},
		{"unrealized", func(p *InventoryMeasurementPlan) { p.Cases[0].ExpectedUnrealized = str("1.5") }},
		{"normalized expectation", func(p *InventoryMeasurementPlan) { p.Cases[2].ExpectedNormalizedAverage = nil }},
		{"operation", func(p *InventoryMeasurementPlan) { p.Cases[0].Operation = "other" }},
		{"expected error", func(p *InventoryMeasurementPlan) { p.Cases[0].ExpectedError = "unexpected" }},
		{"day format", func(p *InventoryMeasurementPlan) {
			p.Cases[0].Operation = periodHistoricalDay
			p.Cases[0].Day = "today"
		}},
		{"day state missing", func(p *InventoryMeasurementPlan) {
			p.Cases[0].Operation = periodHistoricalDay
			p.Cases[0].Day = "2026-10-10"
		}},
		{"day state invalid", func(p *InventoryMeasurementPlan) {
			p.Cases[0].Operation = periodHistoricalDay
			p.Cases[0].Day = "2026-10-10"
			p.Cases[0].ExpectedDayState = &Inventory{Known: "-1"}
		}},
		{"price operation", func(p *InventoryMeasurementPlan) { p.Cases[9].Operation = "active" }},
		{"price quality", func(p *InventoryMeasurementPlan) { p.Cases[0].PricingQuality = "invented" }},
		{"duplicate requests", func(p *InventoryMeasurementPlan) { p.Cases[7].Requests = 99 }},
		{"scenario coverage", func(p *InventoryMeasurementPlan) { p.Cases[10].Scenario = 1 }},
		{"workload duration", func(p *InventoryMeasurementPlan) { p.RequestsPerSecond = 1; p.MaxDurationSeconds = 1 }},
		{"workload count", func(p *InventoryMeasurementPlan) { p.Cases[0].Requests = 100000 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := bindMeasurementPlan(t, base)
			tc.change(&p)
			p = bindMeasurementPlan(t, p)
			if err := validateInventoryMeasurementPlan(worker, p); err == nil {
				t.Fatal("invalid measurement plan accepted")
			}
		})
	}
	altered := base
	altered.Region = "changed"
	if err := validateInventoryMeasurementPlan(worker, altered); err == nil {
		t.Fatal("unbound typed input accepted")
	}
	if err := validateInventoryMeasurementPlan(nil, base); err == nil {
		t.Fatal("missing worker accepted")
	}
	p := bindMeasurementPlan(t, base)
	p.Cases[0].Operation = periodHistoricalDay
	p.Cases[0].Day = "2026-10-10"
	s := Empty()
	p.Cases[0].ExpectedDayState = &s
	if err := validateInventoryMeasurementPlan(worker, bindMeasurementPlan(t, p)); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryMeasurementFilesAndJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "plan.json")
	raw := []byte(`{"version":1,"cases":[]}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := LoadInventoryMeasurementPlan(path)
	if err != nil || plan.Version != 1 || plan.InputPlanSHA256 != delivery.Hash(raw) || !bytes.Equal(plan.InputPlanRaw, raw) {
		t.Fatalf("plan=%+v error=%v", plan, err)
	}
	for _, raw := range []string{`{"version":1,"version":2}`, `{"unknown":1}`, `[]`, `{"cases":[{"id":"a","id":"b"}]}`, `{`, strings.Repeat("[", 130) + strings.Repeat("]", 130)} {
		if _, err := decodeInventoryMeasurementPlan([]byte(raw)); err == nil {
			t.Fatalf("invalid plan accepted: %.100s", raw)
		}
	}
	if _, err := decodeInventoryMeasurementPlan(bytes.Repeat([]byte(" "), 4*1024*1024+1)); err == nil {
		t.Fatal("oversized plan accepted")
	}
	if _, err := LoadInventoryMeasurementPlan(filepath.Join(root, "absent")); err == nil {
		t.Fatal("missing plan accepted")
	}
	file := InventoryMeasurementFile{Path: "plan.json", SHA256: delivery.Hash(raw)}
	if got, err := measurementReadFile(file, root); err != nil || !bytes.Equal(got, raw) {
		t.Fatal(string(got), err)
	}
	for _, file := range []InventoryMeasurementFile{{Path: "plan.json"}, {SHA256: delivery.Hash(raw)}, {Path: root, SHA256: delivery.Hash(raw)}, {Path: path, SHA256: strings.Repeat("0", 64)}, {Path: filepath.Join(root, "absent"), SHA256: delivery.Hash(raw)}} {
		if _, err := measurementReadFile(file, root); err == nil {
			t.Fatal("unbound file accepted", file)
		}
	}
	if digest, err := measurementFileSHA256(path); err != nil || digest != delivery.Hash(raw) {
		t.Fatal(digest, err)
	}
	if _, err := measurementFileSHA256(filepath.Join(root, "absent")); err == nil {
		t.Fatal("missing file hash accepted")
	}
	if _, err := RunInventoryMeasurement(context.Background(), nil, nil, InventoryMeasurementPlan{}, root); err == nil {
		t.Fatal("missing authority accepted")
	}
	worker, p := measurementPlanFixture(t)
	if _, err := RunInventoryMeasurement(context.Background(), worker, nil, p, root); err == nil {
		t.Fatal("unbounded client accepted")
	}
	for _, tc := range []struct {
		header string
		want   int64
		valid  bool
	}{{"portfolio_origin;dur=0", 0, true}, {"portfolio_origin;dur=1.234567", 1234567, true}, {"", 0, false}, {"portfolio_origin;dur=-1", 0, false}, {"portfolio_origin;dur=01", 0, false}, {"portfolio_origin;dur=99999999999999", 0, false}} {
		got, err := inventoryOriginTiming(tc.header)
		if (err == nil) != tc.valid || tc.valid && got != tc.want {
			t.Fatal(tc.header, got, err)
		}
	}
}

func TestMeasurementJSONRejectsTruncatedObjectsAndUnexpectedClosers(t *testing.T) {
	for _, raw := range []string{"", "}", `{"`, `{"key":`, `[1,`} {
		if err := uniqueInventoryMeasurementJSON(json.NewDecoder(strings.NewReader(raw)), 0); err == nil {
			t.Fatalf("incomplete measurement JSON accepted: %q", raw)
		}
	}
}
