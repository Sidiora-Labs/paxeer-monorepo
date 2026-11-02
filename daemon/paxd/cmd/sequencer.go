package cmd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func SequencerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sequencer",
		Short: "Manage sequencer receipt keys",
	}
	var output string
	gen := &cobra.Command{
		Use:   "gen-receipt-key",
		Short: "Generate a receipt signing key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output == "" {
				return fmt.Errorf("--output is required")
			}
			seed := make([]byte, ed25519.SeedSize)
			defer clear(seed)
			if _, err := rand.Read(seed); err != nil {
				return fmt.Errorf("generate receipt key: %w", err)
			}
			private := ed25519.NewKeyFromSeed(seed)
			defer clear(private)
			public := private.Public().(ed25519.PublicKey)

			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return fmt.Errorf("create receipt key file: %w", err)
			}
			encoded := make([]byte, hex.EncodedLen(len(seed))+1)
			defer clear(encoded)
			hex.Encode(encoded, seed)
			encoded[len(encoded)-1] = '\n'
			if _, err := file.Write(encoded); err != nil {
				file.Close()
				return fmt.Errorf("write receipt key file: %w", err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close receipt key file: %w", err)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				PublicKey string `json:"public_key"`
			}{PublicKey: hex.EncodeToString(public)})
		},
	}
	gen.Flags().StringVar(&output, "output", "", "Path for the new receipt key file")
	cmd.AddCommand(gen)
	return cmd
}
