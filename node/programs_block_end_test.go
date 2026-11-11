package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func programsBlockEndRequest(f *hostedProgramsABIFixture, height int64) *abci.RequestFinalizeBlock {
	block := tmtypes.MakeBlock(height, nil, &tmtypes.Commit{}, nil)
	block.ChainID = f.ctx.ChainID()
	block.Time = f.ctx.BlockTime()
	block.ProposerAddress = []byte(f.proposer)
	validatorsHash := sha256.Sum256(f.proposer)
	block.ValidatorsHash = validatorsHash[:]
	block.NextValidatorsHash = block.ValidatorsHash
	block.AppHash = f.app.LastCommitID().Hash
	block.LastBlockID.Hash = f.app.LastBlockID().Hash
	return &abci.RequestFinalizeBlock{Hash: block.Hash(), Header: block.Header.ToProto()}
}

func programsBlockEndCommit(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	_, err := f.app.Commit(context.Background())
	require.NoError(t, err)
	f.ctx = f.app.NewUncachedContext(false, tmproto.Header{
		ChainID: f.ctx.ChainID(), Height: f.app.LastBlockHeight() + 1, Time: f.ctx.BlockTime(),
	})
}

func programsBlockEndIdleBlocks(t *testing.T, f *hostedProgramsABIFixture, first, count int64) {
	t.Helper()
	for height := first; height < first+count; height++ {
		programsBlockEndStep(t, f, height, false)
	}
}

func programsBlockEndStep(t *testing.T, f *hostedProgramsABIFixture, height int64, call bool) time.Duration {
	t.Helper()
	f.ctx = f.ctx.WithBlockHeight(height)
	req := programsBlockEndRequest(f, height)
	var beforeSequence uint64
	if call {
		var err error
		beforeSequence, err = lxphost.NextSequence(f.ctx, f.app.GetKey(LayerXHostStoreKey))
		require.NoError(t, err)
		proposal, err := f.app.ProcessProposal(context.Background(), &abci.RequestProcessProposal{
			Hash: req.Hash, Header: req.Header,
		})
		require.NoError(t, err)
		require.Equal(t, abci.ResponseProcessProposal_ACCEPT, proposal.Status)
		info := f.app.GetOptimisticProcessingInfo()
		require.NotNil(t, info.Completion)
		select {
		case <-info.Completion:
		case <-time.After(30 * time.Second):
			t.Fatal("optimistic block processing did not finish")
		}
		info = f.app.GetOptimisticProcessingInfo()
		require.False(t, info.Aborted)
		f.ctx = info.StateCtx.WithBlockHeader(*req.Header)
		receipt := f.call(t)
		require.Zero(t, receipt.ResultCode, "height %d: %s", height, receipt.ResultCode.Error())
		require.NotNil(t, receipt.ProgramOutcome)
		require.Zero(t, receipt.ProgramOutcome.Status)
		hostedProgramsABIReceiptVersion(t, receipt, 5)
	}
	_, err := f.app.FinalizeBlock(context.Background(), req)
	require.NoError(t, err, "height %d", height)
	duration := f.app.programsOccupancyDuration
	programsBlockEndCommit(t, f)
	if call {
		afterSequence, err := lxphost.NextSequence(f.app.GetCheckCtx(), f.app.GetKey(LayerXHostStoreKey))
		require.NoError(t, err)
		require.Equal(t, beforeSequence+2, afterSequence, "call and transition must both commit at height %d", height)
	}
	return duration
}

func programsBlockEndHostRows(t *testing.T, ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	t.Helper()
	rows := make(map[string][]byte)
	iter := ctx.KVStore(key).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		rows[string(iter.Key())] = bytes.Clone(iter.Value())
	}
	require.NoError(t, iter.Close())
	return rows
}

func programsBlockEndMerge(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	f.merge(t, mergePlanInfo(t, map[string]any{
		"leaders":           [][]byte{mergePrimary[:], mergeStandby[:]},
		"receipt_keys":      seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight))},
		"activation_height": uint64(mergeHeight + 2000),
	}))
}

func programsBlockEndPrepareMerge(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	// Apply the real merge handler in the fixture's initial deliver state so
	// the subsequent block loop commits that state and advances every height.
	programsBlockEndMerge(t, f)
	f.ctx = f.ctx.WithBlockHeight(1)
	for height := int64(1); height < mergeHeight; height++ {
		programsBlockEndStep(t, f, height, false)
	}
}

func TestProgramCallsAtConsecutiveHeightsPassThroughTheBlockFlow(t *testing.T) {
	f := newHostedProgramsABIFixture(t, 1)
	programsBlockEndPrepareMerge(t, f)
	for height := mergeHeight; height < mergeHeight+4; height++ {
		programsBlockEndStep(t, f, height, true)
	}
	require.Equal(t, mergeHeight+3, f.app.LastBlockHeight())
}

func TestProgramCallAfterIdleBlocksPasses(t *testing.T) {
	f := newHostedProgramsABIFixture(t, 1)
	programsBlockEndPrepareMerge(t, f)
	programsBlockEndStep(t, f, mergeHeight, true)
	for height := mergeHeight + 1; height <= mergeHeight+20; height++ {
		programsBlockEndStep(t, f, height, false)
	}
	programsBlockEndStep(t, f, mergeHeight+21, true)
	require.Equal(t, mergeHeight+21, f.app.LastBlockHeight())
}

func TestBlockEndTransitionRunsOncePerBlock(t *testing.T) {
	f := newHostedProgramsABIFixture(t, 1)
	key := f.app.GetKey(LayerXHostStoreKey)
	programsBlockEndStep(t, f, 1, false)
	beforeRows := programsBlockEndHostRows(t, f.app.GetCheckCtx(), key)
	var total, largest time.Duration
	var written []string
	for height := int64(2); height < 1002; height++ {
		before, err := lxphost.NextSequence(f.ctx, key)
		require.NoError(t, err)
		elapsed := programsBlockEndStep(t, f, height, false)
		total += elapsed
		if elapsed > largest {
			largest = elapsed
		}
		after, err := lxphost.NextSequence(f.app.GetCheckCtx(), key)
		require.NoError(t, err)
		require.Equal(t, before+1, after, "height %d", height)
		if height == 2 {
			for key, value := range programsBlockEndHostRows(t, f.app.GetCheckCtx(), key) {
				if !bytes.Equal(beforeRows[key], value) {
					written = append(written, hex.EncodeToString([]byte(key)))
				}
			}
		}
	}
	sort.Strings(written)
	require.NotEmpty(t, written)
	t.Logf("1000 inactive blocks: mean=%s largest=%s host keys=%v", total/1000, largest, written)
	_, err := f.app.FinalizeBlock(context.Background(), programsBlockEndRequest(f, 1002))
	require.NoError(t, err)
	_, err = f.app.FinalizeBlock(context.Background(), programsBlockEndRequest(f, 1002))
	require.ErrorContains(t, err, "LXP_ERR_IDEMPOTENT_REPLAY")
}

func TestBlockEndTransitionLeavesAChainWithoutSchedulesUntouched(t *testing.T) {
	valPub := secp256k1.GenPrivKey().PubKey()
	w := NewTestWrapper(t, time.UnixMilli(1700000000000).UTC(), valPub, true)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, w.Ctx)
	key := a.GetKey(LayerXHostStoreKey)
	require.False(t, lxphost.ProgramsSchedulesPresent(w.Ctx, key))
	before := programsBlockEndHostRows(t, w.Ctx, key)
	header := &tmproto.Header{ChainID: w.Ctx.ChainID(), Height: 1, Time: w.Ctx.BlockTime()}
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: header})
	require.NoError(t, err)
	root := a.NewUncachedContext(false, *header)
	require.Equal(t, before, programsBlockEndHostRows(t, root, key))
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, before, programsBlockEndHostRows(t, a.GetCheckCtx(), key))
}

func TestBlockEndTransitionSurvivesRestart(t *testing.T) {
	f := newHostedProgramsABIFixture(t, 1)
	programsBlockEndPrepareMerge(t, f)
	programsBlockEndStep(t, f, mergeHeight, true)
	info, err := f.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(info))
	last, err := lxphost.NextSequence(f.app.GetCheckCtx(), f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	require.NoError(t, f.app.Close())
	f.app = New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, true, config.TestConfig(),
		MakeEncodingConfig(), wasm.EnableAllProposals, TestAppOpts{}, EmptyWasmOpts, nil)
	f.ctx = f.app.NewUncachedContext(false, tmproto.Header{Height: mergeHeight + 1, Time: f.ctx.BlockTime(), ChainID: f.ctx.ChainID()})
	restored, err := lxphost.NextSequence(f.ctx, f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	require.Equal(t, last, restored)
	programsBlockEndStep(t, f, mergeHeight+1, true)
	after, err := lxphost.NextSequence(f.app.GetCheckCtx(), f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	require.Equal(t, last+2, after)
}
