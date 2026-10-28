package appenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/chainconfig"
)

const ethereumRecordPath = "../tests/fixtures/records/ethereum.json"

func TestEnvNamesMatchTheBridgeAppConfig(t *testing.T) {
	gomod, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	modulePath := strings.TrimSpace(string(gomod))
	if modulePath == "" || modulePath == os.DevNull || !filepath.IsAbs(modulePath) {
		t.Fatalf("go env GOMOD returned no module path: %q", modulePath)
	}
	configPath := filepath.Join(filepath.Dir(modulePath), "apps/bridge/src/lib/bridge/config.ts")
	source, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	_, config, ok := strings.Cut(string(source), "export function readConfig(")
	if !ok {
		t.Fatalf("%s: readConfig function not found", configPath)
	}
	config, _, ok = strings.Cut(config, "export const bridgeConfig")
	if !ok {
		t.Fatalf("%s: bridgeConfig boundary not found", configPath)
	}
	reads := regexp.MustCompile(`\benv\.(NEXT_PUBLIC_BRIDGE_(?:VAULT_[A-Z0-9_]+|SOLANA_PROGRAM_ID))\b`).FindAllStringSubmatch(config, -1)
	if len(reads) == 0 {
		t.Fatalf("%s: no vault or Solana program environment reads found", configPath)
	}
	appNames := make(map[string]bool)
	for _, read := range reads {
		appNames[read[1]] = true
	}
	names := EnvNames()
	appenvNames := make(map[string]bool)
	for _, name := range names {
		if appenvNames[name] {
			t.Errorf("EnvNames repeats %q", name)
		}
		appenvNames[name] = true
	}
	var onlyAppenv, onlyApp []string
	for name := range appenvNames {
		if !appNames[name] {
			onlyAppenv = append(onlyAppenv, name)
		}
	}
	for name := range appNames {
		if !appenvNames[name] {
			onlyApp = append(onlyApp, name)
		}
	}
	sort.Strings(onlyAppenv)
	sort.Strings(onlyApp)
	if len(onlyAppenv) != 0 || len(onlyApp) != 0 {
		t.Errorf("environment names differ: only appenv: %q; only bridge app: %q", onlyAppenv, onlyApp)
	}

	var records []Record
	for _, chain := range chainconfig.Chains() {
		if chain.Kind != chainconfig.KindEVM {
			continue
		}
		path := recordCopy(t, map[string]any{"chain": chain.Name, "chain_id": chain.ID})
		record, err := LoadRecord(path)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	solana, err := LoadRecord("../tests/fixtures/records/solana.json")
	if err != nil {
		t.Fatal(err)
	}
	records = append(records, solana)
	var emittedNames []string
	for _, line := range EnvLines(records) {
		name, _, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("EnvLines emitted an invalid assignment: %q", line)
		}
		emittedNames = append(emittedNames, name)
	}
	if !reflect.DeepEqual(names, emittedNames) {
		t.Errorf("EnvNames() = %q, but EnvLines emits %q", names, emittedNames)
	}
}

func TestCheckReportsAVaultThatDiffers(t *testing.T) {
	record, err := LoadRecord(ethereumRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	raw, err := os.ReadFile(ethereumRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ethereum.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		env  string
	}{
		{"different", "NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=0x0000000000000000000000000000000000000001\n"},
		{"missing", "UNRELATED=value\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envPath := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(envPath, []byte(tc.env), 0600); err != nil {
				t.Fatal(err)
			}
			err := Check(envPath, []Record{record})
			if err == nil || !strings.Contains(err.Error(), "NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM") {
				t.Fatalf("Check must report the vault key: %v", err)
			}
			var output bytes.Buffer
			err = Run([]string{"--check", envPath, dir}, &output)
			if err == nil || !strings.Contains(err.Error(), "NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM") {
				t.Fatalf("Run --check must report the vault key: %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("Run --check wrote environment output: %q", output.String())
			}
		})
	}
}

func TestCheckAcceptsAMatchingEnv(t *testing.T) {
	var records []Record
	dir := t.TempDir()
	for _, name := range []string{"ethereum", "solana"} {
		path := filepath.Join("../tests/fixtures/records", name+".json")
		record, err := LoadRecord(path)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, vault := range []string{records[0].Vault, "0x" + strings.ToUpper(records[0].Vault[2:])} {
		t.Run(vault, func(t *testing.T) {
			envPath := filepath.Join(t.TempDir(), ".env")
			env := "# App environment\r\nUNRELATED=some=value\r\n\r\nNEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=" + vault + "\r\nNEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=" + records[1].ProgramID
			if err := os.WriteFile(envPath, []byte(env), 0600); err != nil {
				t.Fatal(err)
			}
			if err := Check(envPath, records); err != nil {
				t.Fatalf("Check rejected matching environment: %v", err)
			}
			var output bytes.Buffer
			if err := Run([]string{"--check", envPath, dir}, &output); err != nil {
				t.Fatalf("Run --check rejected matching environment: %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("Run --check wrote environment output: %q", output.String())
			}
		})
	}
	t.Run("program_id_case_is_significant", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env")
		env := "NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=" + records[0].Vault + "\nNEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=" + strings.ToLower(records[1].ProgramID) + "\n"
		if err := os.WriteFile(envPath, []byte(env), 0600); err != nil {
			t.Fatal(err)
		}
		err := Check(envPath, records)
		if err == nil || !strings.Contains(err.Error(), "NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID") {
			t.Fatalf("Check must reject a changed program ID: %v", err)
		}
	})
}

func recordCopy(t *testing.T, fields map[string]any) string {
	t.Helper()
	raw, err := os.ReadFile(ethereumRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for name, value := range fields {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		document[name] = encoded
	}
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvLinesFromEVMRecords(t *testing.T) {
	record, err := LoadRecord(ethereumRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	wantRecord := Record{
		Chain: "ethereum", ChainID: 1, Kind: "evm",
		Vault: "0x7a3e5c81b04d296f8e1a7c35d92b6f04e8c1a37d",
	}
	if record != wantRecord {
		t.Fatalf("LoadRecord() = %+v, want %+v", record, wantRecord)
	}
	want := []string{"NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=0x7a3e5c81b04d296f8e1a7c35d92b6f04e8c1a37d"}
	if got := EnvLines([]Record{record}); !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvLines(ethereum) = %q, want %q", got, want)
	}

	var records []Record
	want = nil
	for _, chain := range chainconfig.Chains() {
		if chain.Kind != chainconfig.KindEVM {
			continue
		}
		vault := fmt.Sprintf("0x%040x", chain.ID)
		path := recordCopy(t, map[string]any{"chain": chain.Name, "chain_id": chain.ID, "vault": vault})
		loaded, err := LoadRecord(path)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, loaded)
		want = append(want, "NEXT_PUBLIC_BRIDGE_VAULT_"+strings.ToUpper(chain.Name)+"="+vault)
	}
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	solana, err := LoadRecord("../tests/fixtures/records/solana.json")
	if err != nil {
		t.Fatal(err)
	}
	if solana.ProgramID != "A7SZbByPYuHpunZ9pyMDMrhMYvK44ANT1AVqb8U1FpM9" {
		t.Fatalf("LoadRecord(solana).ProgramID = %q", solana.ProgramID)
	}
	records = append(records, solana)
	want = append(want, "NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID="+solana.ProgramID)
	if got := EnvLines(records); !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvLines(reversed records) = %q, want registry order %q", got, want)
	}
	if got := EnvLines([]Record{records[0]}); len(got) != 1 || got[0] != want[len(want)-2] {
		t.Fatalf("EnvLines(subset) = %q, want %q", got, want[len(want)-2:len(want)-1])
	}
	if got := EnvLines(nil); len(got) != 0 {
		t.Fatalf("EnvLines(nil) = %q, want no variables", got)
	}
	if got := EnvLines([]Record{solana}); !reflect.DeepEqual(got, want[len(want)-1:]) {
		t.Fatalf("EnvLines(solana) = %q, want %q", got, want[len(want)-1:])
	}
}

func TestSolanaRecordEmitsTheProgramID(t *testing.T) {
	record, err := LoadRecord("../tests/fixtures/records/solana.json")
	if err != nil {
		t.Fatal(err)
	}
	const programID = "A7SZbByPYuHpunZ9pyMDMrhMYvK44ANT1AVqb8U1FpM9"
	wantRecord := Record{Chain: "solana", ChainID: 91600046870081, Kind: "solana", ProgramID: programID}
	if record != wantRecord {
		t.Fatalf("LoadRecord(solana) = %+v, want %+v", record, wantRecord)
	}
	want := []string{"NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=" + programID}
	if got := EnvLines([]Record{record}); !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvLines(solana) = %q, want %q", got, want)
	}
	evm, err := LoadRecord(ethereumRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	wantMixed := []string{"NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=" + evm.Vault, want[0]}
	for _, records := range [][]Record{{record, evm}, {evm, record}} {
		if got := EnvLines(records); !reflect.DeepEqual(got, wantMixed) {
			t.Fatalf("EnvLines(%+v) = %q, want EVM lines followed by Solana %q", records, got, wantMixed)
		}
	}

	for _, tc := range []struct {
		name      string
		programID string
	}{
		{"empty", ""},
		{"non_base58", "0OIl"},
		{"short", strings.Repeat("1", 31)},
		{"long", strings.Repeat("1", 33)},
		{"whitespace", programID + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := recordCopy(t, map[string]any{
				"chain": record.Chain, "chain_id": record.ChainID, "kind": record.Kind,
				"program_id": tc.programID,
			})
			invalid, err := LoadRecord(path)
			if err == nil {
				t.Fatalf("LoadRecord accepted program_id %q: %+v", tc.programID, invalid)
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "program_id") {
				t.Fatalf("error must name file %q and program_id: %v", path, err)
			}
			if invalid != (Record{}) {
				t.Fatalf("LoadRecord returned an invalid record: %+v", invalid)
			}
		})
	}
}

func TestRefusesARecordWithAnotherChainID(t *testing.T) {
	for _, id := range []uint64{2, 0, 8453} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			path := recordCopy(t, map[string]any{"chain_id": id})
			record, err := LoadRecord(path)
			if err == nil {
				t.Fatalf("LoadRecord accepted chain_id %d: %+v", id, record)
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "chain_id") {
				t.Fatalf("error must name file %q and chain_id: %v", path, err)
			}
			if record != (Record{}) {
				t.Fatalf("LoadRecord returned an invalid record: %+v", record)
			}
		})
	}
}

func TestRefusesAZeroVault(t *testing.T) {
	for _, tc := range []struct {
		name  string
		vault string
	}{
		{"zero", "0x0000000000000000000000000000000000000000"},
		{"empty", ""},
		{"short", "0x1234"},
		{"nineteen_bytes", "0x" + strings.Repeat("12", 19)},
		{"twenty_one_bytes", "0x" + strings.Repeat("12", 21)},
		{"non_hex", "0x" + strings.Repeat("g", 40)},
		{"missing_prefix", strings.Repeat("12", 20)},
		{"wrong_prefix", "zz" + strings.Repeat("12", 20)},
		{"whitespace", "0x7a3e5c81b04d296f8e1a7c35d92b6f04e8c1a37d\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := recordCopy(t, map[string]any{"vault": tc.vault})
			record, err := LoadRecord(path)
			if err == nil {
				t.Fatalf("LoadRecord accepted vault %q: %+v", tc.vault, record)
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "vault") {
				t.Fatalf("error must name file %q and vault: %v", path, err)
			}
			if record != (Record{}) {
				t.Fatalf("LoadRecord returned an invalid record: %+v", record)
			}
		})
	}
	for _, vault := range []string{
		"0x0000000000000000000000000000000000000001",
		"0x1000000000000000000000000000000000000000",
		"0x7A3e5c81b04D296F8E1a7C35D92b6f04e8C1a37D",
	} {
		t.Run("valid_"+vault, func(t *testing.T) {
			path := recordCopy(t, map[string]any{"vault": vault})
			record, err := LoadRecord(path)
			if err != nil {
				t.Fatalf("LoadRecord refused valid vault %q: %v", vault, err)
			}
			if record.Vault != vault {
				t.Fatalf("LoadRecord().Vault = %q, want %q", record.Vault, vault)
			}
		})
	}
}

func TestRunWritesEveryRecordInChainOrder(t *testing.T) {
	dir := t.TempDir()
	var want []string
	chains := chainconfig.Chains()
	for i, chain := range chains {
		if chain.Kind != chainconfig.KindEVM {
			continue
		}
		vault := fmt.Sprintf("0x%040x", chain.ID)
		path := recordCopy(t, map[string]any{"chain": chain.Name, "chain_id": chain.ID, "vault": vault})
		name := fmt.Sprintf("%02d-%s.json", len(chains)-i, chain.Name)
		if err := os.Rename(path, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
		want = append(want, "NEXT_PUBLIC_BRIDGE_VAULT_"+strings.ToUpper(chain.Name)+"="+vault)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a deployment record"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Run([]string{dir}, &output); err != nil {
		t.Fatal(err)
	}
	if expected := strings.Join(want, "\n") + "\n"; output.String() != expected {
		t.Fatalf("Run() output = %q, want %q in registry order", output.String(), expected)
	}

	t.Run("duplicate_chain", func(t *testing.T) {
		path := recordCopy(t, map[string]any{"vault": "0x0000000000000000000000000000000000000001"})
		duplicate := filepath.Join(dir, "zz-duplicate.json")
		if err := os.Rename(path, duplicate); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err := Run([]string{dir}, &output)
		if err == nil {
			t.Fatal("Run accepted two records for ethereum")
		}
		if !strings.Contains(err.Error(), "duplicate chain") || !strings.Contains(err.Error(), "ethereum") || !strings.Contains(err.Error(), duplicate) {
			t.Fatalf("error must name duplicate chain and file: %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("Run wrote partial output for duplicate records: %q", output.String())
		}
	})
}

func TestRunRefusesAnEmptyDirectory(t *testing.T) {
	for _, nonJSON := range []bool{false, true} {
		t.Run(fmt.Sprintf("non_json_files_%t", nonJSON), func(t *testing.T) {
			dir := t.TempDir()
			if nonJSON {
				if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not JSON"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, "nested.json"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			err := Run([]string{dir}, &output)
			if err == nil {
				t.Fatal("Run accepted a directory without JSON records")
			}
			if !strings.Contains(err.Error(), dir) {
				t.Fatalf("error must name directory %q: %v", dir, err)
			}
			if output.Len() != 0 {
				t.Fatalf("Run wrote output for an empty directory: %q", output.String())
			}
		})
	}
}
