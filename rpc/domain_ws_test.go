package evmrpc_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type domainWSHarness struct {
	*domainScopeHarness
	hashes   []common.Hash
	notifier *evmrpc.BlockHeaderNotifier
}

func newDomainWSHarness(t *testing.T) *domainWSHarness {
	t.Helper()
	h := &domainWSHarness{domainScopeHarness: newDomainScopeHarness(t), notifier: evmrpc.NewBlockHeaderNotifier(4)}
	t.Cleanup(func() { require.NoError(t, h.app.Close()) })
	h.ctx = h.ctx.WithBlockHeight(MockHeight2).WithBlockTime(mockBlockHeader(MockHeight2).Time).
		WithClosestUpgradeName(LatestCtxUpgradeName)
	second := h.create(t, "ws-second")
	require.Equal(t, uint32(2), second.Id)
	previous := []sdk.Tx{MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx}
	t.Cleanup(func() {
		MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx = previous[0], previous[1], previous[2], previous[3]
	})
	k := &h.app.EvmKeeper
	contract := common.HexToAddress("0x123456")
	k.SetCode(h.ctx, contract, common.FromHex("0x602a600052600160206000a100"))
	k.SetNonce(h.ctx, contract, 1)
	chainIDs := []*big.Int{big.NewInt(125), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(2)}
	positions := []int{1, 3, 4, 5}
	var txs []sdk.Tx
	var records []receiptstore.ReceiptRecord
	var blooms []ethtypes.Bloom
	for i, chainID := range chainIDs {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		from := crypto.PubkeyToAddress(key.PublicKey)
		coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(h.ctx), sdk.NewInt(10000000)))
		require.NoError(t, h.app.BankKeeper.MintCoins(h.ctx, "evm", coins))
		require.NoError(t, h.app.BankKeeper.SendCoinsFromModuleToAccount(h.ctx, "evm", sdk.AccAddress(from.Bytes()), coins))
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
		executionCtx := h.ctx.WithTxIndex(positions[i]).WithIsEVM(true).WithEVMNonce(0).WithEVMSenderAddress(from).WithEVMTxHash(tx.Hash())
		require.NoError(t, ante.Preprocess(executionCtx, msg, chainID, false))
		executionCtx, err = ante.NewEVMFeeCheckDecorator(k, &h.app.UpgradeKeeper).AnteHandle(executionCtx, builder.GetTx(), false,
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
	require.Equal(t, h.hashes[1:3], h.app.LayerXChainsKeeper.DomainTxs(h.ctx, 1, MockHeight2))
	require.Equal(t, h.hashes[3:], h.app.LayerXChainsKeeper.DomainTxs(h.ctx, 2, MockHeight2))
	require.NoError(t, k.ReceiptStore().SetReceipts(h.ctx, records))
	require.Eventually(t, func() bool {
		for _, hash := range h.hashes {
			if _, err := k.GetReceipt(h.ctx, hash); err != nil {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond)
	require.NoError(t, k.ReceiptStore().SetLatestVersion(MockHeight2))
	require.NoError(t, k.ReceiptStore().SetEarliestVersion(1))
	k.SetBlockBloom(h.ctx, blooms)
	k.SetEvmOnlyBlockBloom(h.ctx, blooms)
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return h.ctx
		}
		return h.ctx.WithBlockHeight(height)
	}
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.WSPort = TestAddr, 0
	config.IPRateLimitRPS = 0
	service, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{}, k, h.app.BeginBlockKeepers, h.app.BaseApp,
		h.app.TracerAnteHandler, provider, func(int64) client.TxConfig { return TxConfig }, "", nil, h.notifier)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	require.NoError(t, service.Start())
	h.ws = service.(*evmrpc.HTTPServer)
	return h
}

func TestChainPathLogSubscriptionDeliversOnlyTheChainsLogs(t *testing.T) {
	t.Run("committed", func(t *testing.T) {
		h := newDomainWSHarness(t)
		for _, criteria := range []map[string]any{
			{"fromBlock": "0x2", "toBlock": "0x2"},
			{"blockHash": MultiTxBlockHash},
		} {
			for _, view := range []struct {
				path string
				want []common.Hash
			}{{"/chain/1", h.hashes[1:3]}, {"/chain/2", h.hashes[3:]}, {"/", h.hashes}} {
				client := domainWSClient(t, h.ws, view.path)
				if view.path == "/chain/1" {
					assertDomainChainID(t, client, "0x7735941", "125000001")
				} else if view.path == "/chain/2" {
					assertDomainChainID(t, client, "0x7735942", "125000002")
				}
				logs := make(chan *ethtypes.Log, 8)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				sub, err := client.EthSubscribe(ctx, logs, "logs", criteria)
				cancel()
				require.NoError(t, err)
				t.Cleanup(sub.Unsubscribe)
				for _, hash := range view.want {
					select {
					case log := <-logs:
						require.Equal(t, hash, log.TxHash, "subscription %s with criteria %v", view.path, criteria)
						require.Equal(t, uint64(MockHeight2), log.BlockNumber)
						require.Equal(t, common.HexToHash(MultiTxBlockHash), log.BlockHash)
						require.Equal(t, common.HexToAddress("0x123456"), log.Address)
						require.Equal(t, []common.Hash{common.HexToHash("0x1")}, log.Topics)
					case err := <-sub.Err():
						t.Fatalf("subscription %s ended: %v", view.path, err)
					case <-time.After(5 * time.Second):
						t.Fatalf("subscription %s did not deliver %s", view.path, hash)
					}
				}
				select {
				case log := <-logs:
					t.Fatalf("subscription %s delivered an extra log: %+v", view.path, log)
				case <-time.After(200 * time.Millisecond):
				}
				sub.Unsubscribe()
			}
		}
	})
	t.Run("receipted", testDomainWSReceiptedLogs)
}

func testDomainWSReceiptedLogs(t *testing.T) {
	sender := utils.NewSigner()
	q := newPendingFilterSequencer(t, sender)
	state, write := sdk.NewContext(q.app.CommitMultiStore(), q.ctxProvider(evmrpc.LatestCtxHeight).BlockHeader(), false).CacheContext()
	params := q.app.LayerXChainsKeeper.GetParams(state)
	operator := common.HexToAddress("0x1234")
	coins := sdk.NewCoins(sdk.NewCoin(q.app.EvmKeeper.GetBaseDenom(state), params.MinBond.Add(params.CreationFee).MulRaw(2)))
	require.NoError(t, q.app.BankKeeper.MintCoins(state, "evm", coins))
	require.NoError(t, q.app.BankKeeper.SendCoinsFromModuleToAccount(state, "evm", sdk.AccAddress(operator.Bytes()), coins))
	for _, name := range []string{"ws-first", "ws-second"} {
		_, err := q.app.LayerXChainsKeeper.Create(state, operator, name, 2*pendingFilterDeployGas, params.MinByteAllowance,
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
	config.HTTPAddress, config.WSPort = TestAddr, 0
	config.IPRateLimitRPS = 0
	service, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{latestOverride: q.committed}, &q.app.EvmKeeper,
		q.app.BeginBlockKeepers, q.app.BaseApp, q.app.TracerAnteHandler, q.ctxProvider, q.txConfig, "", nil, nil, q.option, pending)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	require.NoError(t, service.Start())
	server := service.(*evmrpc.HTTPServer)
	type view struct {
		logs chan json.RawMessage
		sub  *rpc.ClientSubscription
		want []common.Hash
	}
	views := make([]view, 3)
	for i, path := range []string{"/", "/chain/1", "/chain/2"} {
		client := domainWSClient(t, server, path)
		views[i].logs = make(chan json.RawMessage, 8)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		views[i].sub, err = client.EthSubscribe(ctx, views[i].logs, "logs", map[string]any{"topics": [][]common.Hash{{pendingFilterTopic}}})
		cancel()
		require.NoError(t, err)
		t.Cleanup(views[i].sub.Unsubscribe)
	}
	for i, chainID := range []*big.Int{q.app.EvmKeeper.ChainID(open), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(2), chainstypes.ChainIDOf(1)} {
		tx, err := ethtypes.SignNewTx(sender.EvmPrivateKey, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
			ChainID: chainID, Nonce: uint64(i), GasFeeCap: big.NewInt(pendingGasPriceWei), GasTipCap: big.NewInt(0), Gas: pendingFilterDeployGas, Data: pendingFilterInitCode,
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
		pendingFilterReceipt(t, q.app, q.exec, q.feed, encoded, tx.Hash())
		views[0].want = append(views[0].want, tx.Hash())
		if id, ok := chainstypes.DomainOfChainID(chainID); ok {
			views[id].want = append(views[id].want, tx.Hash())
		}
	}
	for i, view := range views {
		for _, hash := range view.want {
			select {
			case raw := <-view.logs:
				var log struct {
					TxHash    common.Hash     `json:"transactionHash"`
					BlockHash *common.Hash    `json:"blockHash"`
					Seq       *hexutil.Uint64 `json:"seq"`
				}
				require.NoError(t, json.Unmarshal(raw, &log))
				require.Equal(t, hash, log.TxHash)
				require.Nil(t, log.BlockHash)
				require.NotNil(t, log.Seq)
			case err := <-view.sub.Err():
				t.Fatalf("view %d subscription ended: %v", i, err)
			case <-time.After(5 * time.Second):
				t.Fatalf("view %d did not deliver %s", i, hash)
			}
		}
		select {
		case raw := <-view.logs:
			t.Fatalf("view %d delivered an extra log: %s", i, raw)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestChainPathNewHeadsIsUnchanged(t *testing.T) {
	h := newDomainWSHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	channels := make([]chan map[string]json.RawMessage, 3)
	for i, path := range []string{"/", "/chain/1", "/chain/2"} {
		client := domainWSClient(t, h.ws, path)
		channels[i] = make(chan map[string]json.RawMessage, 1)
		sub, err := client.EthSubscribe(ctx, channels[i], "newHeads")
		require.NoError(t, err)
		t.Cleanup(sub.Unsubscribe)
	}
	height := int64(MockHeight2)
	block, err := (&MockClient{}).Block(ctx, &height)
	require.NoError(t, err)
	results, err := (&MockClient{}).BlockResults(ctx, &height)
	require.NoError(t, err)
	header := block.Block.Header.ToProto()
	header.ProposerAddress = common.BytesToAddress(header.ProposerAddress).Bytes()
	h.notifier.OnBlockCommitted(block.BlockID.Hash, header, &abci.ResponseFinalizeBlock{
		AppHash: block.Block.AppHash, TxResults: results.TxsResults,
	})
	var root map[string]json.RawMessage
	for i, channel := range channels {
		select {
		case head := <-channel:
			if i == 0 {
				root = head
				require.JSONEq(t, `"`+MultiTxBlockHash+`"`, string(head["hash"]))
				require.JSONEq(t, `"0x2"`, string(head["number"]))
				require.JSONEq(t, `"`+common.BytesToHash(block.Block.LastBlockID.Hash).Hex()+`"`, string(head["parentHash"]))
			} else {
				require.Equal(t, root, head)
			}
		case <-ctx.Done():
			t.Fatalf("view %d did not receive the shared head: %v", i, ctx.Err())
		}
	}
}
