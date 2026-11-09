package evmrpc_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type repair339View struct {
	path     string
	http, ws *rpc.Client
	indices  []int
}

func repair339Views(t *testing.T, http, ws *evmrpc.HTTPServer) []repair339View {
	t.Helper()
	var views []repair339View
	for i, path := range []string{"/", "/chain/1", "/chain/2"} {
		indices := [][]int{{0, 1, 2, 3}, {1, 2}, {3}}[i]
		views = append(views, repair339View{path, domainHTTPClient(t, http, path), domainWSClient(t, ws, path), indices})
	}
	return views
}

func (v repair339View) allows(index int) bool {
	for _, i := range v.indices {
		if i == index {
			return true
		}
	}
	return false
}

func repair339Committed(t *testing.T) (*domainWSHarness, []repair339View) {
	t.Helper()
	h := newDomainWSHarness(t)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.HTTPPort, config.IPRateLimitRPS = TestAddr, 0, 0
	config.FilterTimeout = time.Minute
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return h.ctx
		}
		return h.ctx.WithBlockHeight(height)
	}
	service, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp,
		h.app.TracerAnteHandler, provider, func(int64) client.TxConfig { return TxConfig }, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	require.NoError(t, service.Start())
	h.http = service.(*evmrpc.HTTPServer)
	return h, repair339Views(t, h.http, h.ws)
}

func repair339Call(t *testing.T, c *rpc.Client, method string, args ...interface{}) json.RawMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result json.RawMessage
	require.NoError(t, c.CallContext(ctx, &result, method, args...), method)
	return result
}

func repair339Txs(t *testing.T) []*ethtypes.Transaction {
	t.Helper()
	var result []*ethtypes.Transaction
	for _, tx := range []sdk.Tx{MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx} {
		msg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
		require.True(t, ok)
		etx, _ := msg.AsTransaction()
		require.NotNil(t, etx)
		result = append(result, etx)
	}
	return result
}

func repair339Raw(t *testing.T, c *rpc.Client, want *ethtypes.Transaction, visible bool, method string, args ...interface{}) {
	t.Helper()
	result := repair339Call(t, c, method, args...)
	var raw hexutil.Bytes
	require.NoError(t, json.Unmarshal(result, &raw))
	if !visible {
		require.Empty(t, raw, "%s leaked %s", method, want.Hash())
		return
	}
	requireRawTransactionEnvelope(t, raw, want)
}

func TestRepair_339_TenantRawHash(t *testing.T) {
	_, views := repair339Committed(t)
	txs := repair339Txs(t)
	for _, view := range views {
		for _, c := range []*rpc.Client{view.http, view.ws} {
			for i, tx := range txs {
				repair339Raw(t, c, tx, view.allows(i), "eth_getRawTransactionByHash", tx.Hash())
				ordinary := repair339Call(t, c, "eth_getTransactionByHash", tx.Hash())
				if view.allows(i) {
					require.NotEqual(t, "null", string(ordinary))
				} else {
					require.JSONEq(t, "null", string(ordinary))
				}
			}
		}
	}
	// The consensus-client peer's committed fixture also supplies the real mempool
	// wire bytes; the decoder, domain index, RPC handler and transports remain real.
	previous := UnconfirmedTx
	t.Cleanup(func() { UnconfirmedTx = previous })
	for i, tx := range []sdk.Tx{MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx} {
		UnconfirmedTx = tx
		for _, view := range views {
			for _, c := range []*rpc.Client{view.http, view.ws} {
				repair339Raw(t, c, txs[i], view.allows(i), "eth_getRawTransactionByHash", txs[i].Hash())
				ordinary := repair339Call(t, c, "eth_getTransactionByHash", txs[i].Hash())
				if view.allows(i) {
					require.NotEqual(t, "null", string(ordinary))
				} else {
					require.JSONEq(t, "null", string(ordinary))
				}
			}
		}
	}
}

func repair339Indexed(t *testing.T, method, ordinary, block string) {
	t.Helper()
	_, views := repair339Committed(t)
	txs := repair339Txs(t)
	for _, view := range views {
		for _, c := range []*rpc.Client{view.http, view.ws} {
			for i, tx := range txs {
				repair339Raw(t, c, tx, view.allows(i), method, block, hexutil.Uint(i))
				result := repair339Call(t, c, ordinary, block, hexutil.Uint(i))
				if !view.allows(i) {
					require.JSONEq(t, "null", string(result))
				} else {
					var decoded struct {
						Hash             common.Hash
						TransactionIndex hexutil.Uint64
					}
					require.NoError(t, json.Unmarshal(result, &decoded))
					require.Equal(t, tx.Hash(), decoded.Hash)
					require.Equal(t, hexutil.Uint64(i), decoded.TransactionIndex)
				}
			}
			for _, index := range []hexutil.Uint{4, 1 << 32} {
				var raw hexutil.Bytes
				require.NoError(t, json.Unmarshal(repair339Call(t, c, method, block, index), &raw))
				require.Empty(t, raw)
			}
		}
	}
}

func TestRepair_339_TenantRawNumber(t *testing.T) {
	repair339Indexed(t, "eth_getRawTransactionByBlockNumberAndIndex", "eth_getTransactionByBlockNumberAndIndex", "0x2")
}

func TestRepair_339_TenantRawBlockHash(t *testing.T) {
	repair339Indexed(t, "eth_getRawTransactionByBlockHashAndIndex", "eth_getTransactionByBlockHashAndIndex", MultiTxBlockHash)
}

func TestRepair_339_TenantWSContext(t *testing.T) {
	h, views := repair339Committed(t)
	// Persist the executed keeper state so ordinary account reads can validate
	// the same committed store version as the shared block fixture.
	h.app.SetDeliverStateToCommit()
	committedStore := h.app.WriteState()
	for height := int64(1); height <= MockHeight2; height++ {
		require.Equal(t, height, committedStore.Commit(true).Version)
	}
	h.ctx = h.ctx.WithMultiStore(committedStore.CacheMultiStore())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	criteria := map[string]interface{}{"fromBlock": "0x2", "toBlock": "0x2"}
	filterIDs := make([]rpc.ID, len(views))
	for round := 0; round < 2; round++ {
		for vi := range views {
			view := &views[vi]
			if round == 1 {
				view.ws.Close()
				view.ws = domainWSClient(t, h.ws, view.path)
				var recovered []ethtypes.Log
				require.NoError(t, view.ws.CallContext(ctx, &recovered, "eth_getFilterLogs", filterIDs[vi]))
				require.Len(t, recovered, len(view.indices))
			}
			for _, call := range []struct {
				method string
				args   []interface{}
			}{
				{"eth_chainId", nil}, {"net_version", nil},
				{"eth_getBlockByNumber", []interface{}{"0x2", false}},
				{"eth_getBlockByNumber", []interface{}{"0x2", true}},
				{"eth_getBlockByHash", []interface{}{MultiTxBlockHash, true}},
				{"eth_getBlockTransactionCountByNumber", []interface{}{"0x2"}},
				{"eth_getBlockTransactionCountByHash", []interface{}{MultiTxBlockHash}},
				{"eth_getBlockReceipts", []interface{}{"0x2"}},
				{"eth_getLogs", []interface{}{criteria}},
				{"eth_getLogs", []interface{}{map[string]interface{}{"blockHash": MultiTxBlockHash}}},
				{"eth_getBalance", []interface{}{common.HexToAddress("0x123456"), "latest"}},
			} {
				httpResult := repair339Call(t, view.http, call.method, call.args...)
				wsResult := repair339Call(t, view.ws, call.method, call.args...)
				require.JSONEq(t, string(httpResult), string(wsResult), "%s %s round %d", view.path, call.method, round)
			}
			var block struct{ Transactions []common.Hash }
			require.NoError(t, json.Unmarshal(repair339Call(t, view.ws, "eth_getBlockByNumber", "0x2", false), &block))
			var expected []common.Hash
			for _, i := range view.indices {
				expected = append(expected, h.hashes[i])
			}
			require.Equal(t, expected, block.Transactions)
			for i, hash := range h.hashes {
				for _, method := range []string{"eth_getTransactionByHash", "eth_getTransactionReceipt", "eth_getRawTransactionByHash"} {
					httpResult := repair339Call(t, view.http, method, hash)
					wsResult := repair339Call(t, view.ws, method, hash)
					require.JSONEq(t, string(httpResult), string(wsResult), "%s %s", view.path, method)
					if method != "eth_getRawTransactionByHash" {
						if view.allows(i) {
							require.NotEqual(t, "null", string(wsResult))
						} else {
							require.JSONEq(t, "null", string(wsResult))
						}
					}
				}
				for _, method := range []string{"eth_getTransactionByBlockNumberAndIndex", "eth_getRawTransactionByBlockNumberAndIndex", "eth_getTransactionByBlockHashAndIndex", "eth_getRawTransactionByBlockHashAndIndex"} {
					selector := "0x2"
					if method == "eth_getTransactionByBlockHashAndIndex" || method == "eth_getRawTransactionByBlockHashAndIndex" {
						selector = MultiTxBlockHash
					}
					require.JSONEq(t, string(repair339Call(t, view.http, method, selector, hexutil.Uint(i))), string(repair339Call(t, view.ws, method, selector, hexutil.Uint(i))))
				}
			}
			for _, c := range []*rpc.Client{view.http, view.ws} {
				var id rpc.ID
				require.NoError(t, c.CallContext(ctx, &id, "eth_newFilter", criteria))
				if c == view.ws {
					filterIDs[vi] = id
				}
				for _, method := range []string{"eth_getFilterLogs", "eth_getFilterChanges"} {
					var logs []ethtypes.Log
					require.NoError(t, c.CallContext(ctx, &logs, method, id))
					var hashes []common.Hash
					for _, log := range logs {
						hashes = append(hashes, log.TxHash)
					}
					require.Equal(t, expected, hashes)
				}
				var empty []ethtypes.Log
				require.NoError(t, c.CallContext(ctx, &empty, "eth_getFilterChanges", id))
				require.Empty(t, empty)
			}
		}
		var forbidden []ethtypes.Log
		require.Error(t, views[2].ws.CallContext(ctx, &forbidden, "eth_getFilterLogs", filterIDs[1]))
		var rootReadsTenant []ethtypes.Log
		require.NoError(t, views[0].ws.CallContext(ctx, &rootReadsTenant, "eth_getFilterLogs", filterIDs[1]))
		require.Len(t, rootReadsTenant, 2)
		repair339SharedHeads(t, h, views)
	}
}

func repair339SharedHeads(t *testing.T, h *domainWSHarness, views []repair339View) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	channels := make([]chan map[string]json.RawMessage, len(views))
	for i, view := range views {
		channels[i] = make(chan map[string]json.RawMessage, 1)
		sub, err := view.ws.EthSubscribe(ctx, channels[i], "newHeads")
		require.NoError(t, err)
		defer sub.Unsubscribe()
	}
	height := int64(MockHeight2)
	block, err := (&MockClient{}).Block(ctx, &height)
	require.NoError(t, err)
	results, err := (&MockClient{}).BlockResults(ctx, &height)
	require.NoError(t, err)
	header := block.Block.Header.ToProto()
	header.ProposerAddress = common.BytesToAddress(header.ProposerAddress).Bytes()
	h.notifier.OnBlockCommitted(block.BlockID.Hash, header, &abci.ResponseFinalizeBlock{AppHash: block.Block.AppHash, TxResults: results.TxsResults})
	var root map[string]json.RawMessage
	for i, channel := range channels {
		select {
		case head := <-channel:
			if i == 0 {
				root = head
				require.JSONEq(t, `"`+MultiTxBlockHash+`"`, string(head["hash"]))
			} else {
				require.Equal(t, root, head)
			}
		case <-ctx.Done():
			t.Fatalf("shared head missing for %s: %v", views[i].path, ctx.Err())
		}
	}
}

func TestRepair_339_TenantPendingRaw(t *testing.T) {
	sender := occutils.NewSigner()
	q := newPendingFilterSequencer(t, sender)
	state, write := sdk.NewContext(q.app.CommitMultiStore(), q.ctxProvider(evmrpc.LatestCtxHeight).BlockHeader(), false).CacheContext()
	params := q.app.LayerXChainsKeeper.GetParams(state)
	operator := common.HexToAddress("0x1234")
	coins := sdk.NewCoins(sdk.NewCoin(q.app.EvmKeeper.GetBaseDenom(state), params.MinBond.Add(params.CreationFee).MulRaw(2)))
	require.NoError(t, q.app.BankKeeper.MintCoins(state, "evm", coins))
	require.NoError(t, q.app.BankKeeper.SendCoinsFromModuleToAccount(state, "evm", sdk.AccAddress(operator.Bytes()), coins))
	for _, name := range []string{"repair-first", "repair-second"} {
		_, err := q.app.LayerXChainsKeeper.Create(state, operator, name, 4*pendingFilterDeployGas, params.MinByteAllowance,
			0, params.MinBond.Add(params.CreationFee), q.app.EvmKeeper.GetNextBaseFeePerGas(state))
		require.NoError(t, err)
	}
	write()
	open, err := q.exec.Snapshot()
	require.NoError(t, err)
	_, err = q.exec.Seal(q.committed + 1)
	require.NoError(t, err)
	q.committed++
	require.Equal(t, q.committed, q.app.CommitMultiStore().Commit(true).Version)
	require.NoError(t, q.feed.Committed(q.committed))
	require.NoError(t, q.exec.Open(q.committed+1, open.BlockTime().Add(time.Second).UnixMilli()))
	pending, err := evmrpc.NewPendingOption(q.exec)
	require.NoError(t, err)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.HTTPPort, config.WSPort, config.IPRateLimitRPS = TestAddr, 0, 0, 0
	config.FilterTimeout = time.Minute
	httpService, err := evmrpc.NewEVMHTTPServer(config, &MockClient{latestOverride: q.committed}, &q.app.EvmKeeper,
		q.app.BeginBlockKeepers, q.app.BaseApp, q.app.TracerAnteHandler, q.ctxProvider, q.txConfig, "", nil, q.option, pending)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	require.NoError(t, httpService.Start())
	wsService, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{latestOverride: q.committed}, &q.app.EvmKeeper,
		q.app.BeginBlockKeepers, q.app.BaseApp, q.app.TracerAnteHandler, q.ctxProvider, q.txConfig, "", nil, nil, q.option, pending)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	require.NoError(t, wsService.Start())
	views := repair339Views(t, httpService.(*evmrpc.HTTPServer), wsService.(*evmrpc.HTTPServer))
	installed := make([][2][3]rpc.ID, len(views))
	for vi, view := range views {
		for ci, c := range []*rpc.Client{view.http, view.ws} {
			for fi := range installed[vi][ci] {
				require.NoError(t, json.Unmarshal(repair339Call(t, c, "eth_newPendingTransactionFilter", false), &installed[vi][ci][fi]))
				require.NotEmpty(t, installed[vi][ci][fi])
			}
		}
	}
	produce := func(firstNonce int) []*ethtypes.Transaction {
		t.Helper()
		var txs []*ethtypes.Transaction
		for i, chainID := range []*big.Int{q.app.EvmKeeper.ChainID(open), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(2)} {
			nonce := firstNonce + i
			tx, err := ethtypes.SignNewTx(sender.EvmPrivateKey, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
				ChainID: chainID, Nonce: uint64(nonce), GasFeeCap: big.NewInt(pendingGasPriceWei), GasTipCap: big.NewInt(0), Gas: pendingFilterDeployGas, Data: pendingFilterInitCode,
			})
			require.NoError(t, err)
			data, err := ethtx.NewTxDataFromTx(tx)
			require.NoError(t, err)
			msg, err := evmtypes.NewMsgEVMTransaction(data)
			require.NoError(t, err)
			builder := q.txConfig(0).NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			builder.SetGasLimit(pendingFilterDeployGas)
			encoded, err := q.txConfig(0).TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			accepted := pendingFilterReceipt(t, q.app, q.exec, q.feed, encoded, tx.Hash())
			// Consensus index zero is reserved for MsgSequenceBatch. RPC lookup
			// positions enumerate the EVM transactions from zero; receipts retain
			// their canonical consensus positions starting at one.
			require.Equal(t, nonce+1, accepted.TxIndex)
			require.Equal(t, uint64(nonce+1), accepted.Seq)
			txs = append(txs, tx)
		}
		return txs
	}
	poll := func(c *rpc.Client, id rpc.ID, want []common.Hash) {
		t.Helper()
		var got []common.Hash
		require.NoError(t, json.Unmarshal(repair339Call(t, c, "eth_getFilterChanges", id), &got))
		require.Equal(t, want, got)
		require.JSONEq(t, "[]", string(repair339Call(t, c, "eth_getFilterChanges", id)))
	}
	wantHashes := func(view repair339View, txs []*ethtypes.Transaction) []common.Hash {
		var hashes []common.Hash
		for i, tx := range txs {
			if view.allows(i % 4) {
				hashes = append(hashes, tx.Hash())
			}
		}
		return hashes
	}
	pollBatch := func(txs []*ethtypes.Transaction, buffered bool) {
		t.Helper()
		for vi, view := range views {
			for ci, c := range []*rpc.Client{view.http, view.ws} {
				fi := 0
				if buffered {
					fi = 2
				}
				if vi != 0 {
					other := views[3-vi]
					cross := []*rpc.Client{other.http, other.ws}[ci]
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					var result json.RawMessage
					err := cross.CallContext(ctx, &result, "eth_getFilterChanges", installed[vi][ci][fi])
					cancel()
					require.EqualError(t, err, "filter belongs to another chain")
					var coded rpc.Error
					require.ErrorAs(t, err, &coded)
					require.Equal(t, -32000, coded.ErrorCode())
				}
				want := wantHashes(view, txs)
				poll(c, installed[vi][ci][fi], want)
				if !buffered {
					root := []*rpc.Client{views[0].http, views[0].ws}[ci]
					poll(root, installed[vi][ci][1], want)
					var visible []common.Hash
					for i, tx := range txs {
						result := repair339Call(t, c, "eth_getTransactionByHash", tx.Hash())
						if view.allows(i % 4) {
							var got struct{ Hash common.Hash }
							require.NoError(t, json.Unmarshal(result, &got))
							require.Equal(t, tx.Hash(), got.Hash)
							visible = append(visible, got.Hash)
						} else {
							require.JSONEq(t, "null", string(result))
						}
					}
					require.Equal(t, want, visible)
				}
			}
		}
	}
	txs := produce(0)
	pollBatch(txs, false)
	for round := 0; round < 2; round++ {
		for vi := range views {
			view := &views[vi]
			if round == 1 {
				view.ws.Close()
				view.ws = domainWSClient(t, wsService.(*evmrpc.HTTPServer), view.path)
			}
			for _, c := range []*rpc.Client{view.http, view.ws} {
				for i, tx := range txs {
					repair339Raw(t, c, tx, view.allows(i), "eth_getRawTransactionByHash", tx.Hash())
					repair339Raw(t, c, tx, view.allows(i), "eth_getRawTransactionByBlockNumberAndIndex", "pending", hexutil.Uint(i))
					for _, lookup := range []struct {
						method string
						args   []interface{}
					}{
						{"eth_getTransactionByHash", []interface{}{tx.Hash()}},
						{"eth_getTransactionByBlockNumberAndIndex", []interface{}{"pending", hexutil.Uint(i)}},
					} {
						result := repair339Call(t, c, lookup.method, lookup.args...)
						if view.allows(i) {
							var got struct {
								Hash             common.Hash
								TransactionIndex hexutil.Uint64
								BlockHash        *common.Hash
							}
							require.NoError(t, json.Unmarshal(result, &got))
							require.Equal(t, tx.Hash(), got.Hash)
							require.Equal(t, hexutil.Uint64(i+1), got.TransactionIndex)
							require.Nil(t, got.BlockHash)
						} else {
							require.JSONEq(t, "null", string(result))
						}
					}
				}
				var count hexutil.Uint
				require.NoError(t, json.Unmarshal(repair339Call(t, c, "eth_getBlockTransactionCountByNumber", "pending"), &count))
				require.Equal(t, hexutil.Uint(len(view.indices)), count)
				var block struct{ Transactions []common.Hash }
				require.NoError(t, json.Unmarshal(repair339Call(t, c, "eth_getBlockByNumber", "pending", false), &block))
				var want []common.Hash
				for _, i := range view.indices {
					want = append(want, txs[i].Hash())
				}
				require.Equal(t, want, block.Transactions)
				var raw hexutil.Bytes
				require.NoError(t, json.Unmarshal(repair339Call(t, c, "eth_getRawTransactionByBlockNumberAndIndex", "pending", "0x4"), &raw))
				require.Empty(t, raw)
			}
		}
	}
	second := produce(len(txs))
	pollBatch(second, false)
	txs = append(txs, second...)
	sealed, err := q.exec.Seal(q.committed + 1)
	require.NoError(t, err)
	require.Len(t, sealed.Txs, len(txs))
	sealed.Store.Write()
	q.committed++
	require.Equal(t, q.committed, q.app.CommitMultiStore().Commit(true).Version)
	require.NoError(t, q.feed.Committed(q.committed))
	for _, tx := range txs {
		_, ok := q.exec.PendingTx(tx.Hash())
		require.False(t, ok)
	}
	pollBatch(txs, true)
	for vi, view := range views {
		for ci, c := range []*rpc.Client{view.http, view.ws} {
			for _, id := range installed[vi][ci] {
				require.JSONEq(t, "[]", string(repair339Call(t, c, "eth_getFilterChanges", id)))
				var removed bool
				require.NoError(t, json.Unmarshal(repair339Call(t, c, "eth_uninstallFilter", id), &removed))
				require.True(t, removed)
			}
		}
	}
}
