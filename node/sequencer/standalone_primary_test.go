package sequencer_test

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmutils "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestStandalonePrimaryReceiptsTransferThroughSendAPI(t *testing.T) {
	defer func() {
		if value := recover(); value != nil {
			t.Fatalf("RPC handler panicked: %v\n%s", value, debug.Stack())
		}
	}()
	sender := utils.NewSigner()
	h := newNodeHarness(t, sender)
	info, err := h.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.StandbyAddress = ""
	cfg.WALDir = filepath.Join(t.TempDir(), "wal")
	cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
	cfg.LeaseDuration = 3 * replicationTimeout
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 1, 2
	n, err := sequencer.NewNode(cfg, h.app)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, n.Start(ctx))
	require.True(t, n.Replicator.Standalone())
	option := evmrpc.NewSequencerOption(n.Accepter, n.ReceiptLogFeed)
	sendConfig, err := option.SendConfig(evmrpcconfig.DefaultConfig)
	require.NoError(t, err)
	api := evmrpc.NewSendAPI(nil,
		func(int64) client.TxConfig { return h.app.GetTxConfig() }, sendConfig,
		&h.app.EvmKeeper, legacyabci.BeginBlockKeepers{}, h.app.RPCContextProvider,
		"", nil, nil, nil, evmrpc.ConnectionTypeHTTP, tmutils.None[time.Duration](),
		evmrpc.NewBlockCache(1), nil, nil)
	server := gethrpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", api))
	rpcClient := gethrpc.DialInProc(server)
	t.Cleanup(rpcClient.Close)
	hashes := make(chan common.Hash, 1)
	sub, err := rpcClient.EthSubscribe(ctx, hashes, "newPendingTransactions")
	require.NoError(t, err)
	defer sub.Unsubscribe()
	pending := h.app.RPCContextProvider(evmrpc.PendingCtxHeight)
	nonce := h.app.EvmKeeper.GetNonce(pending, sender.EvmAddress)
	to := utils.NewSigner().EvmAddress
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: nonce, GasPrice: big.NewInt(gasPriceWei), Gas: transferGas,
		To: &to, Value: big.NewInt(1),
	}), ethtypes.LatestSignerForChainID(h.app.EvmKeeper.ChainID(pending)), sender.EvmPrivateKey)
	require.NoError(t, err)
	raw, err := signed.MarshalBinary()
	require.NoError(t, err)
	hash, err := api.SendRawTransaction(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, signed.Hash(), hash)
	send, err := n.ReceiptSource.SequencerSend(hash)
	require.NoError(t, err)
	require.NotNil(t, send)
	require.Equal(t, nonce, send.Nonce)
	verified, err := verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, nonce, verified.Receipt.FromSequence)
	require.Equal(t, [32]byte(hash), verified.Receipt.ActivityID)
	require.Equal(t, sourceLastSeq+1, verified.Receipt.GlobalSequence)
	require.Zero(t, verified.Receipt.ResultCode)
	require.Empty(t, send.VMError)
	snapshot := h.app.RPCContextProvider(evmrpc.PendingCtxHeight)
	require.Zero(t, big.NewInt(1).Cmp(h.weiBalance(snapshot, to)))
	require.Equal(t, nonce+1, h.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress))
	synced, err := n.WAL.GroupSync()
	require.NoError(t, err)
	require.Zero(t, synced, "the signed receipt must be durable before returning the hash")
	select {
	case emitted := <-hashes:
		require.Equal(t, hash, emitted)
	case err := <-sub.Err():
		t.Fatalf("pending subscription stopped before the receipted hash: %v", err)
	case <-ctx.Done():
		t.Fatalf("pending feed did not emit the receipted hash: %v", ctx.Err())
	}
}

func TestReplicatorStandaloneAcknowledgesAfterWALSync(t *testing.T) {
	sender := utils.NewSigner()
	h := newAccepterAppHarness(t, sender)
	dir := t.TempDir()
	w, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	var syncs int
	r, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, Standalone: true,
		OnWALSync: func(d time.Duration) {
			require.GreaterOrEqual(t, d, time.Duration(0))
			syncs++
		},
	})
	require.NoError(t, err)
	t.Cleanup(r.Close)
	require.True(t, r.Standalone())

	_, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{WAL: w})
	require.Error(t, err, "a replicated primary still requires a dialer")
	_, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, Standalone: true, Dial: sequencer.DialStandby("127.0.0.1:1", replicationTimeout),
		ReplicationTimeout: replicationTimeout,
	})
	require.Error(t, err, "standalone and a standby dialer are mutually exclusive")

	openMs := time.Now().Add(time.Minute).UnixMilli()
	require.NoError(t, h.exec.Open(openHeight, openMs))
	open := &wal.Open{Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)}
	var expected []byte
	release := func(maxSeq uint64, entries ...wal.Entry) {
		t.Helper()
		before := syncs
		ack, err := r.Release(context.Background(), entries...)
		require.NoError(t, err)
		require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: maxSeq}, ack)
		require.Equal(t, before+1, syncs, "Release returns after the WAL sync callback")
		for _, entry := range entries {
			frame, err := wal.NewFrame(entry)
			require.NoError(t, err)
			expected = append(expected, frame...)
		}
		stored, err := os.ReadFile(filepath.Join(dir, "head.seg"))
		require.NoError(t, err)
		require.Equal(t, expected, stored, "frames are flushed before Release returns")
		n, err := w.GroupSync()
		require.NoError(t, err)
		require.Zero(t, n, "Release already synced every appended frame")
	}
	release(0, open)
	var accepts []*wal.Accept
	for nonce := uint64(0); nonce < 2; nonce++ {
		_, tx := transfer(t, sender, nonce, utils.NewSigner().EvmAddress, big.NewInt(1))
		accepted, err := h.exec.Accept(tx)
		require.NoError(t, err)
		receipt, err := receiptBytes(accepted)
		require.NoError(t, err)
		accepts = append(accepts, &wal.Accept{Seq: accepted.Seq, Height: uint64(accepted.Height), Tx: tx, Receipt: receipt})
	}
	release(accepts[1].Seq, accepts[0], accepts[1])
	lease := &wal.Lease{Epoch: 1, KeyID: [32]byte{1}, ExpiryMs: uint64(time.Now().Add(time.Second).UnixMilli())}
	release(accepts[1].Seq, lease)
	require.Equal(t, uint64(1), r.Epoch())
	ack, err := r.Drain(context.Background())
	require.ErrorIs(t, err, sequencer.ErrNoAck)
	require.Equal(t, sequencer.Ack{}, ack)
	require.Equal(t, 3, syncs)

	w.Close()
	w, err = wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	recovered := newAccepterAppHarness(t, sender)
	result, err := w.Replay(appReplayer{exec: recovered.exec})
	require.NoError(t, err)
	require.Equal(t, 2, result.Accepts)
	require.Equal(t, accepts[1], result.LastAccept)
	require.Equal(t, lease, result.Lease)
	require.Nil(t, result.Tail)
}

func TestNodeStartsPrimaryWithoutStandbyAddress(t *testing.T) {
	for _, standalone := range []bool{true, false} {
		name := "standalone"
		if !standalone {
			name = "replicated"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			h := newNodeHarness(t, sender)
			info, err := h.app.UpgradeKeeper.GetUpgradeInfoPath()
			require.NoError(t, err)
			consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
			require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
			pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
			require.NoError(t, err)
			require.NoError(t, pv.Save())
			cfg := sequencer.DefaultConfig()
			cfg.Enable = true
			cfg.WALDir = filepath.Join(t.TempDir(), "wal")
			cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
			cfg.LeaseDuration = 3 * replicationTimeout
			cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 1, 2
			if !standalone {
				l := listen(t)
				cfg.StandbyAddress = l.Addr().String()
				cfg.ReplicationTLS, _ = sequencer.Repair103TLSFixture(t)
				require.NoError(t, l.Close())
				cfg.ReplicationTimeout = replicationTimeout
			}
			n, err := sequencer.NewNode(cfg, h.app)
			require.NoError(t, err)
			require.NotNil(t, n)
			t.Cleanup(n.WAL.Close)
			t.Cleanup(n.Replicator.Close)
			require.Equal(t, standalone, n.Replicator.Standalone())
			require.Nil(t, n.Standby)
			err = n.Start(context.Background())
			if !standalone {
				require.ErrorIs(t, err, sequencer.ErrNoAck, "a configured standby must acknowledge the OPEN")
				return
			}
			require.NoError(t, err)
			_, tx := transfer(t, sender, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
			receipt, err := n.Accepter.Accept(tx)
			require.NoError(t, err)
			verified, err := verify.ReceiptSignature(receipt.Receipt, n.ReceiptKey.PublicKey())
			require.NoError(t, err)
			require.Equal(t, sourceLastSeq+1, verified.Receipt.GlobalSequence)
			nSynced, err := n.WAL.GroupSync()
			require.NoError(t, err)
			require.Zero(t, nSynced, "the receipt is returned only after the ACCEPT is durable")
		})
	}
}
