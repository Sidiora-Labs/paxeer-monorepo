// Proves: appenv.Record, LoadRecord, EnvNames, EnvLines, Run -> deployment JSON and paxeer-bridge-app-env stdout -> appenv.Check and the CLI --check consumer.
// The test is the first external Go consumer of all six exported interfaces and
// launches the real command for both CLI interfaces. Every env file comes from
// the producer's output; each observation uses fresh calls and child processes,
// and rewriting the records proves that neither side reuses cached results.
// Repairs services/bridge/deploy/appenv/appenv.go: Check previously stopped at
// the first difference; it must report every differing or missing variable as
// promised by contracts/bridge-deploy.md#app-env. The proof also corrupts real
// producer output to assert refusal at LoadRecord and Check's decoding boundaries.
package appenv_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/appenv"
)

func TestProofAppEnv(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "paxeer-bridge-app-env")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../cmd/paxeer-bridge-app-env")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real app env command: %v\n%s", err, output)
	}
	dir := t.TempDir()
	envPath := filepath.Join(t.TempDir(), "app.env")
	identities := []struct {
		chain string
		id    uint64
		key   string
	}{
		{"ethereum", 1, "NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM"},
		{"base", 8453, "NEXT_PUBLIC_BRIDGE_VAULT_BASE"},
		{"arbitrum", 42161, "NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM"},
		{"optimism", 10, "NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM"},
		{"bnb", 56, "NEXT_PUBLIC_BRIDGE_VAULT_BNB"},
		{"polygon", 137, "NEXT_PUBLIC_BRIDGE_VAULT_POLYGON"},
		{"avalanche", 43114, "NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE"},
		{"hyperevm", 999, "NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM"},
		{"solana", 91600046870081, "NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID"},
	}
	var names []string
	for _, identity := range identities {
		names = append(names, identity.key)
	}
	if got := appenv.EnvNames(); !reflect.DeepEqual(got, names) {
		t.Fatalf("EnvNames() = %q, want %q", got, names)
	}

	assertCheckRefusal := func(t *testing.T, records []appenv.Record, diagnostics ...string) {
		t.Helper()
		checkErr := appenv.Check(envPath, records)
		var output bytes.Buffer
		runErr := appenv.Run([]string{"--check", envPath, dir}, &output)
		if output.Len() != 0 {
			t.Fatalf("refused Run --check emitted stdout: %q", output.String())
		}
		stdout, stderr := proofAppEnvCommand(t, binary, 1, "--check", envPath, dir)
		if stdout != "" {
			t.Fatalf("refused CLI --check emitted stdout: %q", stdout)
		}
		for _, err := range []error{checkErr, runErr} {
			if err == nil {
				t.Fatal("consumer accepted invalid environment")
			}
			for _, diagnostic := range append([]string{envPath}, diagnostics...) {
				if !strings.Contains(err.Error(), diagnostic) {
					t.Fatalf("refusal %q does not name %q", err, diagnostic)
				}
			}
		}
		for _, diagnostic := range append([]string{"paxeer-bridge-app-env:", envPath}, diagnostics...) {
			if !strings.Contains(stderr, diagnostic) {
				t.Fatalf("CLI refusal %q does not name %q", stderr, diagnostic)
			}
		}
	}

	var records []appenv.Record
	var paths []string
	var produced string
	for generation := 0; generation < 2; generation++ {
		records = nil
		paths = nil
		var wantLines []string
		for i, identity := range identities {
			record := appenv.Record{Chain: identity.chain, ChainID: identity.id, Kind: "evm"}
			value := fmt.Sprintf("0x%040x", identity.id+uint64(generation+1)*100000)
			if identity.chain == "solana" {
				raw, err := os.ReadFile("../tests/fixtures/records/solana.json")
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &record); err != nil {
					t.Fatal(err)
				}
				record.Version = appenv.SolanaRecordVersion
				record.Phase = appenv.SolanaPhaseReady
				programs := []string{"A7SZbByPYuHpunZ9pyMDMrhMYvK44ANT1AVqb8U1FpM9", "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"}
				value = programs[generation]
				record.ProgramID = value
			} else {
				record.Vault = value
			}
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			// Reverse file order independently of the registry's output order.
			path := filepath.Join(dir, fmt.Sprintf("%02d-%s.json", len(identities)-i, identity.chain))
			if record.Kind == "solana" {
				if err := appenv.WriteSolanaRecord(path, record); err != nil {
					t.Fatal(err)
				}
			} else {
				proofAppEnvWrite(t, path, raw)
			}
			loaded, err := appenv.LoadRecord(path)
			if err != nil || !reflect.DeepEqual(loaded, record) {
				t.Fatalf("LoadRecord(%s) = %+v, %v; want %+v", path, loaded, err, record)
			}
			records = append(records, loaded)
			paths = append(paths, path)
			wantLines = append(wantLines, identity.key+"="+value)
		}
		reversed := append([]appenv.Record(nil), records...)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		if got := appenv.EnvLines(reversed); !reflect.DeepEqual(got, wantLines) {
			t.Fatalf("EnvLines() = %q, want %q in registry order", got, wantLines)
		}
		if generation > 0 {
			// The previous real output is now stale for every record, including Solana.
			assertCheckRefusal(t, records, names...)
		}
		want := strings.Join(wantLines, "\n") + "\n"
		for observation := 0; observation < 2; observation++ {
			var output bytes.Buffer
			if err := appenv.Run([]string{dir}, &output); err != nil || output.String() != want {
				t.Fatalf("Run generation %d observation %d = %q, %v; want %q", generation, observation, output.String(), err, want)
			}
			stdout, stderr := proofAppEnvCommand(t, binary, 0, dir)
			if stdout != want || stderr != "" {
				t.Fatalf("CLI generation %d observation %d = stdout %q, stderr %q; want %q", generation, observation, stdout, stderr, want)
			}
			produced = stdout
			proofAppEnvWrite(t, envPath, []byte(stdout))
			if err := appenv.Check(envPath, records); err != nil {
				t.Fatalf("Check refused real command output: %v", err)
			}
			output.Reset()
			if err := appenv.Run([]string{"--check", envPath, dir}, &output); err != nil || output.Len() != 0 {
				t.Fatalf("Run --check = %v, stdout %q", err, output.String())
			}
			stdout, stderr = proofAppEnvCommand(t, binary, 0, "--check", envPath, dir)
			if stdout != "" || stderr != "" {
				t.Fatalf("matching CLI --check emitted stdout %q, stderr %q", stdout, stderr)
			}
		}
	}

	t.Run("malformed_env_at_consumer_decode", func(t *testing.T) {
		proofAppEnvWrite(t, envPath, []byte(strings.Replace(produced, "=", "", 1)))
		assertCheckRefusal(t, records, ":1:", "expected KEY=VALUE")
	})
	t.Run("all_missing_variables_reported", func(t *testing.T) {
		lines := strings.SplitAfter(produced, "\n")
		proofAppEnvWrite(t, envPath, []byte(strings.Join(lines[2:], "")))
		assertCheckRefusal(t, records, "missing "+names[0], "missing "+names[1])
	})
	proofAppEnvWrite(t, envPath, []byte(produced))

	for _, tc := range []struct {
		name    string
		index   int
		field   string
		corrupt func(*appenv.Record)
	}{
		{"wrong_chain_id", 0, "chain_id", func(r *appenv.Record) { r.ChainID++ }},
		{"unknown_chain", 0, "chain", func(r *appenv.Record) { r.Chain = "unregistered" }},
		{"malformed_vault", 0, "vault", func(r *appenv.Record) { r.Vault = "0xmalformed" }},
		{"malformed_program_id", 8, "program_id", func(r *appenv.Record) { r.ProgramID += "0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := records[tc.index]
			tc.corrupt(&record)
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			path := paths[tc.index]
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { proofAppEnvWrite(t, path, original) })
			proofAppEnvWrite(t, path, raw)
			loaded, err := appenv.LoadRecord(path)
			if err == nil || !reflect.DeepEqual(loaded, appenv.Record{}) || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("LoadRecord must refuse %s without a partial record: %+v, %v", tc.field, loaded, err)
			}
			for _, args := range [][]string{{dir}, {"--check", envPath, dir}} {
				var output bytes.Buffer
				err := appenv.Run(args, &output)
				if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.field) || output.Len() != 0 {
					t.Fatalf("Run(%q) must refuse %s without stdout: %v, %q", args, tc.field, err, output.String())
				}
				stdout, stderr := proofAppEnvCommand(t, binary, 1, args...)
				if stdout != "" || !strings.Contains(stderr, path) || !strings.Contains(stderr, tc.field) {
					t.Fatalf("CLI(%q) must refuse %s without stdout: %q, %q", args, tc.field, stdout, stderr)
				}
			}
		})
	}
}

func proofAppEnvWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func proofAppEnvCommand(t *testing.T, binary string, wantExit int, args ...string) (string, string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	gotExit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("launch real command: %v", err)
		}
		gotExit = exitErr.ExitCode()
	}
	if gotExit != wantExit {
		t.Fatalf("CLI(%q) exit = %d, want %d; stdout %q, stderr %q", args, gotExit, wantExit, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}
