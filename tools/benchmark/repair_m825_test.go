package benchmark_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairM_825_BenchmarkReadmeCommand(t *testing.T) {
	doc, err := os.ReadFile("scenarios/README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, usage, ok := strings.Cut(string(doc), "## Usage\n")
	if !ok {
		t.Fatal("scenario README has no Usage section")
	}
	usage, _, _ = strings.Cut(usage, "\n## ")
	preamble, block, ok := strings.Cut(usage, "```bash\n")
	if !ok {
		t.Fatal("Usage has no bash example")
	}
	if !strings.Contains(preamble, "repository root") || !strings.Contains(preamble, "working directory") {
		t.Fatal("Usage must state the repository-root working directory before the examples")
	}
	block, _, ok = strings.Cut(block, "```")
	if !ok {
		t.Fatal("Usage bash example is not closed")
	}
	var commands []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}
	want := []string{
		"./tools/benchmark/benchmark.sh",
		"BENCHMARK_CONFIG=tools/benchmark/scenarios/erc20.json ./tools/benchmark/benchmark.sh",
		"BENCHMARK_CONFIG=tools/benchmark/scenarios/mixed.json ./tools/benchmark/benchmark.sh",
	}
	if len(commands) != len(want) {
		t.Fatalf("Usage has %d commands, want %d: %q", len(commands), len(want), commands)
	}
	root, err := os.OpenRoot(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	launcher, err := os.Stat("benchmark.sh")
	if err != nil {
		t.Fatal(err)
	}
	for i, command := range commands {
		if command != want[i] {
			t.Fatalf("Usage command %d = %q, want %q", i, command, want[i])
		}
		fields := strings.Fields(command)
		path := strings.TrimPrefix(fields[len(fields)-1], "./")
		info, err := root.Stat(path)
		if err != nil {
			t.Fatalf("documented command %q cannot resolve from the repository root: %v", command, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || !os.SameFile(launcher, info) {
			t.Fatalf("documented command %q does not resolve to the real executable benchmark launcher", command)
		}
		if len(fields) == 2 {
			config := strings.TrimPrefix(fields[0], "BENCHMARK_CONFIG=")
			if data, err := root.ReadFile(config); err != nil || len(data) == 0 {
				t.Fatalf("documented scenario %q is not readable from the repository root: %v", config, err)
			}
		}
	}
}

func TestRepairM_825_BenchmarkReadmeDeploymentAndChainIDs(t *testing.T) {
	doc, err := os.ReadFile("scenarios/README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, explanation := range []string{
		"`LoadConfig` always replaces `chainID` and `paxChainID` with the running chain's values",
		"sets `mockDeploy` to `true`",
		"deployment ownership with the in-process benchmark generator",
		"contract scenarios still submit deployment transactions and wait for successful receipts before generating load",
		"refuses to start on a [live EVM chain ID]",
		"including mainnet chain ID 125 (`0x7d`)",
	} {
		if !strings.Contains(string(doc), explanation) {
			t.Errorf("scenario README lost explanation %q", explanation)
		}
	}
}
