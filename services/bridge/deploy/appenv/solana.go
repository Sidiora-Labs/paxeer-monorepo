package appenv

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/chainconfig"
)

const SolanaRecordVersion uint32 = 1

const (
	SolanaPhaseDeployed    = "deployed"
	SolanaPhaseInitialized = "initialized"
	SolanaPhaseReady       = "ready"
)

func (record Record) validateSolana(ready bool) error {
	if record.ProgramID == chainconfig.SolanaZeroPubkey {
		return fmt.Errorf("program_id: zero pubkey is not a deployed program")
	}
	if record.Version != SolanaRecordVersion {
		return fmt.Errorf("version: got %d, want %d for a Solana deployment record", record.Version, SolanaRecordVersion)
	}
	switch record.Phase {
	case SolanaPhaseDeployed, SolanaPhaseInitialized, SolanaPhaseReady:
	default:
		return fmt.Errorf("phase: unknown Solana deployment phase %q", record.Phase)
	}
	if ready && record.Phase != SolanaPhaseReady {
		return fmt.Errorf("phase: %q is not ready; initialized custody configuration and registered assets must be verified", record.Phase)
	}
	if record.Phase == SolanaPhaseDeployed {
		return nil
	}
	owner, err := chainconfig.DecodeBase58(record.Owner)
	if err != nil || len(owner) != 32 || record.Owner == chainconfig.SolanaZeroPubkey {
		return fmt.Errorf("owner: expected a nonzero 32-byte base58 key")
	}
	if len(record.Attestors) == 0 || len(record.Attestors) > chainconfig.MaxAttestors {
		return fmt.Errorf("attestors: expected between 1 and %d signers", chainconfig.MaxAttestors)
	}
	previous := ""
	for i, attestor := range record.Attestors {
		if err := nonzeroAddress(attestor); err != nil {
			return fmt.Errorf("attestors[%d]: %w", i, err)
		}
		key := strings.ToLower(attestor)
		if key <= previous {
			return fmt.Errorf("attestors[%d]: signer set must be unique and strictly ascending", i)
		}
		previous = key
	}
	if record.Threshold == 0 || uint64(record.Threshold) > uint64(len(record.Attestors)) {
		return fmt.Errorf("threshold: must be between 1 and the %d attestors", len(record.Attestors))
	}
	if record.Phase == SolanaPhaseInitialized {
		return nil
	}
	if len(record.Assets) == 0 {
		return fmt.Errorf("assets: ready deployment has no registered assets")
	}
	mints := make(map[string]bool)
	symbols := make(map[string]bool)
	for i, asset := range record.Assets {
		field := fmt.Sprintf("assets[%d]", i)
		mint, err := chainconfig.DecodeBase58(asset.Mint)
		if err != nil || len(mint) != 32 || asset.Mint == chainconfig.SolanaZeroPubkey {
			return fmt.Errorf("%s.mint: expected a nonzero 32-byte base58 key", field)
		}
		if mints[asset.Mint] || symbols[asset.Symbol] || strings.TrimSpace(asset.Symbol) == "" {
			return fmt.Errorf("%s: mint and symbol must be present and unique", field)
		}
		mints[asset.Mint], symbols[asset.Symbol] = true, true
		want, err := chainconfig.SolanaHandle(asset.Mint)
		if err != nil {
			return fmt.Errorf("%s.mint: %w", field, err)
		}
		if asset.Mint == chainconfig.SidioraMint {
			want = chainconfig.SidioraAssetID
			if asset.Symbol != "SID" || asset.Decimals != 6 {
				return fmt.Errorf("%s: Sidiora requires symbol SID and 6 decimals", field)
			}
		}
		if !strings.EqualFold(asset.AssetID, want) {
			return fmt.Errorf("%s.asset_id: got %q, want %s for mint %s", field, asset.AssetID, want, asset.Mint)
		}
		if asset.Decimals == 0 || asset.Decimals > 18 {
			return fmt.Errorf("%s.decimals: expected between 1 and 18", field)
		}
		if asset.Mint == chainconfig.WrappedSolMint && (i != 0 || asset.Symbol != "SOL" || asset.Decimals != 9) {
			return fmt.Errorf("%s: wrapped SOL must be first, with symbol SOL and 9 decimals", field)
		}
		caps := chainconfig.Asset{PerTxCap: asset.PerTxCap, TotalCap: asset.TotalCap}
		perTx, err := caps.PerTx()
		if err != nil || perTx.Sign() == 0 {
			return fmt.Errorf("%s.per_tx_cap: expected a positive decimal amount", field)
		}
		total, err := caps.Total()
		if err != nil || total.Sign() == 0 || perTx.Cmp(total) > 0 {
			return fmt.Errorf("%s.total_cap: expected a positive decimal amount at least per_tx_cap", field)
		}
	}
	if !mints[chainconfig.WrappedSolMint] || !mints[chainconfig.SidioraMint] {
		return fmt.Errorf("assets: ready deployment must register both SOL and SID offered by the bridge app")
	}
	return nil
}

func nonzeroAddress(value string) error {
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return fmt.Errorf("expected a nonzero 20-byte hex address")
	}
	raw, err := hex.DecodeString(value[2:])
	if err != nil {
		return fmt.Errorf("expected a nonzero 20-byte hex address: %w", err)
	}
	for _, b := range raw {
		if b != 0 {
			return nil
		}
	}
	return fmt.Errorf("zero address is not an attestor")
}

// WriteSolanaRecord publishes a versioned checkpoint with an atomic rename.
// The producer may set ready only after reading back custody configuration and
// every registered asset; the reader independently validates that declaration.
func WriteSolanaRecord(path string, record Record) error {
	if record.Chain != "solana" {
		return fmt.Errorf("%s: chain: expected solana", path)
	}
	if err := record.validate(false); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".solana-checkpoint-*")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
