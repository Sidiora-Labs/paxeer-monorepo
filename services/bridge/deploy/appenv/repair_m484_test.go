package appenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	"golang.org/x/crypto/sha3"
)

const repairM484Fixtures = "testdata/repair_m484"

// These transcripts were captured by executing the record-writing commands
// verbatim from the deployment scripts, with the committed deployment inputs.
// The script writer digest binds each recording to the production producer.
func repairM484Transcript(t *testing.T, name string) ([]byte, Record) {
	t.Helper()
	var transcript struct {
		Script       string          `json:"script"`
		WriterIndex  int             `json:"writer_index"`
		WriterSHA256 string          `json:"writer_sha256"`
		Record       json.RawMessage `json:"record"`
	}
	raw, err := os.ReadFile(filepath.Join(repairM484Fixtures, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &transcript); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("../../../..", transcript.Script))
	if err != nil {
		t.Fatal(err)
	}
	writers := regexp.MustCompile(`(?s)jq -n --arg chain .*?> "\$work/record.json"`).FindAll(source, -1)
	if transcript.WriterIndex < 0 || transcript.WriterIndex >= len(writers) {
		t.Fatalf("%s has no recorded writer %d", transcript.Script, transcript.WriterIndex)
	}
	digest := sha256.Sum256(writers[transcript.WriterIndex])
	if hex.EncodeToString(digest[:]) != transcript.WriterSHA256 {
		t.Fatalf("%s writer changed: recapture its real output", transcript.Script)
	}
	var record Record
	if err := json.Unmarshal(transcript.Record, &record); err != nil {
		t.Fatal(err)
	}
	if record.Configuration == "" || record.ChainID == 0 || record.Kind == "" {
		t.Fatalf("incomplete producer transcript: %+v", record)
	}
	return transcript.Record, record
}

// Goja executes readConfig and its Solana helpers from the application source.
// The address modules are unmodified viem 2.41.2 CommonJS sources. UTF-8 and
// Keccak are native runtime primitives backed by Go's real codecs/crypto;
// address validation, checksumming, caches and configuration run in JavaScript.
func repairM484ReadConfig(t *testing.T, environment []byte) map[string]any {
	t.Helper()
	var provenance struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}
	rawProvenance, err := os.ReadFile(filepath.Join(repairM484Fixtures, "viem/provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawProvenance, &provenance); err != nil {
		t.Fatal(err)
	}
	packageJSON, err := os.ReadFile("../../../../apps/bridge/package.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(packageJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Dependencies["viem"] != provenance.Version || len(provenance.Files) == 0 {
		t.Fatal("recorded viem sources must match the app's pinned dependency")
	}
	vm := goja.New()
	vm.SetParserOptions(parser.WithSourceMapLoader(func(name string) ([]byte, error) {
		sourceMap, err := os.ReadFile(filepath.Join(repairM484Fixtures, "viem", name))
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(sourceMap)
		if hex.EncodeToString(digest[:]) != provenance.Files[name] {
			return nil, fmt.Errorf("recorded upstream source map %s changed", name)
		}
		return sourceMap, nil
	}))
	modules := make(map[string]*goja.Object)
	var load func(string) *goja.Object
	load = func(name string) *goja.Object {
		if exports, ok := modules[name]; ok {
			return exports
		}
		exports := vm.NewObject()
		modules[name] = exports
		switch name {
		case "_cjs/utils/encoding/toBytes.js":
			if err := exports.Set("stringToBytes", func(call goja.FunctionCall) goja.Value {
				options := call.Argument(1)
				if !goja.IsUndefined(options) && !goja.IsNull(options) {
					panic(vm.NewTypeError("deployment address encoding takes no size options"))
				}
				return vm.ToValue([]byte(call.Argument(0).String()))
			}); err != nil {
				t.Fatal(err)
			}
			return exports
		case "_cjs/utils/hash/keccak256.js":
			if err := exports.Set("keccak256", func(value goja.Value, to string) goja.Value {
				var data []byte
				if err := vm.ExportTo(value, &data); err != nil {
					panic(vm.NewTypeError("Keccak input: %s", err))
				}
				hash := sha3.NewLegacyKeccak256()
				if _, err := hash.Write(data); err != nil {
					panic(vm.NewGoError(err))
				}
				if to == "bytes" {
					return vm.ToValue(hash.Sum(nil))
				}
				return vm.ToValue("0x" + hex.EncodeToString(hash.Sum(nil)))
			}); err != nil {
				t.Fatal(err)
			}
			return exports
		}
		source, err := os.ReadFile(filepath.Join(repairM484Fixtures, "viem", name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(source)
		if hex.EncodeToString(digest[:]) != provenance.Files[name] {
			t.Fatalf("recorded upstream module %s changed", name)
		}
		wrapper, err := vm.RunScript(name, "(function(require, exports) {\n"+string(source)+"\n})")
		if err != nil {
			t.Fatalf("load real viem source %s: %v", name, err)
		}
		fn, ok := goja.AssertFunction(wrapper)
		if !ok {
			t.Fatalf("%s is not a CommonJS module", name)
		}
		require := func(dependency string) *goja.Object {
			return load(path.Clean(path.Join(path.Dir(name), dependency)))
		}
		if _, err := fn(goja.Undefined(), vm.ToValue(require), exports); err != nil {
			t.Fatalf("initialize real viem module %s: %v", name, err)
		}
		return exports
	}
	getAddress := load("_cjs/utils/address/getAddress.js").Get("getAddress")
	isAddress := load("_cjs/utils/address/isAddress.js").Get("isAddress")
	for name, value := range map[string]any{
		"getAddress": getAddress, "isAddress": isAddress,
		"zeroAddress":        "0x0000000000000000000000000000000000000000",
		"emittedEnvironment": string(environment),
	} {
		if err := vm.Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	// Only erase explicit TypeScript type syntax; all executable statements in
	// these production functions are evaluated unchanged. Missing declarations
	// or unsupported syntax fail the proof instead of substituting an algorithm.
	solana, err := os.ReadFile("../../../../apps/bridge/src/lib/bridge/solana.ts")
	if err != nil {
		t.Fatal(err)
	}
	constants, _, ok := strings.Cut(string(solana), "const RECIPIENT_BYTES")
	if !ok {
		t.Fatal("Solana base58 constants not found")
	}
	constants = constants[strings.Index(constants, "const BASE58_ALPHABET"):]
	decode := repairM484Function(t, string(solana), "export function base58Decode(")
	validate := repairM484Function(t, string(solana), "export function isSolanaAddress(")
	solanaJS := constants + "\n" + decode + "\n" + validate
	solanaJS = strings.NewReplacer(
		"export function ", "function ", "input: string", "input",
		"): Uint8Array | undefined {", ") {", "): boolean {", ") {",
		"const bytes: number[]", "const bytes",
	).Replace(solanaJS)
	if _, err := vm.RunScript("solana.ts", solanaJS); err != nil {
		t.Fatalf("execute real Solana address validation: %v", err)
	}
	config, err := os.ReadFile("../../../../apps/bridge/src/lib/bridge/config.ts")
	if err != nil {
		t.Fatal(err)
	}
	configJS := regexp.MustCompile(`(?m)^import [^\n]*\n`).ReplaceAllString(string(config), "")
	configJS = strings.NewReplacer(
		"export type BridgeEnvironment = Record<string, string | undefined>;", "",
		"value: string | undefined", "value", "env: BridgeEnvironment", "env",
		"): string | undefined {", ") {", "): Address | undefined {", ") {",
		"export function ", "function ", "export const ", "const ", " as const", "",
	).Replace(configJS)
	if _, err := vm.RunString("const process = { env: JSON.parse(emittedEnvironment) };"); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunScript("config.ts", configJS); err != nil {
		t.Fatalf("execute production readConfig: %v", err)
	}
	value, err := vm.RunString(`JSON.stringify({ direct: readConfig(process.env), boot: bridgeConfig, environment: process.env })`)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(value.String()), &result); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result["direct"], result["boot"]) {
		t.Fatalf("fresh application boot differs from readConfig: %v", result)
	}
	var original map[string]any
	if err := json.Unmarshal(environment, &original); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, result["environment"]) {
		t.Fatal("readConfig mutated the emitted environment")
	}
	return result["direct"].(map[string]any)
}

func repairM484Function(t *testing.T, source, declaration string) string {
	t.Helper()
	start := strings.Index(source, declaration)
	if start < 0 {
		t.Fatalf("production function %s not found", declaration)
	}
	opening := strings.Index(source[start:], "{") + start
	depth := 0
	for i := opening; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}
	t.Fatalf("unterminated production function %s", declaration)
	return ""
}

func repairM484AppEnvironment(t *testing.T, output []byte) []byte {
	t.Helper()
	env := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || value == "" {
			t.Fatalf("malformed real environment output: %q", line)
		}
		if _, duplicate := env[key]; duplicate {
			t.Fatalf("duplicate emitted key %s", key)
		}
		env[key] = value
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repairM484Write(t *testing.T, destination string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(destination, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_484_ScriptOutputToReadConfigAfterRestart(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(t.TempDir(), "app.env")
	evmRaw, evm := repairM484Transcript(t, "evm-ready")
	solanaRaw, solana := repairM484Transcript(t, "solana-registered")
	if evm.Vault == "" || evm.Owner == "" || solana.Owner == "" || len(solana.Assets) != 2 {
		t.Fatal("transcripts did not capture completed script writers")
	}
	repairM484Write(t, filepath.Join(dir, "ethereum.json"), evmRaw)
	solanaPath := filepath.Join(dir, "solana.json")
	repairM484Write(t, solanaPath, solanaRaw)
	if _, err := LoadRecord(solanaPath); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("legacy script output must require explicit versioned migration: %v", err)
	}
	// The script migration is owned by domain 14 (repair_m484.kvx). The real
	// versioned producer publishes the script's verified custody/asset values.
	solana.Version, solana.Phase = SolanaRecordVersion, SolanaPhaseReady
	if err := WriteSolanaRecord(solanaPath, solana); err != nil {
		t.Fatal(err)
	}
	var first bytes.Buffer
	if err := Run([]string{dir}, &first); err != nil {
		t.Fatal(err)
	}
	want := "NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=" + evm.Vault + "\nNEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=" + solana.ProgramID + "\n"
	if first.String() != want {
		t.Fatalf("producer-to-env transport = %q, want %q", first.String(), want)
	}
	repairM484Write(t, envPath, first.Bytes())
	initial := repairM484ReadConfig(t, repairM484AppEnvironment(t, first.Bytes()))
	if !strings.EqualFold(initial["vaults"].(map[string]any)["ethereum"].(string), evm.Vault) || initial["solanaProgramId"] != solana.ProgramID {
		t.Fatalf("readConfig lost the scripts' deployment identities: %v", initial)
	}
	// All file writers and the first JavaScript runtime have stopped. Reopen
	// the records and saved transport, then boot an independent app runtime.
	reopened, err := LoadRecord(solanaPath)
	if err != nil || !reflect.DeepEqual(reopened, solana) {
		t.Fatalf("recovered record = %+v, %v, want %+v", reopened, err, solana)
	}
	var restarted bytes.Buffer
	if err := Run([]string{dir}, &restarted); err != nil || restarted.String() != first.String() {
		t.Fatalf("restarted exporter = %q, %v", restarted.String(), err)
	}
	var checked bytes.Buffer
	if err := Run([]string{"--check", envPath, dir}, &checked); err != nil || checked.Len() != 0 {
		t.Fatalf("reopened transport failed real check: %v, %q", err, checked.String())
	}
	saved, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if recovered := repairM484ReadConfig(t, repairM484AppEnvironment(t, saved)); !reflect.DeepEqual(recovered, initial) {
		t.Fatalf("restarted app differs: %v, want %v", recovered, initial)
	}
	// A new checkpoint must invalidate the saved environment, including after
	// reopening; a fresh app boot must consume the newly emitted value.
	solana.ProgramID = solana.Owner
	if err := WriteSolanaRecord(solanaPath, solana); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"--check", envPath, dir}, &checked); err == nil || !strings.Contains(err.Error(), solanaProgramIDEnvName) {
		t.Fatalf("stale saved environment accepted after record replacement: %v", err)
	}
	var updated bytes.Buffer
	if err := Run([]string{dir}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.String() == first.String() {
		t.Fatal("exporter cached the old record")
	}
	if got := repairM484ReadConfig(t, repairM484AppEnvironment(t, updated.Bytes())); got["solanaProgramId"] != solana.ProgramID {
		t.Fatalf("fresh app boot reused old configuration: %v", got)
	}
}

func TestRepairM_484_PartialDeploymentRefusedAcrossRestart(t *testing.T) {
	for _, name := range []string{"solana-deployed", "solana-registered"} {
		t.Run(name, func(t *testing.T) {
			raw, record := repairM484Transcript(t, name)
			if record.ProgramID == "" || (name == "solana-deployed" && (record.Owner != "" || len(record.Assets) != 0)) {
				t.Fatalf("wrong deployment branch recorded: %+v", record)
			}
			dir := t.TempDir()
			evmRaw, _ := repairM484Transcript(t, "evm-ready")
			repairM484Write(t, filepath.Join(dir, "a-ethereum.json"), evmRaw)
			recordPath := filepath.Join(dir, "solana.json")
			repairM484Write(t, recordPath, raw)
			envPath := filepath.Join(t.TempDir(), "app.env")
			// Even a previously published program variable cannot bless partial
			// deployment records through the real --check consumer.
			repairM484Write(t, envPath, []byte(solanaProgramIDEnvName+"="+record.ProgramID+"\n"))
			for restart := 0; restart < 2; restart++ {
				loaded, err := LoadRecord(recordPath)
				if err == nil || !reflect.DeepEqual(loaded, Record{}) || !strings.Contains(err.Error(), recordPath) {
					t.Fatalf("restart %d accepted legacy script checkpoint: %+v, %v", restart, loaded, err)
				}
				for _, args := range [][]string{{dir}, {"--check", envPath, dir}} {
					var output bytes.Buffer
					if err := Run(args, &output); err == nil || output.Len() != 0 {
						t.Fatalf("restart %d exported part of a refused deployment: %v, %q", restart, err, output.String())
					}
					config := repairM484ReadConfig(t, repairM484AppEnvironment(t, output.Bytes()))
					if _, configured := config["solanaProgramId"]; configured || len(config["vaults"].(map[string]any)) != 0 {
						t.Fatalf("partial deployment configured the app: %v", config)
					}
				}
			}
		})
	}
	_, record := repairM484Transcript(t, "solana-registered")
	for _, phase := range []string{SolanaPhaseDeployed, SolanaPhaseInitialized} {
		t.Run("versioned_"+phase, func(t *testing.T) {
			checkpoint := record
			checkpoint.Version, checkpoint.Phase = SolanaRecordVersion, phase
			checkpoint.Assets = nil
			if phase == SolanaPhaseDeployed {
				checkpoint.Owner, checkpoint.Threshold, checkpoint.Attestors = "", 0, nil
			}
			dir := t.TempDir()
			recordPath := filepath.Join(dir, "solana.json")
			if err := WriteSolanaRecord(recordPath, checkpoint); err != nil {
				t.Fatal(err)
			}
			for restart := 0; restart < 2; restart++ {
				if loaded, err := LoadRecord(recordPath); err == nil || !reflect.DeepEqual(loaded, Record{}) || !strings.Contains(err.Error(), "phase") {
					t.Fatalf("versioned partial checkpoint recovered as ready: %+v, %v", loaded, err)
				}
				var output bytes.Buffer
				if err := Run([]string{dir}, &output); err == nil || output.Len() != 0 {
					t.Fatalf("versioned partial checkpoint exported: %v, %q", err, output.String())
				}
				if config := repairM484ReadConfig(t, repairM484AppEnvironment(t, output.Bytes())); config["solanaProgramId"] != nil {
					t.Fatalf("versioned partial checkpoint configured the app: %v", config)
				}
			}
		})
	}
}

func TestRepairM_484_ReadConfigValidatesEmittedNames(t *testing.T) {
	// Published EIP-55 vectors exercise viem's real validation and checksum
	// implementation over the native Keccak primitive, on every emitted name.
	addresses := []string{
		"0x52908400098527886E0F7030069857D2E4169EE7",
		"0x8617E340B3D01FA5F11F306F4090FD50E238070D",
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
	}
	_, solana := repairM484Transcript(t, "solana-registered")
	for _, address := range addresses {
		var emitted strings.Builder
		for _, name := range EnvNames() {
			value := strings.ToLower(address)
			if name == solanaProgramIDEnvName {
				value = solana.ProgramID
			}
			fmt.Fprintf(&emitted, "%s=%s\n", name, value)
		}
		config := repairM484ReadConfig(t, repairM484AppEnvironment(t, []byte(emitted.String())))
		vaults := config["vaults"].(map[string]any)
		if len(vaults) != 8 || config["solanaProgramId"] != solana.ProgramID {
			t.Fatalf("environment names do not reach all consumers: %v", config)
		}
		for chain, got := range vaults {
			if got != address {
				t.Fatalf("%s checksum = %v, want published EIP-55 vector %s", chain, got, address)
			}
		}
	}
	for _, invalid := range []string{"", "0x0000000000000000000000000000000000000000", "PLACEHOLDER:vault", "0xnope"} {
		// Empty values are a malformed transport; pass them directly to the
		// consumer to assert its independent refusal rather than sanitizing them.
		env := make(map[string]string)
		for _, name := range EnvNames() {
			env[name] = invalid
		}
		raw, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		config := repairM484ReadConfig(t, raw)
		if len(config["vaults"].(map[string]any)) != 0 || config["solanaProgramId"] != nil {
			t.Fatalf("invalid configuration enabled a deployment: %q, %v", invalid, config)
		}
	}
}
