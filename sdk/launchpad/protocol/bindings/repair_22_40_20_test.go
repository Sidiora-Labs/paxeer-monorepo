package bindings_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	b "github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestNativeBindingCarriesTheChainLaunch(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "precompiles/launchpad/abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	producer, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	binding, err := b.NativeLaunchpadMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	fromCatalog, err := catalog.NativeLaunchpadABI()
	if err != nil {
		t.Fatal(err)
	}
	if len(binding.Methods) != 13 || len(binding.Events) != len(producer.Events) || len(binding.Errors) != len(producer.Errors) {
		t.Fatalf("native ABI inventory differs: methods %d, events %d/%d, errors %d/%d", len(binding.Methods), len(binding.Events), len(producer.Events), len(binding.Errors), len(producer.Errors))
	}
	if !reflect.DeepEqual(producer, fromCatalog) {
		t.Fatal("catalog native ABI differs from the producer")
	}
	version := sha256.Sum256(raw)
	if catalog.NativeLaunchpadVersion() != hex.EncodeToString(version[:]) {
		t.Fatal("catalog native ABI version differs from the producer")
	}
	registry, err := catalog.New([]catalog.Deployment{{
		ChainID: 125, Address: common.HexToAddress("0x1017"), Family: "NativeLaunchpad",
		Version: catalog.NativeLaunchpadVersion(), Attested: true,
	}})
	if err != nil || !registry.RevisionKnown("NativeLaunchpad", catalog.NativeLaunchpadVersion()) {
		t.Fatalf("catalog native deployment registration differs: %v", err)
	}
	sameArguments := func(got, want abi.Arguments) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i].Name != want[i].Name || got[i].Indexed != want[i].Indexed ||
				got[i].Type.String() != want[i].Type.String() ||
				!reflect.DeepEqual(got[i].Type.TupleRawNames, want[i].Type.TupleRawNames) {
				return false
			}
		}
		return true
	}
	for _, name := range []string{
		"launchChain", "launchChainWithPermit", "openChainRevenueEpoch", "claimChainRevenue",
		"claimChainRevenueBatch", "pauseLaunchedChain", "resumeLaunchedChain", "retireLaunchedChain",
		"launchedChain", "launchedChainOfPool", "homeChainOf", "chainRevenueEpoch", "chainRevenueClaimable",
	} {
		want, exists := producer.Methods[name]
		if !exists {
			t.Fatalf("producer function %s is absent", name)
		}
		got, exists := binding.Methods[name]
		if !exists || got.Sig != want.Sig || !bytes.Equal(got.ID, want.ID) || got.StateMutability != want.StateMutability || !sameArguments(got.Inputs, want.Inputs) || !sameArguments(got.Outputs, want.Outputs) {
			t.Fatalf("native binding function %s differs from the producer", name)
		}
	}
	want, exists := producer.Events["ChainLaunched"]
	if !exists {
		t.Fatal("producer ChainLaunched event is absent")
	}
	got, exists := binding.Events["ChainLaunched"]
	if !exists || got.Sig != want.Sig || got.ID != want.ID || got.Anonymous != want.Anonymous || !sameArguments(got.Inputs, want.Inputs) {
		t.Fatal("native binding ChainLaunched event differs from the producer")
	}
}

func TestRecordedSourceHashesMatchTheTree(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native-execution.json", "native-node-rpc.json"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "repair_m948", name))
			if err != nil {
				t.Fatal(err)
			}
			var record map[string]json.RawMessage
			if err := json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			check := func(table json.RawMessage, requireChainLaunch bool) {
				var hashes map[string]string
				if err := json.Unmarshal(table, &hashes); err != nil || len(hashes) == 0 {
					t.Fatalf("source hashes are absent: %v", err)
				}
				if requireChainLaunch {
					for _, path := range []string{
						"modules/launchpad/keeper/chainlaunch.go", "modules/launchpad/keeper/chainlaunch_store.go",
						"modules/launchpad/keeper/chainrevenue.go", "modules/launchpad/types/chainlaunch.go",
						"precompiles/launchpad/chainlaunch.go", "precompiles/launchpad/abi/ChainLaunch.json",
					} {
						if _, exists := hashes[path]; !exists {
							t.Errorf("chain launch source hash is absent for %s", path)
						}
					}
				}
				for path, recorded := range hashes {
					if filepath.IsAbs(path) || strings.HasPrefix(filepath.Clean(path), "..") {
						t.Fatalf("source path is outside the tree: %s", path)
					}
					source, err := os.ReadFile(filepath.Join(root, path))
					if err != nil {
						t.Fatal(err)
					}
					actual := sha256.Sum256(source)
					if recorded != hex.EncodeToString(actual[:]) {
						t.Errorf("recorded source hash differs for %s", path)
					}
				}
			}
			check(record["SourceHashes"], name == "native-node-rpc.json")
			if name == "native-execution.json" {
				var node map[string]json.RawMessage
				if err := json.Unmarshal(record["NodeRPC"], &node); err != nil {
					t.Fatal(err)
				}
				check(node["SourceHashes"], true)
			}
		})
	}
}
