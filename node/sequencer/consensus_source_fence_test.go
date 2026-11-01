package sequencer

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type consensusFenceHarness struct {
	src       *ConsensusSource
	opts      Options
	consensus *privval.FilePV
	openedAt  int64
}

func newConsensusFenceHarness(t *testing.T, lastSeq uint64, sender utils.TestAcct) *consensusFenceHarness {
	t.Helper()
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	params := app.DefaultConsensusParams
	configured := *params
	block := *params.Block
	block.MaxGas = 1_000_000
	block.MaxGasWanted = 1_000_000
	configured.Block = &block
	app.DefaultConsensusParams = &configured
	t.Cleanup(func() { app.DefaultConsensusParams = params })

	dir := t.TempDir()
	consensusFile := filepath.Join(dir, "consensus-key.json")
	pv, err := privval.GenFilePV(consensusFile, filepath.Join(dir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	seed := sha256.Sum256([]byte(t.Name() + dir))
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

	now := time.Now().UTC()
	valPub := secp256k1.GenPrivKey().PubKey()
	tw := app.NewTestWrapper(t, now, valPub, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := tw.App
	a.EvmKeeper.SetAddressMapping(tw.Ctx, sender.AccountAddress, sender.EvmAddress)
	tw.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1_000_000_000_000))))
	previousHash := sha256.Sum256([]byte("consensus fence previous sequence hash"))
	require.NoError(t, a.SequencerKeeper.InitGenesis(tw.Ctx, seqtypes.GenesisState{
		Leaders: [][32]byte{keyID},
		ReceiptKeys: []seqtypes.ReceiptKey{{
			KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: 100,
		}},
		LastSeq: lastSeq, LastSeqHash: previousHash,
	}))
	a.UpgradeKeeper.SetDone(tw.Ctx.WithBlockHeight(1), app.MergeUpgrade)
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	opts := Options{
		ChainID: "pax-test", ProposerAddress: valPub.Address().Bytes(),
		LastSeq: lastSeq, LastSeqHash: previousHash,
	}
	exec, err := NewExecutor(a, opts)
	require.NoError(t, err)
	src, err := NewConsensusSource(a, exec, key)
	require.NoError(t, err)
	require.True(t, src.Sequenced(2))
	return &consensusFenceHarness{src: src, opts: opts, consensus: pv, openedAt: now.Add(time.Second).UnixMilli()}
}

func consensusFenceStandby(t *testing.T, primary, secondary *consensusFenceHarness) (*Standby, *Replicator) {
	t.Helper()
	primaryWAL, err := wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(primaryWAL.Close)
	standbyWAL, err := wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(standbyWAL.Close)
	primaryPub, err := primary.consensus.GetPubKey(context.Background())
	require.NoError(t, err)
	receipter, err := NewReceipter(secondary.src.exec, secondary.src.app, &secondary.src.app.EvmKeeper, primary.src.key, 12)
	require.NoError(t, err)
	sb, err := NewStandby(context.Background(), StandbyConfig{
		WAL: standbyWAL, Executor: NewAppReplayer(secondary.src.exec, receipter),
		ConsensusKey: secondary.consensus, ReceiptKey: secondary.src.key.private,
		PrimaryConsensusKey: primaryPub, PrimaryReceiptKey: primary.src.key.private.Public().(ed25519.PublicKey),
		CommittedHeight: 1, LastSeq: secondary.opts.LastSeq,
	})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	go func() { _ = sb.Serve(listener) }()
	repl, err := NewReplicator(ReplicatorConfig{
		WAL: primaryWAL,
		Dial: func(ctx context.Context) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", listener.Addr().String())
		},
		ReplicationTimeout: time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(repl.Close)
	return sb, repl
}

func consensusFenceTransfer(t *testing.T, a *app.App, sender utils.TestAcct, nonce uint64) []byte {
	t.Helper()
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000de")
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: 21000, To: &recipient, Value: big.NewInt(1),
	}), ethtypes.LatestSignerForChainID(big.NewInt(config.DefaultChainID)), sender.EvmPrivateKey)
	require.NoError(t, err)
	data, err := ethtx.NewTxDataFromTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	config := a.GetTxConfig()
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(21000)
	raw, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return raw
}

func TestConsensusSourceFencesOnErrFenced(t *testing.T) {
	sender := utils.NewSigner()
	primary := newConsensusFenceHarness(t, 42, sender)
	secondary := newConsensusFenceHarness(t, 42, sender)
	sb, repl := consensusFenceStandby(t, primary, secondary)
	src, err := NewConsensusSource(primary.src.app, primary.src.exec, primary.src.key, WithConsensusReplicator(repl))
	require.NoError(t, err)
	require.NoError(t, src.exec.Open(2, primary.openedAt))
	accepted, err := src.exec.Accept(consensusFenceTransfer(t, src.app, sender, 0))
	require.NoError(t, err)
	require.Equal(t, uint64(43), accepted.Seq)
	_, err = repl.Release(context.Background(),
		&wal.Lease{Epoch: 1, ExpiryMs: uint64(primary.openedAt)},
		&wal.Open{Height: 2, OpenTimeMs: uint64(primary.openedAt), BaseFee: uint256.NewInt(0)},
	)
	require.NoError(t, err)
	result, err := Failover(sb, secondary.src.key, 2, uint64(primary.openedAt)+30_000)
	require.NoError(t, err)
	require.Equal(t, uint64(2), result.Lease.Epoch)
	require.Equal(t, uint64(1), repl.Epoch())

	for range 2 {
		block, err := src.Seal(context.Background(), 2)
		require.ErrorIs(t, err, ErrLeaseFenced)
		require.Empty(t, block)
		require.Nil(t, src.Sealed())
		require.ErrorIs(t, src.exec.Open(2, primary.openedAt), ErrLeaseFenced)
		accepted, err := src.exec.Accept(consensusFenceTransfer(t, src.app, sender, 1))
		require.ErrorIs(t, err, ErrLeaseFenced)
		require.Nil(t, accepted)
		batch, err := src.exec.Seal(2)
		require.ErrorIs(t, err, ErrLeaseFenced)
		require.Nil(t, batch)
	}
	require.NoError(t, sb.Err())
	require.Equal(t, uint64(42), sb.LastSeq())
	stored, err := src.app.SequencerKeeper.LastSeq(src.app.GetCheckCtx())
	require.NoError(t, err)
	require.Equal(t, uint64(42), stored)
}

func TestPromotedConsensusSourceContinuesSeq(t *testing.T) {
	sender := utils.NewSigner()
	primary := newConsensusFenceHarness(t, 42, sender)
	secondary := newConsensusFenceHarness(t, 42, sender)
	sb, _ := consensusFenceStandby(t, primary, secondary)
	result, err := Failover(sb, secondary.src.key, 2, uint64(secondary.openedAt)+30_000)
	require.NoError(t, err)
	require.Equal(t, uint64(42), result.LastSeq)
	opts := secondary.opts
	opts.LastSeq = result.LastSeq + 1
	src, err := NewPromotedConsensusSource(secondary.src.app, secondary.src.app, opts, result.LastSeq, secondary.src.key)
	require.ErrorIs(t, err, ErrPromotionSeq)
	require.Nil(t, src)
	for _, baseline := range []uint64{result.LastSeq - 1, result.LastSeq} {
		opts.LastSeq = baseline
		src, err = NewPromotedConsensusSource(secondary.src.app, secondary.src.app, opts, result.LastSeq, secondary.src.key)
		require.NoError(t, err)
		require.Equal(t, baseline, opts.LastSeq)
		require.NoError(t, src.exec.Open(result.OpenHeight, secondary.openedAt))
		tx := consensusFenceTransfer(t, src.app, sender, 0)
		accepted, err := src.exec.Accept(tx)
		require.NoError(t, err)
		require.Equal(t, uint32(0), accepted.Result.Code, accepted.Result.Log)
		require.Equal(t, result.LastSeq+1, accepted.Seq)
		require.Equal(t, opts.LastSeqHash, accepted.PrevSeqHash)
		block, err := src.Seal(context.Background(), result.OpenHeight)
		require.NoError(t, err)
		require.Equal(t, [][]byte{tx}, block.Txs.ToSliceOfBytes())
		msg, err := app.DecodeSequenceBatch(block.Batch)
		require.NoError(t, err)
		require.Equal(t, result.LastSeq+1, msg.FromSeq)
		require.Equal(t, accepted.Seq, msg.ToSeq)
		require.Equal(t, opts.LastSeqHash[:], msg.PrevSeqHash)
		require.Equal(t, accepted.SeqHash[:], msg.LastSeqHash)
		public := secondary.src.key.PublicKey()
		require.NoError(t, msg.VerifySignature(opts.ChainID, public))
		require.Equal(t, time.UnixMilli(secondary.openedAt).UTC(), block.OpenTime)
		stored, err := src.app.SequencerKeeper.LastSeq(src.Sealed().Context())
		require.NoError(t, err)
		require.Equal(t, result.LastSeq+1, stored)
	}
	badOpts := opts
	badOpts.ChainID = ""
	src, err = NewPromotedConsensusSource(secondary.src.app, secondary.src.app, badOpts, result.LastSeq, secondary.src.key)
	require.ErrorContains(t, err, "empty chain id")
	require.Nil(t, src)
	src, err = NewPromotedConsensusSource(nil, secondary.src.app, opts, result.LastSeq, secondary.src.key)
	require.ErrorContains(t, err, "needs an app")
	require.Nil(t, src)
}
