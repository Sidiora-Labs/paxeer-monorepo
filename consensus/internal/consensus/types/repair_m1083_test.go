package types

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const repairM1083Chain = "repair-m1083"

type repairM1083Fixture struct {
	validators *tmtypes.ValidatorSet
	signers    []*privval.FilePV
	blockID    tmtypes.BlockID
}

func newRepairM1083Fixture(t *testing.T) repairM1083Fixture {
	t.Helper()
	dir := t.TempDir()
	vals := make([]*tmtypes.Validator, 4)
	byAddress := make(map[string]*privval.FilePV)
	for i := range vals {
		key := ed25519.TestSecretKey([]byte(fmt.Sprintf("repair-m1083-validator-%d", i)))
		pv := privval.NewFilePV(key, filepath.Join(dir, fmt.Sprintf("key-%d.json", i)),
			filepath.Join(dir, fmt.Sprintf("state-%d.json", i)))
		require.NoError(t, pv.Save())
		vals[i] = tmtypes.NewValidator(key.Public(), 1)
		byAddress[string(vals[i].Address)] = pv
	}
	validators := tmtypes.NewValidatorSet(vals)
	signers := make([]*privval.FilePV, len(vals))
	for i := range signers {
		address, _, ok := validators.GetByIndex(int32(i))
		require.True(t, ok)
		signers[i] = byAddress[string(address)]
	}
	return repairM1083Fixture{
		validators: validators,
		signers:    signers,
		blockID: tmtypes.BlockID{
			Hash: crypto.Checksum([]byte("repair-m1083-block")).Bytes(),
			PartSetHeader: tmtypes.PartSetHeader{
				Total: 1,
				Hash:  crypto.Checksum([]byte("repair-m1083-parts")).Bytes(),
			},
		},
	}
}

// Produce signatures with the persistent validator, then use the protobuf wire
// codec and domain decoder used to deliver votes to the height vote set.
func (f repairM1083Fixture) vote(t *testing.T, height int64, round, index int32, voteType tmproto.SignedMsgType) *tmtypes.Vote {
	t.Helper()
	vote := &tmproto.Vote{
		Type:             voteType,
		Height:           height,
		Round:            round,
		BlockID:          f.blockID.ToProto(),
		Timestamp:        time.Unix(1700000000+int64(round), 0).UTC(),
		ValidatorAddress: f.signers[index].GetAddress(),
		ValidatorIndex:   index,
	}
	require.NoError(t, f.signers[index].SignVote(t.Context(), repairM1083Chain, vote))
	return repairM1083WireVote(t, vote)
}

func repairM1083WireVote(t *testing.T, vote *tmproto.Vote) *tmtypes.Vote {
	t.Helper()
	wire, err := vote.Marshal()
	require.NoError(t, err)
	var decoded tmproto.Vote
	require.NoError(t, decoded.Unmarshal(wire))
	result, err := tmtypes.VoteFromProto(&decoded)
	require.NoError(t, err)
	require.NoError(t, result.ValidateBasic())
	return result
}

func (f repairM1083Fixture) finishCatchup(t *testing.T, hvs *HeightVoteSet, round int32) {
	t.Helper()
	for index := int32(1); index < int32(len(f.signers)); index++ {
		vote := f.vote(t, hvs.Height(), round, index, tmproto.PrecommitType)
		added, err := hvs.AddVote(vote, tmtypes.NodeID(fmt.Sprintf("catchup-relay-%d", index)))
		require.NoError(t, err)
		require.True(t, added)
	}
	votes := hvs.Precommits(round)
	require.True(t, votes.HasAll())
	blockID, ok := votes.TwoThirdsMajority()
	require.True(t, ok)
	require.True(t, f.blockID.Equals(blockID))
	commit := votes.MakeCommit()
	require.NoError(t, f.validators.VerifyCommit(repairM1083Chain, f.blockID, hvs.Height(), commit))
}

func TestRepairM_1083_InvalidVotesDoNotAllocate(t *testing.T) {
	f := newRepairM1083Fixture(t)
	hvs := NewHeightVoteSet(repairM1083Chain, 1, f.validators)
	cases := []struct {
		name   string
		mutate func(*tmtypes.Vote)
		err    error
	}{
		{"signature", func(v *tmtypes.Vote) { v.Timestamp = v.Timestamp.Add(time.Second) }, tmtypes.ErrVoteInvalidSignature},
		{"missing-signature", func(v *tmtypes.Vote) { v.Signature = utils.None[crypto.Sig]() }, tmtypes.ErrVoteInvalidSignature},
		{"address", func(v *tmtypes.Vote) { v.ValidatorAddress = f.signers[1].GetAddress() }, tmtypes.ErrVoteInvalidValidatorAddress},
		{"index", func(v *tmtypes.Vote) { v.ValidatorIndex = 4 }, tmtypes.ErrVoteInvalidValidatorIndex},
		{"negative-index", func(v *tmtypes.Vote) { v.ValidatorIndex = -1 }, tmtypes.ErrVoteInvalidValidatorIndex},
		{"height", func(v *tmtypes.Vote) { v.Height++ }, tmtypes.ErrVoteUnexpectedStep},
		{"negative-round", func(v *tmtypes.Vote) { v.Round = -1 }, tmtypes.ErrVoteUnexpectedStep},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vote := f.vote(t, 1, int32(i+1), 0, tmproto.PrecommitType)
			tc.mutate(vote)
			added, err := hvs.AddVote(vote, "reused-peer")
			require.ErrorIs(t, err, tc.err)
			require.False(t, added)
			require.Len(t, hvs.roundVoteSets, 1)
			require.Empty(t, hvs.catchupRounds)
			require.Empty(t, hvs.peerCatchupRounds)
		})
	}
	added, err := hvs.AddVote(nil, "nil-peer")
	require.ErrorIs(t, err, tmtypes.ErrVoteNil)
	require.False(t, added)

	for i := 0; i < 3*maxCatchupRounds; i++ {
		vote := f.vote(t, 1, int32(100+i), 0, tmproto.PrecommitType).ToProto()
		vote.Timestamp = vote.Timestamp.Add(time.Second)
		invalid := repairM1083WireVote(t, vote)
		added, err := hvs.AddVote(invalid, tmtypes.NodeID(fmt.Sprintf("invalid-peer-%d", i)))
		require.ErrorIs(t, err, tmtypes.ErrVoteInvalidSignature)
		require.False(t, added)
		require.Len(t, hvs.roundVoteSets, 1)
		require.Empty(t, hvs.catchupRounds)
		require.Empty(t, hvs.peerCatchupRounds)
	}
	// Invalid submissions must not consume either allowance of this same peer.
	for _, round := range []int32{1000, 1001} {
		added, err := hvs.AddVote(f.vote(t, 1, round, 0, tmproto.PrecommitType), "reused-peer")
		require.NoError(t, err)
		require.True(t, added)
	}
	f.finishCatchup(t, hvs, 1000)
}

func TestRepairM_1083_PeerChurnBoundAndValidCatchup(t *testing.T) {
	f := newRepairM1083Fixture(t)
	hvs := NewHeightVoteSet(repairM1083Chain, 1, f.validators)
	var first *tmtypes.Vote
	for round := int32(1); round <= maxCatchupRounds; round++ {
		voteType := tmproto.PrecommitType
		if round%2 == 0 {
			voteType = tmproto.PrevoteType
		}
		vote := f.vote(t, 1, round, 0, voteType)
		if round == 1 {
			first = vote
		}
		added, err := hvs.AddVote(vote, tmtypes.NodeID(fmt.Sprintf("peer-%d", round)))
		require.NoError(t, err)
		require.True(t, added)
		require.Len(t, hvs.roundVoteSets, int(round)+1)
		require.Len(t, hvs.peerCatchupRounds, int(round))
	}
	require.Len(t, hvs.catchupRounds, maxCatchupRounds)
	retained := hvs.Precommits(1)
	for round := int32(1); round <= maxCatchupRounds; round++ {
		hvs.RemovePeer(tmtypes.NodeID(fmt.Sprintf("peer-%d", round)))
	}
	hvs.RemovePeer("already-disconnected")
	require.Empty(t, hvs.peerCatchupRounds)
	require.Same(t, retained, hvs.Precommits(1))
	for i := 0; i < 2*maxCatchupRounds; i++ {
		round := int32(maxCatchupRounds + 1 + i)
		peerID := tmtypes.NodeID(fmt.Sprintf("reconnected-peer-%d", i))
		added, err := hvs.AddVote(f.vote(t, 1, round, 0, tmproto.PrecommitType), peerID)
		require.ErrorIs(t, err, ErrCatchupRoundLimit)
		require.False(t, added)
		hvs.RemovePeer(peerID)
		require.Nil(t, hvs.Precommits(round))
		require.Nil(t, hvs.Prevotes(round))
		require.Len(t, hvs.roundVoteSets, maxCatchupRounds+1)
		require.Len(t, hvs.catchupRounds, maxCatchupRounds)
		require.Empty(t, hvs.peerCatchupRounds)
	}
	// New relays can finish an admitted catchup round even at the global limit.
	added, err := hvs.AddVote(first, "new-relay")
	require.NoError(t, err)
	require.False(t, added)
	f.finishCatchup(t, hvs, 1)
	require.Empty(t, hvs.peerCatchupRounds)
	hvs.SetRound(1)
	require.Same(t, retained, hvs.Precommits(1))
	require.True(t, retained.IsCommit())
	require.Len(t, hvs.catchupRounds, maxCatchupRounds-1)
	added, err = hvs.AddVote(f.vote(t, 1, 1000, 0, tmproto.PrecommitType), "peer-1")
	require.NoError(t, err)
	require.True(t, added)
	require.Len(t, hvs.catchupRounds, maxCatchupRounds)
}

func TestRepairM_1083_RoundRetirementAndHeightReset(t *testing.T) {
	f := newRepairM1083Fixture(t)
	hvs := NewHeightVoteSet(repairM1083Chain, 1, f.validators)
	for _, round := range []int32{1, 4} {
		added, err := hvs.AddVote(f.vote(t, 1, round, 0, tmproto.PrecommitType), "peer")
		require.NoError(t, err)
		require.True(t, added)
	}
	retained := hvs.Precommits(1)
	hvs.SetRound(1)
	require.Equal(t, []int32{4}, hvs.peerCatchupRounds["peer"])
	require.Len(t, hvs.catchupRounds, 1)
	added, err := hvs.AddVote(f.vote(t, 1, 5, 0, tmproto.PrecommitType), "peer")
	require.NoError(t, err)
	require.True(t, added)
	added, err = hvs.AddVote(f.vote(t, 1, 6, 0, tmproto.PrecommitType), "peer")
	require.ErrorIs(t, err, ErrGotVoteFromUnwantedRound)
	require.False(t, added)
	require.Nil(t, hvs.Precommits(6))
	hvs.SetRound(5)
	require.Empty(t, hvs.peerCatchupRounds)
	require.Empty(t, hvs.catchupRounds)
	require.Same(t, retained, hvs.Precommits(1))
	f.finishCatchup(t, hvs, 1)
	hvs.Reset(2, f.validators)
	require.Equal(t, int64(2), hvs.Height())
	require.Zero(t, hvs.Round())
	require.Len(t, hvs.roundVoteSets, 1)
	require.Empty(t, hvs.peerCatchupRounds)
	require.Empty(t, hvs.catchupRounds)
	require.Nil(t, hvs.Precommits(1))
	added, err = hvs.AddVote(f.vote(t, 2, 1, 0, tmproto.PrecommitType), "peer")
	require.NoError(t, err)
	require.True(t, added)
	f.finishCatchup(t, hvs, 1)
}
