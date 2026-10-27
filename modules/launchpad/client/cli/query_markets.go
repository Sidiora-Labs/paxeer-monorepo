package cli

import (
	"fmt"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

func CmdQueryMarkets() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "markets",
		Short: "Query launchpad markets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			offset, limit, err := queryMarketsPagination(cmd)
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Markets(cmd.Context(), &types.QueryMarketsRequest{Offset: offset, Limit: limit})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	cmd.Flags().String(flags.FlagOffset, "0", "Number of markets to skip")
	cmd.Flags().String(flags.FlagLimit, "100", "Maximum number of markets to return (1-100)")
	return cmd
}

func CmdQueryMarketsByCreator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "markets-by-creator [creator]",
		Short: "Query launchpad markets by creator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			offset, limit, err := queryMarketsPagination(cmd)
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.MarketsByCreator(cmd.Context(), &types.QueryMarketsByCreatorRequest{
				Creator: args[0], Offset: offset, Limit: limit,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	cmd.Flags().String(flags.FlagOffset, "0", "Number of creator markets to skip")
	cmd.Flags().String(flags.FlagLimit, "100", "Maximum number of markets to return (1-100)")
	return cmd
}

func queryMarketsPagination(cmd *cobra.Command) (uint64, uint64, error) {
	offsetText, err := cmd.Flags().GetString(flags.FlagOffset)
	if err != nil {
		return 0, 0, err
	}
	offset, err := strconv.ParseUint(offsetText, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid --offset: %w", err)
	}
	limitText, err := cmd.Flags().GetString(flags.FlagLimit)
	if err != nil {
		return 0, 0, err
	}
	limit, err := strconv.ParseUint(limitText, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid --limit: %w", err)
	}
	return offset, limit, nil
}
