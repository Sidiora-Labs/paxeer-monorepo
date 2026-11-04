package cli

import (
	"fmt"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

func CmdQueryAirdropEpoch() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "airdrop-epoch [pool] [epoch]",
		Example: "paxd query launchpad airdrop-epoch 0x0000000000000000000000000000000000000001 1 --output json",
		Short:   "Query a launchpad pool's airdrop epoch",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			epoch, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("epoch %q: %w", args[1], err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			response, err := queryClient.AirdropEpoch(cmd.Context(), &types.QueryAirdropEpochRequest{
				Pool: args[0], Epoch: epoch,
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

func CmdQueryAirdropClaimed() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "airdrop-claimed [pool] [holder] [epoch]",
		Example: "paxd query launchpad airdrop-claimed 0x0000000000000000000000000000000000000001 0x0000000000000000000000000000000000000002 1 --output json",
		Short:   "Query whether a holder claimed a launchpad pool's airdrop",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			epoch, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("epoch %q: %w", args[2], err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			response, err := queryClient.AirdropClaimed(cmd.Context(), &types.QueryAirdropClaimedRequest{
				Pool: args[0], Holder: args[1], Epoch: epoch,
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
