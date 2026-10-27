// node/app.go passes the same four options through the server wiring at
// node/app.go:3266 and node/app.go:3275 (wiring.J38.1 and wiring.J38.2):
// SequencerOption and PendingOption are shared by HTTP and WebSocket;
// SequencerReceiptOption serves HTTP receipts and SequencerForwardOption serves
// follower sends. The sequencer and forwarding options select different roles.
package evmrpc_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type SequencerReceiptOption = evmrpc.SequencerReceiptOption

func NewSequencerOption(accepter evmrpc.SequencerAccepter, feed *evmrpc.ReceiptLogFeed) evmrpc.SequencerOption {
	return evmrpc.NewSequencerOption(accepter, feed)
}

func NewPendingOption(source evmrpc.PendingSnapshot) (evmrpc.PendingOption, error) {
	return evmrpc.NewPendingOption(source)
}

// sequencerOptionsAccepter adapts the real Receipter to the RPC accepter and
// retains its execution outputs in the real receipt source for HTTP lookup.
type sequencerOptionsAccepter struct {
	mu     sync.Mutex
	s      *pendingSequencer
	r      *sequencer.Receipter
	source *sequencer.ReceiptSource
}

func (a *sequencerOptionsAccepter) Accept(txBytes []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	accepted, receipt, err := a.r.Accept(txBytes)
	if err != nil {
		return nil, err
	}
	decoded, err := codec.DecodeReceipt(receipt)
	if err != nil {
		return nil, err
	}
	snapshot, err := a.s.exec.Snapshot()
	if err != nil {
		return nil, err
	}
	transient, err := a.s.app.EvmKeeper.GetTransientReceipt(snapshot, common.Hash(accepted.ActivityID), uint64(accepted.TxIndex))
	if err != nil {
		return nil, err
	}
	var msgData sdk.TxMsgData
	if err := msgData.Unmarshal(accepted.Result.Data); err != nil {
		return nil, err
	}
	if len(msgData.Data) != 1 {
		return nil, fmt.Errorf("want one EVM response, got %d", len(msgData.Data))
	}
	var response evmtypes.MsgEVMTransactionResponse
	if err := response.Unmarshal(msgData.Data[0].Data); err != nil {
		return nil, err
	}
	send := evmrpc.SequencerSend{
		Nonce: decoded.FromSequence, Receipt: receipt, ReturnData: response.ReturnData,
		VMError: transient.VmError, LogData: make([][]byte, len(transient.Logs)),
	}
	for i, log := range transient.Logs {
		send.LogData[i] = log.Data
	}
	a.source.Record(common.Hash(accepted.ActivityID), send)
	return receipt, nil
}

type sequencerOptionsServers struct {
	s        *pendingSequencer
	http, ws *rpc.Client
	source   *sequencer.ReceiptSource
	pins     seqkeeper.ReceiptKeys
}

func newSequencerOptionsServers(t *testing.T, sender utils.TestAcct) *sequencerOptionsServers {
	t.Helper()
	params := app.DefaultConsensusParams
	configured, blockParams := *params, *params.Block
	blockParams.MaxGasWanted = subscribeTwoLogsGas
	configured.Block = &blockParams
	app.DefaultConsensusParams = &configured
	t.Cleanup(func() { app.DefaultConsensusParams = params })
	s := newPendingSequencer(t, 1, sender)
	dir := t.TempDir()
	consensusFile := filepath.Join(dir, "priv_validator_key.json")
	pv, err := privval.GenFilePV(consensusFile, filepath.Join(dir, "priv_validator_state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Key.Save())
	seed := sendSeqLabel("sequencer options integration receipt key")
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	keyID := sendSeqLabel("sequencer options integration key id")
	keyFile := filepath.Join(dir, "receipt_key")
	require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seed)+"\n"), 0o600))
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: 1000}}
	key, err := sequencer.LoadReceiptKey(keyFile, consensusFile, []verify.SequencerAuthorization{{
		SequencerID: [32]byte(keyID), PublicKey: [32]byte(public), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, s.committed+1)
	require.NoError(t, err)
	// Commit the governed key pin before opening the block used by these tests:
	// fresh RPC cache stores read the committed version of the module store.
	state := sdk.NewContext(s.app.CommitMultiStore(), s.ctxProvider(evmrpc.LatestCtxHeight).BlockHeader(), false)
	require.NoError(t, s.app.SequencerKeeper.SetReceiptKeys(state, pins))
	open, err := s.exec.Snapshot()
	require.NoError(t, err)
	_, err = s.exec.Seal(s.committed + 1)
	require.NoError(t, err)
	s.committed++
	require.Equal(t, s.committed, s.app.CommitMultiStore().Commit(true).Version)
	require.NoError(t, s.exec.Open(s.committed+1, open.BlockTime().Add(time.Second).UnixMilli()))
	pinned, err := s.app.SequencerKeeper.ReceiptKeys(s.ctxProvider(evmrpc.LatestCtxHeight))
	require.NoError(t, err)
	require.Len(t, pinned, 1)
	require.Equal(t, keyID, pinned[0].KeyId[:])
	require.Equal(t, []byte(public), pinned[0].PublicKey[:])
	r, err := sequencer.NewReceipter(s.exec, s.app, &s.app.EvmKeeper, key, sendSeqModuleID)
	require.NoError(t, err)
	source := sequencer.NewReceiptSource(1)
	accepter := &sequencerOptionsAccepter{s: s, r: r, source: source}
	option := NewSequencerOption(accepter, evmrpc.NewReceiptLogFeed(s.committed))
	pending, err := NewPendingOption(s.exec)
	require.NoError(t, err)
	receipts := SequencerReceiptOption{Source: source, Keeper: &s.app.SequencerKeeper}
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.HTTPPort, config.WSPort = "127.0.0.1", 0, 0
	config.FilterTimeout = time.Minute
	txConfig := func(int64) client.TxConfig { return s.app.GetTxConfig() }
	httpServer, err := evmrpc.NewEVMHTTPServer(config, &MockClient{latestOverride: s.committed}, &s.app.EvmKeeper,
		s.app.BeginBlockKeepers, s.app.BaseApp, s.app.TracerAnteHandler, s.ctxProvider, txConfig, "", nil, option, receipts, pending)
	require.NoError(t, err)
	t.Cleanup(httpServer.Stop)
	require.NoError(t, httpServer.Start())
	wsServer, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{latestOverride: s.committed}, &s.app.EvmKeeper,
		s.app.BeginBlockKeepers, s.app.BaseApp, s.app.TracerAnteHandler, s.ctxProvider, txConfig, "", nil, nil, option, pending)
	require.NoError(t, err)
	t.Cleanup(wsServer.Stop)
	require.NoError(t, wsServer.Start())
	httpAddr := httpServer.(*evmrpc.HTTPServer).ListenAddr()
	wsAddr := wsServer.(*evmrpc.HTTPServer).ListenAddr()
	require.NotEqual(t, httpAddr, wsAddr)
	httpClient, err := rpc.DialHTTP("http://" + httpAddr)
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wsClient, err := rpc.DialWebsocket(ctx, "ws://"+wsAddr, "http://localhost")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)
	return &sequencerOptionsServers{s: s, http: httpClient, ws: wsClient, source: source, pins: pins}
}

func TestSequencerOptionsShareFeedAcrossHTTPAndWS(t *testing.T) {
	sender := utils.NewSigner()
	servers := newSequencerOptionsServers(t, sender)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pendingHashes := make(chan common.Hash, 1)
	pendingSub, err := servers.ws.EthSubscribe(ctx, pendingHashes, "newPendingTransactions")
	require.NoError(t, err)
	defer pendingSub.Unsubscribe()
	logs := make(chan json.RawMessage, 2)
	logSub, err := servers.ws.EthSubscribe(ctx, logs, "logs", map[string]any{})
	require.NoError(t, err)
	defer logSub.Unsubscribe()
	raw, wantHash := subscribeRawTx(t, sender, 0, nil, big.NewInt(0), subscribeTwoLogsGas, subscribeTwoLogsInitCode)
	var acceptedHash common.Hash
	require.NoError(t, servers.http.CallContext(ctx, &acceptedHash, "eth_sendRawTransaction", raw))
	require.Equal(t, wantHash, acceptedHash)
	select {
	case hash := <-pendingHashes:
		require.Equal(t, acceptedHash, hash)
	case err := <-pendingSub.Err():
		t.Fatalf("pending subscription ended before delivery: %v", err)
	case <-ctx.Done():
		t.Fatal("accepted HTTP hash did not reach the WebSocket subscriber")
	}
	address := crypto.CreateAddress(sender.EvmAddress, 0)
	for i := 0; i < 2; i++ {
		select {
		case rawLog := <-logs:
			var log map[string]any
			require.NoError(t, json.Unmarshal(rawLog, &log))
			require.Equal(t, receiptedLogJSON(address, uint64(i+1), servers.s.committed+1, acceptedHash, 1, uint(i), 1), log)
		case err := <-logSub.Err():
			t.Fatalf("logs subscription ended before delivery: %v", err)
		case <-ctx.Done():
			t.Fatalf("receipt log %d did not reach the WebSocket subscriber", i)
		}
	}
	require.Equal(t, servers.s.committed, servers.s.app.LastBlockHeight(), "notifications must arrive before block commit")
}

func TestSequencerOptionsServePendingAndReceipt(t *testing.T) {
	sender := utils.NewSigner()
	servers := newSequencerOptionsServers(t, sender)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, wantHash := subscribeRawTx(t, sender, 0, nil, big.NewInt(0), subscribeTwoLogsGas, subscribeTwoLogsInitCode)
	var acceptedHash common.Hash
	require.NoError(t, servers.http.CallContext(ctx, &acceptedHash, "eth_sendRawTransaction", raw))
	require.Equal(t, wantHash, acceptedHash)
	for name, transport := range map[string]*rpc.Client{"HTTP": servers.http, "WebSocket": servers.ws} {
		t.Run(name, func(t *testing.T) {
			var block struct {
				Number       hexutil.Uint64 `json:"number"`
				Hash         *common.Hash   `json:"hash"`
				Transactions []common.Hash  `json:"transactions"`
			}
			require.NoError(t, transport.CallContext(ctx, &block, "eth_getBlockByNumber", "pending", false))
			require.Equal(t, hexutil.Uint64(servers.s.committed+1), block.Number)
			require.Nil(t, block.Hash)
			require.Equal(t, []common.Hash{acceptedHash}, block.Transactions)
		})
	}
	var receipt *evmrpc.SequencerReceiptResult
	require.NoError(t, servers.http.CallContext(ctx, &receipt, "pax_sequencerReceipt", acceptedHash))
	require.NotNil(t, receipt)
	require.NotNil(t, receipt.SequencerReceiptView)
	require.Equal(t, acceptedHash, receipt.TxHash)
	require.Equal(t, evmrpc.RungReceipted, receipt.Rung)
	require.Equal(t, hexutil.Uint64(0), receipt.Nonce)
	require.Equal(t, hexutil.Uint64(1), receipt.Seq)
	require.Equal(t, hexutil.Uint64(servers.s.committed+1), receipt.Height)
	// Index zero is reserved for MsgSequenceBatch before the accepted EVM transactions.
	require.Equal(t, hexutil.Uint(1), receipt.Index)
	require.Equal(t, hexutil.Uint64(1), receipt.Status)
	require.Equal(t, sender.EvmAddress, receipt.From)
	address := crypto.CreateAddress(sender.EvmAddress, 0)
	require.Equal(t, address, receipt.ContractAddress)
	require.Equal(t, []evmrpc.SequencerReceiptLog{
		{Address: address, Topics: []common.Hash{common.BigToHash(big.NewInt(1))}, Data: hexutil.Bytes{}, LogIndex: 0},
		{Address: address, Topics: []common.Hash{common.BigToHash(big.NewInt(2))}, Data: hexutil.Bytes{}, LogIndex: 1},
	}, receipt.Logs)
	require.NoError(t, receipt.Verify(servers.pins))
	send, err := servers.source.SequencerSend(acceptedHash)
	require.NoError(t, err)
	require.NotNil(t, send)
	require.Equal(t, send.Receipt, []byte(receipt.Receipt))
	require.Equal(t, servers.s.committed, servers.s.app.LastBlockHeight(), "pending block and receipt must be served before commit")
}
