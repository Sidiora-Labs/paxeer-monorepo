package sequencer

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type failoverRehearsalNode struct {
	source    *ConsensusSource
	opts      Options
	consensus *privval.FilePV
	log       *wal.WAL
}

func newFailoverRehearsalNode(t *testing.T, sender utils.TestAcct) *failoverRehearsalNode {
	t.Helper()
	dir := t.TempDir()
	consensusFile := filepath.Join(dir, "consensus-key.json")
	pv, err := privval.GenFilePV(consensusFile, filepath.Join(dir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	seed := sha256.Sum256([]byte(dir))
	private := ed25519.NewKeyFromSeed(seed[:])
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	keyID := sha256.Sum256(public[:])
	keyFile := filepath.Join(dir, "receipt-key")
	require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seed[:])), 0o600))
	key, err := LoadReceiptKey(keyFile, consensusFile, []verify.SequencerAuthorization{{
		SequencerID: keyID, PublicKey: public, FirstBatchNumber: 1, LastBatchNumber: 100,
	}}, 2)
	require.NoError(t, err)
	valPub := secp256k1.GenPrivKey().PubKey()
	tw := app.NewTestWrapper(t, time.Now().UTC(), valPub, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := tw.App
	a.EvmKeeper.SetAddressMapping(tw.Ctx, sender.AccountAddress, sender.EvmAddress)
	tw.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1_000_000_000_000))))
	previousHash := sha256.Sum256([]byte("failover rehearsal committed sequence"))
	require.NoError(t, a.SequencerKeeper.InitGenesis(tw.Ctx, seqtypes.GenesisState{
		Leaders:     [][32]byte{keyID},
		ReceiptKeys: []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: 100}},
		LastSeq:     42, LastSeqHash: previousHash,
	}))
	a.UpgradeKeeper.SetDone(tw.Ctx.WithBlockHeight(1), app.MergeUpgrade)
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	opts := Options{ChainID: "pax-test", ProposerAddress: valPub.Address().Bytes(), LastSeq: 42, LastSeqHash: previousHash}
	exec, err := NewExecutor(a, opts)
	require.NoError(t, err)
	source, err := NewConsensusSource(a, exec, key)
	require.NoError(t, err)
	log, err := wal.OpenWAL(filepath.Join(dir, "wal"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(log.Close)
	return &failoverRehearsalNode{source: source, opts: opts, consensus: pv, log: log}
}

func (n *failoverRehearsalNode) Failover(t *testing.T, standby *Standby, primaryKey *ReceiptKey, openMs int64) (*ConsensusSource, FailoverResult) {
	t.Helper()
	result, err := Failover(standby, n.source.key, 2, uint64(openMs)+30_000)
	require.NoError(t, err)
	require.Equal(t, uint64(2), result.Lease.Epoch)
	require.Equal(t, int64(2), result.OpenHeight)
	storedSeq, err := n.source.app.SequencerKeeper.LastSeq(n.source.app.GetCheckCtx())
	require.NoError(t, err)
	require.Equal(t, n.opts.LastSeq, storedSeq)
	promoted, err := NewPromotedConsensusSource(n.source.app, n.source.app, n.opts, storedSeq, n.source.key)
	require.NoError(t, err)
	replayer, err := NewReceipter(promoted.exec, promoted.app, &promoted.app.EvmKeeper, primaryKey, 12)
	require.NoError(t, err)
	replayed, err := n.log.Replay(NewAppReplayer(promoted.exec, replayer))
	require.NoError(t, err)
	require.Equal(t, 100, replayed.Accepts)
	require.Equal(t, result.Lease, replayed.Lease)
	require.NotNil(t, replayed.LastAccept)
	require.Equal(t, result.LastSeq, replayed.LastAccept.Seq)
	require.NoError(t, standby.Err())
	return promoted, result
}

func TestFailoverRehearsalNoReceiptLost(t *testing.T) {
	const beforeFailure = 100
	const totalTransfers = 120
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	params := app.DefaultConsensusParams
	configured := *params
	blockParams := *params.Block
	blockParams.MaxGas = (totalTransfers+1)*21_000 + 1_000_000
	blockParams.MaxGasWanted = blockParams.MaxGas
	configured.Block = &blockParams
	app.DefaultConsensusParams = &configured
	t.Cleanup(func() { app.DefaultConsensusParams = params })
	sender := utils.NewSigner()
	primary := newFailoverRehearsalNode(t, sender)
	secondary := newFailoverRehearsalNode(t, sender)
	ctx := context.Background()
	primaryPub, err := primary.consensus.GetPubKey(ctx)
	require.NoError(t, err)
	replayReceipter, err := NewReceipter(secondary.source.exec, secondary.source.app,
		&secondary.source.app.EvmKeeper, primary.source.key, 12)
	require.NoError(t, err)
	primaryReceiptKey := primary.source.key.PublicKey()
	standby, err := NewStandby(ctx, StandbyConfig{
		WAL: secondary.log, Executor: NewAppReplayer(secondary.source.exec, replayReceipter),
		ConsensusKey: secondary.consensus, ReceiptKey: secondary.source.key.private,
		PrimaryConsensusKey: primaryPub, PrimaryReceiptKey: ed25519.PublicKey(primaryReceiptKey[:]),
		CommittedHeight: 1, LastSeq: secondary.opts.LastSeq,
	})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	served := make(chan error, 1)
	go func() { served <- standby.Serve(listener) }()
	replicator, err := NewReplicator(ReplicatorConfig{
		WAL: primary.log, Dial: DialStandby(listener.Addr().String(), 2*time.Second), ReplicationTimeout: 2 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(replicator.Close)
	openMs := time.Now().Add(time.Minute).UnixMilli()
	require.NoError(t, primary.source.exec.Open(2, openMs))
	ack, err := replicator.Release(ctx,
		&wal.Lease{Epoch: 1, ExpiryMs: uint64(openMs) + 10_000},
		&wal.Open{Height: 2, OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)},
	)
	require.NoError(t, err)
	require.Equal(t, AckOK, ack.Status)
	r, err := NewReceipter(primary.source.exec, primary.source.app, &primary.source.app.EvmKeeper, primary.source.key, 12)
	require.NoError(t, err)
	provider := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(ctx)) })
	metrics, err := NewMetrics(provider.Meter("failover-rehearsal"))
	require.NoError(t, err)
	receiptSource := NewReceiptSource(totalTransfers)
	accepter := NewAccepter(r, replicator, receiptSource, metrics)
	txs := make([][]byte, totalTransfers)
	for i := range txs {
		txs[i] = consensusFenceTransfer(t, primary.source.app, sender, uint64(i))
	}
	type outcome struct {
		receipt []byte
		err     error
	}
	returned := make(chan outcome, totalTransfers)
	reachedFailure := make(chan struct{})
	resume := make(chan struct{})
	finished := make(chan struct{})
	var resumeOnce sync.Once
	resumeSender := func() { resumeOnce.Do(func() { close(resume) }) }
	t.Cleanup(resumeSender)
	go func() {
		defer close(finished)
		for i, tx := range txs {
			if i == beforeFailure {
				close(reachedFailure)
				<-resume
			}
			receipt, err := accepter.Accept(tx)
			returned <- outcome{receipt: receipt, err: err}
			if err != nil {
				return
			}
		}
	}()
	receipts := make([]*codec.Receipt, beforeFailure)
	for i := range receipts {
		select {
		case got := <-returned:
			require.NoError(t, got.err)
			_, err := verify.ReceiptSignature(got.receipt, primaryReceiptKey)
			require.NoError(t, err)
			receipts[i], err = codec.DecodeReceipt(got.receipt)
			require.NoError(t, err)
			require.Equal(t, primary.opts.LastSeq+uint64(i)+1, receipts[i].GlobalSequence)
			require.Zero(t, receipts[i].ResultCode)
		case <-time.After(5 * time.Second):
			t.Fatal("primary did not return the next replicated receipt")
		}
	}
	select {
	case <-reachedFailure:
	case <-time.After(5 * time.Second):
		t.Fatal("sender did not reach the failure point")
	}
	require.Equal(t, primary.opts.LastSeq+beforeFailure, standby.LastSeq())
	primary.log.Close()
	replicator.Close()
	resumeSender()
	select {
	case failed := <-returned:
		require.ErrorIs(t, failed.err, ErrReplicationHalted)
		require.ErrorIs(t, failed.err, wal.ErrClosed)
		require.Nil(t, failed.receipt)
	case <-time.After(5 * time.Second):
		t.Fatal("send did not stop after primary failure")
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("sender continued after primary failure")
	}
	require.Empty(t, returned)
	failedState, err := primary.source.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(beforeFailure+1), primary.source.app.EvmKeeper.GetNonce(failedState, sender.EvmAddress))
	require.Equal(t, primary.opts.LastSeq+beforeFailure, standby.LastSeq())
	promoted, result := secondary.Failover(t, standby, primary.source.key, openMs)
	require.Equal(t, primary.opts.LastSeq+beforeFailure, result.LastSeq)
	previousHash := primary.opts.LastSeqHash
	for i, receipt := range receipts {
		require.Equal(t, previousHash, receipt.PreviousStateRoot)
		previousHash = seqtypes.SeqHash(previousHash, receipt.GlobalSequence, receipt.ActivityID)
		require.Equal(t, previousHash, receipt.ResultingStateRoot)
		pending, ok := promoted.exec.PendingTx(receipt.ActivityID)
		require.True(t, ok, "receipt seq %d lost during promotion", receipt.GlobalSequence)
		require.Equal(t, txs[i], pending.TxBytes)
		require.Equal(t, result.OpenHeight, pending.Height)
		require.Equal(t, i+1, pending.TxIndex)
		require.Zero(t, pending.Result.Code)
		retained, err := receiptSource.SequencerSend(receipt.ActivityID)
		require.NoError(t, err)
		require.NotNil(t, retained)
		require.Equal(t, receipt.CanonicalBytes(), retained.Receipt)
	}
	snapshot, err := promoted.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(beforeFailure), promoted.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress))
	for i := beforeFailure; i < totalTransfers; i++ {
		accepted, err := promoted.exec.Accept(txs[i])
		require.NoError(t, err)
		require.Zero(t, accepted.Result.Code, accepted.Result.Log)
		require.Equal(t, primary.opts.LastSeq+uint64(i)+1, accepted.Seq)
		require.Equal(t, previousHash, accepted.PrevSeqHash)
		require.Equal(t, i+1, accepted.TxIndex)
		previousHash = seqtypes.SeqHash(previousHash, accepted.Seq, accepted.ActivityID)
		require.Equal(t, previousHash, accepted.SeqHash)
	}
	sealed, err := promoted.Seal(ctx, result.OpenHeight)
	require.NoError(t, err)
	require.Equal(t, txs, sealed.Txs.ToSliceOfBytes())
	require.Equal(t, time.UnixMilli(openMs).UTC(), sealed.OpenTime)
	batch, err := app.DecodeSequenceBatch(sealed.Batch)
	require.NoError(t, err)
	require.Equal(t, primary.opts.LastSeq+1, batch.FromSeq)
	require.Equal(t, primary.opts.LastSeq+totalTransfers, batch.ToSeq)
	require.Equal(t, primary.opts.LastSeqHash[:], batch.PrevSeqHash)
	require.Equal(t, previousHash[:], batch.LastSeqHash)
	public := secondary.source.key.PublicKey()
	require.NoError(t, batch.VerifySignature(primary.opts.ChainID, public))
	require.NoError(t, standby.Err())
}
