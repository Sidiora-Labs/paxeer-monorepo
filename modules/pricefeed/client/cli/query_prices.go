package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func CmdPrices() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prices [market-ids]",
		Short: "Query prices for up to 64 comma-separated market IDs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := strings.Split(args[0], ",")
			if len(ids) > 64 {
				return fmt.Errorf("prices takes at most 64 market ids")
			}
			request := &types.QueryPricesRequest{MarketIds: make([]uint64, len(ids))}
			for i, id := range ids {
				value, err := strconv.ParseUint(id, 10, 64)
				if err != nil {
					return fmt.Errorf("market id %q: %w", id, err)
				}
				request.MarketIds[i] = value
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			response, err := types.NewQueryClient(clientCtx).Prices(cmd.Context(), request)
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(response)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
