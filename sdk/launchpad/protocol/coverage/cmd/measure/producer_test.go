package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/coverage"
	"github.com/ethereum/go-ethereum/common"
)

func TestMeasurementCommandCapturesReviewedProducerBeforeExclusiveOutput(t *testing.T) {
	root, err := filepath.Abs("../../../../../../contracts/launchpad/contracts")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("../../testdata/repair28producer/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	modfile := filepath.Join(t.TempDir(), "producer.mod")
	if err := os.WriteFile(modfile, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, "go", "run", "-modfile="+modfile, "-mod=mod", ".", root)
	command.Dir = "../../testdata/repair28producer"
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(t.TempDir(), "producer.log"))
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	stopProducer := func() {
		once.Do(func() {
			stdin.Close()
			if err := command.Wait(); err != nil {
				stderr.Close()
				raw, _ := os.ReadFile(stderr.Name())
				t.Errorf("real producer: %v\n%s", err, raw)
			} else {
				stderr.Close()
			}
		})
	}
	t.Cleanup(stopProducer)
	var producer struct {
		Endpoint  string
		Addresses map[string]common.Address
		Direct    common.Hash
	}
	if err := json.NewDecoder(stdout).Decode(&producer); err != nil {
		t.Fatal(err)
	}
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	var deployments []catalog.Deployment
	var enabled []string
	for _, bundle := range bundles {
		if address, ok := producer.Addresses[bundle.Name]; ok {
			deployments = append(deployments, catalog.Deployment{ChainID: 125, Address: address, Family: bundle.Name, Version: bundle.ABIHash, Attested: true})
			enabled = append(enabled, bundle.Name)
		}
		if bundle.Name == "SidioraERC20" {
			deployments = append(deployments, catalog.Deployment{ChainID: 125, Address: producer.Addresses["USDL"], Family: bundle.Name, Version: bundle.ABIHash, Attested: true})
		}
	}
	sources, err := coverage.CompiledMeasurementSourceSHA256()
	if err != nil {
		t.Fatal(err)
	}
	encodedSources, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(encodedSources)
	zeroDigest := strings.Repeat("0", 64)
	input := coverage.MeasurementManifest{
		SchemaVersion: 1, ChainID: 125, SourceManifestSHA256: hex.EncodeToString(sourceHash[:]),
		InterfaceSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest,
		Deployments: deployments, EnabledFamilies: enabled,
		Receipts: []coverage.ReceiptRequest{{TransactionHash: producer.Direct, Scenario: 1, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest}},
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	reviewed := hex.EncodeToString(digest[:])
	inputPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(inputPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	oldArgs, oldFlags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = oldArgs, oldFlags })
	execute := func(historyComplete func()) (bool, error) {
		t.Helper()
		os.Args = []string{"measure", "-manifest", inputPath, "-manifest-sha256", reviewed, "-output-dir", output}
		flag.CommandLine = flag.NewFlagSet("measure", flag.ContinueOnError)
		return runMeasurement(historyComplete)
	}
	if passed, err := execute(nil); err == nil || passed || !strings.Contains(err.Error(), rpcURLVariable) {
		t.Fatalf("missing historical RPC accepted: %v", err)
	}
	t.Setenv(rpcURLVariable, producer.Endpoint)
	evidence := filepath.Join(output, "raw-history.json")
	if err := os.WriteFile(evidence, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	if passed, err := execute(nil); err == nil || passed || !strings.Contains(err.Error(), "output unavailable") {
		t.Fatalf("existing output overwritten: %v", err)
	}
	actual, err := os.ReadFile(evidence)
	if err != nil || string(actual) != "retained" {
		t.Fatalf("existing evidence changed: %q %v", actual, err)
	}
	fullInputPath, fullReviewed, fullOutput := inputPath, reviewed, t.TempDir()
	for _, deployment := range deployments {
		if deployment.Family == "PoolRegistry" {
			input.Deployments = []catalog.Deployment{deployment}
			input.EnabledFamilies = []string{deployment.Family}
			break
		}
	}
	if len(input.Deployments) != 1 {
		t.Fatal("reviewed pool registry deployment absent")
	}
	raw, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(raw)
	reviewed = hex.EncodeToString(digest[:])
	inputPath = filepath.Join(t.TempDir(), "registry-manifest.json")
	if err := os.WriteFile(inputPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output = t.TempDir()
	if passed, err := execute(nil); err == nil || passed || !strings.Contains(err.Error(), "no bounded decoded historical logs") {
		t.Fatalf("receipt without reviewed logs produced a decoder rate: %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(output, "raw-history.json")); err != nil || len(raw) == 0 {
		t.Fatalf("committed raw history absent: %v", err)
	}
	inputPath, reviewed, output = fullInputPath, fullReviewed, fullOutput
	if passed, err := execute(stopProducer); err != nil || passed {
		t.Fatalf("incomplete reviewed receipt corpus produced a release result: passed=%t err=%v", passed, err)
	}
	reportBytes, err := os.ReadFile(filepath.Join(output, "measurement.json"))
	if err != nil {
		t.Fatal(err)
	}
	var measurement report
	if err := json.Unmarshal(reportBytes, &measurement); err != nil {
		t.Fatal(err)
	}
	if measurement.Passed || measurement.Replay == nil || measurement.Replay.ReceiptCount != 1 || measurement.Burst == nil || measurement.Burst.Attempted != coverage.MeasurementRate*coverage.MeasurementSeconds || measurement.Burst.Matched != measurement.Burst.Attempted || len(measurement.Replay.Blockers) == 0 {
		t.Fatalf("incomplete historical measurement report differs: %+v", measurement)
	}
}
