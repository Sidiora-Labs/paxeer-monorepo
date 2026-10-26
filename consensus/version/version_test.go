package version_test

import (
	"os/exec"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
)

func TestTMVersionSetByLinkFlag(t *testing.T) {
	const linkedVersion = "9.9.9-linker-test"
	cmd := exec.Command("go", "run", "-ldflags",
		"-X github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion="+linkedVersion,
		"./testdata/printversion")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run with link flag failed: %v\n%s", err, output)
	}
	if got, want := string(output), linkedVersion+"\n"; got != want {
		t.Fatalf("linked TMVersion = %q, want %q", got, want)
	}
}

func TestTMVersionDefaultsWithoutLinkFlag(t *testing.T) {
	if version.TMVersionDefault != "0.35.0-unreleased" {
		t.Fatalf("TMVersionDefault = %q, want %q", version.TMVersionDefault, "0.35.0-unreleased")
	}
	cmd := exec.Command("go", "run", "./testdata/printversion")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run without link flag failed: %v\n%s", err, output)
	}
	if got, want := string(output), version.TMVersionDefault+"\n"; got != want {
		t.Fatalf("default TMVersion = %q, want %q", got, want)
	}
}
