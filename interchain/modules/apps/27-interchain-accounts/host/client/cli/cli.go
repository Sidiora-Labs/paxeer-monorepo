package cli

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	"github.com/spf13/cobra"
)

// GetQueryCmd returns the query commands for the ICA host submodule
func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        "host",
		Short:                      "interchain-accounts host subcommands",
		Long:                       types.AvailabilityGuidance,
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
	}

	queryCmd.AddCommand(
		GetCmdParams(),
		GetCmdPacketEvents(),
	)

	return queryCmd
}
