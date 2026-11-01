package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func TestGetQueryCmdMountsEveryXWebRead(t *testing.T) {
	cmd := cli.GetQueryCmd()
	require.Equal(t, types.ModuleName, cmd.Name())

	names := make([]string, 0, len(cmd.Commands()))
	for _, subcommand := range cmd.Commands() {
		names = append(names, subcommand.Name())
		t.Run(subcommand.Name(), func(t *testing.T) {
			require.NotNil(t, subcommand.RunE)
			require.NotNil(t, subcommand.Flags().Lookup(flags.FlagNode))
			require.NotNil(t, subcommand.Flags().Lookup(flags.FlagOutput))
		})
	}

	require.ElementsMatch(t, []string{
		"params",
		"attestors",
		"request",
		"result",
		"program-request",
		"program-result",
		"program-counters",
	}, names)
}
