package cli

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

func CmdQueryTreasury() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "treasury [token]",
		Short: "Query a token's launchpad treasury balance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			response, err := queryClient.Treasury(cmd.Context(), &types.QueryTreasuryRequest{Token: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(response)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryQuoteLedger() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quote-ledger [held|fee-held|treasury-held] [address]",
		Short: "Query a launchpad quote ledger balance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			response, err := queryClient.QuoteLedger(cmd.Context(), &types.QueryQuoteLedgerRequest{
				Kind: args[0], Address: args[1],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(response)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
