package cli_test

import (
	"bytes"
	"strings"
	"testing"

	paxdcmd "github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	servercmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	"github.com/stretchr/testify/require"
)

func TestRepairM_945_HasRoleHelp(t *testing.T) {
	const contract = "0x7b14808b72f61664cDb79D25E6d1587655571a66"
	const role = "0x9f2df0fed2c77648de5860a4cc508cd0818c85b8b8a1ab4ceeef8d981c8956a6"
	const account = "0x0000000000000000000000000000000000000002"
	const example = "paxd query launchpad has-role " + contract + " " + role + " " + account + " --output json"

	for _, args := range [][]string{
		{"query", "launchpad", "has-role", "--help"},
		{"help", "query", "launchpad", "has-role"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root, _ := paxdcmd.NewRootCmd()
			require.Equal(t, "paxd", root.Name())
			var output, errors bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&errors)
			root.SetArgs(args)
			require.NoError(t, servercmd.Execute(root, t.TempDir()))
			require.Empty(t, errors.String())
			help := output.String()
			require.Contains(t, help, "paxd query launchpad has-role [contract] [role] [account] [flags]")
			require.Contains(t, help, "Examples:\n"+example+"\n")
			require.NotContains(t, help, "layerxd")

			command, remaining, err := root.Find(strings.Fields(example)[1:])
			require.NoError(t, err)
			require.Equal(t, "paxd query launchpad has-role", command.CommandPath())
			require.Equal(t, "has-role [contract] [role] [account]", command.Use)
			require.Equal(t, example, command.Example)
			require.NotNil(t, command.RunE)
			require.NoError(t, command.ParseFlags(remaining))
			require.Equal(t, []string{contract, role, account}, command.Flags().Args())
			require.NoError(t, command.ValidateArgs(command.Flags().Args()))
			format, err := command.Flags().GetString(flags.FlagOutput)
			require.NoError(t, err)
			require.Equal(t, "json", format)
			require.EqualError(t, command.ValidateArgs([]string{contract, role}), "accepts 3 arg(s), received 2")
			require.EqualError(t, command.ValidateArgs([]string{contract, role, account, "extra"}), "accepts 3 arg(s), received 4")
		})
	}
}
