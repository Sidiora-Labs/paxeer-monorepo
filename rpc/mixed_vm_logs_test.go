package evmrpc_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	evmt "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	signing "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	gateway "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// Task 22.33.752 repair record:
// txcount-latest-race -> TestMixedVMLogsPinnedTransactionCount
// txcount-dualvm-skip -> TestMixedVMLogsTxcountDualVMExclusion
// logs-optional-wasm -> TestMixedVMLogsDeclaredWasmCapability
// logs-transaction-index-origin -> TestMixedVMLogsBlockGlobalOrigin
// subscription-restart-proof -> TestMixedVMLogsSubscriptionRestartRecovery
// Baseline: not run; the explicit gate order forbids pre-implementation verification.
// Harness gotcha: committed contexts need the stored block's time for Wasm queries.
// Pointer fixtures require production registration's linked runtime and associated recipients.
// The shared pending harness disables custom precompiles; Wasm fixtures wire the production set.
// Wiring request: domain 16 owns tests/integration/rpc_tests/scripts/run-ci.sh;
// qualify RPC_TESTS_WASM_MODE=enabled and =disabled in separate chain configurations.
// Verified revision, command, exit and private evidence are recorded by hf-done.

func mixedVMLogsSource(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile("../tests/integration/rpc_tests/eth/" + file + ".spec.ts")
	require.NoError(t, err)
	return string(data)
}

func mixedVMLogsBlockClient(t *testing.T, h *committedFilterHarness) *rpc.Client {
	t.Helper()
	wm := evmrpc.NewWatermarkManager(h.tm, h.s.ctxProvider, nil, h.s.app.EvmKeeper.ReceiptStore())
	api := evmrpc.NewBlockAPI(h.tm, &h.s.app.EvmKeeper, h.s.ctxProvider,
		func(int64) client.TxConfig { return h.s.app.GetTxConfig() }, evmrpc.ConnectionTypeHTTP,
		wm, evmrpc.NewBlockCache(64), &sync.Mutex{})
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", api))
	t.Cleanup(server.Stop)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	c, err := rpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}

type mixedVMLogsBlock struct {
	Number       hexutil.Uint64
	Hash         common.Hash
	Transactions []common.Hash
}

func mixedVMLogsReadBlock(t *testing.T, c *rpc.Client, tag string) mixedVMLogsBlock {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var block *mixedVMLogsBlock
	require.NoError(t, c.CallContext(ctx, &block, "eth_getBlockByNumber", tag, false))
	require.NotNil(t, block)
	return *block
}

func mixedVMLogsCount(t *testing.T, c *rpc.Client, tag string) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count *hexutil.Uint64
	require.NoError(t, c.CallContext(ctx, &count, "eth_getBlockTransactionCountByNumber", tag))
	require.NotNil(t, count)
	return uint64(*count)
}

func mixedVMLogsCommittedContext(h *committedFilterHarness) sdk.Context {
	block := h.blocks[h.s.committed]
	return h.s.ctxProvider(evmrpc.LatestCtxHeight).WithBlockTime(block.Block.Time)
}

func TestMixedVMLogsPinnedTransactionCount(t *testing.T) {
	source := mixedVMLogsSource(t, "eth_getBlockTransactionCountByNumber")
	require.Contains(t, source, "const height = Number(BigInt(head.number))")
	require.Contains(t, source, "txCountByNumber(pax, height)")
	h := newCommittedFilterHarness(t, 100)
	first := mixedVMLogsCommit(t, h, 2)
	c := mixedVMLogsBlockClient(t, h)
	head := mixedVMLogsReadBlock(t, c, "latest")
	require.Equal(t, first, head.Transactions)
	require.Equal(t, uint64(2), mixedVMLogsCount(t, c, "latest"))
	second := mixedVMLogsCommit(t, h, 1)
	next := mixedVMLogsReadBlock(t, c, "latest")
	require.Greater(t, next.Number, head.Number)
	require.Equal(t, second, next.Transactions)
	require.Equal(t, uint64(1), mixedVMLogsCount(t, c, "latest"))
	pinned := hexutil.EncodeUint64(uint64(head.Number))
	require.Equal(t, uint64(len(head.Transactions)), mixedVMLogsCount(t, c, pinned))
	require.Equal(t, head, mixedVMLogsReadBlock(t, c, pinned))
}

// The existing committed-log harness uses the real executor, consensus block store,
// local consensus RPC client, keeper and receipt store. No handler is replaced.
func mixedVMLogsSeal(t *testing.T, h *committedFilterHarness, setup func(sdk.Context)) *coretypes.ResultBlock {
	t.Helper()
	height := h.s.committed + 1
	batch, err := h.s.exec.Seal(height)
	require.NoError(t, err)
	if setup != nil {
		setup(batch.Ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(batch.Ctx)))
	}
	require.NoError(t, h.s.app.EvmKeeper.FlushTransientReceipts(batch.Ctx))
	batch.Store.Write()
	require.Equal(t, height, h.s.app.CommitMultiStore().Commit(true).Version)
	h.s.committed = height
	require.NoError(t, h.s.app.EvmKeeper.ReceiptStore().SetLatestVersion(height))
	block := h.saveBlock(height, batch.Txs, batch.Ctx.BlockTime())
	require.NoError(t, h.tm.StateStore.SaveFinalizeBlockResponses(height, &abci.ResponseFinalizeBlock{
		TxResults: batch.Results, AppHash: h.s.app.CommitMultiStore().LastCommitID().Hash,
	}))
	for _, raw := range batch.Txs {
		decoded, err := h.s.app.GetTxConfig().TxDecoder()(raw)
		require.NoError(t, err)
		if msg, ok := decoded.GetMsgs()[0].(*evmt.MsgEVMTransaction); ok {
			tx, _ := msg.AsTransaction()
			require.NotNil(t, tx)
			require.Eventually(t, func() bool {
				_, err := h.s.app.EvmKeeper.GetReceipt(batch.Ctx, tx.Hash())
				return err == nil
			}, 5*time.Second, time.Millisecond)
		}
	}
	require.NoError(t, h.s.exec.Open(height+1, batch.TimestampMs+1000))
	return block
}

func mixedVMLogsCommit(t *testing.T, h *committedFilterHarness, count int) []common.Hash {
	t.Helper()
	var hashes []common.Hash
	for i := 0; i < count; i++ {
		hashes = append(hashes, mixedVMLogsAcceptEVM(t, h, nil, pendingFilterInitCode))
	}
	mixedVMLogsSeal(t, h, nil)
	return hashes
}

func mixedVMLogsCW20(t *testing.T, h *committedFilterHarness) (sdk.AccAddress, common.Address) {
	t.Helper()
	var contract sdk.AccAddress
	var pointerAddr common.Address
	mixedVMLogsSeal(t, h, func(ctx sdk.Context) {
		h.s.app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.V6120Upgrade)
		h.s.app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.MergeUpgrade)
		custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, h.s.app.GetPrecompileKeepers())
		require.NoError(t, err)
		h.s.app.EvmKeeper.SetCustomPrecompiles(custom, app.LatestUpgrade)
		require.Contains(t, h.s.app.EvmKeeper.CustomPrecompiles(ctx), common.HexToAddress(pointer.PointerAddress))
		params := wasmtypes.DefaultParams()
		h.s.app.WasmKeeper.SetParams(ctx, params)
		code, err := os.ReadFile("../contracts/wasm/cw20_base.wasm")
		require.NoError(t, err)
		keeper := wasmkeeper.NewDefaultPermissionKeeper(h.s.app.WasmKeeper)
		codeID, err := keeper.Create(ctx, h.sender.AccountAddress, code, nil)
		require.NoError(t, err)
		init := fmt.Sprintf(`{"name":"Repair token","symbol":"REPAIR","decimals":6,"initial_balances":[{"address":"%s","amount":"100"}]}`, h.sender.AccountAddress)
		contract, _, err = keeper.Instantiate(ctx, codeID, h.sender.AccountAddress, h.sender.AccountAddress,
			[]byte(init), "repair-m752", sdk.NewCoins())
		require.NoError(t, err)
		registration, err := pointer.NewPrecompile(h.s.app.GetPrecompileKeepers())
		require.NoError(t, err)
		input, err := registration.ABI.Pack(pointer.AddCW20Pointer, contract.String())
		require.NoError(t, err)
		err = h.s.app.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(e *vm.EVM) error {
			output, _, callErr := e.Call(h.sender.EvmAddress, common.HexToAddress(pointer.PointerAddress), input, 20_000_000, uint256.NewInt(0))
			if callErr != nil {
				return callErr
			}
			values, unpackErr := registration.ABI.Methods[pointer.AddCW20Pointer].Outputs.Unpack(output)
			if unpackErr != nil {
				return unpackErr
			}
			pointerAddr = values[0].(common.Address)
			return nil
		}, func(stage, message string) { t.Log(stage, message) })
		require.NoError(t, err)
	})
	require.NotEqual(t, common.Address{}, pointerAddr)
	registered, _, found := h.s.app.EvmKeeper.GetERC20CW20Pointer(mixedVMLogsCommittedContext(h), contract.String())
	require.True(t, found)
	require.Equal(t, pointerAddr, registered)
	return contract, pointerAddr
}

func mixedVMLogsNativeTx(t *testing.T, h *committedFilterHarness, msg sdk.Msg) []byte {
	t.Helper()
	ctx, err := h.s.exec.Snapshot()
	require.NoError(t, err)
	account := h.s.app.AccountKeeper.GetAccount(ctx, h.sender.AccountAddress)
	require.NotNil(t, account)
	config := h.s.app.GetTxConfig()
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(1_500_000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5_250_000)))
	mode := config.SignModeHandler().DefaultMode()
	sig := signing.SignatureV2{PubKey: h.sender.PublicKey, Sequence: account.GetSequence(),
		Data: &signing.SingleSignatureData{SignMode: mode}}
	require.NoError(t, builder.SetSignatures(sig))
	signBytes, err := config.SignModeHandler().GetSignBytes(mode, authsigning.SignerData{
		ChainID: pendingChainID, AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence(),
	}, builder.GetTx())
	require.NoError(t, err)
	signature, err := h.sender.PrivateKey.Sign(signBytes)
	require.NoError(t, err)
	sig.Data = &signing.SingleSignatureData{SignMode: mode, Signature: signature}
	require.NoError(t, builder.SetSignatures(sig))
	encoded, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	accepted, err := h.s.exec.Accept(encoded)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	return encoded
}

func mixedVMLogsAcceptEVM(t *testing.T, h *committedFilterHarness, to *common.Address, data []byte) common.Hash {
	t.Helper()
	snapshot, err := h.s.exec.Snapshot()
	require.NoError(t, err)
	nonce := h.s.app.EvmKeeper.GetNonce(snapshot, h.sender.EvmAddress)
	encoded, hash := pendingFilterTx(t, h.sender, nonce, to, big.NewInt(0), data, 2_000_000)
	accepted, err := h.s.exec.Accept(encoded)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	require.Empty(t, accepted.Result.EvmTxInfo.VmError)
	return hash
}

func TestMixedVMLogsTxcountDualVMExclusion(t *testing.T) {
	source := mixedVMLogsSource(t, "eth_getBlockTransactionCountByNumber")
	start := strings.Index(source, "async function sharedExclusionBlock")
	require.NotEqual(t, -1, start)
	require.NotContains(t, source[start:], "this.skip()")
	require.Contains(t, source, "required shared ${kind}/EVM block not established")
	require.Contains(t, source, "client.getBlock(native.height)")
	for _, kind := range []string{"bank", "cw20"} {
		t.Run(kind, func(t *testing.T) {
			h := newCommittedFilterHarness(t, 100)
			recipient := sdk.AccAddress(crypto.Keccak256([]byte("repair-m752-recipient"))[:20])
			var msg sdk.Msg
			var contract sdk.AccAddress
			if kind == "bank" {
				msg = &banktypes.MsgSend{FromAddress: h.sender.AccountAddress.String(), ToAddress: recipient.String(),
					Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 222111))}
			} else {
				contract, _ = mixedVMLogsCW20(t, h)
				msg = &wasmtypes.MsgExecuteContract{Sender: h.sender.AccountAddress.String(), Contract: contract.String(),
					Msg: []byte(fmt.Sprintf(`{"transfer":{"recipient":"%s","amount":"1"}}`, recipient)), Funds: sdk.NewCoins()}
			}
			native := mixedVMLogsNativeTx(t, h, msg)
			evmHash := mixedVMLogsAcceptEVM(t, h, nil, pendingFilterInitCode)
			block := mixedVMLogsSeal(t, h, nil)
			require.Len(t, block.Block.Txs, 2)
			require.Equal(t, native, []byte(block.Block.Txs[0]))
			decoded, err := h.s.app.GetTxConfig().TxDecoder()(block.Block.Txs[0])
			require.NoError(t, err)
			require.Equal(t, sdk.MsgTypeURL(msg), sdk.MsgTypeURL(decoded.GetMsgs()[0]))
			c := mixedVMLogsBlockClient(t, h)
			tag := hexutil.EncodeUint64(uint64(block.Block.Height))
			view := mixedVMLogsReadBlock(t, c, tag)
			require.Equal(t, []common.Hash{evmHash}, view.Transactions)
			require.NotContains(t, view.Transactions, common.Hash(sha256.Sum256(native)))
			require.Equal(t, uint64(1), mixedVMLogsCount(t, c, tag))
			if kind == "bank" {
				require.Equal(t, sdk.NewInt(222111), h.s.app.BankKeeper.GetBalance(h.s.ctxProvider(evmrpc.LatestCtxHeight), recipient, "uhpx").Amount)
			} else {
				balance, err := h.s.app.WasmKeeper.QuerySmart(mixedVMLogsCommittedContext(h), contract,
					[]byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, recipient)))
				require.NoError(t, err)
				require.JSONEq(t, `{"balance":"1"}`, string(balance))
			}
		})
	}
}

func mixedVMLogsLogs(t *testing.T, h *committedFilterHarness, criteria map[string]any) []ethtypes.Log {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var logs []ethtypes.Log
	require.NoError(t, h.client.CallContext(ctx, &logs, "eth_getLogs", criteria))
	return logs
}

func TestMixedVMLogsDeclaredWasmCapability(t *testing.T) {
	source := mixedVMLogsSource(t, "eth_getLogs")
	require.Contains(t, source, "RPC_TESTS_WASM_MODE")
	require.Contains(t, source, "if (!wasm) throw new Error")
	require.Contains(t, source, "Wasm-enabled qualification requires bootstrap CW20 fixtures")
	require.Contains(t, source, "explicit disabled-Wasm mode")
	h := newCommittedFilterHarness(t, 100)
	contract, pointer := mixedVMLogsCW20(t, h)
	ctx := mixedVMLogsCommittedContext(h)
	mux := gateway.NewServeMux()
	require.NoError(t, proposal.RegisterQueryHandlerServer(context.Background(), mux, h.s.app.ParamsKeeper))
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(sdk.WrapSDKContext(ctx)))
	}))
	t.Cleanup(gatewayServer.Close)
	readParam := func(key string) string {
		t.Helper()
		response, err := (&http.Client{Timeout: 5 * time.Second}).Get(gatewayServer.URL + "/cosmos/params/v1beta1/params?subspace=wasm&key=" + key)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		var consumed proposal.QueryParamsResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&consumed))
		return consumed.Param.Value
	}
	require.JSONEq(t, `{"permission":"Everybody"}`, readParam("uploadAccess"))
	require.JSONEq(t, `"Everybody"`, readParam("instantiateAccess"))
	transfer := abi.NewMethod("transfer", "transfer", abi.Function, "nonpayable", false, false,
		abi.Arguments{{Type: mixedVMLogsABIType(t, "address")}, {Type: mixedVMLogsABIType(t, "uint256")}}, nil)
	recipient := common.HexToAddress("0x752")
	mixedVMLogsSeal(t, h, func(ctx sdk.Context) {
		h.s.app.EvmKeeper.SetAddressMapping(ctx, sdk.AccAddress(recipient.Bytes()), recipient)
	})
	balanceBefore, err := h.s.app.WasmKeeper.QuerySmart(mixedVMLogsCommittedContext(h), contract,
		[]byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, h.sender.AccountAddress)))
	require.NoError(t, err)
	require.JSONEq(t, `{"balance":"100"}`, string(balanceBefore))
	args, err := transfer.Inputs.Pack(recipient, big.NewInt(1))
	require.NoError(t, err)
	hash := mixedVMLogsAcceptEVM(t, h, &pointer, append(transfer.ID, args...))
	block := mixedVMLogsSeal(t, h, nil)
	logs := mixedVMLogsLogs(t, h, map[string]any{"blockHash": common.BytesToHash(block.BlockID.Hash), "address": pointer})
	require.Len(t, logs, 1)
	require.Equal(t, hash, logs[0].TxHash)
	require.Equal(t, crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")), logs[0].Topics[0])
	require.Equal(t, common.BytesToHash(recipient.Bytes()), logs[0].Topics[2])
	require.Equal(t, big.NewInt(1), new(big.Int).SetBytes(logs[0].Data))
	balance, err := h.s.app.WasmKeeper.QuerySmart(mixedVMLogsCommittedContext(h), contract,
		[]byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, sdk.AccAddress(recipient.Bytes()))))
	require.NoError(t, err)
	require.JSONEq(t, `{"balance":"1"}`, string(balance))
	mixedVMLogsSeal(t, h, func(ctx sdk.Context) {
		params := wasmtypes.DefaultParams()
		params.CodeUploadAccess = wasmtypes.AllowNobody
		params.InstantiateDefaultPermission = wasmtypes.AccessTypeNobody
		h.s.app.WasmKeeper.SetParams(ctx, params)
	})
	ctx = mixedVMLogsCommittedContext(h)
	require.JSONEq(t, `"Nobody"`, readParam("instantiateAccess"))
	require.JSONEq(t, `{"permission":"Nobody"}`, readParam("uploadAccess"))
	code, err := os.ReadFile("../contracts/wasm/cw20_base.wasm")
	require.NoError(t, err)
	_, err = wasmkeeper.NewDefaultPermissionKeeper(h.s.app.WasmKeeper).Create(ctx, h.sender.AccountAddress, code, nil)
	require.Error(t, err, "declared disabled capability refuses uploads")
}

func mixedVMLogsABIType(t *testing.T, name string) abi.Type {
	t.Helper()
	typ, err := abi.NewType(name, "", nil)
	require.NoError(t, err)
	return typ
}

func TestMixedVMLogsBlockGlobalOrigin(t *testing.T) {
	source := mixedVMLogsSource(t, "eth_getLogs")
	require.Contains(t, source, "firstIndex + i")
	require.Contains(t, source, "blockLogs[firstIndex + i]")
	h := newCommittedFilterHarness(t, 100)
	earlier := mixedVMLogsAcceptEVM(t, h, nil, pendingFilterInitCode)
	// Two real LOG1 instructions in the second deployment, after an earlier tx's log.
	twice := append(append([]byte{}, pendingFilterInitCode[:len(pendingFilterInitCode)-1]...), pendingFilterInitCode...)
	target := mixedVMLogsAcceptEVM(t, h, nil, twice)
	block := mixedVMLogsSeal(t, h, nil)
	blockHash := common.BytesToHash(block.BlockID.Hash)
	all := mixedVMLogsLogs(t, h, map[string]any{"blockHash": blockHash})
	require.Len(t, all, 3)
	require.Equal(t, earlier, all[0].TxHash)
	filtered := mixedVMLogsLogs(t, h, map[string]any{"blockHash": blockHash, "address": all[1].Address})
	require.Len(t, filtered, 2)
	require.Equal(t, uint(1), filtered[0].Index, "the transaction's first log has a nonzero block-global origin")
	for i, log := range filtered {
		require.Equal(t, target, log.TxHash)
		require.Equal(t, filtered[0].Index+uint(i), log.Index)
		require.Equal(t, filtered[0].TxIndex, log.TxIndex)
		require.Equal(t, all[log.Index], log)
	}
}

func TestMixedVMLogsSubscriptionRestartRecovery(t *testing.T) {
	source := mixedVMLogsSource(t, "eth_subscribe")
	require.Contains(t, source, "ws.close()")
	require.Contains(t, source, "old subscription is connection-local")
	require.Contains(t, source, "HTTP recovers every committed event across the gap")
	h := newCommittedFilterHarness(t, 100)
	blockClient := mixedVMLogsBlockClient(t, h)
	config := rpcconfig.DefaultConfig
	config.HTTPAddress, config.WSPort, config.IPRateLimitRPS = TestAddr, 0, 0
	config.FilterTimeout = time.Minute
	open := func() (*evmrpc.HTTPServer, *websocket.Conn, *evmrpc.BlockHeaderNotifier) {
		t.Helper()
		notifier := evmrpc.NewBlockHeaderNotifier(8)
		service, err := evmrpc.NewEVMWebSocketServer(config, h.tm, &h.s.app.EvmKeeper,
			h.s.app.BeginBlockKeepers, h.s.app.BaseApp, h.s.app.TracerAnteHandler, h.s.ctxProvider,
			func(int64) client.TxConfig { return h.s.app.GetTxConfig() }, "", nil, notifier)
		require.NoError(t, err)
		t.Cleanup(service.Stop)
		require.NoError(t, service.Start())
		server := service.(*evmrpc.HTTPServer)
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+server.ListenAddr()+"/", nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return server, conn, notifier
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	subscribe := func(conn *websocket.Conn) string {
		t.Helper()
		require.NoError(t, conn.SetWriteDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "eth_subscribe", "params": []any{"newHeads"}}))
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		var response struct {
			Result string
			Error  json.RawMessage
		}
		require.NoError(t, conn.ReadJSON(&response))
		require.Empty(t, response.Error)
		require.NotEmpty(t, response.Result)
		return response.Result
	}
	publish := func(n *evmrpc.BlockHeaderNotifier, height int64) {
		t.Helper()
		block := h.blocks[height]
		require.NotNil(t, block)
		header := block.Block.Header.ToProto()
		n.OnBlockCommitted(block.BlockID.Hash, header, &abci.ResponseFinalizeBlock{AppHash: block.Block.AppHash})
	}
	read := func(conn *websocket.Conn, id string, height int64) {
		t.Helper()
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		var notification struct {
			Method string
			Params struct {
				Subscription string
				Result       map[string]json.RawMessage
			}
		}
		require.NoError(t, conn.ReadJSON(&notification))
		require.Equal(t, "eth_subscription", notification.Method)
		require.Equal(t, id, notification.Params.Subscription)
		head := notification.Params.Result
		var number hexutil.Uint64
		require.NoError(t, json.Unmarshal(head["number"], &number))
		require.Equal(t, uint64(height), uint64(number))
		var canonical map[string]json.RawMessage
		require.NoError(t, blockClient.CallContext(ctx, &canonical, "eth_getBlockByNumber", hexutil.EncodeUint64(uint64(height)), false))
		for key, value := range head {
			require.JSONEq(t, string(canonical[key]), string(value), "newHeads field %s agrees with the committed block", key)
		}
	}
	server, ws, notifier := open()
	oldID := subscribe(ws)
	first := mixedVMLogsCommit(t, h, 1)
	firstHeight := h.s.committed
	publish(notifier, firstHeight)
	read(ws, oldID, firstHeight)
	require.NoError(t, ws.Close())
	server.Stop()
	var closed json.RawMessage
	require.Error(t, ws.ReadJSON(&closed), "the stopped transport cannot deliver notifications")
	gap := mixedVMLogsCommit(t, h, 1)
	gapHeight := h.s.committed
	_, ws, notifier = open()
	require.NoError(t, ws.SetWriteDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, ws.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "eth_unsubscribe", "params": []any{oldID}}))
	require.NoError(t, ws.SetReadDeadline(time.Now().Add(5*time.Second)))
	var stale struct{ Error struct{ Message string } }
	require.NoError(t, ws.ReadJSON(&stale))
	require.Contains(t, stale.Error.Message, "subscription not found")
	freshID := subscribe(ws)
	require.NotEqual(t, oldID, freshID)
	last := mixedVMLogsCommit(t, h, 1)
	lastHeight := h.s.committed
	publish(notifier, lastHeight)
	read(ws, freshID, lastHeight)
	logs := mixedVMLogsLogs(t, h, map[string]any{"fromBlock": hexutil.EncodeUint64(uint64(firstHeight)), "toBlock": hexutil.EncodeUint64(uint64(lastHeight))})
	require.Len(t, logs, 3)
	for i, hash := range []common.Hash{first[0], gap[0], last[0]} {
		require.Equal(t, hash, logs[i].TxHash)
		canonical := mixedVMLogsReadBlock(t, blockClient, hexutil.EncodeUint64(logs[i].BlockNumber))
		require.Equal(t, canonical.Hash, logs[i].BlockHash)
		require.Contains(t, canonical.Transactions, hash)
	}
	require.Equal(t, uint64(gapHeight), logs[1].BlockNumber, "HTTP catch-up recovers the event committed during downtime")
	recovered := mixedVMLogsLogs(t, h, map[string]any{"blockHash": logs[1].BlockHash})
	require.Equal(t, []ethtypes.Log{logs[1]}, recovered)
}
