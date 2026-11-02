package cli_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestGetQueryCmdListsLaunchpadQueries(t *testing.T) {
	want := []string{
		"params", "market", "market-by-address", "markets", "markets-by-creator",
		"price-history", "quote", "treasury", "surface", "retired", "fee-right",
		"airdrop-epoch", "airdrop-claimed", "has-role", "quote-ledger",
		"fee-accounting", "pool-metadata",
	}
	basic := launchpad.NewAppModuleBasic()
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"cli", cli.GetQueryCmd()},
		{"module", basic.GetQueryCmd()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd
			require.NotNil(t, cmd)
			require.Equal(t, types.ModuleName, cmd.Use)
			require.True(t, cmd.DisableFlagParsing)
			require.Equal(t, 2, cmd.SuggestionsMinimumDistance)
			require.NotNil(t, cmd.RunE)
			require.Len(t, cmd.Commands(), len(want))
			names := make([]string, 0, len(want))
			for _, subcommand := range cmd.Commands() {
				names = append(names, subcommand.Name())
				require.Same(t, cmd, subcommand.Parent())
				require.NotNil(t, subcommand.RunE, subcommand.Name())
				require.NotNil(t, subcommand.Args, subcommand.Name())
				require.NotNil(t, subcommand.Flags().Lookup(flags.FlagNode), subcommand.Name())
				require.NotNil(t, subcommand.Flags().Lookup(flags.FlagHeight), subcommand.Name())
			}
			require.ElementsMatch(t, want, names)
		})
	}
	require.Nil(t, basic.GetTxCmd())
}
