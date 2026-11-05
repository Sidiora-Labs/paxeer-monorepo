package app_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func newSequencedProposalWrapper(t *testing.T) *app.TestWrapper {
	t.Helper()
	w := newSequencerWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, w.Ctx.BlockHeight()+1))
	activate(t, w)
	return w
}

func commitBeforeSequencedProposal(t *testing.T, w *app.TestWrapper) {
	t.Helper()
	_, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: sequencedChainID, Height: w.Ctx.BlockHeight(), Time: w.Ctx.BlockTime()},
	})
	require.NoError(t, err)
	w.App.SetDeliverStateToCommit()
	_, err = w.App.Commit(context.Background())
	require.NoError(t, err)
}

func processSequencedProposal(t *testing.T, w *app.TestWrapper, proposer []byte, txs ...[]byte) *abci.ResponseProcessProposal {
	t.Helper()
	resp, err := w.App.ProcessProposal(context.Background(), &abci.RequestProcessProposal{
		Header: &tmproto.Header{
			ChainID: sequencedChainID, Height: w.App.LastBlockHeight() + 1,
			Time: w.Ctx.BlockTime().Add(2 * time.Second), ProposerAddress: proposer,
		},
		Hash: []byte(t.Name()), Txs: txs,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	if completion := w.App.GetOptimisticProcessingInfo().Completion; completion != nil {
		<-completion
	}
	return resp
}

func TestProcessProposalRejectsBrokenSeqChain(t *testing.T) {
	w := newSequencedProposalWrapper(t)
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = 2 * 21000
	w.App.StoreConsensusParams(w.Ctx, params)
	tx1, hash1 := evmTransfer(t, w, utils.NewSigner(), utils.NewSigner().EvmAddress)
	tx2, hash2 := evmTransfer(t, w, utils.NewSigner(), utils.NewSigner().EvmAddress)
	commitBeforeSequencedProposal(t, w)
	ids := [][32]byte{[32]byte(hash1), [32]byte(hash2)}
	root := seqtypes.TxRoot(ids)
	last := seqtypes.SeqHash(seqtypes.SeqHash([32]byte{}, 1, ids[0]), 2, ids[1])
	brokenLast := bytes.Clone(last[:])
	brokenLast[0] ^= 1
	broken := signedBatchAt(t, 2, sequencerKeySeed, 1, 2, zeroSeqHash, brokenLast, root[:], sequencerKeyID)
	batch, err := app.DecodeSequenceBatch(broken)
	require.NoError(t, err)
	require.Equal(t, root[:], batch.TxRoot)
	require.NoError(t, batch.VerifySignature(sequencedChainID, [32]byte(sequencerPublicKey())))

	resp := processSequencedProposal(t, w, crypto.AddressHash(sequencerLeader), broken, tx1, tx2)
	require.Equal(t, abci.ResponseProcessProposal_REJECT, resp.Status)
	require.Nil(t, w.App.GetOptimisticProcessingInfo().Completion)
	requireSequencerState(t, w, 0, zeroSeqHash)

	gapLast := seqtypes.SeqHash(seqtypes.SeqHash([32]byte{}, 2, ids[0]), 3, ids[1])
	gap := signedBatchAt(t, 2, sequencerKeySeed, 2, 3, zeroSeqHash, gapLast[:], root[:], sequencerKeyID)
	gapBatch, err := app.DecodeSequenceBatch(gap)
	require.NoError(t, err)
	require.NoError(t, seqtypes.VerifyBatchTxs(gapBatch, ids))
	resp = processSequencedProposal(t, w, crypto.AddressHash(sequencerLeader), gap, tx1, tx2)
	require.Equal(t, abci.ResponseProcessProposal_REJECT, resp.Status)
	require.Nil(t, w.App.GetOptimisticProcessingInfo().Completion)

	valid := signedBatchAt(t, 2, sequencerKeySeed, 1, 2, zeroSeqHash, last[:], root[:], sequencerKeyID)
	resp = processSequencedProposal(t, w, crypto.AddressHash(sequencerLeader), valid, tx1, tx2)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, resp.Status)
	stored, err := w.App.SequencerKeeper.LastSeq(w.App.GetProcessProposalCleanContext())
	require.NoError(t, err)
	require.Zero(t, stored)
}

func TestProcessProposalRejectsProposerOutsideLeaders(t *testing.T) {
	w := newSequencedProposalWrapper(t)
	commitBeforeSequencedProposal(t, w)
	batch := signedBatchAt(t, 2, sequencerKeySeed, 1, 0, zeroSeqHash, zeroSeqHash, seqtypes.EmptyTxRoot[:], sequencerKeyID)
	outsider := bytes.Clone(sequencerLeader)
	outsider[0] ^= 1

	for _, proposer := range [][]byte{outsider, nil, sequencerLeader[:20], sequencerLeader, crypto.AddressHash(outsider)} {
		resp := processSequencedProposal(t, w, proposer, batch)
		require.Equal(t, abci.ResponseProcessProposal_REJECT, resp.Status)
		require.Nil(t, w.App.GetOptimisticProcessingInfo().Completion)
	}
	requireSequencerState(t, w, 0, zeroSeqHash)
	resp := processSequencedProposal(t, w, crypto.AddressHash(sequencerLeader), batch)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, resp.Status)
}

func TestFinalizeBlockHaltsOnBadBatchSignature(t *testing.T) {
	w := newSequencedProposalWrapper(t)
	batch := signedBatchAt(t, 2, sequencerKeySeed, 1, 0, zeroSeqHash, zeroSeqHash, seqtypes.EmptyTxRoot[:], sequencerKeyID)
	batch[len(batch)-1] ^= 1
	decoded, err := app.DecodeSequenceBatch(batch)
	require.NoError(t, err)
	require.NoError(t, seqtypes.VerifyBatchTxs(decoded, nil))

	results, err := processBlock(w, [][]byte{batch})
	require.ErrorIs(t, err, app.ErrSequencedBlockHalt)
	require.ErrorIs(t, err, seqtypes.ErrBadSignature)
	require.Contains(t, err.Error(), "height 2")
	require.Nil(t, results)
	requireSequencerState(t, w, 0, zeroSeqHash)
}

func TestFinalizeBlockHaltsOnBatchTxMismatch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		toSeq    uint64
		flipLast bool
		flipRoot bool
		reason   string
	}{
		{name: "lastSeqHash", toSeq: 2, flipLast: true, reason: "lastSeqHash differs from activity id chain"},
		{name: "sequence count", toSeq: 3, reason: "activity id count is 2, want 3"},
		{name: "txRoot", toSeq: 2, flipRoot: true, reason: "txRoot differs from activity id root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newSequencedProposalWrapper(t)
			tx1, hash1 := evmTransfer(t, w, utils.NewSigner(), utils.NewSigner().EvmAddress)
			tx2, hash2 := evmTransfer(t, w, utils.NewSigner(), utils.NewSigner().EvmAddress)
			ids := [][32]byte{[32]byte(hash1), [32]byte(hash2)}
			root := seqtypes.TxRoot(ids)
			last := seqtypes.SeqHash(seqtypes.SeqHash([32]byte{}, 1, ids[0]), 2, ids[1])
			if tc.flipLast {
				last[0] ^= 1
			}
			if tc.flipRoot {
				root[0] ^= 1
			}
			batch := signedBatchAt(t, 2, sequencerKeySeed, 1, tc.toSeq, zeroSeqHash, last[:], root[:], sequencerKeyID)
			decoded, err := app.DecodeSequenceBatch(batch)
			require.NoError(t, err)
			require.NoError(t, decoded.VerifySignature(sequencedChainID, [32]byte(sequencerPublicKey())))

			results, err := processBlock(w, [][]byte{batch, tx1, tx2})
			require.ErrorIs(t, err, app.ErrSequencedBlockHalt)
			require.ErrorIs(t, err, seqtypes.ErrInvalidSequenceBatch)
			require.Contains(t, err.Error(), "height 2")
			require.Contains(t, err.Error(), tc.reason)
			require.Nil(t, results)
			requireSequencerState(t, w, 0, zeroSeqHash)
		})
	}
}

func TestSequencedReadsKeeperActivationHeight(t *testing.T) {
	w := newSequencerWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	mergeHeight := w.Ctx.BlockHeight()
	w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	require.False(t, w.App.Sequenced(w.Ctx, mergeHeight))
	require.False(t, w.App.Sequenced(w.Ctx, mergeHeight+1))
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, mergeHeight+1))
	activate(t, w)
	for _, height := range []int64{mergeHeight + 1, mergeHeight + 2, mergeHeight + 100} {
		require.True(t, w.App.Sequenced(w.Ctx, height))
	}
	require.False(t, w.App.Sequenced(w.Ctx, mergeHeight))
	w.App.UpgradeKeeper.SetDone(w.Ctx.WithBlockHeight(mergeHeight+100), app.MergeUpgrade)
	require.True(t, w.App.Sequenced(w.Ctx, mergeHeight+1))
}
