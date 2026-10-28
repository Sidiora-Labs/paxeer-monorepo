package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	sequencerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	sequencertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
)

type ReceiptKey = sequencertypes.ReceiptKey

func AddGenesisReceiptKeyCmd(defaultNodeHome string) *cobra.Command {
	var publicKeyHex, homeDir string
	var firstHeight, lastHeight uint64
	cmd := &cobra.Command{
		Use:   "add-genesis-receipt-key",
		Short: "Pin an ed25519 receipt key and its inclusive height range in genesis.json",
		Long:  "Pin an ed25519 receipt public key in genesis.json. The key id is SHA-256 of the 32 public key bytes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			publicKeyBytes, err := hex.DecodeString(publicKeyHex)
			if err != nil || len(publicKeyBytes) != 32 {
				return fmt.Errorf("public-key must be a hex-encoded 32-byte ed25519 public key")
			}
			var publicKey [32]byte
			copy(publicKey[:], publicKeyBytes)
			if !verify.PublicKeyIsCanonical(publicKey) {
				return fmt.Errorf("public-key is not a canonical ed25519 key")
			}
			if firstHeight == 0 {
				return fmt.Errorf("first-height must be greater than zero")
			}
			if firstHeight > lastHeight {
				return fmt.Errorf("first-height %d is above last-height %d", firstHeight, lastHeight)
			}
			key := ReceiptKey{
				KeyId:       sha256.Sum256(publicKey[:]),
				PublicKey:   publicKey,
				FirstHeight: firstHeight,
				LastHeight:  lastHeight,
			}
			if key.KeyId == ([sequencertypes.KeyIDLength]byte{}) {
				return fmt.Errorf("receipt key has the zero key id")
			}

			clientCtx := client.GetClientContextFromCmd(cmd)
			if !cmd.Flags().Changed(flags.FlagHome) && clientCtx.HomeDir != "" {
				homeDir = clientCtx.HomeDir
			}
			config := server.GetServerContextFromCmd(cmd).Config
			config.SetRoot(homeDir)
			genFile := config.GenesisFile()
			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			if err != nil {
				return fmt.Errorf("failed to unmarshal genesis state: %w", err)
			}
			moduleState := make(map[string]json.RawMessage)
			if raw, ok := appState[sequencertypes.ModuleName]; ok {
				if err := json.Unmarshal(raw, &moduleState); err != nil {
					return fmt.Errorf("failed to unmarshal sequencer genesis state: %w", err)
				}
			}
			if moduleState == nil {
				return fmt.Errorf("sequencer genesis state must be an object")
			}
			var receiptKeys sequencerkeeper.ReceiptKeys
			if raw, ok := moduleState["receipt_keys"]; ok {
				if err := json.Unmarshal(raw, &receiptKeys); err != nil {
					return fmt.Errorf("failed to unmarshal sequencer receipt keys: %w", err)
				}
			}
			for _, pinned := range receiptKeys {
				if bytes.Equal(pinned.KeyId, key.KeyId[:]) {
					return fmt.Errorf("receipt key id %x is already pinned", key.KeyId)
				}
			}
			receiptKeys = append(receiptKeys, sequencerkeeper.ReceiptKeyEntry{
				KeyId:       key.KeyId[:],
				PublicKey:   key.PublicKey[:],
				FirstHeight: key.FirstHeight,
				LastHeight:  key.LastHeight,
			})
			moduleState["receipt_keys"], err = json.Marshal(receiptKeys)
			if err != nil {
				return fmt.Errorf("failed to marshal sequencer receipt keys: %w", err)
			}
			sequencerJSON, err := json.Marshal(moduleState)
			if err != nil {
				return fmt.Errorf("failed to marshal sequencer genesis state: %w", err)
			}
			if appState == nil {
				appState = make(map[string]json.RawMessage)
			}
			appState[sequencertypes.ModuleName] = sequencerJSON
			appStateJSON, err := json.Marshal(appState)
			if err != nil {
				return fmt.Errorf("failed to marshal application genesis state: %w", err)
			}
			genDoc.AppState = appStateJSON
			return genutil.ExportGenesisFile(genDoc, genFile)
		},
	}
	cmd.Flags().StringVar(&homeDir, flags.FlagHome, defaultNodeHome, "The application home directory")
	cmd.Flags().StringVar(&publicKeyHex, "public-key", "", "Hex-encoded 32-byte ed25519 receipt public key")
	cmd.Flags().Uint64Var(&firstHeight, "first-height", 0, "First height covered by the key (inclusive)")
	cmd.Flags().Uint64Var(&lastHeight, "last-height", 0, "Last height covered by the key (inclusive)")
	for _, name := range []string{"public-key", "first-height", "last-height"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(err)
		}
	}
	return cmd
}
