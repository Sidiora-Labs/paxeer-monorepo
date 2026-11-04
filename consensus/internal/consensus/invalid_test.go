package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bits"
	tmcons "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/consensus"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
)

// Test checking that if peer sends a ProposalPOLMessage with a bitarray with bad length,
// the node will handle it gracefully.
func TestGossipVotesForHeightPoisonedProposalPOL(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	cfg := configSetup(t)
	states, cleanup := makeConsensusState(ctx, t, cfg, 2, "consensus_reactor_test", newMockTickerFunc(true))
	t.Cleanup(cleanup)

	rts := setup(ctx, t, 2, unwrapTestStates(states), 1)

	var nodeIDs []types.NodeID
	for _, node := range rts.network.Nodes() {
		nodeIDs = append(nodeIDs, node.NodeID)
	}
	require.Len(t, nodeIDs, 2)

	reactor := rts.reactors[nodeIDs[0]]
	peerID := nodeIDs[1]
	state := reactor.state.GetState()
	reactor.SwitchToConsensus(state, false)

	require.Eventually(t, func() bool {
		_, ok := reactor.GetPeerState(peerID)
		return ok
	}, time.Hour, 50*time.Millisecond)

	valSet, privVals := types.RandValidatorSet(4, 1)
	proposerPubKey, err := privVals[0].GetPubKey(ctx)
	require.NoError(t, err)

	proposal := types.NewProposal(
		1,
		1,
		0,
		types.BlockID{
			Hash: crypto.CRandBytes(crypto.HashSize),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  crypto.CRandBytes(crypto.HashSize),
			},
		},
		time.Now(),
		nil,
		types.Header{
			Version:         version.Consensus{Block: version.BlockProtocol},
			Height:          1,
			ProposerAddress: proposerPubKey.Address(),
		},
		&types.Commit{},
		nil,
		proposerPubKey.Address(),
	)
	proposal.Signature = makeSig("invalid-signature")

	require.NoError(t, reactor.handleStateMessage(p2p.RecvMsg[*tmcons.Message]{
		From: peerID,
		Message: MsgToProto(&NewRoundStepMessage{
			HRS: cstypes.HRS{
				Height: 1,
				Round:  1,
				Step:   cstypes.RoundStepPrevote,
			},
			SecondsSinceStartTime: 1,
			LastCommitRound:       -1,
		}),
	}))

	require.NoError(t, reactor.handleDataMessage(ctx, p2p.RecvMsg[*tmcons.Message]{
		From: peerID,
		Message: MsgToProto(&ProposalMessage{
			Proposal: proposal,
		}),
	}))

	require.NoError(t, reactor.handleDataMessage(ctx, p2p.RecvMsg[*tmcons.Message]{
		From: peerID,
		Message: MsgToProto(&ProposalPOLMessage{
			Height:           1,
			ProposalPOLRound: 0,
			ProposalPOL:      bits.NewBitArray(1),
		}),
	}))

	ps, ok := reactor.GetPeerState(peerID)
	require.True(t, ok)
	prs := ps.GetRoundState()
	require.Equal(t, int64(1), prs.Height)
	require.Equal(t, int32(1), prs.Round)
	require.Equal(t, int32(0), prs.ProposalPOLRound)
	require.Equal(t, 1, prs.ProposalPOL.Size())

	voteSet := cstypes.NewHeightVoteSet("test-chain", 1, valSet)
	voteSet.SetRound(1)

	voter := newValidatorStub(privVals[1], 1)
	voter.Height = 1
	voter.Round = 0

	vote := signVote(ctx, t, voter, tmproto.PrevoteType, "test-chain", types.BlockID{
		Hash: crypto.CRandBytes(crypto.HashSize),
		PartSetHeader: types.PartSetHeader{
			Total: 1,
			Hash:  crypto.CRandBytes(crypto.HashSize),
		},
	})
	added, err := voteSet.AddVote(vote, "")
	require.NoError(t, err)
	require.True(t, added)

	rs := &cstypes.RoundState{
		HRS: cstypes.HRS{
			Height: 1,
			Round:  1,
			Step:   cstypes.RoundStepPrevote,
		},
		Votes: voteSet,
	}

	// Gossip should take into consideration that PeerState might contain
	// invalid length bitarrays.
	for range 10 {
		reactor.gossipVotesForHeight(rs, ps.GetRoundState(), ps)
	}
}

func TestReactorInvalidPrecommit(t *testing.T) {
	testRepairM1082InvalidPrecommit(t)
}
