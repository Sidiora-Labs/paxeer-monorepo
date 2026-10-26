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

func TestMakeBuildPrivvalServerDryRun(t *testing.T) {
	output, err := makeDryRun(t, "build_privval_server")
	if err != nil {
		t.Fatalf("make build_privval_server dry run failed: %v\n%s", err, output)
	}
	var buildLine string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "go build ") {
			buildLine = line
			break
		}
	}
	if buildLine == "" {
		t.Fatalf("make build_privval_server dry run did not print a go build command:\n%s", output)
	}
	var foundTarget bool
	for _, field := range strings.Fields(buildLine) {
		if field == "-i" {
			t.Errorf("go build command contains the removed -i flag:\n%s", buildLine)
		}
		if field == "./cmd/priv_val_server/..." {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Errorf("go build command does not target ./cmd/priv_val_server/...:\n%s", buildLine)
	}
}
