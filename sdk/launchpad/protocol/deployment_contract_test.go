package protocol

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestClientDeploymentCarriesCurrentRoleIdentities(t *testing.T) {
	raw, err := os.ReadFile("contracts/v1/chain/client-deployment.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		SchemaVersion        int
		Core                 map[string]any
		OptionalCapabilities map[string]bool
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("node", "-e", `const path = require('node:path');
const generator = require('./contracts/v1/chain/client-deployment.cjs');
process.stdout.write(JSON.stringify(generator.deriveDeployment(path.resolve('../../../contracts/launchpad'))));`)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("derive client deployment: %v\n%s", err, output)
	}
	var expected map[string]any
	if err := json.Unmarshal(output, &expected); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || !reflect.DeepEqual(document.Core, expected) {
		t.Fatalf("client deployment differs from the current role identities:\n got %#v\nwant %#v", document.Core, expected)
	}
	if !reflect.DeepEqual(document.OptionalCapabilities, map[string]bool{"metaAg": false, "pecorOrders": false}) {
		t.Fatal("optional contract capabilities differ", document.OptionalCapabilities)
	}
}

func TestActiveAddressesMatchDeploymentDocument(t *testing.T) {
	raw, err := os.ReadFile("../shared/config/deployment/current.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ChainID           int64
		DeploymentVersion string
		Roles             map[string]struct{ Address string }
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if ActiveAddresses.ChainID != manifest.ChainID || manifest.DeploymentVersion != "paxeer125-"+ActiveAddresses.ActiveFromUpgrade {
		t.Fatal("active deployment identity differs from the manifest")
	}
	if len(ActiveAddresses.Roles)+1 != len(manifest.Roles) {
		t.Fatal("active role inventory differs from the manifest")
	}
	for name, role := range manifest.Roles {
		address := common.HexToAddress(role.Address)
		if name == "LaunchpadPrecompile" {
			if ActiveAddresses.Precompile != address || ActiveAddresses.Router != address || common.HexToAddress(LaunchpadPrecompileHex) != address {
				t.Fatal("native routing address differs from the manifest")
			}
			continue
		}
		if ActiveAddresses.Roles[name] != address {
			t.Errorf("%s address differs from the manifest", name)
		}
	}
}
