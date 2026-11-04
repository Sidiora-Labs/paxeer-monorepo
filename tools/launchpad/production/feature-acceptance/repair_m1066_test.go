package featureacceptance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repairM1066Spec = "spec/layerx-merge/spec.kvx"

func repairM1066Root(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func repairM1066JSON(t *testing.T, name string, value any) {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, value); err != nil {
		t.Fatalf("decode published %s: %v", name, err)
	}
}

func repairM1066Sections(t *testing.T) map[string]bool {
	t.Helper()
	graph, err := readReferenceGraph(os.DirFS(repairM1066Root(t)))
	if err != nil {
		t.Fatal(err)
	}
	return graph.sections
}

func repairM1066Report(t *testing.T, name string) ReferenceReport {
	t.Helper()
	report, err := AuditReferenceDocument(os.DirFS(repairM1066Root(t)), declarationDirectory+name)
	if err != nil {
		t.Fatal(err)
	}
	fixture := os.DirFS("testdata/repair-m1066")
	for _, ref := range report.References {
		expected, err := ReconcileReference(fixture, ref.Section)
		if err != nil {
			t.Fatal(err)
		}
		if ref != expected || ref.Class != "retired-reference" || !strings.Contains(ref.Reason, "decision.1803") {
			t.Fatalf("retirement report differs from committed real-path fixture: %+v", ref)
		}
	}
	if report.ReleaseQualified {
		t.Fatal("reference reporting cannot qualify a release")
	}
	if err := report.RequireCurrent(); err == nil || !strings.Contains(err.Error(), "retired release reference cannot qualify") {
		t.Fatalf("retired reference was allowed into qualification: %v", err)
	}
	return report
}

func repairM1066Command(t *testing.T, command, task string, sections map[string]bool) {
	t.Helper()
	expected := "python3 tools/launchpad/production/qualify.py --task " + task + " --spec " + repairM1066Spec + " --require-current-source"
	if command != expected {
		t.Fatalf("published command does not retain task, authoritative source and current-source obligation:\ngot %q\nwant %q", command, expected)
	}
	if sections["task."+task] {
		t.Fatalf("fixture expects retired legacy task %s; an unrelated merged identity must not be aliased", task)
	}
}

func TestRepairM_1066_InventorySpec(t *testing.T) {
	var inventory struct {
		Version  int `json:"version"`
		Features []struct {
			ID                string                                          `json:"id"`
			State             string                                          `json:"state"`
			Requirements      []string                                        `json:"requirements"`
			Owners            []string                                        `json:"owner_lanes"`
			RequirementOwners []string                                        `json:"requirement_owner_lanes"`
			Inherited         []string                                        `json:"inherited_dependency_ids"`
			Prerequisites     []struct{ ID, Kind, Owner, Remediation string } `json:"prerequisites"`
			Scenarios         map[string]string                               `json:"scenarios"`
		} `json:"features"`
	}
	repairM1066JSON(t, "inventory.json", &inventory)
	if inventory.Version != 1 || len(inventory.Features) != 55 {
		t.Fatal("all 55 inventory declarations must be retained")
	}
	sections := repairM1066Sections(t)
	seen := map[string]bool{}
	missing := map[string]bool{}
	for _, feature := range inventory.Features {
		if seen[feature.ID] {
			t.Fatalf("duplicate feature %s", feature.ID)
		}
		seen[feature.ID] = true
		if feature.State != "unqualified" {
			t.Fatalf("declaration qualified feature %s", feature.ID)
		}
		if len(feature.Requirements) == 0 || len(feature.Owners) == 0 || len(feature.RequirementOwners) == 0 || len(feature.Prerequisites) == 0 {
			t.Fatalf("lost lineage/ownership for %s", feature.ID)
		}
		for _, scenario := range []string{"success", "authorization_denial", "dependency_outage", "reconnect_retry"} {
			if feature.Scenarios[scenario] == "" {
				t.Fatalf("%s missing %s", feature.ID, scenario)
			}
		}
		if len(feature.Scenarios) != 4 {
			t.Fatalf("%s scenario coverage changed", feature.ID)
		}
		for _, requirement := range feature.Requirements {
			if !sections["req."+requirement] {
				missing[requirement] = true
			}
		}
		deps := map[string]bool{}
		for _, dependency := range feature.Prerequisites {
			if dependency.ID == "" || dependency.Kind == "" || dependency.Owner == "" || dependency.Remediation == "" {
				t.Fatalf("lost dependency obligation for %s", feature.ID)
			}
			deps[dependency.ID] = true
		}
		for _, inherited := range feature.Inherited {
			if !deps[feature.ID+":"+inherited] {
				t.Fatalf("lost inherited %s dependency for %s", inherited, feature.ID)
			}
		}
	}
	for _, family := range []struct {
		prefix string
		count  int
	}{{"PF", 30}, {"OC", 25}} {
		for i := 1; i <= family.count; i++ {
			if !seen[fmt.Sprintf("%s%02d", family.prefix, i)] {
				t.Fatalf("missing %s%02d", family.prefix, i)
			}
		}
	}
	report := repairM1066Report(t, "inventory-spec-references.json")
	expected := map[string]bool{}
	for id := range seen {
		expected["coverage."+id] = true
	}
	for id := range missing {
		expected["req."+id] = true
	}
	if len(report.References) != len(expected) {
		t.Fatalf("lost inventory reference: got %d, want %d", len(report.References), len(expected))
	}
	for _, ref := range report.References {
		if !expected[ref.Section] {
			t.Fatalf("duplicate or unexpected inventory reference %s", ref.Section)
		}
		delete(expected, ref.Section)
	}
	if len(expected) != 0 {
		t.Fatal("inventory references were omitted")
	}
	var published, historical any
	repairM1066JSON(t, "inventory.json", &published)
	repairM1066JSON(t, "fixtures/retired-inventory.json", &historical)
	publishedJSON, _ := json.Marshal(published)
	historicalJSON, _ := json.Marshal(historical)
	if string(publishedJSON) != string(historicalJSON) {
		t.Fatal("retirement changed inventory ownership, lineage or scenarios")
	}

}

func repairM1066Phase(t *testing.T, phase int, cases []string) {
	t.Helper()
	var manifest struct {
		Task          string `json:"task"`
		Phase         int    `json:"phase"`
		FinalGate     string `json:"final_gate"`
		VerifyCommand string `json:"verify_cmd"`
		Qualified     bool   `json:"release_qualified"`
		Cases         []struct {
			ID string `json:"id"`
		} `json:"required_native_cases"`
	}
	repairM1066JSON(t, fmt.Sprintf("phase-%d-scenario-manifest.json", phase), &manifest)
	if manifest.Task != fmt.Sprintf("%d.50", phase) || manifest.Phase != phase || manifest.Qualified {
		t.Fatal("phase identity or declaration boundary changed")
	}
	if phase == 4 && manifest.FinalGate != "9.1" {
		t.Fatal("final gate 9.1 must remain mandatory")
	}
	if len(manifest.Cases) != len(cases) {
		t.Fatal("scenario coverage changed")
	}
	for i, expected := range cases {
		if manifest.Cases[i].ID != expected {
			t.Fatalf("scenario %d: got %s, want %s", i, manifest.Cases[i].ID, expected)
		}
	}
	repairM1066Command(t, manifest.VerifyCommand, manifest.Task, repairM1066Sections(t))
	report := repairM1066Report(t, fmt.Sprintf("phase-%d-scenario-manifest.json", phase))
	if len(report.References) != 1 || report.References[0].Section != "task."+manifest.Task {
		t.Fatal("phase reference report lost task identity")
	}
}

func TestRepairM_1066_Phase2ManifestSpec(t *testing.T) {
	repairM1066Phase(t, 2, []string{"PF21-real-vendor-chain125-fund-withdraw", "PF24-physical-native-push-deep-link", "PF27-voice-provider-blocker"})
}

func TestRepairM_1066_Phase3ManifestSpec(t *testing.T) {
	repairM1066Phase(t, 3, []string{"one_byte_tamper", "missing_service_trace", "reviewer_conflict", "stale_thirty_day_evidence", "critical_high_findings", "mandatory_feature_blockers", "secret_redaction_identity"})
}

func TestRepairM_1066_Phase4ManifestSpec(t *testing.T) {
	repairM1066Phase(t, 4, []string{"all55-review-schema-mapping", "isolated-native-evidence-calibration", "missing-task9-final-evidence", "unsupported-mandatory-feature"})
}

type repairM1066Gate struct {
	Command  string   `json:"verify_cmd"`
	Requires []string `json:"requires"`
	Kind     string   `json:"kind"`
}

type repairM1066Readiness struct {
	Task              string                     `json:"task"`
	VerifyCommand     string                     `json:"verify_cmd"`
	Scope             string                     `json:"scope"`
	FinalGate         string                     `json:"final_gate"`
	Gates             map[string]repairM1066Gate `json:"mandatory_gates"`
	Features          []string                   `json:"feature_ids"`
	EvidenceState     string                     `json:"final_integrated_evidence_state"`
	ValidatorReady    bool                       `json:"validator_ready"`
	CapacityQualified bool                       `json:"capacity_qualified"`
	ReleaseQualified  bool                       `json:"release_qualified"`
}

func repairM1066ReadinessDeclaration(t *testing.T) repairM1066Readiness {
	t.Helper()
	var manifest repairM1066Readiness
	repairM1066JSON(t, "final-integrated-gate-readiness.json", &manifest)
	if manifest.Task != "4.50" {
		t.Fatal("readiness declaration task identity changed")
	}
	repairM1066Command(t, manifest.VerifyCommand, manifest.Task, repairM1066Sections(t))
	if manifest.Scope != "declaration_only" || manifest.FinalGate != "9.1" || manifest.EvidenceState != "pending" || manifest.ValidatorReady || manifest.CapacityQualified || manifest.ReleaseQualified {
		t.Fatal("declarations must not establish readiness, capacity or release qualification")
	}
	if len(manifest.Features) != 55 || len(manifest.Gates) != 208 {
		t.Fatal("all 55 features and all 208 lane/serial obligations must remain")
	}
	for phase := 1; phase <= 4; phase++ {
		for lane := 1; lane <= 50; lane++ {
			id := fmt.Sprintf("%d.%02d", phase, lane)
			gate, ok := manifest.Gates[id]
			if !ok || gate.Kind != "parallel_lane" {
				t.Fatalf("lost lane obligation %s", id)
			}
			prior := "0.3"
			if phase > 1 {
				prior = fmt.Sprintf("%d.99", phase-1)
			}
			if len(gate.Requires) != 1 || gate.Requires[0] != prior {
				t.Fatalf("lost preceding serial gate for %s", id)
			}
		}
	}
	for _, id := range []string{"0.1", "0.2", "0.3", "1.99", "2.99", "3.99", "4.99", "9.1"} {
		if manifest.Gates[id].Kind != "serial_gate" {
			t.Fatalf("lost serial gate %s", id)
		}
	}
	for phase := 1; phase <= 4; phase++ {
		gate := manifest.Gates[fmt.Sprintf("%d.99", phase)]
		if len(gate.Requires) != 50 {
			t.Fatal("serial gate must join every lane")
		}
		for lane := 1; lane <= 50; lane++ {
			if gate.Requires[lane-1] != fmt.Sprintf("%d.%02d", phase, lane) {
				t.Fatal("serial gate lane join changed")
			}
		}
	}
	if len(manifest.Gates["9.1"].Requires) != 1 || manifest.Gates["9.1"].Requires[0] != "4.99" {
		t.Fatal("final gate must require all phase gates")
	}
	return manifest
}

func TestRepairM_1066_FinalManifestSpec(t *testing.T) {
	manifest := repairM1066ReadinessDeclaration(t)
	report := repairM1066Report(t, "final-integrated-gate-readiness.json")
	if len(report.References) != len(manifest.Gates) {
		t.Fatal("retirement report lost a lane or serial gate")
	}
	sections := repairM1066Sections(t)
	// Retain the final boundary and every historical command in the report.
	repairM1066Command(t, manifest.Gates["9.1"].Command, "9.1", sections)
	for id, gate := range manifest.Gates {
		if id != "9.1" {
			repairM1066Command(t, gate.Command, id, sections)
		}
	}
}

func TestRepairM_1066_ReadinessSpec(t *testing.T) {
	manifest := repairM1066ReadinessDeclaration(t)
	report := repairM1066Report(t, "final-integrated-gate-readiness.json")
	if len(report.References) != 208 {
		t.Fatal("all historical lane/serial references must be reported")
	}
	seen := map[string]bool{}
	for _, ref := range report.References {
		id := strings.TrimPrefix(ref.Section, "task.")
		if _, ok := manifest.Gates[id]; !ok || seen[id] {
			t.Fatalf("unexpected or duplicate readiness reference %s", id)
		}
		seen[id] = true
	}
	if !seen["9.1"] {
		t.Fatal("final gate retirement must be explicit")
	}
}

func TestRepairM_1066_ReferenceIntegrity(t *testing.T) {
	fixture := os.DirFS("testdata/repair-m1066")
	current, err := ReconcileReference(fixture, "task.22.33.1066")
	if err != nil || current.Class != "current-reference" {
		t.Fatalf("current identity misclassified: %+v, %v", current, err)
	}
	if err := (ReferenceReport{References: []SpecReference{current}}).RequireCurrent(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileReference(os.DirFS(t.TempDir()), "task.9.1"); err == nil {
		t.Fatal("missing authoritative source must fail")
	}
	if _, err := ReconcileReference(fixture, "task.22.33.999999"); err == nil {
		t.Fatal("unknown current identity must not be relabeled as retired")
	}
	original, err := os.ReadFile("phase-4-scenario-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"wrong-spec", "missing-current-source", "false-current", "wrong-reason", "qualified"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, fixture); err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(original, &doc); err != nil {
				t.Fatal(err)
			}
			ref := doc["spec_reference"].(map[string]any)
			switch mutation {
			case "wrong-spec":
				ref["spec"] = RetiredSpecPath
			case "missing-current-source":
				doc["verify_cmd"] = strings.TrimSuffix(doc["verify_cmd"].(string), " --require-current-source")
			case "false-current":
				ref["class"] = "current-reference"
			case "wrong-reason":
				ref["reason"] = "mapped by another domain"
			case "qualified":
				doc["release_qualified"] = true
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := AuditReferenceDocument(os.DirFS(root), "manifest.json"); err == nil {
				t.Fatalf("accepted %s", mutation)
			}
		})
	}
}
