package consensus

import (
	"context"
	"fmt"
	"time"

	tmpubsub "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/gogo/protobuf/proto"

	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils/wireguard/wgtest"
	tmcons "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/consensus"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
)

// TestWiring_DataChannel asserts that the consensus message type implements
// WireguardScan and rejects an over-cap Proposal payload.
func TestWiring_DataChannel(t *testing.T) {
	msg := &tmcons.Message{Sum: &tmcons.Message_Proposal{
		Proposal: &tmcons.Proposal{Proposal: tmproto.Proposal{
			LastCommit: wgtest.CommitWith(wgtest.MaxCommitSignatures + 1),
		}},
	}}
	require.Error(t, msg.WireguardScan(wgtest.Marshal(t, msg)),
		"consensus Message.WireguardScan failed to reject an over-cap last_commit")
}

// TestWiring_OtherChannelsAreNoOp documents that State, Vote, and VoteSet
// messages don't reach a Commit path, so WireguardScan is a no-op for them.
func TestWiring_OtherChannelsAreNoOp(t *testing.T) {
	cases := []struct {
		name string
		msg  *tmcons.Message
	}{
		{"State", &tmcons.Message{Sum: &tmcons.Message_NewRoundStep{NewRoundStep: &tmcons.NewRoundStep{}}}},
		{"Vote", &tmcons.Message{Sum: &tmcons.Message_Vote{Vote: &tmcons.Vote{}}}},
		{"VoteSet", &tmcons.Message{Sum: &tmcons.Message_VoteSetBits{VoteSetBits: &tmcons.VoteSetBits{}}}},
	}
	for _, c := range cases {
		t.Run(strings.ReplaceAll(c.name, ".", "_"), func(t *testing.T) {
			require.NoError(t, c.msg.WireguardScan(wgtest.Marshal(t, c.msg)),
				"consensus %s message should be a no-op for WireguardScan", c.name)
		})
	}
}

// TestWiring_AssembledBlock exercises the scanner at the block-parts consumer.
func TestWiring_AssembledBlock(t *testing.T) {
	testRepairM1082AssembledBlock(t)
}

func testRepairM1082AssembledBlock(t *testing.T) {
	t.Helper()
	for _, overLimit := range []bool{true, false} {
		t.Run(fmt.Sprintf("over_limit_%t", overLimit), func(t *testing.T) {
			f := newRepairM1082Fixture(t, false, time.Now().Add(-time.Minute))
			producer, _ := f.state(t, f.leader)
			block, err := producer.createProposalBlock(t.Context())
			require.NoError(t, err)
			require.NotNil(t, block)
			if overLimit {
				// A hostile producer uses the normal block encoder and part/Merkle
				// producer, but declares too many nested commit signatures.
				commit := *block.LastCommit
				commit.Signatures = make([]types.CommitSig, types.MaxVotesCount+1)
				for i := range commit.Signatures {
					commit.Signatures[i] = types.NewCommitSigAbsent()
				}
				block.LastCommit = &commit
			}
			pb, err := block.ToProto()
			require.NoError(t, err)
			encoded, err := proto.Marshal(pb)
			require.NoError(t, err)
			scanErr := tmproto.SchemaForBlock.Scan(encoded)
			if overLimit {
				require.Error(t, scanErr)
			} else {
				require.NoError(t, scanErr)
			}
			// The over-limit payload is structurally valid protobuf: it is the
			// declared nested budget, rather than corrupt framing, that rejects it.
			require.NoError(t, proto.Unmarshal(encoded, new(tmproto.Block)))
			parts, err := block.MakePartSet(types.BlockPartSizeBytes)
			require.NoError(t, err)
			id := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
			cs, _ := f.state(t, f.honest[0])
			cs.SetPrivValidator(t.Context(), utils.None[types.PrivValidator]())
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)
			validBlocks, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{ClientID: "assembled-valid-block", Query: types.EventQueryValidBlock, Limit: 10})
			require.NoError(t, err)
			votes, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{ClientID: "assembled-vote-barrier", Query: types.EventQueryVote, Limit: 10})
			require.NoError(t, err)
			blocks, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{ClientID: "assembled-block", Query: types.EventQueryNewBlock, Limit: 10})
			require.NoError(t, err)
			cs.startTestRound(ctx, 1, 0)
			// A real signed quorum drives enterCommit and installs the expected
			// part-set header. No proposal or private round state is fabricated.
			for signer := range 3 {
				vote := f.vote(t, signer, tmproto.PrecommitType, 1, 0, id)
				mi := repairM1082Transport(t, msgInfo{Msg: &VoteMessage{vote}, PeerID: "assembled-peer", ReceiveTime: time.Now()})
				require.NoError(t, cs.AddVote(ctx, mi.Msg.(*VoteMessage).Vote, mi.PeerID))
			}
			_, err = validBlocks.Next(ctx)
			require.NoError(t, err)
			require.True(t, cs.GetRoundState().ProposalBlockParts.HasHeader(parts.Header()))
			for i := range parts.Total() {
				mi := repairM1082Transport(t, msgInfo{Msg: &BlockPartMessage{Height: 1, Round: 0, Part: parts.GetPart(int(i))}, PeerID: "assembled-peer", ReceiveTime: time.Now()})
				msg := mi.Msg.(*BlockPartMessage)
				require.NoError(t, cs.AddProposalBlockPart(ctx, msg.Height, msg.Round, msg.Part, mi.PeerID))
			}
			if overLimit {
				// Same queue, later signed vote: its event proves the receive
				// routine finished all preceding parts, including the final one.
				barrier := f.vote(t, 0, tmproto.PrevoteType, 1, 1, types.BlockID{})
				require.NoError(t, cs.AddVote(ctx, barrier, "assembled-peer"))
				for {
					event, err := votes.Next(ctx)
					require.NoError(t, err)
					if event.Data().(types.EventDataVote).Vote.Round == 1 {
						break
					}
				}
				cs.mtx.Lock()
				_, err = cs.getBlockFromBlockParts()
				cs.mtx.Unlock()
				require.ErrorIs(t, err, ErrInvalidBlockWire)
				rs := cs.GetRoundState()
				require.True(t, rs.ProposalBlockParts.IsComplete())
				require.Nil(t, rs.Proposal)
				require.Nil(t, rs.ProposalBlock)
				require.EqualValues(t, 1, rs.Height)
				require.Zero(t, cs.blockStore.Height())
			} else {
				event, err := blocks.Next(ctx)
				require.NoError(t, err)
				accepted := event.Data().(types.EventDataNewBlock).Block
				require.Equal(t, id.Hash, accepted.Hash())
				require.EqualValues(t, 1, accepted.Height)
				cs.mtx.RLock()
				commit := cs.blockStore.LoadSeenCommit()
				cs.mtx.RUnlock()
				require.NoError(t, f.genesis.Validators.VerifyCommit(f.genesis.ChainID, id, 1, commit))
			}
		})
	}
}
