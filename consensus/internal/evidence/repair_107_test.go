package evidence_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/evidence"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepair_107_CommittedEvidenceSurvivesRestart(t *testing.T) {
	for _, locallyPending := range []bool{true, false} {
		name := "never_pending"
		if locallyPending {
			name = "pending"
		}
		t.Run(name, func(t *testing.T) {
			f := newRepair107Fixture(t)
			dir := t.TempDir()
			pool := f.openPool(t, dir, false, f.state)
			if locallyPending {
				for _, ev := range f.offenses {
					require.NoError(t, pool.AddEvidence(t.Context(), ev))
				}
				require.Equal(t, uint32(len(f.offenses)), pool.Size())
			}

			next, block := f.saveCommittedBlock(t)
			var lifecycle sm.EvidencePool = pool
			lifecycle.Update(t.Context(), next, block.Evidence)
			require.Zero(t, pool.Size())
			require.Nil(t, pool.EvidenceFront())
			require.NoError(t, f.states.Save(next))
			require.NoError(t, pool.Close())

			recovered, err := f.states.Load()
			require.NoError(t, err)
			pool = f.openPool(t, dir, false, recovered)
			defer func() { require.NoError(t, pool.Close()) }()
			repair107AssertCommitted(t, pool, block.Evidence)

			// Consensus can report an offense again after restart. The real
			// producer must also consult the recovered committed markers.
			for _, ev := range block.Evidence {
				dve := ev.(*types.DuplicateVoteEvidence)
				pool.ReportConflictingVotes(dve.VoteA.Vote, dve.VoteB.Vote)
			}
			recovered.LastBlockHeight++
			recovered.LastBlockTime = recovered.LastBlockTime.Add(time.Minute)
			pool.Update(t.Context(), recovered, nil)
			repair107AssertCommitted(t, pool, block.Evidence)
		})
	}
}

func TestRepair_107_FailedMarkerStopsLifecycleAndReplays(t *testing.T) {
	for _, locallyPending := range []bool{true, false} {
		name := "never_pending"
		if locallyPending {
			name = "pending"
		}
		t.Run(name, func(t *testing.T) {
			f := newRepair107Fixture(t)
			dir := t.TempDir()
			pool := f.openPool(t, dir, false, f.state)
			if locallyPending {
				for _, ev := range f.offenses {
					require.NoError(t, pool.AddEvidence(t.Context(), ev))
				}
			}
			require.NoError(t, pool.Close())

			// The real backend accepts reads and batch staging, but fails the
			// synchronous marker/deletion transaction with ErrReadOnly.
			pool = f.openPool(t, dir, true, f.state)
			pendingSize := pool.Size()
			next, block := f.saveCommittedBlock(t)
			var lifecycle sm.EvidencePool = pool
			repair107RequirePanicError(t, leveldb.ErrReadOnly, func() {
				lifecycle.Update(t.Context(), next, block.Evidence)
			})
			require.Equal(t, f.state.LastBlockHeight, pool.State().LastBlockHeight)
			require.Equal(t, pendingSize, pool.Size())
			if locallyPending {
				require.NotNil(t, pool.EvidenceFront())
			}
			pending, size := lifecycle.PendingEvidence(-1)
			require.Empty(t, pending)
			require.Zero(t, size)
			for _, ev := range block.Evidence {
				require.ErrorIs(t, lifecycle.AddEvidence(t.Context(), ev), leveldb.ErrReadOnly)
			}
			require.ErrorIs(t, lifecycle.CheckEvidence(t.Context(), block.Evidence), leveldb.ErrReadOnly)
			require.NoError(t, pool.Close())

			// Restart at the last saved state, then replay the block from the
			// real block store before resuming proposal or block validation.
			recovered, err := f.states.Load()
			require.NoError(t, err)
			require.Equal(t, f.state.LastBlockHeight, recovered.LastBlockHeight)
			pool = f.openPool(t, dir, false, recovered)
			require.Equal(t, pendingSize, pool.Size())
			pending, _ = pool.PendingEvidence(-1)
			if locallyPending {
				require.Len(t, pending, len(f.offenses))
			}
			replayed := f.blocks.LoadBlock(next.LastBlockHeight)
			require.NotNil(t, replayed)
			pool.Update(t.Context(), next, replayed.Evidence)
			repair107AssertCommitted(t, pool, replayed.Evidence)
			require.NoError(t, f.states.Save(next))
			require.NoError(t, pool.Close())

			// The successful replay must itself survive another disk reopen.
			pool = f.openPool(t, dir, false, next)
			defer func() { require.NoError(t, pool.Close()) }()
			repair107AssertCommitted(t, pool, replayed.Evidence)
		})
	}
}

func TestRepair_107_ReadErrorsRejectDuplicates(t *testing.T) {
	for _, status := range []string{"unseen", "pending", "committed"} {
		t.Run(status, func(t *testing.T) {
			f := newRepair107Fixture(t)
			pool := f.openPool(t, t.TempDir(), false, f.state)
			if status != "unseen" {
				for _, ev := range f.offenses {
					require.NoError(t, pool.AddEvidence(t.Context(), ev))
				}
			}
			if status == "committed" {
				next, block := f.saveCommittedBlock(t)
				pool.Update(t.Context(), next, block.Evidence)
			}
			require.NoError(t, pool.Close())

			// A closed disk backend reports ErrClosed instead of "not found".
			// In particular, CheckEvidence must not return success after
			// merely logging failed reads or failed pending writes.
			for _, ev := range f.offenses {
				require.ErrorIs(t, pool.AddEvidence(t.Context(), ev), leveldb.ErrClosed)
				require.ErrorIs(t, pool.CheckEvidence(t.Context(), types.EvidenceList{ev}), leveldb.ErrClosed)
			}
			pending, size := pool.PendingEvidence(-1)
			require.Empty(t, pending)
			require.Zero(t, size)

			dve := f.offenses[0].(*types.DuplicateVoteEvidence)
			pool.ReportConflictingVotes(dve.VoteA.Vote, dve.VoteB.Vote)
			next := pool.State()
			next.LastBlockHeight++
			next.LastBlockTime = next.LastBlockTime.Add(time.Minute)
			repair107RequirePanicError(t, leveldb.ErrClosed, func() {
				pool.Update(t.Context(), next, nil)
			})
		})
	}
}

func TestRepair_107_BufferCannotReadmitCommittedEvidence(t *testing.T) {
	f := newRepair107Fixture(t)
	pool := f.openPool(t, t.TempDir(), false, f.state)
	defer func() { require.NoError(t, pool.Close()) }()
	for _, ev := range f.offenses {
		dve := ev.(*types.DuplicateVoteEvidence)
		pool.ReportConflictingVotes(dve.VoteA.Vote, dve.VoteB.Vote)
	}
	next, block := f.saveCommittedBlock(t)
	pool.Update(t.Context(), next, block.Evidence)
	repair107AssertCommitted(t, pool, block.Evidence)
}

func repair107AssertCommitted(t *testing.T, pool *evidence.Pool, offenses types.EvidenceList) {
	t.Helper()
	for _, ev := range offenses {
		// Exercise the evidence wire codec before peer admission and block checking.
		incoming := repair107WireEvidence(t, ev)
		require.NoError(t, pool.AddEvidence(t.Context(), incoming))
		var invalid *types.ErrInvalidEvidence
		require.ErrorAs(t, pool.CheckEvidence(t.Context(), types.EvidenceList{incoming}), &invalid)
	}
	pending, size := pool.PendingEvidence(-1)
	require.Empty(t, pending)
	require.Zero(t, size)
	require.Zero(t, pool.Size())
	require.Nil(t, pool.EvidenceFront())
}

func repair107RequirePanicError(t *testing.T, want error, action func()) {
	t.Helper()
	defer func() {
		value := recover()
		err, ok := value.(error)
		require.True(t, ok, "commit lifecycle must panic with its storage error, got %v", value)
		require.True(t, errors.Is(err, want), "commit lifecycle must preserve its storage error: %v", err)
	}()
	action()
	t.Fatal("commit lifecycle continued after a storage failure")
}

type repair107Fixture struct {
	key      ed25519.SecretKey
	states   sm.Store
	blocks   *store.BlockStore
	state    sm.State
	offenses types.EvidenceList
}

func newRepair107Fixture(t *testing.T) *repair107Fixture {
	t.Helper()
	f := &repair107Fixture{key: ed25519.TestSecretKey([]byte(t.Name()))}
	genesis := &types.GenesisDoc{
		ChainID:         evidenceChainID,
		InitialHeight:   1,
		GenesisTime:     defaultEvidenceTime,
		ConsensusParams: types.DefaultConsensusParams(),
		Validators:      []types.GenesisValidator{{PubKey: f.key.Public(), Power: 10}},
	}
	var err error
	f.state, err = sm.MakeGenesisState(genesis)
	require.NoError(t, err)
	f.states = sm.NewStore(dbm.NewMemDB())
	f.blocks = store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() {
		require.NoError(t, f.states.Close())
		require.NoError(t, f.blocks.Close())
	})
	require.NoError(t, f.states.Save(f.state))
	lastCommit := &types.Commit{}
	for height := int64(1); height <= 2; height++ {
		block := f.state.MakeBlockAt(height, nil, lastCommit, nil,
			f.key.Public().Address(), defaultEvidenceTime.Add(time.Duration(height)*time.Minute))
		lastCommit = f.saveBlock(t, block)
		f.state.LastBlockHeight = height
		f.state.LastBlockTime = block.Time
		f.state.LastBlockID = lastCommit.BlockID
		f.state.LastValidators = f.state.Validators.Copy()
		require.NoError(t, f.states.Save(f.state))
	}

	// Use the real consensus producer, then its protobuf transport, rather
	// than inserting encoded evidence or markers directly into the database.
	producer := evidence.NewPool(dbm.NewMemDB(), f.states, f.blocks, evidence.NopMetrics(), nil)
	require.NoError(t, producer.Start(f.state))
	for round := int32(0); round < 2; round++ {
		hashA := sha256.Sum256([]byte{byte(round), 0})
		hashB := sha256.Sum256([]byte{byte(round), 1})
		voteA := f.signedVote(1, round, types.BlockID{Hash: hashA[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: hashA[:]}}, defaultEvidenceTime.Add(time.Minute))
		voteB := f.signedVote(1, round, types.BlockID{Hash: hashB[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: hashB[:]}}, defaultEvidenceTime.Add(time.Minute))
		producer.ReportConflictingVotes(voteA, voteB)
	}
	next := f.state
	next.LastBlockHeight++
	next.LastBlockTime = next.LastBlockTime.Add(time.Minute)
	producer.Update(t.Context(), next, nil)
	produced, _ := producer.PendingEvidence(-1)
	require.Len(t, produced, 2)
	for _, ev := range produced {
		f.offenses = append(f.offenses, repair107WireEvidence(t, ev))
	}
	require.NoError(t, producer.Close())
	return f
}

func (f *repair107Fixture) signedVote(height int64, round int32, blockID types.BlockID, timestamp time.Time) *types.Vote {
	vote := &types.Vote{
		Type:             tmproto.PrecommitType,
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        timestamp,
		ValidatorAddress: f.key.Public().Address(),
		ValidatorIndex:   0,
	}
	vote.Signature = utils.Some(f.key.Sign(types.VoteSignBytes(evidenceChainID, vote.ToProto())))
	return vote
}

func (f *repair107Fixture) saveBlock(t *testing.T, block *types.Block) *types.Commit {
	t.Helper()
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	vote := f.signedVote(block.Height, 0, blockID, block.Time)
	commit := &types.Commit{Height: block.Height, BlockID: blockID, Signatures: []types.CommitSig{vote.CommitSig()}}
	f.blocks.SaveBlock(block, parts, commit)
	return commit
}

func (f *repair107Fixture) saveCommittedBlock(t *testing.T) (sm.State, *types.Block) {
	t.Helper()
	next := f.state
	next.LastBlockHeight++
	next.LastBlockTime = next.LastBlockTime.Add(time.Minute)
	block := f.state.MakeBlockAt(next.LastBlockHeight, nil, f.blocks.LoadSeenCommit(),
		f.offenses, f.key.Public().Address(), next.LastBlockTime)
	commit := f.saveBlock(t, block)
	next.LastBlockID = commit.BlockID
	next.LastValidators = f.state.Validators.Copy()
	stored := f.blocks.LoadBlock(block.Height)
	require.NotNil(t, stored)
	require.Len(t, stored.Evidence, len(f.offenses))
	return next, stored
}

func (f *repair107Fixture) openPool(t *testing.T, dir string, readOnly bool, state sm.State) *evidence.Pool {
	t.Helper()
	db, err := dbm.NewGoLevelDBWithOpts("repair107", dir, &opt.Options{ReadOnly: readOnly})
	require.NoError(t, err)
	pool := evidence.NewPool(db, f.states, f.blocks, evidence.NopMetrics(), nil)
	require.NoError(t, pool.Start(state))
	return pool
}

func repair107WireEvidence(t *testing.T, ev types.Evidence) types.Evidence {
	t.Helper()
	outgoing, err := types.EvidenceToProto(ev)
	require.NoError(t, err)
	wire, err := outgoing.Marshal()
	require.NoError(t, err)
	var incoming tmproto.Evidence
	require.NoError(t, incoming.Unmarshal(wire))
	decoded, err := types.EvidenceFromProto(&incoming)
	require.NoError(t, err)
	return decoded
}
