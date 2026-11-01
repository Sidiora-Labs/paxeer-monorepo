package cli

import (
	"fmt"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

func CmdQueryPriceHistory() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "price-history [denom]",
		Short: "Query a launchpad market's price history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := cmd.Flags().GetString("from-height")
			if err != nil {
				return err
			}
			fromHeight, err := strconv.ParseUint(from, 10, 64)
			if err != nil {
				return fmt.Errorf("from-height %q: %w", from, err)
			}
			to, err := cmd.Flags().GetString("to-height")
			if err != nil {
				return err
			}
			toHeight, err := strconv.ParseUint(to, 10, 64)
			if err != nil {
				return fmt.Errorf("to-height %q: %w", to, err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			response, err := types.NewQueryClient(clientCtx).PriceHistory(cmd.Context(), &types.QueryPriceHistoryRequest{
				Denom: args[0], FromHeight: fromHeight, ToHeight: toHeight,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(response)
		},
	}
	cmd.Flags().String("from-height", "0", "First height in the price history range")
	cmd.Flags().String("to-height", "0", "Last height in the price history range")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryQuote() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quote [denom] [buy|sell] [amount-in]",
		Short: "Query a launchpad buy or sell quote",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[1] != "buy" && args[1] != "sell" {
				return fmt.Errorf("side must be buy or sell, got %q", args[1])
			}
			if _, err := strconv.ParseUint(args[2], 10, 64); err != nil {
				return fmt.Errorf("amount-in %q: %w", args[2], err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			response, err := types.NewQueryClient(clientCtx).Quote(cmd.Context(), &types.QueryQuoteRequest{
				Denom: args[0], IsBuy: args[1] == "buy", AmountIn: args[2],
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
