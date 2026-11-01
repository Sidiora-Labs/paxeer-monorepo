package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	sequencertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"github.com/spf13/cobra"
)

// GenesisState is the sequencer state edited by the genesis leader command.
type GenesisState = sequencertypes.GenesisState

// AddGenesisSequencerLeaderCmd returns the command that appends a consensus key.
func AddGenesisSequencerLeaderCmd(defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-genesis-sequencer-leader --pubkey HEX",
		Short: "Append a sequencer leader consensus public key to genesis.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pubkeyHex, err := cmd.Flags().GetString("pubkey")
			if err != nil {
				return err
			}
			pubkey, err := hex.DecodeString(pubkeyHex)
			if err != nil {
				return fmt.Errorf("invalid consensus public key hex: %w", err)
			}
			if len(pubkey) != 32 {
				return fmt.Errorf("consensus public key is %d bytes, want 32", len(pubkey))
			}
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}
			config := server.GetServerContextFromCmd(cmd).Config
			config.SetRoot(home)
			genFile := config.GenesisFile()
			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			if err != nil {
				return fmt.Errorf("failed to read genesis state: %w", err)
			}
			var moduleState map[string]json.RawMessage
			if err := json.Unmarshal(appState[sequencertypes.ModuleName], &moduleState); err != nil {
				return fmt.Errorf("failed to unmarshal sequencer genesis state: %w", err)
			}
			if moduleState == nil {
				return fmt.Errorf("sequencer genesis state must be an object")
			}
			// The module's JSON wire format stores keys as byte strings, not arrays.
			var leaders [][]byte
			if raw, ok := moduleState["leaders"]; ok {
				if err := json.Unmarshal(raw, &leaders); err != nil {
					return fmt.Errorf("failed to unmarshal sequencer leaders: %w", err)
				}
			}
			state := GenesisState{Leaders: make([][32]byte, len(leaders), len(leaders)+1)}
			for i, leader := range leaders {
				if len(leader) != 32 {
					return fmt.Errorf("leader %d is %d bytes, want 32", i, len(leader))
				}
				copy(state.Leaders[i][:], leader)
			}
			var leader [32]byte
			copy(leader[:], pubkey)
			state.Leaders = append(state.Leaders, leader)
			if err := sequencertypes.ValidateLeaders(state.Leaders); err != nil {
				return err
			}
			leaders = append(leaders, pubkey)
			moduleState["leaders"], err = json.Marshal(leaders)
			if err != nil {
				return fmt.Errorf("failed to marshal sequencer leaders: %w", err)
			}
			appState[sequencertypes.ModuleName], err = json.Marshal(moduleState)
			if err != nil {
				return fmt.Errorf("failed to marshal sequencer genesis state: %w", err)
			}
			genDoc.AppState, err = json.Marshal(appState)
			if err != nil {
				return fmt.Errorf("failed to marshal application genesis state: %w", err)
			}
			return genutil.ExportGenesisFile(genDoc, genFile)
		},
	}
	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	cmd.Flags().String("pubkey", "", "32-byte consensus public key encoded as hex")
	if err := cmd.MarkFlagRequired("pubkey"); err != nil {
		panic(err)
	}
	return cmd
}
