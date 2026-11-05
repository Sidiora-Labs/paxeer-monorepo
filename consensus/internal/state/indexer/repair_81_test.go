package indexer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb/opt"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repair81Chain struct {
	dir        string
	app        *kvstore.Application
	key        ed25519.SecretKey
	state      sm.State
	states     sm.Store
	blocks     *store.BlockStore
	progress   *dbm.GoLevelDB
	lastCommit *types.Commit
}

func newRepair81Chain(t *testing.T, initialHeight int64) *repair81Chain {
	t.Helper()
	c := &repair81Chain{dir: t.TempDir(), app: kvstore.NewApplication(), key: ed25519.TestSecretKey([]byte(t.Name())), lastCommit: new(types.Commit)}
	t.Cleanup(func() { require.NoError(t, c.app.Close()) })
	c.open(t)
	var err error
	c.state, err = sm.MakeGenesisState(&types.GenesisDoc{
		ChainID: "repair-81", InitialHeight: initialHeight,
		Validators: []types.GenesisValidator{{PubKey: c.key.Public(), Power: 10}},
	})
	require.NoError(t, err)
	require.NoError(t, c.states.Save(c.state))
	return c
}

func (c *repair81Chain) open(t *testing.T) {
	t.Helper()
	blockDB, err := dbm.NewGoLevelDB("blocks", c.dir)
	require.NoError(t, err)
	stateDB, err := dbm.NewGoLevelDB("state", c.dir)
	require.NoError(t, err)
	progressDB, err := dbm.NewGoLevelDB("progress", c.dir)
	require.NoError(t, err)
	c.blocks, c.states, c.progress = store.NewBlockStore(blockDB), sm.NewStore(stateDB), progressDB
	c.state, err = c.states.Load()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = blockDB.Close()
		_ = stateDB.Close()
		_ = progressDB.Close()
	})
}

func (c *repair81Chain) close(t *testing.T) {
	t.Helper()
	require.NoError(t, c.blocks.Close())
	require.NoError(t, c.states.Close())
	require.NoError(t, c.progress.Close())
}

func (c *repair81Chain) recovery() indexer.RecoveryArgs {
	return indexer.RecoveryArgs{BlockStore: c.blocks, StateStore: c.states, ProgressStore: c.progress}
}

// produce runs the real ABCI application and persists the same blocks, results
// and committed-state boundary consumed by restart replay.
func (c *repair81Chain) produce(t *testing.T, txs []types.Tx, committed bool) (*types.Block, *abci.ResponseFinalizeBlock) {
	t.Helper()
	height := max(c.state.LastBlockHeight+1, c.state.InitialHeight)
	block := c.state.MakeBlockAt(height, txs, c.lastCommit, nil, c.key.Public().Address(), time.Unix(1700000000+height, 0).UTC())
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	vote := &types.Vote{Type: tmproto.PrecommitType, Height: height, BlockID: blockID,
		Timestamp: block.Time, ValidatorAddress: c.key.Public().Address()}
	vote.Signature = utils.Some(c.key.Sign(types.VoteSignBytes(c.state.ChainID, vote.ToProto())))
	commit := &types.Commit{Height: height, BlockID: blockID, Signatures: []types.CommitSig{vote.CommitSig()}}
	c.blocks.SaveBlock(block, parts, commit)
	response, err := c.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{Header: block.Header.ToProto(), Txs: block.Data.Txs.ToSliceOfBytes()})
	require.NoError(t, err)
	require.NoError(t, c.states.SaveFinalizeBlockResponses(height, response))
	if committed {
		_, err = c.app.Commit(t.Context())
		require.NoError(t, err)
		c.state.LastBlockHeight = height
		c.state.LastBlockID = blockID
		c.state.LastBlockTime = block.Time
		c.state.LastValidators = c.state.Validators
		c.state.AppHash = response.AppHash
		results, err := abci.MarshalTxResults(response.TxResults)
		require.NoError(t, err)
		c.state.LastResultsHash = merkle.HashFromByteSlices(results)
		require.NoError(t, c.states.Save(c.state))
		c.lastCommit = commit
	}
	return block, response
}

func repair81Sink(t *testing.T, dir string) (indexer.EventSink, *dbm.GoLevelDB) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("index", dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return kv.NewEventSink(db), db
}

func repair81Bus(t *testing.T) *eventbus.EventBus {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	t.Cleanup(func() { cancel(); bus.Wait() })
	return bus
}

func repair81Service(t *testing.T, c *repair81Chain, sink indexer.EventSink, bus *eventbus.EventBus) *indexer.Service {
	t.Helper()
	svc := indexer.NewService(indexer.ServiceArgs{Sinks: []indexer.EventSink{sink}, EventBus: bus, Recovery: utils.Some(c.recovery())})
	require.NoError(t, svc.Start(t.Context()))
	t.Cleanup(svc.Stop)
	return svc
}

func repair81Height(t *testing.T, c *repair81Chain, want int64) {
	t.Helper()
	height, err := c.recovery().IndexedHeight(indexer.KV)
	require.NoError(t, err)
	require.Equal(t, want, height)
}

func repair81Search(t *testing.T, sink indexer.EventSink, block *types.Block, response *abci.ResponseFinalizeBlock) {
	t.Helper()
	heights, err := sink.SearchBlockEvents(t.Context(), query.MustCompile(fmt.Sprintf("block.height = %d", block.Height)))
	require.NoError(t, err)
	require.Equal(t, []int64{block.Height}, heights)
	txs, err := sink.SearchTxEvents(t.Context(), query.MustCompile(fmt.Sprintf("tx.height = %d", block.Height)))
	require.NoError(t, err)
	require.Len(t, txs, len(block.Txs))
	for i, tx := range block.Txs {
		got, err := sink.GetTxByHash(tx.Hash().Bytes())
		require.NoError(t, err)
		require.Equal(t, &abci.TxResultV2{Height: block.Height, Index: uint32(i), Tx: tx, Result: *response.TxResults[i]}, got)
	}
}

func repair81Subscribe(t *testing.T, bus *eventbus.EventBus, q *query.Query) eventbus.Subscription {
	t.Helper()
	sub, err := bus.SubscribeWithArgs(t.Context(), pubsub.SubscribeArgs{ClientID: t.Name(), Query: q, Limit: 20})
	require.NoError(t, err)
	return sub
}

func repair81Next(t *testing.T, sub eventbus.Subscription) pubsub.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	msg, err := sub.Next(ctx)
	require.NoError(t, err)
	return msg
}

func TestRepair_81_FailedWriteReopen(t *testing.T) {
	c := newRepair81Chain(t, 1)
	block1, res1 := c.produce(t, []types.Tx{types.Tx("one=first"), types.Tx("two=second")}, true)
	block2, res2 := c.produce(t, []types.Tx{types.Tx("three=third")}, true)
	dir := t.TempDir()
	sink, db := repair81Sink(t, dir)
	require.NoError(t, db.Close()) // Real backend failure; no sink or store test double.
	bus := repair81Bus(t)
	svc := indexer.NewService(indexer.ServiceArgs{Sinks: []indexer.EventSink{sink}, EventBus: bus, Recovery: utils.Some(c.recovery())})
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, svc.Start(ctx), context.DeadlineExceeded)
	repair81Height(t, c, 0)
	c.close(t)
	c.open(t)
	sink, _ = repair81Sink(t, dir)
	svc = repair81Service(t, c, sink, repair81Bus(t))
	repair81Height(t, c, 2)
	repair81Search(t, sink, block1, res1)
	repair81Search(t, sink, block2, res2)
	matches, err := sink.SearchTxEvents(t.Context(), query.MustCompile("app.key = 'two'"))
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, []byte("two=second"), matches[0].Tx)
	svc.Stop()
}

func TestRepair_81_CheckpointFailureReopen(t *testing.T) {
	c := newRepair81Chain(t, 1)
	block1, res1 := c.produce(t, []types.Tx{types.Tx("first=one")}, true)
	block2, res2 := c.produce(t, nil, true)
	require.NoError(t, c.progress.Close())
	readOnly, err := dbm.NewGoLevelDBWithOpts("progress", c.dir, &opt.Options{ReadOnly: true})
	require.NoError(t, err)
	c.progress = readOnly
	t.Cleanup(func() { _ = readOnly.Close() })
	sinkDir := t.TempDir()
	sink, _ := repair81Sink(t, sinkDir)
	svc := indexer.NewService(indexer.ServiceArgs{Sinks: []indexer.EventSink{sink}, EventBus: repair81Bus(t), Recovery: utils.Some(c.recovery())})
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, svc.Start(ctx), context.DeadlineExceeded)
	repair81Search(t, sink, block1, res1) // Writes succeeded, but were not acknowledged.
	repair81Height(t, c, 0)
	found, err := sink.HasBlock(2)
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, sink.Stop())
	c.close(t)
	c.open(t)
	sink, _ = repair81Sink(t, sinkDir)
	repair81Service(t, c, sink, repair81Bus(t))
	repair81Height(t, c, 2)
	repair81Search(t, sink, block1, res1) // Replay is idempotent.
	repair81Search(t, sink, block2, res2)
}

func TestRepair_81_LiveGapAndOfflineRecovery(t *testing.T) {
	c := newRepair81Chain(t, 1)
	sinkDir := t.TempDir()
	sink, _ := repair81Sink(t, sinkDir)
	bus := repair81Bus(t)
	sub := repair81Subscribe(t, bus, types.EventQueryTx)
	svc := repair81Service(t, c, sink, bus)
	block1, res1 := c.produce(t, []types.Tx{types.Tx("live=one")}, true)
	sm.FireEvents(bus, block1, types.BlockID{}, res1, nil)
	require.Equal(t, int64(1), repair81Next(t, sub).Data().(types.EventDataTx).Height)
	repair81Height(t, c, 1)
	block2, res2 := c.produce(t, []types.Tx{types.Tx("lost=two")}, true)
	// No live events reach the indexer for height two.
	block3, res3 := c.produce(t, []types.Tx{types.Tx("live=three")}, true)
	sm.FireEvents(bus, block3, types.BlockID{}, res3, nil)
	require.Equal(t, int64(3), repair81Next(t, sub).Data().(types.EventDataTx).Height)
	repair81Height(t, c, 3)
	repair81Search(t, sink, block1, res1)
	repair81Search(t, sink, block2, res2)
	repair81Search(t, sink, block3, res3)
	svc.Stop()
	c.close(t)
	c.open(t)
	repair81Height(t, c, 3)
	block4, res4 := c.produce(t, []types.Tx{types.Tx("offline=four")}, true)
	require.NoError(t, c.states.PruneFinalizeBlockResponses(4))
	// The persistent checkpoint permits startup after earlier results are pruned.
	sink, _ = repair81Sink(t, sinkDir)
	bus = repair81Bus(t)
	sub = repair81Subscribe(t, bus, types.EventQueryTx)
	svc = repair81Service(t, c, sink, bus)
	repair81Height(t, c, 4)
	repair81Search(t, sink, block4, res4)
	// Duplicated queued events after replay must not start a stale batch.
	sm.FireEvents(bus, block4, types.BlockID{}, res4, nil)
	require.Equal(t, int64(4), repair81Next(t, sub).Data().(types.EventDataTx).Height)
	block5, res5 := c.produce(t, nil, true)
	headers := repair81Subscribe(t, bus, types.EventQueryNewBlockHeader)
	sm.FireEvents(bus, block5, types.BlockID{}, res5, nil)
	require.Equal(t, int64(5), repair81Next(t, headers).Data().(types.EventDataNewBlockHeader).Header.Height)
	repair81Height(t, c, 5)
	repair81Search(t, sink, block5, res5)
	svc.Stop()
}

func TestRepair_81_MissingHistoryDoesNotAdvance(t *testing.T) {
	c := newRepair81Chain(t, 1)
	c.produce(t, []types.Tx{types.Tx("missing=one")}, true)
	c.produce(t, []types.Tx{types.Tx("later=two")}, true)
	require.NoError(t, c.states.PruneFinalizeBlockResponses(2))
	sink, _ := repair81Sink(t, t.TempDir())
	svc := indexer.NewService(indexer.ServiceArgs{Sinks: []indexer.EventSink{sink}, EventBus: repair81Bus(t), Recovery: utils.Some(c.recovery())})
	require.Error(t, svc.Start(t.Context()))
	repair81Height(t, c, 0)
	found, err := sink.HasBlock(2)
	require.NoError(t, err)
	require.False(t, found)
}

func TestRepair_81_ReplayOnlyCommittedHeight(t *testing.T) {
	c := newRepair81Chain(t, 5)
	block, res := c.produce(t, []types.Tx{types.Tx("committed=five")}, true)
	pending, _ := c.produce(t, []types.Tx{types.Tx("uncommitted=six")}, false)
	sink, _ := repair81Sink(t, t.TempDir())
	repair81Service(t, c, sink, repair81Bus(t))
	repair81Height(t, c, 5)
	repair81Search(t, sink, block, res)
	found, err := sink.HasBlock(pending.Height)
	require.NoError(t, err)
	require.False(t, found)
	tx, err := sink.GetTxByHash(pending.Txs[0].Hash().Bytes())
	require.NoError(t, err)
	require.Nil(t, tx)
}
