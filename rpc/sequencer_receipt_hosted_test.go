package evmrpc_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func hostedReceiptServer(t *testing.T, chain *seqReceiptChain9005, option evmrpc.SequencerReceiptOption) (*evmrpc.HTTPServer, *rpc.Client) {
	t.Helper()
	nodeCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	kv := kvstore.NewApplication()
	nodeService, closer, err := rpctest.StartTendermint(nodeCtx, conf, kv, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closer(context.Background()))
		require.NoError(t, kv.Close())
	})
	nodeWithEnv, ok := nodeService.(rpclocal.NodeService)
	require.True(t, ok)
	tmClient, err := rpclocal.New(nodeWithEnv)
	require.NoError(t, err)
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.HTTPPort = "127.0.0.1", 0
	service, err := evmrpc.NewEVMHTTPServer(config, tmClient, EVMKeeper, testApp.BeginBlockKeepers, testApp.BaseApp,
		testApp.TracerAnteHandler, chain.ctxProvider, func(int64) client.TxConfig { return TxConfig }, "", nil, option)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := service.(*evmrpc.HTTPServer)
	require.NoError(t, server.Start())
	require.Contains(t, server.HTTPMethods(), evmrpc.SequencerReceiptMethod)
	rpcClient, err := rpc.DialHTTP("http://" + server.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	return server, rpcClient
}

func TestSequencerReceiptUnknownTxIsNull(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	server, local := hostedReceiptServer(t, chain, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	endpoint, err := url.Parse("http://" + server.ListenAddr())
	require.NoError(t, err)
	_, forwarded := hostedReceiptServer(t, chain, evmrpc.SequencerReceiptOption{ForwardURL: endpoint, Keeper: &chain.keeper})
	for name, rpcClient := range map[string]*rpc.Client{"local": local, "forwarded": forwarded} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var raw json.RawMessage
			require.NoError(t, rpcClient.CallContext(ctx, &raw, evmrpc.SequencerReceiptMethod, common.HexToHash("0x796101")))
			require.Equal(t, "null", string(raw))
		})
	}
}

func TestSequencerReceiptCarriesSigningKey(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	hash, _ := chain.accept9005(seqReceiptTx9005{
		nonce: 1, height: 7, txIndex: 1, status: 1, returnData: []byte{0x01}, logs: seqReceiptTwoLogs9005(),
	})
	_, rpcClient := hostedReceiptServer(t, chain, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var raw json.RawMessage
	require.NoError(t, rpcClient.CallContext(ctx, &raw, evmrpc.SequencerReceiptMethod, hash))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	public := chain.receiptKey.PublicKey()
	require.JSONEq(t, `"`+hexutil.Encode(public[:])+`"`, string(fields["signingKey"]))
	var result evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(raw, &result))
	require.Equal(t, evmrpc.RungReceipted, result.Rung)
	require.Equal(t, hash, result.TxHash)
	require.Equal(t, hexutil.Uint64(7), result.Height)
	require.Equal(t, public[:], []byte(result.SigningKey))
	send, err := chain.SequencerSend(hash)
	require.NoError(t, err)
	require.Equal(t, send.Receipt, []byte(result.Receipt))
	keyID, err := chain.receiptKey.KeyID(int64(result.Height))
	require.NoError(t, err)
	require.Equal(t, common.Hash(keyID), result.KeyID)
	require.NotEqual(t, common.Hash(sha256.Sum256(public[:])), result.KeyID)

	pinned, err := chain.keeper.ReceiptKeys(chain.ctxProvider(evmrpc.LatestCtxHeight))
	require.NoError(t, err)
	keys := make(seqkeeper.ReceiptKeys, len(pinned))
	for i, pin := range pinned {
		keys[i] = seqkeeper.ReceiptKeyEntry{
			KeyId: bytes.Clone(pin.KeyId[:]), PublicKey: bytes.Clone(pin.PublicKey[:]),
			FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight,
		}
	}
	require.NoError(t, result.Verify(keys))
	verified, err := keys.VerifyReceipt(result.Receipt)
	require.NoError(t, err)
	require.Equal(t, keyID, verified.Receipt.ContextHash)
	require.Equal(t, [32]byte(hash), verified.Receipt.ActivityID)
	_, err = verify.ReceiptSignature(result.Receipt, [32]byte(result.SigningKey))
	require.NoError(t, err)

	t.Run("inclusive height pins", func(t *testing.T) {
		atHeight := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: public[:], FirstHeight: 7, LastHeight: 7}}
		require.NoError(t, result.Verify(atHeight))
		atHeight[0].FirstHeight, atHeight[0].LastHeight = 8, 9
		require.ErrorIs(t, result.Verify(atHeight), seqkeeper.ErrReceiptKeyNotPinned)
		atHeight[0].FirstHeight, atHeight[0].LastHeight = 1, 6
		require.ErrorIs(t, result.Verify(atHeight), seqkeeper.ErrReceiptKeyNotPinned)
	})
	t.Run("key id is pinned independently", func(t *testing.T) {
		digest := sha256.Sum256(public[:])
		wrongID := seqkeeper.ReceiptKeys{{KeyId: digest[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 1999}}
		require.ErrorIs(t, result.Verify(wrongID), seqkeeper.ErrReceiptKeyNotPinned)
		require.ErrorIs(t, result.Verify(seqkeeper.ReceiptKeys{}), seqkeeper.ErrReceiptKeyNotPinned)
	})
	for name, edit := range map[string]func(*evmrpc.SequencerReceiptResult){
		"missing signing key":   func(r *evmrpc.SequencerReceiptResult) { r.SigningKey = nil },
		"short signing key":     func(r *evmrpc.SequencerReceiptResult) { r.SigningKey = r.SigningKey[:31] },
		"different signing key": func(r *evmrpc.SequencerReceiptResult) { r.SigningKey[0] ^= 1 },
		"different key id":      func(r *evmrpc.SequencerReceiptResult) { r.KeyID[0] ^= 1 },
		"invalid signature":     func(r *evmrpc.SequencerReceiptResult) { r.Receipt[len(r.Receipt)-1] ^= 1 },
	} {
		t.Run(name, func(t *testing.T) {
			var altered evmrpc.SequencerReceiptResult
			require.NoError(t, json.Unmarshal(raw, &altered))
			edit(&altered)
			require.Error(t, altered.Verify(keys))
		})
	}

	chain.mu.Lock()
	chain.sends[hash].Receipt = bytes.Clone(send.Receipt)
	chain.sends[hash].Receipt[len(send.Receipt)-1] ^= 1
	chain.mu.Unlock()
	var rejected json.RawMessage
	require.Error(t, rpcClient.CallContext(ctx, &rejected, evmrpc.SequencerReceiptMethod, hash))
}
