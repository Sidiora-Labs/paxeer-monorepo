package cli

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

func CmdQueryMarket() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "market [denom]",
		Example: "paxd query launchpad market launchpad/0x0000000000000000000000000000000000000001 --output json",
		Short:   "Query a launchpad market by denomination",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Market(cmd.Context(), &types.QueryMarketRequest{Denom: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryMarketByAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "market-by-address [address]",
		Example: "paxd query launchpad market-by-address 0x0000000000000000000000000000000000000001 --output json",
		Short:   "Query a launchpad market by pool or token address",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.MarketByAddress(cmd.Context(), &types.QueryMarketByAddressRequest{Address: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
