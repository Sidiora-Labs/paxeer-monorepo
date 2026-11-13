package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/coverage"
	"github.com/ethereum/go-ethereum/common"
)

func TestMeasurementCommandReadsReviewedInputAndWritesExclusiveEvidence(t *testing.T) {
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	first := bundles[0]
	zeroDigest := strings.Repeat("0", 64)
	input := coverage.MeasurementManifest{
		SchemaVersion: 1, ChainID: 125, SourceManifestSHA256: zeroDigest,
		InterfaceSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest,
		Deployments:     []catalog.Deployment{{ChainID: 125, Address: common.HexToAddress("0x1234"), Family: first.Name, Version: first.ABIHash, Attested: true}},
		EnabledFamilies: []string{first.Name},
		Receipts:        []coverage.ReceiptRequest{{TransactionHash: common.HexToHash("0x4567"), Scenario: 1, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest}},
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	reviewed := hex.EncodeToString(digest[:])
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readManifest(path, reviewed)
	if err != nil || got.ChainID != 125 || len(got.Receipts) != 1 {
		t.Fatalf("reviewed manifest: %+v, %v", got, err)
	}
	for _, invalid := range []struct{ path, digest string }{{"", reviewed}, {path + ".missing", reviewed}, {path, zeroDigest}} {
		if _, err := readManifest(invalid.path, invalid.digest); err == nil {
			t.Fatal("unreviewed manifest accepted")
		}
	}

	sources, err := coverage.CompiledMeasurementSourceSHA256()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest := sha256.Sum256(encoded)
	gotSources, err := compiledSources(hex.EncodeToString(sourceDigest[:]))
	if err != nil || gotSources["protocol/coverage/measure.go"] == "" {
		t.Fatalf("compiled source identity: %v", err)
	}
	if _, err := compiledSources(zeroDigest); err == nil {
		t.Fatal("different source identity accepted")
	}

	evidence := filepath.Join(t.TempDir(), "nested", "measurement.json")
	written, err := writeExclusive(evidence, got)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(evidence)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(actual)
	if written != hex.EncodeToString(hash[:]) {
		t.Fatal("evidence digest differs from written bytes")
	}
	if _, err := writeExclusive(evidence, got); err == nil {
		t.Fatal("existing evidence overwritten")
	}
	if _, err := readManifest(t.TempDir(), reviewed); err == nil {
		t.Fatal("directory admitted as a measurement manifest")
	}
	if _, err := writeExclusive(filepath.Join(evidence, "nested.json"), got); err == nil {
		t.Fatal("evidence file admitted as an output parent")
	}
	input.SourceManifestSHA256 = zeroDigest
	raw, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(raw)
	unreviewedSource := filepath.Join(t.TempDir(), "source-manifest.json")
	if err := os.WriteFile(unreviewedSource, raw, 0600); err != nil {
		t.Fatal(err)
	}
	oldArgs, oldFlags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = oldArgs, oldFlags })
	os.Args = []string{"measure", "-manifest", unreviewedSource, "-manifest-sha256", hex.EncodeToString(digest[:]), "-output-dir", t.TempDir()}
	flag.CommandLine = flag.NewFlagSet("measure", flag.ContinueOnError)
	if passed, err := run(); err == nil || passed || !strings.Contains(err.Error(), "compiled measurement sources differ") {
		t.Fatalf("different compiled source manifest admitted: %v", err)
	}
}

func TestMeasurementCommandRequiresReviewedArguments(t *testing.T) {
	oldArgs, oldFlags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = oldArgs, oldFlags })
	for _, args := range [][]string{
		{"measure"},
		{"measure", "-output-dir", t.TempDir()},
		{"measure", "-manifest", "absent", "-manifest-sha256", strings.Repeat("0", 64), "-output-dir", t.TempDir()},
	} {
		os.Args = args
		flag.CommandLine = flag.NewFlagSet("measure", flag.ContinueOnError)
		passed, err := run()
		if err == nil || passed {
			t.Fatalf("unreviewed command accepted %v", args)
		}
	}
}
