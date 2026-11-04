package avail

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus/persist"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/data"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/stretchr/testify/require"
)

type repairM1080Progress struct {
	state  *State
	qcs    []*types.CommitQC
	blocks []*types.Signed[*types.LaneProposal]
}

func repairM1080State(t *testing.T, c *types.Committee, key types.SecretKey, dir utils.Option[string]) *State {
	t.Helper()
	wal, err := data.NewDataWAL(utils.None[string](), c)
	require.NoError(t, err)
	ds, err := data.NewState(&data.Config{Committee: c}, wal)
	require.NoError(t, err)
	s, err := NewState(key, ds, dir)
	require.NoError(t, err)
	return s
}

// Each QC finalizes one real block. The receiver consumes the wire codec's
// output, gathers lane votes, and persists CommitQCs through runPersist.
func repairM1080CertifiedProgress(t *testing.T, rng utils.Rng, c *types.Committee, keys []types.SecretKey, count int, dir utils.Option[string]) repairM1080Progress {
	t.Helper()
	producer := repairM1080State(t, c, keys[0], utils.None[string]())
	f := repairM1080Progress{state: repairM1080State(t, c, keys[0], dir)}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, sc scope.Scope) error {
		sc.SpawnBg(func() error { return utils.IgnoreCancel(f.state.runPersist(ctx, f.state.persisters)) })
		prev := utils.None[*types.CommitQC]()
		for n := range count {
			block, err := producer.ProduceLocalBlock(types.BlockNumber(n), types.GenPayload(rng))
			if err != nil {
				return err
			}
			conv := types.SignedMsgConv[*types.LaneProposal]()
			block, err = conv.Decode(conv.Encode(block))
			if err != nil {
				return err
			}
			if err := f.state.PushBlock(ctx, block); err != nil {
				return err
			}
			f.blocks = append(f.blocks, block)
			for _, vote := range makeLaneVotes(keys, block.Msg().Block().Header()) {
				if err := f.state.PushVote(ctx, vote); err != nil {
					return err
				}
			}
			laneQCs, err := f.state.WaitForLaneQCs(ctx, prev)
			if err != nil {
				return err
			}
			qc := makeCommitQC(c, keys, prev, laneQCs, utils.None[*types.AppQC]())
			if err := f.state.PushCommitQC(ctx, qc); err != nil {
				return err
			}
			if err := f.state.waitForCommitQC(ctx, qc.Index()); err != nil {
				return err
			}
			full, err := f.state.fullCommitQC(ctx, qc.Index())
			if err != nil {
				return err
			}
			if err := full.Verify(c); err != nil {
				return err
			}
			f.qcs = append(f.qcs, qc)
			prev = utils.Some(qc)
		}
		return nil
	}))
	return f
}

func repairM1080AppQC(rng utils.Rng, c *types.Committee, keys []types.SecretKey, qc *types.CommitQC) *types.AppQC {
	return types.NewAppQC(makeAppVotes(keys, types.NewAppProposal(qc.GlobalRange(c).First, qc.Index(), types.GenAppHash(rng))))
}

func repairM1080PersistLiveAnchor(t *testing.T, s *State, app *types.AppQC, qc *types.CommitQC) {
	t.Helper()
	require.NoError(t, s.PushAppQC(app, qc))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, sc scope.Scope) error {
		sc.SpawnBg(func() error { return utils.IgnoreCancel(s.runPersist(ctx, s.persisters)) })
		for inner, ctrl := range s.inner.Lock() {
			return ctrl.WaitUntil(ctx, func() bool {
				return inner.persistedBlockStart[s.PublicKey()] == qc.LaneRange(s.PublicKey()).First()
			})
		}
		panic("unreachable")
	}))
}

func repairM1080AssertRecovered(t *testing.T, s *State, f repairM1080Progress, app *types.AppQC, anchor int, next types.BlockNumber) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	c := s.Data().Committee()
	lane := s.PublicKey()
	require.NoError(t, utils.TestDiff(utils.Some(app), s.LastAppQC()))
	require.Equal(t, types.RoadIndex(anchor), s.FirstCommitQC())
	for _, want := range f.qcs[anchor:] {
		got, err := s.CommitQC(ctx, want.Index())
		require.NoError(t, err)
		require.NoError(t, utils.TestDiff(want, got))
		require.Equal(t, want.GlobalRange(c), got.GlobalRange(c))
	}
	first := f.qcs[anchor].LaneRange(lane).First()
	for inner := range s.inner.Lock() {
		require.Equal(t, first, inner.blocks[lane].first)
		require.Equal(t, next, inner.blocks[lane].next)
		require.Equal(t, next, inner.nextBlockToPersist[lane])
		require.Equal(t, first, inner.persistedBlockStart[lane])
		require.Equal(t, types.RoadIndex(len(f.qcs)), inner.commitQCs.next)
		require.Equal(t, first, inner.votes[lane].first)
		require.Equal(t, app.Proposal().GlobalNumber(), inner.appVotes.first)
	}
	require.Equal(t, next, s.NextBlock(lane))
	for n := first; n < next; n++ {
		block, err := s.Block(ctx, lane, n)
		require.NoError(t, err)
		require.NoError(t, utils.TestDiff(f.blocks[n], block))
	}
	if first > 0 {
		_, err := s.Block(ctx, lane, first-1)
		require.ErrorIs(t, err, data.ErrPruned)
	}
}

func TestRepairM_1080_LiveAnchorRecovery(t *testing.T) {
	rng := utils.TestRng()
	c, keys := types.GenCommittee(rng, 4)
	dir := utils.Some(t.TempDir())
	f := repairM1080CertifiedProgress(t, rng, c, keys, 5, dir)
	app := repairM1080AppQC(rng, c, keys, f.qcs[2])
	repairM1080PersistLiveAnchor(t, f.state, app, f.qcs[2])
	require.NoError(t, f.state.persisters.commitQCs.Close())
	for range 2 {
		s := repairM1080State(t, c, keys[0], dir)
		repairM1080AssertRecovered(t, s, f, app, 2, 5)
		require.NoError(t, s.persisters.commitQCs.Close())
	}
}

// Persist exactly the prefix of a runPersist batch ending with the durable
// anchor, before its WAL pruning/appends. This is the documented crash window.
func repairM1080AnchorCrash(t *testing.T, rng utils.Rng, c *types.Committee, keys []types.SecretKey, anchor, oldTail, commitTail int) {
	t.Helper()
	f := repairM1080CertifiedProgress(t, rng, c, keys, max(6, anchor+1, oldTail, commitTail), utils.None[string]())
	dir := utils.Some(t.TempDir())
	s := repairM1080State(t, c, keys[0], dir)
	lane := keys[0].Public()
	for _, block := range f.blocks[:oldTail] {
		require.NoError(t, s.PushBlock(t.Context(), block))
	}
	require.NoError(t, s.persisters.blocks.MaybePruneAndPersistLane(lane, utils.None[*types.CommitQC](), f.blocks[:oldTail], noBlockCB))
	require.NoError(t, s.persisters.commitQCs.MaybePruneAndPersist(utils.None[*types.CommitQC](), f.qcs[:commitTail], noCommitQCCB))
	app := repairM1080AppQC(rng, c, keys, f.qcs[anchor])
	require.NoError(t, s.PushAppQC(app, f.qcs[anchor]))
	batch, err := s.collectPersistBatch(t.Context(), 0)
	require.NoError(t, err)
	prune, ok := batch.pruneAnchor.Get()
	require.True(t, ok)
	require.NoError(t, s.persisters.pruneAnchor.Persist(PruneAnchorConv.Encode(prune)))
	require.NoError(t, s.persisters.commitQCs.Close())
	if anchor >= commitTail {
		f.qcs = f.qcs[:anchor+1]
	}
	first := f.qcs[anchor].LaneRange(lane).First()
	require.Equal(t, types.BlockNumber(anchor), first)
	if anchor > oldTail {
		require.Greater(t, first, types.BlockNumber(oldTail))
	}
	for range 2 {
		reopened := repairM1080State(t, c, keys[0], dir)
		repairM1080AssertRecovered(t, reopened, f, app, anchor, max(first, types.BlockNumber(oldTail)))
		require.NoError(t, reopened.persisters.commitQCs.Close())
		_, retained, err := persist.NewBlockPersister(dir)
		require.NoError(t, err)
		want := f.blocks[min(anchor, oldTail):oldTail]
		require.Len(t, retained[lane], len(want))
		for n, got := range retained[lane] {
			require.Equal(t, first+types.BlockNumber(n), got.Number)
			require.NoError(t, utils.TestDiff(want[n], got.Proposal))
		}
		cp, retainedQCs, err := persist.NewCommitQCPersister(dir)
		require.NoError(t, err)
		require.Len(t, retainedQCs, len(f.qcs)-anchor)
		for n, got := range retainedQCs {
			require.Equal(t, types.RoadIndex(anchor+n), got.Index)
			require.NoError(t, utils.TestDiff(f.qcs[anchor+n], got.QC))
		}
		require.NoError(t, cp.Close())
	}
}

func TestRepairM_1080_LaneTruncationBeyondTail(t *testing.T) {
	rng := utils.TestRng()
	c, keys := types.GenCommittee(rng, 4)
	repairM1080AnchorCrash(t, rng, c, keys, 4, 3, 6)
}

func TestRepairM_1080_LaneAnchorInsideTail(t *testing.T) {
	rng := utils.TestRng()
	c, keys := types.GenCommittee(rng, 4)
	repairM1080AnchorCrash(t, rng, c, keys, 2, 5, 6)
}

func repairM1080Snapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		files[path] = b
		return err
	}))
	return files
}

func TestRepairM_1080_RejectInvalidAnchorsBeforePruning(t *testing.T) {
	for _, name := range []string{"empty-range", "below-range", "at-range-end", "road-index", "app-committee", "commit-committee", "app-quorum", "commit-quorum", "app-signature", "commit-signature", "incomplete"} {
		t.Run(name, func(t *testing.T) {
			rng := utils.TestRng()
			c, keys := types.GenCommittee(rng, 4)
			dirPath := t.TempDir()
			dir := utils.Some(dirPath)
			f := repairM1080CertifiedProgress(t, rng, c, keys, 5, dir)
			qc := f.qcs[2]
			app := repairM1080AppQC(rng, c, keys, qc)
			switch name {
			case "empty-range":
				qc = makeCommitQC(c, keys, utils.Some(f.qcs[1]), nil, utils.None[*types.AppQC]())
				app = repairM1080AppQC(rng, c, keys, qc)
			case "below-range":
				app = types.NewAppQC(makeAppVotes(keys, types.NewAppProposal(qc.GlobalRange(c).First-1, qc.Index(), types.GenAppHash(rng))))
			case "at-range-end":
				app = types.NewAppQC(makeAppVotes(keys, types.NewAppProposal(qc.GlobalRange(c).Next, qc.Index(), types.GenAppHash(rng))))
			case "road-index":
				app = types.NewAppQC(makeAppVotes(keys, types.NewAppProposal(qc.GlobalRange(c).First, qc.Index()+1, types.GenAppHash(rng))))
			case "app-committee":
				_, otherKeys := types.GenCommittee(rng, 4)
				app = types.NewAppQC(makeAppVotes(otherKeys, app.Proposal()))
			case "commit-committee":
				_, otherKeys := types.GenCommittee(rng, 4)
				var votes []*types.Signed[*types.CommitVote]
				for _, key := range otherKeys {
					votes = append(votes, types.Sign(key, types.NewCommitVote(qc.Proposal())))
				}
				qc = types.NewCommitQC(votes)
			case "app-signature":
				other := types.NewAppProposal(qc.GlobalRange(c).Next, qc.Index(), types.GenAppHash(rng))
				votes := makeAppVotes(keys, other)
				votes[0] = types.Sign(keys[0], types.NewAppVote(app.Proposal()))
				app = types.NewAppQC(votes)
			case "commit-signature":
				var votes []*types.Signed[*types.CommitVote]
				for _, key := range keys {
					votes = append(votes, types.Sign(key, types.NewCommitVote(f.qcs[1].Proposal())))
				}
				votes[0] = types.Sign(keys[0], types.NewCommitVote(qc.Proposal()))
				qc = types.NewCommitQC(votes)
			case "app-quorum":
				app = types.NewAppQC(makeAppVotes(keys[:1], app.Proposal()))
			case "commit-quorum":
				qc = types.NewCommitQC([]*types.Signed[*types.CommitVote]{types.Sign(keys[0], types.NewCommitVote(qc.Proposal()))})
			}
			if name != "incomplete" {
				if name == "empty-range" || name == "below-range" || name == "at-range-end" || name == "road-index" {
					require.NoError(t, app.Verify(c))
					require.NoError(t, qc.Verify(c))
				}
				require.Error(t, f.state.PushAppQC(app, qc))
			}
			raw := PruneAnchorConv.Encode(&PruneAnchor{AppQC: app, CommitQC: qc})
			if name == "incomplete" {
				raw.CommitQc = nil
			}
			require.NoError(t, f.state.persisters.pruneAnchor.Persist(raw))
			require.NoError(t, f.state.persisters.commitQCs.Close())
			before := repairM1080Snapshot(t, dirPath)
			for range 2 {
				_, err := NewState(keys[0], f.state.Data(), dir)
				require.ErrorIs(t, err, ErrInvalidPruneAnchor)
				require.Equal(t, before, repairM1080Snapshot(t, dirPath), "invalid authority must not change persisted data")
			}
			bp, blocks, err := persist.NewBlockPersister(dir)
			require.NoError(t, err)
			require.NotNil(t, bp)
			require.Len(t, blocks[keys[0].Public()], 5)
			cp, qcs, err := persist.NewCommitQCPersister(dir)
			require.NoError(t, err)
			require.Len(t, qcs, 5)
			require.NoError(t, cp.Close())
		})
	}
}

func TestRepairM_1080_RecoveryFixtures(t *testing.T) {
	t.Run("commit-range", TestNewInnerLoadedCommitQCsWithAppQC)
	t.Run("retained-queues", TestNewInnerLoadedAllThree)
}
