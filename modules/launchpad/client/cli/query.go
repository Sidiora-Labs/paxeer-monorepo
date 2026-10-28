package cli

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/spf13/cobra"
)

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryMarket(),
		CmdQueryMarketByAddress(),
		CmdQueryMarkets(),
		CmdQueryMarketsByCreator(),
		CmdQueryPriceHistory(),
		CmdQueryQuote(),
		CmdQueryTreasury(),
		CmdQuerySurface(),
		CmdQueryRetired(),
		CmdQueryFeeRight(),
		CmdQueryAirdropEpoch(),
		CmdQueryAirdropClaimed(),
		CmdQueryHasRole(),
		CmdQueryQuoteLedger(),
		CmdQueryFeeAccounting(),
		CmdQueryPoolMetadata(),
	)
	return cmd
}
