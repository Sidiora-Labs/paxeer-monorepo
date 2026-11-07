package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	protoio "github.com/gogo/protobuf/io"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM1092Runtime struct {
	node    *node
	payer   *ecdsa.PrivateKey
	pins    seqkeeper.ReceiptKeys
	primary *sequencer.Node
	app     *app.App
	backup  *app.App
}

func repairM1092Recorder(t *testing.T, programs bool) *repairM1092Runtime {
	t.Helper()
	return repairM1092PrepareRecorder(t, programs)()
}

func repairM1092PrepareRecorder(t *testing.T, programs bool, configure ...func(sdk.Context, *app.App, *ecdsa.PrivateKey)) func() *repairM1092Runtime {
	t.Helper()
	oldOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = oldOCC })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	now := time.Now().UTC()
	pvDir := t.TempDir()
	primaryPV, err := privval.GenFilePV(filepath.Join(pvDir, "primary.json"), filepath.Join(pvDir, "primary-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, primaryPV.Save())
	primaryPub, err := primaryPV.GetPubKey(ctx)
	require.NoError(t, err)
	standbyPV, err := privval.GenFilePV(filepath.Join(pvDir, "standby.json"), filepath.Join(pvDir, "standby-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, standbyPV.Save())
	standbyPub, err := standbyPV.GetPubKey(ctx)
	require.NoError(t, err)
	consensusKey := &sdked25519.PubKey{Key: primaryPub.Bytes()}
	wrapper := app.NewTestWrapper(t, now, consensusKey, true, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := wrapper.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	home := t.TempDir()
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, home, nil)
	require.NoError(t, err)
	info, _, err := kr.NewMnemonic("fixture", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)
	secret, err := keyring.NewUnsafe(kr).UnsafeExportPrivKeyHex("fixture")
	require.NoError(t, err)
	payer, err := crypto.HexToECDSA(secret)
	require.NoError(t, err)
	t.Cleanup(func() { payer.D.SetInt64(0) })
	sender := utils.TestAcct{AccountAddress: info.GetAddress(), EvmAddress: crypto.PubkeyToAddress(payer.PublicKey), EvmPrivateKey: payer}
	a.EvmKeeper.SetAddressMapping(wrapper.Ctx, sender.AccountAddress, sender.EvmAddress)
	wrapper.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000)))
	seed := sha256.Sum256([]byte("repair 174 receipt signer"))
	public := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	keyID := sha256.Sum256(public)
	standbySeed := sha256.Sum256([]byte("repair m1092 standby signer"))
	standbyPublic := ed25519.NewKeyFromSeed(standbySeed[:]).Public().(ed25519.PublicKey)
	standbyID := sha256.Sum256(standbyPublic)
	pins := seqkeeper.ReceiptKeys{{KeyId: standbyID[:], PublicKey: standbyPublic, FirstHeight: 1, LastHeight: 1000}, {KeyId: keyID[:], PublicKey: public, FirstHeight: 1, LastHeight: 1000}}
	require.NoError(t, a.SequencerKeeper.InitGenesis(wrapper.Ctx, seqtypes.GenesisState{
		ActivationHeight: 2, Leaders: [][32]byte{[32]byte(consensusKey.Bytes()), [32]byte(standbyPub.Bytes())},
		ReceiptKeys: []seqtypes.ReceiptKey{{KeyId: standbyID, PublicKey: [32]byte(standbyPublic), FirstHeight: 1, LastHeight: 1000}, {KeyId: keyID, PublicKey: [32]byte(public), FirstHeight: 1, LastHeight: 1000}},
	}))
	if programs {
		repairM1092SeedPrograms(t, wrapper.Ctx, a, payer)
	}
	for _, seed := range configure {
		seed(wrapper.Ctx, a, payer)
	}
	params, block := *app.DefaultConsensusParams, *app.DefaultConsensusParams.Block
	block.MaxGasWanted = block.MaxGas
	params.Block = &block
	a.StoreConsensusParams(wrapper.Ctx, &params)
	a.UpgradeKeeper.SetDone(wrapper.Ctx, app.MergeUpgrade)
	_, err = a.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: now}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(ctx)
	require.NoError(t, err)
	upgradeInfo, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensusFiles := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(upgradeInfo))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensusFiles.KeyFile()), 0700))
	for destination, source := range map[string]string{consensusFiles.KeyFile(): filepath.Join(pvDir, "primary.json"), consensusFiles.StateFile(): filepath.Join(pvDir, "primary-state.json")} {
		data, err := os.ReadFile(source)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0700))
		require.NoError(t, os.WriteFile(destination, data, 0600))
	}
	var snapshot bytes.Buffer
	require.NoError(t, a.CommitMultiStore().Snapshot(1, protoio.NewDelimitedWriter(&snapshot)))
	// Replication uses the production commitment store. Configure this peer like
	// repair103Clone; the optional historical index is independent of replay.
	opts := viper.New()
	opts.Set("chain-id", a.ChainID)
	opts.Set(app.FlagSCEnable, true)
	opts.Set(app.FlagSCAsyncCommitBuffer, 0)
	opts.Set(app.FlagSCSnapshotInterval, 0)
	opts.Set(app.FlagSSEnable, false)
	opts.Set("evm.http_enabled", false)
	opts.Set("evm.ws_enabled", false)
	backup := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, true,
		tmconfig.TestConfig(), app.MakeEncodingConfig(), wasm.EnableAllProposals, opts, app.EmptyWasmOpts, nil,
		func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(false)
			ba.SetStoreLoader(func(ms sdk.CommitMultiStore) error {
				if err := ms.LoadLatestVersion(); err != nil {
					return err
				}
				_, err := ms.Restore(1, 0, protoio.NewDelimitedReader(bytes.NewReader(snapshot.Bytes()), 64<<20))
				return err
			})
		})
	t.Cleanup(func() { require.NoError(t, backup.Close()) })
	backup.ChainID = "pax-test"
	require.Equal(t, a.LastCommitID().Hash, backup.LastCommitID().Hash, "standby must restore the primary commitment before replay")
	standbyInfo, err := backup.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	standbyConsensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(standbyInfo))).PrivValidator
	for destination, source := range map[string]string{standbyConsensus.KeyFile(): filepath.Join(pvDir, "standby.json"), standbyConsensus.StateFile(): filepath.Join(pvDir, "standby-state.json")} {
		data, err := os.ReadFile(source)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0700))
		require.NoError(t, os.WriteFile(destination, data, 0600))
	}
	standbySeedPath := filepath.Join(t.TempDir(), "receipt.hex")
	require.NoError(t, os.WriteFile(standbySeedPath, []byte(hex.EncodeToString(standbySeed[:])), 0600))
	primaryTLS, standbyTLS := repairM1092TLS(t)
	return func() *repairM1092Runtime {
		host := fmt.Sprintf("127.0.1.%d", repairM1092StandbyHost.Add(1))
		standby, err := sequencer.StartStandby(ctx, sequencer.Config{
			Role: sequencer.RoleStandby, ReplicationTLS: standbyTLS, WALDir: filepath.Join(t.TempDir(), "standby-wal"),
			ReceiptKeyFile: standbySeedPath, StandbyAddress: host + ":0", ReplicationTimeout: 5 * time.Second,
		}, backup)
		require.NoError(t, err)
		address, err := processBoundTCPAddress(os.Getpid(), host)
		require.NoError(t, err)
		t.Cleanup(func() {
			cancel()
			require.Eventually(t, func() bool { return standby.Err() != nil }, 5*time.Second, time.Millisecond)
		})
		keyPath := filepath.Join(t.TempDir(), "receipt-key.hex")
		require.NoError(t, os.WriteFile(keyPath, []byte(hex.EncodeToString(seed[:])), 0600))
		cfg := sequencer.DefaultConfig()
		cfg.Enable = true
		cfg.StandbyAddress = address
		cfg.ReplicationTLS = primaryTLS
		cfg.ReplicationTimeout = 5 * time.Second
		cfg.NonceQueueMaxPerSender = 4
		cfg.NonceQueueMaxTotal = 8
		cfg.LeaseDuration = 3 * time.Second
		cfg.WALDir = filepath.Join(t.TempDir(), "wal")
		cfg.ReceiptKeyFile = keyPath
		primary, err := sequencer.NewNode(cfg, a)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, primary.Stop()) })
		require.NoError(t, primary.Start(ctx))

		consensusConfig, err := rpctest.CreateConfig(t, t.Name())
		require.NoError(t, err)
		consensusApp := kvstore.NewApplication()
		t.Cleanup(func() { require.NoError(t, consensusApp.Close()) })
		service, closeService, err := rpctest.StartTendermint(ctx, consensusConfig, consensusApp, rpctest.SuppressStdout)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, closeService(context.Background())) })
		localClient, err := local.New(service.(local.NodeService))
		require.NoError(t, err)
		// eth_gasPrice reads canonical block history, so an answering RPC listener
		// alone is insufficient: its consensus producer must have committed block 1.
		firstHeight := int64(1)
		require.Eventually(t, func() bool {
			block, err := localClient.Block(ctx, &firstHeight)
			return err == nil && block != nil && block.Block != nil && block.Block.Height == firstHeight
		}, 5*time.Second, 10*time.Millisecond, "the gas-price history must have a committed first block")
		rpcCfg := rpcconfig.DefaultConfig
		rpcCfg.HTTPAddress = fmt.Sprintf("127.0.2.%d", repairM1092EVMHost.Add(1))
		rpcCfg.HTTPPort = 0
		rpcCfg.WorkerPoolSize = 1
		rpcCfg.WorkerQueueSize = 8
		pendingSource, err := a.RPCPendingSnapshot()
		require.NoError(t, err)
		pending, err := evmrpc.NewPendingOption(pendingSource)
		require.NoError(t, err)
		server, err := evmrpc.NewEVMHTTPServer(rpcCfg, localClient, &a.EvmKeeper, a.BeginBlockKeepers, a.BaseApp,
			a.TracerAnteHandler, a.RPCContextProvider, func(int64) client.TxConfig { return a.GetTxConfig() }, "", a.GetStateStore(),
			evmrpc.NewSequencerOption(primary.Accepter, primary.ReceiptLogFeed),
			evmrpc.SequencerReceiptOption{Source: primary.ReceiptSource, Keeper: &a.SequencerKeeper}, pending)
		require.NoError(t, err)
		require.NoError(t, server.Start())
		t.Cleanup(server.Stop)
		rpcClient, err := gethrpc.DialHTTP("http://" + server.(*evmrpc.HTTPServer).ListenAddr())
		require.NoError(t, err)
		t.Cleanup(rpcClient.Close)
		n := &node{ctx: ctx, client: rpcClient, home: home, receiptPublicKey: public, rpcURL: "http://" + server.(*evmrpc.HTTPServer).ListenAddr()}
		require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0700))
		genesis, err := json.Marshal(map[string]any{"chain_id": "pax-test", "app_state": map[string]any{"sequencer": map[string]any{"receipt_keys": pins}}})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(home, "config", "genesis.json"), genesis, 0600))
		return &repairM1092Runtime{n, payer, pins, primary, a, backup}
	}
}

func repairM1092SeedPrograms(t *testing.T, ctx sdk.Context, a *app.App, payer *ecdsa.PrivateKey) {
	t.Helper()
	host := a.LayerXHost
	key := a.GetKey(app.LayerXHostStoreKey)
	worker, err := v1ProgramsSigner(payer)
	require.NoError(t, err)
	defer worker.D.SetInt64(0)
	private, _, err := v1ProgramsIdentity(worker)
	require.NoError(t, err)
	defer clear(private)
	publicKey := [32]byte(private.Public().(ed25519.PublicKey))
	did := []byte(evmtypes.LayerXDid(publicKey))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	authorityHash, err := lxphost.AuthorityHash(lxphost.AuthorityKindOwner, [32]byte{}, publicKey)
	require.NoError(t, err)
	authority := lxphost.KernelAuthority{Kind: lxphost.AuthorityKindOwner, AuthorityHash: authorityHash, Bound: true, Actor: actor, Principal: actor, VerifiedKey: publicKey}
	moduleKey := func(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }
	store := ctx.KVStore(key)
	initialFee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		initialFee = binary.BigEndian.AppendUint64(initialFee, 1)
	}
	occupancyAsset := [32]byte{0x91}
	initialFee = append(initialFee, occupancyAsset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		initialFee = binary.BigEndian.AppendUint64(initialFee, value)
	}
	initialFee = append(initialFee, authority.Actor[:]...)
	initialFee = append(initialFee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), initialFee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), initialFee)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Actor[:]...)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)

	execute := func(activityType uint32, payload []byte) {
		sequence, err := host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), host, authority, activityType, payload, sequence)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	salt := bytes.Repeat([]byte{0x55}, 32)
	assetMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(assetMaterial, salt...))
	register := append([]byte{0, 1}, asset[:]...)
	register = append(register, salt...)
	register = append(register, 1, 'T', 1, 'T', 6)
	register = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(register, 0), 1000000000)
	execute(0x00010001, append(register, 1, 0))
	execute(0x00010004, append([]byte{0, 1}, asset[:]...))
	_, payerAccount, err := lxphost.AssetAccountID(publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, payerAccount[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	execute(0x0001000a, mint)

	mainName, mainID, err := lxphost.MainAccountID(publicKey)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 100000000)
	mainAsset := [32]byte{0x91}
	main = field(main, mainAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, publicKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	mainAccount, err := host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, int64(100000000), mainAccount.Balance.Int64())

	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	treasury := field(nil, treasuryID[:])
	treasury = field(treasury, []byte("system:fees"))
	treasury = append(treasury, 10)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = field(treasury, asset[:])
	treasury = append(treasury, 1)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = append(treasury, 0, 0)
	treasury = field(treasury, make([]byte, 32))
	store.Set(append([]byte{6}, treasuryID[:]...), append(treasury, 0))
	treasuryAccount, err := host.ReadAccount(ctx, "system:fees", treasuryID)
	require.NoError(t, err)
	require.False(t, treasuryAccount.Frozen)
	require.True(t, treasuryAccount.Balance.IsZero())
	require.Equal(t, asset, treasuryAccount.AssetID)

	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

}

func repairM1092TLS(t *testing.T) (sequencer.ReplicationTLSConfig, sequencer.ReplicationTLSConfig) {
	t.Helper()
	primary, standby, err := fixtureReplicationTLS(t.TempDir())
	require.NoError(t, err)
	return primary, standby
}

func TestRepairM_1092_ConcurrentRecorderEndpoints(t *testing.T) {
	// Prepare application stores serially; start both complete network stacks together.
	firstStart := repairM1092PrepareRecorder(t, false)
	secondStart := repairM1092PrepareRecorder(t, false)
	type result struct {
		runtime *repairM1092Runtime
		vector  any
		err     error
	}
	results := make(chan result, 2)
	var starters sync.WaitGroup
	starters.Add(2)
	startTogether := make(chan struct{})
	for _, start := range []func() *repairM1092Runtime{firstStart, secondStart} {
		go func(start func() *repairM1092Runtime) {
			defer starters.Done()
			<-startTogether
			runtime := start()
			value, err := recordV2FromHome(runtime.node, "4299cb15eced23eb9a839162acd11e9a937edd05")
			results <- result{runtime, value, err}
		}(start)
	}
	close(startTogether)
	starters.Wait()
	close(results)
	hashes := map[common.Hash]bool{}
	var runtimes []*repairM1092Runtime
	for recorded := range results {
		require.NoError(t, recorded.err)
		runtimes = append(runtimes, recorded.runtime)
		v := recorded.vector.(*vectorV2)
		require.NoError(t, verifyVectorV2(v))
		require.False(t, hashes[v.Receipt.TxHash])
		hashes[v.Receipt.TxHash] = true
	}
	require.Len(t, runtimes, 2)
	require.NotEqual(t, runtimes[0].node.rpcURL, runtimes[1].node.rpcURL)
	require.NotContains(t, runtimes[0].node.rpcURL, ":0")
	require.NotContains(t, runtimes[1].node.rpcURL, ":0")
	// Bound address discovery consumes the actual endpoint of each live server.
	for _, runtime := range runtimes {
		host, _, err := net.SplitHostPort(strings.TrimPrefix(runtime.node.rpcURL, "http://"))
		require.NoError(t, err)
		address, err := processBoundTCPAddress(os.Getpid(), host)
		require.NoError(t, err)
		discovered := "http://" + address
		require.Equal(t, runtime.node.rpcURL, discovered)
		response, err := http.Post(discovered, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		require.Contains(t, string(body), `"result"`)
	}
}

func TestRepairM_1092_V2RetainsStandbyPins(t *testing.T) {
	runtime := repairM1092Recorder(t, false)
	value, err := recordV2FromHome(runtime.node, "4299cb15eced23eb9a839162acd11e9a937edd05")
	require.NoError(t, err)
	v := value.(*vectorV2)
	require.Equal(t, runtime.pins, v.ReceiptKeys)
	require.Len(t, v.ReceiptKeys, 2)
	require.Equal(t, hexutil.Bytes(runtime.node.receiptPublicKey), v.Receipt.SigningKey)
	require.NoError(t, verifyVectorV2(v))
	source, err := canonicalV2(v)
	require.NoError(t, err)
	require.Equal(t, v.Receipt.KeyID, source.Cases[0].PinnedKey.KeyID)
	require.Equal(t, runtime.pins, source.ReceiptKeys)
	recovered, err := vectorV2FromSource(source)
	require.NoError(t, err)
	require.Equal(t, v, recovered)
	state, err := runtime.backup.PendingSnapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(v.Receipt.Nonce)+1, runtime.backup.EvmKeeper.GetNonce(state, v.Receipt.From), "the real standby must replay the recorder's refused call")
	malformed := *source
	malformed.ReceiptKeys = append(seqkeeper.ReceiptKeys(nil), source.ReceiptKeys...)
	altered := malformed.ReceiptKeys[0]
	altered.LastHeight = 0
	malformed.ReceiptKeys[0] = altered
	_, err = vectorV2FromSource(&malformed)
	require.Error(t, err, "an unused standby pin must still be validated")
}

func TestRepairM_1092_V2RecorderStrictConsumers(t *testing.T) {
	runtime := repairM1092Recorder(t, false)
	value, err := recordV2FromHome(runtime.node, "4299cb15eced23eb9a839162acd11e9a937edd05")
	require.NoError(t, err)
	v := value.(*vectorV2)
	repairM1092VerifyV2Copies(t, v)
	signed, err := verify.ReceiptSignature(v.Receipt.Receipt, [32]byte(v.Receipt.SigningKey))
	require.NoError(t, err)
	require.Equal(t, int32(1), signed.Receipt.ResultCode)
	require.Equal(t, sha256.Sum256(v.Receipt.ReturnData), signed.Receipt.ActivityRoot)
	reason, err := abi.UnpackRevert(v.Receipt.ReturnData)
	require.NoError(t, err)
	require.Equal(t, v.RevertReason, reason)
	require.Equal(t, vectorV2Code, v.LXPCode)
	var tx ethtypes.Transaction
	require.NoError(t, tx.UnmarshalBinary(v.RawTransaction))
	require.Equal(t, layerxledger.GetABI().Methods["payExact"].ID, tx.Data()[:4])
	send, err := runtime.primary.ReceiptSource.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.Equal(t, send.Receipt, []byte(v.Receipt.Receipt))
}

func TestRepairM_1092_V1ProgramsProducer(t *testing.T) {
	TestVectorV1ProgramsRecorder(t)
}

func repairM1092VerifyV2Copies(t *testing.T, v *vectorV2) {
	t.Helper()
	require.NoError(t, verifyVectorV2(v))
	out := t.TempDir()
	require.NoError(t, writeVectorV2Copies(out, v))
	var source v2Source
	require.NoError(t, readJSON(filepath.Join(out, "conformance", "sequencer-receipt-v1.json"), &source))
	recovered, err := vectorV2FromSource(&source)
	require.NoError(t, err)
	require.Equal(t, v, recovered)
	var copy vectorV2
	require.NoError(t, readJSON(filepath.Join(out, vectorV2Name+".json"), &copy))
	require.Equal(t, v, &copy)
	var group struct {
		ReceiptKeys seqkeeper.ReceiptKeys `json:"receiptKeys"`
		Cases       []v2SourceCase        `json:"cases"`
	}
	require.NoError(t, readJSON(filepath.Join(out, "conformance", vectorV2Name+".json"), &group))
	require.Equal(t, source.Cases, group.Cases)
	require.Equal(t, source.ReceiptKeys, group.ReceiptKeys)
	var tx ethtypes.Transaction
	require.NoError(t, tx.UnmarshalBinary(source.Cases[0].RawTransaction))
	require.Equal(t, v.Receipt.TxHash, tx.Hash())
	encoded, err := json.Marshal(source)
	require.NoError(t, err)
	for _, mutation := range []string{"transaction", "signature", "authorization", "standby", "digest", "source", "outcome"} {
		t.Run(mutation, func(t *testing.T) {
			var altered v2Source
			require.NoError(t, json.Unmarshal(encoded, &altered))
			c := &altered.Cases[0]
			switch mutation {
			case "transaction":
				c.RawTransaction[len(c.RawTransaction)-1] ^= 1
			case "signature":
				c.SignedReceipt[len(c.SignedReceipt)-1] ^= 1
			case "authorization":
				c.PinnedKey.PublicKey[0] ^= 1
			case "standby":
				altered.ReceiptKeys[0].LastHeight = 0
			case "digest":
				c.ReceiptDigest[0] ^= 1
			case "source":
				altered.Provenance.Source += "-altered"
			case "outcome":
				c.Receipt.ReturnData[len(c.Receipt.ReturnData)-1] ^= 1
			}
			_, err := vectorV2FromSource(&altered)
			require.Error(t, err)
		})
	}
}

var repairM1092StandbyHost atomic.Uint32

var repairM1092EVMHost atomic.Uint32
