package cli

import (
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/spf13/cobra"
)

func CmdFeedKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed-key [validator]",
		Short: "Query a validator's registered pricefeed key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			validator, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).FeedKey(cmd.Context(), &types.QueryFeedKeyRequest{
				Validator: validator.String(),
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

func CmdValidatorByFeedAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator-by-feed-account [feed-account]",
		Short: "Query the validator registered to a pricefeed account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).ValidatorByFeedAccount(cmd.Context(), &types.QueryValidatorByFeedAccountRequest{
				FeedAccount: account.String(),
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

func CmdObservation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observation [market-id] [validator]",
		Short: "Query a validator's latest observation for a market",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			marketID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			validator, err := sdk.ValAddressFromBech32(args[1])
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(clientCtx).Observation(cmd.Context(), &types.QueryObservationRequest{
				MarketId: marketID, Validator: validator.String(),
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
