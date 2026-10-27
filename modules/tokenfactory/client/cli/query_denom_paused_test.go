package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func TestGetCmdDenomPausedArgs(t *testing.T) {
	root := GetQueryCmd()
	registered, remaining, err := root.Find([]string{"denom-paused"})
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.NotSame(t, root, registered)
	require.Same(t, root, registered.Parent())

	commands := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "standalone", cmd: GetCmdDenomPaused()},
		{name: "registered", cmd: registered},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			cmd := command.cmd
			require.Equal(t, "denom-paused [denom]", cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NotNil(t, cmd.Args)
			for _, flag := range []string{flags.FlagNode, flags.FlagHeight, flags.FlagOutput} {
				require.NotNil(t, cmd.Flags().Lookup(flag), "missing query flag %s", flag)
			}

			require.EqualError(t, cmd.ValidateArgs(nil), "accepts 1 arg(s), received 0")
			require.NoError(t, cmd.ValidateArgs([]string{"factory/creator/token"}))
			require.EqualError(t, cmd.ValidateArgs([]string{"factory/creator/token", "extra"}), "accepts 1 arg(s), received 2")
		})
	}
}
