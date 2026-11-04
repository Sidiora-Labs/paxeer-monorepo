package consensus

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	logwal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/libs/wal"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/test/factory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmcons "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/consensus"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repair80Fixture struct {
	genesis   sm.State
	cfg       *config.Config
	signer    *privval.FilePV
	keyFile   string
	stateFile string
	messages  []msgInfo
	proposal  *types.Proposal
}

func newRepair80Fixture(t *testing.T) repair80Fixture {
	t.Helper()
	ctx := t.Context()
	cfg := getConfig(t)
	var validators []*types.Validator
	var signers []*privval.FilePV
	var keyFiles, stateFiles []string
	for i := range 4 {
		dir := t.TempDir()
		keyFile, stateFile := filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json")
		key := ed25519.TestSecretKey([]byte(fmt.Sprintf("%s/%d", t.Name(), i)))
		signer := privval.NewFilePV(key, keyFile, stateFile)
		require.NoError(t, signer.Save())
		signers = append(signers, signer)
		keyFiles = append(keyFiles, keyFile)
		stateFiles = append(stateFiles, stateFile)
		validators = append(validators, types.NewValidator(key.Public(), 10))
	}
	params := factory.ConsensusParams()
	params.Synchrony = types.SynchronyParams{Precision: time.Second, MessageDelay: time.Second}
	genesis, err := sm.MakeGenesisState(factory.GenesisDoc(cfg, time.Now().Add(-time.Hour), validators, params))
	require.NoError(t, err)
	genesis.Version.Consensus.App = kvstore.ProtocolVersion
	rs := &cstypes.RoundState{HRS: cstypes.HRS{Height: 1}, Validators: genesis.Validators}
	leader := rs.Leader()
	leaderIndex := -1
	for i, signer := range signers {
		if signer.Key.PubKey == leader {
			leaderIndex = i
		}
	}
	require.NotEqual(t, -1, leaderIndex)
	localIndex := (leaderIndex + 1) % len(signers)
	producer := newStateWithConfig(t, getConfig(t), genesis, signers[leaderIndex], kvstore.NewProxy())
	t.Cleanup(producer.blockExec.Stop)
	// Run the production proposal builder and file-backed signer, then take
	// exactly the messages it emits for transport (proposal and all block parts).
	producer.State.decideProposal(ctx, 1, 0, signers[leaderIndex], leader)
	first := repair80NextMessage(t, producer)
	proposalMessage, ok := first.Msg.(*ProposalMessage)
	require.True(t, ok)
	require.False(t, first.ReceiveTime.IsZero())
	messages := []msgInfo{first}
	for range proposalMessage.Proposal.BlockID.PartSetHeader.Total {
		part := repair80NextMessage(t, producer)
		require.IsType(t, &BlockPartMessage{}, part.Msg)
		messages = append(messages, part)
	}
	for i := range messages {
		encoded, err := proto.Marshal(MsgToProto(messages[i].Msg))
		require.NoError(t, err)
		var transported tmcons.Message
		require.NoError(t, proto.Unmarshal(encoded, &transported))
		messages[i].Msg, err = MsgFromProto(&transported)
		require.NoError(t, err)
		messages[i].PeerID = types.NodeID("repair80-proposer")
	}
	return repair80Fixture{
		genesis: genesis, cfg: cfg, signer: signers[localIndex],
		keyFile: keyFiles[localIndex], stateFile: stateFiles[localIndex],
		messages: messages, proposal: proposalMessage.Proposal,
	}
}

func repair80NextMessage(t *testing.T, cs *testState) msgInfo {
	t.Helper()
	select {
	case mi := <-cs.internalMsgQueue:
		return mi
	default:
		t.Fatal("production consensus path did not emit its expected message")
		return msgInfo{}
	}
}

func repair80State(t *testing.T, f repair80Fixture, signer *privval.FilePV) *testState {
	t.Helper()
	cs := newStateWithConfig(t, f.cfg, f.genesis, signer, kvstore.NewProxy())
	t.Cleanup(cs.blockExec.Stop)
	cs.enterNewRound(t.Context(), 1, 0, "repair80")
	return cs
}

func repair80Prevote(t *testing.T, cs *testState, f repair80Fixture, timely bool) *types.Vote {
	t.Helper()
	require.NotNil(t, cs.roundState.Proposal())
	require.NotNil(t, cs.roundState.ProposalBlock())
	require.Equal(t, timely, cs.proposalIsTimely())
	message := repair80NextMessage(t, cs)
	voteMessage, ok := message.Msg.(*VoteMessage)
	require.True(t, ok)
	vote := voteMessage.Vote
	require.Equal(t, tmproto.PrevoteType, vote.Type)
	require.EqualValues(t, 1, vote.Height)
	require.Zero(t, vote.Round)
	require.NoError(t, vote.Verify(f.genesis.ChainID, f.signer.Key.PubKey))
	if timely {
		require.True(t, vote.BlockID.Equals(f.proposal.BlockID))
	} else {
		require.True(t, vote.BlockID.IsNil())
	}
	return vote
}

func TestRepair_80_WALProposalReceiveTime(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset time.Duration
		timely bool
	}{
		{"timely", 1234567 * time.Nanosecond, true},
		{"upper_boundary", 2 * time.Second, true},
		{"late", 2*time.Second + time.Nanosecond, false},
		{"early", -time.Second - time.Nanosecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepair80Fixture(t)
			// The receiver's delivery time is a separate PBTS input; it need not
			// equal either the proposal's signed timestamp or the WAL append clock.
			received := f.proposal.Timestamp.Add(tc.offset)
			f.messages[0].ReceiveTime = received
			live := repair80State(t, f, f.signer)
			for _, mi := range f.messages {
				require.NoError(t, live.wal.Append(NewWALMessage(mi)))
			}
			require.NoError(t, live.wal.Sync())
			for _, mi := range f.messages {
				live.handleMsg(t.Context(), mi, false)
			}
			before := repair80Prevote(t, live, f, tc.timely)
			require.True(t, received.Equal(live.roundState.ProposalReceiveTime()))
			// Crash after signing but before the vote is appended to WAL. Reload
			// the same FilePV state, with no reset of its double-sign protection.
			live.wal.Close()
			live.blockExec.Stop()
			signer, err := privval.LoadFilePV(f.keyFile, f.stateFile)
			require.NoError(t, err)
			recovered := repair80State(t, f, signer)
			height, persisted, err := recovered.wal.ReadLastHeightMsgs()
			require.NoError(t, err)
			require.EqualValues(t, 1, height)
			proposalCount := 0
			for _, entry := range persisted {
				if mi, ok := entry.any.(msgInfo); ok {
					if _, ok := mi.Msg.(*ProposalMessage); ok {
						proposalCount++
						require.Equal(t, f.messages[0].PeerID, mi.PeerID)
						require.True(t, received.Equal(mi.ReceiveTime))
					}
				}
			}
			require.Equal(t, 1, proposalCount)
			require.NoError(t, recovered.catchupReplay(t.Context(), 1))
			require.True(t, received.Equal(recovered.roundState.ProposalReceiveTime()))
			after := repair80Prevote(t, recovered, f, tc.timely)
			require.True(t, before.BlockID.Equals(after.BlockID))
			require.True(t, before.Timestamp.Equal(after.Timestamp))
			require.Equal(t, before.ToProto().Signature, after.ToProto().Signature)
			conflict := after.ToProto()
			if tc.timely {
				conflict.BlockID = (&types.BlockID{}).ToProto()
			} else {
				conflict.BlockID = f.proposal.BlockID.ToProto()
			}
			conflict.Signature = nil
			require.Error(t, signer.SignVote(t.Context(), f.genesis.ChainID, conflict))
		})
	}
}

func TestRepair_80_LegacyProposalFailsClosed(t *testing.T) {
	f := newRepair80Fixture(t)
	// Encode the real producer output with the old schema: its debug time is
	// deliberately timely, but it is still not evidence of actual receipt.
	legacy, err := proto.Marshal(&tmcons.TimedWALMessage{
		Time: f.proposal.Timestamp, Msg: NewWALMessage(f.messages[0]).toProto(),
	})
	require.NoError(t, err)
	cs := repair80State(t, f, f.signer)
	cs.wal.Close()
	inner, err := logwal.OpenLog(f.cfg.Consensus.WalFile(), logwal.DefaultConfig())
	require.NoError(t, err)
	require.NoError(t, inner.Append(legacy))
	require.NoError(t, inner.Sync())
	inner.Close()
	cs.blockExec.Stop()
	recovered := repair80State(t, f, f.signer)
	require.ErrorIs(t, recovered.catchupReplay(t.Context(), 1), ErrWALProposalReceiveTimeMissing)
	require.Nil(t, recovered.roundState.Proposal())
	require.Empty(t, recovered.internalMsgQueue)
}

func TestRepair_80_LegacyNonProposal(t *testing.T) {
	f := newRepair80Fixture(t)
	mi := f.messages[1]
	legacy, err := proto.Marshal(&tmcons.TimedWALMessage{
		Time: time.Now(), Msg: NewWALMessage(mi).toProto(),
	})
	require.NoError(t, err)
	decoded, err := walFromBytes(legacy)
	require.NoError(t, err)
	got, ok := decoded.any.(msgInfo)
	require.True(t, ok)
	require.Equal(t, mi.PeerID, got.PeerID)
	require.True(t, got.ReceiveTime.IsZero())
	require.Equal(t, MsgToProto(mi.Msg), MsgToProto(got.Msg))
}

func TestRepair_80_ProposalSignatureStillRequired(t *testing.T) {
	f := newRepair80Fixture(t)
	proposal := *f.proposal
	proposal.Timestamp = proposal.Timestamp.Add(time.Nanosecond)
	f.messages[0].Msg = &ProposalMessage{Proposal: &proposal}
	live := repair80State(t, f, f.signer)
	for _, mi := range f.messages {
		require.NoError(t, live.wal.Append(NewWALMessage(mi)))
	}
	require.NoError(t, live.wal.Sync())
	live.wal.Close()
	live.blockExec.Stop()
	recovered := repair80State(t, f, f.signer)
	require.NoError(t, recovered.catchupReplay(t.Context(), 1))
	require.Nil(t, recovered.roundState.Proposal())
	require.Empty(t, recovered.internalMsgQueue)
	require.ErrorIs(t, recovered.defaultSetProposal(&proposal, f.messages[0].ReceiveTime), ErrInvalidProposalSignature)
}

func TestRepair_80_InvalidReceiveTime(t *testing.T) {
	f := newRepair80Fixture(t)
	entry, err := proto.Marshal(&tmcons.TimedWALMessage{
		Time: time.Now(), Msg: NewWALMessage(f.messages[0]).toProto(),
	})
	require.NoError(t, err)
	for _, tc := range []struct {
		name      string
		timestamp *gogotypes.Timestamp
	}{
		{"nanoseconds", &gogotypes.Timestamp{Nanos: 1000000000}},
		{"seconds", &gogotypes.Timestamp{Seconds: 253402300800}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := proto.Marshal(tc.timestamp)
			require.NoError(t, err)
			bad := protowire.AppendTag(append([]byte{}, entry...), walReceiveTimeField, protowire.BytesType)
			bad = protowire.AppendBytes(bad, encoded)
			_, err = walFromBytes(bad)
			require.Error(t, err)
		})
	}
	badType := protowire.AppendTag(append([]byte{}, entry...), walReceiveTimeField, protowire.VarintType)
	badType = protowire.AppendVarint(badType, 1)
	_, err = walFromBytes(badType)
	require.Error(t, err)
	encoded, err := proto.Marshal(&gogotypes.Timestamp{})
	require.NoError(t, err)
	duplicate := append([]byte{}, entry...)
	for range 2 {
		duplicate = protowire.AppendTag(duplicate, walReceiveTimeField, protowire.BytesType)
		duplicate = protowire.AppendBytes(duplicate, encoded)
	}
	_, err = walFromBytes(duplicate)
	require.Error(t, err)
}
