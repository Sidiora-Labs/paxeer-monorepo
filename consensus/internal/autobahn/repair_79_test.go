package autobahn_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/avail"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/data"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/pb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils/wireguard"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/stretchr/testify/require"
)

type repair79Workload struct {
	committee    *types.Committee
	keys         []types.SecretKey
	availability *avail.State
	produced     *data.State
	blocks       map[types.LaneID][]*types.Block
	tips         map[types.LaneID]*types.LaneQC
}

func repair79Produce(t *testing.T, ctx context.Context, rng utils.Rng) *repair79Workload {
	t.Helper()
	c, keys := types.GenCommittee(rng, 11)
	w := &repair79Workload{committee: c, keys: keys, blocks: map[types.LaneID][]*types.Block{}, tips: map[types.LaneID]*types.LaneQC{}}
	wal, err := data.NewDataWAL(utils.None[string](), c)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wal.Close()) })
	w.produced, err = data.NewState(&data.Config{Committee: c}, wal)
	require.NoError(t, err)
	w.availability, err = avail.NewState(keys[0], w.produced, utils.None[string]())
	require.NoError(t, err)
	for _, key := range keys {
		producer, err := avail.NewState(key, w.produced, utils.None[string]())
		require.NoError(t, err)
		for n := range types.BlockNumber(10) {
			proposal, err := producer.ProduceLocalBlock(n, types.GenPayload(rng))
			require.NoError(t, err)
			block := proposal.Msg().Block()
			w.blocks[key.Public()] = append(w.blocks[key.Public()], block)
			require.NoError(t, w.availability.PushBlock(ctx, proposal))
			var votes []*types.Signed[*types.LaneVote]
			for _, voter := range keys {
				vote := types.Sign(voter, types.NewLaneVote(block.Header()))
				require.NoError(t, w.availability.PushVote(ctx, vote))
				votes = append(votes, vote)
			}
			w.tips[key.Public()] = types.NewLaneQC(votes)
		}
	}
	return w
}

func (w *repair79Workload) leader(t *testing.T, vs types.ViewSpec) types.SecretKey {
	t.Helper()
	for _, key := range w.keys {
		if key.Public() == w.committee.Leader(vs.View()) {
			return key
		}
	}
	t.Fatal("committee leader has no key")
	return types.SecretKey{}
}

func (w *repair79Workload) commit(p *types.Proposal) *types.CommitQC {
	var votes []*types.Signed[*types.CommitVote]
	for _, key := range w.keys {
		votes = append(votes, types.Sign(key, types.NewCommitVote(p)))
	}
	return types.NewCommitQC(votes)
}

func TestRepair_79_FullQCHeaderBoundRestart(t *testing.T) {
	rng := utils.TestRng()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	w := repair79Produce(t, ctx, rng)
	c := w.committee
	dir := utils.Some(t.TempDir())
	wal, err := data.NewDataWAL(dir, c)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wal.Close()) })
	consumer, err := data.NewState(&data.Config{Committee: c}, wal)
	require.NoError(t, err)
	var finalized []*types.BlockHeader
	var certificates []*types.FullCommitQC
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("availability", func() error { return utils.IgnoreCancel(w.availability.Run(ctx)) })
		s.SpawnBgNamed("consumer", func() error { return utils.IgnoreCancel(consumer.Run(ctx)) })
		prev := utils.None[*types.CommitQC]()
		for _, count := range []int{100, 10} {
			selected, err := w.availability.WaitForLaneQCs(ctx, prev)
			require.NoError(t, err)
			vs := types.ViewSpec{CommitQC: prev}
			proposal, err := types.NewProposal(w.leader(t, vs), c, vs, c.GenesisTimestamp(), selected, utils.None[*types.AppQC]())
			require.NoError(t, err)
			require.NoError(t, proposal.Verify(c, vs))
			qc := w.commit(proposal.Proposal().Msg())
			require.NoError(t, w.availability.PushCommitQC(ctx, qc))
			gr := qc.GlobalRange(c)
			require.Equal(t, uint64(count), gr.Len())
			// State.Run calls the real fullCommitQC producer and inserts its output.
			full, err := w.produced.QC(ctx, gr.First)
			require.NoError(t, err)
			require.Len(t, full.Headers(), count)
			require.NoError(t, full.Verify(c))
			wire := types.FullCommitQCConv.Marshal(full)
			transported, err := types.FullCommitQCConv.Unmarshal(wire)
			require.NoError(t, err)
			require.NoError(t, transported.Verify(c))
			require.Equal(t, wire, types.FullCommitQCConv.Marshal(transported))
			var blocks []*types.Block
			for _, header := range full.Headers() {
				block := w.blocks[header.Lane()][header.BlockNumber()]
				require.Equal(t, block.Header().Hash(), header.Hash())
				blocks = append(blocks, block)
			}
			require.NoError(t, consumer.PushQC(ctx, transported, blocks))
			finalized = append(finalized, full.Headers()...)
			certificates = append(certificates, full)
			prev = utils.Some(qc)
		}
		// This waits for both WALs to be durable before stopping the consumer.
		return consumer.PushAppHash(ctx, c.FirstBlock()+109, types.GenAppHash(rng))
	}))
	require.Len(t, finalized, 110)
	require.NoError(t, wal.Close())
	reopenedWAL, err := data.NewDataWAL(dir, c)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopenedWAL.Close()) })
	reopened, err := data.NewState(&data.Config{Committee: c}, reopenedWAL)
	require.NoError(t, err)
	require.Equal(t, c.FirstBlock(), reopened.FirstBlock())
	require.Equal(t, c.FirstBlock()+110, reopened.NextBlock())
	seen := map[types.BlockHeaderHash]bool{}
	for i, header := range finalized {
		n := c.FirstBlock() + types.GlobalBlockNumber(i)
		block, err := reopened.Block(ctx, n)
		require.NoError(t, err)
		require.Equal(t, header.Hash(), block.Header().Hash())
		require.False(t, seen[header.Hash()], "a finalized block appeared twice")
		seen[header.Hash()] = true
		qc, err := reopened.QC(ctx, n)
		require.NoError(t, err)
		gr := qc.QC().GlobalRange(c)
		require.Equal(t, header.Hash(), qc.Headers()[n-gr.First].Hash())
	}
	for _, blocks := range w.blocks {
		for _, block := range blocks {
			require.True(t, seen[block.Header().Hash()], "a produced block was dropped")
		}
	}
	for _, original := range certificates {
		loaded, err := reopened.QC(ctx, original.QC().GlobalRange(c).First)
		require.NoError(t, err)
		require.Equal(t, types.FullCommitQCConv.Marshal(original), types.FullCommitQCConv.Marshal(loaded))
	}
}

func TestRepair_79_HeaderBudgetRejectsOversize(t *testing.T) {
	rng := utils.TestRng()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	w := repair79Produce(t, ctx, rng)
	c := w.committee
	vs := types.ViewSpec{}
	_, err := types.NewProposal(w.leader(t, vs), c, vs, c.GenesisTimestamp(), w.tips, utils.None[*types.AppQC]())
	require.Error(t, err)
	field := (&pb.FullCommitQC{}).ProtoReflect().Descriptor().Fields().ByName("headers")
	rule := pb.SchemaForFullCommitQC.Rules[wireguard.Number(field.Number())]
	require.EqualValues(t, types.MaxFullCommitQCHeaders, rule.MaxCount)

	// Model an oversized peer certificate using public codecs and real signed blocks.
	for _, total := range []int{101, 110} {
		var headers []*types.BlockHeader
		var ranges []*types.LaneRange
		for lane := range c.Lanes().All() {
			count := min(10, total-len(headers))
			var last utils.Option[*types.BlockHeader]
			for _, block := range w.blocks[lane][:count] {
				headers = append(headers, block.Header())
				last = utils.Some(block.Header())
			}
			ranges = append(ranges, types.NewLaneRange(lane, 0, last))
		}
		p, err := types.ProposalConv.Decode(&pb.Proposal{
			View:       types.ViewConv.Encode(vs.View()),
			Timestamp:  types.TimeConv.Encode(c.GenesisTimestamp()),
			LaneRanges: types.LaneRangeConv.EncodeSlice(ranges),
		})
		require.NoError(t, err)
		require.Error(t, p.Verify(c))
		qc := w.commit(p)
		require.Error(t, qc.Verify(c))
		full := types.NewFullCommitQC(qc, headers)
		require.Error(t, full.Verify(c))
		require.Error(t, w.availability.PushCommitQC(ctx, qc))
		require.Error(t, w.produced.PushQC(ctx, full, nil))
		_, err = types.FullCommitQCConv.Unmarshal(types.FullCommitQCConv.Marshal(full))
		require.Error(t, err)
	}
}
