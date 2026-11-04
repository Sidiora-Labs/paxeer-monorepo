package state

import (
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	blockstore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM1098Stores struct {
	dir    string
	states Store
	blocks *blockstore.BlockStore
	state  State
}

func newRepairM1098Stores(t *testing.T) *repairM1098Stores {
	t.Helper()
	key := ed25519.GenerateSecretKey()
	vals := types.NewValidatorSet([]*types.Validator{
		types.NewValidator(key.Public(), 10),
	})
	f := &repairM1098Stores{
		dir: t.TempDir(),
		state: State{
			Version: InitStateVersion, ChainID: "repair-m1098", InitialHeight: 1,
			Validators: vals, NextValidators: vals.Copy(), LastValidators: vals.Copy(),
			ConsensusParams: *types.DefaultConsensusParams(),
			LastBlockTime:   time.Unix(1, 0).UTC(),
		},
	}
	f.open(t)
	t.Cleanup(func() { f.close(t) })
	commit := new(types.Commit)
	for h := int64(1); h <= 6; h++ {
		// Omit the save that produces validators at height 5. PruneStates(5)
		// will delete consensus parameters, then fail on the missing validators.
		if h != 4 {
			f.save(t, h)
		}
		block := f.state.MakeBlockAt(h, nil, commit, nil,
			vals.GetProposer().Address, time.Unix(h, 0).UTC())
		require.NoError(t, block.ValidateBasic(types.DefaultConsensusPolicy()))
		parts, err := block.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)
		blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
		vote := &types.Vote{
			Type: tmproto.PrecommitType, Height: h, BlockID: blockID,
			Timestamp: time.Unix(h, 0).UTC(), ValidatorAddress: key.Public().Address(),
		}
		vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(f.state.ChainID, vote.ToProto())))
		commit = &types.Commit{Height: h, BlockID: blockID, Signatures: []types.CommitSig{vote.CommitSig()}}
		require.NoError(t, commit.ValidateBasic())
		f.blocks.SaveBlock(block, parts, commit)
		require.NoError(t, f.states.SaveFinalizeBlockResponses(h, &abci.ResponseFinalizeBlock{
			TxResults: []*abci.ExecTxResult{{Data: []byte{byte(h)}}},
		}))
		f.state.LastBlockHeight = h
		f.state.LastBlockID = blockID
		f.state.LastBlockTime = block.Time
	}
	return f
}

func (f *repairM1098Stores) open(t *testing.T) {
	t.Helper()
	stateDB, err := dbm.NewGoLevelDB("state", f.dir)
	require.NoError(t, err)
	f.states = NewStore(stateDB)
	blockDB, err := dbm.NewGoLevelDB("blocks", f.dir)
	require.NoError(t, err)
	f.blocks = blockstore.NewBlockStore(blockDB)
}

func (f *repairM1098Stores) close(t *testing.T) {
	t.Helper()
	if f.states != nil {
		require.NoError(t, f.states.Close())
		f.states = nil
	}
	if f.blocks != nil {
		require.NoError(t, f.blocks.Close())
		f.blocks = nil
	}
}

func (f *repairM1098Stores) save(t *testing.T, height int64) {
	t.Helper()
	s := f.state.Copy()
	s.LastBlockHeight = height - 1
	s.LastHeightValidatorsChanged = height + 1
	s.LastHeightConsensusParamsChanged = height
	require.NoError(t, f.states.Save(s))
}

func repairM1098Wait(t *testing.T, p *AsyncPruner) {
	t.Helper()
	// Observe the production worker's completion signal without polling or
	// stopping it, so another request exercises the same running component.
	p.worker.mtx.Lock()
	idle := p.worker.idle
	p.worker.mtx.Unlock()
	if idle != nil {
		select {
		case <-idle:
		case <-t.Context().Done():
			t.Fatal("pruning did not finish before the test context ended")
		}
	}
}

func (f *repairM1098Stores) failedPrune(t *testing.T) *AsyncPruner {
	t.Helper()
	p := newAsyncPruner(f.blocks, f.states, NopMetrics())
	t.Cleanup(p.stop)
	p.Request(5)
	repairM1098Wait(t, p)
	require.Error(t, p.Err())
	require.Zero(t, p.RetainedBase())
	require.Equal(t, int64(5), f.blocks.Base())
	require.Nil(t, f.blocks.LoadBlock(4))
	_, err := f.states.LoadConsensusParams(1)
	require.Error(t, err, "the state prune must have made partial progress")
	_, err = f.states.LoadFinalizeBlockResponses(1)
	require.NoError(t, err, "state pruning has not completed")
	return p
}

func (f *repairM1098Stores) requirePruned(t *testing.T, p *AsyncPruner, height int64) {
	t.Helper()
	require.NoError(t, p.Err())
	require.Equal(t, height, p.RetainedBase())
	require.Equal(t, height, f.blocks.Base())
	for h := int64(1); h < height; h++ {
		require.Nil(t, f.blocks.LoadBlock(h))
		_, err := f.states.LoadValidators(h)
		require.Error(t, err)
		_, err = f.states.LoadConsensusParams(h)
		require.Error(t, err)
		_, err = f.states.LoadFinalizeBlockResponses(h)
		require.ErrorAs(t, err, new(ErrNoFinalizeBlockResponsesForHeight))
	}
	for h := height; h <= 6; h++ {
		require.NotNil(t, f.blocks.LoadBlock(h))
		vals, err := f.states.LoadValidators(h)
		require.NoError(t, err)
		require.Equal(t, f.state.Validators.Hash(), vals.Hash())
		params, err := f.states.LoadConsensusParams(h)
		require.NoError(t, err)
		require.Equal(t, f.state.ConsensusParams, params)
		response, err := f.states.LoadFinalizeBlockResponses(h)
		require.NoError(t, err)
		require.Equal(t, []byte{byte(h)}, response.TxResults[0].Data)
	}
}

func TestRepairM_1098_RetryStatePruneAtBlockBase(t *testing.T) {
	f := newRepairM1098Stores(t)
	p := f.failedPrune(t)
	p.Request(5)
	repairM1098Wait(t, p)
	require.Error(t, p.Err(), "equal block base must retry the unfinished state prune")
	require.Zero(t, p.RetainedBase())
	f.save(t, 4)
	f.save(t, 6)
	p.Request(5)
	repairM1098Wait(t, p)
	f.requirePruned(t, p, 5)
	for _, height := range []int64{5, 3} {
		p.Request(height)
		repairM1098Wait(t, p)
		f.requirePruned(t, p, 5)
	}
}

func TestRepairM_1098_RestartResumesStatePrune(t *testing.T) {
	f := newRepairM1098Stores(t)
	p := f.failedPrune(t)
	p.stop()
	f.close(t)
	f.open(t)
	require.Equal(t, int64(5), f.blocks.Base())
	f.save(t, 4)
	f.save(t, 6)
	p = newAsyncPruner(f.blocks, f.states, NopMetrics())
	t.Cleanup(p.stop)
	// No new retain-height request: reopening reconstructs pending pruning.
	repairM1098Wait(t, p)
	f.requirePruned(t, p, 5)
	p.stop()
	f.close(t)
	f.open(t)
	p = newAsyncPruner(f.blocks, f.states, NopMetrics())
	t.Cleanup(p.stop)
	repairM1098Wait(t, p)
	f.requirePruned(t, p, 5)
}

func TestRepairM_1098_RestartKeepsFailedPrunePending(t *testing.T) {
	f := newRepairM1098Stores(t)
	p := f.failedPrune(t)
	p.stop()
	f.close(t)
	f.open(t)
	p = newAsyncPruner(f.blocks, f.states, NopMetrics())
	t.Cleanup(p.stop)
	repairM1098Wait(t, p)
	require.Error(t, p.Err())
	require.Zero(t, p.RetainedBase())
	p.Request(3)
	repairM1098Wait(t, p)
	require.Error(t, p.Err(), "a lower request must not acknowledge the pending boundary")
	require.Zero(t, p.RetainedBase())
	f.save(t, 4)
	f.save(t, 6)
	p.Request(3)
	repairM1098Wait(t, p)
	f.requirePruned(t, p, 5)
	p.Request(6)
	repairM1098Wait(t, p)
	f.requirePruned(t, p, 6)
}
