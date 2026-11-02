// Proves: RawTransactionAPI, UncleAPI and SendAPI.SendRawTransactionSync -> NewEVMHTTPServer HTTP JSON-RPC -> rpc.Client.CallContext, types.Transaction.UnmarshalBinary and types.Receipt.UnmarshalJSON.
//
// The consensus node stores the envelopes submitted by SendAPI; the real EVM
// message server executes those envelopes and emits the receipts the RPC reads.
// No consensus client, RPC service, HTTP transport or decoder is replaced.
// Each round uses a fresh listener and client; every read is made twice and a
// second signed transfer proves that the consumer observes new producer state.
// The RPC anchor has no persistent state that needs a process restart.
//
// Repairs rpc/raw_tx.go:GetRawTransactionByHash: a corrupt receipt index could
// return another transaction's valid envelope. All hash lookup paths now check
// the envelope's hash. The proof corrupts a real emitted receipt's lookup key
// and requires a JSON-RPC refusal, and corrupts real wire results at the consumer
// boundary to exercise the same strict decoders used for successful responses.
//
// Finding 21.44.1.1, decision 2300: rpc/server.go:NewEVMHTTPServer and
// NewEVMWebSocketServer register RawTransactionAPI directly. Missing raw
// transactions therefore encode nil hexutil.Bytes as the JSON string "0x",
// matching go-ethereum; missing uncles and uncle counts encode as JSON null.
// The proof checks these distinct wire results and refuses empty envelopes at
// the transaction consumer rather than treating them as signed transactions.
package evmrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// decodeProofClientParityEnvelope is the first consumer of the raw JSON-RPC
// result. It checks both the wire encoding and the signed envelope identity.
func decodeProofClientParityEnvelope(result json.RawMessage, want common.Hash) (*ethtypes.Transaction, error) {
	var raw hexutil.Bytes
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("transaction not found")
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	if tx.Hash() != want {
		return nil, fmt.Errorf("unexpected transaction hash %s", tx.Hash())
	}
	return &tx, nil
}

func TestProofClientParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	service, closeConsensus, err := rpctest.StartTendermint(ctx, conf, consensusApp, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closeConsensus(context.Background()))
		require.NoError(t, consensusApp.Close())
	})
	node, ok := service.(rpclocal.NodeService)
	require.True(t, ok)
	tm, err := rpclocal.New(node)
	require.NoError(t, err)

	application := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	k := &application.EvmKeeper
	sdkctx := application.GetContextForDeliverTx(nil).
		WithBlockHeight(1).WithChainID("hyperpax_125-1").WithBlockTime(time.Now()).
		WithClosestUpgradeName(LatestCtxUpgradeName).
		WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 10000000}})
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x210441")
	coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(sdkctx), sdk.NewInt(1000000000)))
	require.NoError(t, application.BankKeeper.MintCoins(sdkctx, "evm", coins))
	require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(sdkctx, "evm", sdk.AccAddress(from.Bytes()), coins))
	chainID := k.ChainID(sdkctx)
	var height atomic.Int64
	height.Store(1)
	ctxProvider := func(h int64) sdk.Context {
		if h == evmrpc.LatestCtxHeight {
			h = height.Load()
		}
		return sdkctx.WithBlockHeight(h).WithIsTracing(true)
	}
	txConfig := application.GetTxConfig()
	cfg := evmrpcconfig.DefaultConfig
	cfg.HTTPAddress, cfg.HTTPPort = "127.0.0.1", 0
	cfg.IPRateLimitRPS = 0
	cfg.Slow = true
	cfg.SendRawTxSyncTimeout = 15 * time.Second
	cfg.TraceBakeEnabled = false
	unknown := common.HexToHash("0x210441dead")

	// Use separate listener/client lifetimes so the second round cannot read the
	// first server's block cache or a retained client result.
	for round := uint64(0); round < 2; round++ {
		t.Run(fmt.Sprintf("round_%d", round), func(t *testing.T) {
			serverService, err := evmrpc.NewEVMHTTPServer(cfg, tm, k, application.BeginBlockKeepers,
				application.BaseApp, application.TracerAnteHandler, ctxProvider,
				func(int64) client.TxConfig { return txConfig }, "", nil)
			require.NoError(t, err)
			t.Cleanup(serverService.Stop)
			require.NoError(t, serverService.Start())
			server := serverService.(*evmrpc.HTTPServer)
			rpcClient, err := rpc.DialHTTP("http://" + server.ListenAddr())
			require.NoError(t, err)
			t.Cleanup(rpcClient.Close)

			signed, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
				ChainID: chainID, Nonce: round, Gas: 100000, GasFeeCap: big.NewInt(1000000000000),
				GasTipCap: big.NewInt(0), To: &to, Value: big.NewInt(int64(round + 1)),
			})
			require.NoError(t, err)
			raw, err := signed.MarshalBinary()
			require.NoError(t, err)
			status, err := tm.Status(ctx)
			require.NoError(t, err)
			next := status.SyncInfo.LatestBlockHeight + 1
			type sendResult struct {
				raw json.RawMessage
				err error
			}
			sent := make(chan sendResult, 1)
			go func() {
				var result json.RawMessage
				err := rpcClient.CallContext(ctx, &result, "eth_sendRawTransactionSync", hexutil.Bytes(raw), hexutil.Uint64(15000))
				sent <- sendResult{result, err}
			}()

			// Discover the envelope actually committed by SendAPI, rather than making
			// a block fixture or predicting how its gas estimate is encoded.
			var block *coretypes.ResultBlock
			var message *evmtypes.MsgEVMTransaction
			var envelope sdk.Tx
			var consensusIndex int
			for block == nil {
				select {
				case result := <-sent:
					require.NoError(t, result.err)
					t.Fatal("sync send returned before its execution receipt existed")
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(20 * time.Millisecond):
				}
				status, err := tm.Status(ctx)
				require.NoError(t, err)
				for next <= status.SyncInfo.LatestBlockHeight && block == nil {
					candidate, err := tm.Block(ctx, &next)
					require.NoError(t, err)
					for i, encoded := range candidate.Block.Txs {
						decoded, err := txConfig.TxDecoder()(encoded)
						if err != nil {
							continue
						}
						msg := evmtypes.GetEVMTransactionMessage(decoded)
						if msg == nil {
							continue
						}
						tx, _ := msg.AsTransaction()
						if tx != nil && tx.Hash() == signed.Hash() {
							block, message, envelope, consensusIndex = candidate, msg, decoded, i
							break
						}
					}
					next++
				}
			}

			executionCtx := sdkctx.WithBlockHeight(block.Block.Height).WithBlockTime(block.Block.Time).
				WithTxIndex(consensusIndex).WithIsEVM(true).WithEVMNonce(round).
				WithEVMSenderAddress(from).WithEVMTxHash(signed.Hash())
			require.NoError(t, ante.Preprocess(executionCtx, message, chainID, false))
			executionCtx, err = ante.NewEVMFeeCheckDecorator(k, &application.UpgradeKeeper).AnteHandle(
				executionCtx, envelope, false, func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
			require.NoError(t, err)
			executed, err := evmkeeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(executionCtx), message)
			require.NoError(t, err)
			require.Empty(t, executed.VmError)
			produced, err := k.GetTransientReceipt(executionCtx, signed.Hash(), uint64(consensusIndex))
			require.NoError(t, err)
			require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), produced.Status)
			require.NoError(t, k.ReceiptStore().SetReceipts(executionCtx, []receiptstore.ReceiptRecord{{TxHash: signed.Hash(), Receipt: produced}}))
			require.Eventually(t, func() bool {
				_, err := k.GetReceipt(executionCtx, signed.Hash())
				return err == nil
			}, 5*time.Second, time.Millisecond)
			require.NoError(t, k.ReceiptStore().SetEarliestVersion(1))
			require.NoError(t, k.ReceiptStore().SetLatestVersion(block.Block.Height))
			height.Store(block.Block.Height)

			var result sendResult
			select {
			case result = <-sent:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			require.NoError(t, result.err)
			var receipt ethtypes.Receipt
			require.NoError(t, receipt.UnmarshalJSON(result.raw))
			blockHash := common.BytesToHash(block.BlockID.Hash)
			require.Equal(t, signed.Hash(), receipt.TxHash)
			require.Equal(t, blockHash, receipt.BlockHash)
			require.Equal(t, block.Block.Height, receipt.BlockNumber.Int64())
			require.Equal(t, ethtypes.ReceiptStatusSuccessful, receipt.Status)
			require.Equal(t, produced.GasUsed, receipt.GasUsed)
			require.Equal(t, uint(0), receipt.TransactionIndex)

			// Every read crosses the HTTP endpoint anew, into freshly declared results.
			for repeat := 0; repeat < 2; repeat++ {
				for _, call := range []struct {
					method string
					args   []interface{}
				}{
					{"eth_getRawTransactionByHash", []interface{}{signed.Hash()}},
					{"eth_getRawTransactionByBlockHashAndIndex", []interface{}{blockHash, "0x0"}},
					{"eth_getRawTransactionByBlockNumberAndIndex", []interface{}{hexutil.EncodeUint64(uint64(block.Block.Height)), "0x0"}},
				} {
					var observed json.RawMessage
					require.NoError(t, rpcClient.CallContext(ctx, &observed, call.method, call.args...))
					decoded, err := decodeProofClientParityEnvelope(observed, signed.Hash())
					require.NoError(t, err, call.method)
					binary, err := decoded.MarshalBinary()
					require.NoError(t, err)
					require.Equal(t, raw, binary)
					sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(chainID), decoded)
					require.NoError(t, err)
					require.Equal(t, from, sender)

					// Mutate an actual producer result at the consumer boundary: invalid hex,
					// invalid RLP, and a valid envelope under the wrong requested identity.
					var invalidHex json.RawMessage = append(json.RawMessage(nil), observed...)
					invalidHex[3] = 'g'
					_, err = decodeProofClientParityEnvelope(invalidHex, signed.Hash())
					require.Error(t, err)
					truncated, err := json.Marshal(hexutil.Bytes(binary[:1]))
					require.NoError(t, err)
					_, err = decodeProofClientParityEnvelope(truncated, signed.Hash())
					require.Error(t, err)
					_, err = decodeProofClientParityEnvelope(observed, unknown)
					require.ErrorContains(t, err, "unexpected transaction hash")
				}
				for _, call := range []struct {
					method string
					args   []interface{}
				}{
					{"eth_getUncleByBlockHashAndIndex", []interface{}{blockHash, "0x0"}},
					{"eth_getUncleByBlockNumberAndIndex", []interface{}{hexutil.EncodeUint64(uint64(block.Block.Height)), "0x0"}},
				} {
					var uncle *ethtypes.Header
					require.NoError(t, rpcClient.CallContext(ctx, &uncle, call.method, call.args...))
					require.Nil(t, uncle)
				}
				for _, call := range []struct {
					method string
					arg    interface{}
				}{
					{"eth_getUncleCountByBlockHash", blockHash},
					{"eth_getUncleCountByBlockNumber", hexutil.EncodeUint64(uint64(block.Block.Height))},
				} {
					var count *hexutil.Uint
					require.NoError(t, rpcClient.CallContext(ctx, &count, call.method, call.arg))
					require.NotNil(t, count)
					require.Equal(t, hexutil.Uint(0), *count)
				}
				var standard ethtypes.Receipt
				require.NoError(t, rpcClient.CallContext(ctx, &standard, "eth_getTransactionReceipt", signed.Hash()))
				require.Equal(t, receipt, standard)
			}

			// Decision 2300: missing raw transactions encode as the JSON string "0x".
			for _, call := range []struct {
				method string
				args   []interface{}
			}{
				{"eth_getRawTransactionByHash", []interface{}{unknown}},
				{"eth_getRawTransactionByBlockHashAndIndex", []interface{}{unknown, "0x0"}},
				{"eth_getRawTransactionByBlockHashAndIndex", []interface{}{blockHash, "0x1"}},
				{"eth_getRawTransactionByBlockNumberAndIndex", []interface{}{"0x7fffffffffffffff", "0x0"}},
				{"eth_getRawTransactionByBlockNumberAndIndex", []interface{}{hexutil.EncodeUint64(uint64(block.Block.Height)), "0x1"}},
			} {
				t.Run("empty/"+call.method, func(t *testing.T) {
					var observed json.RawMessage
					require.NoError(t, rpcClient.CallContext(ctx, &observed, call.method, call.args...))
					require.Equal(t, `"0x"`, string(observed), call.method)
					_, err := decodeProofClientParityEnvelope(observed, signed.Hash())
					require.Error(t, err, call.method)
				})
			}
			// Uncle lookups retain JSON null for unknown blocks and missing indexes.
			for _, call := range []struct {
				method string
				args   []interface{}
			}{
				{"eth_getUncleByBlockHashAndIndex", []interface{}{unknown, "0x0"}},
				{"eth_getUncleByBlockHashAndIndex", []interface{}{blockHash, "0x1"}},
				{"eth_getUncleByBlockNumberAndIndex", []interface{}{"0x7fffffffffffffff", "0x0"}},
				{"eth_getUncleByBlockNumberAndIndex", []interface{}{hexutil.EncodeUint64(uint64(block.Block.Height)), "0x1"}},
				{"eth_getUncleCountByBlockHash", []interface{}{unknown}},
				{"eth_getUncleCountByBlockNumber", []interface{}{"0x7fffffffffffffff"}},
			} {
				t.Run("null/"+call.method, func(t *testing.T) {
					var observed json.RawMessage
					require.NoError(t, rpcClient.CallContext(ctx, &observed, call.method, call.args...))
					require.Equal(t, "null", string(observed), call.method)
				})
			}
			// The production dispatcher rejects malformed parameters on all 8 surfaces.
			for _, method := range []string{
				"eth_getRawTransactionByHash", "eth_getRawTransactionByBlockHashAndIndex", "eth_getRawTransactionByBlockNumberAndIndex",
				"eth_getUncleByBlockHashAndIndex", "eth_getUncleByBlockNumberAndIndex", "eth_getUncleCountByBlockHash",
				"eth_getUncleCountByBlockNumber", "eth_sendRawTransactionSync",
			} {
				args := []interface{}{"0xnothex"}
				if strings.HasSuffix(method, "AndIndex") {
					args = append(args, "0x0")
				}
				var observed json.RawMessage
				err := rpcClient.CallContext(ctx, &observed, method, args...)
				var rpcErr rpc.Error
				require.ErrorAs(t, err, &rpcErr, method)
				require.Equal(t, -32602, rpcErr.ErrorCode(), method)
				require.Empty(t, observed)
			}
			var refused json.RawMessage
			err = rpcClient.CallContext(ctx, &refused, "eth_sendRawTransactionSync", "0x1234")
			var rpcErr rpc.Error
			require.ErrorAs(t, err, &rpcErr)
			require.Equal(t, -32000, rpcErr.ErrorCode())
			require.Empty(t, refused)

			// Corrupt a real producer result, then use the receipt store and the real
			// endpoint again. The old implementation returned signed's valid envelope
			// for this different lookup hash instead of refusing the corrupt index.
			corruptHash := common.BytesToHash(crypto.Keccak256(signed.Hash().Bytes(), []byte("corrupt index")))
			require.NoError(t, k.ReceiptStore().SetReceipts(executionCtx, []receiptstore.ReceiptRecord{{TxHash: corruptHash, Receipt: produced}}))
			require.Eventually(t, func() bool {
				_, err := k.GetReceipt(executionCtx, corruptHash)
				return err == nil
			}, 5*time.Second, time.Millisecond)
			err = rpcClient.CallContext(ctx, &refused, "eth_getRawTransactionByHash", corruptHash)
			require.ErrorAs(t, err, &rpcErr)
			require.Equal(t, -32000, rpcErr.ErrorCode())
			require.ErrorContains(t, err, "raw transaction hash mismatch")
			require.Empty(t, refused)

			// Exercise the real receipt consumer with a malformed result derived from
			// the sync producer, without installing an HTTP responder or decoder double.
			malformed := bytes.Replace(result.raw, []byte(`"status":"0x1"`), []byte(`"status":"0xzz"`), 1)
			require.NotEqual(t, result.raw, json.RawMessage(malformed))
			var invalidReceipt ethtypes.Receipt
			require.Error(t, invalidReceipt.UnmarshalJSON(malformed))
		})
	}
}
