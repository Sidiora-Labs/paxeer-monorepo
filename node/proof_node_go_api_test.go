// Proves: App.SetOpenBlock, App.Sequenced, App.SetSequencerSource, App.SequencerSource, App.SetCommitListener, App.SetSequencerRPC and PrecompileKeepers.LayerXAuthK/LayerXLedgerK/AnchorK -> exported Go interfaces and App.RegisterLocalServices -> sequencer.Node, ConsensusSource, nonce reads, real precompiles and RPC clients.
//
// The proof follows node/sequencer/resolution_test.go's read-context and batch
// consumers, using Node.Start to install the real producers. All services and
// stores are owned by this process. Committed application state is closed and
// reopened before consumption; open state is read repeatedly across new sends.
// Repairs node/app.go:App.DomainAllowance, which refused the configured root
// EVM chain ID unless it was 125, preventing real sequencer RPC admission.
// Repairs node/sequencer/resolution_test.go:resolutionMerged and
// TestResolutionActivationHeight, whose harnesses still used only the upgrade
// marker instead of initializing the persisted SequencerKeeper activation height.
// Malformed producer bytes are refused by DecodeSequenceBatch and receipt Verify.
// rpc/server.go serves pax_sequencerReceipt over HTTP and WebSocket; the proof
// asserts the same unknown-hash response on both transports.
package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestProofNodeGoApi(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	oldOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = oldOCC })
	home := t.TempDir()
	// Reserve both ports together, releasing them only when the real server binds.
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer httpListener.Close()
	wsListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer wsListener.Close()
	httpPort := httpListener.Addr().(*net.TCPAddr).Port
	wsPort := wsListener.Addr().(*net.TCPAddr).Port
	opts := viper.New()
	opts.Set("chain-id", "pax-test")
	opts.Set(app.FlagSCEnable, true)
	opts.Set(app.FlagSCAsyncCommitBuffer, 0)
	opts.Set(app.FlagSCSnapshotInterval, 0)
	opts.Set(app.FlagSSEnable, false)
	opts.Set("evm.http_enabled", true)
	opts.Set("evm.ws_enabled", true)
	opts.Set("evm.http_address", "127.0.0.1")
	opts.Set("evm.http_port", httpPort)
	opts.Set("evm.ws_port", wsPort)
	opts.Set("evm.worker_pool_size", 1)
	opts.Set("evm.worker_queue_size", 8)
	opts.Set("evm.filter_timeout", time.Minute)
	encoding := app.MakeEncodingConfig()
	open := func() *app.App {
		t.Helper()
		db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, filepath.Join(home, "data"))
		require.NoError(t, err)
		config := tmconfig.DefaultConfig().SetRoot(home)
		config.AutobahnConfigFile = "proof-node-go-api"
		return app.New(db, nil, true, map[int64]bool{}, home, 1, true,
			config, encoding, nil, opts, app.EmptyWasmOpts, nil,
			func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	}
	a := open()
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	at := time.Now().Add(-time.Second).UTC()
	genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
	require.NoError(t, err)
	_, err = a.InitChain(ctx, &abci.RequestInitChain{Time: at, ChainId: "pax-test", ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis})
	require.NoError(t, err)
	producerCtx := a.NewContext(false, tmproto.Header{ChainID: "pax-test", Height: 1, Time: at})
	sender := utils.NewSigner()
	a.EvmKeeper.SetAddressMapping(producerCtx, sender.AccountAddress, sender.EvmAddress)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(producerCtx, minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(producerCtx, minttypes.ModuleName, sender.AccountAddress, coins))
	seed := sha256.Sum256([]byte("proof node go api receipt key"))
	public := [32]byte(ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey))
	keyID := sha256.Sum256(public[:])
	require.NoError(t, a.SequencerKeeper.InitGenesis(producerCtx, seqtypes.GenesisState{
		Leaders:          [][32]byte{sha256.Sum256([]byte("proof leader"))},
		ActivationHeight: 2,
		ReceiptKeys:      []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: 1000}},
	}))
	cp, bp := *app.DefaultConsensusParams, *app.DefaultConsensusParams.Block
	bp.MaxGasWanted = bp.MaxGas
	cp.Block = &bp
	a.StoreConsensusParams(producerCtx, &cp)
	a.UpgradeKeeper.SetDone(producerCtx, app.MergeUpgrade)
	_, err = a.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(ctx)
	require.NoError(t, err)
	committedHash := bytes.Clone(a.LastCommitID().Hash)
	require.NoError(t, a.Close())
	a = nil
	a = open()
	require.Equal(t, int64(1), a.LastBlockHeight())
	require.Equal(t, committedHash, a.LastCommitID().Hash)
	require.False(t, a.Sequenced(a.GetCheckCtx(), 1))
	require.True(t, a.Sequenced(a.GetCheckCtx(), 2))
	_, err = a.PendingSnapshot()
	require.ErrorIs(t, err, app.ErrPendingServedBySequencer)
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensusFiles := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensusFiles.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensusFiles.KeyFile(), consensusFiles.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	keyPath := filepath.Join(t.TempDir(), "receipt-key")
	require.NoError(t, os.WriteFile(keyPath, []byte(hex.EncodeToString(seed[:])), 0o600))
	config := sequencer.DefaultConfig()
	config.Enable = true
	config.StandbyAddress = ""
	config.NonceQueueMaxPerSender = 4
	config.NonceQueueMaxTotal = 8
	config.LeaseDuration = 3 * time.Second
	config.WALDir = filepath.Join(t.TempDir(), "wal")
	config.ReceiptKeyFile = keyPath
	n, err := sequencer.NewNode(config, a)
	require.NoError(t, err, "node/sequencer.NewNode must host the producer")
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	require.NoError(t, n.Start(ctx))
	require.Same(t, n.ConsensusSource, a.SequencerSource())
	require.False(t, a.SequencerSource().Sequenced(1))
	require.True(t, a.SequencerSource().Sequenced(2))
	for range 2 {
		id, gas, size, active, known := a.DomainAllowance(a.LatestCtx(), a.EvmKeeper.ChainID(a.LatestCtx()))
		require.Zero(t, id)
		require.Zero(t, gas)
		require.Zero(t, size)
		require.True(t, active, "node/app.go:App.DomainAllowance must admit the configured root chain")
		require.True(t, known)
		_, _, _, active, known = a.DomainAllowance(a.LatestCtx(), big.NewInt(125000999))
		require.False(t, active)
		require.False(t, known)
	}

	consensusConfig, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, consensusApp.Close()) })
	service, closeService, err := rpctest.StartTendermint(ctx, consensusConfig, consensusApp, rpctest.SuppressStdout)
	require.NoError(t, err, "consensus/rpc/test.StartTendermint must host the local RPC consumer")
	t.Cleanup(func() { require.NoError(t, closeService(context.Background())) })
	nodeService, ok := service.(local.NodeService)
	require.True(t, ok)
	localClient, err := local.New(nodeService)
	require.NoError(t, err)
	require.NoError(t, httpListener.Close())
	require.NoError(t, wsListener.Close())
	a.RegisterLocalServices(localClient, a.GetTxConfig())
	a.SendEVMServerStartSignals()
	httpClient, err := gethrpc.DialHTTP(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	require.NoError(t, err)
	defer httpClient.Close()
	require.Eventually(t, func() bool {
		var version string
		return httpClient.CallContext(ctx, &version, "web3_clientVersion") == nil
	}, 5*time.Second, 10*time.Millisecond)
	var wsClient *gethrpc.Client
	require.Eventually(t, func() bool {
		wsClient, err = gethrpc.DialContext(ctx, fmt.Sprintf("ws://127.0.0.1:%d", wsPort))
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	defer wsClient.Close()
	var absent *evmrpc.SequencerReceiptResult
	require.NoError(t, httpClient.CallContext(ctx, &absent, evmrpc.SequencerReceiptMethod, common.Hash{0xff}))
	require.Nil(t, absent)
	require.NoError(t, wsClient.CallContext(ctx, &absent, evmrpc.SequencerReceiptMethod, common.Hash{0xff}))
	require.Nil(t, absent)
	var rpcErr gethrpc.Error
	require.ErrorAs(t, wsClient.CallContext(ctx, &absent, "pax_unregisteredProofMethod"), &rpcErr)
	require.Equal(t, -32601, rpcErr.ErrorCode())
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 1000}}
	recipient := utils.NewSigner().EvmAddress
	for nonce, transport := range []*gethrpc.Client{httpClient, wsClient} {
		tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: uint64(nonce), To: &recipient, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(500_000_000_000)}), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(a.LatestCtx())), sender.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := tx.MarshalBinary()
		require.NoError(t, err)
		var hash common.Hash
		require.NoError(t, transport.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
		require.Equal(t, tx.Hash(), hash)
		for range 2 {
			// This is resolutionOpenNonces.EvmNonce's real ante read path.
			require.Equal(t, uint64(nonce+1), a.EvmKeeper.GetNonce(a.LatestCtx(), sender.EvmAddress))
			require.Zero(t, a.EvmKeeper.GetNonce(a.GetCheckCtx(), sender.EvmAddress))
			var pending hexutil.Uint64
			require.NoError(t, transport.CallContext(ctx, &pending, "eth_getTransactionCount", sender.EvmAddress, "pending"))
			require.Equal(t, uint64(nonce+1), uint64(pending))
			var receipt *evmrpc.SequencerReceiptResult
			require.NoError(t, transport.CallContext(ctx, &receipt, evmrpc.SequencerReceiptMethod, hash))
			require.NotNil(t, receipt)
			require.Equal(t, evmrpc.RungReceipted, receipt.Rung)
			require.Equal(t, uint64(nonce+1), uint64(receipt.Seq))
			require.NoError(t, receipt.Verify(pins))
			wrongPublic := sha256.Sum256([]byte("untrusted receipt signer"))
			require.Error(t, receipt.Verify(seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: wrongPublic[:], FirstHeight: 1, LastHeight: 1000}}))
			// Corrupt a real producer result at the consumer's own decoder.
			receipt.Receipt[0] ^= 0xff
			require.Error(t, receipt.Verify(pins))
		}
		require.Error(t, transport.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes{0xff}))
		require.Equal(t, uint64(nonce+1), a.EvmKeeper.GetNonce(a.LatestCtx(), sender.EvmAddress))
	}
	sealed, err := a.SequencerSource().Seal(ctx, 2)
	require.NoError(t, err)
	require.Len(t, sealed.Txs, 2)
	for range 2 {
		batch, err := app.DecodeSequenceBatch(sealed.Batch)
		require.NoError(t, err)
		require.Equal(t, uint64(1), batch.FromSeq)
		require.Equal(t, uint64(2), batch.ToSeq)
		require.NoError(t, batch.VerifySignature("pax-test", public))
	}
	_, err = app.DecodeSequenceBatch(sealed.Batch[:len(sealed.Batch)-1])
	require.Error(t, err)
	layer := n.ConsensusSource.Sealed()
	require.NotNil(t, layer)
	require.NoError(t, a.SetSealedStateToCommit(layer.Context(), layer.Context().MultiStore().(sdk.CacheMultiStore)))
	_, err = a.Commit(ctx)
	require.NoError(t, err, "App.SetCommitListener must deliver to the real Driver.OnCommit")
	require.Equal(t, uint64(2), n.WAL.LastCommitted().Height)
	require.Equal(t, [32]byte(a.LastCommitID().Hash), n.WAL.LastCommitted().AppHash)
	next, err := n.Executor.Snapshot()
	require.NoError(t, err)
	require.Equal(t, int64(3), next.BlockHeight(), "commit consumer opens the next block")
}

func TestProofNodeGoApiKeepers(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockHeight(100).WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	keepers := a.GetPrecompileKeepers().(*app.PrecompileKeepers)
	require.Same(t, a.LayerXAuthKeeper, keepers.LayerXAuthK())
	require.Same(t, a.LayerXLedgerKeeper, keepers.LayerXLedgerK())
	require.NotNil(t, keepers.AnchorK())
	auth, err := layerxauth.NewPrecompile(keepers)
	require.NoError(t, err)
	ledger, err := layerxledger.NewPrecompile(keepers)
	require.NoError(t, err)
	caller := common.Address{0x21, 0x11, 1}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, 32))
	public := [32]byte(private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
	require.NoError(t, err)
	_, err = a.EvmKeeper.BindLayerX(ctx, caller, public, [64]byte(ed25519.Sign(private, message)))
	require.NoError(t, err)
	actor, err := keepers.LayerXAuthK().Actor(ctx, caller)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &a.EvmKeeper, true)
	ctx = db.Ctx()
	evm := &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: caller}}
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), public[:]...)
	input, err := auth.GetABI().Pack("recordLifecycle", registration, [32]byte{0x51})
	require.NoError(t, err)
	output, err := auth.Run(evm, caller, auth.Address(), input, nil, false, false, nil)
	require.NoError(t, err, "%x", output)
	values, err := auth.GetABI().Methods["recordLifecycle"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.NotZero(t, values[0].(uint64))
	for range 2 {
		identity, err := a.LayerXHost.ReadIdentityState(ctx, actor.DID)
		require.NoError(t, err)
		require.Equal(t, public, identity.PrimaryKey)
		require.Equal(t, uint64(1), identity.NextSequence)
	}
	salt := [32]byte{0x41}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...), salt[:]...))
	input, err = ledger.GetABI().Pack("register", asset, salt, "PROOF", "Proof asset", uint8(6), big.NewInt(1000), uint8(ledgertypes.AssetIssuerNative), []byte{})
	require.NoError(t, err)
	output, err = ledger.Run(evm, caller, ledger.Address(), input, nil, false, false, nil)
	require.NoError(t, err, "%x", output)
	require.Equal(t, asset[:], output)
	for range 2 {
		input, err = ledger.GetABI().Pack("getAsset", asset)
		require.NoError(t, err)
		output, err = ledger.Run(evm, caller, ledger.Address(), input, nil, true, false, nil)
		require.NoError(t, err, "%x", output)
		values, err = ledger.GetABI().Methods["getAsset"].Outputs.Unpack(output)
		require.NoError(t, err)
		require.Equal(t, asset, values[0])
		require.Equal(t, "PROOF", values[1])
	}
	_, err = ledgerkeeper.AccountID(public, asset)
	require.NoError(t, err)
	for _, contract := range []struct {
		input []byte
		run   func([]byte) ([]byte, error)
	}{
		{auth.GetABI().Methods["getGrant"].ID, func(input []byte) ([]byte, error) {
			return auth.Run(evm, caller, auth.Address(), input, nil, true, false, nil)
		}},
		{ledger.GetABI().Methods["getAsset"].ID, func(input []byte) ([]byte, error) {
			return ledger.Run(evm, caller, ledger.Address(), input, nil, true, false, nil)
		}},
	} {
		_, err = contract.run(contract.input)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
	}
	input, err = auth.GetABI().Pack("getGrant", [32]byte{0xff})
	require.NoError(t, err)
	_, err = auth.Run(evm, caller, auth.Address(), input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
}
