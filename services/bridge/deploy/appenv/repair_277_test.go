package appenv

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readySolanaRecord(t *testing.T) Record {
	t.Helper()
	raw, err := os.ReadFile("../tests/fixtures/records/solana.json")
	if err != nil {
		t.Fatal(err)
	}
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	record.Version, record.Phase = SolanaRecordVersion, SolanaPhaseReady
	return record
}

func readySolanaRecordPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "solana.json")
	if err := WriteSolanaRecord(path, readySolanaRecord(t)); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRepair_277_DeploymentPhasesToApp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "solana.json")
	envPath := filepath.Join(t.TempDir(), "app.env")
	var environments []map[string]string
	for _, phase := range []string{SolanaPhaseDeployed, SolanaPhaseInitialized, SolanaPhaseReady} {
		record := readySolanaRecord(t)
		record.Phase = phase
		if phase != SolanaPhaseReady {
			record.Assets = nil
		}
		if phase == SolanaPhaseDeployed {
			record.Owner, record.Threshold, record.Attestors = "", 0, nil
		}
		if err := WriteSolanaRecord(path, record); err != nil {
			t.Fatal(err)
		}
		loaded, loadErr := LoadRecord(path)
		var output bytes.Buffer
		runErr := Run([]string{dir}, &output)
		if phase != SolanaPhaseReady {
			if loadErr == nil || runErr == nil || !reflect.DeepEqual(loaded, Record{}) || output.Len() != 0 {
				t.Fatalf("phase %s advertised readiness: record %+v, load %v, run %v, output %q", phase, loaded, loadErr, runErr, output.String())
			}
			if !strings.Contains(loadErr.Error(), path) || !strings.Contains(loadErr.Error(), "phase") {
				t.Fatalf("missing phase diagnostic: %v", loadErr)
			}
			if lines := EnvLines([]Record{record}); len(lines) != 0 {
				t.Fatalf("direct EnvLines exported %s: %q", phase, lines)
			}
		} else {
			if loadErr != nil || runErr != nil || !reflect.DeepEqual(loaded, record) {
				t.Fatalf("ready record rejected: %+v, %v, %v", loaded, loadErr, runErr)
			}
			want := "NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=" + record.ProgramID + "\n"
			if output.String() != want || strings.Join(EnvLines([]Record{loaded}), "\n")+"\n" != want {
				t.Fatalf("ready output %q, want %q", output.String(), want)
			}
		}
		if err := os.WriteFile(envPath, output.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		checkErr := Check(envPath, []Record{record})
		var checked bytes.Buffer
		runCheckErr := Run([]string{"--check", envPath, dir}, &checked)
		if (checkErr == nil) != (phase == SolanaPhaseReady) || (runCheckErr == nil) != (phase == SolanaPhaseReady) || checked.Len() != 0 {
			t.Fatalf("phase %s check results: %v, %v, stdout %q", phase, checkErr, runCheckErr, checked.String())
		}
		env := map[string]string{"NEXT_PUBLIC_BRIDGE_API_URL": "https://attestation.invalid"}
		for _, line := range strings.Split(output.String(), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok {
				env[key] = value
			}
		}
		environments = append(environments, env)
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(environments)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "node", "repair_277_app.mjs", root)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("real app configuration: %v\n%s", err, stderr.String())
	}
	var results []struct {
		Program string `json:"program"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(output, &results); err != nil {
		t.Fatalf("app output %q: %v", output, err)
	}
	if len(results) != 3 {
		t.Fatalf("app returned %d results", len(results))
	}
	for i, result := range results {
		if i < 2 {
			if result.Program != "" || result.State != "awaiting-backend" {
				t.Fatalf("checkpoint configured the app: %+v", result)
			}
		} else if result.Program != readySolanaRecord(t).ProgramID || result.State != "configured" {
			t.Fatalf("ready record did not configure the app: %+v", result)
		}
	}
}

func TestRepair_277_RejectsUnverifiedReadiness(t *testing.T) {
	cases := []struct {
		name, field string
		corrupt     func(*Record)
	}{
		{"unversioned", "version", func(r *Record) { r.Version = 0 }},
		{"future_version", "version", func(r *Record) { r.Version++ }},
		{"missing_phase", "phase", func(r *Record) { r.Phase = "" }},
		{"unknown_phase", "phase", func(r *Record) { r.Phase = "registered" }},
		{"deployed", "phase", func(r *Record) { r.Phase = SolanaPhaseDeployed }},
		{"initialized", "phase", func(r *Record) { r.Phase = SolanaPhaseInitialized }},
		{"zero_program", "program_id", func(r *Record) { r.ProgramID = strings.Repeat("1", 32) }},
		{"missing_owner", "owner", func(r *Record) { r.Owner = "" }},
		{"placeholder_owner", "owner", func(r *Record) { r.Owner = "PLACEHOLDER:owner" }},
		{"zero_owner", "owner", func(r *Record) { r.Owner = strings.Repeat("1", 32) }},
		{"no_attestors", "attestors", func(r *Record) { r.Attestors = nil }},
		{"too_many_attestors", "attestors", func(r *Record) { r.Attestors = make([]string, 65) }},
		{"zero_attestor", "attestors", func(r *Record) { r.Attestors[0] = "0x" + strings.Repeat("0", 40) }},
		{"placeholder_attestor", "attestors", func(r *Record) { r.Attestors[0] = "PLACEHOLDER:attestor" }},
		{"duplicate_attestors", "attestors", func(r *Record) { r.Attestors[1] = "0x" + strings.ToUpper(r.Attestors[0][2:]) }},
		{"unsorted_attestors", "attestors", func(r *Record) { r.Attestors[0], r.Attestors[1] = r.Attestors[1], r.Attestors[0] }},
		{"zero_threshold", "threshold", func(r *Record) { r.Threshold = 0 }},
		{"unreachable_threshold", "threshold", func(r *Record) { r.Threshold = uint32(len(r.Attestors) + 1) }},
		{"no_assets", "assets", func(r *Record) { r.Assets = nil }},
		{"missing_sid", "assets", func(r *Record) { r.Assets = r.Assets[:1] }},
		{"missing_sol", "assets", func(r *Record) { r.Assets = r.Assets[1:] }},
		{"duplicate_asset", "assets", func(r *Record) { r.Assets = append(r.Assets, r.Assets[0]) }},
		{"wrong_mint", "mint", func(r *Record) { r.Assets[0].Mint = "not-a-key" }},
		{"wrong_asset_id", "asset_id", func(r *Record) { r.Assets[0].AssetID = r.Assets[1].AssetID }},
		{"wrong_sid_id", "asset_id", func(r *Record) { r.Assets[1].AssetID = r.Assets[0].AssetID }},
		{"wrong_sol_decimals", "assets", func(r *Record) { r.Assets[0].Decimals = 6 }},
		{"wrong_sid_decimals", "assets", func(r *Record) { r.Assets[1].Decimals = 9 }},
		{"zero_cap", "per_tx_cap", func(r *Record) { r.Assets[0].PerTxCap = "0" }},
		{"malformed_cap", "per_tx_cap", func(r *Record) { r.Assets[0].PerTxCap = "1e6" }},
		{"inverted_caps", "total_cap", func(r *Record) { r.Assets[0].TotalCap = "1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := readySolanaRecord(t)
			tc.corrupt(&record)
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "solana.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			ethereum, err := os.ReadFile(ethereumRecordPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "a-ethereum.json"), ethereum, 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadRecord(path)
			if err == nil || !reflect.DeepEqual(loaded, Record{}) || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("invalid readiness accepted or wrong diagnostic: %+v, %v", loaded, err)
			}
			var output bytes.Buffer
			if err := Run([]string{dir}, &output); err == nil || output.Len() != 0 {
				t.Fatalf("invalid deployment exported: %v, %q", err, output.String())
			}
			if lines := EnvLines([]Record{record}); len(lines) != 0 {
				t.Fatalf("direct EnvLines exported invalid record: %q", lines)
			}
			envPath := filepath.Join(t.TempDir(), "app.env")
			if err := os.WriteFile(envPath, []byte("NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID="+record.ProgramID+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := Check(envPath, []Record{record}); err == nil {
				t.Fatal("Check accepted invalid readiness")
			}
		})
	}
}

func TestRepair_277_AtomicCheckpoint(t *testing.T) {
	path := readySolanaRecordPath(t)
	old, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	record := readySolanaRecord(t)
	record.Phase, record.Assets = SolanaPhaseInitialized, nil
	if err := WriteSolanaRecord(path, record); err != nil {
		t.Fatal(err)
	}
	oldInfo, err := old.Stat()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(oldInfo, info) {
		t.Fatal("checkpoint was rewritten in place")
	}
	retained := make([]byte, len(before))
	if n, err := old.Read(retained); err != nil || n != len(before) || !bytes.Equal(before, retained) {
		t.Fatalf("open reader saw partial checkpoint: %d, %v", n, err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("checkpoint permissions %v", info.Mode())
	}
	if _, err := LoadRecord(path); err == nil {
		t.Fatal("reopened checkpoint became ready before assets were verified")
	}
	checkpoint, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = SolanaPhaseReady
	if err := WriteSolanaRecord(path, record); err == nil {
		t.Fatal("writer published ready without assets")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, checkpoint) {
		t.Fatalf("failed validation changed checkpoint: %v", err)
	}
	if err := WriteSolanaRecord(path, readySolanaRecord(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRecord(path); err != nil {
		t.Fatalf("reopened ready record: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("checkpoint leaked temporary files: %v, %v", entries, err)
	}
	// Force rename failure after the temporary file has been flushed.
	blocked := filepath.Join(filepath.Dir(path), "blocked")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := WriteSolanaRecord(blocked, readySolanaRecord(t)); err == nil {
		t.Fatal("writer replaced a directory")
	}
	entries, err = os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed rename leaked temporary files: %v, %v", entries, err)
	}
}

func TestRepair_277_LegacyProducerCheckpoint(t *testing.T) {
	source, err := os.ReadFile("../deploy-solana-program.sh")
	if err != nil {
		t.Fatal(err)
	}
	_, branch, ok := strings.Cut(string(source), "case $configured_program_id in")
	if !ok {
		t.Fatal("real producer placeholder branch not found")
	}
	start := strings.Index(branch, "jq -n --arg chain solana")
	end := strings.Index(branch, `> "$work/record.json"`)
	if start < 0 || end < start {
		t.Fatal("real producer checkpoint writer not found")
	}
	command := branch[start : end+len(`> "$work/record.json"`)]
	raw, err := os.ReadFile("../tests/fixtures/records/solana.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "record.json")
	cmd := exec.CommandContext(t.Context(), "bash", "-eu", "-c", command)
	cmd.Env = append(os.Environ(), "work="+dir, "REPO_ROOT=", "config=contracts/bridge/solana/chains/solana/config.json", "program_keypair="+filepath.Join(dir, "program-keypair.json"))
	for variable, field := range map[string]string{
		"chain_id": "chain_id", "genesis_hash": "genesis_hash", "program_id": "program_id", "program_data": "program_data_account", "UPGRADEABLE_LOADER": "upgradeable_loader_id", "elf_sha256": "program_elf_sha256", "elf_bytes": "program_elf_bytes", "deployment_signature": "deployment_signature", "deployed_slot": "deployment_slot", "rooted_slot": "rooted_slot", "publisher": "publisher", "authority": "upgrade_authority", "commitment": "commitment", "vault_authority": "vault_authority", "vault_handle": "vault_handle",
	} {
		var text string
		if err := json.Unmarshal(fixture[field], &text); err != nil {
			text = string(fixture[field])
		}
		cmd.Env = append(cmd.Env, variable+"="+text)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real checkpoint producer: %v\n%s", err, output)
	}
	var produced Record
	checkpoint, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(checkpoint, &produced); err != nil {
		t.Fatal(err)
	}
	if produced.ProgramID != readySolanaRecord(t).ProgramID || produced.Owner != "" {
		t.Fatalf("wrong producer branch exercised: %+v", produced)
	}
	if _, err := LoadRecord(path); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("unversioned producer checkpoint accepted: %v", err)
	}
	var output bytes.Buffer
	if err := Run([]string{dir}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("producer checkpoint exported: %v, %q", err, output.String())
	}
}
