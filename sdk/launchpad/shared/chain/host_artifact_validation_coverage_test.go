package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
)

func TestMeasurementRejectsChangedHostAndCompatibilityArtifacts(t *testing.T) {
	topology := boundMeasurementFile(t, "topology.json", []byte(`{"hosts":4}`))
	environment := boundMeasurementFile(t, "environment.json", []byte(`{"chainId":125}`))
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	build, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(build)
	digest := hex.EncodeToString(sum[:])
	binding := readpolicy.MeasurementBinding{CandidateSHA256: digest, InterfaceSHA256: digest, ManifestSHA256: digest, TopologySHA256: topology.SHA256, EnvironmentSHA256: environment.SHA256, BuildSHA256: digest, WorkloadSeed: "artifact-validation", ProfileVersion: "e1"}
	started, ended := time.Now().Add(-time.Minute), time.Now()
	bindings := map[string]string{"candidate": digest, "interface": digest, "topology": topology.SHA256, "generator": digest}
	encode := func(v any) []byte {
		t.Helper()
		wire, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return wire
	}
	for _, tc := range []struct{ name, code string }{
		{"missing manifest", "MISSING_WORKLOAD_RAW_MANIFEST"},
		{"malformed manifest", "MALFORMED_WORKLOAD_RAW_MANIFEST"},
		{"trailing manifest", "MALFORMED_WORKLOAD_RAW_MANIFEST"},
		{"missing capture", "MISSING_WORKLOAD_RAW_CAPTURE"},
		{"changed capture", "WORKLOAD_RAW_CAPTURE_CHANGED"},
		{"trailing report", "MALFORMED_WORKLOAD_HOST_REPORT"},
		{"missing compatibility", "MISSING_COMPATIBILITY_EVIDENCE"},
		{"malformed compatibility", "MALFORMED_COMPATIBILITY_EVIDENCE"},
		{"trailing compatibility", "COMPATIBILITY_BINDING_MISMATCH"},
		{"foreign compatibility", "COMPATIBILITY_BINDING_MISMATCH"},
		{"failed scenario", "COMPATIBILITY_SCENARIO_FAILED"},
		{"missing scenario raw", "COMPATIBILITY_RAW_EVIDENCE_MISSING"},
		{"malformed scenario raw", "COMPATIBILITY_RAW_EVIDENCE_MALFORMED"},
		{"skipped scenario", "COMPATIBILITY_REAL_PATH_FAILED"},
		{"missing scenario pass", "COMPATIBILITY_REAL_PATH_MISSING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evidence := ChainMeasurementEvidence{Topology: topology, Environment: environment}
			hostDigests := []string{}
			for host := 0; host < 4; host++ {
				dir := t.TempDir()
				manifest := map[string]string{}
				for _, name := range []string{"http-samples.ndjson", "resource-pool-telemetry.ndjson", "chain-receipts.ndjson", "receiver-events.ndjson"} {
					wire := []byte(fmt.Sprintf("{\"host_index\":%d}\n", host))
					hash := sha256.Sum256(wire)
					manifest[name] = hex.EncodeToString(hash[:])
					if err := os.WriteFile(filepath.Join(dir, name), wire, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if tc.name == "missing capture" {
					delete(manifest, "http-samples.ndjson")
				}
				wire := encode(manifest)
				if tc.name == "malformed manifest" {
					wire = []byte("{")
				}
				if tc.name == "trailing manifest" {
					wire = append(wire, []byte(" {}")...)
				}
				path := filepath.Join(dir, "manifest.json")
				if err := os.WriteFile(path, wire, 0600); err != nil {
					t.Fatal(err)
				}
				hash := sha256.Sum256(wire)
				artifact := readpolicy.BoundMeasurementArtifact{Path: path, SHA256: hex.EncodeToString(hash[:]), MaxBytes: int64(len(wire))}
				hostDigests = append(hostDigests, artifact.SHA256)
				if tc.name == "missing manifest" && host == 0 {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				if tc.name == "changed capture" && host == 0 {
					if err := os.WriteFile(filepath.Join(dir, "http-samples.ndjson"), []byte("{}\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				report := map[string]any{"host_id": fmt.Sprintf("generator-%d", host), "host_index": host, "bindings": bindings, "started_utc_ns": started.Add(-time.Minute).UnixNano(), "ended_utc_ns": ended.Add(time.Minute).UnixNano(), "duration_ns": int64(600 * time.Second), "accepted_connections": 25000, "artifact_manifest_sha256": artifact.SHA256, "http": map[string]any{"groups": map[string]any{"all": map[string]int{"scheduled": 1500000}}}}
				reportWire := encode(report)
				if tc.name == "trailing report" {
					reportWire = append(reportWire, []byte(" {}")...)
				}
				evidence.Hosts = append(evidence.Hosts, ChainMeasurementHostArtifact{Report: boundMeasurementFile(t, "host.json", reportWire), Manifest: artifact, MaxRawBytes: 1 << 20})
			}
			// Malformed manifests must still have distinct source identities.
			if tc.name == "malformed manifest" || tc.name == "trailing manifest" {
				for i := 1; i < 4; i++ {
					hostDigests[i] = fmt.Sprintf("%064x", i)
				}
			}
			evidence.Workload = boundMeasurementFile(t, "workload.json", encode(map[string]any{"bindings": bindings, "started_utc_ns": started.UnixNano(), "ended_utc_ns": ended.UnixNano(), "accepted_connections": 100000, "capacity_qualified": false, "phase3_scenario_qualified": true, "host_artifact_digests": hostDigests}))
			scenarios := map[string]any{}
			for i := 1; i <= 10; i++ {
				scenarios[fmt.Sprintf("scenario_%d", i)] = map[string]any{"exitCode": 0, "command": []string{"go", "test", "-json"}, "rawArtifact": readpolicy.BoundMeasurementArtifact{}}
			}
			if tc.name == "failed scenario" {
				scenarios["scenario_1"] = map[string]any{"exitCode": 1, "command": []string{"go", "test", "-json"}}
			}
			rawWire := []byte("not-json\n")
			if tc.name == "skipped scenario" {
				rawWire = []byte(`{"Action":"skip","Test":"TestWrongChainCannotServe","Package":"github.com/Sidiora-Technologies/KindleLaunch/shared/chain"}` + "\n")
			}
			if tc.name == "missing scenario pass" {
				rawWire = []byte(`{"Action":"run","Test":"TestWrongChainCannotServe","Package":"github.com/Sidiora-Technologies/KindleLaunch/shared/chain"}` + "\n")
			}
			if tc.name == "malformed scenario raw" || tc.name == "skipped scenario" || tc.name == "missing scenario pass" {
				scenarios["scenario_1"] = map[string]any{"exitCode": 0, "command": []string{"go", "test", "-json"}, "rawArtifact": boundMeasurementFile(t, "scenario.ndjson", rawWire)}
			}
			compat := map[string]any{"binding": binding, "scenarios": scenarios}
			if tc.name == "foreign compatibility" {
				compat["binding"] = readpolicy.MeasurementBinding{}
			}
			wire := encode(compat)
			if tc.name == "malformed compatibility" {
				wire = []byte("{")
			}
			if tc.name == "trailing compatibility" {
				wire = append(wire, []byte(" {}")...)
			}
			if tc.name != "missing compatibility" {
				evidence.Compatibility = boundMeasurementFile(t, "compatibility.json", wire)
			}
			capture, err := readpolicy.NewMeasurementCapture(filepath.Join(t.TempDir(), "capture"), binding, 1000, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UnixNano()
			if err := capture.Append(readpolicy.MeasurementSample{RequestID: "artifact-validation", Scenario: "head", ChainID: "125", ObservedUTCNS: now, FinishedUTCNS: now + 1, ElapsedNS: 1, Outcome: "success"}); err != nil {
				t.Fatal(err)
			}
			run := &ChainMeasurementRun{capture: capture, Binding: binding, Started: started, Ended: ended, IngestLatencies: []int64{1}}
			result, err := FinishChainMeasurement(run, evidence)
			if err == nil || result.LaneHarnessPassed || result.FailureCode != tc.code || result.RawRows != 1 {
				t.Fatalf("artifact validation = %+v, %v", result, err)
			}
		})
	}
}
