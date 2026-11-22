package bindings_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestABIInventoryIncludesNativeAndRetainedBindings(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(t.TempDir(), "inventory.json")
	command := exec.Command("python3", filepath.Join(root, "sdk/launchpad/protocol/bootstrap/abi_baseline.py"),
		"--contracts", filepath.Join(root, "contracts/launchpad"), "--output", reportPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("read ABI inventory: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		ChainID  int `json:"chain_id"`
		Bindings []struct {
			Contract        string
			Source          string
			CurrentAddress  string   `json:"current_address"`
			EmbeddedABIHash string   `json:"embedded_abi_sha256"`
			CurrentABIHash  string   `json:"current_abi_sha256"`
			Missing         []string `json:"missing_signatures"`
			Removed         []string `json:"removed_signatures"`
			Changed         []string `json:"changed_schemas"`
		}
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"EventEmitter": "artifact", "PoolBeacon": "artifact", "NativeLaunchpad": "native-abi",
		"FeeAccumulator": "role-abi", "FeesRouter": "role-abi", "PoolRegistry": "role-abi",
		"ProtocolConfig": "role-abi", "Quoter": "role-abi", "Router": "role-abi",
		"SidioraFactory": "role-abi", "SidioraNFT": "role-abi",
	}
	if report.ChainID != 125 || len(report.Bindings) != len(want) {
		t.Fatal("ABI inventory differs", report)
	}
	for _, binding := range report.Bindings {
		if source, ok := want[binding.Contract]; !ok || source != binding.Source {
			t.Fatalf("binding source differs: %+v", binding)
		}
		delete(want, binding.Contract)
		if binding.EmbeddedABIHash == "" || binding.EmbeddedABIHash != binding.CurrentABIHash || len(binding.Missing) != 0 || len(binding.Removed) != 0 || len(binding.Changed) != 0 {
			t.Errorf("binding differs from its producer: %+v", binding)
		}
		if binding.Contract == "NativeLaunchpad" && binding.CurrentAddress != "0x0000000000000000000000000000000000001017" {
			t.Fatal("native binding address differs", binding.CurrentAddress)
		}
	}
	if len(want) != 0 {
		t.Fatal("binding inventory is incomplete", want)
	}
}
