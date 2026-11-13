package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
)

func boundMeasurementFile(t *testing.T, name string, content []byte) readpolicy.BoundMeasurementArtifact {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return readpolicy.BoundMeasurementArtifact{Path: path, SHA256: hex.EncodeToString(digest[:]), MaxBytes: int64(len(content))}
}

func TestMeasurementEvidenceRequiresBoundSources(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	build, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	buildHash := sha256.Sum256(build)
	buildDigest := hex.EncodeToString(buildHash[:])
	topology := boundMeasurementFile(t, "topology.json", []byte(`{"hosts":4}`))
	environment := boundMeasurementFile(t, "environment.json", []byte(`{"chainId":125}`))
	binding := readpolicy.MeasurementBinding{CandidateSHA256: buildDigest, InterfaceSHA256: buildDigest, ManifestSHA256: buildDigest, TopologySHA256: topology.SHA256, EnvironmentSHA256: environment.SHA256, BuildSHA256: buildDigest, WorkloadSeed: "bound-real-io", ProfileVersion: "e1"}
	hostDigests := []string{strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64), strings.Repeat("4", 64)}
	workload := map[string]any{
		"bindings":       map[string]string{"candidate": binding.CandidateSHA256, "interface": binding.InterfaceSHA256, "topology": binding.TopologySHA256, "generator": buildDigest},
		"started_utc_ns": time.Now().Add(-time.Minute).UnixNano(), "ended_utc_ns": time.Now().UnixNano(),
		"accepted_connections": 100000, "capacity_qualified": false, "phase3_scenario_qualified": true, "host_artifact_digests": hostDigests,
	}
	workloadWire, err := json.Marshal(workload)
	if err != nil {
		t.Fatal(err)
	}
	validWorkload := boundMeasurementFile(t, "workload.json", workloadWire)
	alteredWorkload := func(edit func(map[string]any)) readpolicy.BoundMeasurementArtifact {
		t.Helper()
		candidate := make(map[string]any, len(workload))
		for key, value := range workload {
			candidate[key] = value
		}
		edit(candidate)
		encoded, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		return boundMeasurementFile(t, "workload.json", encoded)
	}
	withSources := func(e *ChainMeasurementEvidence, artifact readpolicy.BoundMeasurementArtifact) {
		e.Topology, e.Environment, e.Workload = topology, environment, artifact
	}
	for _, tc := range []struct {
		name string
		edit func(*ChainMeasurementEvidence)
		code string
	}{
		{"missing binding", func(*ChainMeasurementEvidence) {}, "MISSING_ENVIRONMENT_OR_TOPOLOGY_BINDING"},
		{"missing topology file", func(e *ChainMeasurementEvidence) { e.Topology.SHA256 = topology.SHA256; e.Environment = environment }, "ENVIRONMENT_OR_TOPOLOGY_ARTIFACT_MISSING"},
		{"missing workload", func(e *ChainMeasurementEvidence) { e.Topology = topology; e.Environment = environment }, "MISSING_WORKLOAD_EVIDENCE"},
		{"malformed workload", func(e *ChainMeasurementEvidence) {
			e.Topology = topology
			e.Environment = environment
			e.Workload = boundMeasurementFile(t, "workload.json", []byte("{"))
		}, "MALFORMED_WORKLOAD_EVIDENCE"},
		{"workload scope", func(e *ChainMeasurementEvidence) {
			e.Topology = topology
			e.Environment = environment
			e.Workload = boundMeasurementFile(t, "workload.json", []byte(`{"accepted_connections":1}`))
		}, "WORKLOAD_BINDING_OR_COVERAGE_MISMATCH"},
		{"trailing workload document", func(e *ChainMeasurementEvidence) {
			withSources(e, boundMeasurementFile(t, "workload.json", append(append([]byte(nil), workloadWire...), []byte("{}")...)))
		}, "MALFORMED_WORKLOAD_EVIDENCE"},
		{"repeated host digest", func(e *ChainMeasurementEvidence) {
			withSources(e, alteredWorkload(func(w map[string]any) {
				w["host_artifact_digests"] = []string{hostDigests[0], hostDigests[0], hostDigests[2], hostDigests[3]}
			}))
		}, "WORKLOAD_HOST_IDENTITY_MISMATCH"},
		{"malformed host digest", func(e *ChainMeasurementEvidence) {
			withSources(e, alteredWorkload(func(w map[string]any) {
				w["host_artifact_digests"] = []string{strings.Repeat("z", 64), hostDigests[1], hostDigests[2], hostDigests[3]}
			}))
		}, "WORKLOAD_HOST_IDENTITY_MISMATCH"},
		{"missing hosts", func(e *ChainMeasurementEvidence) {
			e.Topology = topology
			e.Environment = environment
			e.Workload = validWorkload
		}, "MISSING_WORKLOAD_HOST_ARTIFACTS"},
		{"host byte budget", func(e *ChainMeasurementEvidence) {
			e.Topology = topology
			e.Environment = environment
			e.Workload = validWorkload
			e.Hosts = make([]ChainMeasurementHostArtifact, 4)
		}, "WORKLOAD_RAW_BUDGET_REQUIRED"},
		{"missing host report", func(e *ChainMeasurementEvidence) {
			e.Topology = topology
			e.Environment = environment
			e.Workload = validWorkload
			e.Hosts = make([]ChainMeasurementHostArtifact, 4)
			e.Hosts[0].MaxRawBytes = 1 << 20
		}, "MISSING_WORKLOAD_HOST_REPORT"},
		{"malformed host report", func(e *ChainMeasurementEvidence) {
			withSources(e, validWorkload)
			e.Hosts = make([]ChainMeasurementHostArtifact, 4)
			e.Hosts[0] = ChainMeasurementHostArtifact{Report: boundMeasurementFile(t, "host.json", []byte("{")), MaxRawBytes: 1 << 20}
		}, "MALFORMED_WORKLOAD_HOST_REPORT"},
		{"unbound host report", func(e *ChainMeasurementEvidence) {
			withSources(e, validWorkload)
			e.Hosts = make([]ChainMeasurementHostArtifact, 4)
			e.Hosts[0] = ChainMeasurementHostArtifact{Report: boundMeasurementFile(t, "host.json", []byte(`{"host_id":"unbound"}`)), MaxRawBytes: 1 << 20}
		}, "WORKLOAD_HOST_COVERAGE_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture, err := readpolicy.NewMeasurementCapture(filepath.Join(t.TempDir(), "capture"), binding, 1000, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UnixNano()
			if err := capture.Append(readpolicy.MeasurementSample{RequestID: "bound-sample", Scenario: "head", ChainID: "125", ObservedUTCNS: now, FinishedUTCNS: now + 1, ElapsedNS: 1, Outcome: "success"}); err != nil {
				t.Fatal(err)
			}
			run := &ChainMeasurementRun{capture: capture, Binding: binding, Started: time.Now().Add(-time.Minute), Ended: time.Now(), IngestLatencies: []int64{1}}
			evidence := ChainMeasurementEvidence{}
			tc.edit(&evidence)
			result, err := FinishChainMeasurement(run, evidence)
			if err == nil || result.LaneHarnessPassed || result.FailureCode != tc.code || result.RawRows != 1 {
				t.Fatalf("bound evidence result = %+v, %v", result, err)
			}
		})
	}
}
