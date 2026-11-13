package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/ethereum/go-ethereum/common"
)

func TestMeasurementCommandRequiresPinnedAuthority(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	buildDigest := repairM951Digest(binary)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	const urlVariable = "MEASUREMENT_COVERAGE_URL"
	t.Setenv(urlVariable, server.URL)
	previousFlags, previousArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = previousFlags, previousArgs })
	call := func(t *testing.T, plan capabilities.MeasurementPlan, approved []byte, path, digest string, extra ...string) error {
		t.Helper()
		root := t.TempDir()
		planFile := filepath.Join(root, "plan.json")
		if path == "" {
			path = planFile
		}
		if approved != nil {
			authorityFile := filepath.Join(root, "authority.json")
			if err := os.WriteFile(authorityFile, approved, 0600); err != nil {
				t.Fatal(err)
			}
			plan.AuthoritySHA256 = repairM951Digest(approved)
			os.Args = []string{"measure", "-plan", path, "-authority", authorityFile}
		} else {
			os.Args = []string{"measure", "-plan", path}
		}
		if path == planFile {
			wire, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(planFile, wire, 0600); err != nil {
				t.Fatal(err)
			}
			if digest == "" {
				digest = repairM951Digest(wire)
			}
		}
		os.Args = append(os.Args, "-plan-sha256", digest)
		os.Args = append(os.Args, extra...)
		flag.CommandLine = flag.NewFlagSet("measure", flag.ContinueOnError)
		return run()
	}
	base := capabilities.MeasurementPlan{HarnessBuildSHA256: buildDigest}
	repairM951Issue(t, call(t, base, nil, filepath.Join(t.TempDir(), "absent.json"), strings.Repeat("a", 64)), "pinned_file_unavailable")
	repairM951Issue(t, call(t, base, nil, "", strings.Repeat("a", 64)), "pinned_file_digest_mismatch")
	changedBuild := base
	changedBuild.HarnessBuildSHA256 = strings.Repeat("0", 64)
	repairM951Issue(t, call(t, changedBuild, nil, "", ""), "harness_build_identity_mismatch")
	repairM951Issue(t, call(t, base, []byte(`{"unknown":true}`), "", ""), valueMeasurementInputMalformed)
	approved := authority{SchemaVersion: 1, Realm: "private_fixture", EndpointID: "chain", URLVariable: urlVariable, URLSHA256: repairM951Digest([]byte(server.URL)), GenesisHash: common.HexToHash("0x01")}
	encode := func(t *testing.T, value authority) []byte {
		t.Helper()
		wire, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return wire
	}
	badSchema := approved
	badSchema.SchemaVersion = 2
	repairM951Issue(t, call(t, base, encode(t, badSchema), "", ""), "endpoint_authority_invalid")
	badURL := approved
	badURL.URLSHA256 = strings.Repeat("0", 64)
	repairM951Issue(t, call(t, base, encode(t, badURL), "", ""), "approved_endpoint_unavailable")
	badRealm := approved
	badRealm.Realm = "unlisted"
	repairM951Issue(t, call(t, base, encode(t, badRealm), "", ""), "endpoint_realm_invalid")
	production := approved
	production.Realm = "production"
	repairM951Issue(t, call(t, base, encode(t, production), "", ""), "approved_endpoint_transport_invalid")
	repairM951Issue(t, call(t, base, encode(t, approved), "", ""), "pinned_file_input_missing")
	valid := base
	valid.SamplesPerCase, valid.Concurrency, valid.DeadlineMilliseconds = 1, 1, 1000
	valid.P95BudgetMicroseconds, valid.MaxRawBytes = 1000, 1<<20
	policy := budget{SamplesPerCase: 1, Concurrency: 1, DeadlineMilliseconds: 1000, P95BudgetMicroseconds: 1000, MaxRawBytes: 1 << 20, ReadDeadlineMilliseconds: 1000, AttestationMaxAgeSeconds: 60}
	policyWire, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	policyFile := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(policyFile, policyWire, 0600); err != nil {
		t.Fatal(err)
	}
	valid.PolicySHA256 = repairM951Digest(policyWire)
	repairM951Issue(t, call(t, valid, encode(t, approved), "", "", "-policy", policyFile), "pinned_file_input_missing")
	wrongBudget := policy
	wrongBudget.SamplesPerCase = 2
	wrongWire, err := json.Marshal(wrongBudget)
	if err != nil {
		t.Fatal(err)
	}
	wrongFile := filepath.Join(t.TempDir(), "wrong-policy.json")
	if err := os.WriteFile(wrongFile, wrongWire, 0600); err != nil {
		t.Fatal(err)
	}
	wrongPlan := valid
	wrongPlan.PolicySHA256 = repairM951Digest(wrongWire)
	repairM951Issue(t, call(t, wrongPlan, encode(t, approved), "", "", "-policy", wrongFile), "approved_measurement_budget_mismatch")
	root := repairM951Candidate(t)
	_, sourcePath, sourceDigest := repairM951Index(t, root)
	valid.CandidateSHA256 = sourceDigest
	repairM951Issue(t, call(t, valid, encode(t, approved), "", "", "-policy", policyFile, "-source-root", root, "-source-index", sourcePath), "pinned_file_input_missing")
	manifestPath := filepath.Join(repairM951Root(t), "sdk/launchpad/shared/config/deployment/current.json")
	manifestWire, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	approved.ManifestFileSHA256 = repairM951Digest(manifestWire)
	repairM951Issue(t, call(t, valid, encode(t, approved), "", "", "-policy", policyFile, "-source-root", root, "-source-index", sourcePath, "-manifest", manifestPath), "pinned_file_input_missing")
	buildExport, err := json.Marshal(capabilities.ConsumerBuildIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	exportFile := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(exportFile, buildExport, 0600); err != nil {
		t.Fatal(err)
	}
	approved.GoBuildExportSHA256 = repairM951Digest(buildExport)
	approved.FrontendBuildExportSHA256 = approved.GoBuildExportSHA256
	repairM951Issue(t, call(t, valid, encode(t, approved), "", "", "-policy", policyFile, "-source-root", root, "-source-index", sourcePath, "-manifest", manifestPath, "-go-export", exportFile, "-frontend-export", exportFile), "consumer_build_artifact_missing")
	if err := checkBuild(executable, buildDigest); err != nil {
		t.Fatalf("actual command build identity: %v", err)
	}
	repairM951Issue(t, checkBuild(executable, strings.Repeat("0", 64)), "consumer_build_artifact_mismatch")
	actualBuild := capabilities.ConsumerBuildIdentity{BuildSHA256: buildDigest}
	actualExport, err := json.Marshal(actualBuild)
	if err != nil {
		t.Fatal(err)
	}
	actualExportFile := filepath.Join(t.TempDir(), "actual-export.json")
	if err := os.WriteFile(actualExportFile, actualExport, 0600); err != nil {
		t.Fatal(err)
	}
	approved.GoBuildExportSHA256 = repairM951Digest(actualExport)
	approved.FrontendBuildExportSHA256 = approved.GoBuildExportSHA256
	valid.GoBuild, valid.FrontendBuild = actualBuild, actualBuild
	repairM951Issue(t, call(t, valid, encode(t, approved), "", "", "-policy", policyFile, "-source-root", root, "-source-index", sourcePath, "-manifest", manifestPath, "-go-export", actualExportFile, "-frontend-export", actualExportFile, "-go-build", executable, "-frontend-build", executable), "approved_endpoint_chain_mismatch")
}
