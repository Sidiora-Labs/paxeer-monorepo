package evmrpc_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type domainReadsHarness struct {
	root, chain, other *rpc.Client
	hashes             []common.Hash
}

func newDomainReadsHarness(t *testing.T) *domainReadsHarness {
	t.Helper()
	previous := []sdk.Tx{MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx}
	t.Cleanup(func() {
		MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx = previous[0], previous[1], previous[2], previous[3]
	})
	application := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	ctx := application.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight2).WithChainID("hyperpax_125-1").
		WithBlockTime(mockBlockHeader(MockHeight2).Time).WithClosestUpgradeName(LatestCtxUpgradeName).
		WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 10000000}})
	k := &application.EvmKeeper
	fund := func(address common.Address, amount sdk.Int) {
		coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), amount))
		require.NoError(t, application.BankKeeper.MintCoins(ctx, "evm", coins))
		require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", sdk.AccAddress(address.Bytes()), coins))
	}
	operator := common.HexToAddress("0x1234")
	params := application.LayerXChainsKeeper.GetParams(ctx)
	fund(operator, params.MinBond.Add(params.CreationFee).MulRaw(3))
	domainHarness := &domainScopeHarness{app: application, ctx: ctx, operator: operator}
	first := domainHarness.create(t, "reads-first")
	second := domainHarness.create(t, "reads-second")
	require.Equal(t, uint32(1), first.Id)
	require.Equal(t, uint32(2), second.Id)
	contract := common.HexToAddress("0x123456")
	k.SetCode(ctx, contract, common.FromHex("0x602a600052600160206000a100"))
	k.SetNonce(ctx, contract, 1)
	chainIDs := []*big.Int{big.NewInt(125), chainstypes.ChainIDOf(first.Id), chainstypes.ChainIDOf(first.Id), chainstypes.ChainIDOf(second.Id)}
	txs := make([]sdk.Tx, 0, 4)
	h := &domainReadsHarness{}
	records := make([]receiptstore.ReceiptRecord, 0, 4)
	blooms := make([]ethtypes.Bloom, 0, 4)
	// These are the consensus transaction positions in setup_test.go's block 2.
	positions := []int{1, 3, 4, 5}
	for i, chainID := range chainIDs {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		from := crypto.PubkeyToAddress(key.PublicKey)
		fund(from, sdk.NewInt(10000000))
		tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
			ChainID: chainID, Gas: 100000, GasFeeCap: big.NewInt(1000000000000), GasTipCap: big.NewInt(0), To: &contract, Value: big.NewInt(0),
		})
		require.NoError(t, err)
		data, err := ethtx.NewDynamicFeeTx(tx)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(data)
		require.NoError(t, err)
		builder := TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		txs = append(txs, builder.GetTx())
		h.hashes = append(h.hashes, tx.Hash())
		executionCtx := ctx.WithTxIndex(positions[i]).WithIsEVM(true).WithEVMNonce(0).WithEVMSenderAddress(from).WithEVMTxHash(tx.Hash())
		require.NoError(t, ante.Preprocess(executionCtx, msg, chainID, false))
		executionCtx, err = ante.NewEVMFeeCheckDecorator(k, &application.UpgradeKeeper).AnteHandle(executionCtx, builder.GetTx(), false,
			func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
		require.NoError(t, err)
		response, err := evmkeeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(executionCtx), msg)
		require.NoError(t, err)
		require.Empty(t, response.VmError)
		receipt, err := k.GetTransientReceipt(executionCtx, tx.Hash(), uint64(positions[i]))
		require.NoError(t, err)
		require.Equal(t, uint32(1), receipt.Status)
		require.Len(t, receipt.Logs, 1)
		records = append(records, receiptstore.ReceiptRecord{TxHash: tx.Hash(), Receipt: receipt})
		blooms = append(blooms, ethtypes.BytesToBloom(receipt.LogsBloom))
	}
	MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx = txs[0], txs[1], txs[2], txs[3]
	require.Equal(t, h.hashes[1:3], application.LayerXChainsKeeper.DomainTxs(ctx, first.Id, MockHeight2))
	require.Equal(t, h.hashes[3:], application.LayerXChainsKeeper.DomainTxs(ctx, second.Id, MockHeight2))
	require.NoError(t, k.ReceiptStore().SetReceipts(ctx, records))
	require.Eventually(t, func() bool {
		for _, hash := range h.hashes {
			if _, err := k.GetReceipt(ctx, hash); err != nil {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond)
	require.NoError(t, k.ReceiptStore().SetLatestVersion(MockHeight2))
	require.NoError(t, k.ReceiptStore().SetEarliestVersion(1))
	k.SetBlockBloom(ctx, blooms)
	k.SetEvmOnlyBlockBloom(ctx, blooms)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.IPRateLimitRPS = 0
	config.FilterTimeout = time.Minute
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return ctx
		}
		return ctx.WithBlockHeight(height)
	}
	service, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, k, application.BeginBlockKeepers, application.BaseApp,
		application.TracerAnteHandler, provider, func(int64) client.TxConfig { return TxConfig }, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	require.NoError(t, service.Start())
	server := service.(*evmrpc.HTTPServer)
	h.root = domainHTTPClient(t, server, "/")
	h.chain = domainHTTPClient(t, server, "/chain/1")
	h.other = domainHTTPClient(t, server, "/chain/2")
	return h
}

func TestChainPathBlocksListOnlyTheChainsTransactions(t *testing.T) {
	h := newDomainReadsHarness(t)
	ctx := context.Background()
	for _, full := range []bool{false, true} {
		var root, chain map[string]json.RawMessage
		require.NoError(t, h.root.CallContext(ctx, &root, "eth_getBlockByNumber", "0x2", full))
		require.NoError(t, h.chain.CallContext(ctx, &chain, "eth_getBlockByNumber", "0x2", full))
		var txs []json.RawMessage
		require.NoError(t, json.Unmarshal(chain["transactions"], &txs))
		require.Len(t, txs, 2)
		for i, raw := range txs {
			var hash common.Hash
			if full {
				var tx struct {
					Hash             common.Hash
					TransactionIndex hexutil.Uint64
				}
				require.NoError(t, json.Unmarshal(raw, &tx))
				hash = tx.Hash
				require.Equal(t, hexutil.Uint64(i+1), tx.TransactionIndex)
			} else {
				require.NoError(t, json.Unmarshal(raw, &hash))
			}
			require.Equal(t, h.hashes[i+1], hash)
		}
		for field, value := range root {
			if field != "transactions" {
				require.JSONEq(t, string(value), string(chain[field]), field)
			}
		}
		var byHash map[string]json.RawMessage
		require.NoError(t, h.chain.CallContext(ctx, &byHash, "eth_getBlockByHash", MultiTxBlockHash, full))
		require.Equal(t, chain, byHash)
	}
	for _, call := range []struct{ method, arg string }{{"eth_getBlockTransactionCountByNumber", "0x2"}, {"eth_getBlockTransactionCountByHash", MultiTxBlockHash}} {
		var count hexutil.Uint
		require.NoError(t, h.chain.CallContext(ctx, &count, call.method, call.arg))
		require.Equal(t, hexutil.Uint(2), count)
	}
}

func TestChainPathIndexedReadsHideOtherChainsTransactions(t *testing.T) {
	h := newDomainReadsHarness(t)
	ctx := context.Background()
	for _, path := range []struct {
		name    string
		client  *rpc.Client
		indices []int
	}{
		{"chain/1", h.chain, []int{1, 2}},
		{"chain/2", h.other, []int{3}},
		{"root", h.root, []int{0, 1, 2, 3}},
	} {
		t.Run(path.name, func(t *testing.T) {
			for _, call := range []struct {
				method, block, missing string
			}{
				{"eth_getTransactionByBlockNumberAndIndex", "0x2", "0x1000"},
				{"eth_getTransactionByBlockHashAndIndex", MultiTxBlockHash, (common.Hash{}).Hex()},
			} {
				t.Run(call.method, func(t *testing.T) {
					for i := 0; i <= len(h.hashes); i++ {
						var result *struct {
							Hash             common.Hash
							TransactionIndex *hexutil.Uint64
						}
						require.NoError(t, path.client.CallContext(ctx, &result, call.method, call.block, hexutil.Uint(i)))
						visible := false
						for _, index := range path.indices {
							if i == index {
								visible = true
							}
						}
						if visible {
							require.NotNil(t, result, "canonical index %d", i)
							require.Equal(t, h.hashes[i], result.Hash)
							require.NotNil(t, result.TransactionIndex)
							require.Equal(t, hexutil.Uint64(i), *result.TransactionIndex)
						} else {
							require.Nil(t, result, "canonical index %d", i)
						}
					}
					var missing json.RawMessage
					require.NoError(t, path.client.CallContext(ctx, &missing, call.method, call.missing, "0x0"))
					require.JSONEq(t, "null", string(missing))
				})
			}
		})
	}
}

func TestChainPathHidesOtherChainsTransactionsAndReceipts(t *testing.T) {
	h := newDomainReadsHarness(t)
	ctx := context.Background()
	for _, method := range []string{"eth_getTransactionByHash", "eth_getTransactionReceipt"} {
		for i, hash := range h.hashes {
			var result map[string]interface{}
			require.NoError(t, h.chain.CallContext(ctx, &result, method, hash))
			if i == 1 || i == 2 {
				require.NotNil(t, result)
			} else {
				require.Nil(t, result)
			}
			require.NoError(t, h.other.CallContext(ctx, &result, method, hash))
			if i == 3 {
				require.NotNil(t, result)
			} else {
				require.Nil(t, result)
			}
		}
	}
	for _, selector := range []string{"0x2", MultiTxBlockHash} {
		var receipts []struct {
			TransactionHash  common.Hash
			TransactionIndex hexutil.Uint64
		}
		require.NoError(t, h.chain.CallContext(ctx, &receipts, "eth_getBlockReceipts", selector))
		require.Len(t, receipts, 2)
		for i, receipt := range receipts {
			require.Equal(t, h.hashes[i+1], receipt.TransactionHash)
			require.Equal(t, hexutil.Uint64(i+1), receipt.TransactionIndex)
		}
	}
	client := ethclient.NewClient(h.chain)
	receipt, err := client.TransactionReceipt(ctx, h.hashes[1])
	require.NoError(t, err)
	require.Equal(t, h.hashes[1], receipt.TxHash)
	_, err = client.TransactionReceipt(ctx, h.hashes[3])
	require.ErrorIs(t, err, ethereum.NotFound)
}

func TestChainPathFiltersLogsToTheChain(t *testing.T) {
	h := newDomainReadsHarness(t)
	ctx := context.Background()
	criteria := map[string]interface{}{"fromBlock": "0x2", "toBlock": "0x2"}
	assertLogs := func(logs []ethtypes.Log, want []common.Hash) {
		t.Helper()
		require.Len(t, logs, len(want))
		for i, log := range logs {
			require.Equal(t, want[i], log.TxHash)
			require.Equal(t, common.HexToHash(MultiTxBlockHash), log.BlockHash)
			require.Equal(t, uint(i+1), log.TxIndex)
			require.Equal(t, uint(i+1), log.Index)
		}
	}
	client := ethclient.NewClient(h.chain)
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: big.NewInt(2), ToBlock: big.NewInt(2)})
	require.NoError(t, err)
	assertLogs(logs, h.hashes[1:3])
	var byHash []ethtypes.Log
	require.NoError(t, h.chain.CallContext(ctx, &byHash, "eth_getLogs", map[string]interface{}{"blockHash": MultiTxBlockHash}))
	assertLogs(byHash, h.hashes[1:3])
	var id rpc.ID
	require.NoError(t, h.chain.CallContext(ctx, &id, "eth_newFilter", criteria))
	for _, reader := range []*rpc.Client{h.chain, h.root} {
		var logs []ethtypes.Log
		require.NoError(t, reader.CallContext(ctx, &logs, "eth_getFilterLogs", id))
		assertLogs(logs, h.hashes[1:3])
	}
	var logsFromOther []ethtypes.Log
	require.Error(t, h.other.CallContext(ctx, &logsFromOther, "eth_getFilterLogs", id))
	var changes rpc.ID
	require.NoError(t, h.chain.CallContext(ctx, &changes, "eth_newFilter", criteria))
	var changed []ethtypes.Log
	require.NoError(t, h.chain.CallContext(ctx, &changed, "eth_getFilterChanges", changes))
	assertLogs(changed, h.hashes[1:3])
	require.NoError(t, h.chain.CallContext(ctx, &changed, "eth_getFilterChanges", changes))
	require.Empty(t, changed)
}

func TestRootPathStillListsEveryTransaction(t *testing.T) {
	h := newDomainReadsHarness(t)
	ctx := context.Background()
	var block struct{ Transactions []common.Hash }
	require.NoError(t, h.root.CallContext(ctx, &block, "eth_getBlockByNumber", "0x2", false))
	require.Equal(t, h.hashes, block.Transactions)
	var count hexutil.Uint
	require.NoError(t, h.root.CallContext(ctx, &count, "eth_getBlockTransactionCountByNumber", "0x2"))
	require.Equal(t, hexutil.Uint(4), count)
	var receipts []struct{ TransactionHash common.Hash }
	require.NoError(t, h.root.CallContext(ctx, &receipts, "eth_getBlockReceipts", "0x2"))
	require.Len(t, receipts, 4)
	var logs []ethtypes.Log
	require.NoError(t, h.root.CallContext(ctx, &logs, "eth_getLogs", map[string]interface{}{"fromBlock": "0x2", "toBlock": "0x2"}))
	require.Len(t, logs, 4)
	for i, hash := range h.hashes {
		require.Equal(t, hash, receipts[i].TransactionHash)
		require.Equal(t, hash, logs[i].TxHash)
		for _, method := range []string{"eth_getTransactionByHash", "eth_getTransactionReceipt"} {
			var result map[string]interface{}
			require.NoError(t, h.root.CallContext(ctx, &result, method, hash))
			require.NotNil(t, result)
		}
	}
}
