package receipt_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/stretchr/testify/require"
)

func setupReceiptStore(t *testing.T) (receipt.ReceiptStore, sdk.Context, storetypes.StoreKey) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey("evm")
	tkey := storetypes.NewTransientStoreKey("evm_transient")
	ctx := testutil.DefaultContext(storeKey, tkey).WithBlockHeight(0)
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 0
	store, err := receipt.NewReceiptStore(cfg, storeKey)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store, ctx, storeKey
}

func makeReceipt(txHash common.Hash, addr common.Address, topics []common.Hash, logIndex uint32) *types.Receipt {
	topicHex := make([]string, 0, len(topics))
	for _, topic := range topics {
		topicHex = append(topicHex, topic.Hex())
	}
	return &types.Receipt{
		TxHashHex:        txHash.Hex(),
		BlockNumber:      1,
		TransactionIndex: logIndex,
		Logs: []*types.Log{
			{
				Address: addr.Hex(),
				Topics:  topicHex,
				Data:    []byte{0x1},
				Index:   logIndex,
			},
		},
	}
}

func TestNewReceiptStoreConfigErrors(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("evm")
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = ""
	store, err := receipt.NewReceiptStore(cfg, storeKey)
	require.Error(t, err)
	require.Nil(t, store)

	cfg.DBDirectory = t.TempDir()
	cfg.Backend = "rocksdb"
	store, err = receipt.NewReceiptStore(cfg, storeKey)
	require.Error(t, err)
	require.Nil(t, store)

	cfg.Backend = "pebble"
	store, err = receipt.NewReceiptStore(cfg, storeKey)
	require.NoError(t, err)
	require.NotNil(t, store)
	require.NoError(t, store.Close())

	cfg.Backend = "pebble"
	store, err = receipt.NewReceiptStore(cfg, storeKey)
	require.NoError(t, err)
	require.NotNil(t, store)
	require.NoError(t, store.Close())
}

func TestSetReceiptsAndGet(t *testing.T) {
	store, ctx, _ := setupReceiptStore(t)
	txHash := common.HexToHash("0x1")
	addr := common.HexToAddress("0x1")
	topic := common.HexToHash("0x2")
	r := makeReceipt(txHash, addr, []common.Hash{topic}, 0)

	err := store.SetReceipts(ctx.WithBlockHeight(0), []receipt.ReceiptRecord{
		{TxHash: txHash, Receipt: r},
		{TxHash: txHash},
	})
	require.NoError(t, err)

	got, err := store.GetReceipt(ctx, txHash)
	require.NoError(t, err)
	require.Equal(t, r.TxHashHex, got.TxHashHex)

	got, err = store.GetReceiptFromStore(ctx, txHash)
	require.NoError(t, err)
	require.Equal(t, r.TxHashHex, got.TxHashHex)

	_, err = store.GetReceiptFromStore(ctx, common.HexToHash("0x3"))
	require.Error(t, err)

	require.GreaterOrEqual(t, store.LatestVersion(), int64(1))
	require.NoError(t, store.SetLatestVersion(10))
	require.Equal(t, int64(10), store.LatestVersion())
	require.NoError(t, store.SetEarliestVersion(1))
	require.Equal(t, int64(1), store.EarliestVersion())
}

func TestReceiptStoreLegacyFallback(t *testing.T) {
	store, ctx, storeKey := setupReceiptStore(t)
	txHash := common.HexToHash("0x4")
	r := makeReceipt(txHash, common.HexToAddress("0x2"), []common.Hash{common.HexToHash("0x5")}, 0)
	bz, err := r.Marshal()
	require.NoError(t, err)

	ctx.KVStore(storeKey).Set(types.ReceiptKey(txHash), bz)

	got, err := store.GetReceipt(ctx, txHash)
	require.NoError(t, err)
	require.Equal(t, r.TxHashHex, got.TxHashHex)

	_, err = store.GetReceiptFromStore(ctx, txHash)
	require.Error(t, err)
}

func TestSetReceiptsAsync(t *testing.T) {
	store, ctx, _ := setupReceiptStore(t)
	txHash := common.HexToHash("0x6")
	r := makeReceipt(txHash, common.HexToAddress("0x3"), []common.Hash{common.HexToHash("0x7")}, 0)
	require.NoError(t, store.SetReceipts(ctx.WithBlockHeight(2), []receipt.ReceiptRecord{{TxHash: txHash, Receipt: r}}))

	require.Eventually(t, func() bool {
		_, err := store.GetReceipt(ctx, txHash)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond)
}

func TestReceiptStorePebbleBackendBasic(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("evm")
	tkey := storetypes.NewTransientStoreKey("evm_transient")
	ctx := testutil.DefaultContext(storeKey, tkey).WithBlockHeight(0)
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 0
	cfg.Backend = "pebble"

	store, err := receipt.NewReceiptStore(cfg, storeKey)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	txHash := common.HexToHash("0x55")
	addr := common.HexToAddress("0x500")
	topic := common.HexToHash("0x501")
	r := makeReceipt(txHash, addr, []common.Hash{topic}, 0)

	require.NoError(t, store.SetReceipts(ctx, []receipt.ReceiptRecord{{TxHash: txHash, Receipt: r}}))

	got, err := store.GetReceipt(ctx, txHash)
	require.NoError(t, err)
	require.Equal(t, r.TxHashHex, got.TxHashHex)

	// The cache covers block 1 (just written via SetReceipts), so FilterLogs
	// returns the cached log without hitting the pebble backend.
	logs, err := store.FilterLogs(ctx, 1, 1, filters.FilterCriteria{
		Addresses: []common.Address{addr},
		Topics:    [][]common.Hash{{topic}},
	})
	require.ErrorIs(t, err, receipt.ErrRangeQueryNotSupported)
	require.Empty(t, logs)
}

func TestFilterLogsRangeQueryNotSupported(t *testing.T) {
	store, ctx, _ := setupReceiptStore(t)
	// Pebble backend does not support range queries, so FilterLogs returns ErrRangeQueryNotSupported.
	_, err := store.FilterLogs(ctx, 1, 10, filters.FilterCriteria{})
	require.ErrorIs(t, err, receipt.ErrRangeQueryNotSupported)
}

func TestRecoverReceiptStoreReplaysChangelog(t *testing.T) {
	for _, tc := range []struct {
		name         string
		prefix, tail int
	}{
		{"unapplied_singleton", 0, 1},
		{"singleton_after_applied_prefix", 1, 1},
		{"multiple_unapplied_entries", 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, key, records := repairM1146CrashTail(t, tc.prefix, tc.tail)
			repairM1146AssertReopens(t, cfg, key, records)
		})
	}
}
