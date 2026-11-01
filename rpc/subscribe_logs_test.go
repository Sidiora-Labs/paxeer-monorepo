package evmrpc

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func dialLogsSubscription(t *testing.T, api *SubscriptionAPI) *rpc.Client {
	t.Helper()
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", api))
	t.Cleanup(server.Stop)
	ws := httptest.NewServer(server.WebsocketHandler([]string{"*"}))
	t.Cleanup(ws.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := rpc.DialWebsocket(ctx, "ws"+strings.TrimPrefix(ws.URL, "http"), "")
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func TestLogsSubscriptionNeverNotifiesErrorPayload(t *testing.T) {
	for _, blockHash := range []bool{false, true} {
		name := "polling"
		if blockHash {
			name = "blockHash"
		}
		t.Run(name, func(t *testing.T) {
			provider := func(int64) sdk.Context { return sdk.Context{}.WithBlockHeight(1) }
			fetcher := &LogFetcher{watermarks: NewWatermarkManager(nil, provider, nil, nil)}
			crit := filters.FilterCriteria{FromBlock: big.NewInt(1), ToBlock: big.NewInt(2)}
			wireCrit := map[string]any{"fromBlock": "0x1", "toBlock": "0x2"}
			wantError := "after latest available block"
			if blockHash {
				hash := common.HexToHash("0x1234")
				crit = filters.FilterCriteria{BlockHash: &hash}
				wireCrit = map[string]any{"blockHash": hash}
				fetcher.watermarks = NewWatermarkManager(nil, nil, nil, nil)
				wantError = "unable to determine height information"
			}
			_, _, err := fetcher.GetLogsByFilters(context.Background(), crit, 0)
			require.ErrorContains(t, err, wantError)
			api := &SubscriptionAPI{logFetcher: fetcher, connectionType: ConnectionTypeWS}
			client := dialLogsSubscription(t, api)
			metrics := GetGlobalMetrics()
			active, errors := metrics.ActiveSubscriptions.Load(), metrics.SubscriptionErrors.Load()
			payloads := make(chan json.RawMessage, 4)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sub, err := client.EthSubscribe(ctx, payloads, "logs", wireCrit)
			require.NoError(t, err)
			defer sub.Unsubscribe()
			require.Eventually(t, func() bool {
				return metrics.ActiveSubscriptions.Load() == active && metrics.SubscriptionErrors.Load() == errors+1
			}, time.Second, time.Millisecond, "fetch failure must count one error and end the goroutine")
			select {
			case payload := <-payloads:
				t.Fatalf("fetch failure was sent as a log notification: %s", payload)
			case <-time.After(200 * time.Millisecond):
			}
		})
	}
}

func TestLogsSubscriptionOutlivesSubscribeCall(t *testing.T) {
	storeKey := sdk.NewKVStoreKey("evm")
	tkey := sdk.NewTransientStoreKey("evm_transient")
	ctx := testutil.DefaultContext(storeKey, tkey).WithBlockHeight(1).
		WithBlockTime(time.Unix(1_700_000_000, 0).UTC())
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 0
	store, err := receiptstore.NewReceiptStore(cfg, storeKey)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	registry := codectypes.NewInterfaceRegistry()
	evmtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), storeKey, tkey, "evm")
	k := evmkeeper.NewKeeper(storeKey, tkey, params, store, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	cache := NewBlockCache(4)
	var latest atomic.Int64
	provider := func(height int64) sdk.Context {
		if height == LatestCtxHeight {
			height = latest.Load()
		}
		return ctx.WithBlockHeight(height)
	}
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	address := common.HexToAddress("0x1234")
	topic := common.HexToHash("0x5678")
	commit := func(height int64) *ethtypes.Log {
		t.Helper()
		tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(k.ChainID(ctx)), &ethtypes.DynamicFeeTx{
			ChainID: k.ChainID(ctx), Nonce: uint64(height - 1), Gas: 100_000,
			GasFeeCap: big.NewInt(1_000_000_000), GasTipCap: big.NewInt(1), To: &address,
		})
		require.NoError(t, err)
		data, err := ethtx.NewDynamicFeeTx(tx)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(data)
		require.NoError(t, err)
		builder := txConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		encoded, err := txConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		block := &tmtypes.Block{
			Header:     tmtypes.Header{Height: height, Time: ctx.BlockTime().Add(time.Duration(height) * time.Second)},
			Data:       tmtypes.Data{Txs: []tmtypes.Tx{encoded}},
			LastCommit: &tmtypes.Commit{Height: height - 1},
		}
		blockID := tmtypes.BlockID{Hash: common.BigToHash(big.NewInt(height)).Bytes()}
		log := &ethtypes.Log{
			Address: address, Topics: []common.Hash{topic}, Data: []byte{byte(height)},
			BlockNumber: uint64(height), BlockHash: common.BytesToHash(blockID.Hash), TxHash: tx.Hash(),
		}
		bloom := ethtypes.CreateBloom(&ethtypes.Receipt{Logs: []*ethtypes.Log{log}})
		rcpt := &evmtypes.Receipt{
			TxHashHex: tx.Hash().Hex(), BlockNumber: uint64(height), Status: 1,
			TxType: uint32(ethtypes.DynamicFeeTxType), GasUsed: 21000, EffectiveGasPrice: 1_000_000_000,
			Logs: evmtypes.NewLogsFromEth([]*ethtypes.Log{log}), LogsBloom: bloom.Bytes(),
		}
		require.NoError(t, store.SetReceipts(ctx.WithBlockHeight(height), []receiptstore.ReceiptRecord{{TxHash: tx.Hash(), Receipt: rcpt}}))
		require.Eventually(t, func() bool {
			_, err := store.GetReceiptFromStore(ctx, tx.Hash())
			return err == nil
		}, 5*time.Second, time.Millisecond)
		ctx.KVStore(storeKey).Set([]byte("committed-block"), blockID.Hash)
		id := ctx.MultiStore().(sdk.CommitMultiStore).Commit(true)
		require.Equal(t, height, id.Version)
		cache.Add(height, &BlockCacheEntry{Block: &coretypes.ResultBlock{Block: block, BlockID: blockID}, Receipts: make(map[common.Hash]*evmtypes.Receipt)})
		latest.Store(height)
		return log
	}
	first := commit(1)
	fetcher := &LogFetcher{
		k: k, ctxProvider: provider, txConfigProvider: func(int64) client.TxConfig { return txConfig },
		filterConfig:    &FilterConfig{maxLog: DefaultMaxLogLimit, maxBlock: DefaultMaxBlockRange},
		dbReadSemaphore: make(chan struct{}, 1), globalBlockCache: cache, cacheCreationMutex: &sync.Mutex{},
		globalLogSlicePool: NewLogSlicePool(), watermarks: NewWatermarkManager(nil, provider, nil, store),
	}
	api := &SubscriptionAPI{logFetcher: fetcher, connectionType: ConnectionTypeWS}
	require.Nil(t, api.receiptLogs, "exercise polling, without a receipt log feed")
	rpcClient := dialLogsSubscription(t, api)
	metrics := GetGlobalMetrics()
	active, errors := metrics.ActiveSubscriptions.Load(), metrics.SubscriptionErrors.Load()
	payloads := make(chan *ethtypes.Log, 4)
	callCtx, cancelCall := context.WithTimeout(context.Background(), 5*time.Second)
	sub, err := rpcClient.EthSubscribe(callCtx, payloads, "logs", map[string]any{
		"fromBlock": "0x1", "address": []common.Address{address}, "topics": [][]common.Hash{{topic}},
	})
	cancelCall()
	require.NoError(t, err)
	defer sub.Unsubscribe()
	receive := func(want *ethtypes.Log) {
		t.Helper()
		select {
		case got := <-payloads:
			require.Equal(t, want, got)
		case err := <-sub.Err():
			t.Fatalf("subscription ended before receiving block %d: %v", want.BlockNumber, err)
		case <-time.After(SleepInterval + 2*time.Second):
			t.Fatalf("no log received for committed block %d after eth_subscribe returned", want.BlockNumber)
		}
	}
	receive(first)
	receive(commit(2))
	receive(commit(3))
	require.Equal(t, active+1, metrics.ActiveSubscriptions.Load())
	sub.Unsubscribe()
	require.Eventually(t, func() bool {
		return metrics.ActiveSubscriptions.Load() == active
	}, time.Second, time.Millisecond, "eth_unsubscribe must end the polling goroutine before its next timer fires")
	require.Equal(t, errors, metrics.SubscriptionErrors.Load())
}
