package state_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair89Fixture struct {
	app        *app.App
	node       *sequencer.Node
	cfg        sequencer.Config
	state      sm.State
	stateDir   string
	db         dbm.DB
	dbClosed   bool
	stateStore sm.Store
	exec       *sm.BlockExecutor
	pool       *mempool.TxMempool
	senders    []utils.TestAcct
	recipient  utils.TestAcct
}

func newRepair89Fixture(t *testing.T) *repair89Fixture {
	t.Helper()
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	val := sdked25519.GenPrivKey().PubKey()
	at := time.Now().Add(-time.Minute).UTC()
	tw := app.NewTestWrapper(t, at, val, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	f := &repair89Fixture{app: tw.App, recipient: utils.NewSigner(), stateDir: t.TempDir()}
	for range 3 {
		sender := utils.NewSigner()
		f.senders = append(f.senders, sender)
		f.app.EvmKeeper.SetAddressMapping(tw.Ctx, sender.AccountAddress, sender.EvmAddress)
		tw.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1_000_000_000_000))))
	}
	seed := sha256.Sum256([]byte("repair 89 receipt signing key"))
	public := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	var pub [32]byte
	copy(pub[:], public)
	keyID := sha256.Sum256(pub[:])
	require.NoError(t, f.app.SequencerKeeper.InitGenesis(tw.Ctx, seqtypes.GenesisState{
		ActivationHeight: 2,
		Leaders:          [][32]byte{[32]byte(val.Bytes())},
		ReceiptKeys:      []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: pub, FirstHeight: 1, LastHeight: 1000}},
	}))
	f.app.UpgradeKeeper.SetDone(tw.Ctx.WithBlockHeight(1), app.MergeUpgrade)
	_, err := f.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at},
	})
	require.NoError(t, err)
	f.app.SetDeliverStateToCommit()
	_, err = f.app.Commit(t.Context())
	require.NoError(t, err)
	info, err := f.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	f.cfg = sequencer.DefaultConfig()
	f.cfg.Enable = true
	f.cfg.WALDir = filepath.Join(t.TempDir(), "wal")
	f.cfg.ReceiptKeyFile = filepath.Join(t.TempDir(), "receipt-key")
	f.cfg.LeaseDuration = time.Minute
	f.cfg.NonceQueueMaxPerSender, f.cfg.NonceQueueMaxTotal = 2, 8
	require.NoError(t, os.WriteFile(f.cfg.ReceiptKeyFile, []byte(hex.EncodeToString(seed[:])), 0o600))
	f.node, err = sequencer.NewNode(f.cfg, f.app)
	require.NoError(t, err)
	require.NoError(t, f.node.Start(t.Context()))
	f.state, _, _ = makeState(t, 1, 2)
	f.state.ChainID = "pax-test"
	f.state.LastBlockTime = at
	f.openConsensus(t)
	t.Cleanup(func() {
		f.exec.Stop()
		if !f.dbClosed {
			require.NoError(t, f.stateStore.Close())
		}
		require.NoError(t, f.node.Stop())
	})
	return f
}

func (f *repair89Fixture) openConsensus(t *testing.T) {
	t.Helper()
	var err error
	f.db, err = dbm.NewGoLevelDB("state", f.stateDir)
	require.NoError(t, err)
	f.dbClosed = false
	f.stateStore = sm.NewStore(f.db)
	require.NoError(t, f.stateStore.Save(f.state))
	proxyApp := proxy.New(f.app, proxy.NopMetrics())
	f.pool = makeTxMempool(t, proxyApp)
	f.exec = sm.NewBlockExecutor(f.stateStore, proxyApp, f.pool, sm.EmptyEvidencePool{},
		store.NewBlockStore(dbm.NewMemDB()), eventbus.NewDefault(), sm.NopMetrics(), tmtypes.DefaultConsensusPolicy())
	f.exec.SetSequencerSource(f.node.ConsensusSource)
}

func (f *repair89Fixture) restart(t *testing.T) {
	t.Helper()
	f.exec.Stop()
	if !f.dbClosed {
		require.NoError(t, f.stateStore.Close())
	}
	require.NoError(t, f.node.Stop())
	f.app.SetOpenBlock(nil)
	f.app.SetSequencerSource(nil)
	f.app.SetSequencerRPC(evmrpc.SequencerOption{}, evmrpc.SequencerReceiptOption{})
	f.app.SetCommitListener(nil)
	var err error
	f.node, err = sequencer.NewNode(f.cfg, f.app)
	require.NoError(t, err)
	require.NoError(t, f.node.Start(t.Context()))
	f.openConsensus(t)
	require.Zero(t, f.pool.Size(), "recovery must not depend on mempool resubmission")
}

func (f *repair89Fixture) tx(t *testing.T, sender utils.TestAcct, amount int64) tmtypes.Tx {
	t.Helper()
	acct := f.app.AccountKeeper.GetAccount(f.app.GetCheckCtx(), sender.AccountAddress)
	require.NotNil(t, acct)
	tc := f.app.GetTxConfig()
	builder := tc.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(banktypes.NewMsgSend(sender.AccountAddress, f.recipient.AccountAddress,
		sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(amount))))))
	builder.SetGasLimit(200_000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(30_000))))
	mode := tc.SignModeHandler().DefaultMode()
	require.NoError(t, builder.SetSignatures(signing.SignatureV2{
		PubKey: sender.PrivateKey.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: acct.GetSequence(),
	}))
	sig, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{
		ChainID: "pax-test", AccountNumber: acct.GetAccountNumber(), Sequence: acct.GetSequence(),
	}, builder, sender.PrivateKey, tc, acct.GetSequence())
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(sig))
	raw, err := tc.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return raw
}

func (f *repair89Fixture) add(t *testing.T, txs ...tmtypes.Tx) {
	t.Helper()
	for _, tx := range txs {
		result, err := f.pool.CheckTx(t.Context(), tx)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, result.Code, result.Log)
	}
}

func (f *repair89Fixture) propose(t *testing.T) (*tmtypes.Block, error) {
	t.Helper()
	return f.exec.CreateProposalBlock(t.Context(), f.app.LastBlockHeight()+1, f.state,
		&tmtypes.Commit{}, f.state.Validators.GetProposer().Address)
}

func (f *repair89Fixture) journal(t *testing.T) (tmtypes.Txs, map[string]bool) {
	t.Helper()
	raw, err := f.db.Get([]byte("sequencer/proposal-admission/v1"))
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	var saved struct {
		Pending      tmtypes.Txs
		Acknowledged map[string]bool
	}
	require.NoError(t, json.Unmarshal(raw, &saved))
	return saved.Pending, saved.Acknowledged
}

func (f *repair89Fixture) commit(t *testing.T, block *tmtypes.Block) {
	t.Helper()
	result, err := f.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: block.Header.ToProto(), Txs: block.Txs.ToSliceOfBytes(),
	})
	require.NoError(t, err)
	require.Len(t, result.TxResults, len(block.Txs))
	for _, tx := range result.TxResults {
		require.Zero(t, tx.Code, tx.Log)
	}
	f.app.SetDeliverStateToCommit()
	_, err = f.app.Commit(t.Context())
	require.NoError(t, err)
	f.state.LastBlockHeight, f.state.LastBlockTime = block.Height, block.Time
	require.Equal(t, block.Height, f.app.LastBlockHeight())
}

func (f *repair89Fixture) assertCommitted(t *testing.T, txs tmtypes.Txs, amount int64) {
	t.Helper()
	ctx := f.app.GetCheckCtx()
	require.Equal(t, sdk.NewInt(amount), f.app.BankKeeper.GetBalance(ctx, f.recipient.AccountAddress, "uhpx").Amount)
	for _, tx := range txs {
		receipt, err := f.node.ReceiptSource.SequencerSend(common.Hash(sha256.Sum256(tx)))
		require.NoError(t, err)
		require.NotNil(t, receipt)
		verified, err := verify.ReceiptSignature(receipt.Receipt, f.node.ReceiptKey.PublicKey())
		require.NoError(t, err)
		require.Equal(t, sha256.Sum256(tx), verified.Receipt.ActivityID)
	}
}

func TestRepair_89_FatalAdmissionRestart(t *testing.T) {
	for _, failure := range []string{"fenced", "layer-not-open", "replication-halted"} {
		t.Run(failure, func(t *testing.T) {
			f := newRepair89Fixture(t)
			txs := tmtypes.Txs{f.tx(t, f.senders[0], 7), f.tx(t, f.senders[1], 11)}
			f.add(t, txs...)
			var want error
			switch failure {
			case "fenced":
				require.NoError(t, f.node.Executor.Fence())
				want = sequencer.ErrLeaseFenced
			case "layer-not-open":
				_, err := f.node.Executor.Seal(2)
				require.NoError(t, err)
				want = sequencer.ErrLayerNotOpen
			case "replication-halted":
				f.node.WAL.Close()
				want = sequencer.ErrReplicationHalted
			}
			block, err := f.propose(t)
			require.ErrorIs(t, err, want)
			require.Nil(t, block)
			left, _ := f.pool.ReapTxs(mempool.ReapLimits{}, false)
			require.ElementsMatch(t, txs, left)
			pending, acknowledged := f.journal(t)
			require.ElementsMatch(t, txs, pending)
			require.Empty(t, acknowledged)
			f.restart(t)
			block, err = f.propose(t)
			require.NoError(t, err)
			require.ElementsMatch(t, txs, tmtypes.Txs(block.Txs[1:]))
			f.commit(t, block)
			f.assertCommitted(t, txs, 18)
			for _, sender := range f.senders[:2] {
				require.Equal(t, uint64(1), f.app.AccountKeeper.GetAccount(f.app.GetCheckCtx(), sender.AccountAddress).GetSequence())
			}
		})
	}
}

func TestRepair_89_PartialReplicationRestart(t *testing.T) {
	f := newRepair89Fixture(t)
	txs := tmtypes.Txs{f.tx(t, f.senders[0], 3), f.tx(t, f.senders[1], 5), f.tx(t, f.senders[2], 7)}
	f.add(t, txs...)
	// The production sync observer closes the real WAL after its first durable
	// ACCEPT. The next send executes but cannot replicate; it must remain pending.
	f.node.Replicator.Close()
	rep, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: f.node.WAL, Standalone: true, OnWALSync: func(time.Duration) { f.node.WAL.Close() },
	})
	require.NoError(t, err)
	f.node.Replicator = rep
	f.node.Accepter = sequencer.NewAccepter(f.node.Receipter, rep, f.node.ReceiptSource, f.node.Metrics)
	f.node.ConsensusSource, err = sequencer.NewConsensusSource(f.app, f.node.Executor, f.node.ReceiptKey, sequencer.WithConsensusReplicator(rep))
	require.NoError(t, err)
	f.node.ConsensusSource.SetAccepter(f.node.Accepter)
	f.app.SetSequencerSource(f.node.ConsensusSource)
	f.exec.SetSequencerSource(f.node.ConsensusSource)
	block, err := f.propose(t)
	require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
	require.Nil(t, block)
	pending, acked := f.journal(t)
	require.Len(t, pending, 2)
	require.Len(t, acked, 1)
	require.Equal(t, 3, f.pool.Size())
	left, _ := f.pool.ReapTxs(mempool.ReapLimits{}, false)
	require.ElementsMatch(t, txs, left)
	f.restart(t)
	var accepts []*wal.Accept
	require.NoError(t, f.node.WAL.Scan(func(entry wal.Entry) error {
		if accepted, ok := entry.(*wal.Accept); ok {
			accepts = append(accepts, accepted)
		}
		return nil
	}))
	require.Len(t, accepts, 1)
	require.NotContains(t, pending, tmtypes.Tx(accepts[0].Tx))
	block, err = f.propose(t)
	require.NoError(t, err)
	require.Len(t, block.Txs, 4)
	require.ElementsMatch(t, txs, tmtypes.Txs(block.Txs[1:]))
	require.Equal(t, tmtypes.Tx(accepts[0].Tx), block.Txs[1], "the durable prefix retains its sequence position")
	f.commit(t, block)
	f.assertCommitted(t, txs, 15)
	for _, sender := range f.senders {
		require.Equal(t, uint64(1), f.app.AccountKeeper.GetAccount(f.app.GetCheckCtx(), sender.AccountAddress).GetSequence())
	}
}

func TestRepair_89_SealedRetryAndNextHeightRestart(t *testing.T) {
	f := newRepair89Fixture(t)
	first, later := f.tx(t, f.senders[0], 7), f.tx(t, f.senders[1], 11)
	f.add(t, first)
	block, err := f.propose(t)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{first}, tmtypes.Txs(block.Txs[1:]))
	require.Zero(t, f.pool.Size())
	f.add(t, later)
	retry, err := f.propose(t)
	require.NoError(t, err)
	require.Equal(t, block.Hash(), retry.Hash())
	require.Equal(t, block.Txs, retry.Txs)
	require.Equal(t, 1, f.pool.Size())
	pending, _ := f.journal(t)
	require.Equal(t, tmtypes.Txs{later}, pending)
	f.commit(t, block)
	f.restart(t)
	next, err := f.propose(t)
	require.NoError(t, err)
	require.Equal(t, int64(3), next.Height)
	require.Equal(t, tmtypes.Txs{later}, tmtypes.Txs(next.Txs[1:]))
	f.commit(t, next)
	f.assertCommitted(t, tmtypes.Txs{first, later}, 18)
}

func TestRepair_89_IndividualRefusalAndAdmission(t *testing.T) {
	f := newRepair89Fixture(t)
	earlier := f.tx(t, f.senders[0], 2)
	stale := f.tx(t, f.senders[0], 3)
	good := f.tx(t, f.senders[1], 7)
	f.add(t, stale, good)
	_, err := f.node.Accepter.Accept(earlier)
	require.NoError(t, err)
	block, err := f.propose(t)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{earlier, good}, tmtypes.Txs(block.Txs[1:]))
	require.NotContains(t, block.Txs, stale)
	require.Zero(t, f.pool.Size(), "both admitted and individually refused transactions are acknowledged")
	pending, acked := f.journal(t)
	require.Empty(t, pending)
	require.True(t, acked[stale.Hash().String()])
	require.True(t, acked[good.Hash().String()])
	refused, err := f.node.ReceiptSource.SequencerSend(common.Hash(sha256.Sum256(stale)))
	require.NoError(t, err)
	require.Nil(t, refused)
	f.commit(t, block)
	f.assertCommitted(t, tmtypes.Txs{earlier, good}, 9)
}

func TestRepair_89_ReplayedAcceptanceIsNotRefused(t *testing.T) {
	f := newRepair89Fixture(t)
	tx := f.tx(t, f.senders[0], 7)
	original, err := f.node.Accepter.Accept(tx)
	require.NoError(t, err)
	f.restart(t)
	admitted, processed, err := f.node.ConsensusSource.AdmitWithAcknowledgements(t.Context(), 2, tmtypes.Txs{tx})
	require.NoError(t, err)
	require.Equal(t, 1, admitted)
	require.Equal(t, 1, processed)
	receipt, err := f.node.ReceiptSource.SequencerSend(common.Hash(sha256.Sum256(tx)))
	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Equal(t, original.Receipt, receipt.Receipt)
	f.add(t, tx)
	block, err := f.propose(t)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{tx}, tmtypes.Txs(block.Txs[1:]))
	f.commit(t, block)
	f.assertCommitted(t, tmtypes.Txs{tx}, 7)
	require.Equal(t, uint64(1), f.app.AccountKeeper.GetAccount(f.app.GetCheckCtx(), f.senders[0].AccountAddress).GetSequence())
}

func TestRepair_89_CancelledAdmissionRetainsTransactions(t *testing.T) {
	f := newRepair89Fixture(t)
	tx := f.tx(t, f.senders[0], 7)
	f.add(t, tx)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	block, err := f.exec.CreateProposalBlock(ctx, 2, f.state, &tmtypes.Commit{}, f.state.Validators.GetProposer().Address)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, block)
	require.Equal(t, 1, f.pool.Size())
	pending, acked := f.journal(t)
	require.Equal(t, tmtypes.Txs{tx}, pending)
	require.Empty(t, acked)
	f.restart(t)
	block, err = f.propose(t)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{tx}, tmtypes.Txs(block.Txs[1:]))
	f.commit(t, block)
	f.assertCommitted(t, tmtypes.Txs{tx}, 7)
}

func TestRepair_89_PendingSealRetryRetainsTransactions(t *testing.T) {
	f := newRepair89Fixture(t)
	first, later := f.tx(t, f.senders[0], 7), f.tx(t, f.senders[1], 11)
	f.add(t, first)
	pins, err := f.app.SequencerKeeper.ReceiptKeyPins(f.app.GetCheckCtx())
	require.NoError(t, err)
	wrong := append([]verify.SequencerAuthorization(nil), pins...)
	require.NotEmpty(t, wrong)
	wrong[0].SequencerID = sha256.Sum256([]byte("unregistered repair 89 receipt key id"))
	require.NoError(t, f.node.ReceiptKey.SetPins(wrong))
	block, err := f.propose(t)
	require.Error(t, err)
	require.Nil(t, block)
	_, err = f.node.Executor.Snapshot()
	require.ErrorIs(t, err, sequencer.ErrLayerNotOpen, "Seal has closed the executor and retained a pending batch")
	f.add(t, later)
	admitted, processed, err := f.node.ConsensusSource.AdmitWithAcknowledgements(t.Context(), 2, tmtypes.Txs{later})
	require.NoError(t, err)
	require.Zero(t, admitted)
	require.Zero(t, processed)
	require.NoError(t, f.node.ReceiptKey.SetPins(pins))
	block, err = f.propose(t)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{first}, tmtypes.Txs(block.Txs[1:]))
	require.Equal(t, 1, f.pool.Size())
	pending, _ := f.journal(t)
	require.Equal(t, tmtypes.Txs{later}, pending)
	f.commit(t, block)
	f.restart(t)
	next, err := f.propose(t)
	require.NoError(t, err)
	require.Equal(t, tmtypes.Txs{later}, tmtypes.Txs(next.Txs[1:]))
	f.commit(t, next)
	f.assertCommitted(t, tmtypes.Txs{first, later}, 18)
}

func TestRepair_89_AcknowledgementWriteFailureRestart(t *testing.T) {
	f := newRepair89Fixture(t)
	txs := tmtypes.Txs{f.tx(t, f.senders[0], 7), f.tx(t, f.senders[1], 11)}
	f.add(t, txs...)
	f.node.Replicator.Close()
	rep, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: f.node.WAL, Standalone: true, OnWALSync: func(time.Duration) {
			if !f.dbClosed {
				require.NoError(t, f.db.Close())
				f.dbClosed = true
			}
		},
	})
	require.NoError(t, err)
	f.node.Replicator = rep
	f.node.Accepter = sequencer.NewAccepter(f.node.Receipter, rep, f.node.ReceiptSource, f.node.Metrics)
	f.node.ConsensusSource, err = sequencer.NewConsensusSource(f.app, f.node.Executor, f.node.ReceiptKey, sequencer.WithConsensusReplicator(rep))
	require.NoError(t, err)
	f.node.ConsensusSource.SetAccepter(f.node.Accepter)
	f.app.SetSequencerSource(f.node.ConsensusSource)
	f.exec.SetSequencerSource(f.node.ConsensusSource)
	block, err := f.propose(t)
	require.ErrorContains(t, err, "acknowledge proposal admission")
	require.Nil(t, block)
	require.Equal(t, len(txs), f.pool.Size())
	originals := make(map[common.Hash][]byte)
	for _, tx := range txs {
		hash := common.Hash(sha256.Sum256(tx))
		receipt, err := f.node.ReceiptSource.SequencerSend(hash)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		originals[hash] = receipt.Receipt
	}
	f.restart(t)
	pending, acked := f.journal(t)
	require.ElementsMatch(t, txs, pending, "the durable pre-admission journal survives the failed acknowledgement write")
	require.Empty(t, acked)
	block, err = f.propose(t)
	require.NoError(t, err)
	require.ElementsMatch(t, txs, tmtypes.Txs(block.Txs[1:]))
	require.Len(t, block.Txs, 3, "replaying an unacknowledged durable admission cannot duplicate execution")
	for hash, original := range originals {
		receipt, err := f.node.ReceiptSource.SequencerSend(hash)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, original, receipt.Receipt)
	}
	f.commit(t, block)
	f.assertCommitted(t, txs, 18)
	for _, sender := range f.senders[:2] {
		require.Equal(t, uint64(1), f.app.AccountKeeper.GetAccount(f.app.GetCheckCtx(), sender.AccountAddress).GetSequence())
	}
}
