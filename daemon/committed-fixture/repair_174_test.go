package main

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
	"reflect"
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
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestRepair_174_FixtureV2SourceContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	oldOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = oldOCC })
	now := time.Now().UTC()
	consensusKey := sdked25519.GenPrivKey().PubKey()
	wrapper := app.NewTestWrapper(t, now, consensusKey, true, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := wrapper.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	sender := utils.NewSigner()
	a.EvmKeeper.SetAddressMapping(wrapper.Ctx, sender.AccountAddress, sender.EvmAddress)
	wrapper.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000)))
	seed := sha256.Sum256([]byte("repair 174 receipt signer"))
	public := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	keyID := sha256.Sum256(public)
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: public, FirstHeight: 1, LastHeight: 1000}}
	require.NoError(t, a.SequencerKeeper.InitGenesis(wrapper.Ctx, seqtypes.GenesisState{
		ActivationHeight: 2, Leaders: [][32]byte{[32]byte(consensusKey.Bytes())},
		ReceiptKeys: []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: [32]byte(public), FirstHeight: 1, LastHeight: 1000}},
	}))
	params, block := *app.DefaultConsensusParams, *app.DefaultConsensusParams.Block
	block.MaxGasWanted = block.MaxGas
	params.Block = &block
	a.StoreConsensusParams(wrapper.Ctx, &params)
	a.UpgradeKeeper.SetDone(wrapper.Ctx, app.MergeUpgrade)
	_, err := a.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: now}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(ctx)
	require.NoError(t, err)
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensusFiles := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensusFiles.KeyFile()), 0700))
	pv, err := privval.GenFilePV(consensusFiles.KeyFile(), consensusFiles.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	keyPath := filepath.Join(t.TempDir(), "receipt-key.hex")
	require.NoError(t, os.WriteFile(keyPath, []byte(hex.EncodeToString(seed[:])), 0600))
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.StandbyAddress = ""
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	rpcCfg := rpcconfig.DefaultConfig
	rpcCfg.HTTPAddress = "127.0.0.1"
	rpcCfg.HTTPPort = listener.Addr().(*net.TCPAddr).Port
	rpcCfg.WorkerPoolSize = 1
	rpcCfg.WorkerQueueSize = 8
	require.NoError(t, listener.Close())
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
	rpcClient, err := gethrpc.DialHTTP(fmt.Sprintf("http://127.0.0.1:%d", rpcCfg.HTTPPort))
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	n := &node{ctx: ctx, client: rpcClient}
	// This is the recorder's live producer: it signs and submits the setup and
	// oversized payment over HTTP, then consumes the real keeper's signed answer.
	v, err := recordV2RPC(n, sender.EvmPrivateKey, pins, vectorV2Provenance{
		Revision: "a2c1b83e1c3e7ecd65fa771738df55e7e34f56fd", ChainID: "pax-test",
		Generator: "daemon/committed-fixture/vector_v2.go", Command: "committed-fixture record --vector " + vectorV2Name,
	})
	require.NoError(t, err)
	require.Equal(t, vectorV2Code, v.LXPCode)
	require.Equal(t, vectorV2SourceRef, v.Provenance.Source)
	require.Equal(t, sender.EvmAddress, v.Receipt.From)
	require.Zero(t, v.Receipt.Status)
	require.Empty(t, v.Receipt.Logs)
	var tx ethtypes.Transaction
	require.NoError(t, tx.UnmarshalBinary(v.RawTransaction))
	require.Equal(t, layerxledger.GetABI().Methods["payExact"].ID, tx.Data()[:4])
	args, err := layerxledger.GetABI().Methods["payExact"].Inputs.Unpack(tx.Data()[4:])
	require.NoError(t, err)
	require.Equal(t, new(big.Int).Lsh(big.NewInt(1), 100), args[3])
	state, err := primary.Executor.Snapshot()
	require.NoError(t, err)
	balance, err := a.LayerXLedgerKeeper.BalanceOf(state, args[0].([32]byte), args[2].([32]byte))
	require.NoError(t, err)
	require.True(t, balance.IsZero(), "the intended refusal is an unfunded ledger account")
	actor, err := a.LayerXLedgerKeeper.Actor(state, v.Receipt.From)
	require.NoError(t, err)
	require.True(t, actor.Authority.Bound)
	sequence, err := a.LayerXLedgerKeeper.NextSequence(state, actor.DIDKey, args[2].([32]byte))
	require.NoError(t, err)
	require.Zero(t, sequence, "refusal must not consume the payer's ledger sequence")
	signed, err := verify.ReceiptSignature(v.Receipt.Receipt, [32]byte(public))
	require.NoError(t, err)
	require.Equal(t, int32(1), signed.Receipt.ResultCode)
	require.Equal(t, sha256.Sum256(v.Receipt.ReturnData), signed.Receipt.ActivityRoot)
	reason, err := abi.UnpackRevert(v.Receipt.ReturnData)
	require.NoError(t, err)
	require.Equal(t, v.RevertReason, reason)
	send, err := primary.ReceiptSource.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.NotNil(t, send)
	require.Equal(t, []byte(v.Receipt.Receipt), send.Receipt)

	out := t.TempDir()
	require.NoError(t, writeVectorV2Copies(out, v))
	var source v2Source
	require.NoError(t, readJSON(filepath.Join(out, "conformance", "sequencer-receipt-v1.json"), &source))
	recovered, err := vectorV2FromSource(&source)
	require.NoError(t, err)
	require.Equal(t, v, recovered)
	var daemonCopy vectorV2
	require.NoError(t, readJSON(filepath.Join(out, vectorV2Name+".json"), &daemonCopy))
	require.True(t, reflect.DeepEqual(v, &daemonCopy), "daemon copy must preserve the entire producer result")
	var group struct {
		Cases []json.RawMessage `json:"cases"`
	}
	var canonical struct {
		Cases []json.RawMessage `json:"cases"`
	}
	require.NoError(t, readJSON(filepath.Join(out, "conformance", vectorV2Name+".json"), &group))
	require.NoError(t, readJSON(filepath.Join(out, "conformance", "sequencer-receipt-v1.json"), &canonical))
	require.Equal(t, canonical.Cases, group.Cases, "copied cases must be byte-identical to their canonical source")
	require.Equal(t, common.Hash(signed.Digest), source.Cases[0].ReceiptDigest)
	require.Equal(t, hexutil.Bytes(v.RawTransaction), source.Cases[0].RawTransaction)
	require.Equal(t, hexutil.Bytes(send.Receipt), source.Cases[0].SignedReceipt)

	encoded, err := json.Marshal(source)
	require.NoError(t, err)
	for _, mutation := range []string{"transaction", "signature", "authorization", "code", "source", "digest", "predecessor", "receipt"} {
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
			case "code":
				c.LXPCode = "LXP_PRECOMPILE_UNKNOWN_METHOD"
			case "source":
				altered.Provenance.Source += "-substituted"
			case "digest":
				c.ReceiptDigest[0] ^= 1
			case "predecessor":
				c.PrevSeqHash[0] ^= 1
			case "receipt":
				c.Receipt.ReturnData[len(c.Receipt.ReturnData)-1] ^= 1
			}
			_, err := vectorV2FromSource(&altered)
			require.Error(t, err, "canonical consumer must refuse altered %s", mutation)
		})
	}
}

func TestRepair_174_HistoricalSourceEquality(t *testing.T) {
	var v vectorV2
	require.NoError(t, readJSON("vectors/"+vectorV2Name+".json", &v))
	require.NoError(t, verifyVectorV2(&v))
	signed, err := verify.ReceiptSignature(v.Receipt.Receipt, [32]byte(v.Receipt.SigningKey))
	require.NoError(t, err)
	vector_v2_test(t, &v, signed.Digest)
	generated, err := canonicalV2(&v)
	require.NoError(t, err)
	sourceJSON, err := os.ReadFile("../../" + vectorV2SourcePath)
	require.NoError(t, err)
	var source struct {
		Cases []v2SourceCase `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(sourceJSON, &source))
	var matches int
	for _, c := range source.Cases {
		if c.ID == vectorV2CaseID {
			matches++
			require.Equal(t, c, generated.Cases[0])
			require.True(t, bytes.Equal(c.RawTransaction, v.RawTransaction))
		}
	}
	require.Equal(t, 1, matches)
}

func TestRepair_174_MergedBootstrap(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	n, err := prepareHome(ctx, executable)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(n.home) })
	_, err = n.prepareStandby()
	require.NoError(t, err)
	require.NoError(t, n.configureV2Genesis())
	stop, err := n.start()
	if err != nil {
		for _, peer := range []*node{n, n.standby} {
			var tail bytes.Buffer
			peer.printLogTail(&tail)
			t.Log(tail.String())
		}
	}
	require.NoError(t, err)
	t.Cleanup(stop)
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	require.NoError(t, err)
	secret, err := keyring.NewUnsafe(kr).UnsafeExportPrivKeyHex("fixture")
	require.NoError(t, err)
	payer, err := crypto.HexToECDSA(secret)
	require.NoError(t, err)
	defer payer.D.SetInt64(0)
	var genesis struct {
		AppState struct {
			Sequencer struct {
				ReceiptKeys seqkeeper.ReceiptKeys `json:"receipt_keys"`
			} `json:"sequencer"`
		} `json:"app_state"`
	}
	require.NoError(t, readJSON(filepath.Join(n.home, "config", "genesis.json"), &genesis))
	var pins seqkeeper.ReceiptKeys
	for _, pin := range genesis.AppState.Sequencer.ReceiptKeys {
		if bytes.Equal(pin.PublicKey, n.receiptPublicKey) {
			pins = append(pins, pin)
		}
	}
	require.Len(t, pins, 1)
	v, err := recordV2RPC(n, payer, pins, vectorV2Provenance{
		Revision: "a2c1b83e1c3e7ecd65fa771738df55e7e34f56fd", ChainID: fixtureChainID,
		Generator: "daemon/committed-fixture/vector_v2.go", Command: "committed-fixture record --vector " + vectorV2Name,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, uint64(v.Receipt.Height), uint64(3))
	require.Equal(t, vectorV2Code, v.LXPCode)
	require.NoError(t, verifyVectorV2(v))
	source, err := canonicalV2(v)
	require.NoError(t, err)
	recovered, err := vectorV2FromSource(source)
	require.NoError(t, err)
	require.Equal(t, v, recovered)
}
