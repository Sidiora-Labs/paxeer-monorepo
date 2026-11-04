package rpc

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
)

// BlockTrust is an independently authenticated validator set for one chain height.
// Encode it with consensus/libs/json. Obtain it through a trusted channel, never
// from the node being queried. It does not authorize other heights or rotations.
type BlockTrust struct {
	ChainID      string                `json:"chain_id"`
	Height       int64                 `json:"height,string"`
	ValidatorSet *tmtypes.ValidatorSet `json:"validator_set"`
}

const trustedValidatorSetFlag = "trusted-validator-set"

// BlockCommand authenticates a block against the local BlockTrust file.
// An omitted height or zero requests the latest block; negative heights fail.
func BlockCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block [height]",
		Short: "Get verified block data using an independently trusted validator set",
		Long: `Authenticate the block header and commit using --trusted-validator-set.
The local JSON file contains chain_id, height (a string), and validator_set,
encoded with the consensus JSON codec. Authenticate this file independently of
the queried RPC node. Only its exact chain and height are authorized; validator
rotation requires a new trusted file. Zero or an omitted height requests latest,
which must still match the trusted height. Negative heights are invalid.
The legacy /blocks REST endpoints return unverified node data.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			var height *int64

			// optional height
			if len(args) > 0 {
				h, err := strconv.ParseInt(args[0], 10, 64)
				if err != nil {
					return err
				}
				if h < 0 {
					return fmt.Errorf("block height must not be negative: %d", h)
				}
				if h > 0 {
					height = &h
				}
			}

			trustPath, err := cmd.Flags().GetString(trustedValidatorSetFlag)
			if err != nil {
				return err
			}
			if trustPath == "" {
				return fmt.Errorf("--%s is required for verified block data", trustedValidatorSetFlag)
			}
			data, err := os.ReadFile(trustPath)
			if err != nil {
				return fmt.Errorf("read trusted validator set: %w", err)
			}
			var trust BlockTrust
			if err := tmjson.Unmarshal(data, &trust); err != nil {
				return fmt.Errorf("decode trusted validator set: %w", err)
			}
			output, err := getVerifiedBlock(cmd.Context(), clientCtx, height, trust)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(output))
			return err
		},
	}

	cmd.Flags().String(trustedValidatorSetFlag, "", "Independently authenticated BlockTrust JSON file for the requested height")
	cmd.Flags().StringP(flags.FlagNode, "n", "tcp://localhost:26657", "Node to connect to")

	return cmd
}

func getVerifiedBlock(ctx context.Context, clientCtx client.Context, height *int64, trust BlockTrust) ([]byte, error) {
	if trust.ChainID == "" || trust.Height <= 0 {
		return nil, fmt.Errorf("trusted chain ID and positive height are required")
	}
	if clientCtx.ChainID != "" && clientCtx.ChainID != trust.ChainID {
		return nil, fmt.Errorf("trusted chain ID does not match client chain ID")
	}
	if height != nil && (*height < 0 || (*height > 0 && *height != trust.Height)) {
		return nil, fmt.Errorf("requested height does not match trusted height %d", trust.Height)
	}
	if err := trust.ValidatorSet.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid trusted validator set: %w", err)
	}
	// Bound the total before the consensus verifier computes voting power.
	var power int64
	seen := make(map[string]bool)
	for _, val := range trust.ValidatorSet.Validators {
		if val.VotingPower <= 0 || val.VotingPower > tmtypes.MaxTotalVotingPower-power || seen[string(val.Address)] {
			return nil, fmt.Errorf("invalid trusted validator voting power or duplicate address")
		}
		power += val.VotingPower
		seen[string(val.Address)] = true
	}
	node, err := clientCtx.GetNode()
	if err != nil {
		return nil, err
	}
	res, err := node.Block(ctx, height)
	if err != nil {
		return nil, err
	}
	if res == nil || res.Block == nil {
		return nil, fmt.Errorf("missing block")
	}
	if res.Block.Height != trust.Height {
		return nil, fmt.Errorf("returned block height %d does not match trusted height %d", res.Block.Height, trust.Height)
	}
	if err := res.Block.ValidateBasic(tmtypes.DefaultConsensusPolicy()); err != nil {
		return nil, fmt.Errorf("invalid block: %w", err)
	}
	// Pin the commit query to the returned height even for a latest request.
	commitHeight := res.Block.Height
	signed, err := node.Commit(ctx, &commitHeight)
	if err != nil {
		return nil, err
	}
	if signed == nil {
		return nil, fmt.Errorf("missing signed header and commit")
	}
	if err := signed.SignedHeader.ValidateBasic(trust.ChainID); err != nil {
		return nil, fmt.Errorf("invalid signed header: %w", err)
	}
	if !bytes.Equal(res.Block.Header.Hash(), signed.Header.Hash()) || !res.BlockID.Equals(signed.Commit.BlockID) {
		return nil, fmt.Errorf("block does not match signed header and commit")
	}
	if !bytes.Equal(res.Block.ValidatorsHash, trust.ValidatorSet.Hash()) {
		return nil, fmt.Errorf("block validator hash does not match trusted validator set")
	}
	if err := trust.ValidatorSet.VerifyCommitLightAllSignatures(trust.ChainID, res.BlockID, trust.Height, signed.Commit); err != nil {
		return nil, fmt.Errorf("verify block commit: %w", err)
	}
	return legacy.Cdc.MarshalAsJSON(res)
}

// getBlock returns unverified node data for the legacy REST endpoints.
func getBlock(clientCtx client.Context, height *int64) ([]byte, error) {
	// get the node
	node, err := clientCtx.GetNode()
	if err != nil {
		return nil, err
	}

	// header -> BlockchainInfo
	// header, tx -> Block
	// results -> BlockResults
	res, err := node.Block(context.Background(), height)
	if err != nil {
		return nil, err
	}

	return legacy.Cdc.MarshalAsJSON(res)
}

// get the current blockchain height
func GetChainHeight(ctx context.Context, node client.Client) (int64, error) {
	status, err := node.Status(ctx)
	if err != nil {
		return -1, err
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

// REST handler to get a block
func BlockRequestHandlerFn(clientCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		height, err := strconv.ParseInt(vars["height"], 10, 64)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest,
				"couldn't parse block height. Assumed format is '/block/{height}'.")
			return
		}

		node, err := clientCtx.GetNode()
		if rest.CheckInternalServerError(w, err) {
			return
		}
		chainHeight, err := GetChainHeight(r.Context(), node)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, "failed to parse chain height")
			return
		}

		if height > chainHeight {
			rest.WriteErrorResponse(w, http.StatusNotFound, "requested block height is bigger then the chain length")
			return
		}

		output, err := getBlock(clientCtx, &height)
		if rest.CheckInternalServerError(w, err) {
			return
		}

		rest.PostProcessResponseBare(w, clientCtx, output)
	}
}

// REST handler to get the latest block
func LatestBlockRequestHandlerFn(clientCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		output, err := getBlock(clientCtx, nil)
		if rest.CheckInternalServerError(w, err) {
			return
		}

		rest.PostProcessResponseBare(w, clientCtx, output)
	}
}
