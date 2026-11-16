package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestEthBlockReceiptsFollowBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	conf.Consensus.UnsafeCommitTimeoutOverride = 2 * time.Second
	bypassCommitTimeout := false
	conf.Consensus.UnsafeBypassCommitTimeoutOverride = &bypassCommitTimeout
	consensusApp := kvstore.NewApplication()
	node, closeNode, err := rpctest.StartTendermint(ctx, conf, consensusApp, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closeNode(context.Background()))
		require.NoError(t, consensusApp.Close())
	})
	service, ok := node.(rpclocal.NodeService)
	require.True(t, ok)
	tm, err := rpclocal.New(service)
	require.NoError(t, err)
	waitForHeight := func(t *testing.T, height int64) {
		t.Helper()
		require.Eventually(t, func() bool {
			status, err := tm.Status(ctx)
			return err == nil && status.SyncInfo.LatestBlockHeight >= height
		}, 10*time.Second, 10*time.Millisecond)
	}
	waitForHeight(t, 2)

	for _, shell := range []bool{true, false} {
		for _, evmTransactions := range []bool{true, false} {
			t.Run(fmt.Sprintf("shell_%t_evm_transactions_%t", shell, evmTransactions), func(t *testing.T) {
				a := app.Setup(t, false, false, false)
				t.Cleanup(func() { require.NoError(t, a.Close()) })
				k := &a.EvmKeeper
				sdkCtx := a.GetContextForDeliverTx(nil).WithBlockHeight(1).
					WithChainID("hyperpax_125-1").WithClosestUpgradeName(LatestCtxUpgradeName)
				sdkCtx = sdkCtx.WithConsensusParams(a.GetConsensusParams(sdkCtx))
				fund := func(address common.Address, amount sdk.Int) {
					coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(sdkCtx), amount))
					require.NoError(t, a.BankKeeper.MintCoins(sdkCtx, "evm", coins))
					require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(sdkCtx, "evm", sdk.AccAddress(address.Bytes()), coins))
				}
				operatorKey, err := crypto.GenerateKey()
				require.NoError(t, err)
				operator := crypto.PubkeyToAddress(operatorKey.PublicKey)
				params := a.LayerXChainsKeeper.GetParams(sdkCtx)
				fund(operator, params.MinBond.Add(params.CreationFee))
				domain := (&domainScopeHarness{app: a, ctx: sdkCtx, operator: operator}).create(t, "receipt-domain")
				bank := banktypes.NewMsgSend(sdk.AccAddress(operator.Bytes()), sdk.AccAddress(operator.Bytes()), sdk.NewCoins(sdk.NewInt64Coin(k.GetBaseDenom(sdkCtx), 1)))
				builder := TxConfig.NewTxBuilder()
				require.NoError(t, builder.SetMsgs(bank))
				bankBytes, err := Encoder(builder.GetTx())
				require.NoError(t, err)
				txs := []sdk.Tx{builder.GetTx()}
				rawTxs := [][]byte{bankBytes}
				hashes := make([]common.Hash, 0, 2)
				if evmTransactions {
					for _, chainID := range []*big.Int{k.ChainID(sdkCtx), chainstypes.ChainIDOf(domain.Id)} {
						key, err := crypto.GenerateKey()
						require.NoError(t, err)
						fund(crypto.PubkeyToAddress(key.PublicKey), sdk.NewInt(10000000))
						tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
							ChainID: chainID, Gas: 200000, GasFeeCap: big.NewInt(1000000000000), To: &syntheticRealEmitter,
						})
						require.NoError(t, err)
						data, err := ethtx.NewDynamicFeeTx(tx)
						require.NoError(t, err)
						msg, err := types.NewMsgEVMTransaction(data)
						require.NoError(t, err)
						builder := TxConfig.NewTxBuilder()
						require.NoError(t, builder.SetMsgs(msg))
						raw, err := Encoder(builder.GetTx())
						require.NoError(t, err)
						txs = append(txs, builder.GetTx())
						rawTxs = append(rawTxs, raw)
						hashes = append(hashes, tx.Hash())
					}
				}

				status, err := tm.Status(ctx)
				require.NoError(t, err)
				height := status.SyncInfo.LatestBlockHeight + 1
				waitForHeight(t, height)
				height++
				for _, raw := range rawTxs {
					response, err := tm.BroadcastTxSync(ctx, raw)
					require.NoError(t, err)
					require.Equal(t, abci.CodeTypeOK, response.Code, response.Log)
				}
				waitForHeight(t, height)
				block, err := tm.Block(ctx, &height)
				require.NoError(t, err)
				require.Len(t, block.Block.Txs, len(rawTxs))
				for i, raw := range rawTxs {
					require.Equal(t, raw, []byte(block.Block.Txs[i]))
				}
				sdkCtx = sdkCtx.WithBlockHeight(height).WithBlockTime(block.Block.Time)
				k.BeginBlock(sdkCtx)
				results, err := tm.BlockResults(ctx, &height)
				require.NoError(t, err)
				k.SetTxResults(results.TxsResults)
				k.SetCode(sdkCtx, syntheticRealEmitter, filterLogCode([]common.Hash{syntheticRealTopic}, []common.Hash{syntheticRealTopic}))
				k.SetNonce(sdkCtx, syntheticRealEmitter, 1)
				for i, tx := range txs[1:] {
					msg := tx.GetMsgs()[0].(*types.MsgEVMTransaction)
					transaction, _ := msg.AsTransaction()
					from, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(transaction.ChainId()), transaction)
					require.NoError(t, err)
					execution := sdkCtx.WithTxIndex(i + 1).WithIsEVM(true).WithEVMNonce(0).
						WithEVMSenderAddress(from).WithEVMTxHash(transaction.Hash())
					require.NoError(t, ante.Preprocess(execution, msg, transaction.ChainId(), false))
					execution, err = ante.NewEVMFeeCheckDecorator(k, &a.UpgradeKeeper).AnteHandle(execution, tx, false,
						func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
					require.NoError(t, err)
					response, err := keeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(execution), msg)
					require.NoError(t, err)
					require.Empty(t, response.VmError)
				}
				for _, kind := range keeper.SyntheticReceiptKinds {
					if shell && kind == keeper.SyntheticReceiptPerps {
						require.NoError(t, a.PerpsKeeper.WriteEndBlockReceipt(sdkCtx, []perpstypes.EventRecord{
							{Name: perpstypes.EventMarkPriceUpdated, Args: []any{big.NewInt(0), big.NewInt(50000), big.NewInt(49990), big.NewInt(50010)}},
						}))
					} else {
						_, err := k.CallEVM(keeper.SyntheticReceiptContext(sdkCtx, kind), operator, &syntheticRealEmitter, nil, nil)
						require.NoError(t, err)
					}
				}
				require.NoError(t, k.FlushTransientReceipts(sdkCtx))
				generated := make([]common.Hash, 0, len(keeper.SyntheticReceiptKinds))
				for _, kind := range keeper.SyntheticReceiptKinds {
					generated = append(generated, keeper.SyntheticReceiptHash(kind, height))
				}
				for _, hash := range append(append([]common.Hash{}, hashes...), generated...) {
					require.Eventually(t, func() bool {
						receipt, err := k.GetReceipt(sdkCtx, hash)
						return err == nil && receipt != nil && receipt.BlockNumber == uint64(height)
					}, 5*time.Second, 10*time.Millisecond)
				}
				perps, err := k.GetReceipt(sdkCtx, generated[0])
				require.NoError(t, err)
				if shell {
					require.Equal(t, uint32(types.ShellEVMTxType), perps.TxType)
				} else {
					require.Equal(t, uint32(ethtypes.LegacyTxType), perps.TxType)
				}
				provider := func(h int64) sdk.Context {
					if h == evmrpc.LatestCtxHeight {
						return sdkCtx
					}
					return sdkCtx.WithBlockHeight(h)
				}
				configProvider := func(int64) client.TxConfig { return TxConfig }
				wm := evmrpc.NewWatermarkManager(tm, provider, nil, nil)
				cache, mu := evmrpc.NewBlockCache(32), &sync.Mutex{}
				eth := evmrpc.NewBlockAPI(tm, k, provider, configProvider, evmrpc.ConnectionTypeHTTP, wm, cache, mu)
				pax := evmrpc.NewPaxBlockAPI(tm, k, provider, configProvider, evmrpc.ConnectionTypeHTTP, wm, cache, mu)
				transactions := evmrpc.NewTransactionAPI(tm, k, provider, configProvider, "", evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), wm, cache, mu)
				server := rpc.NewServer()
				t.Cleanup(server.Stop)
				require.NoError(t, server.RegisterName("eth", eth))
				require.NoError(t, server.RegisterName("eth", transactions))
				require.NoError(t, server.RegisterName("pax", pax))
				http := httptest.NewServer(server)
				t.Cleanup(http.Close)
				consumer, err := rpc.DialHTTP(http.URL)
				require.NoError(t, err)
				t.Cleanup(consumer.Close)
				tag := hexutil.EncodeUint64(uint64(height))
				blockHash := common.BytesToHash(block.BlockID.Hash)
				for _, selector := range []string{tag, blockHash.Hex()} {
					var receipts []map[string]interface{}
					require.NoError(t, consumer.CallContext(ctx, &receipts, "eth_getBlockReceipts", selector))
					require.NotNil(t, receipts)
					require.Len(t, receipts, len(hashes))
					for i, receipt := range receipts {
						require.Equal(t, hashes[i].Hex(), receipt["transactionHash"])
						require.Equal(t, hexutil.EncodeUint64(uint64(i)), receipt["transactionIndex"])
						require.Equal(t, hexutil.EncodeUint64(ethtypes.DynamicFeeTxType), receipt["type"])
						logs := receipt["logs"].([]interface{})
						require.Len(t, logs, 2)
						for j, entry := range logs {
							log := entry.(map[string]interface{})
							require.Equal(t, hashes[i].Hex(), log["transactionHash"])
							require.Equal(t, hexutil.EncodeUint64(uint64(i)), log["transactionIndex"])
							require.Equal(t, hexutil.EncodeUint64(uint64(2*i+j)), log["logIndex"])
						}
						var single map[string]interface{}
						require.NoError(t, consumer.CallContext(ctx, &single, "eth_getTransactionReceipt", hashes[i]))
						require.Equal(t, receipt, single)
					}
				}
				for _, full := range []bool{false, true} {
					for _, byHash := range []bool{false, true} {
						method, selector := "eth_getBlockByNumber", tag
						if byHash {
							method, selector = "eth_getBlockByHash", blockHash.Hex()
						}
						var served map[string]interface{}
						require.NoError(t, consumer.CallContext(ctx, &served, method, selector, full))
						list := served["transactions"].([]interface{})
						require.Len(t, list, len(hashes))
						for i, entry := range list {
							if full {
								transaction := entry.(map[string]interface{})
								require.Equal(t, hashes[i].Hex(), transaction["hash"])
								require.Equal(t, hexutil.EncodeUint64(uint64(i)), transaction["transactionIndex"])
							} else {
								require.Equal(t, hashes[i].Hex(), entry)
							}
						}
					}
				}
				for _, call := range []struct{ method, selector string }{
					{"eth_getBlockTransactionCountByNumber", tag},
					{"eth_getBlockTransactionCountByHash", blockHash.Hex()},
				} {
					var count hexutil.Uint
					require.NoError(t, consumer.CallContext(ctx, &count, call.method, call.selector))
					require.Equal(t, len(hashes), int(count))
				}
				var paxReceipts []map[string]interface{}
				require.NoError(t, consumer.CallContext(ctx, &paxReceipts, "pax_getBlockReceipts", tag))
				require.Len(t, paxReceipts, len(hashes)+len(generated))
				for i, hash := range generated {
					receipt := paxReceipts[len(hashes)+i]
					require.Equal(t, hash.Hex(), receipt["transactionHash"])
					require.Equal(t, hexutil.EncodeUint64(uint64(len(rawTxs)+i)), receipt["transactionIndex"])
					require.NotEmpty(t, receipt["logs"])
				}
				for _, id := range []uint32{domain.Id, domain.Id + 1} {
					scoped := evmrpc.WithDomainScope(ctx, evmrpc.DomainScope{ID: id, ChainID: chainstypes.ChainIDOf(id)})
					receipts, err := eth.GetBlockReceipts(scoped, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(height)))
					require.NoError(t, err)
					want := 0
					if evmTransactions && id == domain.Id {
						want = 1
						require.Len(t, receipts, 1)
						require.Equal(t, hashes[1], receipts[0]["transactionHash"])
						single, err := transactions.GetTransactionReceipt(scoped, hashes[1])
						require.NoError(t, err)
						require.Equal(t, receipts[0], single)
					}
					require.Len(t, receipts, want)
					served, err := eth.GetBlockByNumber(scoped, rpc.BlockNumber(height), false)
					require.NoError(t, err)
					require.Len(t, served["transactions"], want)
					count, err := eth.GetBlockTransactionCountByNumber(scoped, rpc.BlockNumber(height))
					require.NoError(t, err)
					require.NotNil(t, count)
					require.Equal(t, want, int(*count))
				}
			})
		}
	}
}
