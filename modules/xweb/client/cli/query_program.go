package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func NewQueryProgramRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program-request [program-id] [request-id]",
		Short: "Query a web request made by a program",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			programID, requestID, err := parseProgramQueryArgs(args)
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).ProgramRequest(cmd.Context(), &types.QueryProgramRequestRequest{
				ProgramId: programID,
				RequestId: requestID,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryProgramResultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program-result [program-id] [request-id]",
		Short: "Query the result of a program's web request",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			programID, requestID, err := parseProgramQueryArgs(args)
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).ProgramResult(cmd.Context(), &types.QueryProgramResultRequest{
				ProgramId: programID,
				RequestId: requestID,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryProgramCountersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program-counters",
		Short: "Query the pending and committed program web request counters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).ProgramCounters(cmd.Context(), &types.QueryProgramCountersRequest{})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func parseProgramQueryArgs(args []string) ([]byte, uint64, error) {
	if !strings.HasPrefix(args[0], "0x") {
		return nil, 0, fmt.Errorf("the program id must be 0x-prefixed")
	}
	programID, err := parseHexBytes(args[0])
	if err != nil {
		return nil, 0, fmt.Errorf("the program id %q: %w", args[0], err)
	}
	if len(programID) != 32 {
		return nil, 0, fmt.Errorf("the program id must be exactly 32 bytes (got %d)", len(programID))
	}
	requestID, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("the request id %q: %w", args[1], err)
	}
	return programID, requestID, nil
}
