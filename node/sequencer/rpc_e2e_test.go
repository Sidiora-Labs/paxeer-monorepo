package sequencer_test

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type rpcE2EHarness struct {
	ctx       context.Context
	node      *sequencer.Node
	standby   *sequencer.Standby
	backup    *app.App
	client    *gethrpc.Client
	consensus *local.Local
	url       string
	sender    utils.TestAcct
	genesis   time.Time
}

func rpcE2EPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func newRPCE2EApp(t *testing.T, sender utils.TestAcct, genesisTime time.Time, port int) *app.App {
	t.Helper()
	opts := viper.New()
	opts.Set("chain-id", testChainID)
	opts.Set(app.FlagSCEnable, true)
	opts.Set(app.FlagSSEnable, true)
	opts.Set(app.FlagSSBackend, "pebbledb")
	opts.Set("evm.http_enabled", port != 0)
	opts.Set("evm.ws_enabled", false)
	opts.Set("evm.http_address", "127.0.0.1")
	opts.Set("evm.http_port", port)
	opts.Set("evm.worker_pool_size", 1)
	opts.Set("evm.worker_queue_size", 8)
	opts.Set("evm.filter_timeout", time.Minute)
	config := tmconfig.TestConfig()
	config.AutobahnConfigFile = "sequencer-rpc-e2e"
	encoding := app.MakeEncodingConfig()
	a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, false,
		config, encoding, wasm.EnableAllProposals, opts, app.EmptyWasmOpts, nil,
		func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(false)
			ba.SetConcurrencyWorkers(1)
		})
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
	require.NoError(t, err)
	_, err = a.InitChain(t.Context(), &abci.RequestInitChain{
		Time: genesisTime, ChainId: testChainID, ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx := a.NewContext(false, tmproto.Header{Height: committedTip, ChainID: testChainID, Time: genesisTime})
	a.EvmKeeper.SetAddressMapping(ctx, sender.AccountAddress, sender.EvmAddress)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", fundingUhpx))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender.AccountAddress, coins))
	require.NoError(t, a.SequencerKeeper.InitGenesis(ctx, seqtypes.GenesisState{
		Leaders: [][32]byte{[32]byte(label("rpc e2e leader"))},
		ReceiptKeys: []seqtypes.ReceiptKey{{
			KeyId: testKeyID, PublicKey: publicOf(testKeySeed), FirstHeight: 1, LastHeight: 1000,
		}},
		LastSeq: sourceLastSeq, LastSeqHash: sourcePrevSeqHash,
	}))
	resolutionMerged(ctx, a)
	_, err = a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{Height: committedTip, ChainID: testChainID, Time: genesisTime},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(t.Context())
	require.NoError(t, err)
	require.Equal(t, committedTip, a.LastBlockHeight())
	return a
}

func rpcE2EClient(t *testing.T, ctx context.Context, a *app.App, consensus *local.Local, port int) (*gethrpc.Client, string) {
	t.Helper()
	a.RegisterLocalServices(consensus, a.GetTxConfig())
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	c, err := gethrpc.DialHTTP(endpoint)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	require.Eventually(t, func() bool {
		var version string
		return c.CallContext(ctx, &version, "web3_clientVersion") == nil
	}, 5*time.Second, 10*time.Millisecond)
	return c, endpoint
}

func newRPCE2EHarness(t *testing.T) *rpcE2EHarness {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	sender := utils.NewSigner()
	genesis := time.Now().Add(-time.Second).UTC()
	port := rpcE2EPort(t)
	a := newRPCE2EApp(t, sender, genesis, port)
	backup := newRPCE2EApp(t, sender, genesis, 0)
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensusFiles := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensusFiles.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensusFiles.KeyFile(), consensusFiles.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	l := listen(t)
	t.Cleanup(func() { _ = l.Close() })
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.WALDir = filepath.Join(t.TempDir(), "primary-wal")
	cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt-key", testKeySeed, 0o600)
	cfg.StandbyAddress = l.Addr().String()
	cfg.ReplicationTimeout = replicationTimeout
	cfg.LeaseDuration = 3 * replicationTimeout
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 4, 8
	n, err := sequencer.NewNode(cfg, a)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	exec, err := sequencer.NewExecutor(backup, sequencer.Options{
		ChainID: testChainID, ProposerAddress: pv.Key.PubKey.Address(),
		LastSeq: sourceLastSeq, LastSeqHash: sourcePrevSeqHash,
	})
	require.NoError(t, err)
	floor, err := sequencer.NewFeeFloor(exec, backup, "", backup.ParamsKeeper, backup.OracleKeeper, backup.PricefeedKeeper, &backup.EvmKeeper)
	require.NoError(t, err)
	backup.SetOpenBlock(floor)
	r, err := sequencer.NewReceipter(floor, backup, &backup.EvmKeeper, n.ReceiptKey, app.EVMReceiptModuleID)
	require.NoError(t, err)
	w, err := wal.OpenWAL(filepath.Join(t.TempDir(), "standby-wal"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	standbyDir := t.TempDir()
	standbyPV, err := privval.GenFilePV(filepath.Join(standbyDir, "key.json"), filepath.Join(standbyDir, "state.json"), "")
	require.NoError(t, err)
	_, standbyKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	public := n.ReceiptKey.PublicKey()
	standbyCtx, stopStandby := context.WithCancel(ctx)
	sb, err := sequencer.NewStandby(standbyCtx, sequencer.StandbyConfig{
		WAL: w, Executor: sequencer.NewAppReplayer(floor, r),
		ConsensusKey: standbyPV, ReceiptKey: standbyKey,
		PrimaryConsensusKey: pv.Key.PubKey, PrimaryReceiptKey: public[:],
		CommittedHeight: uint64(committedTip), LastSeq: sourceLastSeq,
	})
	require.NoError(t, err)
	serveDone := make(chan error, 1)
	go func() { serveDone <- sb.Serve(l) }()
	t.Cleanup(func() {
		require.NoError(t, n.Stop())
		stopStandby()
		_ = l.Close()
		select {
		case <-serveDone:
		case <-time.After(5 * time.Second):
			t.Error("standby did not stop")
		}
	})
	require.NoError(t, n.Start(ctx))
	consensusConfig, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, consensusApp.Close()) })
	service, closeConsensus, err := rpctest.StartTendermint(ctx, consensusConfig, consensusApp, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closeConsensus(context.Background())) })
	consensusNode, ok := service.(local.NodeService)
	require.True(t, ok)
	consensus, err := local.New(consensusNode)
	require.NoError(t, err)
	client, endpoint := rpcE2EClient(t, ctx, a, consensus, port)
	return &rpcE2EHarness{
		ctx: ctx, node: n, standby: sb, backup: backup, client: client, consensus: consensus,
		url: endpoint, sender: sender, genesis: genesis,
	}
}

func (h *rpcE2EHarness) send(t *testing.T, nonce uint64, to common.Address, value *big.Int) (*ethtypes.Transaction, *evmrpc.SequencerReceiptResult) {
	t.Helper()
	tx, _ := transfer(t, h.sender, nonce, to, value)
	raw, err := tx.MarshalBinary()
	require.NoError(t, err)
	var hash common.Hash
	require.NoError(t, h.client.CallContext(h.ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
	require.Equal(t, tx.Hash(), hash)
	var receipt *evmrpc.SequencerReceiptResult
	require.NoError(t, h.client.CallContext(h.ctx, &receipt, evmrpc.SequencerReceiptMethod, hash))
	require.NotNil(t, receipt)
	require.Equal(t, evmrpc.RungReceipted, receipt.Rung)
	require.NotNil(t, receipt.SequencerReceiptView)
	require.Equal(t, hash, receipt.TxHash)
	require.Equal(t, nonce, uint64(receipt.Nonce))
	require.Equal(t, sourceLastSeq+nonce+1, uint64(receipt.Seq))
	require.Equal(t, uint64(openHeight), uint64(receipt.Height))
	require.Equal(t, uint64(1), uint64(receipt.Status))
	require.Equal(t, h.sender.EvmAddress, receipt.From)
	require.Equal(t, to, receipt.To)
	require.Equal(t, uint64(receipt.Seq), h.standby.LastSeq())
	require.NoError(t, h.standby.Err())
	return tx, receipt
}

func rpcE2EVerifyReceipt(t *testing.T, a *app.App, receipt *evmrpc.SequencerReceiptResult) {
	t.Helper()
	pins, err := a.SequencerKeeper.ReceiptKeys(a.GetCheckCtx())
	require.NoError(t, err)
	keys := make(seqkeeper.ReceiptKeys, len(pins))
	for i, pin := range pins {
		keys[i] = seqkeeper.ReceiptKeyEntry{
			KeyId: pin.KeyId[:], PublicKey: pin.PublicKey[:], FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight,
		}
	}
	pin, err := keys.KeyForHeight([32]byte(receipt.KeyID), uint64(receipt.Height))
	require.NoError(t, err)
	require.Equal(t, pin.PublicKey, []byte(receipt.SigningKey))
	require.NoError(t, receipt.Verify(keys))
	verified, err := verify.ReceiptSignature(receipt.Receipt, [32]byte(pin.PublicKey))
	require.NoError(t, err)
	require.Equal(t, [32]byte(receipt.TxHash), verified.Receipt.ActivityID)
	require.Equal(t, uint64(receipt.Seq), verified.Receipt.GlobalSequence)
	require.Equal(t, app.EVMReceiptModuleID, verified.Receipt.ModuleID)
}

func TestSequencerRPCSendReturnsSignedReceipt(t *testing.T) {
	h := newRPCE2EHarness(t)
	_, receipt := h.send(t, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
	rpcE2EVerifyReceipt(t, h.node.App, receipt)
	require.Equal(t, common.Hash(testKeyID), receipt.KeyID)
	require.Equal(t, publicOf(testKeySeed), h.node.ReceiptKey.PublicKey())
	require.Equal(t, committedTip, h.node.App.LastBlockHeight())
	require.Nil(t, receipt.BlockNumber)
	require.Nil(t, receipt.BlockHash)
}

func TestSequencerRPCPendingReadsOpenBlock(t *testing.T) {
	h := newRPCE2EHarness(t)
	a := h.node.App
	to := utils.NewSigner().EvmAddress
	before, err := a.PendingSnapshot()
	require.NoError(t, err)
	readBalance := func(ctx sdk.Context) *big.Int {
		return a.EvmKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, to))
	}
	requireWei(t, big.NewInt(0), readBalance(before))
	var initial hexutil.Big
	require.NoError(t, h.client.CallContext(h.ctx, &initial, "eth_getBalance", to, "pending"))
	requireWei(t, big.NewInt(0), (*big.Int)(&initial))
	total := new(big.Int)
	for nonce, value := range []*big.Int{big.NewInt(onePaxInWei), big.NewInt(7)} {
		_, receipt := h.send(t, uint64(nonce), to, value)
		rpcE2EVerifyReceipt(t, a, receipt)
		total.Add(total, value)
		pending, err := a.PendingSnapshot()
		require.NoError(t, err)
		require.Equal(t, openHeight, pending.BlockHeight())
		require.Equal(t, uint64(nonce+1), a.EvmKeeper.GetNonce(pending, h.sender.EvmAddress))
		requireWei(t, total, readBalance(pending))
		var balance hexutil.Big
		require.NoError(t, h.client.CallContext(h.ctx, &balance, "eth_getBalance", to, "pending"))
		requireWei(t, total, (*big.Int)(&balance))
		require.Equal(t, committedTip, a.LastBlockHeight())
		requireWei(t, big.NewInt(0), readBalance(a.GetCheckCtx()))
		require.Zero(t, a.EvmKeeper.GetNonce(a.GetCheckCtx(), h.sender.EvmAddress))
	}
	backup, err := h.backup.PendingSnapshot()
	require.NoError(t, err)
	requireWei(t, total, h.backup.EvmKeeper.GetBalance(backup, h.backup.EvmKeeper.GetPaxAddressOrDefault(backup, to)))
}

func TestForwardedReceiptOnFollowerNode(t *testing.T) {
	h := newRPCE2EHarness(t)
	port := rpcE2EPort(t)
	follower := newRPCE2EApp(t, h.sender, h.genesis, port)
	cfg := sequencer.DefaultConfig()
	cfg.ReceiptForwardURL = h.url
	option, ok := sequencer.NewFollowerReceiptOption(cfg)
	require.True(t, ok)
	follower.SetSequencerRPC(evmrpc.SequencerOption{}, option)
	client, _ := rpcE2EClient(t, h.ctx, follower, h.consensus, port)
	require.Nil(t, follower.SequencerSource())
	_, err := follower.PendingSnapshot()
	require.ErrorIs(t, err, app.ErrPendingServedBySequencer)
	for nonce := range uint64(2) {
		tx, receipt := h.send(t, nonce, utils.NewSigner().EvmAddress, big.NewInt(int64(nonce+1)))
		var forwarded *evmrpc.SequencerReceiptResult
		require.NoError(t, client.CallContext(h.ctx, &forwarded, evmrpc.SequencerReceiptMethod, tx.Hash()))
		require.NotNil(t, forwarded)
		require.Equal(t, receipt, forwarded)
		require.Equal(t, []byte(receipt.Receipt), []byte(forwarded.Receipt))
		rpcE2EVerifyReceipt(t, follower, forwarded)
		require.Equal(t, committedTip, follower.LastBlockHeight())
		require.Zero(t, follower.EvmKeeper.GetNonce(follower.GetCheckCtx(), h.sender.EvmAddress))
	}
	var unknown *evmrpc.SequencerReceiptResult
	require.NoError(t, client.CallContext(h.ctx, &unknown, evmrpc.SequencerReceiptMethod, common.HexToHash("0xdeadbeef")))
	require.Nil(t, unknown)
}
