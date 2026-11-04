package catalog

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestRepairM_924_CheckoutProvenance(t *testing.T) {
	TestOpticalRegistryReviewUsesRealCompiledArtifactAndExplicitAttestation(t)
}

func TestRepairM_924_ReviewedArtifactConsumer(t *testing.T) {
	b, raw, digest := reviewedOpticalRegistryFixture(t)
	base, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	address := common.HexToAddress("0x123")
	deployment := Deployment{ChainID: 125, Address: address, Family: b.Name, Version: b.ABIHash, From: 100, To: 200, Attested: true}
	admitted, err := base.WithReviewedOpticalRegistry([]Deployment{deployment}, raw, digest)
	if err != nil {
		t.Fatal(err)
	}
	producer, err := abi.JSON(strings.NewReader(string(b.ABI)))
	if err != nil {
		t.Fatal(err)
	}
	event, ok := producer.Events["OpticalDeregistered"]
	if !ok || event.Sig != "OpticalDeregistered(address,uint256)" {
		t.Fatal("actual compiler removal event missing", event)
	}
	optical := common.HexToAddress("0x456")
	timestamp := big.NewInt(1700000000)
	data, err := event.Inputs.NonIndexed().Pack(timestamp)
	if err != nil {
		t.Fatal(err)
	}
	topics, err := abi.MakeTopics([]any{optical})
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Address: address, BlockNumber: 100, Topics: append([]common.Hash{event.ID}, topics[0]...), Data: data}
	decoded := admitted.Decode(125, log)
	if decoded.Status != "decoded" || decoded.Version != b.ABIHash || decoded.EmitterRole != b.Name || decoded.Signature != event.Sig || decoded.Fields["optical"] != optical {
		t.Fatalf("reviewed compiler artifact did not reach catalog consumer: %#v", decoded)
	}
	got, ok := decoded.Fields["timestamp"].(*big.Int)
	if !ok || got.Cmp(timestamp) != 0 {
		t.Fatalf("decoded timestamp = %v", decoded.Fields["timestamp"])
	}
	if base.RevisionKnown(b.Name, b.ABIHash) || base.Decode(125, log).Status != "origin_unrecognized" {
		t.Fatal("base frozen registry mutated")
	}
	if _, ok := base.DeploymentAt(125, address, 100); ok {
		t.Fatal("base deployment list mutated")
	}
	if d, ok := admitted.DeploymentAt(125, address, 100); !ok || d != deployment {
		t.Fatal("attested activation missing", d, ok)
	}
	for _, block := range []uint64{99, 200} {
		log.BlockNumber = block
		if admitted.Decode(125, log).Status != "origin_unrecognized" {
			t.Fatalf("optional deployment admitted outside activation at %d", block)
		}
	}
	log.BlockNumber = 100
	if admitted.Decode(1, log).Status != "origin_unrecognized" {
		t.Fatal("optional deployment admitted on another chain")
	}
}

func TestRepairM_924_ReviewRefusals(t *testing.T) {
	b, raw, digest := reviewedOpticalRegistryFixture(t)
	base, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	d := Deployment{ChainID: 125, Address: common.HexToAddress("0x123"), Family: b.Name, Version: b.ABIHash, From: 100, Attested: true}
	type refusal struct {
		name        string
		deployments []Deployment
		review      []byte
		digest      string
		reason      string
	}
	cases := []refusal{
		{"digest", []Deployment{d}, raw, strings.Repeat("0", 64), "unreviewed optical registry artifact"},
		{"no_attestation", nil, raw, digest, "optical deployment attestation required"},
		{"overlap", []Deployment{d, d}, raw, digest, "overlapping optical activation"},
	}
	for _, field := range []string{"attested", "chain", "address", "family", "version", "range"} {
		changed := d
		switch field {
		case "attested":
			changed.Attested = false
		case "chain":
			changed.ChainID = 1
		case "address":
			changed.Address = common.Address{}
		case "family":
			changed.Family = "EventEmitter"
		case "version":
			changed.Version = strings.Repeat("0", 64)
		case "range":
			changed.To = changed.From
		}
		cases = append(cases, refusal{field, []Deployment{changed}, raw, digest, "invalid optical deployment attestation"})
	}
	for _, field := range []string{"artifact", "source", "build", "compiler", "abi"} {
		changed := b
		switch field {
		case "artifact":
			changed.ArtifactHash = ""
		case "source":
			changed.SourceHash = ""
		case "build":
			changed.BuildHash = ""
		case "compiler":
			changed.Compiler = ""
		case "abi":
			changed.ABI = json.RawMessage(`[]`)
		}
		altered, err := json.Marshal(OpticalRegistryReview{SchemaVersion: 1, Kind: "reviewed-optical-registry", Bundle: changed})
		if err != nil {
			t.Fatal(err)
		}
		reason := "missing optical provenance"
		if field == "compiler" {
			reason = "wrong reviewed optical identity"
		}
		if field == "abi" {
			reason = "optical ABI hash differs"
		}
		cases = append(cases, refusal{field, []Deployment{d}, altered, opticalFixtureSHA256(altered), reason})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := base.WithReviewedOpticalRegistry(tc.deployments, tc.review, tc.digest); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("want %s refusal, got %v", tc.reason, err)
			}
			if base.RevisionKnown(b.Name, b.ABIHash) {
				t.Fatal("refused review mutated base revisions")
			}
			if _, ok := base.DeploymentAt(125, d.Address, 100); ok {
				t.Fatal("refused review mutated base deployments")
			}
		})
	}
}
