package migration

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/stretchr/testify/require"
)

func TestBankMigrationRoutersMoveMergeStores(t *testing.T) {
	mergeStores := []string{
		keys.LayerXLedgerStoreKey,
		keys.LayerXAuthStoreKey,
		keys.LayerXPayStoreKey,
		keys.LayerXProgramsStoreKey,
		keys.SequencerStoreKey,
		keys.LayerXAgentStoreKey,
		keys.LayerXHostStoreKey,
	}
	memIAVL := NewTestMemIAVLCommitStore(t, t.TempDir(), keys.MemIAVLStoreKeys)
	flatKV := NewTestFlatKVCommitStore(t, t.TempDir())
	commitBoth := func() {
		t.Helper()
		_, err := memIAVL.Commit()
		require.NoError(t, err)
		_, err = flatKV.Commit()
		require.NoError(t, err)
	}

	var changesets []*proto.NamedChangeSet
	for _, store := range mergeStores {
		changesets = append(changesets, &proto.NamedChangeSet{
			Name: store,
			Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte{0x00, 0x01}, Value: []byte(store + "/first")},
				{Key: []byte{0x00, 0x01, 0x00}, Value: []byte(store + "/second")},
				{Key: []byte{0xff}, Value: []byte(store + "/last")},
			}},
		})
	}
	bankKey, bankValue := []byte("balance"), []byte("bank-value")
	require.NoError(t, memIAVL.ApplyChangeSets(append(changesets, &proto.NamedChangeSet{
		Name: keys.BankStoreKey,
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: bankKey, Value: bankValue},
		}},
	})))
	commitBoth()
	SeedMigrationVersionInFlatKV(t, flatKV, Version1_MigrateEVM)
	for _, cs := range changesets {
		for _, pair := range cs.Changeset.Pairs {
			require.Equal(t, pair.Value, memIAVL.GetChildStoreByName(cs.Name).Get(pair.Key))
			_, found := flatKV.Get(cs.Name, pair.Key)
			require.False(t, found, "%s/%x must start in memiavl", cs.Name, pair.Key)
		}
	}

	assertMergeReads := func(router Router) {
		t.Helper()
		for _, cs := range changesets {
			for _, pair := range cs.Changeset.Pairs {
				value, found, err := router.Read(cs.Name, pair.Key)
				require.NoError(t, err, "%s/%x", cs.Name, pair.Key)
				require.True(t, found, "%s/%x", cs.Name, pair.Key)
				require.Equal(t, pair.Value, value, "%s/%x", cs.Name, pair.Key)
			}
		}
	}
	assertMergePlacement := func() {
		t.Helper()
		for _, cs := range changesets {
			for _, pair := range cs.Changeset.Pairs {
				require.Nil(t, memIAVL.GetChildStoreByName(cs.Name).Get(pair.Key),
					"%s/%x must leave memiavl", cs.Name, pair.Key)
				value, found := flatKV.Get(cs.Name, pair.Key)
				require.True(t, found, "%s/%x must be in flatkv", cs.Name, pair.Key)
				require.Equal(t, pair.Value, value, "%s/%x", cs.Name, pair.Key)
			}
		}
	}
	drain := func(router Router, target uint64, maxBatches int) {
		t.Helper()
		var version uint64
		for batch := 0; batch < maxBatches; batch++ {
			require.NoError(t, router.ApplyChangeSets(nil, true))
			commitBoth()
			assertMergeReads(router)
			var found bool
			version, found = ReadMigrationVersionFromFlatKV(t, flatKV)
			require.True(t, found)
			if version == target {
				break
			}
		}
		require.Equal(t, target, version, "migration must finish within the seeded key count")
		_, found := ReadMigrationBoundaryFromFlatKV(t, flatKV)
		require.False(t, found, "completed migration must clear its boundary")
	}

	const batchSize = 2
	migrating, err := buildMigrateAllButBankRouter(t.Context(), memIAVL, flatKV, batchSize)
	require.NoError(t, err)
	assertMergeReads(migrating)
	drain(migrating, Version2_MigrateAllButBank, len(mergeStores)*3/batchSize+1)
	assertMergePlacement()
	require.Equal(t, bankValue, memIAVL.GetChildStoreByName(keys.BankStoreKey).Get(bankKey))
	_, found := flatKV.Get(keys.BankStoreKey, bankKey)
	require.False(t, found, "all-but-bank migration must leave bank in memiavl")

	allButBank, err := buildAllMigratedButBankRouter(memIAVL, flatKV)
	require.NoError(t, err)
	assertMergeReads(allButBank)
	value, found, err := allButBank.Read(keys.BankStoreKey, bankKey)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, bankValue, value)

	migratingBank, err := buildMigrateBankRouter(t.Context(), memIAVL, flatKV, batchSize)
	require.NoError(t, err)
	assertMergeReads(migratingBank)
	assertMergePlacement()
	drain(migratingBank, Version3_FlatKVOnly, 2)
	assertMergePlacement()
	require.Nil(t, memIAVL.GetChildStoreByName(keys.BankStoreKey).Get(bankKey))
	value, found = flatKV.Get(keys.BankStoreKey, bankKey)
	require.True(t, found)
	require.Equal(t, bankValue, value)

	flatOnly, err := buildFlatKVOnlyRouter(flatKV)
	require.NoError(t, err)
	assertMergeReads(flatOnly)
}
