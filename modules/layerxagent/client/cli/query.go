package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Query commands for the layerxagent module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(CmdQueryParams(), CmdQueryJob(), CmdQueryPending())
	return cmd
}

func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the agent scheduler parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryJob() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job [id]",
		Short: "Query a scheduled job by its uint64 id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("the job id %q: %w", args[0], err)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).Job(cmd.Context(), &types.QueryJobRequest{Id: id})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryPending() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending [creator-0x-address]",
		Short: "Query the pending job ids for a creator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			creator := args[0]
			if !strings.HasPrefix(creator, "0x") || !common.IsHexAddress(creator) {
				return fmt.Errorf("the creator %q must be a 0x-prefixed 20-byte hex address", creator)
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).Pending(cmd.Context(), &types.QueryPendingRequest{Creator: creator})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
