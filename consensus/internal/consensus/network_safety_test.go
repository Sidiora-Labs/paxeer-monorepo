package consensus

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	kitprom "github.com/go-kit/kit/metrics/prometheus"
	"github.com/gogo/protobuf/proto"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/evidence"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/libs/protoio"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	tmpubsub "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/test/factory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmcons "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/consensus"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// The peers use file-backed signers, a real kvstore application, and independent
// stores. Only the adversary signs conflicting transcripts directly with its key.
type consensusNetworkFixture struct {
	genesis   sm.State
	keys      []ed25519.SecretKey
	signers   []*privval.FilePV
	leader    int
	byzantine int
	honest    []int
}

func newConsensusNetworkFixture(t *testing.T, weighted bool, genesisTime time.Time) consensusNetworkFixture {
	t.Helper()
	f := consensusNetworkFixture{}
	vals := make([]*types.Validator, 4)
	for i := range 4 {
		key := ed25519.TestSecretKey([]byte(fmt.Sprintf("%s/validator/%d", t.Name(), i)))
		dir := t.TempDir()
		pv := privval.NewFilePV(key, filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
		require.NoError(t, pv.Save())
		f.keys = append(f.keys, key)
		f.signers = append(f.signers, pv)
		power := int64(10)
		if weighted {
			power += int64(i)
		}
		vals[i] = types.NewValidator(key.Public(), power)
	}
	params := factory.ConsensusParams()
	params.Timeout.Propose = time.Minute
	params.Timeout.Vote = time.Minute
	params.Timeout.Commit = time.Millisecond
	params.Synchrony = types.SynchronyParams{Precision: 10 * time.Second, MessageDelay: 10 * time.Second}
	cfg := getConfig(t)
	var err error
	f.genesis, err = sm.MakeGenesisState(factory.GenesisDoc(cfg, genesisTime, vals, params))
	require.NoError(t, err)
	f.genesis.Version.Consensus.App = kvstore.ProtocolVersion
	rs := &cstypes.RoundState{HRS: cstypes.HRS{Height: 1}, Validators: f.genesis.Validators}
	f.leader = f.index(t, rs.Leader())
	f.byzantine = (f.leader + 1) % 4
	for i := range 4 {
		if i != f.leader && i != f.byzantine {
			f.honest = append(f.honest, i)
		}
	}
	return f
}

func (f consensusNetworkFixture) index(t *testing.T, key crypto.PubKey) int {
	t.Helper()
	for i, k := range f.keys {
		if k.Public() == key {
			return i
		}
	}
	t.Fatal("leader does not belong to fixture validator set")
	return -1
}

func newConsensusNetworkState(t *testing.T, cfg *config.Config, state sm.State, pv types.PrivValidator, blockStore *store.BlockStore, metrics *Metrics) (*testState, *evidence.Pool) {
	t.Helper()
	app := kvstore.NewApplication()
	app.SetValidators(types.TM2PB.ValidatorUpdates(state.Validators))
	appProxy := proxy.New(app, proxy.NopMetrics())
	pool := mempool.NewTxMempool(cfg.Mempool.ToMempoolConfig(), appProxy, mempool.NopMetrics(), mempool.NopTxConstraintsFetcher)
	stateStore := sm.NewStore(dbm.NewMemDB())
	require.NoError(t, stateStore.Save(state))
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(t.Context()))
	evpool := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), bus)
	require.NoError(t, evpool.Start(state))
	executor := sm.NewBlockExecutor(stateStore, appProxy, pool, evpool, blockStore, bus, sm.NopMetrics(), types.DefaultConsensusPolicy())
	wal, err := OpenWAL(cfg.Consensus.WalFile())
	require.NoError(t, err)
	cs := &testState{State: NewState(cfg.Consensus, wal, stateStore, executor, blockStore, pool, evpool, bus, []trace.TracerProviderOption{}, metrics)}
	require.NoError(t, cs.updateStateFromStore())
	cs.SetPrivValidator(t.Context(), utils.Some(pv))
	t.Cleanup(func() {
		cs.waitForTestRoutines()
		executor.Stop()
		wal.Close()
		bus.Wait()
		require.NoError(t, evpool.Close())
		require.NoError(t, app.Close())
	})
	return cs, evpool
}

func (f consensusNetworkFixture) state(t *testing.T, signer int) (*testState, *evidence.Pool) {
	t.Helper()
	return newConsensusNetworkState(t, getConfig(t), f.genesis, f.signers[signer], store.NewBlockStore(dbm.NewMemDB()), NopMetrics())
}

func consensusNetworkProducerMessages(t *testing.T, producer *testState, height int64, round int32) []msgInfo {
	t.Helper()
	producer.enterNewRound(t.Context(), height, round, "repair-m1082-producer")
	first := proposalReplayNextMessage(t, producer)
	proposal, ok := first.Msg.(*ProposalMessage)
	require.True(t, ok)
	require.False(t, first.ReceiveTime.IsZero())
	require.NoError(t, producer.roundState.Leader().Verify(types.ProposalSignBytes(producer.state.ChainID, proposal.Proposal.ToProto()), proposal.Proposal.Signature))
	result := []msgInfo{first}
	for range proposal.Proposal.BlockID.PartSetHeader.Total {
		part := proposalReplayNextMessage(t, producer)
		require.IsType(t, &BlockPartMessage{}, part.Msg)
		result = append(result, part)
	}
	return result
}

func consensusNetworkTransport(t *testing.T, mi msgInfo) msgInfo {
	t.Helper()
	encoded, err := proto.Marshal(MsgToProto(mi.Msg))
	require.NoError(t, err)
	var wire tmcons.Message
	require.NoError(t, proto.Unmarshal(encoded, &wire))
	mi.Msg, err = MsgFromProto(&wire)
	require.NoError(t, err)
	return mi
}

func (f consensusNetworkFixture) vote(t *testing.T, signer int, kind tmproto.SignedMsgType, height int64, round int32, blockID types.BlockID) *types.Vote {
	t.Helper()
	vs := newValidatorStub(f.signers[signer], 0)
	var ok bool
	vs.Index, _, ok = f.genesis.Validators.GetByAddress(f.keys[signer].Public().Address())
	require.True(t, ok)
	vs.Height, vs.Round = height, round
	return signVote(t.Context(), t, vs, kind, f.genesis.ChainID, blockID)
}

func (f consensusNetworkFixture) adversarialVote(t *testing.T, kind tmproto.SignedMsgType, blockID types.BlockID) *types.Vote {
	return f.adversarialVoteAtRound(t, kind, 0, blockID)
}

func (f consensusNetworkFixture) adversarialVoteAtRound(t *testing.T, kind tmproto.SignedMsgType, round int32, blockID types.BlockID) *types.Vote {
	t.Helper()
	key := f.keys[f.byzantine]
	index, _, ok := f.genesis.Validators.GetByAddress(key.Public().Address())
	require.True(t, ok)
	v := &types.Vote{Type: kind, Height: 1, Round: round, Timestamp: time.Now().UTC(), ValidatorAddress: key.Public().Address(), ValidatorIndex: index, BlockID: blockID}
	v.Signature = utils.Some(key.Sign(types.VoteSignBytes(f.genesis.ChainID, v.ToProto())))
	require.NoError(t, v.ValidateBasic())
	require.NoError(t, v.Verify(f.genesis.ChainID, key.Public()))
	return v
}

type consensusNetworkNetwork struct {
	ctx       context.Context
	fixture   consensusNetworkFixture
	consumers []*testState
	pools     []*evidence.Pool
	reactors  []*Reactor
	targets   []*p2p.TestNode
	senders   []*p2p.TestNode
	channels  []channelBundle
	votes     []eventbus.Subscription
	proposals []eventbus.Subscription
	blocks    []eventbus.Subscription
}

func newConsensusNetworkNetwork(t *testing.T, f consensusNetworkFixture, consumers, senders int) *consensusNetworkNetwork {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	net := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: consumers + senders})
	nodes := net.Nodes()
	n := &consensusNetworkNetwork{ctx: ctx, fixture: f, targets: nodes[:consumers], senders: nodes[consumers:]}
	for i, node := range n.targets {
		cs, pool := f.state(t, f.honest[i])
		n.consumers = append(n.consumers, cs)
		n.pools = append(n.pools, pool)
		voteSub, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{ClientID: "repair-m1082-votes", Query: types.EventQueryVote, Limit: 100})
		require.NoError(t, err)
		n.votes = append(n.votes, voteSub)
		proposalSub, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{ClientID: "repair-m1082-proposals", Query: types.EventQueryCompleteProposal, Limit: 10})
		require.NoError(t, err)
		n.proposals = append(n.proposals, proposalSub)
		blockSub, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{ClientID: "repair-m1082-blocks", Query: types.EventQueryNewBlock, Limit: 10})
		require.NoError(t, err)
		n.blocks = append(n.blocks, blockSub)
		r, err := NewReactor(cs.State, node.Router, cs.eventBus, true, NopMetrics(), config.DefaultConfig())
		require.NoError(t, err)
		require.NoError(t, r.Start(ctx))
		t.Cleanup(r.Stop)
		r.SwitchToConsensus(f.genesis, false)
		n.reactors = append(n.reactors, r)
	}
	for _, node := range n.senders {
		open := func(desc desc) *p2p.Channel[*tmcons.Message] {
			ch, err := p2p.OpenChannel(node.Router, desc)
			require.NoError(t, err)
			return ch
		}
		n.channels = append(n.channels, channelBundle{state: open(GetStateChannelDescriptor()), data: open(GetDataChannelDescriptor()), vote: open(GetVoteChannelDescriptor()), votSet: open(GetVoteSetChannelDescriptor())})
	}
	net.ConnectCycle(ctx, t)
	for j := range n.senders {
		seen := map[types.NodeID]bool{}
		for len(seen) < consumers {
			msg, err := n.channels[j].state.Recv(ctx)
			require.NoError(t, err)
			if step := msg.Message.GetNewRoundStep(); step != nil && step.Height == 1 {
				seen[msg.From] = true
			}
		}
	}
	return n
}

func (n *consensusNetworkNetwork) sendVote(v *types.Vote) {
	for _, target := range n.targets {
		n.channels[0].vote.Send(MsgToProto(&VoteMessage{Vote: v}), target.NodeID)
	}
}

func (n *consensusNetworkNetwork) sendProposal(sender int, messages []msgInfo) {
	for _, target := range n.targets {
		for _, mi := range messages {
			n.channels[sender].data.Send(MsgToProto(mi.Msg), target.NodeID)
		}
	}
}

func (n *consensusNetworkNetwork) awaitVote(t *testing.T, consumer int, v *types.Vote) {
	t.Helper()
	for {
		msg, err := n.votes[consumer].Next(n.ctx)
		require.NoError(t, err)
		got := msg.Data().(types.EventDataVote).Vote
		if got.Type == v.Type && got.ValidatorIndex == v.ValidatorIndex && got.Height == v.Height && got.Round == v.Round {
			require.True(t, v.BlockID.Equals(got.BlockID))
			require.NoError(t, got.Verify(n.fixture.genesis.ChainID, n.fixture.genesis.Validators.Validators[got.ValidatorIndex].PubKey))
			return
		}
	}
}

func (n *consensusNetworkNetwork) awaitProposal(t *testing.T, consumer int, id types.BlockID) {
	t.Helper()
	msg, err := n.proposals[consumer].Next(n.ctx)
	require.NoError(t, err)
	event := msg.Data().(types.EventDataCompleteProposal)
	require.EqualValues(t, 1, event.Height)
	require.Zero(t, event.Round)
	require.True(t, id.Equals(event.BlockID))
}

func (n *consensusNetworkNetwork) awaitAgreement(t *testing.T, id types.BlockID) {
	t.Helper()
	for i, sub := range n.blocks {
		msg, err := sub.Next(n.ctx)
		require.NoError(t, err)
		block := msg.Data().(types.EventDataNewBlock).Block
		require.EqualValues(t, 1, block.Height)
		require.Equal(t, id.Hash, block.Hash())
		cs := n.consumers[i]
		cs.mtx.RLock()
		commit := cs.blockStore.LoadSeenCommit()
		cs.mtx.RUnlock()
		require.NotNil(t, commit)
		require.NoError(t, n.fixture.genesis.Validators.VerifyCommit(n.fixture.genesis.ChainID, id, 1, commit))
	}
}

func testConsensusNetworkEquivocation(t *testing.T) {
	f := newConsensusNetworkFixture(t, false, time.Now().Add(-time.Minute))
	producer, _ := f.state(t, f.leader)
	messages := consensusNetworkProducerMessages(t, producer, 1, 0)
	id := messages[0].Msg.(*ProposalMessage).Proposal.BlockID
	n := newConsensusNetworkNetwork(t, f, 2, 1)
	blockVote := f.adversarialVote(t, tmproto.PrevoteType, id)
	nilVote := f.adversarialVote(t, tmproto.PrevoteType, types.BlockID{})
	require.False(t, blockVote.BlockID.Equals(nilVote.BlockID))
	// Observe acceptance at both nodes before sending the conflict and before
	// releasing the quorum. This avoids racing equivocation against commitment.
	n.sendVote(blockVote)
	for i := range n.consumers {
		n.awaitVote(t, i, blockVote)
	}
	n.sendVote(nilVote)
	// A single future-round vote cannot advance the round and is not gossiped
	// as a current-round vote. Its event orders each sender's vote transcript.
	barrier := f.adversarialVoteAtRound(t, tmproto.PrevoteType, 1, types.BlockID{})
	n.sendVote(barrier)
	for i, cs := range n.consumers {
		n.awaitVote(t, i, barrier)
		require.False(t, cs.GetRoundState().Votes.Prevotes(0).HasTwoThirdsMajority())
		_, ok := n.reactors[i].GetPeerState(n.senders[0].NodeID)
		require.True(t, ok, "forwarding an equivocating vote must not disconnect the forwarding peer")
	}
	n.sendProposal(0, messages)
	for i := range n.consumers {
		n.awaitProposal(t, i, id)
	}
	n.sendVote(f.vote(t, f.leader, tmproto.PrevoteType, 1, 0, id))
	n.sendVote(f.vote(t, f.leader, tmproto.PrecommitType, 1, 0, id))
	n.awaitAgreement(t, id)
	for i, pool := range n.pools {
		element, err := pool.WaitEvidenceFront(n.ctx)
		require.NoError(t, err)
		ev, ok := element.Value().(*types.DuplicateVoteEvidence)
		require.True(t, ok)
		require.EqualValues(t, 1, ev.Height())
		require.Equal(t, f.keys[f.byzantine].Public().Address(), ev.VoteA.ValidatorAddress)
		require.False(t, ev.VoteA.BlockID.Equals(ev.VoteB.BlockID))
		require.NoError(t, ev.VoteA.Verify(f.genesis.ChainID, f.keys[f.byzantine].Public()))
		require.NoError(t, ev.VoteB.Verify(f.genesis.ChainID, f.keys[f.byzantine].Public()))
		require.Equal(t, n.consumers[i].blockStore.LoadBlock(1).Time, ev.Time())
		pending, _ := pool.PendingEvidence(-1)
		require.Len(t, pending, 1)
		require.Equal(t, ev.Hash(), pending[0].Hash())
	}
}

func testConsensusNetworkInvalidPrecommit(t *testing.T) {
	f := newConsensusNetworkFixture(t, false, time.Now().Add(-time.Minute))
	producer, _ := f.state(t, f.leader)
	messages := consensusNetworkProducerMessages(t, producer, 1, 0)
	id := messages[0].Msg.(*ProposalMessage).Proposal.BlockID
	n := newConsensusNetworkNetwork(t, f, 2, 1)
	valid := f.adversarialVote(t, tmproto.PrecommitType, id)
	invalid := *valid
	invalid.Timestamp = invalid.Timestamp.Add(time.Nanosecond)
	require.NoError(t, invalid.ValidateBasic())
	require.Error(t, invalid.Verify(f.genesis.ChainID, f.keys[f.byzantine].Public()))
	n.sendVote(&invalid)
	barrier := f.adversarialVoteAtRound(t, tmproto.PrevoteType, 1, types.BlockID{})
	n.sendVote(barrier)
	for i, cs := range n.consumers {
		n.awaitVote(t, i, barrier)
		votes := cs.GetRoundState().Votes.Precommits(0)
		for j := range votes.Size() {
			_, ok := votes.GetByIndex(int32(j))
			require.False(t, ok, "invalid precommit must contribute no voting power")
		}
		require.False(t, votes.HasTwoThirdsAny())
		require.False(t, votes.HasTwoThirdsMajority())
	}
	n.sendVote(valid)
	for i := range n.consumers {
		n.awaitVote(t, i, valid)
	}
	n.sendProposal(0, messages)
	for i := range n.consumers {
		n.awaitProposal(t, i, id)
	}
	n.sendVote(f.vote(t, f.leader, tmproto.PrevoteType, 1, 0, id))
	n.awaitAgreement(t, id)
}

func testConsensusNetworkMemoryLimit(t *testing.T) {
	f := newConsensusNetworkFixture(t, false, time.Now().Add(-time.Minute))
	producer, _ := f.state(t, f.leader)
	messages := consensusNetworkProducerMessages(t, producer, 1, 0)
	valid := messages[0].Msg.(*ProposalMessage).Proposal
	n := newConsensusNetworkNetwork(t, f, 1, 2)
	bad := *valid
	bad.BlockID.PartSetHeader.Total = types.MaxBlockPartsCount + 1
	// The normal signing helper validates the part count. A hostile signer
	// still uses the canonical wire encoding, with the oversized count signed.
	canonical := types.CanonicalizeProposal(f.genesis.ChainID, valid.ToProto())
	signBytes, err := protoio.MarshalDelimited(&canonical)
	require.NoError(t, err)
	require.Equal(t, types.ProposalSignBytes(f.genesis.ChainID, valid.ToProto()), signBytes)
	canonical.BlockID.PartSetHeader.Total = bad.BlockID.PartSetHeader.Total
	signBytes, err = protoio.MarshalDelimited(&canonical)
	require.NoError(t, err)
	bad.Signature = f.keys[f.leader].Sign(signBytes)
	require.NoError(t, f.keys[f.leader].Public().Verify(signBytes, bad.Signature))
	_, err = MsgFromProto(MsgToProto(&ProposalMessage{&bad}))
	require.Error(t, err)
	peer, ok := n.reactors[0].GetPeerState(n.senders[0].NodeID)
	require.True(t, ok)
	n.channels[0].data.Send(MsgToProto(&ProposalMessage{&bad}), n.targets[0].NodeID)
	require.NoError(t, n.senders[0].WaitForConn(n.ctx, n.targets[0].NodeID, false))
	require.Nil(t, peer.GetRoundState().ProposalBlockParts)
	require.False(t, peer.GetRoundState().Proposal)
	rejected := n.consumers[0].GetRoundState()
	require.Nil(t, rejected.Proposal)
	require.Nil(t, rejected.ProposalBlockParts)
	require.Nil(t, rejected.ProposalBlock)
	n.sendProposal(1, messages)
	n.awaitProposal(t, 0, valid.BlockID)
	rs := n.consumers[0].GetRoundState()
	require.Equal(t, valid.BlockID.PartSetHeader, rs.ProposalBlockParts.Header())
	require.LessOrEqual(t, rs.ProposalBlockParts.Total(), uint32(types.MaxBlockPartsCount))
	require.NoError(t, rs.Leader().Verify(types.ProposalSignBytes(f.genesis.ChainID, rs.Proposal.ToProto()), rs.Proposal.Signature))
}

func TestConsensusNetworkEquivocation(t *testing.T)     { testConsensusNetworkEquivocation(t) }
func TestConsensusNetworkInvalidPrecommit(t *testing.T) { testConsensusNetworkInvalidPrecommit(t) }
func TestConsensusNetworkMemoryLimit(t *testing.T)      { testConsensusNetworkMemoryLimit(t) }
func TestConsensusNetworkWALReceiveTime(t *testing.T) {
	testConsensusNetworkWALProto(t)
	testConsensusNetworkWALRecovery(t)
}
func TestConsensusNetworkNonzeroProposerTimeout(t *testing.T) {
	testConsensusNetworkNonzeroProposerTimeout(t)
}
func TestConsensusNetworkLeaderSelection(t *testing.T) {
	testConsensusNetworkProposerSelection(t, 0)
	testConsensusNetworkProposerSelection(t, 2)
}
func TestConsensusNetworkAssembledBlock(t *testing.T) { testConsensusNetworkAssembledBlock(t) }

func TestConsensusNetworkStepLatency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prom.NewRegistry()
		gauge := prom.NewGaugeVec(prom.GaugeOpts{Name: "repair_m1082_step_latency", Help: "Step latency in nanoseconds"}, []string{"step"})
		registry.MustRegister(gauge)
		metrics := NopMetrics()
		metrics.StepLatency = kitprom.NewGauge(gauge)
		value := func(step cstypes.RoundStepType) float64 {
			families, err := registry.Gather()
			require.NoError(t, err)
			for _, family := range families {
				for _, m := range family.Metric {
					for _, label := range m.Label {
						if label.GetName() == "step" && label.GetValue() == step.String() {
							return m.GetGauge().GetValue()
						}
					}
				}
			}
			return 0
		}
		metrics.MarkStepLatency(cstypes.RoundStepNewHeight)
		require.Zero(t, value(cstypes.RoundStepNewHeight))
		time.Sleep(2 * time.Millisecond) // virtual time inside synctest
		metrics.MarkStepLatency(cstypes.RoundStepPropose)
		require.Equal(t, float64(2*time.Millisecond), value(cstypes.RoundStepPropose))
		f := newConsensusNetworkFixture(t, false, time.Now().Add(-time.Minute))
		cs, _ := newConsensusNetworkState(t, getConfig(t), f.genesis, f.signers[f.leader], store.NewBlockStore(dbm.NewMemDB()), metrics)
		for height := int64(2); height <= 3; height++ {
			time.Sleep(5 * time.Second)
			cs.updateHeight(height)
			require.Zero(t, value(cstypes.RoundStepPropose))
			time.Sleep(3 * time.Millisecond)
			metrics.MarkStepLatency(cstypes.RoundStepPropose)
			require.Equal(t, float64(3*time.Millisecond), value(cstypes.RoundStepPropose))
			time.Sleep(time.Millisecond)
			metrics.MarkStepLatency(cstypes.RoundStepPrevote)
			require.Equal(t, float64(time.Millisecond), value(cstypes.RoundStepPrevote))
		}
	})
}
