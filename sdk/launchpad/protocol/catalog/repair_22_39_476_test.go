package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReviewedHistoricalEmitterRevisionDecodesItsOwnEvent(t *testing.T) {
	rawABI, err := os.ReadFile("../bindings/abi/eventemitter_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var parsed any
	if err := json.Unmarshal(rawABI, &parsed); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	version := sha256.Sum256(canonical)
	zeroDigest := strings.Repeat("0", 64)
	bundle := Bundle{
		Name: "EventEmitter", SourceName: "contracts/core/EventEmitter.sol",
		ArtifactPath: "artifacts/contracts/core/EventEmitter.sol/EventEmitter.json",
		Compiler:     "reviewed", ABI: canonical, ABIHash: hex.EncodeToString(version[:]),
		SourceHash: zeroDigest, ArtifactHash: zeroDigest, BuildHash: zeroDigest,
	}
	review, err := json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{bundle}})
	if err != nil {
		t.Fatal(err)
	}
	reviewHash := sha256.Sum256(review)
	address := common.HexToAddress("0x4321")
	deployment := Deployment{ChainID: 125, Address: address, Family: bundle.Name, Version: bundle.ABIHash, From: 10, Attested: true}
	registry, err := NewWithReviewedRevisions([]Deployment{deployment}, review, hex.EncodeToString(reviewHash[:]))
	if err != nil {
		t.Fatal(err)
	}
	if !registry.RevisionKnown(bundle.Name, bundle.ABIHash) {
		t.Fatal("reviewed emitter revision absent")
	}
	var absent *Registry
	if absent.RevisionKnown(bundle.Name, bundle.ABIHash) {
		t.Fatal("missing registry admitted a revision")
	}
	if got, ok := registry.DeploymentAt(125, address, 10); !ok || got.Version != bundle.ABIHash {
		t.Fatal("reviewed deployment absent at activation")
	}
	a, err := abi.JSON(strings.NewReader(string(canonical)))
	if err != nil {
		t.Fatal(err)
	}
	event, ok := a.Events["Upgraded"]
	if !ok {
		t.Fatal("historical upgrade event absent")
	}
	log := types.Log{Address: address, BlockNumber: 10, Topics: []common.Hash{event.ID, common.BytesToHash(common.HexToAddress("0xabcd").Bytes())}}
	result := registry.Decode(125, log)
	if result.Status != "decoded" || result.Version != bundle.ABIHash {
		t.Fatalf("historical event: %+v", result)
	}
	if _, err := NewWithReviewedRevisions([]Deployment{deployment, deployment}, review, hex.EncodeToString(reviewHash[:])); err == nil {
		t.Fatal("overlapping historical deployment accepted")
	}
	for _, item := range []struct {
		name   string
		change func(*Bundle)
	}{
		{"DifferentFamily", func(b *Bundle) { b.Name = "SidioraNFT" }},
		{"MissingSource", func(b *Bundle) { b.SourceName = "" }},
		{"UnreviewedSourcePath", func(b *Bundle) { b.SourceName = "../EventEmitter.sol" }},
		{"MissingArtifactDigest", func(b *Bundle) { b.ArtifactHash = "" }},
		{"DifferentABI", func(b *Bundle) { b.ABIHash = zeroDigest }},
	} {
		t.Run(item.name, func(t *testing.T) {
			changed := bundle
			item.change(&changed)
			raw, err := json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{changed}})
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			if _, err := NewWithReviewedRevisions([]Deployment{deployment}, raw, hex.EncodeToString(sum[:])); err == nil {
				t.Fatal("unsupported reviewed revision admitted")
			}
		})
	}
	for _, item := range []Deployment{
		{ChainID: 1, Address: address, Family: bundle.Name, Version: bundle.ABIHash, Attested: true},
		{ChainID: 125, Address: address, Family: bundle.Name, Version: "unknown", Attested: true},
		{ChainID: 125, Address: address, Family: bundle.Name, Version: bundle.ABIHash, From: 11, To: 10, Attested: true},
	} {
		if _, err := NewWithReviewedRevisions([]Deployment{item}, review, hex.EncodeToString(reviewHash[:])); err == nil {
			t.Fatal("invalid archived deployment admitted")
		}
	}
	for _, raw := range [][]byte{[]byte(`{`), []byte(`{"schemaVersion":0,"bundles":[]}`)} {
		sum := sha256.Sum256(raw)
		if _, err := NewWithReviewedRevisions(nil, raw, hex.EncodeToString(sum[:])); err == nil {
			t.Fatal("incomplete archived review admitted")
		}
	}
	malformedABI := bundle
	malformedABI.ABI = []byte(`[{"type":"event","name":"Broken","inputs":"invalid"}]`)
	malformedHash := sha256.Sum256(malformedABI.ABI)
	malformedABI.ABIHash = hex.EncodeToString(malformedHash[:])
	malformedReview, err := json.Marshal(ReviewedRevision{SchemaVersion: 1, Bundles: []Bundle{malformedABI}})
	if err != nil {
		t.Fatal(err)
	}
	malformedReviewHash := sha256.Sum256(malformedReview)
	if _, err := NewWithReviewedRevisions(nil, malformedReview, hex.EncodeToString(malformedReviewHash[:])); err == nil {
		t.Fatal("archived revision with invalid ABI admitted")
	}
}

func TestReviewedCatalogSeparatesMissingRegistrySignatureAndOverlappingDeployment(t *testing.T) {
	r := registry(t)
	address := common.HexToAddress("0x1")
	event := r.abis["SidioraERC20"].Events["Transfer"]
	log := types.Log{Address: address, BlockNumber: 12, Topics: []common.Hash{event.ID, {}, {}}, Data: make([]byte, 32)}
	var absent *Registry
	if got := absent.Decode(125, log); got.Status != "origin_unrecognized" {
		t.Fatalf("missing registry selected an event: %s", got.Status)
	}
	log.Topics = nil
	if got := r.Decode(125, log); got.Status != "signature_unrecognized" || got.EmitterRole != "SidioraERC20" {
		t.Fatalf("origin without a signature gained event authority: %+v", got)
	}
	version := revision(t, "SidioraERC20")
	first := Deployment{ChainID: 125, Address: address, Family: "SidioraERC20", Version: version, From: 10, To: 20, Attested: true}
	second := first
	second.From = 19
	second.To = 30
	if _, err := New([]Deployment{first, second}); err == nil {
		t.Fatal("overlapping reviewed deployment epochs admitted")
	}
}

func TestReviewedCatalogRejectsMalformedSourceAndOversizedNestedValue(t *testing.T) {
	original := reviewed
	reviewed = []byte(`{`)
	t.Cleanup(func() { reviewed = original })
	if _, err := Bundles(); err == nil {
		t.Fatal("malformed reviewed source parsed as a catalog")
	}
	if _, err := New(nil); err == nil {
		t.Fatal("malformed reviewed source entered a registry")
	}
	if _, err := Index(); err == nil {
		t.Fatal("malformed reviewed source produced an index")
	}
	reviewed = original
	seen := 0
	value := struct{ Items []byte }{Items: make([]byte, MaxElements+1)}
	if boundedValue(reflect.ValueOf(value), 0, &seen) || seen <= MaxElements {
		t.Fatal("oversized nested reviewed value admitted")
	}
}

func TestCatalogBoundsUseTheReviewedABIShapes(t *testing.T) {
	for _, signature := range []string{"bytes", "string", "uint256[]", "uint256[2]"} {
		typ, err := abi.NewType(signature, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		args := abi.Arguments{{Type: typ}}
		var value any
		switch signature {
		case "bytes":
			value = []byte("record")
		case "string":
			value = "record"
		case "uint256[]":
			value = []*big.Int{big.NewInt(1), big.NewInt(2)}
		case "uint256[2]":
			value = [2]*big.Int{big.NewInt(1), big.NewInt(2)}
		}
		encoded, err := args.Pack(value)
		if err != nil {
			t.Fatal(err)
		}
		if !ArgumentsBounded(args, encoded) {
			t.Fatalf("canonical %s rejected", signature)
		}
		if ArgumentsBounded(args, encoded[:len(encoded)-32]) {
			t.Fatalf("truncated %s admitted", signature)
		}
	}
	if ArgumentsBounded(nil, make([]byte, MaxPayload+1)) {
		t.Fatal("oversized call admitted")
	}
	for _, item := range []struct {
		second string
		value  any
	}{
		{"uint256", big.NewInt(4)},
		{"string", "memo"},
	} {
		typ, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{{Name: "amount", Type: "uint256"}, {Name: "detail", Type: item.second}})
		if err != nil {
			t.Fatal(err)
		}
		tuple := reflect.New(typ.GetType()).Elem()
		tuple.Field(0).Set(reflect.ValueOf(big.NewInt(3)))
		tuple.Field(1).Set(reflect.ValueOf(item.value))
		args := abi.Arguments{{Type: typ}}
		encoded, err := args.Pack(tuple.Interface())
		if err != nil {
			t.Fatal(err)
		}
		if !ArgumentsBounded(args, encoded) || ArgumentsBounded(args, encoded[:len(encoded)-32]) {
			t.Fatalf("tuple with %s failed bounded admission", item.second)
		}
	}
	seen := 0
	if !boundedValue(reflect.ValueOf([]*big.Int{big.NewInt(1), nil}), 0, &seen) || seen != 3 {
		t.Fatal("bounded decoded array rejected")
	}
	seen = 0
	if boundedValue(reflect.ValueOf(make([]byte, MaxElements+1)), 0, &seen) {
		t.Fatal("oversized decoded array admitted")
	}
	seen = 0
	if boundedValue(reflect.ValueOf(big.NewInt(1)), 17, &seen) {
		t.Fatal("overdeep decoded value admitted")
	}
	seen = 0
	if !boundedValue(reflect.Value{}, 0, &seen) {
		t.Fatal("absent optional decoded value rejected")
	}
	if _, ok := number(nil, 0); ok {
		t.Fatal("missing ABI word admitted")
	}
	invalidWord := make([]byte, 32)
	invalidWord[0] = 1
	if _, ok := number(invalidWord, 0); ok {
		t.Fatal("oversized ABI integer admitted")
	}
	uintType, err := abi.NewType("uint256", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	seen = MaxElements
	if value(uintType, make([]byte, 32), 0, 0, &seen) {
		t.Fatal("element count overflow admitted")
	}
	arrayType, err := abi.NewType("uint256[2]", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	seen = MaxElements - 1
	if value(arrayType, make([]byte, 64), 0, 0, &seen) {
		t.Fatal("fixed array beyond element limit admitted")
	}
	seen = 0
	if value(uintType, make([]byte, 32), 0, 17, &seen) {
		t.Fatal("overdeep ABI value admitted")
	}
	if _, err := NativeLaunchpadABI(); err != nil {
		t.Fatal(err)
	}
	sources, err := CompiledSourceSHA256()
	if err != nil {
		t.Fatal(err)
	}
	if sources["protocol/catalog/catalog.go"] == "" || sources["protocol/catalog/native_launchpad.json"] == "" {
		t.Fatal("compiled catalog source identity missing")
	}
}

func TestReviewedOpticalRegistryRequiresItsRemovalEvent(t *testing.T) {
	raw, err := os.ReadFile("testdata/repair-m924/review.json")
	if err != nil {
		t.Fatal(err)
	}
	var review OpticalRegistryReview
	if err := json.Unmarshal(raw, &review); err != nil {
		t.Fatal(err)
	}
	base, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	address := common.HexToAddress("0x6789")
	deployment := Deployment{ChainID: 125, Address: address, Family: review.Bundle.Name, Version: review.Bundle.ABIHash, Attested: true}
	digest := sha256.Sum256(raw)
	selected, err := base.WithReviewedOpticalRegistry([]Deployment{deployment}, raw, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selected.WithReviewedOpticalRegistry([]Deployment{deployment}, raw, hex.EncodeToString(digest[:])); err == nil {
		t.Fatal("same optical revision admitted twice")
	}
	var absent *Registry
	if _, err := absent.WithReviewedOpticalRegistry([]Deployment{deployment}, raw, hex.EncodeToString(digest[:])); err == nil {
		t.Fatal("optical review without base registry admitted")
	}
	for _, invalid := range [][]byte{[]byte(`{`), append(append([]byte(nil), raw...), []byte(`{}`)...)} {
		sum := sha256.Sum256(invalid)
		if _, err := base.WithReviewedOpticalRegistry([]Deployment{deployment}, invalid, hex.EncodeToString(sum[:])); err == nil {
			t.Fatal("malformed optical review admitted")
		}
	}
	invalidABI := review
	invalidABI.Bundle.ABI = []byte(`[{"type":"event","name":"OpticalDeregistered","inputs":"invalid"}]`)
	invalidABIHash := sha256.Sum256(invalidABI.Bundle.ABI)
	invalidABI.Bundle.ABIHash = hex.EncodeToString(invalidABIHash[:])
	invalidReview, err := json.Marshal(invalidABI)
	if err != nil {
		t.Fatal(err)
	}
	invalidReviewHash := sha256.Sum256(invalidReview)
	if _, err := base.WithReviewedOpticalRegistry(nil, invalidReview, hex.EncodeToString(invalidReviewHash[:])); err == nil {
		t.Fatal("optical revision with invalid ABI admitted")
	}
	var events []map[string]any
	if err := json.Unmarshal(review.Bundle.ABI, &events); err != nil {
		t.Fatal(err)
	}
	retained := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if event["name"] != "OpticalDeregistered" {
			retained = append(retained, event)
		}
	}
	if len(retained) != len(events)-1 {
		t.Fatal("reviewed removal event absent from fixture")
	}
	review.Bundle.ABI, err = json.Marshal(retained)
	if err != nil {
		t.Fatal(err)
	}
	abiHash := sha256.Sum256(review.Bundle.ABI)
	review.Bundle.ABIHash = hex.EncodeToString(abiHash[:])
	changed, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	changedHash := sha256.Sum256(changed)
	deployment.Version = review.Bundle.ABIHash
	if _, err := base.WithReviewedOpticalRegistry([]Deployment{deployment}, changed, hex.EncodeToString(changedHash[:])); err == nil {
		t.Fatal("optical review without removal event admitted")
	}
}
