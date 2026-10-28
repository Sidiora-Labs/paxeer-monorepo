package composite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
)

func Exporter(t *testing.T, source *CompositeCommitStore, version int64, visit func(interface{})) {
	t.Helper()
	exporter, err := source.Exporter(version)
	require.NoError(t, err)
	defer func() { require.NoError(t, exporter.Close()) }()
	for {
		item, err := exporter.Next()
		if err != nil {
			require.ErrorIs(t, err, errorutils.ErrorExportDone)
			return
		}
		visit(item)
	}
}

func Importer(t *testing.T, destination, source *CompositeCommitStore, version int64) map[string]int {
	t.Helper()
	importer, err := destination.Importer(version)
	require.NoError(t, err)
	defer func() { require.NoError(t, importer.Close()) }()
	modules := make(map[string]int)
	var currentModule string
	Exporter(t, source, version, func(item interface{}) {
		switch value := item.(type) {
		case string:
			require.NotEmpty(t, value)
			require.NotContains(t, modules, value, "duplicate module header")
			currentModule = value
			modules[value] = 0
			require.NoError(t, importer.AddModule(value))
		case *types.SnapshotNode:
			require.NotEmpty(t, currentModule, "snapshot node before module header")
			require.NotNil(t, value)
			modules[currentModule]++
			importer.AddNode(value)
		default:
			t.Fatalf("unexpected snapshot item %T", item)
		}
	})
	return modules
}

func mergeStateSyncCommitHash(info proto.CommitInfo) []byte {
	commitInfo := storetypes.CommitInfo{Version: info.Version}
	for _, store := range info.StoreInfos {
		commitInfo.StoreInfos = append(commitInfo.StoreInfos, storetypes.StoreInfo{
			Name: store.Name,
			CommitId: storetypes.CommitID{
				Version: store.CommitId.Version,
				Hash:    store.CommitId.Hash,
			},
		})
	}
	return commitInfo.Hash()
}

func TestStateSyncRoundTripsMergeStores(t *testing.T) {
	for _, mode := range []config.WriteMode{config.MemiavlOnly, config.EVMMigrated, config.FlatKVOnly} {
		t.Run(string(mode), func(t *testing.T) {
			cfg := evmMigratedConfig()
			cfg.WriteMode = mode
			newStore := func() *CompositeCommitStore {
				store, err := NewCompositeCommitStore(t.Context(), t.TempDir(), cfg)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				require.NoError(t, store.Initialize(keys.MemIAVLStoreKeys))
				_, err = store.LoadVersion(0, false)
				require.NoError(t, err)
				return store
			}

			source := newStore()
			storeNames := []string{
				keys.LayerXLedgerStoreKey,
				keys.LayerXAuthStoreKey,
				keys.LayerXPayStoreKey,
				keys.LayerXProgramsStoreKey,
				keys.SequencerStoreKey,
			}
			testKeys := [][]byte{[]byte("shared"), []byte("other"), {0x00, 0x80, 0xff}}
			var changes []*proto.NamedChangeSet
			for _, name := range storeNames {
				change := &proto.NamedChangeSet{Name: name}
				for i, key := range testKeys {
					change.Changeset.Pairs = append(change.Changeset.Pairs, &proto.KVPair{
						Key: key, Value: []byte(fmt.Sprintf("%s/value/%d", name, i)),
					})
				}
				changes = append(changes, change)
			}
			require.NoError(t, source.ApplyChangeSets(changes))
			version, err := source.Commit()
			require.NoError(t, err)
			require.Equal(t, int64(1), version)
			sourceInfo := cloneCommitInfo(source.LastCommitInfo())
			require.Equal(t, version, sourceInfo.Version)
			require.NotEmpty(t, mergeStateSyncCommitHash(sourceInfo))

			destination := newStore()
			require.Equal(t, int64(0), destination.Version())
			for _, name := range storeNames {
				for _, key := range testKeys {
					require.Nil(t, destination.GetChildStoreByName(name).Get(key), name)
				}
			}
			require.NoError(t, destination.Close())
			modules := Importer(t, destination, source, version)
			if mode == config.FlatKVOnly {
				require.GreaterOrEqual(t, modules[keys.FlatKVStoreKey], len(storeNames)*len(testKeys))
			} else {
				for _, name := range storeNames {
					require.GreaterOrEqual(t, modules[name], len(testKeys), name)
				}
			}

			_, err = destination.LoadVersion(version, false)
			require.NoError(t, err)
			require.Equal(t, version, destination.Version())
			require.Empty(t, destination.PendingStores())
			importedInfo := cloneCommitInfo(destination.LastCommitInfo())
			require.Equal(t, sourceInfo.Version, importedInfo.Version)
			require.Equal(t, mergeStateSyncCommitHash(sourceInfo), mergeStateSyncCommitHash(importedInfo))
			for _, name := range storeNames {
				if mode != config.FlatKVOnly {
					sourceStoreInfo := storeInfo(sourceInfo, name)
					importedStoreInfo := storeInfo(importedInfo, name)
					require.NotNil(t, sourceStoreInfo, name)
					require.NotNil(t, importedStoreInfo, name)
					require.NotEmpty(t, sourceStoreInfo.CommitId.Hash, name)
					require.Equal(t, sourceStoreInfo.CommitId, importedStoreInfo.CommitId, name)
				}
				for i, key := range testKeys {
					expected := []byte(fmt.Sprintf("%s/value/%d", name, i))
					require.Equal(t, expected, source.GetChildStoreByName(name).Get(key), name)
					require.Equal(t, expected, destination.GetChildStoreByName(name).Get(key), name)
				}
			}
		})
	}
}
