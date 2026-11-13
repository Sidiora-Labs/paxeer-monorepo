package load

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Task step 15 permits the qualification environment's Python interpreter to
// execute the repository's production modules without external services.
func runContractAdmissionCase(t *testing.T, name string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatalf("required Python interpreter unavailable: %v", err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("repair proof source location unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, "-B", "-m", "unittest", "-v", "contract_admission_cases.ContractAdmissionTests."+name)
	command.Dir = filepath.Dir(source)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("real Python proof %s: %v\n%s", name, err, output)
	}
	t.Logf("%s", output)
}

func TestContractAdmissionPythonCapability(t *testing.T) {
	runContractAdmissionCase(t, "test_python_capability")
}

func TestContractAdmissionLocalCommandDoc(t *testing.T) {
	runContractAdmissionCase(t, "test_local_command_doc")
}

func TestContractAdmissionLocalManifestCommand(t *testing.T) {
	runContractAdmissionCase(t, "test_local_manifest_command")
}

func TestContractAdmissionV1Contract(t *testing.T) {
	runContractAdmissionCase(t, "test_v1_contract")
}

func TestContractAdmissionV2Contract(t *testing.T) {
	runContractAdmissionCase(t, "test_v2_contract")
}

func TestContractAdmissionRouteCatalog(t *testing.T) {
	runContractAdmissionCase(t, "test_route_catalog")
}

func TestContractAdmissionFinalAdmission(t *testing.T) {
	runContractAdmissionCase(t, "test_final_admission")
}

func TestContractAdmissionPhase4Qualification(t *testing.T) {
	runContractAdmissionCase(t, "test_phase4_qualification")
}
