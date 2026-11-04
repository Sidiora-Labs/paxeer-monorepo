package cli_test

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client/cli"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
)

func TestRepair463_SignRejectsInvalidGasAdjustment(t *testing.T) {
	for name, command := range map[string]func() *cobra.Command{
		"sign":       cli.GetSignCommand,
		"sign-batch": cli.GetSignBatchCommand,
	} {
		t.Run(name, func(t *testing.T) {
			r := repair660Setup(t)
			file := repair660File(t, r.unsigned(t, false, "factory validation"))
			for _, adjustment := range []string{"-1", "NaN", "+Inf", "-Inf"} {
				t.Run(adjustment, func(t *testing.T) {
					out, _, err := repair660Execute(command(), r.ctx, nil,
						repair660OfflineArgs(file, "--from=alice", "--gas-adjustment="+adjustment)...)
					require.EqualError(t, err, fmt.Sprintf("invalid --gas-adjustment: gas adjustment must be finite and non-negative: %s", adjustment))
					require.Empty(t, out)
				})
			}
		})
	}
}

func TestRepair463_SignAcceptsValidGasAdjustment(t *testing.T) {
	for name, command := range map[string]func() *cobra.Command{
		"sign":       cli.GetSignCommand,
		"sign-batch": cli.GetSignBatchCommand,
	} {
		t.Run(name, func(t *testing.T) {
			r := repair660Setup(t)
			file := repair660File(t, r.unsigned(t, false, "valid factory"))
			out, _, err := repair660Execute(command(), r.ctx, nil,
				repair660OfflineArgs(file, "--from=alice", "--gas-adjustment=1.25", "--sign-mode=amino-json", "--signature-only=false")...)
			require.NoError(t, err)
			parsed, err := r.ctx.TxConfig.TxJSONDecoder()(out)
			require.NoError(t, err)
			signed := parsed.(authsigning.Tx)
			require.Equal(t, "valid factory", signed.GetMemo())
			sigs, err := signed.GetSignaturesV2()
			require.NoError(t, err)
			require.Len(t, sigs, 1)
			require.True(t, sigs[0].PubKey.Equals(r.keys[0].GetPubKey()))
			require.Equal(t, uint64(11), sigs[0].Sequence)
			require.NoError(t, authsigning.VerifySignature(sigs[0].PubKey, authsigning.SignerData{
				ChainID: r.ctx.ChainID, AccountNumber: 7, Sequence: 11,
			}, sigs[0].Data, r.ctx.TxConfig.SignModeHandler(), parsed))
		})
	}
}
