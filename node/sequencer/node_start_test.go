package sequencer_test

import (
	"context"
	"crypto/ed25519"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type startHarness struct {
	*sourceHarness
	n        *sequencer.Node
	cfg      sequencer.Config
	pv       *privval.FilePV
	listener net.Listener
}

func newStartHarness(t *testing.T, mode sequencer.Mode, sender utils.TestAcct) *startHarness {
	t.Helper()
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}, sender)
	info, err := h.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	l := listen(t)
	t.Cleanup(func() { _ = l.Close() })
	cfg := sequencer.DefaultConfig()
	cfg.Enable, cfg.Mode = true, mode
	cfg.WALDir = filepath.Join(t.TempDir(), "wal")
	cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
	cfg.StandbyAddress = l.Addr().String()
	cfg.ReplicationTimeout = replicationTimeout
	cfg.LeaseDuration = 3 * replicationTimeout
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 4, 8
	cfg.MicroBatchWindow, cfg.MicroBatchMaxCount = time.Millisecond, 2
	n, err := sequencer.NewNode(cfg, h.app)
	require.NoError(t, err)
	f := &startHarness{sourceHarness: h, n: n, cfg: cfg, pv: pv, listener: l}
	t.Cleanup(func() { f.n.Replicator.Close(); f.n.WAL.Close() })
	return f
}

func (f *startHarness) restart(t *testing.T, entries []wal.Entry) *sequencer.Standby {
	t.Helper()
	for _, entry := range entries {
		_, err := f.n.WAL.Append(entry)
		require.NoError(t, err)
	}
	if len(entries) > 0 {
		_, err := f.n.WAL.GroupSync()
		require.NoError(t, err)
	}
	f.n.Replicator.Close()
	f.n.WAL.Close()
	var err error
	f.n, err = sequencer.NewNode(f.cfg, f.app)
	require.NoError(t, err)
	w := reopenReplayWAL(t, entries...)
	exec, err := sequencer.NewExecutor(f.app, sequencer.Options{
		Mode: f.cfg.Mode, ChainID: f.app.ChainID, ProposerAddress: f.pv.Key.PubKey.Address(),
		LastSeq: sourceLastSeq, LastSeqHash: sourcePrevSeqHash,
		MicroBatch: sequencer.MicroBatchOptions{Window: f.cfg.MicroBatchWindow, MaxCount: f.cfg.MicroBatchMaxCount},
	})
	require.NoError(t, err)
	floor, err := sequencer.NewFeeFloor(exec, f.app, "", f.app.ParamsKeeper, f.app.OracleKeeper, f.app.PricefeedKeeper, &f.app.EvmKeeper)
	require.NoError(t, err)
	r, err := sequencer.NewReceipter(floor, f.app, &f.app.EvmKeeper, f.n.ReceiptKey, app.EVMReceiptModuleID)
	require.NoError(t, err)
	keys := t.TempDir()
	sv, err := privval.GenFilePV(filepath.Join(keys, "standby.json"), filepath.Join(keys, "standby-state.json"), "")
	require.NoError(t, err)
	_, receipt, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	primaryReceipt := f.n.ReceiptKey.PublicKey()
	sb, err := sequencer.NewStandby(t.Context(), sequencer.StandbyConfig{
		WAL: w, Executor: sequencer.NewAppReplayer(floor, r), ConsensusKey: sv, ReceiptKey: receipt,
		PrimaryConsensusKey: f.pv.Key.PubKey, PrimaryReceiptKey: primaryReceipt[:],
		CommittedHeight: uint64(f.app.LastBlockHeight()), LastSeq: sourceLastSeq,
	})
	require.NoError(t, err)
	go func() { _ = sb.Serve(f.listener) }()
	return sb
}

func (f *startHarness) committed() *wal.Committed {
	return &wal.Committed{
		Height: uint64(f.app.LastBlockHeight()), LastSeq: sourceLastSeq,
		AppHash: [32]byte(f.app.CommitMultiStore().LastCommitID().Hash),
	}
}

func (f *startHarness) loggedTransfers(t *testing.T, sender utils.TestAcct, count int) ([]wal.Entry, [][]byte) {
	t.Helper()
	openTime := time.Now().Add(time.Second).UnixMilli()
	require.NoError(t, f.n.Executor.Open(openHeight, openTime))
	fee, err := f.n.FeeFloor.BaseFeePerGas()
	require.NoError(t, err)
	entries := []wal.Entry{f.committed(), &wal.Open{
		Height: uint64(openHeight), OpenTimeMs: uint64(openTime), BaseFee: uint256.MustFromBig(fee.TruncateInt().BigInt()),
	}}
	var txs [][]byte
	for i := 0; i < count; i++ {
		_, tx := transfer(t, sender, uint64(i), sender.EvmAddress, big.NewInt(1))
		acc, receipt, err := f.n.Receipter.Accept(tx)
		require.NoError(t, err)
		require.Zero(t, acc.Result.Code, acc.Result.Log)
		entries = append(entries, &wal.Accept{Height: uint64(acc.Height), Seq: acc.Seq, Tx: tx, Receipt: receipt})
		txs = append(txs, tx)
	}
	return entries, txs
}

func TestStartReplaysWALBeforeAccepting(t *testing.T) {
	for _, mode := range []sequencer.Mode{sequencer.ModeSequential, sequencer.ModeMicroBatch} {
		t.Run(mode.String(), func(t *testing.T) {
			sender := utils.NewSigner()
			f := newStartHarness(t, mode, sender)
			entries, txs := f.loggedTransfers(t, sender, 3)
			sb := f.restart(t, entries)
			_, err := f.n.Accepter.Accept(txs[0])
			require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
			require.NoError(t, f.n.Start(t.Context()))
			pending, err := f.app.PendingSnapshot()
			require.NoError(t, err)
			require.Equal(t, openHeight, pending.BlockHeight())
			require.Equal(t, int64(entries[1].(*wal.Open).OpenTimeMs), pending.BlockTime().UnixMilli())
			require.Equal(t, uint64(3), f.app.EvmKeeper.GetNonce(pending, sender.EvmAddress))
			require.Equal(t, uint64(0), f.app.EvmKeeper.GetNonce(f.app.GetCheckCtx(), sender.EvmAddress))
			_, next := transfer(t, sender, 3, sender.EvmAddress, big.NewInt(1))
			receipt, err := f.n.Accepter.Accept(next)
			require.NoError(t, err)
			verified, err := verify.ReceiptSignature(receipt, f.n.ReceiptKey.PublicKey())
			require.NoError(t, err)
			require.Equal(t, sourceLastSeq+4, verified.Receipt.GlobalSequence)
			require.Equal(t, sourceLastSeq+4, sb.LastSeq())
			batch, err := f.n.Executor.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, append(txs, next), batch.Txs)
			require.Equal(t, sourceLastSeq+1, batch.FromSeq)
			require.Equal(t, sourceLastSeq+4, batch.LastSeq)
		})
	}
	t.Run("tampered receipt refuses installation and admission", func(t *testing.T) {
		sender := utils.NewSigner()
		f := newStartHarness(t, sequencer.ModeSequential, sender)
		entries, txs := f.loggedTransfers(t, sender, 1)
		a := entries[2].(*wal.Accept)
		a.Receipt[len(a.Receipt)-1] ^= 1
		for _, entry := range entries {
			_, err := f.n.WAL.Append(entry)
			require.NoError(t, err)
		}
		_, err := f.n.WAL.GroupSync()
		require.NoError(t, err)
		f.n.WAL.Close()
		f.n.Replicator.Close()
		f.n, err = sequencer.NewNode(f.cfg, f.app)
		require.NoError(t, err)
		require.ErrorIs(t, f.n.Start(t.Context()), wal.ErrReceiptMismatch)
		require.Nil(t, f.app.SequencerSource())
		_, err = f.app.PendingSnapshot()
		require.ErrorIs(t, err, app.ErrPendingServedBySequencer)
		_, err = f.n.Accepter.Accept(txs[0])
		require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
	})
}

func TestStartInstallsOpenBlockAndSource(t *testing.T) {
	for _, committedTail := range []bool{false, true} {
		name := "empty WAL"
		if committedTail {
			name = "committed WAL"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			f := newStartHarness(t, sequencer.ModeSequential, sender)
			var entries []wal.Entry
			if committedTail {
				entries = append(entries, f.committed())
			}
			f.restart(t, entries)
			require.NoError(t, f.n.Start(t.Context()))
			require.Same(t, f.n.ConsensusSource, f.app.SequencerSource())
			pending, err := f.app.PendingSnapshot()
			require.NoError(t, err)
			require.Equal(t, f.app.LastBlockHeight()+1, pending.BlockHeight())
			require.Equal(t, pending.BlockHeight(), f.app.LatestCtx().BlockHeight())
			logged := driverEntries(t, f.cfg.WALDir)
			require.Len(t, logged, len(entries)+1)
			require.Equal(t, uint64(openHeight), logged[len(entries)].(*wal.Open).Height)
			runtime := reflect.ValueOf(f.app).Elem()
			rpc := runtime.FieldByName("sequencerRPC")
			require.Equal(t, reflect.ValueOf(f.n.Accepter).Pointer(), rpc.FieldByName("accepter").Elem().Pointer())
			require.Equal(t, reflect.ValueOf(f.n.ReceiptLogFeed).Pointer(), rpc.FieldByName("receiptLogs").Pointer())
			require.False(t, rpc.FieldByName("pendingTxs").IsNil())
			rcpt := runtime.FieldByName("sequencerReceipt")
			require.Equal(t, reflect.ValueOf(f.n.ReceiptSource).Pointer(), rcpt.FieldByName("Source").Elem().Pointer())
			require.Equal(t, reflect.ValueOf(&f.app.SequencerKeeper).Pointer(), rcpt.FieldByName("Keeper").Pointer())
			require.False(t, runtime.FieldByName("commitListener").IsNil())
			_, tx := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
			receipt, err := f.n.Accepter.Accept(tx)
			require.NoError(t, err)
			decoded, err := codec.DecodeReceipt(receipt)
			require.NoError(t, err)
			require.Equal(t, sourceLastSeq+1, decoded.GlobalSequence)
			block, err := f.app.SequencerSource().Seal(t.Context(), openHeight)
			require.NoError(t, err)
			response, err := f.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{ChainID: f.app.ChainID, Height: openHeight, Time: block.OpenTime, ProposerAddress: f.pv.Key.PubKey.Address()},
				Txs:    append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...),
			})
			require.NoError(t, err)
			for _, result := range response.TxResults {
				require.Zero(t, result.Code, result.Log)
			}
			f.app.SetDeliverStateToCommit()
			_, err = f.app.Commit(t.Context())
			require.NoError(t, err)
			pending, err = f.app.PendingSnapshot()
			require.NoError(t, err, "commit listener must reopen through Driver.OnCommit")
			require.Equal(t, openHeight+1, pending.BlockHeight())
			require.Equal(t, uint64(openHeight), f.n.WAL.LastCommitted().Height)
			require.Equal(t, sourceLastSeq+1, f.n.WAL.LastCommitted().LastSeq)
			require.Equal(t, f.app.CommitMultiStore().LastCommitID().Hash, f.n.WAL.LastCommitted().AppHash[:])
			require.Error(t, f.n.Start(t.Context()), "starting twice must not replay over a live runtime")
		})
	}
	t.Run("key is loaded again at startup", func(t *testing.T) {
		f := newStartHarness(t, sequencer.ModeSequential, utils.NewSigner())
		require.NoError(t, os.Remove(f.cfg.ReceiptKeyFile))
		require.ErrorIs(t, f.n.Start(t.Context()), sequencer.ErrReceiptKeyFile)
		require.Nil(t, f.app.SequencerSource())
		require.Empty(t, driverEntries(t, f.cfg.WALDir))
	})
	t.Run("cancelled startup does not install runtime", func(t *testing.T) {
		f := newStartHarness(t, sequencer.ModeSequential, utils.NewSigner())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, f.n.Start(ctx), context.Canceled)
		require.Nil(t, f.app.SequencerSource())
		require.Empty(t, driverEntries(t, f.cfg.WALDir))
	})
}
