package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	govcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func repairM572ClientContext() client.Context {
	registry := cdctypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	proposer := sdk.AccAddress(bytes.Repeat([]byte{0x57}, 20))
	return client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(registry).
		WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)).
		WithKeyring(keyring.NewInMemory()).
		WithChainID("mint-cli-repair").
		WithFrom(proposer.String()).
		WithFromAddress(proposer).
		WithOffline(true)
}

func repairM572RunProposal(t *testing.T, clientCtx client.Context, path string, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	root := govcli.NewTxCmd([]*cobra.Command{UpdateMinterHandler.CLIHandler()})
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(&output)
	root.SetErr(&output)
	commandArgs := []string{"submit-proposal", "update-minter", path, "--node", ""}
	root.SetArgs(append(commandArgs, args...))
	var err error
	require.NotPanics(t, func() {
		err = root.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	})
	return output.String(), err
}

func repairM572ProposalFile(t *testing.T, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "minter proposal.json")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}

func TestRepairM_572_MalformedProposalFiles(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "truncated JSON", body: `{"title":"Update minter",`},
		{name: "invalid title type", body: `{"title":42}`},
		{name: "invalid protobuf amount", body: `{"minter":{"total_mint_amount":"not-a-number"}}`},
		{name: "overflowing protobuf amount", body: `{"minter":{"total_mint_amount":"18446744073709551616"}}`},
		{name: "unknown protobuf field", body: `{"unknown_minter_field":true}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientCtx := repairM572ClientContext()
			path := repairM572ProposalFile(t, []byte(tc.body))
			var proposal types.UpdateMinterProposal
			decodeErr := clientCtx.Codec.UnmarshalAsJSON([]byte(tc.body), &proposal)
			require.Error(t, decodeErr, "fixture must fail the real protobuf decoder")

			for _, deposit := range []string{"1250uhpx", "invalid-deposit"} {
				t.Run(deposit, func(t *testing.T) {
					output, err := repairM572RunProposal(t, clientCtx, path, "--deposit", deposit)
					require.EqualError(t, err, fmt.Sprintf("decode minter proposal file %q: %s", path, decodeErr))
					require.EqualError(t, errors.Unwrap(err), decodeErr.Error())
					require.Empty(t, output, "bad files must return before transaction generation or broadcast")
				})
			}
		})
	}
}

func TestRepairM_572_ValidProposalFile(t *testing.T) {
	clientCtx := repairM572ClientContext()
	minter := types.NewMinter("2026-01-01", "2026-12-31", "uhpx", 1000)
	minter.RemainingMintAmount = 750
	minter.LastMintDate = "2026-01-03"
	minter.LastMintHeight = 42
	minter.LastMintAmount = 100
	proposal := types.NewUpdateMinterProposalHandler("Update minter", "A scheduled token release", minter)
	require.NoError(t, proposal.ValidateBasic())
	contents, err := clientCtx.Codec.MarshalAsJSON(proposal)
	require.NoError(t, err)
	path := repairM572ProposalFile(t, contents)

	output, err := repairM572RunProposal(t, clientCtx, path,
		"--generate-only", "--offline", "--from", clientCtx.From,
		"--deposit", "1250uhpx")
	require.NoError(t, err)
	require.NotEmpty(t, output)
	decoded, err := clientCtx.TxConfig.TxJSONDecoder()([]byte(output))
	require.NoError(t, err)
	msgs := decoded.GetMsgs()
	require.Len(t, msgs, 1)
	msg, ok := msgs[0].(*govtypes.MsgSubmitProposal)
	require.True(t, ok, "unexpected generated message type %T", msgs[0])
	require.Equal(t, clientCtx.From, msg.Proposer)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1250)), msg.InitialDeposit)
	content, ok := msg.GetContent().(*types.UpdateMinterProposal)
	require.True(t, ok, "unexpected proposal content type %T", msg.GetContent())
	require.Equal(t, proposal.Title, content.Title)
	require.Equal(t, proposal.Description, content.Description)
	require.Equal(t, proposal.Minter, content.Minter)
	require.NoError(t, msg.ValidateBasic())
}

func TestRepairM_572_UnreadableProposalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing proposal.json")
	output, err := repairM572RunProposal(t, repairM572ClientContext(), path, "--deposit", "1250uhpx")
	require.ErrorContains(t, err, fmt.Sprintf("read minter proposal file %q:", path))
	require.ErrorIs(t, err, os.ErrNotExist)
	var pathErr *os.PathError
	require.ErrorAs(t, err, &pathErr)
	require.Equal(t, path, pathErr.Path)
	require.Empty(t, output)
}
