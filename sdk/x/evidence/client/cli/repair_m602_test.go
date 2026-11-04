package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
)

// repairM602Submission is a supplied module command fixture. It uses the SDK's
// evidence codec, message constructor and transaction generation path.
func repairM602Submission(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:  name + " [evidence-json]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			var evidence types.Equivocation
			if err := clientCtx.Codec.UnmarshalAsJSON([]byte(args[0]), &evidence); err != nil {
				return err
			}
			msg, err := types.NewMsgSubmitEvidence(clientCtx.GetFromAddress(), &evidence)
			if err != nil {
				return err
			}
			return clienttx.GenerateOrBroadcastTxCLI(cmd.Context(), clientCtx, cmd.Flags(), msg)
		},
	}
	cmd.Flags().String(flags.FlagChainID, "", "Chain ID")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func repairM602Context(output *bytes.Buffer) client.Context {
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	return client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(registry).
		WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)).
		WithOutput(output)
}

func repairM602Evidence() *types.Equivocation {
	return &types.Equivocation{
		Height:           42,
		Power:            100,
		ConsensusAddress: sdk.ConsAddress(bytes.Repeat([]byte{2}, 20)).String(),
		Time:             time.Unix(1700000000, 0).UTC(),
	}
}

func repairM602Args(submitter sdk.AccAddress) []string {
	return []string{
		"--" + flags.FlagFrom, submitter.String(),
		"--" + flags.FlagChainID, "repair-m602",
		"--" + flags.FlagGenerateOnly,
		"--" + flags.FlagOffline,
		"--" + flags.FlagKeyringBackend, "memory",
		"--" + flags.FlagNode, "",
		"--" + flags.FlagAccountNumber, "7",
		"--" + flags.FlagSequence, "9",
		"--" + flags.FlagGas, "123456",
		"--" + flags.FlagFees, "17stake",
		"--" + flags.FlagNote, "evidence submission",
		"--" + flags.FlagTimeoutHeight, "88",
	}
}

func TestRepairM_602_SubmissionThroughCommandTree(t *testing.T) {
	for _, mode := range []string{flags.SignModeDirect, flags.SignModeLegacyAminoJSON} {
		t.Run(mode, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := repairM602Context(&output)
			first := repairM602Submission("equivocation")
			second := repairM602Submission("other-equivocation")
			root := GetTxCmd([]*cobra.Command{first, second})
			root.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
			root.SetOut(&output)
			root.SetErr(&output)

			for _, child := range []*cobra.Command{first, second} {
				found, remaining, err := root.Find([]string{"submit", child.Name()})
				require.NoError(t, err)
				require.Empty(t, remaining)
				require.Same(t, child, found)
				require.Equal(t, "submit", child.Parent().Name())
				require.Same(t, root, child.Parent().Parent())
			}

			submitter := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
			evidence := repairM602Evidence()
			encoded, err := clientCtx.Codec.MarshalAsJSON(evidence)
			require.NoError(t, err)
			args := []string{"submit", second.Name(), string(encoded)}
			args = append(args, repairM602Args(submitter)...)
			args = append(args, "--"+flags.FlagSignMode, mode)
			root.SetArgs(args)
			executed, err := root.ExecuteC()
			require.NoError(t, err, output.String())
			require.Same(t, second, executed)

			parsedCtx, err := client.GetClientTxContext(second)
			require.NoError(t, err)
			require.Equal(t, submitter, parsedCtx.GetFromAddress())
			require.Equal(t, mode, parsedCtx.SignModeStr)
			require.True(t, parsedCtx.GenerateOnly)
			require.True(t, parsedCtx.Offline)
			factory, err := clienttx.NewFactoryCLI(parsedCtx, second.Flags())
			require.NoError(t, err)
			require.Equal(t, "repair-m602", factory.ChainID())
			require.Equal(t, uint64(7), factory.AccountNumber())
			require.Equal(t, uint64(9), factory.Sequence())

			decoded, err := clientCtx.TxConfig.TxJSONDecoder()(output.Bytes())
			require.NoError(t, err, output.String())
			require.Len(t, decoded.GetMsgs(), 1)
			msg, ok := decoded.GetMsgs()[0].(*types.MsgSubmitEvidence)
			require.True(t, ok)
			require.NoError(t, msg.ValidateBasic())
			require.Equal(t, submitter.String(), msg.Submitter)
			require.Equal(t, []sdk.AccAddress{submitter}, msg.GetSigners())
			require.Equal(t, evidence, msg.GetEvidence())
			require.Equal(t, evidence.Hash(), msg.GetEvidence().Hash())
			tx, ok := decoded.(authsigning.Tx)
			require.True(t, ok)
			require.Equal(t, "evidence submission", tx.GetMemo())
			require.Equal(t, uint64(123456), tx.GetGas())
			require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("stake", 17)), tx.GetFee())
			require.Equal(t, uint64(88), tx.GetTimeoutHeight())
		})
	}
}

func TestRepairM_602_SubmissionValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing argument", want: "accepts 1 arg(s), received 0"},
		{name: "extra argument", args: []string{"{}", "{}"}, want: "accepts 1 arg(s), received 2"},
		{name: "invalid evidence", args: []string{"{}"}, want: "invalid equivocation time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := repairM602Context(&output)
			child := repairM602Submission("equivocation")
			root := GetTxCmd([]*cobra.Command{child})
			root.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
			root.SetOut(&output)
			root.SetErr(&output)
			root.SilenceErrors = true
			root.SilenceUsage = true
			args := append([]string{"submit", child.Name()}, tc.args...)
			args = append(args, repairM602Args(sdk.AccAddress(bytes.Repeat([]byte{1}, 20)))...)
			root.SetArgs(args)
			err := root.Execute()
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, output.String())
		})
	}
}
