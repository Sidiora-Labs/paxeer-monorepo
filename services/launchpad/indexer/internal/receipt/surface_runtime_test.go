package receipt

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestSurfacePoolRuntimeQualification(t *testing.T) {
	node := privateNode(t)
	manifest := deployment.Current()
	factory := manifest.Roles["SidioraFactory"]
	programs := []string{
		"0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3",
		migratedSurfaceRuntime,
	}
	var digests []string
	for i, program := range programs {
		runtime := common.FromHex(program)
		init := append([]byte{0x60, byte(len(runtime)), 0x60, 0x0c, 0x60, 0, 0x39, 0x60, byte(len(runtime)), 0x60, 0, 0xf3}, runtime...)
		address, tx, _, err := bind.DeployContract(node.auth, abi.ABI{}, init, node.client)
		if err != nil {
			t.Fatal(err)
		}
		mined := node.mined(t, tx)
		actual, err := node.client.CodeAtHash(node.ctx, address, mined.BlockHash)
		if err != nil || !reflect.DeepEqual(actual, runtime) {
			t.Fatalf("deployed surface code %x: %v", actual, err)
		}
		template := RPCDiscovery{Role: valueSidioraPool, Kind: valueSurface, RuntimeHash: factory.RuntimeHash, StagedRuntimeHash: factory.StagedRuntimeHash, StagedUpgrade: factory.StagedUpgrade}
		options := RPCOptions{Manifest: manifest, Discovery: []RPCDiscovery{template}}
		authority := &RPCQualification{client: node.client, options: options, manifestHash: "surface-deployment", privateFixture: true, reads: &rpcReadState{calls: make(chan struct{}, 16)}}
		epoch, err := authority.discoveryEpoch(options, address, valueSidioraPool, decode.Order{Block: mined.BlockNumber.Uint64()})
		if err != nil {
			t.Fatal(err)
		}
		want := factory
		want.Address = address.Hex()
		if epoch.Identity != want {
			t.Fatalf("discovered identity %+v, want %+v", epoch.Identity, want)
		}
		got, err := authority.validateCode(node.ctx, epoch, mined.BlockHash)
		if err != nil || got == "" {
			t.Fatalf("accepted runtime %d refused: %q %v", i, got, err)
		}
		base, err := authority.codeEpochDigest(epoch)
		if err != nil {
			t.Fatal(err)
		}
		wantDigest := accounting.HashTuple("rpc-surface-code-epoch-v1", base, crypto.Keccak256Hash(runtime).Hex())
		if got != wantDigest {
			t.Fatalf("runtime %d digest %q, want observed code binding %q", i, got, wantDigest)
		}
		cached, err := authority.validateCode(node.ctx, epoch, mined.BlockHash)
		if err != nil || cached != got {
			t.Fatalf("cached runtime digest %q: %v", cached, err)
		}
		digests = append(digests, got)
		changed := epoch
		changed.Identity.RuntimeHash = manifest.Roles["USDL"].RuntimeHash
		changed.Identity.StagedRuntimeHash = manifest.Roles["USDL"].RuntimeHash
		if _, err := authority.validateCode(node.ctx, changed, mined.BlockHash); !errors.Is(err, ErrQualification) {
			t.Fatalf("unaccepted pool code admitted: %v", err)
		}
		changed = epoch
		changed.Identity.StagedUpgrade = ""
		_, err = authority.validateCode(node.ctx, changed, mined.BlockHash)
		if i == 0 && err != nil || i == 1 && !errors.Is(err, ErrQualification) {
			t.Fatalf("runtime %d without staged upgrade: %v", i, err)
		}
		if i == 1 {
			for _, role := range []string{valueSidioraERC20, valueRouter} {
				changed = epoch
				changed.Deployment.Role = role
				if role == valueSidioraERC20 {
					changed.Identity.Kind = valueDirect
				}
				if _, err := authority.validateCode(node.ctx, changed, mined.BlockHash); !errors.Is(err, ErrQualification) {
					t.Fatalf("%s accepted a staged pool hash: %v", role, err)
				}
			}
		}
	}
	if digests[0] == digests[1] {
		t.Fatal("different accepted pool programs share a code epoch digest")
	}
}

func TestSurfacePoolRestoredIdentityMatchesDiscoveryTemplate(t *testing.T) {
	manifest := deployment.Current()
	ctx, client := realReceiptNode(t, "--number", strconv.FormatUint(manifest.ActivationHeight, 10))
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	options := receiptManifestOptions(t, manifest, BlockAnchor{Height: manifest.ActivationHeight, Hash: header.Hash()})
	var catalog decode.Catalog
	if err := json.Unmarshal(options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	factory := manifest.Roles["SidioraFactory"]
	template := RPCDiscovery{Role: valueSidioraPool, Kind: valueSurface, RuntimeHash: factory.RuntimeHash, StagedRuntimeHash: factory.StagedRuntimeHash, StagedUpgrade: factory.StagedUpgrade}
	for _, contract := range catalog.Contracts {
		if contract.Name == valueSidioraPool {
			template.ABIRevision = contract.ABIRevision
		}
	}
	options.Discovery = []RPCDiscovery{template}
	a := &RPCQualification{}
	epoch, err := a.discoveryEpoch(options, crypto.CreateAddress(common.HexToAddress(factory.Address), 1), valueSidioraPool, decode.Order{Block: manifest.ActivationHeight})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"matching", "staged hash", "staged upgrade"} {
		t.Run(mode, func(t *testing.T) {
			changed := epoch
			if mode == "staged hash" {
				changed.Identity.StagedRuntimeHash = manifest.Roles["USDL"].RuntimeHash
			}
			if mode == "staged upgrade" {
				changed.Identity.StagedUpgrade += "-other"
			}
			in := options
			in.Epochs = append(append([]RPCCodeEpoch(nil), options.Epochs...), changed)
			_, err := NewRPCQualificationWithAttestation(ctx, client, in, nil)
			if mode == "matching" && err != nil || mode != "matching" && !errors.Is(err, ErrQualification) {
				t.Fatalf("restored pool %s: %v", mode, err)
			}
		})
	}
}
