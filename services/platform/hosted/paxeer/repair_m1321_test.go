package paxeer

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRepairM_1321_CompatibilityRefusals(t *testing.T) {
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "LAYERX_PAXEER_") {
			environment = append(environment, entry)
		}
	}
	for _, argument := range []string{
		"--custody-genesis", "--custody-genesis=/retired/genesis.json",
		"--deposit-root", "--deposit-root=retired",
		"--sequencer-authorizations", "--sequencer-authorizations=/retired/leaders.json",
	} {
		t.Run(argument, func(t *testing.T) {
			command := exec.Command("bash", "init-chain.sh", argument)
			command.Env = environment
			output, err := command.CombinedOutput()
			if err == nil || command.ProcessState.ExitCode() != 2 {
				t.Fatalf("retired input %s was not refused before native provisioning: %v, %s", argument, err, output)
			}
			if string(output) != "init-chain: relay genesis inputs are retired; use native bank and ledger funding via ../node/bootstrap.sh\n" {
				t.Fatalf("unexpected retired-input diagnostic: %q", output)
			}
		})
	}
	t.Run("legacy environment", func(t *testing.T) {
		command := exec.Command("bash", "init-chain.sh")
		command.Env = append(append([]string{}, environment...), "LAYERX_PAXEER_CHAIN_ID=125")
		output, err := command.CombinedOutput()
		if err == nil || command.ProcessState.ExitCode() != 2 {
			t.Fatalf("legacy environment was not refused before native provisioning: %v, %s", err, output)
		}
		if string(output) != "init-chain: legacy LAYERX_PAXEER configuration is retired; use ../node/bootstrap.sh with authoritative native genesis inputs\n" {
			t.Fatalf("unexpected legacy-environment diagnostic: %q", output)
		}
	})
}
