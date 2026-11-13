package protocol_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

func TestCanonicalEventSchema(t *testing.T) {
	var topic common.Hash
	for _, tc := range []struct {
		meta *bind.MetaData
		name string
	}{
		{bindings.SidioraNFTMetaData, "tokenId"},
		{bindings.FeesRouterMetaData, "nftId"},
	} {
		parsed, err := tc.meta.GetAbi()
		if err != nil {
			t.Fatal(err)
		}
		original := parsed.Events["FeeStrategyChanged"]
		topic = original.ID
		if original.Inputs[0].Name != tc.name {
			t.Fatalf("role ABI mutated: %+v", original.Inputs)
		}
		want := original.Inputs
		def, ok := protocol.LookupEvent(topic)
		if !ok {
			t.Fatal("missing shared event")
		}
		if def.Sig != "FeeStrategyChanged(uint256,uint8,uint8)" || def.Event.ID != original.ID || def.Event.Inputs[0].Name != "nftId" || !slices.Equal(def.Contracts, []string{"SidioraNFT", "FeesRouter"}) {
			t.Fatalf("canonical definition: %+v", def)
		}
		if len(def.Event.Inputs) != len(want) {
			t.Fatal("argument arity changed")
		}
		for i, input := range def.Event.Inputs {
			expected := want[i]
			if i == 0 {
				expected.Name = "nftId"
			}
			if !reflect.DeepEqual(input, expected) {
				t.Fatalf("argument %d changed: %+v vs %+v", i, input, expected)
			}
		}
		def.Event.Inputs[0].Name = "caller mutation"
		again, _ := protocol.LookupEvent(topic)
		if again.Event.Inputs[0].Name != "nftId" {
			t.Fatal("caller changed canonical registry")
		}
	}
	found := false
	for _, def := range protocol.Events() {
		if def.Topic0 == topic {
			found = true
			if def.Event.Inputs[0].Name != "nftId" {
				t.Fatal("Events differs from LookupEvent")
			}
		}
	}
	if !found {
		t.Fatal("canonical event absent from Events")
	}
	emitter, err := bindings.EventEmitterMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	full := emitter.Events["FeeStrategyChanged"]
	def, ok := protocol.LookupEvent(full.ID)
	if !ok || def.Sig != full.Sig || !reflect.DeepEqual(def.Event.Inputs, full.Inputs) {
		t.Fatal("distinct hub event changed")
	}
	nft, err := bindings.SidioraNFTMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	transfer := nft.Events["Transfer"]
	def, ok = protocol.LookupEvent(transfer.ID)
	if !ok || def.Event.Inputs[2].Name != "tokenId" {
		t.Fatal("unrelated tokenId changed")
	}
}

func runEventSchemaProof(t *testing.T, dir string, env []string, program string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", program, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func eventSchemaOverlay(t *testing.T, root, target, source string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fee_strategy_events", source))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "bridge_test.go")
	if err = os.WriteFile(replacement, raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(root, target, "event_schema_bridge_test.go"): replacement}})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	if err = os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func TestEventArgumentCollisionIntegration(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	artifacts := t.TempDir()
	runEventSchemaProof(t, root, nil, "npm", "install", "--prefix", artifacts, "--no-audit", "--no-fund", "--ignore-scripts", "solc@0.8.27")
	runEventSchemaProof(t, root, nil, "node", filepath.Join(root, "sdk/launchpad/protocol/testdata/fee_strategy_events/compile.cjs"), root, artifacts)
	env := []string{"FEE_STRATEGY_ARTIFACTS=" + artifacts, "FEE_STRATEGY_LOGS=" + filepath.Join(artifacts, "logs.json")}
	producer := "sdk/launchpad/protocol/bindings/testdata/contract_events/contracts"
	overlay := eventSchemaOverlay(t, root, producer, "producer.go.txt")
	out := runEventSchemaProof(t, filepath.Join(root, producer), env, "go", "test", "-mod=mod", "-v", "-count=1", "-overlay", overlay, "-run", "^TestFeeStrategyEventProducer$")
	if !strings.Contains(out, "--- PASS: TestFeeStrategyEventProducer") {
		t.Fatalf("producer proof did not execute:\n%s", out)
	}
	t.Log(out)
	overlay = eventSchemaOverlay(t, root, "services/launchpad/indexer/internal/processor", "consumer.go.txt")
	out = runEventSchemaProof(t, filepath.Join(root, "services/launchpad/indexer"), env, "go", "test", "-mod=readonly", "-v", "-count=1", "-overlay", overlay, "./internal/processor", "-run", "^TestFeeStrategyEventConsumer$")
	if !strings.Contains(out, "--- PASS: TestFeeStrategyEventConsumer") {
		t.Fatalf("consumer proof did not execute:\n%s", out)
	}
	t.Log(out)
}
