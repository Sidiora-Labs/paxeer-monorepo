package app_test

import (
	"context"
	"crypto/sha256"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestOptimisticProcessingAdoptsSequencedBlock(t *testing.T) {
	at := time.Now().UTC()
	validator := secp256k1.GenPrivKey().PubKey()
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	optimistic := newSequencerWrapper(t, at, validator, false)
	synchronous := newSequencerWrapper(t, at, validator, false)
	for _, w := range []*app.TestWrapper{optimistic, synchronous} {
		w := w
		t.Cleanup(func() {
			if completion := w.App.GetOptimisticProcessingInfo().Completion; completion != nil {
				<-completion
			}
			require.NoError(t, w.App.Close())
		})
		require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
		activate(t, w)
		params := w.App.GetConsensusParams(w.Ctx)
		params.Block.MaxGasWanted = params.Block.MaxGas
		w.App.StoreConsensusParams(w.Ctx, params)
		w.App.EvmKeeper.SetAddressMapping(w.Ctx, sender.AccountAddress, sender.EvmAddress)
		w.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", seqCtxFundingUhpx)))
	}
	synchronous.App.EpochKeeper.SetEpoch(synchronous.Ctx, optimistic.App.EpochKeeper.GetEpoch(optimistic.Ctx))
	for _, w := range []*app.TestWrapper{optimistic, synchronous} {
		commitBeforeSequencedProposal(t, w)
		require.Equal(t, int64(1), w.App.LastBlockHeight())
	}
	require.Equal(t, optimistic.App.LastCommitID(), synchronous.App.LastCommitID())

	txs := [][]byte{nil,
		seqCtxTransfer(t, sender, 0, recipient.EvmAddress, big.NewInt(1)),
		seqCtxTransfer(t, sender, 1, recipient.EvmAddress, big.NewInt(2)),
	}
	ids := make([][32]byte, len(txs)-1)
	last := [32]byte{}
	for i, tx := range txs[1:] {
		decoded, err := optimistic.App.GetTxConfig().TxDecoder()(tx)
		require.NoError(t, err)
		msg := optimistic.App.GetEVMMsg(decoded)
		require.NotNil(t, msg)
		etx, _ := msg.AsTransaction()
		require.NotNil(t, etx)
		ids[i] = [32]byte(etx.Hash())
		last = seqtypes.SeqHash(last, uint64(i+1), ids[i])
	}
	root := seqtypes.TxRoot(ids)
	txs[0] = signedBatch(t, 1, 2, zeroSeqHash, last[:], root[:], sequencerKeyID)
	hash := sha256.Sum256([]byte(t.Name()))
	header := &tmproto.Header{
		ChainID: sequencedChainID, Height: 2, Time: at.Add(2 * time.Second), ProposerAddress: crypto.AddressHash(sequencerLeader),
	}
	proposal := &abci.RequestProcessProposal{Hash: hash[:], Header: header, Txs: txs}
	finalize := &abci.RequestFinalizeBlock{Hash: hash[:], Header: header, Txs: txs}

	var responses []*abci.ResponseFinalizeBlock
	for _, w := range []*app.TestWrapper{optimistic, synchronous} {
		require.True(t, w.App.Sequenced(w.App.GetCheckCtx(), header.Height))
		accepted, err := w.App.ProcessProposal(context.Background(), proposal)
		require.NoError(t, err)
		require.NotNil(t, accepted)
		require.Equal(t, abci.ResponseProcessProposal_ACCEPT, accepted.Status)
		info := w.App.GetOptimisticProcessingInfo()
		require.NotNil(t, info.Completion)
		select {
		case <-info.Completion:
		case <-time.After(30 * time.Second):
			t.Fatal("optimistic processing did not finish")
		}
		info = w.App.GetOptimisticProcessingInfo()
		require.False(t, info.Aborted)
		require.Equal(t, header.Height, info.Height)
		require.Equal(t, hash[:], info.Hash)
		require.Len(t, info.TxRes, len(txs))
		require.Equal(t, abci.ExecTxResult{}, *info.TxRes[0])
		for _, result := range info.TxRes[1:] {
			require.Zero(t, result.Code, result.Log)
			require.Equal(t, int64(seqCtxTransferGas), result.GasUsed)
			require.NotNil(t, result.EvmTxInfo)
		}
		clean := w.App.GetProcessProposalCleanContext()
		seq, err := w.App.SequencerKeeper.LastSeq(clean)
		require.NoError(t, err)
		require.Zero(t, seq)
		require.Zero(t, w.App.EvmKeeper.GetNonce(clean, sender.EvmAddress))
		requireSeqCtxWei(t, big.NewInt(0), seqCtxWei(w.App, clean, recipient.EvmAddress))

		if w == synchronous {
			w.App.ClearOptimisticProcessingInfo()
			require.Nil(t, w.App.GetOptimisticProcessingInfo().Completion)
		}
		response, err := w.App.FinalizeBlock(context.Background(), finalize)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.TxResults, len(txs))
		for i, result := range response.TxResults {
			if w == optimistic {
				require.Same(t, info.TxRes[i], result)
			} else {
				require.NotSame(t, info.TxRes[i], result)
			}
		}
		require.Equal(t, info.Events, response.Events)
		expectedUpdates := make([]abci.ValidatorUpdate, len(info.EndBlockResp.ValidatorUpdates))
		for i, update := range info.EndBlockResp.ValidatorUpdates {
			expectedUpdates[i] = abci.ValidatorUpdate{PubKey: update.PubKey, Power: update.Power}
		}
		require.Equal(t, expectedUpdates, response.ValidatorUpdates)
		require.NotEmpty(t, response.AppHash)
		require.Nil(t, w.App.GetOptimisticProcessingInfo().Completion)
		_, err = w.App.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, header.Height, w.App.LastBlockHeight())
		require.Equal(t, response.AppHash, w.App.LastCommitID().Hash)
		w.Ctx = w.App.GetCheckCtx()
		requireSequencerState(t, w, 2, last[:])
		require.Equal(t, uint64(2), w.App.EvmKeeper.GetNonce(w.Ctx, sender.EvmAddress))
		requireSeqCtxWei(t, big.NewInt(3), seqCtxWei(w.App, w.Ctx, recipient.EvmAddress))
		responses = append(responses, response)
	}
	require.Equal(t, responses[1], responses[0])
	require.Equal(t, synchronous.App.LastCommitID(), optimistic.App.LastCommitID())
}
