package sequencer_test

import (
	"bytes"
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

type crashReceiptReplay struct {
	*sequencer.AppReplayer
	receipts [][]byte
}

func (r *crashReceiptReplay) Execute(a *wal.Accept) ([]byte, error) {
	receipt, err := r.AppReplayer.Execute(a)
	if err != nil {
		return nil, err
	}
	r.receipts = append(r.receipts, bytes.Clone(receipt))
	return receipt, nil
}

func (r *crashReceiptReplay) Replay(log *wal.WAL) (wal.ReplayResult, error) {
	return log.Replay(r)
}

func TestCrashAfterReceiptReplaysIdenticalReceipt(t *testing.T) {
	for _, mode := range []sequencer.Mode{sequencer.ModeSequential, sequencer.ModeMicroBatch} {
		t.Run(mode.String(), func(t *testing.T) {
			sender, recipient := utils.NewSigner(), utils.NewSigner()
			f := newStartHarness(t, mode, sender)
			committed := f.committed()
			sb := f.restart(t, []wal.Entry{committed})
			require.NoError(t, f.n.Start(t.Context()))
			opened, err := f.n.Executor.Snapshot()
			require.NoError(t, err)
			openTime := opened.BlockTime()
			commitID := f.app.CommitMultiStore().LastCommitID()
			txs, receipts := make([][]byte, 10), make([][]byte, 10)
			lastHash := sourcePrevSeqHash
			for i := range txs {
				signed, tx := transfer(t, sender, uint64(i), recipient.EvmAddress, big.NewInt(int64(i+1)))
				receipt, err := f.n.Accepter.Accept(tx)
				require.NoError(t, err)
				verified, err := verify.ReceiptSignature(receipt, f.n.ReceiptKey.PublicKey())
				require.NoError(t, err)
				require.Equal(t, sourceLastSeq+uint64(i)+1, verified.Receipt.GlobalSequence)
				txs[i], receipts[i] = bytes.Clone(tx), bytes.Clone(receipt)
				lastHash = expectedSeqHash(lastHash, sourceLastSeq+uint64(i)+1, [32]byte(signed.Hash()))
			}
			lastSeq := sourceLastSeq + uint64(len(txs))
			require.Equal(t, lastSeq, sb.LastSeq())
			require.NoError(t, sb.Err())
			require.Nil(t, f.n.ConsensusSource.Sealed())
			entries := driverEntries(t, f.cfg.WALDir)
			require.Len(t, entries, len(txs)+2)
			require.Equal(t, committed, entries[0])
			require.IsType(t, &wal.Open{}, entries[1])
			for i := range txs {
				require.Equal(t, &wal.Accept{
					Height: uint64(openHeight), Seq: sourceLastSeq + uint64(i) + 1,
					Tx: txs[i], Receipt: receipts[i],
				}, entries[i+2])
			}
			require.Equal(t, commitID, f.app.CommitMultiStore().LastCommitID())
			requireStoredSequence(t, f.app, f.app.GetCheckCtx(), sourceLastSeq, sourcePrevSeqHash)
			require.Zero(t, f.app.EvmKeeper.GetNonce(f.app.GetCheckCtx(), sender.EvmAddress))

			dropRuntime := func() {
				f.n.Replicator.Close()
				f.n.WAL.Close()
				f.app.SetOpenBlock(nil)
				f.app.SetSequencerSource(nil)
				f.app.SetSequencerRPC(evmrpc.SequencerOption{}, evmrpc.SequencerReceiptOption{})
				f.app.SetCommitListener(nil)
				f.n = nil
				_, err := f.app.PendingSnapshot()
				require.ErrorIs(t, err, app.ErrPendingServedBySequencer)
				f.n, err = sequencer.NewNode(f.cfg, f.app)
				require.NoError(t, err)
				_, err = f.n.Executor.Snapshot()
				require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
			}
			dropRuntime()
			replayer := &crashReceiptReplay{AppReplayer: sequencer.NewAppReplayer(f.n.Executor, f.n.Receipter)}
			replayed, err := replayer.Replay(f.n.WAL)
			require.NoError(t, err)
			require.Equal(t, committed, replayed.From)
			require.Equal(t, len(receipts), replayed.Accepts)
			require.Equal(t, receipts, replayer.receipts)
			require.Equal(t, entries[len(entries)-1], replayed.LastAccept)
			require.Nil(t, replayed.Tail)
			replayer = nil
			dropRuntime()
			require.NoError(t, f.n.Start(t.Context()))
			require.Equal(t, entries, driverEntries(t, f.cfg.WALDir), "Start must recover the existing open block")
			pending, err := f.app.PendingSnapshot()
			require.NoError(t, err)
			require.Equal(t, openHeight, pending.BlockHeight())
			require.Equal(t, openTime, pending.BlockTime())
			require.Equal(t, uint64(len(txs)), f.app.EvmKeeper.GetNonce(pending, sender.EvmAddress))
			requireWei(t, big.NewInt(55), f.weiBalance(pending, recipient.EvmAddress))
			require.Equal(t, commitID, f.app.CommitMultiStore().LastCommitID())

			block, err := f.app.SequencerSource().Seal(t.Context(), openHeight)
			require.NoError(t, err)
			require.Equal(t, txs, block.Txs.ToSliceOfBytes())
			require.Equal(t, openTime, block.OpenTime)
			sealed := f.n.ConsensusSource.Sealed()
			require.NotNil(t, sealed)
			require.Equal(t, sourceLastSeq+1, sealed.Batch.FromSeq)
			require.Equal(t, lastSeq, sealed.Batch.ToSeq)
			require.Equal(t, lastHash, sealed.Batch.LastSeqHash)
			requireStoredSequence(t, f.app, sealed.Context(), lastSeq, lastHash)
			response, err := f.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{
					ChainID: f.app.ChainID, Height: openHeight, Time: block.OpenTime,
					ProposerAddress: f.pv.Key.PubKey.Address(),
				},
				Txs: append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...),
			})
			require.NoError(t, err)
			require.Len(t, response.TxResults, len(txs)+1)
			for _, result := range response.TxResults {
				require.Zero(t, result.Code, result.Log)
			}
			f.app.SetDeliverStateToCommit()
			_, err = f.app.Commit(t.Context())
			require.NoError(t, err)
			require.Equal(t, openHeight, f.app.LastBlockHeight())
			requireStoredSequence(t, f.app, f.app.GetCheckCtx(), lastSeq, lastHash)
			require.Equal(t, uint64(len(txs)), f.app.EvmKeeper.GetNonce(f.app.GetCheckCtx(), sender.EvmAddress))
			requireWei(t, big.NewInt(55), f.weiBalance(f.app.GetCheckCtx(), recipient.EvmAddress))
			require.Equal(t, &wal.Committed{
				Height: uint64(openHeight), LastSeq: lastSeq,
				AppHash: [32]byte(f.app.CommitMultiStore().LastCommitID().Hash),
			}, f.n.WAL.LastCommitted())
			pending, err = f.app.PendingSnapshot()
			require.NoError(t, err)
			require.Equal(t, openHeight+1, pending.BlockHeight())
			require.NoError(t, sb.Err())
		})
	}
}
