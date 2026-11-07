package evmrpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

const (
	sendSeqChainID      = "pax-test"
	sendSeqCommittedTip = int64(1)
	sendSeqOpenHeight   = int64(2)
	sendSeqModuleID     = uint16(12)
	sendSeqFundingUhpx  = 1_000_000_000_000
	sendSeqGasPriceWei  = 500_000_000_000
	sendSeqTransferGas  = 21_000
	sendSeqGasLimit     = 100_000
)

// sendSeqBroadcastClient counts every attempt to place a transaction in the local mempool.
type sendSeqBroadcastClient struct {
	*MockClient
	mu         sync.Mutex
	broadcasts int
	proxyURL   *url.URL
}

func (c *sendSeqBroadcastClient) BroadcastTx(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	c.mu.Lock()
	c.broadcasts++
	c.mu.Unlock()
	return c.MockClient.BroadcastTx(ctx, tx)
}

func (c *sendSeqBroadcastClient) BroadcastTxCommit(context.Context, tmtypes.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	c.mu.Lock()
	c.broadcasts++
	c.mu.Unlock()
	return &coretypes.ResultBroadcastTxCommit{}, nil
}

func (c *sendSeqBroadcastClient) EvmProxy(common.Address) (*url.URL, bool) {
	return c.proxyURL, c.proxyURL != nil
}

func (c *sendSeqBroadcastClient) broadcastCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.broadcasts
}

// sendSeqReceipter exposes the real Receipter as the send path's SequencerAccepter and keeps
// every outcome for the assertions.
type sendSeqReceipter struct {
	r        *sequencer.Receipter
	txBytes  [][]byte
	accepted []*sequencer.Accepted
	receipts [][]byte
}

func (s *sendSeqReceipter) Accept(txBytes []byte) (evmrpc.AcceptedSend, error) {
	s.txBytes = append(s.txBytes, txBytes)
	acc, signed, err := s.r.Accept(txBytes)
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	s.accepted = append(s.accepted, acc)
	s.receipts = append(s.receipts, signed)
	exec, err := sendSeqExecutionReceipt(acc)
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	return evmrpc.AcceptedSend{Receipt: signed, Height: acc.Height, Exec: exec}, nil
}

// sendSeqExecutionReceipt reads the real execution output retained by the accepter.
func sendSeqExecutionReceipt(acc *sequencer.Accepted) (*evmtypes.Receipt, error) {
	for _, key := range acc.ReceiptContext.MultiStore().StoreKeys() {
		if key.Name() != evmtypes.TransientStoreKey {
			continue
		}
		encoded := acc.ReceiptContext.TransientStore(key).Get(evmtypes.NewTransientReceiptKey(uint64(acc.TxIndex), common.Hash(acc.ActivityID)))
		if encoded == nil {
			return nil, fmt.Errorf("execution receipt missing at height %d index %d", acc.Height, acc.TxIndex)
		}
		receipt := &evmtypes.Receipt{}
		if err := receipt.Unmarshal(encoded); err != nil {
			return nil, err
		}
		return receipt, nil
	}
	return nil, fmt.Errorf("execution receipt store missing")
}

type sendSeqHarness struct {
	app      *app.App
	exec     sequencer.Executor
	key      *sequencer.ReceiptKey
	accepter *sendSeqReceipter
	client   *sendSeqBroadcastClient
	api      *evmrpc.SendAPI
	ctxReads int
}

func sendSeqLabel(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

// newSendSeqHarness builds the real App, funds the given EVM-mapped accounts, commits height 1,
// opens height 2 on a Sequential executor and serves eth_sendRawTransaction through a Receipter
// signing with a receipt key pinned for heights 1..1000.
func newSendSeqHarness(t *testing.T, funded ...occutils.TestAcct) *sendSeqHarness {
	t.Helper()
	tm := time.Now().UTC()
	valPub := secp256k1.GenPrivKey().PubKey()
	tw := app.NewTestWrapper(t, tm, valPub, false)
	a := tw.App
	params := a.GetConsensusParams(tw.Ctx)
	params.Block.MaxGasWanted = 50_000_000
	a.StoreConsensusParams(tw.Ctx, params)
	for _, acct := range funded {
		a.EvmKeeper.SetAddressMapping(tw.Ctx, acct.AccountAddress, acct.EvmAddress)
		tw.FundAcc(acct.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(sendSeqFundingUhpx))))
	}
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: sendSeqChainID, Height: sendSeqCommittedTip},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)

	exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: sendSeqChainID, ProposerAddress: valPub.Address().Bytes()})
	require.NoError(t, err)
	require.NoError(t, exec.Open(sendSeqOpenHeight, tm.Add(time.Second).UnixMilli()))

	dir := t.TempDir()
	consensusPriv, err := tmed25519.SecretKeyFromSecretBytes(ed25519.NewKeyFromSeed(sendSeqLabel("send sequencer consensus key")))
	require.NoError(t, err)
	consensusFile := filepath.Join(dir, "priv_validator_key.json")
	require.NoError(t, privval.NewFilePV(consensusPriv, consensusFile, filepath.Join(dir, "priv_validator_state.json")).Key.Save())
	seed := sendSeqLabel("send sequencer receipt key")
	keyFile := filepath.Join(dir, "receipt_key")
	require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seed)+"\n"), 0o600))
	require.NoError(t, os.Chmod(keyFile, 0o600))
	key, err := sequencer.LoadReceiptKey(keyFile, consensusFile, []verify.SequencerAuthorization{{
		SequencerID:      [32]byte(sendSeqLabel("send sequencer key id")),
		PublicKey:        [32]byte(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)),
		FirstBatchNumber: 1,
		LastBatchNumber:  1000,
	}}, sendSeqOpenHeight)
	require.NoError(t, err)
	r, err := sequencer.NewReceipter(exec, a, &a.EvmKeeper, key, sendSeqModuleID)
	require.NoError(t, err)

	h := &sendSeqHarness{
		app:      a,
		exec:     exec,
		key:      key,
		accepter: &sendSeqReceipter{r: r},
		client:   &sendSeqBroadcastClient{MockClient: &MockClient{}},
	}
	h.api = evmrpc.NewSendAPI(
		h.client,
		func(int64) client.TxConfig { return a.GetTxConfig() },
		&evmrpc.SendConfig{Sequencer: h.accepter},
		&a.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) sdk.Context {
			h.ctxReads++
			return a.GetContextForDeliverTx([]byte{})
		},
		"",
		nil,
		nil,
		nil,
		evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](),
		evmrpc.NewBlockCache(1),
		nil,
		nil,
	)
	return h
}

func (h *sendSeqHarness) openBalance(t *testing.T, addr common.Address) *big.Int {
	t.Helper()
	ctx, err := h.exec.Snapshot()
	require.NoError(t, err)
	return h.app.EvmKeeper.GetBalance(ctx, h.app.EvmKeeper.GetPaxAddressOrDefault(ctx, addr))
}

func sendSeqTransfer(t *testing.T, from occutils.TestAcct, nonce uint64, to common.Address, value *big.Int, gas uint64) (*ethtypes.Transaction, hexutil.Bytes) {
	t.Helper()
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(sendSeqGasPriceWei),
		Gas:      gas,
		To:       &to,
		Value:    value,
	}), ethtypes.LatestSignerForChainID(big.NewInt(config.DefaultChainID)), from.EvmPrivateKey)
	require.NoError(t, err)
	raw, err := signed.MarshalBinary()
	require.NoError(t, err)
	return signed, raw
}

func (h *sendSeqHarness) verifiedReceipt(t *testing.T, i int) *codec.Receipt {
	t.Helper()
	verified, err := verify.ReceiptSignature(h.accepter.receipts[i], h.key.PublicKey())
	require.NoError(t, err)
	require.Equal(t, h.accepter.accepted[i].Seq, verified.Receipt.GlobalSequence)
	require.Equal(t, h.accepter.accepted[i].SeqHash, verified.Receipt.ResultingStateRoot)
	return verified.Receipt
}

type sendSeqRecordedAccepter struct {
	accepter *sequencer.Accepter
	source   *sequencer.ReceiptSource
	calls    int
}

func (s *sendSeqRecordedAccepter) Accept(txBytes []byte) (evmrpc.AcceptedSend, error) {
	s.calls++
	return s.accepter.Accept(txBytes)
}

func (s *sendSeqRecordedAccepter) SequencerSend(hash common.Hash) (*evmrpc.SequencerSend, error) {
	return s.source.SequencerSend(hash)
}

func TestSendRawTransactionIdenticalResubmissionKeepsHashAndReceipt(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	w, err := wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	replicator, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{WAL: w, Standalone: true})
	require.NoError(t, err)
	t.Cleanup(replicator.Close)
	_, err = replicator.Release(context.Background(), &wal.Open{
		Height: uint64(sendSeqOpenHeight), OpenTimeMs: uint64(time.Now().UnixMilli()), BaseFee: uint256.NewInt(0),
	})
	require.NoError(t, err)
	metrics, err := sequencer.NewMetrics(otel.Meter("send-identical-resubmission"))
	require.NoError(t, err)
	source := sequencer.NewReceiptSource(16)
	accepter := &sendSeqRecordedAccepter{
		accepter: sequencer.NewAccepter(h.accepter.r, replicator, source, metrics),
		source:   source,
	}
	keyCtx := sdk.NewContext(h.app.CommitMultiStore().CacheMultiStore(), tmproto.Header{ChainID: sendSeqChainID, Height: sendSeqCommittedTip}, false)
	publicKey := h.key.PublicKey()
	keyID, err := h.key.KeyID(sendSeqOpenHeight)
	require.NoError(t, err)
	pins := seqkeeper.ReceiptKeys{{
		KeyId: keyID[:], PublicKey: publicKey[:], FirstHeight: 1, LastHeight: 1000,
	}}
	require.NoError(t, h.app.SequencerKeeper.SetReceiptKeys(keyCtx, pins))
	ctxProvider := func(int64) sdk.Context {
		ctx, _ := keyCtx.CacheContext()
		return ctx
	}
	cfg := evmrpcconfig.DefaultConfig
	cfg.HTTPAddress = TestAddr
	cfg.HTTPPort = seqReceiptFreePort9005(t)
	server, err := evmrpc.NewEVMHTTPServer(cfg, h.client, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp,
		h.app.TracerAnteHandler, ctxProvider, func(int64) client.TxConfig { return h.app.GetTxConfig() }, "", nil,
		evmrpc.NewSequencerOption(accepter, evmrpc.NewReceiptLogFeed(sendSeqCommittedTip)),
		evmrpc.SequencerReceiptOption{Source: accepter, Keeper: &h.app.SequencerKeeper})
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)

	to := common.BytesToAddress(sendSeqLabel("identical resubmission recipient")[:20])
	value := big.NewInt(1_000_000)
	tx, raw := sendSeqTransfer(t, sender, 0, to, value, sendSeqTransferGas)
	result, rpcErr := callSeqReceipt9005(t, cfg.HTTPPort, "eth_sendRawTransaction", raw)
	require.Nil(t, rpcErr)
	var hash common.Hash
	require.NoError(t, json.Unmarshal(result, &hash))
	require.Equal(t, tx.Hash(), hash)
	_, before, beforeJSON := seqReceiptResult9005(t, cfg.HTTPPort, hash)
	require.NotNil(t, before)
	require.Equal(t, evmrpc.RungReceipted, before.Rung)
	require.Equal(t, hash, before.TxHash)
	require.Equal(t, hexutil.Uint64(0), before.Nonce)
	require.Equal(t, hexutil.Uint64(1), before.Seq)
	require.NoError(t, before.Verify(pins))
	require.Equal(t, 1, accepter.calls)
	snapshot, err := h.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(1), h.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress))
	balance := h.openBalance(t, sender.EvmAddress)

	result, rpcErr = callSeqReceipt9005(t, cfg.HTTPPort, "eth_sendRawTransaction", raw)
	require.Nil(t, rpcErr)
	var resubmittedHash common.Hash
	require.NoError(t, json.Unmarshal(result, &resubmittedHash))
	require.Equal(t, hash, resubmittedHash)
	_, after, afterJSON := seqReceiptResult9005(t, cfg.HTTPPort, hash)
	require.NotNil(t, after)
	require.Equal(t, beforeJSON, afterJSON)
	require.Equal(t, 1, accepter.calls)
	snapshot, err = h.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(1), h.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress))
	require.Equal(t, balance, h.openBalance(t, sender.EvmAddress))
	require.Equal(t, value, h.openBalance(t, to))

	distinct, distinctRaw := sendSeqTransfer(t, sender, 0, to, big.NewInt(2_000_000), sendSeqTransferGas)
	require.NotEqual(t, hash, distinct.Hash())
	_, rpcErr = callSeqReceipt9005(t, cfg.HTTPPort, "eth_sendRawTransaction", distinctRaw)
	require.NotNil(t, rpcErr)
	require.Equal(t, -32000, rpcErr.Code)
	require.Equal(t, ": "+sdkerrors.ErrWrongSequence.Error(), rpcErr.Message)
	require.Equal(t, 2, accepter.calls)
	unknown, err := source.SequencerSend(distinct.Hash())
	require.NoError(t, err)
	require.Nil(t, unknown)
	_, _, refusedJSON := seqReceiptResult9005(t, cfg.HTTPPort, hash)
	require.Equal(t, beforeJSON, refusedJSON)

	_, nextRaw := sendSeqTransfer(t, sender, 1, to, value, sendSeqTransferGas)
	result, rpcErr = callSeqReceipt9005(t, cfg.HTTPPort, "eth_sendRawTransaction", nextRaw)
	require.Nil(t, rpcErr)
	require.Equal(t, 3, accepter.calls)
	var nextHash common.Hash
	require.NoError(t, json.Unmarshal(result, &nextHash))
	_, next, _ := seqReceiptResult9005(t, cfg.HTTPPort, nextHash)
	require.NotNil(t, next)
	require.Equal(t, hexutil.Uint64(1), next.Nonce)
	require.Equal(t, hexutil.Uint64(2), next.Seq)
	require.NoError(t, next.Verify(pins))
	snapshot, err = h.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(2), h.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress))
	require.Zero(t, h.client.broadcastCount())
}

func TestSendSequencerReturnsAfterReceipt(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	watermarks := evmrpc.NewWatermarkManager(nil, nil, nil, nil)
	watermarks.SetPendingSnapshot(h.exec)
	watermarks.SetPendingOpenWait(50 * time.Millisecond)
	h.api = evmrpc.NewSendAPI(
		h.client,
		func(int64) client.TxConfig { return h.app.GetTxConfig() },
		&evmrpc.SendConfig{Sequencer: h.accepter},
		&h.app.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) sdk.Context { return h.app.GetContextForDeliverTx([]byte{}) },
		"",
		nil,
		nil,
		nil,
		evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](),
		evmrpc.NewBlockCache(1),
		nil,
		watermarks,
	)
	to := common.BytesToAddress(sendSeqLabel("send sequencer recipient")[:20])
	value := big.NewInt(1_000_000)

	etx, raw := sendSeqTransfer(t, sender, 0, to, value, sendSeqTransferGas)
	hash, err := h.api.SendRawTransaction(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, etx.Hash(), hash)

	require.Len(t, h.accepter.txBytes, 1)
	require.Len(t, h.accepter.receipts, 1)
	rec := h.verifiedReceipt(t, 0)
	require.Equal(t, [32]byte(etx.Hash()), rec.ActivityID)
	require.Equal(t, uint64(1), rec.GlobalSequence)
	require.Equal(t, [32]byte{}, rec.PreviousStateRoot)
	require.Equal(t, h.accepter.accepted[0].SeqHash, rec.ResultingStateRoot)
	require.Equal(t, int32(0), rec.ResultCode)
	require.Equal(t, sendSeqOpenHeight, h.accepter.accepted[0].Height)
	require.Equal(t, 1, h.accepter.accepted[0].TxIndex)
	require.Zero(t, value.Cmp(h.openBalance(t, to)))
	require.Zero(t, h.client.broadcastCount())

	// A refused transaction answers in the send path's ABCI error shape and takes no seq.
	_, err = h.api.SendRawTransaction(context.Background(), raw)
	require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
	require.EqualError(t, err, ": "+sdkerrors.ErrWrongSequence.Error())
	require.Len(t, h.accepter.txBytes, 2)
	require.Len(t, h.accepter.receipts, 1)

	etx, raw = sendSeqTransfer(t, sender, 1, to, value, sendSeqTransferGas)
	hash, err = h.api.SendRawTransaction(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, etx.Hash(), hash)
	require.Equal(t, uint64(2), h.verifiedReceipt(t, 1).GlobalSequence)
	require.Equal(t, h.accepter.accepted[0].SeqHash, h.verifiedReceipt(t, 1).PreviousStateRoot)

	// A sealed layer waits for the next open block and refuses without a receipt if none opens.
	_, err = h.exec.Seal(sendSeqOpenHeight)
	require.NoError(t, err)
	_, raw = sendSeqTransfer(t, sender, 2, to, value, sendSeqTransferGas)
	_, err = h.api.SendRawTransaction(context.Background(), raw)
	require.ErrorIs(t, err, evmrpc.ErrPendingBlockClosed)
	require.NotErrorIs(t, err, sequencer.ErrLayerNotOpen)
	require.EqualError(t, err, "no block is open; retry")
	require.Len(t, h.accepter.txBytes, 4)
	require.Len(t, h.accepter.accepted, 2)
	require.Len(t, h.accepter.receipts, 2)
	require.Zero(t, h.client.broadcastCount())
}

func TestSendSequencerUsesExecutedGas(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	to := common.BytesToAddress(sendSeqLabel("send sequencer gas recipient")[:20])

	_, raw := sendSeqTransfer(t, sender, 0, to, big.NewInt(1), sendSeqGasLimit)
	_, err := h.api.SendRawTransaction(context.Background(), raw)
	require.NoError(t, err)

	require.Len(t, h.accepter.txBytes, 1)
	decoded, err := h.app.GetTxConfig().TxDecoder()(h.accepter.txBytes[0])
	require.NoError(t, err)
	require.Equal(t, uint64(0), decoded.GetGasEstimate())

	require.Equal(t, int64(sendSeqTransferGas), h.accepter.accepted[0].Result.GasUsed)
	rec := h.verifiedReceipt(t, 0)
	require.NotEmpty(t, rec.Effects)
	require.Equal(t, uint64(sendSeqTransferGas), binary.BigEndian.Uint64(rec.Effects[0].Body[0:8]))
	require.Equal(t, uint64(sendSeqTransferGas), binary.BigEndian.Uint64(rec.Effects[0].Body[8:16]))
	require.Equal(t, uint64(sendSeqGasPriceWei), binary.BigEndian.Uint64(rec.Effects[0].Body[16:24]))
}

func TestSendSequencerSkipsLatestSimulation(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	to := common.BytesToAddress(sendSeqLabel("send sequencer simulation recipient")[:20])

	for nonce := uint64(0); nonce < 3; nonce++ {
		_, raw := sendSeqTransfer(t, sender, nonce, to, big.NewInt(1), sendSeqGasLimit)
		_, err := h.api.SendRawTransaction(context.Background(), raw)
		require.NoError(t, err)
	}
	require.Len(t, h.accepter.receipts, 3)
	require.Zero(t, h.ctxReads, "the sequencer send path read the latest state")
	require.Zero(t, h.client.broadcastCount())
}

func TestSendSequencerForwardsFromOtherNodes(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	to := common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314")
	sign := func(nonce uint64) (*ethtypes.Transaction, hexutil.Bytes) {
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
			ChainID:   big.NewInt(config.DefaultChainID),
			Nonce:     nonce,
			GasTipCap: big.NewInt(sendSeqGasPriceWei),
			GasFeeCap: big.NewInt(sendSeqGasPriceWei),
			Gas:       sendSeqTransferGas,
			To:        &to,
			Value:     big.NewInt(1000),
		}), ethtypes.LatestSignerForChainID(big.NewInt(config.DefaultChainID)), key)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		return signed, raw
	}
	first, firstRaw := sign(0)
	_, refusedRaw := sign(1)

	var mu sync.Mutex
	var methods, payloads []string
	sequencerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Len(t, req.Params, 1)
		var payload string
		require.NoError(t, json.Unmarshal(req.Params[0], &payload))
		mu.Lock()
		methods = append(methods, req.Method)
		payloads = append(payloads, payload)
		mu.Unlock()
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if payload == hexutil.Encode(refusedRaw) {
			resp["error"] = map[string]any{"code": -32000, "message": "nonce too high"}
		} else {
			resp["result"] = first.Hash().Hex()
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer sequencerSrv.Close()
	ownerHits := 0
	ownerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		ownerHits++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ownerSrv.Close()

	sequencerURL, err := url.Parse(sequencerSrv.URL)
	require.NoError(t, err)
	ownerURL, err := url.Parse(ownerSrv.URL)
	require.NoError(t, err)
	tmClient := &sendSeqBroadcastClient{MockClient: &MockClient{}, proxyURL: ownerURL}
	ctxReads := 0
	api := evmrpc.NewSendAPI(
		tmClient,
		func(int64) client.TxConfig { return TxConfig },
		&evmrpc.SendConfig{SequencerURL: sequencerURL},
		EVMKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) sdk.Context {
			ctxReads++
			return Ctx
		},
		"",
		nil,
		nil,
		nil,
		evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](),
		evmrpc.NewBlockCache(1),
		nil,
		nil,
	)

	hash, err := api.SendRawTransaction(context.Background(), firstRaw)
	require.NoError(t, err)
	require.Equal(t, first.Hash(), hash)

	_, err = api.SendRawTransaction(context.Background(), refusedRaw)
	require.EqualError(t, err, "nonce too high")
	var rpcErr interface{ ErrorCode() int }
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, -32000, rpcErr.ErrorCode())

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"eth_sendRawTransaction", "eth_sendRawTransaction"}, methods)
	require.Equal(t, []string{hexutil.Encode(firstRaw), hexutil.Encode(refusedRaw)}, payloads)
	require.Zero(t, ownerHits)
	require.Zero(t, tmClient.broadcastCount())
	require.Zero(t, ctxReads)
}
