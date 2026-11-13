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
func runOwnerMaterialCase(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-B", "owner_material_cases.py", name)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("real production case %s: %v\n%s", name, err, output.String())
	}
	t.Log(output.String())
}

func TestHumanDocCustody(t *testing.T) {
	runOwnerMaterialCase(t, "HumanDocCustody")
}

func TestHistoryFinalityMaterial(t *testing.T) {
	runOwnerMaterialCase(t, "HistoryFinalityMaterial")
}

func TestMaterialShellTrustHistory(t *testing.T) {
	runOwnerMaterialCase(t, "MaterialShellTrustHistory")
}

func TestNativeManifestRetiredInput(t *testing.T) {
	runOwnerMaterialCase(t, "NativeManifestRetiredInput")
}

func TestOwnerKmsPrepareBounds(t *testing.T) {
	runOwnerMaterialCase(t, "OwnerKmsPrepareBounds")
}

func TestProvisionMovementSource(t *testing.T) {
	runOwnerMaterialCase(t, "ProvisionMovementSource")
}

func TestProvisionRetiredDaemon(t *testing.T) {
	runOwnerMaterialCase(t, "ProvisionRetiredDaemon")
}

func TestMaterialTestRetiredNode(t *testing.T) {
	runOwnerMaterialCase(t, "MaterialTestRetiredNode")
}
