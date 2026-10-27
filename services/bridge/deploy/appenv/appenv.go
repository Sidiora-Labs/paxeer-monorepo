package appenv

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/chainconfig"
)

type Record struct {
	Chain     string `json:"chain"`
	ChainID   uint64 `json:"chain_id"`
	Kind      string `json:"kind"`
	Vault     string `json:"vault"`
	ProgramID string `json:"program_id"`
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
	chain, ok := chainconfig.ChainByName(record.Chain)
	if !ok {
		return Record{}, fmt.Errorf("%s: chain: %q is not a bridge chain", path, record.Chain)
	}
	if record.ChainID != chain.ID {
		return Record{}, fmt.Errorf("%s: chain_id: got %d, want %d for %s", path, record.ChainID, chain.ID, chain.Name)
	}
	if record.Kind != string(chain.Kind) {
		return Record{}, fmt.Errorf("%s: kind: got %q, want %q for %s", path, record.Kind, chain.Kind, chain.Name)
	}
	if chain.Kind == chainconfig.KindEVM {
		if len(record.Vault) != 42 || !strings.HasPrefix(record.Vault, "0x") {
			return Record{}, fmt.Errorf("%s: vault: expected a 0x-prefixed 20-byte hex address", path)
		}
		address, err := hex.DecodeString(record.Vault[2:])
		if err != nil {
			return Record{}, fmt.Errorf("%s: vault: expected a 20-byte hex address: %w", path, err)
		}
		nonzero := false
		for _, b := range address {
			if b != 0 {
				nonzero = true
				break
			}
		}
		if !nonzero {
			return Record{}, fmt.Errorf("%s: vault: zero address is not a deployed vault", path)
		}
	}
	return record, nil
}

func EnvLines(records []Record) []string {
	lines := make([]string, 0, len(records))
	for _, chain := range chainconfig.Chains() {
		if chain.Kind != chainconfig.KindEVM {
			continue
		}
		for _, record := range records {
			if record.Chain == chain.Name && record.Kind == string(chainconfig.KindEVM) {
				lines = append(lines, "NEXT_PUBLIC_BRIDGE_VAULT_"+strings.ToUpper(chain.Name)+"="+record.Vault)
			}
		}
	}
	return lines
}

func Run(args []string, w io.Writer) error {
	if len(args) != 1 || args[0] == "" {
		return fmt.Errorf("usage: paxeer-bridge-app-env <records-directory>")
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
	for _, line := range EnvLines(records) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("write app environment: %w", err)
		}
	}
	return nil
}
