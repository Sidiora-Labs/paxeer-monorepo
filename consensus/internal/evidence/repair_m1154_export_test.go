package evidence

import (
	"context"
	"crypto/sha256"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	pb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func RepairM1154RepeatedStartAndRestart(t *testing.T) {
	f := newRepairM1154Fixture(t)
	dir := t.TempDir()
	db := repairM1154DB(t, dir)
	pool := NewPool(db, f.states, f.blocks, NopMetrics(), nil)
	require.NoError(t, pool.Start(f.state))
	for _, ev := range f.offenses {
		require.NoError(t, pool.AddEvidence(t.Context(), ev))
	}
	require.NoError(t, pool.Close())

	db = repairM1154DB(t, dir)
	pool = NewPool(db, f.states, f.blocks, NopMetrics(), nil)
	defer func() { require.NoError(t, pool.Close()) }()
	recovered, err := f.states.Load()
	require.NoError(t, err)
	const attempts = 8
	results := make(chan error, attempts)
	var starts sync.WaitGroup
	for range attempts {
		starts.Go(func() { results <- pool.Start(recovered) })
	}
	starts.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.EqualError(t, err, "pool is already running")
		}
	}
	require.Equal(t, 1, successes)
	repairM1154AssertPending(t, pool, f.offenses)
	changed := recovered
	changed.LastBlockHeight++
	changed.ChainID = "must-not-replace-state"
	require.EqualError(t, pool.Start(changed), "pool is already running")
	require.Equal(t, recovered, pool.State())
	repairM1154AssertPending(t, pool, f.offenses)
}

func RepairM1154FailedStartRetry(t *testing.T) {
	f := newRepairM1154Fixture(t)
	dir := t.TempDir()
	db := repairM1154DB(t, dir)
	producer := NewPool(db, f.states, f.blocks, NopMetrics(), nil)
	require.NoError(t, producer.Start(f.state))
	for _, ev := range f.offenses {
		require.NoError(t, producer.AddEvidence(t.Context(), ev))
	}
	require.NoError(t, producer.Close())
	db = repairM1154DB(t, dir)
	pool := NewPool(db, f.states, f.blocks, NopMetrics(), nil)
	defer func() { require.NoError(t, pool.Close()) }()
	// Corrupt the second persisted record, after recovery has decoded a valid one.
	iter, err := dbm.IteratePrefix(db, prefixToBytes(prefixPending))
	require.NoError(t, err)
	require.True(t, iter.Valid())
	iter.Next()
	require.True(t, iter.Valid())
	key, value := append([]byte(nil), iter.Key()...), append([]byte(nil), iter.Value()...)
	require.NoError(t, iter.Close())
	require.NoError(t, db.SetSync(key, []byte{0xff}))
	require.Error(t, pool.Start(f.state))
	require.Equal(t, sm.State{}, pool.State())
	require.Zero(t, pool.Size())
	require.Nil(t, pool.EvidenceFront())
	require.False(t, pool.isStarted)
	require.NoError(t, db.SetSync(key, value))
	require.NoError(t, pool.Start(f.state))
	repairM1154AssertPending(t, pool, f.offenses)
	require.EqualError(t, pool.Start(f.state), "pool is already running")
	repairM1154AssertPending(t, pool, f.offenses)
}

func RepairM1154ExpiredRecovery(t *testing.T) {
	f := newRepairM1154Fixture(t)
	dir := t.TempDir()
	db := repairM1154DB(t, dir)
	pool := NewPool(db, f.states, f.blocks, NopMetrics(), nil)
	require.NoError(t, pool.Start(f.state))
	for _, ev := range f.offenses {
		require.NoError(t, pool.AddEvidence(t.Context(), ev))
	}
	require.NoError(t, pool.Close())
	expired := f.state
	expired.LastBlockHeight += expired.ConsensusParams.Evidence.MaxAgeNumBlocks + 1
	expired.LastBlockTime = expired.LastBlockTime.Add(expired.ConsensusParams.Evidence.MaxAgeDuration + time.Second)
	readOnly, err := dbm.NewGoLevelDBWithOpts("repair-m1154", dir, &opt.Options{ReadOnly: true})
	require.NoError(t, err)
	pool = NewPool(readOnly, f.states, f.blocks, NopMetrics(), nil)
	require.ErrorIs(t, pool.Start(expired), leveldb.ErrReadOnly)
	require.Equal(t, sm.State{}, pool.State())
	require.Zero(t, pool.Size())
	require.Nil(t, pool.EvidenceFront())
	require.False(t, pool.isStarted)
	require.NoError(t, pool.Close())
	db = repairM1154DB(t, dir)
	pool = NewPool(db, f.states, f.blocks, NopMetrics(), nil)
	require.NoError(t, pool.Start(expired))
	require.Zero(t, pool.Size())
	require.Nil(t, pool.EvidenceFront())
	pending, size := pool.PendingEvidence(-1)
	require.Empty(t, pending)
	require.Zero(t, size)
	for _, ev := range f.offenses {
		present, err := db.Has(keyPending(ev))
		require.NoError(t, err)
		require.False(t, present)
	}
	require.NoError(t, pool.Close())
}

func RepairM1154ShutdownJoinsUsers(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		name := "Stop"
		if cancelParent {
			name = "parent_cancellation"
		}
		t.Run(name, func(t *testing.T) {
			f := newRepairM1154Fixture(t)
			dir := t.TempDir()
			db := repairM1154DB(t, dir)
			pool := NewPool(db, f.states, f.blocks, NopMetrics(), nil)
			require.NoError(t, pool.Start(f.state))
			require.NoError(t, pool.AddEvidence(t.Context(), f.offenses[0]))
			network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 2})
			nodes := network.Nodes()
			reactor, err := NewReactor(nodes[0].Router, pool)
			require.NoError(t, err)
			peer, err := p2p.OpenChannel(nodes[1].Router, GetChannelDescriptor())
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			require.NoError(t, reactor.Start(ctx))
			t.Cleanup(reactor.Stop)
			network.Start(t)
			deadline, stopDeadline := context.WithTimeout(t.Context(), 10*time.Second)
			defer stopDeadline()
			// A received message proves the peer update and broadcast path are active.
			outgoing, err := peer.Recv(deadline)
			require.NoError(t, err)
			decoded, err := types.EvidenceFromProto(outgoing.Message)
			require.NoError(t, err)
			require.Equal(t, f.offenses[0].Hash(), decoded.Hash())
			reactor.mtx.Lock()
			activePeers := len(reactor.peerRoutines)
			reactor.mtx.Unlock()
			require.Equal(t, 1, activePeers)

			// Hold the real state mutex while inbound AddEvidence owns the store mutex.
			// This pins an actual store user in verification without replacing any code.
			pool.mtx.Lock()
			var release sync.Once
			unlock := func() { release.Do(pool.mtx.Unlock) }
			defer unlock()
			incoming, err := types.EvidenceToProto(f.offenses[1])
			require.NoError(t, err)
			peer.Send(incoming, nodes[0].NodeID)
			require.Eventually(t, func() bool {
				if pool.storeMtx.TryLock() {
					pool.storeMtx.Unlock()
					return false
				}
				return true
			}, 10*time.Second, time.Millisecond, "inbound AddEvidence did not reach verification")
			stopped := make(chan struct{})
			if cancelParent {
				cancel()
			}
			go func() { reactor.Stop(); close(stopped) }()
			require.Eventually(t, func() bool {
				reactor.mtx.Lock()
				defer reactor.mtx.Unlock()
				return len(reactor.peerRoutines) == 0
			}, 10*time.Second, time.Millisecond, "peer broadcast did not terminate")
			select {
			case <-stopped:
				t.Fatal("Stop returned with AddEvidence in flight")
			default:
			}
			_, err = db.Has(keyPending(f.offenses[0]))
			require.NoError(t, err, "store closed before the inbound worker completed")
			unlock()
			select {
			case <-stopped:
			case <-deadline.Done():
				t.Fatal("Stop did not join evidence workers")
			}
			require.False(t, reactor.IsRunning())
			select {
			case <-reactor.workersDone:
			default:
				t.Fatal("worker scope still running after Stop")
			}
			reactor.mtx.Lock()
			remainingPeers := len(reactor.peerRoutines)
			reactor.mtx.Unlock()
			require.Zero(t, remainingPeers)
			_, err = db.Has(keyPending(f.offenses[0]))
			require.ErrorIs(t, err, leveldb.ErrClosed)
			require.Equal(t, uint32(2), pool.Size())

			// The stopped reactor leaves subsequent network messages queued, with no
			// pool activity. Reopening the real store also proves all ownership ended.
			reopened := repairM1154DB(t, dir)
			recovered := NewPool(reopened, f.states, f.blocks, NopMetrics(), nil)
			defer func() { require.NoError(t, recovered.Close()) }()
			require.NoError(t, recovered.Start(f.state))
			repairM1154AssertPending(t, recovered, f.offenses)
			peer.Send(incoming, nodes[0].NodeID)
			require.Eventually(t, func() bool { return reactor.channel.ReceiveLen() == 1 }, 10*time.Second, time.Millisecond)
			require.Equal(t, uint32(2), pool.Size())
			quiet, stopQuiet := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer stopQuiet()
			_, err = peer.Recv(quiet)
			require.ErrorIs(t, err, context.DeadlineExceeded, "broadcast continued after Stop")
			repairM1154AssertPending(t, recovered, f.offenses)
		})
	}
}

func repairM1154DB(t *testing.T, dir string) *dbm.GoLevelDB {
	t.Helper()
	db, err := dbm.NewGoLevelDB("repair-m1154", dir)
	require.NoError(t, err)
	return db
}

func repairM1154AssertPending(t *testing.T, pool *Pool, want types.EvidenceList) {
	t.Helper()
	pending, size := pool.PendingEvidence(-1)
	require.ElementsMatch(t, want, pending)
	require.Positive(t, size)
	require.Equal(t, uint32(len(want)), pool.Size())
	var gossiped types.EvidenceList
	for next := pool.EvidenceFront(); next != nil; next = next.Next() {
		gossiped = append(gossiped, next.Value())
	}
	require.ElementsMatch(t, want, gossiped)
}

type repairM1154Fixture struct {
	state    sm.State
	states   sm.Store
	blocks   *store.BlockStore
	offenses types.EvidenceList
}

func newRepairM1154Fixture(t *testing.T) *repairM1154Fixture {
	t.Helper()
	key := ed25519.TestSecretKey([]byte(t.Name()))
	timestamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	genesis := &types.GenesisDoc{ChainID: "repair-m1154", InitialHeight: 1, GenesisTime: timestamp,
		ConsensusParams: types.DefaultConsensusParams(), Validators: []types.GenesisValidator{{PubKey: key.Public(), Power: 10}}}
	state, err := sm.MakeGenesisState(genesis)
	require.NoError(t, err)
	f := &repairM1154Fixture{state: state, states: sm.NewStore(dbm.NewMemDB()), blocks: store.NewBlockStore(dbm.NewMemDB())}
	t.Cleanup(func() { require.NoError(t, f.states.Close()); require.NoError(t, f.blocks.Close()) })
	require.NoError(t, f.states.Save(state))
	sign := func(height int64, round int32, id types.BlockID) *types.Vote {
		vote := &types.Vote{Type: pb.PrecommitType, Height: height, Round: round, BlockID: id,
			Timestamp: timestamp, ValidatorAddress: key.Public().Address(), ValidatorIndex: 0}
		vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(state.ChainID, vote.ToProto())))
		return vote
	}
	block := state.MakeBlockAt(1, nil, &types.Commit{}, nil, key.Public().Address(), timestamp)
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	id := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	commit := &types.Commit{Height: 1, BlockID: id, Signatures: []types.CommitSig{sign(1, 0, id).CommitSig()}}
	f.blocks.SaveBlock(block, parts, commit)
	f.state.LastBlockHeight = 1
	f.state.LastBlockID = id
	f.state.LastValidators = state.Validators.Copy()
	require.NoError(t, f.states.Save(f.state))
	producer := NewPool(dbm.NewMemDB(), f.states, f.blocks, NopMetrics(), nil)
	require.NoError(t, producer.Start(f.state))
	for round := int32(0); round < 2; round++ {
		hashA := sha256.Sum256([]byte{byte(round), 0})
		hashB := sha256.Sum256([]byte{byte(round), 1})
		a := types.BlockID{Hash: hashA[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: hashA[:]}}
		b := types.BlockID{Hash: hashB[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: hashB[:]}}
		producer.ReportConflictingVotes(sign(1, round, a), sign(1, round, b))
	}
	next := f.state
	next.LastBlockHeight++
	producer.Update(t.Context(), next, nil)
	pending, _ := producer.PendingEvidence(-1)
	require.Len(t, pending, 2)
	for _, ev := range pending {
		outgoing, err := types.EvidenceToProto(ev)
		require.NoError(t, err)
		wire, err := outgoing.Marshal()
		require.NoError(t, err)
		var incoming pb.Evidence
		require.NoError(t, incoming.Unmarshal(wire))
		decoded, err := types.EvidenceFromProto(&incoming)
		require.NoError(t, err)
		f.offenses = append(f.offenses, decoded)
	}
	require.NoError(t, producer.Close())
	return f
}
