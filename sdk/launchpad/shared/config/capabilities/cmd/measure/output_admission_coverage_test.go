package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestMeasurementCommandPersistsRefusedPinnedBlockEvidence(t *testing.T) {
	ctx := context.Background()
	endpoint := os.Getenv("L34_ANVIL_URL")
	if endpoint == "" {
		t.Fatal("chain fixture required")
	}
	client, err := rpc.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reader, err := capabilities.NewPinnedRPCReader(client)
	if err != nil {
		t.Fatal(err)
	}
	head, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, genesis, err := reader.ReportedHeaderByNumber(ctx, head.Number)
	if err != nil {
		t.Fatal(err)
	}
	_, actualGenesis, err := reader.ReportedHeaderByNumber(ctx, common.Big0)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	buildDigest := repairM951Digest(binary)
	manifestPath := filepath.Join(repairM951Root(t), "sdk/launchpad/shared/config/deployment/current.json")
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := deployment.Parse(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	poolRuntime := "0x" + strings.Repeat("a", 64)
	serial := strings.Repeat("b", 64)
	engine, err := capabilities.NewPreflightEngine(reader, manifest, poolRuntime, serial, time.Minute, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	root := repairM951Candidate(t)
	_, indexPath, sourceDigest := repairM951Index(t, root)
	build := capabilities.ConsumerBuildIdentity{BuildSHA256: buildDigest, CandidateSHA256: sourceDigest, ManifestSHA256: engine.Revision().ManifestSHA256, SourceManifestSHA256: manifest.SourceManifestSHA256, RevisionSHA256: engine.Revision().RevisionSHA256, NetworkSHA256: strings.Repeat("c", 64), Addresses: manifest.Addresses()}
	plan := capabilities.MeasurementPlan{SchemaVersion: 1, CandidateSHA256: sourceDigest, HarnessBuildSHA256: buildDigest, GoBuild: build, FrontendBuild: build, SamplesPerCase: 1, Concurrency: 1, DeadlineMilliseconds: 10000, P95BudgetMicroseconds: 10000000, MaxErrorBasisPoints: 0, MaxRawBytes: 1 << 20}
	// The reported hash belongs to a different height from the approved probe.
	for _, kind := range []string{"exports", "pending-governance", "unknown-implementation", "no-oracle", "quotes", "creation-fees", "zero-real-sell", "private-export", "quoter-revoked"} {
		request := capabilities.PreflightRequest{Family: capabilities.PlainQuote, BlockHash: genesis, ExpectedRevision: engine.Revision().RevisionSHA256, Pool: common.HexToAddress("0x01"), Trader: common.HexToAddress("0x02"), Recipient: common.HexToAddress("0x03"), AmountIn: "1", IsBuy: true}
		probe := capabilities.MeasurementCase{Kind: kind, BlockHash: genesis, BlockNumber: manifest.ActivationHeight, Request: request, OpticalRequest: request, ReceiptHashes: []common.Hash{{1}, {2}}}
		probe.OpticalRequest.Pool = common.HexToAddress("0x04")
		plan.Cases = append(plan.Cases, probe)
	}
	approved := authority{SchemaVersion: 1, Realm: "private_fixture", EndpointID: "chain", URLVariable: "MEASUREMENT_BOUND_CHAIN", URLSHA256: repairM951Digest([]byte(endpoint)), GenesisHash: actualGenesis, CommittedBlocks: map[string]uint64{genesis.Hex(): manifest.ActivationHeight}, ManifestFileSHA256: repairM951Digest(manifestRaw), PoolRuntimeHash: poolRuntime, SerialInterfaceSHA256: serial}
	t.Setenv(approved.URLVariable, endpoint)
	bounds := budget{SamplesPerCase: 1, Concurrency: 1, DeadlineMilliseconds: 10000, P95BudgetMicroseconds: 10000000, MaxErrorBasisPoints: 0, MaxRawBytes: 1 << 20, ReadDeadlineMilliseconds: 1000, AttestationMaxAgeSeconds: 60}
	previousFlags, previousArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = previousFlags, previousArgs })
	for _, mode := range []string{"genesis", "engine", "plan", "paths", "parent", "raw exists", "report exists", "capture"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			candidate, authorityValue := plan, approved
			write := func(name string, value any) (string, string) {
				t.Helper()
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(directory, name)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				return path, repairM951Digest(raw)
			}
			policyPath, policyDigest := write("policy.json", bounds)
			candidate.PolicySHA256 = policyDigest
			exportPath, exportDigest := write("consumer.json", build)
			authorityValue.GoBuildExportSHA256 = exportDigest
			authorityValue.FrontendBuildExportSHA256 = exportDigest
			rawPath, reportPath := filepath.Join(directory, "raw.jsonl"), filepath.Join(directory, "report.json")
			want := "measurement_gate_failed"
			switch mode {
			case "genesis":
				authorityValue.GenesisHash = common.HexToHash("0x01")
				want = "approved_endpoint_genesis_mismatch"
			case "engine":
				authorityValue.PoolRuntimeHash = "invalid"
				want = "preflight_configuration_invalid"
			case "plan":
				candidate.Cases = append([]capabilities.MeasurementCase(nil), plan.Cases...)
				candidate.Cases[0].Kind = "unregistered"
				want = "measurement_case_invalid"
			case "paths":
				reportPath = rawPath
				want = "measurement_output_paths_invalid"
			case "parent":
				rawPath = filepath.Join(exportPath, "raw.jsonl")
				want = "measurement_output_parent_unavailable"
			case "raw exists":
				if err := os.WriteFile(rawPath, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "measurement_raw_output_unavailable"
			case "report exists":
				if err := os.WriteFile(reportPath, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "measurement_report_output_unavailable"
			}
			authorityPath, authorityDigest := write("authority.json", authorityValue)
			candidate.AuthoritySHA256 = authorityDigest
			planPath, planDigest := write("plan.json", candidate)
			os.Args = []string{"measure", "-plan", planPath, "-plan-sha256", planDigest, "-authority", authorityPath, "-source-root", root, "-source-index", indexPath, "-policy", policyPath, "-manifest", manifestPath, "-go-export", exportPath, "-frontend-export", exportPath, "-go-build", executable, "-frontend-build", executable, "-raw-output", rawPath, "-report-output", reportPath}
			flag.CommandLine = flag.NewFlagSet("measure", flag.ContinueOnError)
			repairM951Issue(t, run(), want)
			if mode == "capture" {
				raw, err := os.ReadFile(rawPath)
				if err != nil || len(raw) == 0 {
					t.Fatalf("pinned read capture: %d %v", len(raw), err)
				}
				data, err := os.ReadFile(reportPath)
				if err != nil {
					t.Fatal(err)
				}
				var report capabilities.MeasurementReport
				if err := json.Unmarshal(data, &report); err != nil || report.MeasurementPassed || report.ReleaseReady || report.FailedSamples != 9 || report.CompletedSamples != 9 || report.PlanFileSHA256 != planDigest || report.RawSHA256 != repairM951Digest(raw) {
					t.Fatalf("refused probe report: %+v %v", report, err)
				}
				for _, sample := range report.Samples {
					if sample.Passed || sample.Issue == nil || sample.Issue.Code != "measurement_canonical_pin_unverified" {
						t.Fatalf("unverified canonical probe: %+v", sample)
					}
				}
			}
			if mode == "raw exists" || mode == "report exists" {
				path := rawPath
				if mode == "report exists" {
					path = reportPath
				}
				if raw, err := os.ReadFile(path); err != nil || string(raw) != "retained" {
					t.Fatalf("existing evidence replaced: %s %v", raw, err)
				}
			}
		})
	}
}
