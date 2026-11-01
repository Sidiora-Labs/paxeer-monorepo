package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
)

func TestPayQueryParamsCmd(t *testing.T) {
	t.Run("registration and query flags", func(t *testing.T) {
		root := cli.GetQueryCmd()
		require.Equal(t, types.ModuleName, root.Use)
		cmd, remaining, err := root.Find([]string{"params"})
		require.NoError(t, err)
		require.Empty(t, remaining)
		require.Equal(t, "layerxpay params", cmd.CommandPath())
		require.NotNil(t, cmd.RunE)
		require.NotNil(t, cmd.Args)
		require.NoError(t, cmd.Args(cmd, nil))
		require.Error(t, cmd.Args(cmd, []string{"unexpected"}))
		for _, name := range []string{flags.FlagNode, flags.FlagHeight, flags.FlagOutput} {
			require.NotNil(t, cmd.Flags().Lookup(name), "missing query flag %s", name)
		}
	})

	for _, tc := range []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "reject positional argument",
			args: []string{"params", "unexpected", "--node="},
			err:  "unknown command \"unexpected\" for \"layerxpay params\"",
		},
		{
			name: "propagate client context error",
			args: []string{"params", "--node=://invalid"},
			err:  "missing protocol scheme",
		},
		{
			name: "propagate Params query error",
			args: []string{"params", "--node=", "--output=json", "--height=7"},
			err:  "no RPC client is defined in offline mode",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := client.Context{}.
				WithCodec(codec.NewProtoCodec(codectypes.NewInterfaceRegistry())).
				WithOutput(&output)
			root := cli.GetQueryCmd()
			root.SilenceErrors = true
			root.SilenceUsage = true
			root.SetArgs(tc.args)
			ctx := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)
			err := root.ExecuteContext(ctx)
			require.ErrorContains(t, err, tc.err)
			require.Empty(t, output.String())
		})
	}

	t.Run("standalone params command", func(t *testing.T) {
		var output bytes.Buffer
		cmd := cli.CmdQueryParams()
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{"--help"})
		require.NoError(t, cmd.Execute())
		require.Contains(t, output.String(), "Query the current layerxpay parameters")
		require.Contains(t, output.String(), "--height")
		require.Contains(t, output.String(), "--output")
	})
}
