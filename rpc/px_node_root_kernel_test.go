package evmrpc_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmutils "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	seqnode "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	signing "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type pxRootKernelHarness struct {
	*pxRootAccountHarness
	source *seqnode.ReceiptSource
	pins   seqkeeper.ReceiptKeys
	tx     *ethtypes.Transaction
	batch  []byte
	trust  []map[string]interface{}
}

// Connect the one produced sequenced height to the node's proposal builder.
// Other heights use the real kvstore application and ordinary consensus, so
// the tests can also exercise blocks with an absent or malformed batch.
type pxRootKernelConsensusApp struct {
	*kvstore.Application
	*seqnode.ConsensusSource
}

func (a *pxRootKernelConsensusApp) Sequenced(height int64) bool {
	sealed := a.ConsensusSource.Sealed()
	return sealed != nil && sealed.Height == height && a.ConsensusSource.Sequenced(height)
}

// The real executor, receipter, WAL release and ConsensusSource produce the
// transactions and receipts. A local consensus node commits those exact bytes;
// the RPC listeners read its local client and the committed sequencer stores.
func newPxRootKernelHarness(t *testing.T, mixed ...bool) *pxRootKernelHarness {
	mixedBlock := len(mixed) != 0 && mixed[0]
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	config, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	genesis, err := tmtypes.GenesisDocFromFile(config.GenesisFile())
	require.NoError(t, err)
	genesis.ChainID, genesis.InitialHeight = "pax-test", 2
	genesis.Validators[0].Power = 10000
	// A real second page, with one signer still holding a supermajority.
	for i := 0; i < 100; i++ {
		key := tmed25519.GenerateSecretKey().Public()
		genesis.Validators = append(genesis.Validators, tmtypes.GenesisValidator{Address: key.Address(), PubKey: key, Power: 1})
	}
	require.NoError(t, genesis.SaveAs(config.GenesisFile()))
	validator, err := privval.LoadFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile())
	require.NoError(t, err)
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	now := time.Now().Add(-time.Second).UTC()
	application := app.SetupWithSc(t, true, false, app.TestAppOpts{UseSc: true}, func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	state, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	_, err = application.InitChain(ctx, &abci.RequestInitChain{Time: now, ChainId: genesis.ChainID, ConsensusParams: app.DefaultConsensusParams, AppStateBytes: state})
	require.NoError(t, err)
	producerCtx := application.NewContext(false, tmproto.Header{ChainID: genesis.ChainID, Height: 1, Time: now})
	sender := utils.NewSigner()
	application.EvmKeeper.SetAddressMapping(producerCtx, sender.AccountAddress, sender.EvmAddress)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000))
	require.NoError(t, application.BankKeeper.MintCoins(producerCtx, minttypes.ModuleName, coins))
	require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(producerCtx, minttypes.ModuleName, sender.AccountAddress, coins))
	bankSender := utils.NewSigner()
	if mixedBlock {
		require.NoError(t, application.BankKeeper.MintCoins(producerCtx, minttypes.ModuleName, coins))
		require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(producerCtx, minttypes.ModuleName, bankSender.AccountAddress, coins))
		domainParams := application.LayerXChainsKeeper.GetParams(producerCtx)
		operator := sender.EvmAddress
		bond := domainParams.MinBond.Add(domainParams.CreationFee)
		domainCoins := sdk.NewCoins(sdk.NewCoin(application.EvmKeeper.GetBaseDenom(producerCtx), bond))
		require.NoError(t, application.BankKeeper.MintCoins(producerCtx, "evm", domainCoins))
		require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(producerCtx, "evm", sender.AccountAddress, domainCoins))
		domain, err := application.LayerXChainsKeeper.Create(producerCtx, operator, "batch-activity", domainParams.MinGasAllowance, domainParams.MinByteAllowance, 0, bond, application.EvmKeeper.GetNextBaseFeePerGas(producerCtx))
		require.NoError(t, err)
		require.Equal(t, uint32(1), domain.Id)
	}
	seed := bytes.Repeat([]byte{0x34}, 32)
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	keyID := bytes.Repeat([]byte{0x24}, 32)
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: math.MaxUint64}}
	require.NoError(t, application.SequencerKeeper.SetReceiptKeys(producerCtx, pins))
	require.NoError(t, application.SequencerKeeper.SetActivationHeight(producerCtx, 2))
	application.UpgradeKeeper.SetDone(producerCtx, app.MergeUpgrade)
	params := application.GetConsensusParams(producerCtx)
	params.Block.MaxGasWanted = 50_000_000
	application.StoreConsensusParams(producerCtx, params)
	_, err = application.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: genesis.ChainID, Height: 1, Time: now}})
	require.NoError(t, err)
	application.SetDeliverStateToCommit()
	_, err = application.Commit(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), application.LastBlockHeight())
	require.True(t, application.Sequenced(application.GetCheckCtx(), 2))
	exec, err := seqnode.NewExecutor(application, seqnode.Options{ChainID: genesis.ChainID, ProposerAddress: validator.Key.PubKey.Address()})
	require.NoError(t, err)
	require.NoError(t, exec.Open(2, now.Add(time.Second).UnixMilli()))
	keyPath := filepath.Join(t.TempDir(), "receipt-key")
	require.NoError(t, os.WriteFile(keyPath, []byte(hex.EncodeToString(seed)), 0o600))
	authorizations, err := application.SequencerKeeper.ReceiptKeyPins(application.GetCheckCtx())
	require.NoError(t, err)
	key, err := seqnode.LoadReceiptKey(keyPath, config.PrivValidator.KeyFile(), authorizations, 2)
	require.NoError(t, err)
	receipter, err := seqnode.NewReceipter(exec, application, &application.EvmKeeper, key, app.EVMReceiptModuleID)
	require.NoError(t, err)
	log, err := wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(log.Close)
	replicator, err := seqnode.NewReplicator(seqnode.ReplicatorConfig{WAL: log, Standalone: true})
	require.NoError(t, err)
	t.Cleanup(replicator.Close)
	_, err = replicator.Release(ctx, &wal.Open{Height: 2, OpenTimeMs: uint64(now.Add(time.Second).UnixMilli()), BaseFee: uint256.NewInt(0)})
	require.NoError(t, err)
	source := seqnode.NewReceiptSource(8)
	metrics, err := seqnode.NewMetrics(otel.Meter("px-root-kernel"))
	require.NoError(t, err)
	accepter := seqnode.NewAccepter(receipter, replicator, source, metrics)
	to := common.HexToAddress("0x223404")
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: 0, GasPrice: big.NewInt(500_000_000_000), Gas: 21000, To: &to, Value: big.NewInt(1)}), ethtypes.LatestSignerForChainID(application.EvmKeeper.ChainID(application.GetCheckCtx())), sender.EvmPrivateKey)
	require.NoError(t, err)
	data, err := ethtx.NewTxDataFromTx(tx)
	require.NoError(t, err)
	message, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	builder := application.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(message))
	builder.SetGasLimit(21000)
	encoded, err := application.GetTxConfig().TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	acceptBank := func() {
		open, err := exec.Snapshot()
		require.NoError(t, err)
		acct := application.AccountKeeper.GetAccount(open, bankSender.AccountAddress)
		require.NotNil(t, acct)
		tc := application.GetTxConfig()
		bank := tc.NewTxBuilder()
		require.NoError(t, bank.SetMsgs(banktypes.NewMsgSend(bankSender.AccountAddress, sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5)))))
		bank.SetGasLimit(200_000)
		bank.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 30_000)))
		mode := tc.SignModeHandler().DefaultMode()
		require.NoError(t, bank.SetSignatures(signing.SignatureV2{PubKey: bankSender.PrivateKey.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: acct.GetSequence()}))
		sig, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{ChainID: genesis.ChainID, AccountNumber: acct.GetAccountNumber(), Sequence: acct.GetSequence()}, bank, bankSender.PrivateKey, tc, acct.GetSequence())
		require.NoError(t, err)
		require.NoError(t, bank.SetSignatures(sig))
		raw, err := tc.TxEncoder()(bank.GetTx())
		require.NoError(t, err)
		_, err = accepter.Accept(raw)
		require.NoError(t, err)
	}
	if mixedBlock {
		acceptBank()
	}
	accepted, err := accepter.Accept(encoded)
	require.NoError(t, err)
	require.NotEmpty(t, accepted.Receipt)
	if mixedBlock {
		acceptBank()
	}
	consensusSource, err := seqnode.NewConsensusSource(application, exec, key)
	require.NoError(t, err)
	sealedBlock, err := consensusSource.Seal(ctx, 2)
	require.NoError(t, err)
	sealed := consensusSource.Sealed()
	require.NotNil(t, sealed)
	if mixedBlock {
		require.Len(t, sealed.Txs, 4)
	} else {
		require.Len(t, sealed.Txs, 2)
	}
	require.NoError(t, application.SetSealedStateToCommit(sealed.Context(), sealed.Context().MultiStore().(sdk.CacheMultiStore)))
	_, err = application.Commit(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), application.LastBlockHeight())
	source.Committed(2)
	consensusApp := &pxRootKernelConsensusApp{Application: kvstore.NewApplication(), ConsensusSource: consensusSource}
	consensusApp.SetValidators(genesis.ValidatorUpdates())
	node, err := tmnode.New(ctx, config, func() {}, consensusApp, nil, nil, tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	consensus, ok := node.RPCEnvironment().ConsensusState.Get()
	require.True(t, ok)
	signing, ok := consensus.(interface {
		SetPrivValidator(context.Context, tmutils.Option[tmtypes.PrivValidator])
	})
	require.True(t, ok)
	require.NoError(t, node.Start(ctx))
	t.Cleanup(func() { cancel(); node.Wait(); require.NoError(t, consensusApp.Close()) })
	require.NoError(t, rpctest.WaitForRPC(ctx, config))
	tm, err := rpclocal.New(node)
	require.NoError(t, err)
	signing.SetPrivValidator(ctx, tmutils.Some[tmtypes.PrivValidator](validator))
	// This isolated genesis network has no peers to sync from. Start consensus
	// from its initialized genesis state while retaining the full validator set.
	reactor, ok := node.RPCEnvironment().ConsensusReactor.Get()
	require.True(t, ok)
	reactor.SwitchToConsensus(consensus.GetState(), false)
	require.Eventually(t, func() bool {
		block, err := tm.Block(ctx, ptrPxHeight(2))
		return err == nil && block != nil && block.Block != nil
	}, 20*time.Second, 20*time.Millisecond)
	block, err := tm.Block(ctx, ptrPxHeight(2))
	require.NoError(t, err)
	require.Equal(t, sealed.Txs, block.Block.Txs.ToSliceOfBytes())
	root := &pxRootAccountHarness{app: application, tm: tm, height: 2}
	root.provider = func(height int64) sdk.Context {
		if height <= 0 {
			height = root.height
		}
		store, err := application.CommitMultiStore().CacheMultiStoreWithVersion(height)
		if err != nil {
			panic(err)
		}
		read := sdk.NewContext(store, tmproto.Header{ChainID: genesis.ChainID, Height: height}, false).WithClosestUpgradeName(LatestCtxUpgradeName).WithIsTracing(true)
		return read.WithConsensusParams(application.GetConsensusParams(read))
	}
	h := &pxRootKernelHarness{pxRootAccountHarness: root, source: source, pins: pins, tx: tx, batch: sealedBlock.Batch}
	validators := make([]*tmtypes.Validator, len(genesis.Validators))
	for i, v := range genesis.Validators {
		validators[i] = tmtypes.NewValidator(v.PubKey, v.Power)
	}
	for _, v := range tmtypes.NewValidatorSet(validators).Validators {
		h.trust = append(h.trust, map[string]interface{}{"key": hexutil.Encode(v.PubKey.Bytes()), "power": v.VotingPower})
	}
	root.start(t, h.receiptOption())
	return h
}

func ptrPxHeight(height int64) *int64 { return &height }
func (h *pxRootKernelHarness) receiptOption() evmrpc.SequencerReceiptOption {
	return evmrpc.SequencerReceiptOption{Source: h.source, Keeper: &h.app.SequencerKeeper}
}
func (h *pxRootKernelHarness) restart(t *testing.T, opts ...evmrpc.ServerOption) {
	t.Helper()
	for _, c := range h.clients {
		c.Close()
	}
	h.http.Stop()
	h.ws.Stop()
	h.start(t, opts...)
}

func pxKernelCall(t *testing.T, c *rpc.Client, method string, args ...interface{}) map[string]interface{} {
	t.Helper()
	if args == nil {
		args = []interface{}{}
	}
	return pxRootCall(t, c, method, args...)
}

func pxKernelError(t *testing.T, c *rpc.Client, method string, code int, message, dataCode string, args ...interface{}) {
	t.Helper()
	var result interface{}
	err := c.CallContext(t.Context(), &result, method, args...)
	require.Error(t, err)
	var rpcErr rpc.Error
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, code, rpcErr.ErrorCode(), "%s: %v", method, err)
	if message != "" {
		require.Equal(t, message, err.Error())
	}
	if dataCode != "" {
		var dataErr rpc.DataError
		require.ErrorAs(t, err, &dataErr)
		data, ok := dataErr.ErrorData().(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, dataCode, data["code"])
		if code == -32004 {
			require.Equal(t, method, data["method"])
		}
	}
}

func TestPxNodeRoot_Kernel_CommittedReadsAndParity(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	latency, err := provider.Meter("evmrpc").Float64Histogram("px_kernel_latency")
	require.NoError(t, err)
	restore := evmrpc.SwapRequestLatencyForTest(latency)
	t.Cleanup(restore)
	h := newPxRootKernelHarness(t)
	names := []string{"px_getCommit", "px_getValidators", "px_getReceiptKeys", "px_getSequenceBatch", "pax_sequencerReceipt", "px_register", "px_requestFunds"}
	for _, name := range names {
		require.Contains(t, h.http.HTTPMethods(), name)
		require.Contains(t, h.ws.WSMethods(), name)
	}
	var previous []map[string]interface{}
	for _, c := range h.clients {
		commit := pxKernelCall(t, c, "px_getCommit", "0002")
		require.Equal(t, "2", commit["height"])
		localCommit, err := h.tm.Commit(t.Context(), ptrPxHeight(2))
		require.NoError(t, err)
		localJSON, err := json.Marshal(localCommit.SignedHeader)
		require.NoError(t, err)
		var expected map[string]interface{}
		require.NoError(t, json.Unmarshal(localJSON, &expected))
		require.Equal(t, expected, commit["signedHeader"])
		validators := pxKernelCall(t, c, "px_getValidators", "2")
		require.Equal(t, "101", validators["total"])
		require.Len(t, validators["validators"], 101)
		local, err := h.tm.Validators(t.Context(), ptrPxHeight(2), nil, ptrPxInt(100))
		require.NoError(t, err)
		page, err := h.tm.Validators(t.Context(), ptrPxHeight(2), ptrPxInt(2), ptrPxInt(100))
		require.NoError(t, err)
		all := append(local.Validators, page.Validators...)
		set := tmtypes.NewValidatorSet(all)
		require.NoError(t, set.VerifyCommitLight("pax-test", localCommit.Commit.BlockID, 2, localCommit.Commit))
		for i, item := range validators["validators"].([]interface{}) {
			value := item.(map[string]interface{})
			require.Equal(t, all[i].Address.String(), value["address"])
			require.Equal(t, strconv.FormatInt(all[i].VotingPower, 10), value["votingPower"])
			wire, err := json.Marshal(all[i])
			require.NoError(t, err)
			var expected map[string]interface{}
			require.NoError(t, json.Unmarshal(wire, &expected))
			require.Equal(t, expected["pub_key"], value["pubKey"])
		}
		keys := pxKernelCall(t, c, "px_getReceiptKeys")
		var typedKeys evmrpc.ReceiptKeysResult
		require.NoError(t, c.CallContext(t.Context(), &typedKeys, "px_getReceiptKeys", []interface{}{}...))
		require.Len(t, typedKeys.Keys, 1)
		require.Equal(t, h.pins[0].KeyId, typedKeys.Keys[0].KeyID.Bytes())
		require.Equal(t, h.pins[0].PublicKey, []byte(typedKeys.Keys[0].PublicKey))
		require.Equal(t, uint64(math.MaxUint64), typedKeys.Keys[0].LastHeight)
		batch := pxKernelCall(t, c, "px_getSequenceBatch", "2")
		require.Equal(t, hexutil.Encode(h.batch), batch["tx"])
		localBlock, err := h.tm.Block(t.Context(), ptrPxHeight(2))
		require.NoError(t, err)
		proof := localBlock.Block.Txs.Proof(0)
		p := batch["proof"].(map[string]interface{})
		require.Equal(t, float64(2), p["total"])
		require.Equal(t, float64(0), p["index"])
		require.Equal(t, hexutil.Encode(proof.Proof.LeafHash), p["leafHash"])
		require.Len(t, p["aunts"], 1)
		require.Equal(t, hexutil.Encode(proof.Proof.Aunts[0]), p["aunts"].([]interface{})[0])
		require.NoError(t, proof.Validate(localBlock.Block.DataHash))
		decoded, err := seqtypes.DecodeSequenceBatchTx(h.batch)
		require.NoError(t, err)
		require.NoError(t, decoded.VerifySignature("pax-test", [32]byte(h.pins[0].PublicKey)))
		require.NoError(t, seqtypes.VerifyBatchTxs(decoded, [][32]byte{[32]byte(h.tx.Hash())}))
		receipt := pxKernelCall(t, c, "pax_sequencerReceipt", h.tx.Hash())
		require.Equal(t, "committed", receipt["rung"])
		require.Equal(t, "0x2", receipt["blockNumber"])
		require.Equal(t, common.BytesToHash(localBlock.BlockID.Hash).Hex(), receipt["blockHash"])
		var typedReceipt evmrpc.SequencerReceiptResult
		require.NoError(t, c.CallContext(t.Context(), &typedReceipt, "pax_sequencerReceipt", h.tx.Hash()))
		require.NoError(t, typedReceipt.Verify(h.pins))
		var unknown interface{}
		require.NoError(t, c.CallContext(t.Context(), &unknown, "pax_sequencerReceipt", common.HexToHash("0xdead")))
		require.Nil(t, unknown)
		pxRootAssertSchema(t, "pax_sequencerReceipt", unknown)
		pxKernelError(t, c, "px_register", -32004, "Hosted gateway service operation", "hosted_gateway_service_required", strings.Repeat("ab", 32), strings.Repeat("cd", 64))
		pxKernelError(t, c, "px_requestFunds", -32004, "Hosted gateway service operation", "hosted_gateway_service_required", "did:layerx:kernel", strings.Repeat("ab", 32))
		current := []map[string]interface{}{commit, validators, keys, batch, receipt}
		if previous != nil {
			require.Equal(t, previous, current)
		}
		previous = current
	}
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &collected))
	counts := make(map[string]uint64)
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			for _, point := range histogram.DataPoints {
				endpoint, _ := point.Attributes.Value(attribute.Key("endpoint"))
				connection, _ := point.Attributes.Value(attribute.Key("connection"))
				counts[endpoint.AsString()+"/"+connection.AsString()] += point.Count
			}
		}
	}
	for _, name := range names {
		for _, connection := range []string{"http", "websocket"} {
			require.Positive(t, counts[name+"/"+connection])
		}
	}
}
func ptrPxInt(value int) *int { return &value }

func TestPxNodeRoot_Kernel_ParamsAndUnavailable(t *testing.T) {
	h := newPxRootKernelHarness(t)
	h.restart(t)
	for _, c := range h.clients {
		// Ordinary nodes still read local governed pins, independently of receipts.
		require.Len(t, pxKernelCall(t, c, "px_getReceiptKeys")["keys"], 1)
		pxKernelError(t, c, "pax_sequencerReceipt", -32000, "pax_sequencerReceipt served by sequencer", "", h.tx.Hash())
		for _, method := range []string{"px_getCommit", "px_getValidators", "px_getSequenceBatch"} {
			for _, args := range [][]interface{}{nil, {nil}, {1}, {""}, {"0"}, {"-1"}, {"+2"}, {"0x2"}, {" 2"}, {"2.0"}, {"18446744073709551616"}, {"2", "extra"}} {
				pxKernelError(t, c, method, -32602, "", "", args...)
			}
			for _, height := range []string{"18446744073709551615", "99999999", "1"} {
				pxKernelError(t, c, method, -32003, "Chain read unavailable", "consensus_unavailable", height)
			}
		}
		for _, args := range [][]interface{}{nil, {nil}, {"bad"}, {"0x12"}, {strings.Repeat("ab", 32)}, {h.tx.Hash(), "extra"}} {
			pxKernelError(t, c, "pax_sequencerReceipt", -32602, "", "", args...)
		}
		for _, args := range [][]interface{}{nil, {"key"}, {strings.Repeat("AB", 32), strings.Repeat("cd", 64)}, {strings.Repeat("ab", 32), "bad"}, {strings.Repeat("ab", 32), strings.Repeat("cd", 64), "extra"}} {
			pxKernelError(t, c, "px_register", -32602, "", "", args...)
		}
		for _, args := range [][]interface{}{nil, {"did:layerx:kernel"}, {"bad/did", strings.Repeat("ab", 32)}, {"did:layerx:kernel", strings.Repeat("00", 32)}, {"did:layerx:kernel", strings.Repeat("AB", 32)}, {"did:layerx:kernel", strings.Repeat("ab", 32), "extra"}} {
			pxKernelError(t, c, "px_requestFunds", -32602, "", "", args...)
		}
	}
	// Exercise explicit-empty params on both raw transports, including a mixed
	// batch with an unrelated read and an invalid notification.
	for _, params := range []string{"", `,"params":null`, `,"params":{}`, `,"params":[null]`, `,"params":[1]`} {
		request := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"px_getReceiptKeys"%s}`, params)
		for _, ws := range []bool{false, true} {
			raw := pxKernelWire(t, h, ws, request)
			var result struct {
				Error struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(raw, &result))
			require.Equal(t, -32602, result.Error.Code)
			require.Equal(t, "Invalid params", result.Error.Message)
		}
	}
	mixed := `[{"jsonrpc":"2.0","id":1,"method":"px_getReceiptKeys","params":[]},{"jsonrpc":"2.0","id":2,"method":"px_getReceiptKeys"},{"jsonrpc":"2.0","id":3,"method":"px_getCommit","params":["2"]},{"jsonrpc":"2.0","method":"px_getReceiptKeys","params":null}]`
	for _, ws := range []bool{false, true} {
		var responses []struct {
			ID    int `json:"id"`
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		require.NoError(t, json.Unmarshal(pxKernelWire(t, h, ws, mixed), &responses))
		require.Len(t, responses, 3)
		for _, response := range responses {
			if response.ID == 2 {
				require.NotNil(t, response.Error)
				require.Equal(t, -32602, response.Error.Code)
			} else {
				require.Nil(t, response.Error)
				require.NotEmpty(t, response.Result)
			}
		}
	}
	require.Eventually(t, func() bool {
		s, err := h.tm.Status(t.Context())
		return err == nil && s.SyncInfo.LatestBlockHeight >= 3
	}, 20*time.Second, 20*time.Millisecond)
	for _, c := range h.clients {
		pxKernelError(t, c, "px_getSequenceBatch", -32602, "Invalid params", "no_sequence_batch", "3")
	}
	// A malformed batch travels through real consensus storage and proof creation.
	status, err := h.tm.Status(t.Context())
	require.NoError(t, err)
	bad := append([]byte(seqtypes.SequenceBatchDomain), []byte("malformed")...)
	response, err := h.tm.BroadcastTxCommit(t.Context(), bad)
	require.NoError(t, err)
	require.Greater(t, response.Height, status.SyncInfo.LatestBlockHeight)
	for _, c := range h.clients {
		pxKernelError(t, c, "px_getSequenceBatch", -32003, "Chain read unavailable", "invalid_consensus_response", strconv.FormatInt(response.Height, 10))
	}
}

func pxKernelWire(t *testing.T, h *pxRootKernelHarness, ws bool, body string) []byte {
	t.Helper()
	if ws {
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+h.ws.ListenAddr()+"/", nil)
		require.NoError(t, err)
		defer conn.Close()
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(body)))
		_, raw, err := conn.ReadMessage()
		require.NoError(t, err)
		return raw
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+h.http.ListenAddr()+"/", strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return raw
}

func TestPxNodeRoot_Kernel_RestartAndGovernedKeys(t *testing.T) {
	h := newPxRootKernelHarness(t)
	before := pxKernelCall(t, h.clients[0], "px_getReceiptKeys")
	beforeCommit := pxKernelCall(t, h.clients[0], "px_getCommit", "2")
	beforeBatch := pxKernelCall(t, h.clients[1], "px_getSequenceBatch", "2")
	beforeReceipt := pxKernelCall(t, h.clients[1], "pax_sequencerReceipt", h.tx.Hash())
	h.restart(t, h.receiptOption())
	for _, c := range h.clients {
		require.Equal(t, before, pxKernelCall(t, c, "px_getReceiptKeys"))
		require.Equal(t, beforeCommit, pxKernelCall(t, c, "px_getCommit", "2"))
		require.Equal(t, beforeBatch, pxKernelCall(t, c, "px_getSequenceBatch", "2"))
		require.Equal(t, beforeReceipt, pxKernelCall(t, c, "pax_sequencerReceipt", h.tx.Hash()))
	}
	// The forwarded compatibility alias still verifies the actual signed output.
	endpoint, err := url.Parse("http://" + h.http.ListenAddr() + "/")
	require.NoError(t, err)
	forwarding := &pxRootAccountHarness{app: h.app, tm: h.tm, provider: h.provider, height: 2}
	forwarding.start(t, evmrpc.SequencerReceiptOption{ForwardURL: endpoint})
	for _, c := range forwarding.clients {
		require.Equal(t, beforeReceipt, pxKernelCall(t, c, "pax_sequencerReceipt", h.tx.Hash()))
	}
	// A deliberate empty governed list and a missing store entry are distinct.
	write := sdk.NewContext(h.app.CommitMultiStore(), tmproto.Header{ChainID: "pax-test", Height: 3}, false)
	require.NoError(t, h.app.SequencerKeeper.SetReceiptKeys(write, seqkeeper.ReceiptKeys{}))
	require.Equal(t, int64(3), h.app.CommitMultiStore().Commit(true).Version)
	h.height = 3
	for _, c := range h.clients {
		require.Empty(t, pxKernelCall(t, c, "px_getReceiptKeys")["keys"])
	}
	write.KVStore(h.app.GetKey(seqtypes.StoreKey)).Delete(seqtypes.ReceiptKeysKey)
	require.Equal(t, int64(4), h.app.CommitMultiStore().Commit(true).Version)
	h.height = 4
	h.restart(t)
	for _, c := range h.clients {
		pxKernelError(t, c, "px_getReceiptKeys", -32003, "Chain read unavailable", "receipt_keys_unavailable")
	}
	// Missing authority takes precedence over parameter validation on both
	// transports, including the explicit empty form used before the restart.
	for _, params := range []string{"", `,"params":[]`, `,"params":null`, `,"params":{}`, `,"params":[null]`, `,"params":[1]`} {
		request := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"px_getReceiptKeys"%s}`, params)
		for _, ws := range []bool{false, true} {
			var response struct {
				Error struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Data    struct {
						Code string `json:"code"`
					} `json:"data"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(pxKernelWire(t, h, ws, request), &response))
			require.Equal(t, -32003, response.Error.Code)
			require.Equal(t, "Chain read unavailable", response.Error.Message)
			require.Equal(t, "receipt_keys_unavailable", response.Error.Data.Code)
		}
	}
	require.NoError(t, h.app.SequencerKeeper.SetReceiptKeys(write, h.pins))
	require.Equal(t, int64(5), h.app.CommitMultiStore().Commit(true).Version)
	h.height = 5
	for _, c := range h.clients {
		require.Equal(t, before, pxKernelCall(t, c, "px_getReceiptKeys"))
	}
}

func TestCommittedWaitUsesBatchActivityIDs(t *testing.T) {
	h := newPxRootKernelHarness(t, true)
	block, err := h.tm.Block(t.Context(), ptrPxHeight(2))
	require.NoError(t, err)
	require.Len(t, block.Block.Txs, 4)
	expected := make([]interface{}, 3)
	typed := make([]sdk.Tx, 3)
	for i, raw := range block.Block.Txs[1:] {
		typed[i], err = h.app.GetTxConfig().TxDecoder()(raw)
		require.NoError(t, err)
		id := sha256.Sum256(raw)
		if i == 1 {
			id = [32]byte(h.tx.Hash())
		} else {
			require.Nil(t, h.app.GetEVMMsg(typed[i]))
		}
		expected[i] = hexutil.Encode(id[:])
	}
	require.Equal(t, seqtypes.TxRoot([][32]byte{sha256.Sum256(block.Block.Txs[1]), [32]byte(h.tx.Hash()), sha256.Sum256(block.Block.Txs[3])}), h.app.SequencedTxRoot(block.Block.Txs[1:].ToSliceOfBytes(), typed))
	decoded, err := seqtypes.DecodeSequenceBatchTx(h.batch)
	require.NoError(t, err)
	for i, c := range append(h.clients, domainHTTPClient(t, h.http, "/chain/1"), domainWSClient(t, h.ws, "/chain/1")) {
		batch := pxKernelCall(t, c, "px_getSequenceBatch", "2")
		require.Equal(t, expected, batch["activityIds"])
		receipt := pxKernelCall(t, c, "pax_sequencerReceipt", h.tx.Hash())
		require.Equal(t, "committed", receipt["rung"])
		var view map[string]interface{}
		require.NoError(t, c.CallContext(t.Context(), &view, "eth_getBlockByNumber", "0x2", false))
		require.NotNil(t, view)
		transactions := view["transactions"].([]interface{})
		require.NotContains(t, transactions, expected[0])
		require.NotContains(t, transactions, expected[2])
		if i < 2 {
			require.Equal(t, []interface{}{h.tx.Hash().Hex()}, transactions)
		} else {
			require.Empty(t, transactions)
		}
	}
	endpoint := "http://" + h.http.ListenAddr() + "/"
	target, err := url.Parse(endpoint)
	require.NoError(t, err)
	endpoints := map[string]string{"genuine": endpoint}
	for _, mode := range []string{"missing", "truncated", "reordered", "altered", "malformed", "short_eth"} {
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(response *http.Response) error {
			method := response.Request.Header.Get("X-Issue-1764-Method")
			if method != "px_getSequenceBatch" && !(mode == "short_eth" && method == "eth_getBlockByNumber") {
				return nil
			}
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			response.Body.Close()
			var wire map[string]interface{}
			if err := json.Unmarshal(raw, &wire); err != nil {
				return err
			}
			result, ok := wire["result"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("production response has no result: %s", raw)
			}
			if method == "eth_getBlockByNumber" {
				result["transactions"] = []interface{}{}
			} else {
				ids := result["activityIds"].([]interface{})
				switch mode {
				case "missing":
					delete(result, "activityIds")
				case "truncated":
					result["activityIds"] = ids[:len(ids)-1]
				case "reordered":
					ids[0], ids[2] = ids[2], ids[0]
				case "altered":
					ids[2] = common.HexToHash("0xbad").Hex()
				case "malformed":
					ids[0] = "0x01"
				}
			}
			encoded, err := json.Marshal(wire)
			if err != nil {
				return err
			}
			response.Body = io.NopCloser(bytes.NewReader(encoded))
			response.ContentLength = int64(len(encoded))
			response.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
			return nil
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			var request struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal(raw, &request); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			r.Header.Set("X-Issue-1764-Method", request.Method)
			proxy.ServeHTTP(w, r)
		}))
		t.Cleanup(server.Close)
		endpoints[mode] = server.URL
	}
	config := map[string]interface{}{"endpoints": endpoints, "validators": h.trust, "chainId": "pax-test", "activity": h.tx.Hash().Hex(), "keyId": hexutil.Encode(h.pins[0].KeyId), "receiptKey": hexutil.Encode(h.pins[0].PublicKey), "height": 2, "fromSequence": decoded.FromSeq, "targetSequence": decoded.FromSeq + 1, "blockHash": hexutil.Encode(block.BlockID.Hash)}
	path := filepath.Join(t.TempDir(), "issue-1764.json")
	encoded, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encoded, 0600))
	command := exec.CommandContext(t.Context(), "cargo", "test", "--locked", "--manifest-path", "../services/agent/crates/layerx-sdk/Cargo.toml", "--test", "issue_1764", "committed_wait_uses_unfiltered_batch_activity_ids", "--", "--exact", "--nocapture")
	command.Env = append(os.Environ(), "ISSUE_1764_CONFIG="+path)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Contains(t, string(output), "1 passed", "%s", output)
	t.Logf("%s", output)
}
