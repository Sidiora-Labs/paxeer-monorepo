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

func TestMakeBuildLinksConsensusVersion(t *testing.T) {
	output, err := makeDryRun(t, "build", "VERSION=9.9.9")
	if err != nil {
		t.Fatalf("make build dry run failed: %v\n%s", err, output)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "go build ") {
			if !strings.Contains(line, "-X github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion=9.9.9") {
				t.Errorf("go build command does not link the consensus version:\n%s", line)
			}
			return
		}
	}
	t.Fatalf("make build dry run did not print a go build command:\n%s", output)
}

func TestMakeInstallQuotesBuildTags(t *testing.T) {
	output, err := makeDryRun(t, "install", "BUILD_TAGS=alpha beta")
	if err != nil {
		t.Fatalf("make install dry run failed: %v\n%s", err, output)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "go install ") {
			if !strings.Contains(line, "-tags 'alpha beta'") {
				t.Errorf("go install command does not quote the build tags:\n%s", line)
			}
			return
		}
	}
	t.Fatalf("make install dry run did not print a go install command:\n%s", output)
}
