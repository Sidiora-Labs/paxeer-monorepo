package app_test

import (
	"context"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestEmptySequencedBlockKeepsSequence(t *testing.T) {
	w := newSequencerWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	activate(t, w)

	genesis, err := w.App.SequencerKeeper.ExportGenesis(w.Ctx)
	require.NoError(t, err)
	genesis.LastSeq = 4
	genesis.LastSeqHash[0] = 0x77
	require.NoError(t, w.App.SequencerKeeper.InitGenesis(w.Ctx, genesis))
	lastSeq, err := w.App.SequencerKeeper.LastSeq(w.Ctx)
	require.NoError(t, err)
	lastSeqHash, err := w.App.SequencerKeeper.LastSeqHash(w.Ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(4), lastSeq)
	require.Equal(t, genesis.LastSeqHash, lastSeqHash)

	batch := signedBatch(t, lastSeq+1, lastSeq, lastSeqHash[:], lastSeqHash[:], seqtypes.EmptyTxRoot[:], sequencerKeyID)
	results, err := processBlock(w, [][]byte{batch})
	require.NoError(t, err)
	require.True(t, w.App.Sequenced(w.Ctx, w.Ctx.BlockHeight()))
	require.Equal(t, []*abci.ExecTxResult{{}}, results)
	requireSequencerState(t, w, lastSeq, lastSeqHash[:])

	height := w.Ctx.BlockHeight()
	require.Equal(t, height-1, w.App.LastBlockHeight())
	w.App.SetDeliverStateToCommit()
	committed, err := w.App.Commit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, committed)
	require.Equal(t, height, w.App.LastBlockHeight())
	require.Equal(t, height, w.App.LastCommitID().Version)
	w.Ctx = w.App.GetCheckCtx()
	requireSequencerState(t, w, lastSeq, lastSeqHash[:])
}
