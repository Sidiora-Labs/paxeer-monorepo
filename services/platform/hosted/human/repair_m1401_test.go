package human

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The implementation under repair is Python/shell. This adapter invokes its
// real focused cases; it does not compile a second toolchain or contact a service.
func repairM1401(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-B", "repair_m1401_cases.py", name)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("real production case %s: %v\n%s", name, err, output.String())
	}
	t.Log(output.String())
}

func TestRepairM_1401_HumanDocCustody(t *testing.T) {
	repairM1401(t, "HumanDocCustody")
}

func TestRepairM_1401_HistoryFinalityMaterial(t *testing.T) {
	repairM1401(t, "HistoryFinalityMaterial")
}

func TestRepairM_1401_MaterialShellTrustHistory(t *testing.T) {
	repairM1401(t, "MaterialShellTrustHistory")
}

func TestRepairM_1401_NativeManifestRetiredInput(t *testing.T) {
	repairM1401(t, "NativeManifestRetiredInput")
}

func TestRepairM_1401_OwnerKmsPrepareBounds(t *testing.T) {
	repairM1401(t, "OwnerKmsPrepareBounds")
}

func TestRepairM_1401_ProvisionMovementSource(t *testing.T) {
	repairM1401(t, "ProvisionMovementSource")
}

func TestRepairM_1401_ProvisionRetiredDaemon(t *testing.T) {
	repairM1401(t, "ProvisionRetiredDaemon")
}

func TestRepairM_1401_MaterialTestRetiredNode(t *testing.T) {
	repairM1401(t, "MaterialTestRetiredNode")
}
