package receipt

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func assertQualificationRefusal(t *testing.T, err error, role, kind string, address common.Address, cause string) {
	t.Helper()
	if !errors.Is(err, ErrQualification) {
		t.Fatalf("qualification refusal: %v", err)
	}
	want := "role=" + role + " kind=" + kind + " address=" + strings.ToLower(address.Hex()) + " cause=" + cause
	if !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("refusal = %q, want suffix %q", err.Error(), want)
	}
}

func TestCodeQualificationRefusalCauses(t *testing.T) {
	ctx, client := realReceiptNode(t)
	address := common.HexToAddress("0x0125")
	implementation := common.HexToAddress("0x0126")
	beacon := common.HexToAddress("0x0127")
	code := common.FromHex("0x60006000f3")
	setCode := func(address common.Address, code []byte) {
		t.Helper()
		var result any
		if err := client.Client().CallContext(ctx, &result, "anvil_setCode", address, hexutil.Encode(code)); err != nil {
			t.Fatal(err)
		}
	}
	setStorage := func(slot common.Hash, value common.Address) {
		t.Helper()
		var result any
		if err := client.Client().CallContext(ctx, &result, "anvil_setStorageAt", address, slot, common.BytesToHash(value.Bytes())); err != nil {
			t.Fatal(err)
		}
	}
	a := &RPCQualification{client: client, privateFixture: true, manifestHash: "qualified", reads: &rpcReadState{calls: make(chan struct{}, 16)}}
	original := RPCCodeEpoch{Deployment: decode.Deployment{Address: address, Role: valueSidioraPool}, Identity: deployment.Role{Address: address.Hex(), Kind: valueDirect, RuntimeHash: crypto.Keccak256Hash(code).Hex()}}
	for _, tc := range []struct{ name, cause string }{
		{"absent", "code_absent"}, {"oversized", "code_size"}, {"hash", "runtime_hash"},
		{"implementation slot", "implementation"}, {"implementation code", "implementation"},
		{"beacon slot", "beacon"}, {"beacon code", "beacon"}, {"unknown kind", "identity"},
		{"precompile identity", "identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setCode(address, code)
			setStorage(deployment.ImplementationSlot, common.Address{})
			setStorage(deployment.BeaconSlot, common.Address{})
			epoch := original
			switch tc.name {
			case "absent":
				setCode(address, nil)
			case "oversized":
				setCode(address, make([]byte, (128<<10)+1))
			case "hash":
				epoch.Identity.RuntimeHash = common.HexToHash("0x01").Hex()
			case "implementation slot", "implementation code":
				epoch.Identity.Kind = valueErc1967
				epoch.Identity.ImplementationAddress = implementation.Hex()
				epoch.Identity.ImplementationHash = original.Identity.RuntimeHash
				if tc.name == "implementation code" {
					setStorage(deployment.ImplementationSlot, implementation)
				}
			case "beacon slot", "beacon code":
				epoch.Identity.Kind = valueBeaconProxy
				epoch.Identity.BeaconAddress = beacon.Hex()
				epoch.Identity.ImplementationAddress = implementation.Hex()
				epoch.Identity.ImplementationHash = original.Identity.RuntimeHash
				a.options.Manifest.Roles = map[string]deployment.Role{"PoolBeacon": {Address: beacon.Hex(), Kind: valueBeacon, RuntimeHash: original.Identity.RuntimeHash}}
				if tc.name == "beacon code" {
					setStorage(deployment.BeaconSlot, beacon)
				}
			case "unknown kind":
				epoch.Identity.Kind = "unsupported"
			case "precompile identity":
				epoch.Identity.Kind = valuePrecompile
			}
			var mined any
			if err := client.Client().CallContext(ctx, &mined, "evm_mine"); err != nil {
				t.Fatal(err)
			}
			header, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = a.validateCodeUncached(ctx, epoch, header.Hash())
			assertQualificationRefusal(t, err, epoch.Deployment.Role, epoch.Identity.Kind, address, tc.cause)
		})
	}
	setCode(address, code)
	var mined any
	if err := client.Client().CallContext(ctx, &mined, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if epoch, err := a.validateCodeUncached(ctx, original, header.Hash()); err != nil || epoch == "" {
		t.Fatalf("matching direct runtime: %q %v", epoch, err)
	}
}

func TestDiscoveryQualificationRefusalCauses(t *testing.T) {
	address := common.HexToAddress("0x0125")
	template := RPCDiscovery{Role: valueSidioraPool, Kind: valueSurface, RuntimeHash: common.HexToHash("0x01").Hex()}
	a := &RPCQualification{options: RPCOptions{Discovery: []RPCDiscovery{template}}}
	for _, tc := range []struct {
		name, kind string
		templates  []RPCDiscovery
	}{
		{"missing", "unknown", nil},
		{"duplicate", valueSurface, []RPCDiscovery{template, template}},
		{"kind", "unsupported", []RPCDiscovery{{Role: valueSidioraPool, Kind: "unsupported"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owned := a.options
			owned.Discovery = tc.templates
			_, err := a.discoveryEpoch(owned, address, valueSidioraPool, decode.Order{})
			assertQualificationRefusal(t, err, valueSidioraPool, tc.kind, address, "template")
		})
	}
	epoch, err := a.discoveryEpoch(a.options, address, valueSidioraPool, decode.Order{Block: 10})
	if err != nil {
		t.Fatal(err)
	}
	a.options.Epochs = []RPCCodeEpoch{epoch}
	if _, err = a.epoch(address, decode.Order{Block: 9}); err == nil {
		t.Fatal("epoch before activation admitted")
	} else {
		assertQualificationRefusal(t, err, valueSidioraPool, valueSurface, address, "coverage")
	}
	if got, err := a.epoch(address, decode.Order{Block: 10}); err != nil || got.Identity != epoch.Identity {
		t.Fatalf("active identity: %+v %v", got, err)
	}
}

func TestQualificationRefusalOmitsUnstructuredLabels(t *testing.T) {
	address := common.HexToAddress("0x0125")
	err := qualificationRefusal("unstructured\nrole", "unstructured\nresponse", address, "identity")
	assertQualificationRefusal(t, err, "unknown", "unknown", address, "identity")
}

func TestConfiguredCodeQualificationRefusalCauses(t *testing.T) {
	ctx, client := realReceiptNode(t)
	manifest := deployment.Current()
	options := receiptManifestOptions(t, manifest, BlockAnchor{Height: manifest.ActivationHeight, Hash: common.HexToHash("0x01")})
	for _, tc := range []struct{ name, cause string }{{"identity", "identity"}, {"template", "template"}, {"coverage", "coverage"}} {
		t.Run(tc.name, func(t *testing.T) {
			owned, err := clone(options)
			if err != nil {
				t.Fatal(err)
			}
			epoch := owned.Epochs[0]
			address, role, kind := epoch.Deployment.Address, epoch.Deployment.Role, epoch.Identity.Kind
			switch tc.name {
			case "identity":
				owned.Epochs[0].Identity.Address = common.HexToAddress("0xbad").Hex()
			case "template":
				owned.Discovery = []RPCDiscovery{{Role: valueSidioraPool, Kind: valueSurface}}
				address, role, kind = common.Address{}, valueSidioraPool, valueSurface
			case "coverage":
				owned.Epochs = owned.Epochs[1:]
			}
			_, err = newRPCQualification(ctx, client, owned, "qualified", false, nil)
			assertQualificationRefusal(t, err, role, kind, address, tc.cause)
		})
	}
}
