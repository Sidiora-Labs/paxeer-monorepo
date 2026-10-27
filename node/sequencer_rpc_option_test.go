package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"go.opentelemetry.io/otel"
)

func mergeRPCPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func mergeRPCApp(t *testing.T, sender utils.TestAcct, now time.Time, pins seqkeeper.ReceiptKeys, httpPort, wsPort int) *app.App {
	t.Helper()
	options := viper.New()
	options.Set("chain-id", "pax-test")
	options.Set(app.FlagSCEnable, true)
	options.Set(app.FlagSSEnable, true)
	options.Set(app.FlagSSBackend, "pebbledb")
	options.Set("evm.http_enabled", httpPort != 0)
	options.Set("evm.ws_enabled", wsPort != 0)
	options.Set("evm.http_address", "127.0.0.1")
	options.Set("evm.http_port", httpPort)
	options.Set("evm.ws_port", wsPort)
	options.Set("evm.worker_pool_size", 1)
	options.Set("evm.worker_queue_size", 8)
	options.Set("evm.filter_timeout", time.Minute)
	config := tmconfig.TestConfig()
	config.AutobahnConfigFile = "sequencer-rpc-wiring"
	encoding := app.MakeEncodingConfig()
	a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, false, config, encoding,
		wasm.EnableAllProposals, options, app.EmptyWasmOpts, nil)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
	require.NoError(t, err)
	params := *app.DefaultConsensusParams
	blockParams := *params.Block
	blockParams.MaxGasWanted = 2 * 21_000
	params.Block = &blockParams
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: now, ChainId: "pax-test", ConsensusParams: &params, AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx := a.NewContext(false, tmproto.Header{Height: 1, ChainID: "pax-test", Time: now})
	a.EvmKeeper.SetAddressMapping(ctx, sender.AccountAddress, sender.EvmAddress)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender.AccountAddress, coins))
	require.NoError(t, a.SequencerKeeper.SetReceiptKeys(ctx, pins))
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{Height: 1, ChainID: "pax-test", Time: now}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	return a
}

func TestEVMServerServesSequencerReceiptOption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sender := utils.NewSigner()
	now := time.Unix(1_800_000_000, 0).UTC()
	keyDir := t.TempDir()
	primaryPV, err := privval.GenFilePV(filepath.Join(keyDir, "primary.json"), filepath.Join(keyDir, "primary-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, primaryPV.Save())
	standbyPV, err := privval.GenFilePV(filepath.Join(keyDir, "standby.json"), filepath.Join(keyDir, "standby-state.json"), "")
	require.NoError(t, err)
	seed := bytes.Repeat([]byte{0x51}, ed25519.SeedSize)
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	keyID := [32]byte{0: 0x52}
	keyFile := filepath.Join(keyDir, "receipt-key")
	require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seed)), 0o600))
	key, err := sequencer.LoadReceiptKey(keyFile, filepath.Join(keyDir, "primary.json"), []verify.SequencerAuthorization{{
		SequencerID: keyID, PublicKey: [32]byte(public), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, 2)
	require.NoError(t, err)
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: public, FirstHeight: 1, LastHeight: 1000}}
	httpPort, wsPort := mergeRPCPort(t), mergeRPCPort(t)
	a := mergeRPCApp(t, sender, now, pins, httpPort, wsPort)
	backup := mergeRPCApp(t, sender, now, pins, 0, 0)
	primaryPub, err := primaryPV.GetPubKey(ctx)
	require.NoError(t, err)
	executors := make([]sequencer.Executor, 2)
	receipters := make([]*sequencer.Receipter, 2)
	for i, host := range []*app.App{a, backup} {
		executors[i], err = sequencer.NewExecutor(host, sequencer.Options{ChainID: "pax-test", ProposerAddress: primaryPub.Address()})
		require.NoError(t, err)
		receipters[i], err = sequencer.NewReceipter(executors[i], host, &host.EvmKeeper, key, app.EVMReceiptModuleID)
		require.NoError(t, err)
	}
	primaryWAL, err := wal.OpenWAL(filepath.Join(t.TempDir(), "primary"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(primaryWAL.Close)
	standbyWAL, err := wal.OpenWAL(filepath.Join(t.TempDir(), "standby"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(standbyWAL.Close)
	_, standbySigningKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	standby, err := sequencer.NewStandby(ctx, sequencer.StandbyConfig{
		WAL: standbyWAL, Executor: sequencer.NewAppReplayer(executors[1], receipters[1]),
		ConsensusKey: standbyPV, ReceiptKey: standbySigningKey,
		PrimaryConsensusKey: primaryPub, PrimaryReceiptKey: public, CommittedHeight: 1,
	})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveDone := make(chan error, 1)
	go func() { serveDone <- standby.Serve(listener) }()
	t.Cleanup(func() { _ = listener.Close(); <-serveDone })
	replicator, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: primaryWAL, Dial: sequencer.DialStandby(listener.Addr().String(), 5*time.Second), ReplicationTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(replicator.Close)
	openMs := now.Add(time.Second).UnixMilli()
	require.NoError(t, executors[0].Open(2, openMs))
	_, err = replicator.Release(ctx, &wal.Open{Height: 2, OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)})
	require.NoError(t, err)
	source := sequencer.NewReceiptSource(8)
	metrics, err := sequencer.NewMetrics(otel.Meter("node-merge-rpc-test"))
	require.NoError(t, err)
	accepter := sequencer.NewAccepter(receipters[0], replicator, source, metrics)
	a.SetOpenBlock(executors[0])
	a.SetSequencerRPC(evmrpc.NewSequencerOption(accepter, evmrpc.NewReceiptLogFeed(1)), evmrpc.SequencerReceiptOption{Source: source, Keeper: &a.SequencerKeeper})
	consensusConfig, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, consensusApp.Close()) })
	consensusService, closeConsensus, err := rpctest.StartTendermint(ctx, consensusConfig, consensusApp, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closeConsensus(context.Background())) })
	consensusNode, ok := consensusService.(local.NodeService)
	require.True(t, ok)
	consensusClient, err := local.New(consensusNode)
	require.NoError(t, err)
	a.RegisterLocalServices(consensusClient, a.GetTxConfig())
	httpClient, err := gethrpc.DialHTTP(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	require.Eventually(t, func() bool {
		var response string
		return httpClient.CallContext(ctx, &response, "web3_clientVersion") == nil
	}, 5*time.Second, 10*time.Millisecond)
	wsClient, err := gethrpc.DialWebsocket(ctx, fmt.Sprintf("ws://127.0.0.1:%d", wsPort), "http://localhost")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)
	pendingHashes := make(chan common.Hash, 2)
	subscription, err := wsClient.EthSubscribe(ctx, pendingHashes, "newPendingTransactions")
	require.NoError(t, err)
	defer subscription.Unsubscribe()
	to := utils.NewSigner().EvmAddress
	for nonce, rpcClient := range []*gethrpc.Client{httpClient, wsClient} {
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: uint64(nonce), GasPrice: big.NewInt(500_000_000_000), Gas: 21_000, To: &to, Value: big.NewInt(1),
		}), ethtypes.LatestSignerForChainID(big.NewInt(evmconfig.DefaultChainID)), sender.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		var hash common.Hash
		require.NoError(t, rpcClient.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
		require.Equal(t, signed.Hash(), hash)
		var receipt evmrpc.SequencerReceiptResult
		require.NoError(t, httpClient.CallContext(ctx, &receipt, evmrpc.SequencerReceiptMethod, hash))
		require.Equal(t, evmrpc.RungReceipted, receipt.Rung)
		require.Equal(t, hash, receipt.TxHash)
		require.Equal(t, hexutil.Uint64(nonce), receipt.Nonce)
		require.NoError(t, receipt.Verify(pins))
		select {
		case pending := <-pendingHashes:
			require.Equal(t, hash, pending)
		case <-ctx.Done():
			t.Fatal("WebSocket did not receive the receipted transaction hash")
		}
		for _, client := range []*gethrpc.Client{httpClient, wsClient} {
			var balance hexutil.Big
			require.NoError(t, client.CallContext(ctx, &balance, "eth_getBalance", to, "pending"))
			require.Equal(t, big.NewInt(int64(nonce+1)), (*big.Int)(&balance))
		}
	}
}
