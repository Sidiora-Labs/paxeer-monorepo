package mvcc_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss/composite"
	"github.com/cockroachdb/pebble/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func repair178Changes(store string, pairs ...*proto.KVPair) []*proto.NamedChangeSet {
	return []*proto.NamedChangeSet{{Name: store, Changeset: proto.ChangeSet{Pairs: pairs}}}
}

func repair178Pair(key, value string) *proto.KVPair {
	return &proto.KVPair{Key: []byte(key), Value: []byte(value)}
}

func repair178Config() config.StateStoreConfig {
	cfg := config.DefaultStateStoreConfig()
	cfg.KeepRecent = 0
	return cfg
}

// Seed with the real legacy batch producer, leaving the database unmarked.
func repair178Legacy(t *testing.T, dir string) {
	t.Helper()
	raw, err := pebble.Open(dir, &pebble.Options{Comparer: mvcc.MVCCComparer})
	require.NoError(t, err)
	t.Cleanup(func() {
		if raw != nil {
			_ = raw.Close()
		}
	})
	batch, err := mvcc.NewBatch(raw, 1, false)
	require.NoError(t, err)
	for _, key := range []string{"b", "c", "d", "z"} {
		require.NoError(t, batch.Set("bank", []byte(key), []byte(key+"1")))
	}
	require.NoError(t, batch.Write())
	_, closer, err := raw.Get([]byte("s/_mvcc_descending"))
	require.ErrorIs(t, err, pebble.ErrNotFound)
	if closer != nil {
		require.NoError(t, closer.Close())
	}
	require.NoError(t, raw.Close())
	// Raw Pebble Close is not idempotent; the cleanup above is only for failures.
	raw = nil
}

func TestRepair_178_DurableHeadRegression(t *testing.T) {
	for _, mode := range []string{"descending", "default-comparer", "legacy", "composite"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "db")
			cfg := repair178Config()
			cfg.DBDirectory = dir
			cfg.UseDefaultComparer = mode == "default-comparer"
			if mode == "legacy" {
				repair178Legacy(t, dir)
			}
			open := func() types.StateStore {
				if mode == "composite" {
					store, err := composite.NewCompositeStateStore(cfg, filepath.Dir(dir))
					require.NoError(t, err)
					return store
				}
				store, err := mvcc.OpenDB(dir, cfg)
				require.NoError(t, err)
				return store
			}
			store := open()
			t.Cleanup(func() { _ = store.Close() })
			require.NoError(t, store.ApplyChangesetSync(10, repair178Changes("bank", repair178Pair("balance", "ten"), repair178Pair("high", "ten"))))
			require.NoError(t, store.ApplyChangesetSync(5, repair178Changes("bank", repair178Pair("balance", "five"), repair178Pair("low", "five"))))
			check := func() {
				require.Equal(t, int64(10), store.GetLatestVersion())
				for _, read := range []struct {
					version   int64
					key, want string
				}{{5, "balance", "five"}, {10, "balance", "ten"}, {10, "high", "ten"}, {10, "low", "five"}} {
					got, err := store.Get("bank", read.version, []byte(read.key))
					require.NoError(t, err)
					require.Equal(t, []byte(read.want), got)
				}
				got, err := store.Get("bank", 4, []byte("balance"))
				require.NoError(t, err)
				require.Nil(t, got)
			}
			check()
			require.Error(t, store.ApplyChangesetSync(-1, repair178Changes("bank", repair178Pair("invalid", "bad"))))
			require.Error(t, store.SetLatestVersion(-1))
			check()
			require.NoError(t, store.Close())
			store = open()
			check()
			// Explicit rollback remains authoritative even with higher data on disk.
			require.NoError(t, store.SetLatestVersion(4))
			require.Equal(t, int64(4), store.GetLatestVersion())
			require.NoError(t, store.Close())
			store = open()
			require.Equal(t, int64(4), store.GetLatestVersion())
			require.NoError(t, store.ApplyChangesetSync(5, repair178Changes("bank", repair178Pair("after-rollback", "five"))))
			require.Equal(t, int64(5), store.GetLatestVersion())
			require.NoError(t, store.Close())
			store = open()
			require.Equal(t, int64(5), store.GetLatestVersion())
		})
	}
}

func TestRepair_178_ConcurrentHeadRecovery(t *testing.T) {
	dir := t.TempDir()
	store, err := mvcc.OpenDB(dir, repair178Config())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	const count = 32
	var wg sync.WaitGroup
	start := make(chan struct{})
	errors := make(chan error, count)
	for version := int64(1); version <= count; version++ {
		wg.Add(1)
		go func(version int64) {
			defer wg.Done()
			<-start
			errors <- store.ApplyChangesetSync(version, repair178Changes("bank", repair178Pair(fmt.Sprint(version), fmt.Sprint(version))))
		}(version)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, int64(count), store.GetLatestVersion())
	require.NoError(t, store.Close())
	store, err = mvcc.OpenDB(dir, repair178Config())
	require.NoError(t, err)
	require.Equal(t, int64(count), store.GetLatestVersion())
	for version := int64(1); version <= count; version++ {
		got, err := store.Get("bank", count, []byte(fmt.Sprint(version)))
		require.NoError(t, err)
		require.Equal(t, []byte(fmt.Sprint(version)), got)
	}
}

func TestRepair_178_ReceiptHeadRecovery(t *testing.T) {
	cfg := config.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 0
	key := sdk.NewKVStoreKey("evm")
	store, err := receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	ctx := sdk.Context{}
	records := []receipt.ReceiptRecord{
		{TxHash: common.HexToHash("0x10"), Receipt: &evmtypes.Receipt{TxHashHex: common.HexToHash("0x10").Hex(), BlockNumber: 10}},
		{TxHash: common.HexToHash("0x05"), Receipt: &evmtypes.Receipt{TxHashHex: common.HexToHash("0x05").Hex(), BlockNumber: 5}},
	}
	for _, record := range records {
		require.NoError(t, store.SetReceipts(ctx.WithBlockHeight(int64(record.Receipt.BlockNumber)), []receipt.ReceiptRecord{record}))
	}
	// Seeing the second receipt proves the FIFO background writer drained both commits.
	require.Eventually(t, func() bool {
		_, err := store.GetReceiptFromStore(ctx, records[1].TxHash)
		return err == nil
	}, 5*time.Second, time.Millisecond)
	check := func() {
		require.Equal(t, int64(10), store.LatestVersion())
		for _, record := range records {
			got, err := store.GetReceiptFromStore(ctx, record.TxHash)
			require.NoError(t, err)
			require.Equal(t, record.Receipt, got)
		}
	}
	check()
	require.NoError(t, store.Close())
	for reopen := 0; reopen < 2; reopen++ {
		store, err = receipt.NewReceiptStore(cfg, key)
		require.NoError(t, err)
		check()
		require.NoError(t, store.Close())
	}
}

func TestRepair_178_LegacyReverseVisibleKey(t *testing.T) {
	dir := t.TempDir()
	repair178Legacy(t, dir)
	store, err := mvcc.OpenDB(dir, repair178Config())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.ApplyChangesetSync(2, repair178Changes("bank", &proto.KVPair{Key: []byte("b")})))
	require.NoError(t, store.ApplyChangesetSync(3, repair178Changes("bank", repair178Pair("a", "a3"), repair178Pair("c", "c3"), &proto.KVPair{Key: []byte("d")}, repair178Pair("z", "z3"))))
	require.NoError(t, store.ApplyChangesetSync(4, repair178Changes("bank", repair178Pair("e", "e4"), repair178Pair("d", "d4"))))
	visible := []map[string]string{
		{"b": "b1", "c": "c1", "d": "d1", "z": "z1"},
		{"c": "c1", "d": "d1", "z": "z1"},
		{"a": "a3", "c": "c3", "z": "z3"},
		{"a": "a3", "c": "c3", "d": "d4", "e": "e4", "z": "z3"},
	}
	keys := []string{"a", "b", "c", "d", "e", "z"}
	bounds := []struct{ start, end []byte }{
		{}, {[]byte("b"), []byte("z")}, {[]byte("z"), nil},
		{nil, []byte("d")}, {[]byte("a"), []byte("b")},
		{[]byte("d"), []byte("z")}, {[]byte("z"), []byte("zz")},
	}
	for reopen := 0; reopen < 2; reopen++ {
		for index, expected := range visible {
			version := int64(index + 1)
			for _, key := range keys {
				got, err := store.Get("bank", version, []byte(key))
				require.NoError(t, err)
				if value, ok := expected[key]; ok {
					require.Equal(t, []byte(value), got)
				} else {
					require.Nil(t, got)
				}
			}
			for _, bound := range bounds {
				for _, reverse := range []bool{false, true} {
					want := make([]string, 0)
					for _, key := range keys {
						if value, ok := expected[key]; ok && (bound.start == nil || bytes.Compare([]byte(key), bound.start) >= 0) && (bound.end == nil || bytes.Compare([]byte(key), bound.end) < 0) {
							want = append(want, key+"="+value)
						}
					}
					if reverse {
						for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
							want[i], want[j] = want[j], want[i]
						}
					}
					newIterator := store.Iterator
					if reverse {
						newIterator = store.ReverseIterator
					}
					iter, err := newIterator("bank", version, bound.start, bound.end)
					require.NoError(t, err)
					got := make([]string, 0)
					for ; iter.Valid(); iter.Next() {
						require.Less(t, len(got), len(keys), "iterator must terminate without duplicate keys")
						got = append(got, string(iter.Key())+"="+string(iter.Value()))
					}
					require.NoError(t, iter.Error())
					require.NoError(t, iter.Close())
					require.Equal(t, want, got, "reopen=%d height=%d reverse=%t bounds=[%s,%s)", reopen, version, reverse, bound.start, bound.end)
				}
			}
		}
		require.NoError(t, store.Close())
		if reopen == 0 {
			store, err = mvcc.OpenDB(dir, repair178Config())
			require.NoError(t, err)
		}
	}
	raw, err := pebble.Open(dir, &pebble.Options{Comparer: mvcc.MVCCComparer})
	require.NoError(t, err)
	defer func() { require.NoError(t, raw.Close()) }()
	_, closer, err := raw.Get([]byte("s/_mvcc_descending"))
	require.ErrorIs(t, err, pebble.ErrNotFound, "legacy database must remain unmarked after reopen")
	if closer != nil {
		require.NoError(t, closer.Close())
	}
}
