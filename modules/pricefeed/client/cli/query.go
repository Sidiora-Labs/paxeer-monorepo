package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the pricefeed module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(
		CmdParams(),
		CmdPrice(),
		CmdPrices(),
		CmdFeedKey(),
		CmdValidatorByFeedAccount(),
		CmdObservation(),
	)
	return cmd
}

func CmdParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current pricefeed parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			response, err := types.NewQueryClient(clientCtx).Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}
			// Market is hand-defined without protobuf field tags.
			return clientCtx.PrintObjectLegacy(response)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdPrice() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "price [market-id]",
		Short: "Query the price of a market",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			marketID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("market id %q: %w", args[0], err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			response, err := types.NewQueryClient(clientCtx).Price(cmd.Context(), &types.QueryPriceRequest{MarketId: marketID})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(response)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
