package coverage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReviewedMeasurementAdmitsOnlyCanonicalSelectorsAndReturns(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	address := addresses["SidioraERC20"]
	deployment, ok := registry.DeploymentAt(125, address, 12)
	if !ok {
		t.Fatal("reviewed token deployment absent")
	}
	input := MeasurementManifest{Deployments: []catalog.Deployment{deployment}, EnabledFamilies: []string{deployment.Family}}
	rows, parsed, err := measurementSurface(bundles, input)
	if err != nil {
		t.Fatal(err)
	}
	call, err := abis["SidioraERC20"].Pack("totalSupply")
	if err != nil {
		t.Fatal(err)
	}
	output, err := abis["SidioraERC20"].Methods["totalSupply"].Outputs.Pack(big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	if err := observeCall(rows, parsed, registry, address, 12, call, output, true); err != nil {
		t.Fatal(err)
	}
	observed := false
	for _, row := range rows {
		if row.Kind == "function" && row.Signature == "totalSupply()" && row.Observations == 1 {
			observed = true
		}
	}
	if !observed {
		t.Fatal("canonical selector not observed")
	}
	for _, item := range []struct {
		address common.Address
		call    []byte
		output  []byte
	}{
		{common.HexToAddress("0xdead"), call, output},
		{address, call[:3], output},
		{address, []byte{0xff, 0xff, 0xff, 0xff}, output},
		{address, append(bytes.Clone(call), make([]byte, 32)...), output},
		{address, call, output[:31]},
		{address, call, append(bytes.Clone(output), make([]byte, 32)...)},
	} {
		if err := observeCall(rows, parsed, registry, item.address, 12, item.call, item.output, true); err == nil {
			t.Fatal("noncanonical reviewed call admitted")
		}
	}
}

func TestReviewedMeasurementRejectsDuplicateAndOmittedFamilies(t *testing.T) {
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	first := bundles[0]
	zeroDigest := strings.Repeat("0", 64)
	manifest := MeasurementManifest{
		SchemaVersion: 1, ChainID: 125,
		SourceManifestSHA256: zeroDigest, InterfaceSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest,
		Deployments:     []catalog.Deployment{{ChainID: 125, Address: common.HexToAddress("0x1234"), Family: first.Name, Version: first.ABIHash, Attested: true}},
		EnabledFamilies: []string{first.Name},
		Receipts:        []ReceiptRequest{{TransactionHash: common.HexToHash("0x5678"), Scenario: 1, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest}},
	}
	read := func(value MeasurementManifest) (MeasurementManifest, error) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return ReadMeasurementManifest(raw, measurementDigest(raw))
	}
	input, err := read(manifest)
	if err != nil {
		t.Fatal(err)
	}
	rawTrailing, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	rawTrailing = append(rawTrailing, []byte(` {}`)...)
	if _, err := ReadMeasurementManifest(rawTrailing, measurementDigest(rawTrailing)); err == nil {
		t.Fatal("trailing reviewed manifest value admitted")
	}
	if _, _, err := MeasurementRegistry(input); err != nil {
		t.Fatal(err)
	}
	unattested := manifest
	unattested.Deployments = append([]catalog.Deployment(nil), manifest.Deployments...)
	unattested.Deployments[0].Attested = false
	selected, err := read(unattested)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := MeasurementRegistry(selected); err == nil {
		t.Fatal("unattested measurement deployment entered registry")
	}
	for _, families := range [][]string{{first.Name, first.Name}, {"UnknownFamily"}, {bundles[1].Name}} {
		changed := manifest
		changed.EnabledFamilies = families
		selected, err := read(changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := MeasurementRegistry(selected); err == nil {
			t.Fatalf("inconsistent enabled families admitted: %v", families)
		}
	}
	omitted := manifest
	omitted.Deployments = append([]catalog.Deployment(nil), manifest.Deployments...)
	omitted.Deployments = append(omitted.Deployments, catalog.Deployment{ChainID: 125, Address: common.HexToAddress("0x1235"), Family: bundles[1].Name, Version: bundles[1].ABIHash, Attested: true})
	selected, err = read(omitted)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := MeasurementRegistry(selected); err == nil {
		t.Fatal("attested deployment outside the enabled family set admitted")
	}
	invalid := manifest
	invalid.Getters = []GetterRequest{{BlockHash: common.HexToHash("0x12"), Address: common.HexToAddress("0x1234"), Input: []byte{1, 2, 3}, ExpectedOutputSHA256: zeroDigest}}
	if _, err := read(invalid); err == nil {
		t.Fatal("short reviewed getter selector admitted")
	}
	invalid = manifest
	invalid.Receipts = []ReceiptRequest{{TransactionHash: common.HexToHash("0x5678"), Scenario: 13, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest}}
	if _, err := read(invalid); err == nil {
		t.Fatal("out of range reviewed scenario admitted")
	}
}

func TestReviewedArchivedEmitterIsAvailableToMeasurement(t *testing.T) {
	rawABI, err := os.ReadFile("../bindings/abi/eventemitter_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(rawABI, &value); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	abiHash := sha256.Sum256(canonical)
	zeroDigest := strings.Repeat("0", 64)
	bundle := catalog.Bundle{
		Name: "EventEmitter", SourceName: "contracts/core/EventEmitter.sol",
		ArtifactPath: "artifacts/contracts/core/EventEmitter.sol/EventEmitter.json", Compiler: "reviewed",
		ArtifactHash: zeroDigest, ABIHash: hex.EncodeToString(abiHash[:]), SourceHash: zeroDigest, BuildHash: zeroDigest, ABI: canonical,
	}
	revision, err := json.Marshal(catalog.ReviewedRevision{SchemaVersion: 1, Bundles: []catalog.Bundle{bundle}})
	if err != nil {
		t.Fatal(err)
	}
	revisionHash := measurementDigest(revision)
	address := common.HexToAddress("0x1234")
	manifest := MeasurementManifest{
		SchemaVersion: 1, ChainID: 125, SourceManifestSHA256: zeroDigest, InterfaceSHA256: zeroDigest,
		ExpectedProjectionSHA256: zeroDigest, ReviewedRevisions: revision, ReviewedRevisionsSHA256: revisionHash,
		Deployments:     []catalog.Deployment{{ChainID: 125, Address: address, Family: bundle.Name, Version: bundle.ABIHash, Attested: true}},
		EnabledFamilies: []string{bundle.Name},
		Receipts:        []ReceiptRequest{{TransactionHash: common.HexToHash("0x5678"), Scenario: 1, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest}},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := measurementDigest(raw)
	input, err := ReadMeasurementManifest(raw, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	registry, bundles, err := MeasurementRegistry(input)
	if err != nil || !registry.RevisionKnown(bundle.Name, bundle.ABIHash) || len(bundles) == 0 {
		t.Fatalf("reviewed archived emitter unavailable: %v", err)
	}
	if got, ok := registry.DeploymentAt(125, address, 12); !ok || got.Version != bundle.ABIHash {
		t.Fatal("archived emitter deployment absent")
	}
	capture := &HistoricalCapture{SchemaVersion: 1, ChainID: 125, ManifestSHA256: reviewed, Receipts: []CapturedReceipt{{SignedTransaction: []byte{1}}}}
	capture.seal = captureSeal(capture)
	if _, err := ReplayHistorical(context.Background(), nil, input, capture); err == nil {
		t.Fatal("invalid captured signed transaction admitted")
	}
	manifest.ReviewedRevisionsSHA256 = zeroDigest
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := ReadMeasurementManifest(raw, measurementDigest(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := MeasurementRegistry(changed); err == nil {
		t.Fatal("unreviewed archive identity admitted")
	}
}

func TestDecoderBurstRequiresIntactDecodedReceiptLogs(t *testing.T) {
	registry, _, _ := sourceRegistry(t)
	for _, receipt := range []*types.Receipt{nil, {Logs: []*types.Log{nil}}, {Logs: []*types.Log{}}} {
		capture := &HistoricalCapture{SchemaVersion: 1, ChainID: 125, Receipts: []CapturedReceipt{{Receipt: receipt}}}
		capture.seal = captureSeal(capture)
		if _, err := MeasureDecoderBurst(context.Background(), registry, capture); err == nil {
			t.Fatal("undecodable historical corpus admitted")
		}
	}
	unknown := &HistoricalCapture{SchemaVersion: 1, ChainID: 125, Receipts: []CapturedReceipt{{Receipt: &types.Receipt{Logs: []*types.Log{{Address: common.HexToAddress("0xdead")}}}}}}
	unknown.seal = captureSeal(unknown)
	if _, err := MeasureDecoderBurst(context.Background(), registry, unknown); err == nil {
		t.Fatal("unrecognized historical log supplied a decoder measurement")
	}
}
