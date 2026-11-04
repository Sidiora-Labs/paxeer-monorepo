package catalog

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestArchivedRevisionRequiresExactReviewedArtifactAndProvenance(t *testing.T) {
	bundles, err := Bundles()
	if err != nil {
		t.Fatal(err)
	}
	var current Bundle
	for _, b := range bundles {
		if b.Name == "EventEmitter" {
			current = b
		}
	}
	raw, err := json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{current}})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	expected := hex.EncodeToString(sum[:])
	if _, err = NewWithReviewedRevisions(nil, raw, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unreviewed historical bytes admitted")
	}
	if _, err = NewWithReviewedRevisions(nil, raw, expected); err == nil || !strings.Contains(err.Error(), "duplicate ABI revision") {
		t.Fatal("existing revision overwritten", err)
	}
	changed := current
	changed.Name = "SidioraERC20"
	bad, _ := json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{changed}})
	h := sha256.Sum256(bad)
	if _, err = NewWithReviewedRevisions(nil, bad, hex.EncodeToString(h[:])); err == nil {
		t.Fatal("unallocated archived family admitted")
	}
	changed = current
	changed.SourceName = "../outside.sol"
	bad, _ = json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{changed}})
	h = sha256.Sum256(bad)
	if _, err = NewWithReviewedRevisions(nil, bad, hex.EncodeToString(h[:])); err == nil {
		t.Fatal("archived source traversal admitted")
	}
	changed = current
	changed.BuildHash = ""
	bad, _ = json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{changed}})
	h = sha256.Sum256(bad)
	if _, err = NewWithReviewedRevisions(nil, bad, hex.EncodeToString(h[:])); err == nil {
		t.Fatal("missing archived compiler provenance admitted")
	}
	trailing := append(append([]byte(nil), raw...), []byte(` {"schemaVersion":1}`)...)
	h = sha256.Sum256(trailing)
	if _, err = NewWithReviewedRevisions(nil, trailing, hex.EncodeToString(h[:])); err == nil {
		t.Fatal("trailing review payload admitted")
	}
}

func TestOpticalRegistryReviewUsesRealCompiledArtifactAndExplicitAttestation(t *testing.T) {
	b, raw, digest := reviewedOpticalRegistryFixture(t)
	base, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	ds := []Deployment{{ChainID: 125, Address: common.HexToAddress("0x123"), Family: b.Name, Version: b.ABIHash, From: 100, Attested: true}}
	admitted, err := base.WithReviewedOpticalRegistry(ds, raw, digest)
	if err != nil || !admitted.RevisionKnown(b.Name, b.ABIHash) {
		t.Fatal("real reviewed optional artifact unavailable", err)
	}
	if base.RevisionKnown(b.Name, b.ABIHash) {
		t.Fatal("base frozen registry mutated")
	}
	if _, err = base.WithReviewedOpticalRegistry(ds, raw, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unreviewed optical artifact admitted")
	}
	ds[0].Attested = false
	if _, err = base.WithReviewedOpticalRegistry(ds, raw, digest); err == nil {
		t.Fatal("optional deployment auto-enabled")
	}
}

//go:embed testdata/repair-m924/*.json
var opticalRegistryFixture embed.FS

const opticalRegistryReviewSHA256 = "da1d4cad4551158d234430a096addde5ec3432065ebd05be99aaf6cb7e6e3408"

func opticalFixtureSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func reviewedOpticalRegistryFixture(t *testing.T) (Bundle, []byte, string) {
	t.Helper()
	read := func(name string) []byte {
		t.Helper()
		raw, err := opticalRegistryFixture.ReadFile("testdata/repair-m924/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	raw := read("review")
	if opticalFixtureSHA256(raw) != opticalRegistryReviewSHA256 {
		t.Fatal("checked-in optical review digest differs")
	}
	var review OpticalRegistryReview
	if err := json.Unmarshal(raw, &review); err != nil {
		t.Fatal(err)
	}
	b := review.Bundle
	artifact, build := read("artifact"), read("build-input")
	if opticalFixtureSHA256(artifact) != b.ArtifactHash || opticalFixtureSHA256(build) != b.BuildHash {
		t.Fatal("checked-in compiler artifact or build input digest differs")
	}
	var compiled struct {
		ABI      json.RawMessage `json:"abi"`
		Metadata string          `json:"metadata"`
		EVM      struct {
			Bytecode struct {
				Object string `json:"object"`
			} `json:"bytecode"`
			DeployedBytecode struct {
				Object string `json:"object"`
			} `json:"deployedBytecode"`
		} `json:"evm"`
	}
	var info struct {
		Compiler string `json:"solcLongVersion"`
		Input    struct {
			Language string `json:"language"`
			Sources  map[string]struct {
				Content string `json:"content"`
			} `json:"sources"`
			Settings struct {
				Optimizer struct {
					Enabled bool `json:"enabled"`
					Runs    int  `json:"runs"`
				} `json:"optimizer"`
				ViaIR bool `json:"viaIR"`
			} `json:"settings"`
		} `json:"input"`
	}
	if err := json.Unmarshal(artifact, &compiled); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(build, &info); err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Compiler struct {
			Version string `json:"version"`
		} `json:"compiler"`
		Sources map[string]struct {
			Keccak256 string `json:"keccak256"`
		} `json:"sources"`
		Settings struct {
			CompilationTarget map[string]string `json:"compilationTarget"`
		} `json:"settings"`
	}
	if err := json.Unmarshal([]byte(compiled.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if info.Compiler != b.Compiler || !strings.HasPrefix(info.Compiler, metadata.Compiler.Version) ||
		metadata.Settings.CompilationTarget[b.SourceName] != b.Name || info.Input.Language != "Solidity" ||
		!info.Input.Settings.Optimizer.Enabled || info.Input.Settings.Optimizer.Runs != 200 || !info.Input.Settings.ViaIR ||
		len(common.FromHex(compiled.EVM.Bytecode.Object)) == 0 || len(common.FromHex(compiled.EVM.DeployedBytecode.Object)) == 0 {
		t.Fatal("actual compiler identity, target, settings or bytecode missing")
	}
	source, ok := info.Input.Sources[b.SourceName]
	if !ok || opticalFixtureSHA256([]byte(source.Content)) != b.SourceHash {
		t.Fatal("reviewed source differs from compiler input")
	}
	if len(metadata.Sources) == 0 || len(metadata.Sources) != len(info.Input.Sources) {
		t.Fatal("compiler source inventory differs")
	}
	for name, source := range info.Input.Sources {
		if crypto.Keccak256Hash([]byte(source.Content)).Hex() != metadata.Sources[name].Keccak256 {
			t.Fatalf("compiler metadata source identity differs: %s", name)
		}
	}
	canonicalABI := func(raw json.RawMessage) []byte {
		t.Helper()
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return canonical
	}
	canonical := canonicalABI(compiled.ABI)
	if opticalFixtureSHA256(canonical) != b.ABIHash || !bytes.Equal(canonical, canonicalABI(b.ABI)) {
		t.Fatal("reviewed ABI differs from actual compiler artifact")
	}
	return b, raw, opticalRegistryReviewSHA256
}
