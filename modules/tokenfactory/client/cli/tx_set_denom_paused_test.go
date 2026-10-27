package cli

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
)

func TestNewSetDenomPausedCmd(t *testing.T) {
	cmd := NewSetDenomPausedCmd()
	require.Equal(t, "set-denom-paused [denom] [true|false]", cmd.Use)
	for _, name := range []string{flags.FlagFrom, flags.FlagGenerateOnly, flags.FlagFees, flags.FlagGas} {
		require.NotNil(t, cmd.Flags().Lookup(name), "missing --%s", name)
	}
	found, rest, err := GetTxCmd().Find([]string{"set-denom-paused", "denom", "true"})
	require.NoError(t, err)
	require.Equal(t, cmd.Use, found.Use)
	require.Equal(t, []string{"denom", "true"}, rest)

	registry := cdctypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	sender := sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	denom, err := types.GetTokenDenom(sender, "paused-token")
	require.NoError(t, err)

	testCases := []struct {
		name      string
		args      []string
		paused    bool
		wantErr   string
		boolError bool
	}{
		{name: "pause", args: []string{denom, "true"}, paused: true},
		{name: "unpause", args: []string{denom, "false"}},
		{name: "uppercase true", args: []string{denom, "TRUE"}, paused: true},
		{name: "numeric false", args: []string{denom, "0"}},
		{name: "invalid boolean", args: []string{denom, "paused"}, wantErr: "invalid syntax", boolError: true},
		{name: "empty boolean", args: []string{denom, ""}, wantErr: "invalid syntax", boolError: true},
		{name: "no arguments", wantErr: "accepts 2 arg(s), received 0"},
		{name: "missing boolean", args: []string{denom}, wantErr: "accepts 2 arg(s), received 1"},
		{name: "extra argument", args: []string{denom, "true", "extra"}, wantErr: "accepts 2 arg(s), received 3"},
		{name: "invalid denom", args: []string{"invalid", "true"}, wantErr: "not enough parts of denom invalid"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := client.Context{}.
				WithCodec(cdc).
				WithInterfaceRegistry(registry).
				WithTxConfig(txConfig).
				WithChainID("tokenfactory-cli-test").
				WithOutput(&output)
			root := GetTxCmd()
			args := append([]string{"set-denom-paused"}, tc.args...)
			root.SetArgs(append(args, "--from", sender, "--generate-only", "--offline", "--keyring-backend", "memory"))
			root.SetOut(&output)
			root.SetErr(&output)
			root.SilenceErrors = true
			root.SilenceUsage = true
			err := root.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				if tc.boolError {
					var parseErr *strconv.NumError
					require.ErrorAs(t, err, &parseErr)
					require.Equal(t, "ParseBool", parseErr.Func)
					require.Equal(t, tc.args[1], parseErr.Num)
				}
				require.Empty(t, output.String())
				return
			}
			require.NoError(t, err)
			decoded, err := txConfig.TxJSONDecoder()(output.Bytes())
			require.NoError(t, err)
			msgs := decoded.GetMsgs()
			require.Len(t, msgs, 1)
			msg, ok := msgs[0].(*types.MsgSetDenomPaused)
			require.True(t, ok, "unexpected message type %T", msgs[0])
			require.Equal(t, sender, msg.Sender)
			require.Equal(t, denom, msg.Denom)
			require.Equal(t, tc.paused, msg.Paused)
			require.NoError(t, msg.ValidateBasic())
		})
	}
}
