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
func runM965Case(t *testing.T, name string) {
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
	command := exec.CommandContext(ctx, python, "-B", "-m", "unittest", "-v", "repair_m965_cases.RepairM965."+name)
	command.Dir = filepath.Dir(source)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("real Python proof %s: %v\n%s", name, err, output)
	}
	t.Logf("%s", output)
}

func TestRepairM_965_PythonCapability(t *testing.T) {
	runM965Case(t, "test_python_capability")
}

func TestRepairM_965_LocalCommandDoc(t *testing.T) {
	runM965Case(t, "test_local_command_doc")
}

func TestRepairM_965_LocalManifestCommand(t *testing.T) {
	runM965Case(t, "test_local_manifest_command")
}

func TestRepairM_965_V1Contract(t *testing.T) {
	runM965Case(t, "test_v1_contract")
}

func TestRepairM_965_V2Contract(t *testing.T) {
	runM965Case(t, "test_v2_contract")
}

func TestRepairM_965_RouteCatalog(t *testing.T) {
	runM965Case(t, "test_route_catalog")
}

func TestRepairM_965_FinalAdmission(t *testing.T) {
	runM965Case(t, "test_final_admission")
}

func TestRepairM_965_Phase4Qualification(t *testing.T) {
	runM965Case(t, "test_phase4_qualification")
}
