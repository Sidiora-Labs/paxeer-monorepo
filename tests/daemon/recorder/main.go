// Produce the committed/reopened transcript once; Python verification only replays it.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	slashingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const deletionHeight = int64(4)

var output = flag.String("out", "tests/daemon/fixtures/programs-commit-reopen.json", "recording path")

// The wrapper supplies fixture genesis and starts the production sequencer after
// the bootstrap block. FinalizeBlock, admission, execution and Commit are App's.
type recordingApp struct {
	*app.App
	initialize     func()
	afterBootstrap func() error
}

// Consensus captures its source at construction, before bootstrap commits. Keep
// that interface connected to the production source installed after block one.
func (a *recordingApp) Sequenced(height int64) bool {
	return a.App.Sequenced(a.GetCheckCtx(), height)
}

func (a *recordingApp) Admit(ctx context.Context, height int64, txs tmtypes.Txs) (int, error) {
	source := a.App.SequencerSource()
	if source == nil {
		return 0, fmt.Errorf("recording sequencer not started at height %d", height)
	}
	return source.Admit(ctx, height, txs)
}

func (a *recordingApp) AdmitWithAcknowledgements(ctx context.Context, height int64, txs tmtypes.Txs) (int, int, error) {
	source, ok := a.App.SequencerSource().(*sequencer.ConsensusSource)
	if !ok {
		return 0, 0, fmt.Errorf("recording primary sequencer not started at height %d", height)
	}
	return source.AdmitWithAcknowledgements(ctx, height, txs)
}

func (a *recordingApp) Seal(ctx context.Context, height int64) (tmtypes.SealedBlock, error) {
	source := a.App.SequencerSource()
	if source == nil {
		return tmtypes.SealedBlock{}, fmt.Errorf("recording sequencer not started at height %d", height)
	}
	return source.Seal(ctx, height)
}

func (a *recordingApp) InitChain(ctx context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	response, err := a.App.InitChain(ctx, req)
	if err == nil {
		a.initialize()
		// Publish fixture-only kernel/binding/upgrade rows to the shared genesis
		// parent so both production genesis branches observe the same inputs.
		// Bank genesis remains on its normal path into the supply invariant.
		genesisContext := a.GetContextForDeliverTx(nil)
		for _, name := range []string{app.LayerXHostStoreKey, "evm", "upgrade", "sequencer", "params", "layerxanchor", "layerxcustody"} {
			genesisContext.MultiStore().GetKVStore(a.GetKey(name)).(sdk.CacheKVStore).Write()
		}
	}
	return response, err
}
func (a *recordingApp) Commit(ctx context.Context) (*abci.ResponseCommit, error) {
	response, err := a.App.Commit(ctx)
	if err == nil && a.LastBlockHeight() == 1 && a.afterBootstrap != nil {
		err = a.afterBootstrap()
		a.afterBootstrap = nil
	}
	return response, err
}

func main() {
	testing.Main(func(string, string) (bool, error) { return true, nil }, []testing.InternalTest{{Name: "RecordProgramsCommitReopen", F: record}}, nil, nil)
}

func record(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	app.EnableOCC = false
	cfg, err := rpctest.CreateConfig(t, "programs-commit-reopen")
	require.NoError(t, err)
	cfg.Consensus.UnsafeCommitTimeoutOverride = 3 * time.Second
	cfg.Consensus.CreateEmptyBlocks = true
	home := cfg.RootDir
	cfg.DBBackend = "goleveldb"
	cfg.AutobahnConfigFile = ""
	cfg.Consensus.Sequencer.Enable = true
	cfg.Consensus.Sequencer.TimeoutCommit = 3 * time.Second
	appConfig := *cfg
	appConfig.AutobahnConfigFile = "programs-commit-reopen"
	pv, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	senderKey, err := crypto.HexToECDSA("8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63")
	require.NoError(t, err)
	sender := crypto.PubkeyToAddress(senderKey.PublicKey)
	receiptSeed := sha256.Sum256([]byte("daemon programs commitment and recovery"))
	receiptPublic := ed25519.NewKeyFromSeed(receiptSeed[:]).Public().(ed25519.PublicKey)
	receiptID := sha256.Sum256(receiptPublic)
	pins := seqkeeper.ReceiptKeys{{KeyId: receiptID[:], PublicKey: receiptPublic, FirstHeight: 1, LastHeight: 1000}}
	receiptPath := filepath.Join(home, "config", "receipt-key")
	require.NoError(t, os.WriteFile(receiptPath, []byte(hex.EncodeToString(receiptSeed[:])), 0600))
	genesis, err := tmtypes.GenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	genesis.ChainID = "hyperpax_125-1"
	genesis.GenesisTime = time.Now().Add(-time.Second).UTC()
	genesis.Validators = []tmtypes.GenesisValidator{{Address: pv.Key.PubKey.Address(), PubKey: pv.Key.PubKey, Power: 1, Name: "programs-recorder"}}
	enc := app.MakeEncodingConfig()
	state := app.NewDefaultGenesisState(enc.Marshaler)
	state["sequencer"], err = json.Marshal(map[string]any{"leaders": [][]byte{pv.Key.PubKey.Bytes()}, "receipt_keys": pins, "activation_height": uint64(2)})
	require.NoError(t, err)
	pub, err := cryptocodec.FromTmPubKeyInterface(pv.Key.PubKey)
	require.NoError(t, err)
	anyPub, err := codectypes.NewAnyWithValue(pub)
	require.NoError(t, err)
	account := authtypes.NewBaseAccountWithAddress(sdk.AccAddress(sender.Bytes()))
	state[authtypes.ModuleName] = enc.Marshaler.MustMarshalJSON(authtypes.NewGenesisState(authtypes.DefaultParams(), []authtypes.GenesisAccount{account}))
	validator := stakingtypes.Validator{OperatorAddress: sdk.ValAddress(pv.Key.PubKey.Address()).String(), ConsensusPubkey: anyPub,
		Status: stakingtypes.Bonded, Tokens: sdk.NewInt(1_000_000), DelegatorShares: sdk.OneDec(), UnbondingTime: time.Unix(0, 0).UTC(),
		Commission: stakingtypes.NewCommission(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec()), MinSelfDelegation: sdk.ZeroInt()}
	stakingState := stakingtypes.NewGenesisState(stakingtypes.DefaultParams(), []stakingtypes.Validator{validator}, []stakingtypes.Delegation{stakingtypes.NewDelegation(account.GetAddress(), pv.Key.PubKey.Address().Bytes(), sdk.OneDec())})
	stakingState.Params.MaxVotingPowerRatio = sdk.OneDec()
	state[stakingtypes.ModuleName] = enc.Marshaler.MustMarshalJSON(stakingState)
	consensusAddress := sdk.ConsAddress(pv.Key.PubKey.Address())
	slashingState := slashingtypes.DefaultGenesisState()
	slashingState.SigningInfos = []slashingtypes.SigningInfo{{
		Address:              consensusAddress.String(),
		ValidatorSigningInfo: slashingtypes.NewValidatorSigningInfo(consensusAddress, 0, 0, time.Unix(0, 0).UTC(), false, 0),
	}}
	state[slashingtypes.ModuleName] = enc.Marshaler.MustMarshalJSON(slashingState)
	balances := []banktypes.Balance{{Address: account.GetAddress().String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000_000))},
		{Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000))}}
	state[banktypes.ModuleName] = enc.Marshaler.MustMarshalJSON(banktypes.NewGenesisState(banktypes.DefaultGenesisState().Params, balances,
		balances[0].Coins.Add(balances[1].Coins...), []banktypes.Metadata{}, []banktypes.WeiBalance{}))
	genesis.AppState, err = json.Marshal(state)
	require.NoError(t, err)
	genesis.ConsensusParams = tmtypes.DefaultConsensusParams()
	genesis.ConsensusParams.Block.MaxGas = 100_000_000
	genesis.ConsensusParams.Block.MaxGasWanted = 100_000_000
	require.NoError(t, genesis.SaveAs(cfg.GenesisFile()))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	rpcPort := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	opts := viper.New()
	opts.Set("chain-id", "hyperpax_125-1")
	opts.Set("giga_executor.enabled", false)
	opts.Set("giga_executor.occ_enabled", false)
	opts.Set(app.FlagSCEnable, true)
	opts.Set(app.FlagSCDirectory, home)
	opts.Set(app.FlagSCAsyncCommitBuffer, 0)
	opts.Set(app.FlagSCSnapshotInterval, 0)
	opts.Set(app.FlagSSEnable, true)
	opts.Set(app.FlagSSBackend, "pebbledb")
	opts.Set("evm.http_enabled", true)
	opts.Set("evm.ws_enabled", false)
	opts.Set("evm.http_address", "127.0.0.1")
	opts.Set("evm.http_port", rpcPort)
	opts.Set("evm.worker_pool_size", 1)
	opts.Set("evm.worker_queue_size", 8)
	opts.Set("evm.filter_timeout", time.Minute)
	genesisBytes, err := os.ReadFile(cfg.GenesisFile())
	require.NoError(t, err)
	genesisHash := sha256.Sum256(genesisBytes)
	open := func() *app.App {
		persistedGenesis, err := os.ReadFile(cfg.GenesisFile())
		require.NoError(t, err)
		require.Equal(t, genesisHash, sha256.Sum256(persistedGenesis))
		db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, filepath.Join(home, "data"))
		require.NoError(t, err)
		opened := app.New(db, nil, true, map[int64]bool{}, home, 1, true, &appConfig, enc, nil, opts, app.EmptyWasmOpts, nil,
			func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
		id := opened.CommitMultiStore().LastCommitID()
		fmt.Printf("recorder open home=%s application=goleveldb sc=%s ss=pebbledb genesis=%X height=%d app_hash=%X\n", home, home, genesisHash, id.Version, id.Hash)
		return opened
	}
	a := open()
	var node *sequencer.Node
	startSequencer := func() error {
		config := sequencer.DefaultConfig()
		config.Enable, config.StandbyAddress = true, ""
		config.WALDir, config.ReceiptKeyFile = filepath.Join(home, "sequencer-wal"), receiptPath
		config.LeaseDuration = 30 * time.Second
		config.NonceQueueMaxPerSender, config.NonceQueueMaxTotal = 4, 8
		node, err = sequencer.NewNode(config, a)
		if err != nil {
			return err
		}
		return node.Start(ctx)
	}
	var program [32]byte
	var deploy []byte
	bootstrapReady := make(chan struct{})
	var bootstrapErr error
	wrapped := &recordingApp{App: a, afterBootstrap: func() error {
		bootstrapErr = startSequencer()
		close(bootstrapReady)
		return bootstrapErr
	}}
	wrapped.initialize = func() {
		chainCtx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
		for _, name := range []string{app.ActivationUpgrade, app.V610Upgrade, app.V6120Upgrade} {
			a.UpgradeKeeper.SetDone(chainCtx, name)
		}
		info, err := json.Marshal(map[string]any{"leaders": [][]byte{pv.Key.PubKey.Bytes()}, "receipt_keys": pins})
		require.NoError(t, err)
		a.RefreshPrecompiles(chainCtx)
		a.UpgradeKeeper.ApplyUpgrade(chainCtx, upgradetypes.Plan{Name: app.MergeUpgrade, Height: 1, Info: string(info)})
		require.Equal(t, int64(1), a.UpgradeKeeper.GetDoneHeight(chainCtx, app.MergeUpgrade))
		a.EvmKeeper.SetAddressMapping(chainCtx, account.GetAddress(), sender)
		program, deploy = seedProgram(t, a, chainCtx, sender)
	}
	service, stop, err := rpctest.StartTendermint(ctx, cfg, wrapped, rpctest.SuppressStdout, rpctest.PreserveRoot)
	require.NoError(t, err)
	register := func(service any) *gethrpc.Client {
		localNode, ok := service.(local.NodeService)
		require.True(t, ok)
		localClient, err := local.New(localNode)
		require.NoError(t, err)
		a.RegisterLocalServices(localClient, a.GetTxConfig())
		a.SendEVMServerStartSignals()
		client, err := gethrpc.DialHTTP(fmt.Sprintf("http://127.0.0.1:%d", rpcPort))
		require.NoError(t, err)
		require.Eventually(t, func() bool { var version string; return client.CallContext(ctx, &version, "web3_clientVersion") == nil }, 10*time.Second, 10*time.Millisecond)
		return client
	}
	select {
	case <-bootstrapReady:
		require.NoError(t, bootstrapErr)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	client := register(service)
	waitHeight := func(height int64) {
		require.Eventually(t, func() bool {
			var n hexutil.Uint64
			return client.CallContext(ctx, &n, "eth_blockNumber") == nil && int64(n) >= height
		}, 45*time.Second, 10*time.Millisecond)
	}
	waitHeight(1)
	send := func(nonce uint64, data []byte) (string, string) {
		tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{ChainID: big.NewInt(125), Nonce: nonce, GasTipCap: big.NewInt(1_000_000_000), GasFeeCap: big.NewInt(500_000_000_000), Gas: 20_000_000, To: &lp.Address, Value: new(big.Int), Data: data}), ethtypes.NewLondonSigner(big.NewInt(125)), senderKey)
		require.NoError(t, err)
		raw, err := tx.MarshalBinary()
		require.NoError(t, err)
		var hash common.Hash
		require.NoError(t, client.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
		require.Equal(t, tx.Hash(), hash)
		return hexutil.Encode(raw), hash.Hex()
	}
	_, deployment := send(0, deploy)
	committed := func(hash string) json.RawMessage {
		var result json.RawMessage
		require.Eventually(t, func() bool {
			if client.CallContext(ctx, &result, "pax_sequencerReceipt", hash) != nil {
				return false
			}
			var answer struct {
				Rung   string `json:"rung"`
				Status string `json:"status"`
			}
			if json.Unmarshal(result, &answer) != nil || answer.Rung != "committed" {
				return false
			}
			require.Equal(t, "0x1", answer.Status, string(result))
			return true
		}, 30*time.Second, 10*time.Millisecond)
		return result
	}
	committed(deployment)
	programs := programsABI(t)
	input, err := programs.Pack("call", program, uint16(1), "layerx_call", []byte{}, []byte{0, 0}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), uint32(16), [7]uint64{1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096}, big.NewInt(100_000_000))
	require.NoError(t, err)
	raw, hash := send(1, input)
	answer := committed(hash)
	var receipt evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(answer, &receipt))
	require.NoError(t, receipt.Verify(pins))
	callData, err := programs.Pack("getProgram", program)
	require.NoError(t, err)
	phase := func(submit bool) map[string]any {
		records := []map[string]any{}
		if submit {
			records = append(records, map[string]any{"method": "eth_sendRawTransaction", "params": []any{raw}, "result": hash})
		}
		observe := func(method string, params ...any) json.RawMessage {
			var result json.RawMessage
			require.NoError(t, client.CallContext(ctx, &result, method, params...))
			records = append(records, map[string]any{"method": method, "params": params, "result": result})
			return result
		}
		observe("pax_sequencerReceipt", hash)
		status := observe("eth_getTransactionReceipt", hash)
		var mined struct {
			Status      string `json:"status"`
			BlockNumber string `json:"blockNumber"`
		}
		require.NoError(t, json.Unmarshal(status, &mined))
		require.Equal(t, "0x1", mined.Status)
		params := []any{map[string]string{"to": strings.ToLower(lp.Address.Hex()), "data": hexutil.Encode(callData)}, mined.BlockNumber}
		state := observe("eth_call", params...)
		return map[string]any{"rpc": records, "program_id": hex.EncodeToString(program[:]), "state": map[string]any{"method": "eth_call", "params": params, "result": state}, "expected_program_state": state}
	}
	before := phase(true)
	waitHeight(deletionHeight - 1)
	require.NoError(t, stop(context.Background()))
	require.NoError(t, node.Stop())
	require.Equal(t, deletionHeight-1, a.LastBlockHeight())
	committedID := a.CommitMultiStore().LastCommitID()
	fmt.Printf("recorder stopped height=%d app_hash=%X\n", committedID.Version, committedID.Hash)
	require.NoError(t, a.Close())
	client.Close()
	// Check persistence before changing the mounted store inventory.
	a = open()
	require.Equal(t, committedID, a.CommitMultiStore().LastCommitID(), "plain reopen must preserve the committed height and hash")
	require.NoError(t, a.UpgradeKeeper.DumpUpgradeInfoToDisk(deletionHeight, app.AnchorStoreDeleteUpgrade))
	require.NoError(t, a.Close())
	a = open()
	wrapped = &recordingApp{App: a}
	service, stop, err = rpctest.StartTendermint(ctx, cfg, wrapped, rpctest.SuppressStdout, rpctest.PreserveRoot)
	require.NoError(t, err)
	require.NoError(t, startSequencer())
	client = register(service)
	waitHeight(deletionHeight)
	after := phase(false)
	require.Equal(t, deletionHeight, a.UpgradeKeeper.GetDoneHeight(a.GetCheckCtx(), app.AnchorStoreDeleteUpgrade))
	var stores []string
	require.Nil(t, a.CommitMultiStore().GetCommitKVStore(a.GetKey("layerxanchor")))
	for _, key := range a.CommitMultiStore().StoreKeys() {
		if a.CommitMultiStore().GetCommitKVStore(key) != nil {
			stores = append(stores, key.Name())
		}
	}
	require.NotContains(t, stores, "layerxanchor")
	revision, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	decoded, err := codec.DecodeReceipt(receipt.Receipt)
	require.NoError(t, err)
	digest := decoded.Digest()
	document := map[string]any{"schema": "paxd/programs-commit-reopen/v1", "provenance": map[string]any{"generator": "tests/daemon/recorder/main.go", "paxd_revision": string(bytes.TrimSpace(revision)), "command": "go run ./tests/daemon/recorder"},
		"case":         map[string]any{"rawTransaction": raw, "receiptJson": json.RawMessage(answer), "signedReceipt": hexutil.Encode(receipt.Receipt), "receiptDigest": hexutil.Encode(digest[:]), "pinnedKey": map[string]any{"keyId": hexutil.Encode(receiptID[:]), "publicKey": hexutil.Encode(receiptPublic), "firstHeight": 1, "lastHeight": 1000}},
		"merge_height": 1, "anchor_store_delete_height": deletionHeight, "node_stopped_between_phases": true, "anchor_store_absent_after_reopen": true, "stores_after_reopen": stores,
		"phases": map[string]any{"after_merge": before, "after_anchor_delete_reopen": after}}
	require.NoError(t, stop(context.Background()))
	require.NoError(t, node.Stop())
	require.NoError(t, a.Close())
	client.Close()
	encoded, err := json.MarshalIndent(document, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(*output), 0755))
	require.NoError(t, os.WriteFile(*output, append(encoded, '\n'), 0644))
}

// Seed prerequisite accounts and governance records using the retained runtime fixture.
func seedProgram(t *testing.T, a *app.App, ctx sdk.Context, sender common.Address) ([32]byte, []byte) {
	host := a.LayerXHost
	key := a.GetKey(app.LayerXHostStoreKey)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	publicKey := [32]byte(private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), sender, 0)
	require.NoError(t, err)
	_, err = a.EvmKeeper.BindLayerX(ctx, sender, publicKey, [64]byte(ed25519.Sign(private, message)))
	require.NoError(t, err)
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
	_, payer, err := lxphost.AssetAccountID(publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, payer[:]...)
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

	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}

	program := sha256.Sum256([]byte("sequencer receipt vector program"))
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)

	programs := programsABI(t)
	deploy, err := programs.Pack("deploy", program, uint16(1), uint8(1), authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	return program, deploy
}

func programsABI(t *testing.T) abi.ABI {
	raw, err := os.ReadFile("precompiles/layerxprograms/abi.json")
	require.NoError(t, err)
	parsed, err := abi.JSON(bytes.NewReader(raw))
	require.NoError(t, err)
	return parsed
}
