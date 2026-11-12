package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func privateIdentityFixture() PrivateFixtureManifest {
	manifest := deployment.Current()
	manifest.DeploymentVersion = "private-fixture-local"
	manifest.SourceManifestSHA256 = strings.Repeat("a", 64)
	manifest.ActivationHeight = 1
	manifest.Status = "active"
	manifest.Governance = deployment.Governance{Mode: "timelock-handoff", ExpectedBeaconOwner: fmt.Sprintf("0x%040x", 90), TimelockDelaySeconds: 172800}
	manifest.Roles = make(map[string]deployment.Role)
	creation := make(map[string]string)
	for i, name := range fixtureRoles {
		address := fmt.Sprintf("0x%040x", i+1)
		implementation := fmt.Sprintf("0x%040x", i+20)
		kind := "erc1967"
		if name == valueUSDL {
			kind = "direct"
		}
		if name == valuePoolBeacon {
			kind = "beacon"
		}
		role := deployment.Role{Address: address, Kind: kind, RuntimeHash: "0x" + strings.Repeat("b", 64)}
		creation[address] = "0x" + strings.Repeat("c", 64)
		if kind != "direct" {
			role.ImplementationAddress = implementation
			role.ImplementationHash = "0x" + strings.Repeat("d", 64)
			creation[implementation] = "0x" + strings.Repeat("e", 64)
		}
		manifest.Roles[name] = role
	}
	return PrivateFixtureManifest{SchemaVersion: 1, Realm: valuePrivateFixture, Deployment: manifest, RPCURL: "http://127.0.0.1:8545", GenesisHash: "0x" + strings.Repeat("f", 64), CreationTransactions: creation}
}

func TestPrivateFixtureIdentityFromBoundFile(t *testing.T) {
	proof := privateIdentityFixture()
	if err := proof.validateIdentity(); err != nil {
		t.Fatalf("complete private deployment: %v", err)
	}
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "private-manifest.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	env := BaseEnv{DeploymentRealm: valuePrivateFixture, RPCURL: proof.RPCURL, PrivateFixtureManifestFile: path, PrivateFixtureManifestSHA256: hex.EncodeToString(hash[:])}
	actual, err := env.PrivateFixtureEvidence()
	if err != nil || actual.Deployment.DeploymentVersion != proof.Deployment.DeploymentVersion || len(actual.CreationTransactions) != len(proof.CreationTransactions) {
		t.Fatalf("bound private identity: %+v, %v", actual, err)
	}
	manifest, err := env.PrivateFixtureDeployment()
	if err != nil || manifest.DeploymentVersion != proof.Deployment.DeploymentVersion {
		t.Fatalf("bound private deployment: %+v, %v", manifest, err)
	}
	mutations := []struct {
		name string
		edit func(*PrivateFixtureManifest)
		want string
	}{
		{"deployment", func(p *PrivateFixtureManifest) { p.Deployment.ActivationHeight = 0 }, "private_fixture_deployment_incomplete"},
		{"asset", func(p *PrivateFixtureManifest) { p.Deployment.Native.Decimals = 17 }, "private_fixture_asset_policy_invalid"},
		{"governance", func(p *PrivateFixtureManifest) { p.Deployment.Governance.TimelockDelaySeconds = 1 }, "private_fixture_governance_policy_invalid"},
		{"role inventory", func(p *PrivateFixtureManifest) { p.CreationTransactions = nil }, "private_fixture_role_inventory_invalid"},
		{"role missing", func(p *PrivateFixtureManifest) {
			p.Deployment.Roles["Other"] = p.Deployment.Roles[fixtureRoles[0]]
			delete(p.Deployment.Roles, fixtureRoles[0])
		}, "private_fixture_role_missing"},
		{"role kind", func(p *PrivateFixtureManifest) {
			r := p.Deployment.Roles[valueUSDL]
			r.Kind = "beacon"
			p.Deployment.Roles[valueUSDL] = r
		}, "private_fixture_role_kind_invalid"},
		{"role identity", func(p *PrivateFixtureManifest) {
			r := p.Deployment.Roles[fixtureRoles[0]]
			r.Address = "0x0"
			p.Deployment.Roles[fixtureRoles[0]] = r
		}, "private_fixture_role_identity_invalid"},
		{"implementation", func(p *PrivateFixtureManifest) {
			r := p.Deployment.Roles[fixtureRoles[0]]
			r.ImplementationAddress = ""
			p.Deployment.Roles[fixtureRoles[0]] = r
		}, "private_fixture_implementation_identity_missing"},
		{"direct identity", func(p *PrivateFixtureManifest) {
			r := p.Deployment.Roles[valueUSDL]
			r.ImplementationAddress = fmt.Sprintf("0x%040x", 80)
			p.Deployment.Roles[valueUSDL] = r
		}, "private_fixture_direct_identity_conflict"},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			var changed PrivateFixtureManifest
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			tc.edit(&changed)
			if err := changed.validateIdentity(); err == nil || err.Error() != tc.want {
				t.Fatalf("changed private identity: %v, want %s", err, tc.want)
			}
		})
	}
}
