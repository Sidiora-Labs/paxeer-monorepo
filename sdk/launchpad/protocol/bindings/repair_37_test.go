package bindings_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func repair37Root(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func repair37Run(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real producer/consumer %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if !bytes.Contains(output, []byte("--- PASS: TestRepair")) {
		t.Fatalf("producer/consumer did not execute its proof:\n%s", output)
	}
	t.Logf("%s\n%s", strings.Join(args, " "), output)
}
func repair37Overlay(t *testing.T, root, packagePath, source string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "repair37", source))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "bridge_test.go")
	if err = os.WriteFile(replacement, raw, 0600); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	data, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(root, packagePath, "repair37_bridge_test.go"): replacement}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	return overlay
}
func TestRepair_37_native_family_events(t *testing.T) {
	root := repair37Root(t)
	raw, err := os.ReadFile(filepath.Join(root, "precompiles/launchpad/abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	native, err := bindings.NativeLaunchpadMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	assertProofABI(t, abi.ABI{Events: canonical.Events}, *native)
	if len(native.Events) != 12 || len(native.Methods) != 0 {
		t.Fatalf("native family inventory = %+v", native)
	}
	logFile := filepath.Join(t.TempDir(), "native-logs.json")
	env := []string{"REPAIR37_LOGS=" + logFile}
	overlay := repair37Overlay(t, root, "precompiles/launchpad", "native_producer.go.txt")
	repair37Run(t, root, env, "test", "-v", "-count=1", "-overlay", overlay, "./precompiles/launchpad/", "-run", "^TestRepair37NativeProducer$")
	raw, err = os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	var observations []struct {
		Name string
		Log  types.Log
		Args map[string]any
	}
	if err = json.Unmarshal(raw, &observations); err != nil {
		t.Fatal(err)
	}
	if len(observations) != 12 {
		t.Fatalf("producer returned %d logs", len(observations))
	}
	consumer, err := bindings.NewNativeLaunchpad(common.HexToAddress("0x1017"), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range observations {
		if seen[o.Name] {
			t.Fatalf("duplicate native event %s", o.Name)
		}
		seen[o.Name] = true
		definition, ok := protocol.LookupEvent(o.Log.Topics[0])
		if !ok || definition.Sig != native.Events[o.Name].Sig {
			t.Fatalf("native %s missing from bindings-based lookup", o.Name)
		}
		// Exercise each generated typed parser against bytes emitted by the real native producer.
		method := reflect.ValueOf(consumer).MethodByName("Parse" + o.Name)
		if !method.IsValid() {
			t.Fatalf("missing native parser %s", o.Name)
		}
		result := method.Call([]reflect.Value{reflect.ValueOf(o.Log)})
		if !result[1].IsNil() {
			t.Fatal(result[1].Interface())
		}
		parsed := result[0].Elem()
		for name, want := range o.Args {
			field := parsed.FieldByName(abi.ToCamelCase(name))
			if !field.IsValid() {
				t.Fatalf("%s missing %s", o.Name, name)
			}
			got := repair37Humanize(field.Interface())
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var normalized any
			if err = json.Unmarshal(encoded, &normalized); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalized, want) {
				t.Fatalf("%s.%s = %#v, want %#v", o.Name, name, normalized, want)
			}
		}
		unknown := o.Log
		unknown.Topics = append([]common.Hash(nil), o.Log.Topics...)
		unknown.Topics[0] = common.HexToHash("0xdeadbeef")
		result = method.Call([]reflect.Value{reflect.ValueOf(unknown)})
		if result[1].IsNil() {
			t.Fatalf("%s parser accepted unknown topic", o.Name)
		}
	}
	if def, ok := protocol.LookupEvent(common.HexToHash("0xdeadbeef")); ok || def != nil {
		t.Fatal("lookup accepted unknown topic")
	}
	decoderOverlay := repair37Overlay(t, root, "services/launchpad/indexer/internal/decode", "decoder_consumer.go.txt")
	repair37Run(t, filepath.Join(root, "services/launchpad/indexer"), env, "test", "-v", "-count=1", "-overlay", decoderOverlay, "./internal/decode/", "-run", "^TestRepair37DecoderConsumer$")
}
func repair37Humanize(v any) any {
	switch x := v.(type) {
	case *big.Int:
		return x.String()
	case common.Address:
		return strings.ToLower(x.Hex())
	case [32]byte:
		return "0x" + hex.EncodeToString(x[:])
	default:
		return v
	}
}
func repair37Artifact(t *testing.T, name string, metadata *bind.MetaData) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "repair37", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var a struct {
		Compiler     string
		ABI          json.RawMessage
		Bytecode     string
		SourceHashes map[string]string
	}
	if err = json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.Compiler, "0.8.27+") || len(common.FromHex(a.Bytecode)) == 0 || len(a.SourceHashes) == 0 {
		t.Fatal("missing real Solidity artifact provenance")
	}
	for path, want := range a.SourceHashes {
		source, err := os.ReadFile(filepath.Join(repair37Root(t), "contracts/launchpad", path))
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(source)
		if fmt.Sprintf("%x", got) != want {
			t.Fatalf("artifact source changed: %s", path)
		}
	}
	if metadata != nil {
		canonical, err := abi.JSON(bytes.NewReader(a.ABI))
		if err != nil {
			t.Fatal(err)
		}
		exported, err := metadata.GetAbi()
		if err != nil {
			t.Fatal(err)
		}
		assertProofABI(t, canonical, *exported)
	}
}
func TestRepair_37_retained_emitter_v2(t *testing.T) {
	repair37Artifact(t, "EventEmitter", bindings.EventEmitterMetaData)
	repair37Artifact(t, "ERC1967Proxy", nil)
	if bindings.EventEmitterMetadataAtHeight(99, 100) != bindings.HistoricalEventEmitterMetaData || bindings.EventEmitterMetadataAtHeight(100, 100) != bindings.EventEmitterMetaData {
		t.Fatal("retained ABI activation boundary incorrect")
	}
	historical, err := bindings.HistoricalEventEmitterMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	if len(historical.Methods) != 22 {
		t.Fatalf("historical methods = %d", len(historical.Methods))
	}
	if _, ok := historical.Events["EventLog"]; ok {
		t.Fatal("historical metadata contains v2 logs")
	}
	repair37Run(t, filepath.Join("testdata", "repair37", "contracts"), nil, "test", "-mod=mod", "-v", "-count=1", "-run", "^TestRepair_37_retained_emitter_v2$")
}
func TestRepair_37_beacon_ownership_handoff(t *testing.T) {
	repair37Artifact(t, "PoolBeacon", bindings.PoolBeaconMetaData)
	repair37Run(t, filepath.Join("testdata", "repair37", "contracts"), nil, "test", "-mod=mod", "-v", "-count=1", "-run", "^TestRepair_37_beacon_ownership_handoff$")
}
