package rootmulti

import (
	"bytes"
	"io"
	"testing"

	snapshottypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/snapshots/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	protoio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/require"
)

func iavlTotalKeyBytes(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(rootmultiMeterName, "iavl_total_key_bytes")
}

func iavlTotalValueBytes(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(rootmultiMeterName, "iavl_total_value_bytes")
}

func iavlTotalNumKeys(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(rootmultiMeterName, "iavl_total_num_keys")
}

func stateSyncKeysExported(reader *oteltest.Reader) int64 {
	return reader.CounterSum(rootmultiMeterName, "state_sync_keys_exported")
}

func TestSnapshotRecordsIAVLGaugesAndExportedKeys(t *testing.T) {
	reader := oteltest.Install()
	store, keys := newTestRootMulti(t, t.TempDir(), memiavlOnlyConfig())
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	data := map[string]struct{ key, value string }{
		"acc":  {key: "a", value: "one"},
		"bank": {key: "bb", value: "four"},
		"evm":  {key: "ccc", value: "seven"},
	}
	cms := store.CacheMultiStore()
	for name, pair := range data {
		cms.GetKVStore(keys[name]).Set([]byte(pair.key), []byte(pair.value))
	}
	cms.Write()
	commit := store.Commit(true)
	require.Equal(t, int64(1), commit.Version)

	for export := 0; export < 2; export++ {
		beforeKeyBytes, _ := iavlTotalKeyBytes(reader)
		beforeValueBytes, _ := iavlTotalValueBytes(reader)
		beforeNumKeys, _ := iavlTotalNumKeys(reader)
		beforeExported := stateSyncKeysExported(reader)
		var buf bytes.Buffer
		require.NoError(t, store.Snapshot(uint64(commit.Version), protoio.NewDelimitedWriter(&buf)))

		keyBytes, found := iavlTotalKeyBytes(reader)
		require.True(t, found)
		require.Equal(t, int64(6), keyBytes)
		require.Equal(t, int64(6)-beforeKeyBytes, keyBytes-beforeKeyBytes)
		valueBytes, found := iavlTotalValueBytes(reader)
		require.True(t, found)
		require.Equal(t, int64(12), valueBytes)
		require.Equal(t, int64(12)-beforeValueBytes, valueBytes-beforeValueBytes)
		numKeys, found := iavlTotalNumKeys(reader)
		require.True(t, found)
		require.Equal(t, int64(3), numKeys)
		require.Equal(t, int64(3)-beforeNumKeys, numKeys-beforeNumKeys)
		require.Equal(t, int64(3), stateSyncKeysExported(reader)-beforeExported)

		decoder := protoio.NewDelimitedReader(&buf, 1<<20)
		currentStore := ""
		seen := make(map[string]bool)
		for {
			var item snapshottypes.SnapshotItem
			err := decoder.ReadMsg(&item)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			switch item := item.Item.(type) {
			case *snapshottypes.SnapshotItem_Store:
				currentStore = item.Store.Name
			case *snapshottypes.SnapshotItem_IAVL:
				pair, ok := data[currentStore]
				require.True(t, ok)
				require.False(t, seen[currentStore])
				require.Zero(t, item.IAVL.Height)
				require.Equal(t, commit.Version, item.IAVL.Version)
				require.Equal(t, []byte(pair.key), item.IAVL.Key)
				require.Equal(t, []byte(pair.value), item.IAVL.Value)
				seen[currentStore] = true
			default:
				t.Fatalf("unexpected snapshot item %T", item)
			}
		}
		require.Len(t, seen, 3)
	}
}
