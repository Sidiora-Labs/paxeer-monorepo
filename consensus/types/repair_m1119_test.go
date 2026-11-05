package types_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/statesync"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	pb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/statesync"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
)

func TestRepairM_1119_ValidatorPowerBounds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		powers  []int64
		wantErr error
	}{
		{"single_over_limit", []int64{types.MaxTotalVotingPower + 1}, types.ErrTotalVotingPowerOverflow},
		{"single_int64_max", []int64{math.MaxInt64}, types.ErrTotalVotingPowerOverflow},
		{"sum_one_over_limit", []int64{types.MaxTotalVotingPower, 1}, types.ErrTotalVotingPowerOverflow},
		{"sum_over_limit", []int64{types.MaxTotalVotingPower/2 + 1, types.MaxTotalVotingPower/2 + 1}, types.ErrTotalVotingPowerOverflow},
		{"int64_sum_overflow", []int64{math.MaxInt64, math.MaxInt64}, types.ErrTotalVotingPowerOverflow},
		{"negative_power", []int64{-1, 10}, types.ErrNegativeVotingPower},
		{"negative_cannot_offset_power", []int64{-types.MaxTotalVotingPower, types.MaxTotalVotingPower + 1}, types.ErrNegativeVotingPower},
		{"int64_sum_underflow", []int64{math.MinInt64, math.MinInt64}, types.ErrNegativeVotingPower},
		{"exact_limit", []int64{types.MaxTotalVotingPower - 1, 1}, nil},
		{"single_exact_limit", []int64{types.MaxTotalVotingPower}, nil},
		{"ordinary", []int64{10, 20}, nil},
		{"zero_power", []int64{0, 0}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, claimedTotal := range []int64{0, -1, math.MaxInt64} {
				wire := repairM1119LightBlock(t)
				wire.ValidatorSet.Validators = wire.ValidatorSet.Validators[:len(tc.powers)]
				for i, power := range tc.powers {
					wire.ValidatorSet.Validators[i].VotingPower = power
				}
				wire.ValidatorSet.Proposer = wire.ValidatorSet.Validators[0]
				wire.ValidatorSet.TotalVotingPower = claimedTotal
				wire = repairM1119WireRoundTrip(t, wire)

				var decoded *types.ValidatorSet
				var err error
				require.NotPanics(t, func() { decoded, err = types.ValidatorSetFromProto(wire.ValidatorSet) })
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
					require.Nil(t, decoded)
					var block *types.LightBlock
					require.NotPanics(t, func() { block, err = types.LightBlockFromProto(wire) })
					require.ErrorIs(t, err, tc.wantErr)
					require.Nil(t, block)
					continue
				}
				require.NoError(t, err)
				var total int64
				for _, power := range tc.powers {
					total += power
				}
				require.Equal(t, total, decoded.TotalVotingPower())
				require.NoError(t, decoded.ValidateBasic())
				block, err := types.LightBlockFromProto(wire)
				require.NoError(t, err)
				require.Equal(t, total, block.ValidatorSet.TotalVotingPower())
			}
		})
	}
}

func TestRepairM_1119_IndividualValidatorValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*tmproto.ValidatorSet)
		wantErr error
	}{
		{"invalid_address", func(v *tmproto.ValidatorSet) { v.Validators[0].Address = nil }, types.ErrBadAddressSize},
		{"negative_proposer", func(v *tmproto.ValidatorSet) { v.Proposer.VotingPower = -1 }, types.ErrNegativeVotingPower},
		{"nil_validator", func(v *tmproto.ValidatorSet) { v.Validators[0] = nil }, nil},
		{"nil_proposer", func(v *tmproto.ValidatorSet) { v.Proposer = nil }, nil},
		{"empty_set", func(v *tmproto.ValidatorSet) { v.Validators = nil }, types.ErrValidatorSetEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := repairM1119LightBlock(t)
			tc.mutate(wire.ValidatorSet)
			var err error
			require.NotPanics(t, func() { _, err = types.ValidatorSetFromProto(wire.ValidatorSet) })
			require.Error(t, err)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

func TestRepairM_1119_DispatcherRejectsOversizedResponse(t *testing.T) {
	good := repairM1119LightBlock(t)
	bad := repairM1119LightBlock(t)
	bad.ValidatorSet.Validators[0].VotingPower = types.MaxTotalVotingPower
	bad.ValidatorSet.Validators[1].VotingPower = 1
	bad.ValidatorSet.TotalVotingPower = 1

	network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 2})
	nodes := network.Nodes()
	local, err := p2p.OpenChannel(nodes[0].Router, statesync.GetLightBlockChannelDescriptor())
	require.NoError(t, err)
	remote, err := p2p.OpenChannel(nodes[1].Router, statesync.GetLightBlockChannelDescriptor())
	require.NoError(t, err)
	network.Start(t)
	dispatcher := statesync.NewDispatcher(local)
	t.Cleanup(dispatcher.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.Spawn(func() error {
			request, err := remote.Recv(ctx)
			if err != nil {
				return err
			}
			if request.From != nodes[0].NodeID || request.Message.GetLightBlockRequest().Height != uint64(good.SignedHeader.Header.Height) {
				return fmt.Errorf("unexpected light block request: %v", request)
			}
			for _, block := range []*tmproto.LightBlock{bad, good} {
				remote.Send(&pb.Message{Sum: &pb.Message_LightBlockResponse{
					LightBlockResponse: &pb.LightBlockResponse{LightBlock: block},
				}}, request.From)
			}
			return nil
		})
		s.Spawn(func() error {
			for i := range 2 {
				response, err := local.Recv(ctx)
				if err != nil {
					return err
				}
				if response.From != nodes[1].NodeID {
					return fmt.Errorf("unexpected response peer: %v", response.From)
				}
				err = dispatcher.Respond(ctx, response.Message.GetLightBlockResponse().LightBlock, response.From)
				if i == 0 {
					if !errors.Is(err, types.ErrTotalVotingPowerOverflow) {
						return fmt.Errorf("oversized response: expected power overflow, got %v", err)
					}
				} else if err != nil {
					return err
				}
			}
			return nil
		})
		block, err := dispatcher.LightBlock(ctx, good.SignedHeader.Header.Height, nodes[1].NodeID)
		if err != nil {
			return err
		}
		if block == nil {
			return errors.New("no valid response delivered")
		}
		if block.ValidatorSet.TotalVotingPower() != 20 {
			return fmt.Errorf("invalid response delivered: power %d", block.ValidatorSet.TotalVotingPower())
		}
		if err := block.ValidateBasic(good.SignedHeader.Header.ChainID); err != nil {
			return err
		}
		return block.ValidatorSet.VerifyCommitLight(block.ChainID, block.Commit.BlockID, block.Height, block.Commit)
	})
	require.NoError(t, err)
}

func repairM1119WireRoundTrip(t *testing.T, block *tmproto.LightBlock) *tmproto.LightBlock {
	t.Helper()
	encoded, err := block.Marshal()
	require.NoError(t, err)
	var decoded tmproto.LightBlock
	require.NoError(t, decoded.Unmarshal(encoded))
	return &decoded
}

func repairM1119LightBlock(t *testing.T) *tmproto.LightBlock {
	t.Helper()
	keys := []ed25519.SecretKey{
		ed25519.TestSecretKey([]byte("repair-m1119-validator-one")),
		ed25519.TestSecretKey([]byte("repair-m1119-validator-two")),
	}
	vals := types.NewValidatorSet([]*types.Validator{
		types.NewValidator(keys[0].Public(), 10), types.NewValidator(keys[1].Public(), 10),
	})
	header := &types.Header{
		Version: version.Consensus{Block: version.BlockProtocol},
		ChainID: "repair-m1119", Height: 7, Time: time.Unix(1700000000, 0).UTC(),
		ValidatorsHash: vals.Hash(), NextValidatorsHash: vals.Hash(), ProposerAddress: vals.Proposer.Address,
	}
	blockID := types.BlockID{Hash: header.Hash(), PartSetHeader: types.PartSetHeader{Total: 1, Hash: header.Hash()}}
	commit := &types.Commit{Height: header.Height, BlockID: blockID}
	for i, val := range vals.Validators {
		vote := &types.Vote{
			Type: tmproto.PrecommitType, Height: header.Height, BlockID: blockID,
			Timestamp: header.Time, ValidatorAddress: val.Address, ValidatorIndex: int32(i),
		}
		for _, key := range keys {
			if key.Public() == val.PubKey {
				vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(header.ChainID, vote.ToProto())))
			}
		}
		commit.Signatures = append(commit.Signatures, vote.CommitSig())
	}
	block := &types.LightBlock{SignedHeader: &types.SignedHeader{Header: header, Commit: commit}, ValidatorSet: vals}
	require.NoError(t, block.ValidateBasic(header.ChainID))
	require.NoError(t, vals.VerifyCommitLight(header.ChainID, blockID, header.Height, commit))
	wire, err := block.ToProto()
	require.NoError(t, err)
	return repairM1119WireRoundTrip(t, wire)
}
