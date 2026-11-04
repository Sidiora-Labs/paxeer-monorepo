package cli

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	"github.com/spf13/cobra"
)

// GetQueryCmd returns the query commands for the ICA controller submodule
func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        "controller",
		Short:                      "interchain-accounts controller subcommands",
		Long:                       types.AvailabilityGuidance,
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
	}

	queryCmd.AddCommand(
		GetCmdQueryInterchainAccount(),
		GetCmdParams(),
	)

	return queryCmd
}
