package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestHaltedBlockLeavesCommittedStateAtPreviousHeight(t *testing.T) {
	home, directory := t.TempDir(), t.TempDir()
	open := func() (*app.App, func()) {
		database, err := dbm.NewGoLevelDB("application", directory)
		require.NoError(t, err)
		a := app.New(database, nil, true, map[int64]bool{}, home, 1, false, nil,
			app.MakeEncodingConfig(), wasm.EnableAllProposals, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
		closed := false
		closeApp := func() {
			if !closed {
				require.NoError(t, a.Close())
				closed = true
			}
		}
		t.Cleanup(closeApp)
		return a, closeApp
	}

	a, closeApp := open()
	const previousHeight int64 = 1
	const haltHeight int64 = previousHeight + 1
	start := time.Unix(1_800_000_000, 0).UTC()
	genesis := app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler)
	state, err := json.Marshal(genesis)
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: start, ChainId: sequencedChainID,
		ConsensusParams: app.DefaultConsensusParams, AppStateBytes: state,
	})
	require.NoError(t, err)

	commitCalls := 0
	a.SetCommitListener(func(height int64, hash []byte) { commitCalls++ })
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: sequencedChainID, Height: previousHeight, Time: start},
	})
	require.NoError(t, err)
	ctx := a.GetContextForDeliverTx(nil)
	activationState, err := json.Marshal(map[string]json.RawMessage{seqtypes.ModuleName: activationGenesis(t)})
	require.NoError(t, err)
	a.InitChainer(ctx, abci.RequestInitChain{AppStateBytes: activationState})
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, haltHeight))
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, commitCalls, "the real Commit must notify the listener")
	require.Equal(t, previousHeight, a.LastBlockHeight())
	committedHash := bytes.Clone(a.LastCommitID().Hash)
	require.NotEmpty(t, committedHash)
	require.True(t, a.Sequenced(a.GetCheckCtx(), haltHeight))

	unpinnedSeed := bytes.Repeat([]byte{0x99}, ed25519.SeedSize)
	unpinnedPublicKey := ed25519.NewKeyFromSeed(unpinnedSeed).Public().(ed25519.PublicKey)
	keys, err := a.SequencerKeeper.ReceiptKeys(a.GetCheckCtx())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, sequencerKeyID, keys[0].KeyId[:])
	require.Equal(t, sequencerPublicKey(), keys[0].PublicKey[:])
	require.NotEqual(t, []byte(unpinnedPublicKey), keys[0].PublicKey[:])
	batch := signedBatchAt(t, uint64(haltHeight), unpinnedSeed, 1, 0,
		zeroSeqHash, zeroSeqHash, seqtypes.EmptyTxRoot[:], sequencerKeyID)
	decoded, err := app.DecodeSequenceBatch(batch)
	require.NoError(t, err)
	require.NoError(t, seqtypes.VerifyBatchTxs(decoded, nil))
	require.NoError(t, decoded.VerifySignature(sequencedChainID, [32]byte(unpinnedPublicKey)))
	require.ErrorIs(t, decoded.VerifySignature(sequencedChainID, keys[0].PublicKey), seqtypes.ErrBadSignature)

	request := &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: sequencedChainID, Height: haltHeight,
			Time: start.Add(2 * time.Second), ProposerAddress: sequencerLeader},
		Txs: [][]byte{batch},
	}
	response, err := a.FinalizeBlock(context.Background(), request)
	require.ErrorIs(t, err, app.ErrSequencedBlockHalt)
	require.ErrorIs(t, err, seqtypes.ErrBadSignature)
	require.Contains(t, err.Error(), "height 2")
	require.Nil(t, response)
	require.Equal(t, 1, commitCalls, "FinalizeBlocker must return the halt without calling Commit")
	require.Equal(t, previousHeight, a.LastBlockHeight())
	require.Equal(t, committedHash, a.LastCommitID().Hash)
	closeApp()
	require.Equal(t, 1, commitCalls, "closing the halted node must not commit the failed block")

	restarted, _ := open()
	require.NotSame(t, a, restarted)
	info, err := restarted.Info(context.Background(), &abci.RequestInfo{})
	require.NoError(t, err)
	require.Equal(t, previousHeight, info.LastBlockHeight)
	require.Equal(t, committedHash, info.LastBlockAppHash)
	require.Equal(t, previousHeight, restarted.LastBlockHeight())
	require.True(t, restarted.Sequenced(restarted.GetCheckCtx(), haltHeight))
	lastSeq, err := restarted.SequencerKeeper.LastSeq(restarted.GetCheckCtx())
	require.NoError(t, err)
	require.Zero(t, lastSeq)
	lastHash, err := restarted.SequencerKeeper.LastSeqHash(restarted.GetCheckCtx())
	require.NoError(t, err)
	require.Equal(t, zeroSeqHash, lastHash[:])
	restartedKeys, err := restarted.SequencerKeeper.ReceiptKeys(restarted.GetCheckCtx())
	require.NoError(t, err)
	require.Equal(t, keys, restartedKeys)

	request.Txs[0] = signedBatchAt(t, uint64(haltHeight), sequencerKeySeed, 1, 0,
		zeroSeqHash, zeroSeqHash, seqtypes.EmptyTxRoot[:], sequencerKeyID)
	response, err = restarted.FinalizeBlock(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, previousHeight, restarted.LastBlockHeight())
	_, err = restarted.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, haltHeight, restarted.LastBlockHeight())
	require.Equal(t, response.AppHash, restarted.LastCommitID().Hash)
}
