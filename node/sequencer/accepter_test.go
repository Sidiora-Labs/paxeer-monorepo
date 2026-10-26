package sequencer_test

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type accepterReceiptReplayer struct {
	appReplayer
	r *sequencer.Receipter
}

func (r accepterReceiptReplayer) Execute(entry *wal.Accept) ([]byte, error) {
	acc, receipt, err := r.r.Accept(entry.Tx)
	if err != nil {
		return nil, err
	}
	if acc.Seq != entry.Seq || uint64(acc.Height) != entry.Height {
		return nil, fmt.Errorf("replayed position differs: seq %d height %d", acc.Seq, acc.Height)
	}
	return receipt, nil
}

type accepterHarness struct {
	h      *harness
	sb     *sequencer.Standby
	w      *wal.WAL
	sw     *wal.WAL
	dir    string
	sdir   string
	l      net.Listener
	a      *sequencer.Accepter
	src    *sequencer.ReceiptSource
	key    *sequencer.ReceiptKey
	reader *sdkmetric.ManualReader
	synced chan struct{}
}

func newAccepterAppHarness(t *testing.T, funded ...utils.TestAcct) *harness {
	t.Helper()
	tm := time.Now().UTC()
	valPub := secp256k1.GenPrivKey().PubKey()
	tw := app.NewTestWrapper(t, tm, valPub, false)
	a := tw.App
	admissionParams(1_000_000, 1_000_000, 200_000)(tw.Ctx, a)
	for _, acct := range funded {
		a.EvmKeeper.SetAddressMapping(tw.Ctx, acct.AccountAddress, acct.EvmAddress)
		tw.FundAcc(acct.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(fundingUhpx))))
	}
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: committedTip},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	proposer := valPub.Address().Bytes()
	exec, err := sequencer.NewExecutor(a, sequencer.Options{
		ChainID: testChainID, ProposerAddress: proposer, LastSeq: startSeq, LastSeqHash: startSeqHash,
	})
	require.NoError(t, err)
	return &harness{t: t, app: a, exec: exec, proposer: proposer, genesis: tm}
}

func newAccepterHarness(t *testing.T, funded ...utils.TestAcct) *accepterHarness {
	t.Helper()
	h, sh := newAccepterAppHarness(t, funded...), newAccepterAppHarness(t, funded...)
	key, err := loadKey(t, testKeySeed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(testKeySeed), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, openHeight)
	require.NoError(t, err)
	r, err := sequencer.NewReceipter(h.exec, h.app, &h.app.EvmKeeper, key, receiptModuleID)
	require.NoError(t, err)
	sr, err := sequencer.NewReceipter(sh.exec, sh.app, &sh.app.EvmKeeper, key, receiptModuleID)
	require.NoError(t, err)
	dir, sdir := t.TempDir(), t.TempDir()
	w, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	sw, err := wal.OpenWAL(sdir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(sw.Close)
	openMs := time.Now().Add(time.Minute).UnixMilli()
	open := &wal.Open{Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)}
	for _, log := range []*wal.WAL{w, sw} {
		pending, err := log.Append(open)
		require.NoError(t, err)
		_, err = log.GroupSync()
		require.NoError(t, err)
		require.NoError(t, pending.Wait(context.Background()))
	}
	require.NoError(t, h.exec.Open(openHeight, openMs))
	keyDir := t.TempDir()
	pv, err := privval.GenFilePV(filepath.Join(keyDir, "standby-key.json"), filepath.Join(keyDir, "standby-state.json"), "")
	require.NoError(t, err)
	primaryPV, err := privval.GenFilePV(filepath.Join(keyDir, "primary-key.json"), filepath.Join(keyDir, "primary-state.json"), "")
	require.NoError(t, err)
	primaryPub, err := primaryPV.GetPubKey(context.Background())
	require.NoError(t, err)
	_, standbyKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	pub := key.PublicKey()
	sb, err := sequencer.NewStandby(context.Background(), sequencer.StandbyConfig{
		WAL: sw, Executor: accepterReceiptReplayer{appReplayer{exec: sh.exec}, sr},
		ConsensusKey: pv, ReceiptKey: standbyKey,
		PrimaryConsensusKey: primaryPub, PrimaryReceiptKey: ed25519.PublicKey(pub[:]),
		CommittedHeight: uint64(committedTip), LastSeq: startSeq,
	})
	require.NoError(t, err)
	l := listen(t)
	t.Cleanup(func() { _ = l.Close() })
	m, reader := newSequencerMetricsReader(t)
	synced := make(chan struct{}, 32)
	rep, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, Dial: sequencer.DialStandby(l.Addr().String(), replicationTimeout),
		ReplicationTimeout: replicationTimeout,
		OnWALSync: func(d time.Duration) {
			m.ObserveWALSync(d)
			synced <- struct{}{}
		},
	})
	require.NoError(t, err)
	t.Cleanup(rep.Close)
	src := sequencer.NewReceiptSource(32)
	return &accepterHarness{
		h: h, sb: sb, w: w, sw: sw, dir: dir, sdir: sdir, l: l,
		a: sequencer.NewAccepter(r, rep, src, m), src: src, key: key, reader: reader, synced: synced,
	}
}

func (h *accepterHarness) serve() {
	go func() { _ = h.sb.Serve(h.l) }()
}

func accepterWALEntries(t *testing.T, dir string) []wal.Entry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "head.seg"))
	require.NoError(t, err)
	var entries []wal.Entry
	for len(data) > 0 {
		require.GreaterOrEqual(t, len(data), 8)
		size := int(binary.BigEndian.Uint32(data[4:8]))
		require.GreaterOrEqual(t, len(data), 8+size)
		entry, err := wal.DecodeEntry(data[8 : 8+size])
		require.NoError(t, err)
		entries = append(entries, entry)
		data = data[8+size:]
	}
	return entries
}

func accepterHistogram(t *testing.T, h *accepterHarness, name string) metricdata.HistogramDataPoint[float64] {
	t.Helper()
	metrics := collectSequencerMetrics(t, h.reader)
	require.Contains(t, metrics, name)
	hist, ok := metrics[name].Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	require.Len(t, hist.DataPoints, 1)
	return hist.DataPoints[0]
}

func TestAccepterReturnsReceiptAfterStandbyAck(t *testing.T) {
	sender := utils.NewSigner()
	to := utils.NewSigner().EvmAddress
	h := newAccepterHarness(t, sender)
	tx, bz := transfer(t, sender, 0, to, big.NewInt(onePaxInWei))
	type outcome struct {
		receipt []byte
		err     error
	}
	returned := make(chan outcome, 1)
	go func() {
		receipt, err := h.a.Accept(bz)
		returned <- outcome{receipt, err}
	}()
	select {
	case <-h.synced:
	case result := <-returned:
		t.Fatalf("Accept returned before WAL sync: %x, %v", result.receipt, result.err)
	case <-time.After(5 * time.Second):
		t.Fatal("primary did not sync the accepted send")
	}
	entries := accepterWALEntries(t, h.dir)
	require.Len(t, entries, 2)
	persisted, ok := entries[1].(*wal.Accept)
	require.True(t, ok)
	require.Equal(t, bz, persisted.Tx)
	require.Equal(t, startSeq+1, persisted.Seq)
	require.Equal(t, uint64(openHeight), persisted.Height)
	require.Equal(t, startSeq, h.sb.LastSeq())
	send, err := h.src.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.Nil(t, send)
	require.NotContains(t, collectSequencerMetrics(t, h.reader), "sequencer_receipt_latency_seconds")
	const ackDelay = 50 * time.Millisecond
	select {
	case result := <-returned:
		t.Fatalf("Accept returned before standby served the group: %x, %v", result.receipt, result.err)
	case <-time.After(ackDelay):
	}
	h.serve()
	var result outcome
	select {
	case result = <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("Accept did not return after standby acknowledgement")
	}
	require.NoError(t, result.err)
	require.Equal(t, persisted.Receipt, result.receipt)
	verified, err := verify.ReceiptSignature(result.receipt, h.key.PublicKey())
	require.NoError(t, err)
	require.Equal(t, startSeq+1, verified.Receipt.GlobalSequence)
	require.Equal(t, startSeq+1, h.sb.LastSeq())
	require.NoError(t, h.sb.Err())
	sentries := accepterWALEntries(t, h.sdir)
	require.Len(t, sentries, 2)
	require.Equal(t, persisted, sentries[1])
	send, err = h.src.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.NotNil(t, send)
	require.Equal(t, result.receipt, send.Receipt)
	for _, name := range []string{"sequencer_receipt_latency_seconds", "sequencer_standby_ack_seconds"} {
		point := accepterHistogram(t, h, name)
		require.Equal(t, uint64(1), point.Count)
		require.GreaterOrEqual(t, point.Sum, ackDelay.Seconds())
	}
}

func TestAccepterHaltsAfterReleaseFailure(t *testing.T) {
	for _, failure := range []string{"timeout", "standby WAL closed", "primary WAL closed"} {
		t.Run(failure, func(t *testing.T) {
			sender := utils.NewSigner()
			to := utils.NewSigner().EvmAddress
			h := newAccepterHarness(t, sender)
			if failure != "timeout" {
				h.serve()
			}
			want := sequencer.ErrNoAck
			if failure == "standby WAL closed" {
				h.sw.Close()
				want = sequencer.ErrStandbyRefused
			}
			if failure == "primary WAL closed" {
				h.w.Close()
				want = wal.ErrClosed
			}
			tx, bz := transfer(t, sender, 0, to, big.NewInt(onePaxInWei))
			receipt, err := h.a.Accept(bz)
			require.Nil(t, receipt)
			require.ErrorIs(t, err, want)
			require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
			if failure == "timeout" {
				h.serve()
			}
			before := accepterWALEntries(t, h.dir)
			_, next := transfer(t, sender, 1, to, big.NewInt(onePaxInWei))
			var group sync.WaitGroup
			errors := make(chan error, 8)
			for i := 0; i < 8; i++ {
				group.Add(1)
				go func() {
					defer group.Done()
					receipt, err := h.a.Accept(next)
					if receipt != nil {
						errors <- fmt.Errorf("halted send returned receipt")
						return
					}
					errors <- err
				}()
			}
			group.Wait()
			close(errors)
			for err := range errors {
				require.Equal(t, sequencer.ErrReplicationHalted, err)
			}
			receipt, err = h.a.Accept(nil)
			require.Nil(t, receipt)
			require.Equal(t, sequencer.ErrReplicationHalted, err)
			require.Equal(t, before, accepterWALEntries(t, h.dir))
			require.Equal(t, uint64(1), h.h.app.EvmKeeper.GetNonce(h.h.openCtx(), sender.EvmAddress))
			send, err := h.src.SequencerSend(tx.Hash())
			require.NoError(t, err)
			require.Nil(t, send)
			require.NotContains(t, collectSequencerMetrics(t, h.reader), "sequencer_receipt_latency_seconds")
			require.Equal(t, uint64(1), accepterHistogram(t, h, "sequencer_standby_ack_seconds").Count)
		})
	}
}

func TestAccepterRecordsReceiptLatency(t *testing.T) {
	sender := utils.NewSigner()
	to := utils.NewSigner().EvmAddress
	h := newAccepterHarness(t, sender)
	h.serve()
	contract := crypto.CreateAddress(sender.EvmAddress, 1)
	txs := []*ethtypes.LegacyTx{
		receiptLegacyTx(0, &to, big.NewInt(onePaxInWei), transferGas, nil),
		receiptLegacyTx(1, nil, new(big.Int), createGas, logInit),
		receiptLegacyTx(2, &contract, new(big.Int), callGas, nil),
		receiptLegacyTx(3, nil, new(big.Int), createGas, revertInit),
	}
	for i, input := range txs {
		tx, bz := evmTx(t, sender, input)
		receipt, err := h.a.Accept(bz)
		require.NoError(t, err)
		_, err = verify.ReceiptSignature(receipt, h.key.PublicKey())
		require.NoError(t, err)
		send, err := h.src.SequencerSend(tx.Hash())
		require.NoError(t, err)
		require.NotNil(t, send)
		require.Equal(t, uint64(i), send.Nonce)
		require.Equal(t, receipt, send.Receipt)
		rcpt, err := h.h.app.EvmKeeper.GetTransientReceipt(h.h.openCtx(), tx.Hash(), uint64(i+1))
		require.NoError(t, err)
		require.Equal(t, rcpt.VmError, send.VMError)
		require.Len(t, send.LogData, len(rcpt.Logs))
		for j, log := range rcpt.Logs {
			require.Equal(t, log.Data, send.LogData[j])
		}
		if i == 2 {
			require.Len(t, send.LogData, 3)
		}
		if i == 3 {
			require.NotEmpty(t, send.VMError)
			require.Equal(t, errorX, send.ReturnData)
		}
		for _, name := range []string{"sequencer_receipt_latency_seconds", "sequencer_standby_ack_seconds", "sequencer_wal_sync_seconds"} {
			point := accepterHistogram(t, h, name)
			require.Equal(t, uint64(i+1), point.Count)
			require.Positive(t, point.Sum)
		}
	}
	for _, invalid := range [][]byte{nil, {0xff}} {
		receipt, err := h.a.Accept(invalid)
		require.Error(t, err)
		require.Nil(t, receipt)
	}
	require.Equal(t, uint64(4), accepterHistogram(t, h, "sequencer_receipt_latency_seconds").Count)
	require.Equal(t, uint64(4), accepterHistogram(t, h, "sequencer_standby_ack_seconds").Count)
	require.Equal(t, startSeq+4, h.sb.LastSeq())
	_, next := transfer(t, sender, 4, common.Address{1}, big.NewInt(onePaxInWei))
	h.w.Close()
	receipt, err := h.a.Accept(next)
	require.Nil(t, receipt)
	require.ErrorIs(t, err, wal.ErrClosed)
	require.Equal(t, uint64(4), accepterHistogram(t, h, "sequencer_receipt_latency_seconds").Count)
	require.Equal(t, uint64(5), accepterHistogram(t, h, "sequencer_standby_ack_seconds").Count)
}
