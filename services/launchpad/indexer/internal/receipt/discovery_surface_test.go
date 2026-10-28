package receipt

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestDiscoveryEpochSurfacePool(t *testing.T) {
	manifest := deployment.Current()
	runtime := common.FromHex("0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3")
	template := RPCDiscovery{Role: "SidioraPool", Kind: "surface", ABIRevision: strings.Repeat("a", 64), RuntimeHash: crypto.Keccak256Hash(runtime).Hex()}
	options := RPCOptions{Manifest: manifest, Discovery: []RPCDiscovery{template}}
	authority := &RPCQualification{options: options}
	address := common.HexToAddress("0x0000000000000000000000000000000000000677")
	from := decode.Order{Block: manifest.ActivationHeight + 1, Transaction: 2, Log: 3}
	epoch, err := authority.discoveryEpoch(options, address, "SidioraPool", from)
	if err != nil {
		t.Fatal(err)
	}
	want := deployment.Role{Address: address.Hex(), Kind: "surface", RuntimeHash: template.RuntimeHash}
	if epoch.Identity != want {
		t.Fatalf("surface identity: got %+v, want %+v", epoch.Identity, want)
	}
	if epoch.Identity.BeaconAddress != "" || epoch.Identity.ImplementationAddress != "" || epoch.Identity.ImplementationHash != "" {
		t.Fatalf("surface identity carries beacon or implementation fields: %+v", epoch.Identity)
	}
	if !validRPCIdentity(epoch.Identity) {
		t.Fatalf("surface identity rejected: %+v", epoch.Identity)
	}
	wantDeployment := decode.Deployment{Address: address, Role: template.Role, ABIRevision: template.ABIRevision, From: from}
	if !reflect.DeepEqual(epoch.Deployment, wantDeployment) {
		t.Fatalf("surface deployment: got %+v, want %+v", epoch.Deployment, wantDeployment)
	}
	options.Discovery[0].Kind = "unsupported"
	if _, err := authority.discoveryEpoch(options, address, "SidioraPool", from); err != ErrQualification {
		t.Fatalf("unsupported discovery kind: got %v, want ErrQualification", err)
	}
}

func TestDiscoveryEpochBeaconPoolLegacy(t *testing.T) {
	manifest := deployment.Current()
	beacon := manifest.Roles["PoolBeacon"]
	if beacon.Address == "" || beacon.ImplementationAddress == "" || beacon.ImplementationHash == "" {
		t.Fatalf("manifest beacon identity incomplete: %+v", beacon)
	}
	for _, kind := range []string{"", "beacon_proxy"} {
		t.Run("kind="+kind, func(t *testing.T) {
			template := RPCDiscovery{Role: "SidioraPool", Kind: kind, ABIRevision: strings.Repeat("b", 64), RuntimeHash: beacon.RuntimeHash}
			options := RPCOptions{Manifest: manifest, Discovery: []RPCDiscovery{template}}
			authority := &RPCQualification{options: options}
			address := common.HexToAddress("0x0000000000000000000000000000000000000678")
			from := decode.Order{Block: manifest.ActivationHeight + 1, Transaction: 4, Log: 5}
			epoch, err := authority.discoveryEpoch(options, address, "SidioraPool", from)
			if err != nil {
				t.Fatal(err)
			}
			want := deployment.Role{Address: address.Hex(), Kind: "beacon_proxy", RuntimeHash: template.RuntimeHash, BeaconAddress: beacon.Address, ImplementationAddress: beacon.ImplementationAddress, ImplementationHash: beacon.ImplementationHash}
			if epoch.Identity != want {
				t.Fatalf("legacy identity: got %+v, want %+v", epoch.Identity, want)
			}
			if !validRPCIdentity(epoch.Identity) {
				t.Fatalf("legacy identity rejected: %+v", epoch.Identity)
			}
			wantDeployment := decode.Deployment{Address: address, Role: template.Role, ABIRevision: template.ABIRevision, From: from}
			if !reflect.DeepEqual(epoch.Deployment, wantDeployment) {
				t.Fatalf("legacy deployment: got %+v, want %+v", epoch.Deployment, wantDeployment)
			}
		})
	}
}
