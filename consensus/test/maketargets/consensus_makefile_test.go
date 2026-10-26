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

func TestMakeFormatUsesModuleLocalPrefix(t *testing.T) {
	output, err := makeDryRun(t, "format")
	if err != nil {
		t.Fatalf("make format dry run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "find . -name '*.go' -type f -not -path \"*.git*\" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s\n") {
		t.Errorf("make format dry run did not preserve the gofmt command:\n%s", output)
	}
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "| xargs goimports ") {
			continue
		}
		if !strings.Contains(line, "-not -name '*.pb.go' -not -name '*pb_test.go'") {
			t.Errorf("goimports command did not preserve generated protobuf exclusions:\n%s", line)
		}
		fields := strings.Fields(line)
		localFlags := 0
		for i, field := range fields {
			if field == "-local" {
				localFlags++
				if i+1 >= len(fields) || fields[i+1] != "github.com/Sidiora-Labs/Paxeer-X-Network" {
					t.Errorf("goimports command does not use the fork module local prefix:\n%s", line)
				}
			} else if strings.HasPrefix(field, "-local=") {
				localFlags++
				t.Errorf("goimports command must use -local github.com/Sidiora-Labs/Paxeer-X-Network:\n%s", line)
			}
		}
		if localFlags != 1 {
			t.Errorf("goimports command has %d -local flags, want exactly one:\n%s", localFlags, line)
		}
		return
	}
	t.Fatalf("make format dry run did not print a goimports command:\n%s", output)
}

func TestMakeRetiredContractTestTargets(t *testing.T) {
	for _, target := range []string{"lint", "build-contract-tests-hooks", "contract-tests"} {
		t.Run(target, func(t *testing.T) {
			output, err := makeDryRun(t, target)
			if err == nil {
				t.Fatalf("make %s dry run succeeded for a retired target:\n%s", target, output)
			}
			if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == 0 {
				t.Fatalf("make %s did not exit with a non-zero status: %v\n%s", target, err, output)
			}
			if !strings.Contains(output, "No rule to make target '"+target+"'") {
				t.Errorf("make %s did not report a missing target:\n%s", target, output)
			}
		})
	}
	output, err := makeDryRun(t, "build")
	if err != nil {
		t.Fatalf("make build dry run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "go build ") {
		t.Errorf("make build dry run did not print a go build command:\n%s", output)
	}
}
