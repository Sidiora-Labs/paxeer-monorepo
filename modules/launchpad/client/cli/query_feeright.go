package cli

import (
	"fmt"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

func CmdQueryFeeRight() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fee-right [collection] [id]",
		Short: "Query a launchpad fee right by collection and ID",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("id %q: %w", args[1], err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			response, err := queryClient.FeeRight(cmd.Context(), &types.QueryFeeRightRequest{
				Collection: args[0], Id: id,
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

func CmdQueryPoolMetadata() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool-metadata [pool]",
		Short: "Query a launchpad pool's metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			response, err := queryClient.PoolMetadata(cmd.Context(), &types.QueryPoolMetadataRequest{Pool: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(response)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
