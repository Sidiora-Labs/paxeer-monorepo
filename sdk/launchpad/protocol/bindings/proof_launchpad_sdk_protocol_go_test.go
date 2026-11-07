// Proves: bindings.*MetaData -> bind.MetaData.GetAbi / abi.ABI codecs -> protocol.LookupEvent, decode.Decoder.Decode and chain.ReadAdapter.identifyPool's decoder.
//
// The eight metadata objects are the producers under this contract: they carry
// ABI definitions, not chain or database state. Initialized round trips below
// are schema checks; native lifecycle evidence must come from executor logs.
// Retained interfaces run reviewed Solidity artifacts in the focused proofs.
// The metadata proof creates fresh encoded
// results on each observation and invokes the actual indexer and chain consumer
// tests as subprocesses from their owning modules, without replacing a codec,
// registry, RPC layer or consumer with a test double. No service is required.
//
// Repairs: sdk/launchpad/protocol/events.go LookupEvent and Events exposed the
// mutable registry; a consumer could corrupt subsequent decodes. They now return
// independent definitions. sdk/launchpad/shared/chain/client.go identifyPool
// indexed an unchecked membership result and accepted trailing ABI words; its
// extracted decodePoolMembership now refuses malformed result shapes. The
// consumer subprocess asserts that same production decoder, including refusal.
package bindings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

func proofLaunchpadRoleMetadata(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	producers := []struct {
		name string
		meta *bind.MetaData
	}{
		{"FeeAccumulator", bindings.FeeAccumulatorMetaData},
		{"PoolRegistry", bindings.PoolRegistryMetaData},
		{"FeesRouter", bindings.FeesRouterMetaData},
		{"ProtocolConfig", bindings.ProtocolConfigMetaData},
		{"Quoter", bindings.QuoterMetaData},
		{"Router", bindings.RouterMetaData},
		{"SidioraFactory", bindings.SidioraFactoryMetaData},
		{"SidioraNFT", bindings.SidioraNFTMetaData},
	}
	for _, producer := range producers {
		t.Run(producer.name, func(t *testing.T) {
			source := filepath.Join(root, "precompiles/launchpad/abi", producer.name+".json")
			raw, err := os.ReadFile(source)
			if err != nil {
				t.Fatalf("missing producer %s: %v", source, err)
			}
			canonical, err := abi.JSON(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("malformed producer %s: %v", source, err)
			}
			exported, err := producer.meta.GetAbi()
			if err != nil || exported == nil {
				t.Fatalf("%s metadata: %v", producer.name, err)
			}
			// Compare wire shapes as well as selectors: output and indexed-field drift
			// would otherwise compile while breaking these consumers at runtime.
			assertProofABI(t, canonical, *exported)
			for _, event := range exported.Events {
				observed, ok := protocol.LookupEvent(event.ID)
				if !ok || observed == nil || observed.Sig != event.Sig || !slices.Contains(observed.Contracts, producer.name) {
					t.Fatalf("%s.%s did not reach protocol.LookupEvent: %+v", producer.name, event.Name, observed)
				}
			}
			initialized, ok := exported.Events["Initialized"]
			if !ok {
				t.Fatalf("%s producer has no Initialized event", producer.name)
			}
			for version := uint64(1); version <= 2; version++ {
				// GetAbi may cache the immutable schema; encoded values and decoded
				// observations must still be fresh and must follow the changed producer.
				data, err := initialized.Inputs.NonIndexed().Pack(version)
				if err != nil {
					t.Fatal(err)
				}
				consumer, ok := protocol.LookupEvent(initialized.ID)
				if !ok {
					t.Fatal("Initialized not registered")
				}
				observed := make(map[string]any)
				if err := consumer.Event.Inputs.NonIndexed().UnpackIntoMap(observed, data); err != nil {
					t.Fatal(err)
				}
				if observed["version"] != version {
					t.Fatalf("consumer version = %v, want %d", observed["version"], version)
				}
				if err := consumer.Event.Inputs.NonIndexed().UnpackIntoMap(make(map[string]any), data[:len(data)-1]); err == nil {
					t.Fatal("consumer accepted a truncated producer result")
				}
				// Mutating either public registry API must not alter a later lookup.
				consumer.Name = "corrupted"
				consumer.Contracts[0] = "corrupted"
				consumer.Event.Inputs[0].Name = "corrupted"
				for _, entry := range protocol.Events() {
					if entry.Topic0 == initialized.ID {
						entry.Contracts[0] = "corrupted"
						entry.Event.Inputs[0].Name = "corrupted"
					}
				}
				again, ok := protocol.LookupEvent(initialized.ID)
				if !ok || again.Name != "Initialized" || again.Event.Inputs[0].Name != "version" || !slices.Contains(again.Contracts, producer.name) {
					t.Fatalf("caller mutation corrupted the registry: %+v", again)
				}
			}
		})
	}
	if observed, ok := protocol.LookupEvent(common.HexToHash("0xdeadbeef")); ok || observed != nil {
		t.Fatalf("unknown producer identifier was accepted: %+v", observed)
	}
}

func TestProofLaunchpadSdkProtocolGo(t *testing.T) {
	proofLaunchpadRoleMetadata(t)
	t.Run("RetainedEmitterV2", TestRepairM_948_RetainedEmitterV2)
	t.Run("BeaconOwnershipHandoff", TestRepairM_948_BeaconOwnershipHandoff)
	t.Run("NativeExecutorTransport", TestRepairM_948_NativeExecutorTransport)
	t.Run("NativeExecutorNode", TestRepairM_948_NativeExecutorNode)
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	// These modules own internal consumer functions and their dependency graphs.
	// Invoke their real Go tests rather than copying their implementations into
	// the lowest-level protocol module or adding an upward module dependency.
	runProofConsumer(t, filepath.Join(root, "services/launchpad/indexer"), "./internal/processor", "TestProofLaunchpadSdkProtocolGoIndexerConsumer")
	runProofConsumer(t, filepath.Join(root, "sdk/launchpad/shared"), "./chain", "TestProofLaunchpadSdkProtocolGoPoolMembership")
}

func assertProofABI(t *testing.T, want, got abi.ABI) {
	t.Helper()
	// Internal Solidity type annotations do not affect the exported wire format.
	arguments := func(args abi.Arguments) []string {
		result := make([]string, len(args))
		for i, arg := range args {
			result[i] = arg.Name + ":" + arg.Type.String()
			if arg.Indexed {
				result[i] += ":indexed"
			}
		}
		return result
	}
	if len(want.Methods) != len(got.Methods) || len(want.Events) != len(got.Events) || len(want.Errors) != len(got.Errors) {
		t.Fatalf("ABI inventory differs: methods %d/%d, events %d/%d, errors %d/%d", len(got.Methods), len(want.Methods), len(got.Events), len(want.Events), len(got.Errors), len(want.Errors))
	}
	for name, expected := range want.Methods {
		observed, ok := got.Methods[name]
		if !ok || observed.Sig != expected.Sig || !bytes.Equal(observed.ID, expected.ID) || observed.StateMutability != expected.StateMutability || !reflect.DeepEqual(arguments(observed.Inputs), arguments(expected.Inputs)) || !reflect.DeepEqual(arguments(observed.Outputs), arguments(expected.Outputs)) {
			t.Fatalf("exported method %s differs from the producer ABI", name)
		}
	}
	for name, expected := range want.Events {
		observed, ok := got.Events[name]
		if !ok || observed.Sig != expected.Sig || observed.ID != expected.ID || observed.Anonymous != expected.Anonymous || !reflect.DeepEqual(arguments(observed.Inputs), arguments(expected.Inputs)) {
			t.Fatalf("exported event %s differs from the producer ABI", name)
		}
	}
	for name, expected := range want.Errors {
		observed, ok := got.Errors[name]
		if !ok || observed.Sig != expected.Sig || observed.ID != expected.ID || !reflect.DeepEqual(arguments(observed.Inputs), arguments(expected.Inputs)) {
			t.Fatalf("exported error %s differs from the producer ABI", name)
		}
	}
}

func runProofConsumer(t *testing.T, directory, pkg, testName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", pkg, "-run", "^"+testName+"$")
	cmd.Dir = directory
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("consumer %s in %s failed: %v\n%s\n%s", testName, directory, err, stdout.String(), stderr.String())
	}
	passed := false
	decoder := json.NewDecoder(&stdout)
	for {
		var event struct{ Action, Test string }
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("consumer evidence: %v", err)
		}
		if event.Action == "skip" {
			t.Fatalf("consumer skipped %s", event.Test)
		}
		if event.Test == testName && event.Action == "pass" {
			passed = true
		}
	}
	if !passed {
		t.Fatalf("consumer %s did not execute and pass", testName)
	}
}
