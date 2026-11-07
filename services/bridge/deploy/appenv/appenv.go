package appenv

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/chainconfig"
)

type Record struct {
	Chain               string        `json:"chain"`
	ChainID             uint64        `json:"chain_id"`
	Kind                string        `json:"kind"`
	Vault               string        `json:"vault"`
	ProgramID           string        `json:"program_id"`
	Version             uint32        `json:"version,omitempty"`
	Phase               string        `json:"phase,omitempty"`
	Owner               string        `json:"owner,omitempty"`
	Threshold           uint32        `json:"threshold,omitempty"`
	Attestors           []string      `json:"attestors,omitempty"`
	Assets              []SolanaAsset `json:"assets,omitempty"`
	Configuration       string        `json:"configuration,omitempty"`
	GenesisHash         string        `json:"genesis_hash,omitempty"`
	ProgramDataAccount  string        `json:"program_data_account,omitempty"`
	UpgradeableLoaderID string        `json:"upgradeable_loader_id,omitempty"`
	ProgramELFSHA256    string        `json:"program_elf_sha256,omitempty"`
	ProgramELFBytes     uint64        `json:"program_elf_bytes,omitempty"`
	DeploymentSignature string        `json:"deployment_signature,omitempty"`
	DeploymentSlot      uint64        `json:"deployment_slot,omitempty"`
	RootedSlot          uint64        `json:"rooted_slot,omitempty"`
	Publisher           string        `json:"publisher,omitempty"`
	UpgradeAuthority    string        `json:"upgrade_authority,omitempty"`
	Commitment          string        `json:"commitment,omitempty"`
	VaultAuthority      string        `json:"vault_authority,omitempty"`
	VaultHandle         string        `json:"vault_handle,omitempty"`
	ProgramKeypairFile  string        `json:"program_keypair_file,omitempty"`
}

type SolanaAsset struct {
	Symbol   string `json:"symbol"`
	Mint     string `json:"mint"`
	AssetID  string `json:"asset_id"`
	Decimals uint8  `json:"decimals"`
	PerTxCap string `json:"per_tx_cap"`
	TotalCap string `json:"total_cap"`
}

func LoadRecord(path string) (Record, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("%s: %w", path, err)
	}
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := record.validate(true); err != nil {
		return Record{}, fmt.Errorf("%s: %w", path, err)
	}
	return record, nil
}

func (record Record) validate(ready bool) error {
	chain, ok := chainconfig.ChainByName(record.Chain)
	if !ok {
		return fmt.Errorf("chain: %q is not a bridge chain", record.Chain)
	}
	if record.ChainID != chain.ID {
		return fmt.Errorf("chain_id: got %d, want %d for %s", record.ChainID, chain.ID, chain.Name)
	}
	if record.Kind != string(chain.Kind) {
		return fmt.Errorf("kind: got %q, want %q for %s", record.Kind, chain.Kind, chain.Name)
	}
	if chain.Kind == chainconfig.KindEVM {
		if len(record.Vault) != 42 || !strings.HasPrefix(record.Vault, "0x") {
			return fmt.Errorf("vault: expected a 0x-prefixed 20-byte hex address")
		}
		address, err := hex.DecodeString(record.Vault[2:])
		if err != nil {
			return fmt.Errorf("vault: expected a 20-byte hex address: %w", err)
		}
		nonzero := false
		for _, b := range address {
			if b != 0 {
				nonzero = true
				break
			}
		}
		if !nonzero {
			return fmt.Errorf("vault: zero address is not a deployed vault")
		}
	} else if chain.Kind == chainconfig.KindSolana {
		programID, err := chainconfig.DecodeBase58(record.ProgramID)
		if err != nil {
			return fmt.Errorf("program_id: expected a 32-byte base58 key: %w", err)
		}
		if len(programID) != 32 {
			return fmt.Errorf("program_id: decoded to %d bytes, want 32", len(programID))
		}
		return record.validateSolana(ready)
	}
	return nil
}

const solanaProgramIDEnvName = "NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID"

func evmVaultEnvName(chain string) string {
	return "NEXT_PUBLIC_BRIDGE_VAULT_" + strings.ToUpper(chain)
}

// EnvNames lists every variable EnvLines can emit, in deployment order.
func EnvNames() []string {
	var names []string
	for _, chain := range chainconfig.Chains() {
		if chain.Kind == chainconfig.KindEVM {
			names = append(names, evmVaultEnvName(chain.Name))
		}
	}
	return append(names, solanaProgramIDEnvName)
}

func EnvLines(records []Record) []string {
	lines := make([]string, 0, len(records))
	for _, chain := range chainconfig.Chains() {
		if chain.Kind != chainconfig.KindEVM {
			continue
		}
		for _, record := range records {
			if record.Chain == chain.Name && record.Kind == string(chainconfig.KindEVM) {
				lines = append(lines, evmVaultEnvName(chain.Name)+"="+record.Vault)
			}
		}
	}
	for _, record := range records {
		if record.Chain == "solana" && record.Kind == string(chainconfig.KindSolana) && record.validate(true) == nil {
			lines = append(lines, solanaProgramIDEnvName+"="+record.ProgramID)
		}
	}
	return lines
}

// Check compares the app environment with the values emitted for the records.
func Check(envPath string, records []Record) error {
	for _, record := range records {
		if err := record.validate(true); err != nil {
			return fmt.Errorf("%s: deployment record: %w", envPath, err)
		}
	}
	raw, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("%s: %w", envPath, err)
	}
	values := make(map[string]string)
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", envPath, i+1)
		}
		values[key] = strings.TrimSpace(value)
	}
	var differences []error
	for _, line := range EnvLines(records) {
		key, want, _ := strings.Cut(line, "=")
		got, ok := values[key]
		if !ok {
			differences = append(differences, fmt.Errorf("%s: missing %s", envPath, key))
			continue
		}
		matches := got == want
		if strings.HasPrefix(key, "NEXT_PUBLIC_BRIDGE_VAULT_") {
			matches = strings.EqualFold(got, want)
		}
		if !matches {
			differences = append(differences, fmt.Errorf("%s: %s differs from deployment record: got %q, want %q", envPath, key, got, want))
		}
	}
	return errors.Join(differences...)
}

func Run(args []string, w io.Writer) error {
	var envPath string
	if len(args) == 3 && args[0] == "--check" && args[1] != "" {
		envPath = args[1]
		args = args[2:]
	}
	if len(args) != 1 || args[0] == "" || strings.HasPrefix(args[0], "--") {
		return fmt.Errorf("usage: paxeer-bridge-app-env [--check <env-file>] <records-directory>")
	}
	dir := args[0]
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var records []Record
	seen := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		record, err := LoadRecord(path)
		if err != nil {
			return err
		}
		if previous, ok := seen[record.Chain]; ok {
			return fmt.Errorf("%s: duplicate chain %q, already recorded in %s", path, record.Chain, previous)
		}
		seen[record.Chain] = path
		records = append(records, record)
	}
	if len(records) == 0 {
		return fmt.Errorf("%s: no *.json deployment records", dir)
	}
	if envPath != "" {
		return Check(envPath, records)
	}
	for _, line := range EnvLines(records) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("write app environment: %w", err)
		}
	}
	return nil
}
