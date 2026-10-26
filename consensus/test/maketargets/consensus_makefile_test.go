package maketargets_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func makeDryRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve consensus directory: %v", err)
	}
	output, err := exec.CommandContext(t.Context(), "make", append([]string{"-n", "-C", dir}, args...)...).CombinedOutput()
	return string(output), err
}

func TestMakeAllDryRunResolves(t *testing.T) {
	output, err := makeDryRun(t, "all")
	if err != nil {
		t.Fatalf("make all dry run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "go build ") {
		t.Errorf("make all dry run did not print a go build command:\n%s", output)
	}
	if !strings.Contains(output, "go install ") {
		t.Errorf("make all dry run did not print a go install command:\n%s", output)
	}
}
