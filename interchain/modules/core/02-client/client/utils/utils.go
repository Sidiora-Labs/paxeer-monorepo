package utils

import (
	"bytes"
	"context"
	"fmt"
	"math"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibcclient "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
)

// QueryClientState returns a client state. If prove is true, it performs an ABCI store query
// in order to retrieve the merkle proof. Otherwise, it uses the gRPC query client.
func QueryClientState(
	clientCtx client.Context, clientID string, prove bool,
) (*types.QueryClientStateResponse, error) {
	if prove {
		return QueryClientStateABCI(clientCtx, clientID)
	}

	queryClient := types.NewQueryClient(clientCtx)
	req := &types.QueryClientStateRequest{
		ClientId: clientID,
	}

	return queryClient.ClientState(context.Background(), req)
}

// QueryClientStateABCI queries the store to get the light client state and a merkle proof.
func QueryClientStateABCI(
	clientCtx client.Context, clientID string,
) (*types.QueryClientStateResponse, error) {
	key := host.FullClientStateKey(clientID)

	value, proofBz, proofHeight, err := ibcclient.QueryTendermintProof(clientCtx, key)
	if err != nil {
		return nil, err
	}

	// check if client exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrap(types.ErrClientNotFound, clientID)
	}

	cdc := codec.NewProtoCodec(clientCtx.InterfaceRegistry)

	clientState, err := types.UnmarshalClientState(cdc, value)
	if err != nil {
		return nil, err
	}

	anyClientState, err := types.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	clientStateRes := types.NewQueryClientStateResponse(anyClientState, proofBz, proofHeight)
	return clientStateRes, nil
}

// QueryConsensusState returns a consensus state. If prove is true, it performs an ABCI store
// query in order to retrieve the merkle proof. Otherwise, it uses the gRPC query client.
func QueryConsensusState(
	clientCtx client.Context, clientID string, height exported.Height, prove, latestHeight bool,
) (*types.QueryConsensusStateResponse, error) {
	if prove {
		if !latestHeight {
			return QueryConsensusStateABCI(clientCtx, clientID, height)
		}

		// The client proof selects a store snapshot as well as the latest
		// consensus height. Reuse that snapshot even if the chain advances.
		clientRes, err := QueryClientStateABCI(clientCtx, clientID)
		if err != nil {
			return nil, err
		}
		clientState, err := types.UnpackClientState(clientRes.ClientState)
		if err != nil {
			return nil, err
		}
		if clientRes.ProofHeight.RevisionHeight > math.MaxInt64 {
			return nil, fmt.Errorf("proof height exceeds supported query height: %s", clientRes.ProofHeight)
		}
		pinnedCtx := clientCtx.WithHeight(int64(clientRes.ProofHeight.RevisionHeight))
		res, err := QueryConsensusStateABCI(pinnedCtx, clientID, clientState.GetLatestHeight())
		if err != nil {
			return nil, err
		}
		if res.ProofHeight != clientRes.ProofHeight {
			return nil, fmt.Errorf("consensus proof snapshot %s differs from client snapshot %s", res.ProofHeight, clientRes.ProofHeight)
		}
		return res, nil
	}

	queryClient := types.NewQueryClient(clientCtx)
	req := &types.QueryConsensusStateRequest{
		ClientId:       clientID,
		RevisionNumber: height.GetRevisionNumber(),
		RevisionHeight: height.GetRevisionHeight(),
		LatestHeight:   latestHeight,
	}

	return queryClient.ConsensusState(context.Background(), req)
}

// QueryConsensusStateABCI queries the store to get the consensus state of a light client and a
// merkle proof of its existence or non-existence.
func QueryConsensusStateABCI(
	clientCtx client.Context, clientID string, height exported.Height,
) (*types.QueryConsensusStateResponse, error) {
	key := host.FullConsensusStateKey(clientID, height)

	value, proofBz, proofHeight, err := ibcclient.QueryTendermintProof(clientCtx, key)
	if err != nil {
		return nil, err
	}

	// check if consensus state exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrap(types.ErrConsensusStateNotFound, clientID)
	}

	cdc := codec.NewProtoCodec(clientCtx.InterfaceRegistry)

	cs, err := types.UnmarshalConsensusState(cdc, value)
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := types.PackConsensusState(cs)
	if err != nil {
		return nil, err
	}

	return types.NewQueryConsensusStateResponse(anyConsensusState, proofBz, proofHeight), nil
}

// QueryTendermintHeader takes a client context and returns the appropriate
// tendermint header
func QueryTendermintHeader(clientCtx client.Context) (ibctmtypes.Header, int64, error) {
	node, err := clientCtx.GetNode()
	if err != nil {
		return ibctmtypes.Header{}, 0, err
	}

	info, err := node.ABCIInfo(context.Background())
	if err != nil {
		return ibctmtypes.Header{}, 0, err
	}

	var height int64
	if clientCtx.Height != 0 {
		height = clientCtx.Height
	} else {
		height = info.Response.LastBlockHeight
	}

	commit, err := node.Commit(context.Background(), &height)
	if err != nil {
		return ibctmtypes.Header{}, 0, err
	}

	if commit == nil || commit.Header == nil || commit.Height != height {
		return ibctmtypes.Header{}, 0, fmt.Errorf("commit does not contain the requested header at height %d", height)
	}

	validators, err := queryValidatorSet(node, height, commit.ValidatorsHash)
	if err != nil {
		return ibctmtypes.Header{}, 0, err
	}

	protoCommit := commit.ToProto()
	protoValset, err := validators.ToProto()
	if err != nil {
		return ibctmtypes.Header{}, 0, err
	}

	header := ibctmtypes.Header{
		SignedHeader: protoCommit,
		ValidatorSet: protoValset,
	}

	return header, height, nil
}

// QuerySelfConsensusState takes a client context and returns the appropriate
// tendermint consensus state
func QuerySelfConsensusState(clientCtx client.Context) (*ibctmtypes.ConsensusState, int64, error) {
	node, err := clientCtx.GetNode()
	if err != nil {
		return &ibctmtypes.ConsensusState{}, 0, err
	}

	info, err := node.ABCIInfo(context.Background())
	if err != nil {
		return &ibctmtypes.ConsensusState{}, 0, err
	}

	var height int64
	if clientCtx.Height != 0 {
		height = clientCtx.Height
	} else {
		height = info.Response.LastBlockHeight
	}

	commit, err := node.Commit(context.Background(), &height)
	if err != nil {
		return &ibctmtypes.ConsensusState{}, 0, err
	}

	if commit == nil || commit.Header == nil || commit.Height != height {
		return &ibctmtypes.ConsensusState{}, 0, fmt.Errorf("commit does not contain the requested header at height %d", height)
	}
	if height == math.MaxInt64 {
		return &ibctmtypes.ConsensusState{}, 0, fmt.Errorf("next validator height overflows at %d", height)
	}

	nextVals, err := queryValidatorSet(node, height+1, commit.NextValidatorsHash)
	if err != nil {
		return &ibctmtypes.ConsensusState{}, 0, err
	}

	state := &ibctmtypes.ConsensusState{
		Timestamp:          commit.Time,
		Root:               commitmenttypes.NewMerkleRoot(commit.AppHash),
		NextValidatorsHash: nextVals.Hash(),
	}

	return state, height, nil
}

// queryValidatorSet collects the complete set at one height using the safe RPC
// page limit, then checks it against the hash committed by the selected header.
func queryValidatorSet(node client.Client, height int64, expectedHash []byte) (*tmtypes.ValidatorSet, error) {
	const perPage = 100
	var validators []*tmtypes.Validator
	seen := make(map[string]struct{})
	total := 0
	var power int64
	for page := 1; ; page++ {
		count := perPage
		res, err := node.Validators(context.Background(), &height, &page, &count)
		if err != nil {
			return nil, err
		}
		if res == nil || res.BlockHeight != height {
			return nil, fmt.Errorf("validator page %d does not match height %d", page, height)
		}
		if page == 1 {
			total = res.Total
		}
		if total <= 0 || res.Total != total {
			return nil, fmt.Errorf("inconsistent validator total on page %d: got %d, expected %d", page, res.Total, total)
		}
		want := min(perPage, total-len(validators))
		if want <= 0 || res.Count != len(res.Validators) || len(res.Validators) != want {
			return nil, fmt.Errorf("incomplete validator page %d: count %d, validators %d, expected %d", page, res.Count, len(res.Validators), want)
		}
		for _, val := range res.Validators {
			if err := val.ValidateBasic(); err != nil {
				return nil, fmt.Errorf("invalid validator on page %d: %w", page, err)
			}
			if !bytes.Equal(val.Address, val.PubKey.Address()) || val.VotingPower <= 0 || val.VotingPower > tmtypes.MaxTotalVotingPower-power {
				return nil, fmt.Errorf("invalid validator key or voting power on page %d", page)
			}
			if _, exists := seen[string(val.Address)]; exists {
				return nil, fmt.Errorf("duplicate validator on page %d: %X", page, val.Address)
			}
			seen[string(val.Address)] = struct{}{}
			power += val.VotingPower
			validators = append(validators, val)
		}
		if len(validators) == total {
			break
		}
	}
	set := tmtypes.NewValidatorSet(validators)
	if !bytes.Equal(set.Hash(), expectedHash) {
		return nil, fmt.Errorf("validator set hash at height %d does not match committed hash: got %X, expected %X", height, set.Hash(), expectedHash)
	}
	return set, nil
}
