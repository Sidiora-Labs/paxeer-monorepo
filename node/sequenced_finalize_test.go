package app_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	protoio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/require"
)

type finalizeSealedHarness struct {
	w         *app.TestWrapper
	source    *sequencer.ConsensusSource
	sealed    *sequencer.SealedLayer
	req       *abci.RequestFinalizeBlock
	sender    utils.TestAcct
	recipient common.Address
	votes     []abci.VoteInfo
}

func newFinalizeSealedHarness(t *testing.T, seal bool) *finalizeSealedHarness {
	t.Helper()
	w := newSequencerWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
	activate(t, w)
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	w.App.StoreConsensusParams(w.Ctx, params)
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	w.App.EvmKeeper.SetAddressMapping(w.Ctx, sender.AccountAddress, sender.EvmAddress)
	w.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", seqCtxFundingUhpx)))
	validators := w.App.StakingKeeper.GetAllValidators(w.Ctx)
	require.Len(t, validators, 1)
	consAddr, err := validators[0].GetConsAddr()
	require.NoError(t, err)
	votes := []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 100}, SignedLastBlock: true}}
	require.NoError(t, w.App.SequencerKeeper.StoreVotes(w.Ctx, 1, votes))
	commitBeforeSequencedProposal(t, w)

	exec, err := sequencer.NewExecutor(w.App, sequencer.Options{ChainID: sequencedChainID, ProposerAddress: consAddr})
	require.NoError(t, err)
	dir := t.TempDir()
	consensusKey := filepath.Join(dir, "consensus.json")
	consensusPV, err := privval.GenFilePV(consensusKey, filepath.Join(dir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, consensusPV.Save())
	receiptKey := filepath.Join(dir, "receipt-key")
	require.NoError(t, os.WriteFile(receiptKey, []byte(hex.EncodeToString(sequencerKeySeed)), 0o600))
	pins, err := w.App.SequencerKeeper.ReceiptKeyPins(w.App.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(receiptKey, consensusKey, pins, 2)
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(w.App, exec, key)
	require.NoError(t, err)
	w.App.SetSequencerSource(source)
	at := w.Ctx.BlockTime().Add(2 * time.Second).Truncate(time.Millisecond)
	require.NoError(t, exec.Open(2, at.UnixMilli()))
	accepted, err := exec.Accept(seqCtxTransfer(t, sender, 0, recipient, big.NewInt(1)))
	require.NoError(t, err)
	require.Equal(t, uint32(0), accepted.Result.Code, accepted.Result.Log)
	h := &finalizeSealedHarness{w: w, source: source, sender: sender, recipient: recipient, votes: votes}
	h.req = &abci.RequestFinalizeBlock{
		Hash:              []byte(t.Name()),
		Header:            &tmproto.Header{ChainID: sequencedChainID, Height: 2, Time: at, ProposerAddress: consAddr},
		DecidedLastCommit: abci.CommitInfo{Votes: votes},
	}
	if seal {
		_, err := source.Seal(context.Background(), 2)
		require.NoError(t, err)
		h.sealed = source.Sealed()
		require.NotNil(t, h.sealed)
		h.req.Txs = h.sealed.Txs
	}
	return h
}

func commitFinalizedSealed(t *testing.T, h *finalizeSealedHarness) sdk.Context {
	t.Helper()
	_, err := h.w.App.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), h.w.App.LastBlockHeight())
	return h.w.App.GetCheckCtx()
}

func TestFinalizeAdoptsSealedBlock(t *testing.T) {
	h := newFinalizeSealedHarness(t, true)
	resp, err := h.w.App.FinalizeBlock(context.Background(), h.req)
	require.NoError(t, err)
	require.Len(t, resp.TxResults, 2)
	require.Same(t, h.sealed.Results[1], resp.TxResults[1])
	require.Equal(t, h.sealed.Events, resp.Events)
	require.Len(t, resp.ValidatorUpdates, len(h.sealed.EndBlock.ValidatorUpdates))
	for i, update := range h.sealed.EndBlock.ValidatorUpdates {
		require.Equal(t, update.PubKey, resp.ValidatorUpdates[i].PubKey)
		require.Equal(t, update.Power, resp.ValidatorUpdates[i].Power)
	}
	require.NotEmpty(t, resp.AppHash)
	ctx := commitFinalizedSealed(t, h)
	require.Equal(t, uint64(1), h.w.App.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress))
	requireSeqCtxWei(t, big.NewInt(1), seqCtxWei(h.w.App, ctx, h.recipient))
	last, err := h.w.App.SequencerKeeper.LastSeq(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), last)
	require.Equal(t, resp.AppHash, h.w.App.LastCommitID().Hash)
}

func TestFinalizeSealedHashMismatchReexecutes(t *testing.T) {
	for _, mode := range []string{"different transaction", "different length", "no source", "not sealed", "different height"} {
		t.Run(mode, func(t *testing.T) {
			h := newFinalizeSealedHarness(t, mode != "not sealed")
			replacement := utils.NewSigner().EvmAddress
			tx := seqCtxTransfer(t, h.sender, 0, replacement, big.NewInt(2))
			typed, err := h.w.App.GetTxConfig().TxDecoder()(tx)
			require.NoError(t, err)
			msg := h.w.App.GetEVMMsg(typed)
			require.NotNil(t, msg)
			etx, _ := msg.AsTransaction()
			require.NotNil(t, etx)
			ids := [][32]byte{[32]byte(etx.Hash())}
			last := seqtypes.SeqHash([32]byte{}, 1, ids[0])
			root := seqtypes.TxRoot(ids)
			h.req.Txs = [][]byte{signedBatch(t, 1, 1, zeroSeqHash, last[:], root[:], sequencerKeyID), tx}
			if mode == "different length" {
				h.req.Txs = [][]byte{signedBatch(t, 1, 0, zeroSeqHash, zeroSeqHash, seqtypes.EmptyTxRoot[:], sequencerKeyID)}
			}
			if mode == "no source" {
				h.req.Txs = h.sealed.Txs
				h.w.App.SetSequencerSource(nil)
			}
			if mode == "different height" {
				h.req.Txs = h.sealed.Txs
				h.sealed.Height++
			}
			resp, err := h.w.App.FinalizeBlock(context.Background(), h.req)
			require.NoError(t, err)
			require.Len(t, resp.TxResults, len(h.req.Txs))
			for _, result := range resp.TxResults {
				require.Zero(t, result.Code, result.Log)
			}
			if h.sealed != nil && len(resp.TxResults) > 1 {
				require.NotSame(t, h.sealed.Results[1], resp.TxResults[1])
			}
			ctx := commitFinalizedSealed(t, h)
			switch mode {
			case "different length":
				require.Zero(t, h.w.App.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress))
				requireSeqCtxWei(t, big.NewInt(0), seqCtxWei(h.w.App, ctx, h.recipient))
			case "no source", "different height":
				require.Equal(t, uint64(1), h.w.App.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress))
				requireSeqCtxWei(t, big.NewInt(1), seqCtxWei(h.w.App, ctx, h.recipient))
			default:
				require.Equal(t, uint64(1), h.w.App.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress))
				requireSeqCtxWei(t, big.NewInt(0), seqCtxWei(h.w.App, ctx, h.recipient))
				requireSeqCtxWei(t, big.NewInt(2), seqCtxWei(h.w.App, ctx, replacement))
			}
		})
	}
}

func TestFinalizeAdoptedBlockStoresVotes(t *testing.T) {
	for _, mode := range []string{"decided votes", "empty votes", "nil votes", "invalid votes"} {
		t.Run(mode, func(t *testing.T) {
			h := newFinalizeSealedHarness(t, true)
			old := []abci.VoteInfo{{Validator: h.votes[0].Validator, SignedLastBlock: false}}
			require.NoError(t, h.w.App.SequencerKeeper.StoreVotes(h.sealed.Context(), 2, old))
			switch mode {
			case "empty votes":
				h.req.DecidedLastCommit.Votes = []abci.VoteInfo{}
			case "nil votes":
				h.req.DecidedLastCommit.Votes = nil
			case "invalid votes":
				h.req.DecidedLastCommit.Votes = []abci.VoteInfo{{Validator: abci.Validator{Address: bytes.Repeat([]byte{1}, 256)}}}
			}
			resp, err := h.w.App.FinalizeBlock(context.Background(), h.req)
			if mode == "invalid votes" {
				require.EqualError(t, err, "stored votes: vote 0 address length 256 exceeds 255")
				require.Nil(t, resp)
				require.Equal(t, int64(1), h.w.App.LastBlockHeight())
				stored, err := h.w.App.SequencerKeeper.StoredVotes(h.sealed.Context(), 2)
				require.NoError(t, err)
				require.Equal(t, old, stored)
				return
			}
			require.NoError(t, err)
			require.Same(t, h.sealed.Results[1], resp.TxResults[1])
			ctx := commitFinalizedSealed(t, h)
			stored, err := h.w.App.SequencerKeeper.StoredVotes(ctx, 2)
			require.NoError(t, err)
			if mode == "decided votes" {
				require.Equal(t, h.votes, stored)
			} else {
				require.Empty(t, stored)
			}
			previous, err := h.w.App.SequencerKeeper.StoredVotes(ctx, 1)
			require.NoError(t, err)
			require.Equal(t, h.votes, previous)
		})
	}
}

func TestFinalizeAdoptedSealedBlockMatchesReexecution(t *testing.T) {
	h := newFinalizeSealedHarness(t, true)
	require.Len(t, h.sealed.Txs, 2)
	batch, err := app.DecodeSequenceBatch(h.sealed.Txs[0])
	require.NoError(t, err)
	require.Equal(t, uint64(1), batch.ToSeq)

	var snapshot bytes.Buffer
	require.NoError(t, h.w.App.CommitMultiStore().Snapshot(1, protoio.NewDelimitedWriter(&snapshot)))
	follower := app.Setup(t, true, false, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetStoreLoader(func(ms sdk.CommitMultiStore) error {
			if err := ms.LoadLatestVersion(); err != nil {
				return err
			}
			_, err := ms.Restore(1, 0, protoio.NewDelimitedReader(bytes.NewReader(snapshot.Bytes()), 64<<20))
			return err
		})
	})
	t.Cleanup(func() { require.NoError(t, follower.Close()) })
	require.Equal(t, h.w.App.LastCommitID(), follower.LastCommitID())
	require.Equal(t, int64(1), follower.LastBlockHeight())

	h.req.Header.AppHash = bytes.Clone(h.w.App.LastCommitID().Hash)
	h.req.Header.DataHash = bytes.Repeat([]byte{0x12}, 32)
	h.req.Header.ValidatorsHash = bytes.Repeat([]byte{0x23}, 32)
	h.req.Header.NextValidatorsHash = bytes.Repeat([]byte{0x34}, 32)
	h.req.Header.ConsensusHash = bytes.Repeat([]byte{0x45}, 32)
	h.req.Header.LastResultsHash = bytes.Repeat([]byte{0x56}, 32)
	provisional, found := h.w.App.StakingKeeper.GetHistoricalInfo(h.sealed.Context(), 2)
	require.True(t, found)
	require.NotEqual(t, *h.req.Header, provisional.Header)

	adopted, err := h.w.App.FinalizeBlock(context.Background(), h.req)
	require.NoError(t, err)
	require.Same(t, h.sealed.Results[1], adopted.TxResults[1])
	reexecuted, err := follower.FinalizeBlock(context.Background(), h.req)
	require.NoError(t, err)
	require.NotSame(t, h.sealed.Results[1], reexecuted.TxResults[1])
	for _, response := range []*abci.ResponseFinalizeBlock{adopted, reexecuted} {
		require.Len(t, response.TxResults, 2)
		for _, result := range response.TxResults {
			require.Zero(t, result.Code, result.Log)
		}
	}
	require.Equal(t, reexecuted.AppHash, adopted.AppHash)
	primaryCtx := commitFinalizedSealed(t, h)
	_, err = follower.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), follower.LastBlockHeight())
	require.Equal(t, adopted.AppHash, h.w.App.LastCommitID().Hash)
	require.Equal(t, reexecuted.AppHash, follower.LastCommitID().Hash)
	require.Equal(t, follower.LastCommitID(), h.w.App.LastCommitID())
	for _, a := range []*app.App{h.w.App, follower} {
		ctx := a.GetCheckCtx()
		require.Equal(t, uint64(1), a.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress))
		requireSeqCtxWei(t, big.NewInt(1), seqCtxWei(a, ctx, h.recipient))
		historical, found := a.StakingKeeper.GetHistoricalInfo(ctx, 2)
		require.True(t, found)
		require.Equal(t, *h.req.Header, historical.Header)
		require.Equal(t, provisional.Valset, historical.Valset)
	}
	require.Equal(t, primaryCtx.BlockHeight(), follower.GetCheckCtx().BlockHeight())
}
