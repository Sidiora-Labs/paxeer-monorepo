package runtimedocs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func currentSurfaceInputs(t *testing.T) Inputs {
	t.Helper()
	in := realInputs(t)
	in.Manifest = deployment.Current()
	var err error
	in.ReviewedCatalog, err = os.ReadFile(filepath.Join(repoRoot(t), "sdk", "launchpad", "protocol", "catalog", "reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	in.Anchor.Height = in.Manifest.ActivationHeight
	return in
}

func TestReceiptOptionsUseSurfaceFactoryRuntimeIdentity(t *testing.T) {
	for _, staged := range []bool{false, true} {
		for _, override := range []common.Hash{{}, common.HexToHash("0x01")} {
			in := currentSurfaceInputs(t)
			factory := in.Manifest.Roles["SidioraFactory"]
			if !staged {
				factory.StagedRuntimeHash, factory.StagedUpgrade = "", ""
				in.Manifest.Roles["SidioraFactory"] = factory
			}
			in.PoolRuntimeHash = override
			options, catalog, err := ReceiptOptions(in)
			if err != nil {
				t.Fatal(err)
			}
			want := receipt.RPCDiscovery{Role: "SidioraPool", Kind: "surface", ABIRevision: catalog.Revisions["SidioraPool"], RuntimeHash: factory.RuntimeHash, StagedRuntimeHash: factory.StagedRuntimeHash, StagedUpgrade: factory.StagedUpgrade}
			if len(options.Discovery) != 2 || !reflect.DeepEqual(options.Discovery[0], want) {
				t.Fatalf("surface discovery with staged=%t override=%s: %+v, want %+v", staged, override, options.Discovery, want)
			}
			if options.Discovery[1].Role != "SidioraERC20" || options.Discovery[1].StagedRuntimeHash != "" || options.Discovery[1].StagedUpgrade != "" || len(options.Discovery[1].Immutables) == 0 {
				t.Fatalf("token discovery %+v", options.Discovery[1])
			}
			body, err := json.Marshal(options)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "receipt.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(body)
			var decoded receipt.RPCOptions
			if err := config.ReadBoundDocument(path, hex.EncodeToString(digest[:]), &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded.Discovery, options.Discovery) {
				t.Fatalf("receipt reader changed discovery: %+v", decoded.Discovery)
			}
			if !staged {
				poolJSON, err := json.Marshal(options.Discovery[0])
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(poolJSON, []byte("StagedRuntimeHash")) || bytes.Contains(poolJSON, []byte("StagedUpgrade")) {
					t.Fatalf("unstaged pool document includes staged members: %s", poolJSON)
				}
			}
		}
	}
}

func TestReceiptOptionsRequireLegacyPoolRuntime(t *testing.T) {
	in := realInputs(t)
	options, catalog, err := ReceiptOptions(in)
	if err != nil {
		t.Fatal(err)
	}
	want := receipt.RPCDiscovery{Role: "SidioraPool", Kind: "beacon_proxy", ABIRevision: catalog.Revisions["SidioraPool"], RuntimeHash: in.PoolRuntimeHash.Hex()}
	if !reflect.DeepEqual(options.Discovery[0], want) {
		t.Fatalf("legacy pool discovery %+v, want %+v", options.Discovery[0], want)
	}
	in.PoolRuntimeHash = common.Hash{}
	if _, _, err := ReceiptOptions(in); err == nil {
		t.Fatal("legacy deployment accepted without a pool runtime hash")
	}
}
