package flatkv_test

import (
	"errors"
	"testing"

	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/composite"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/ktype"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
	"github.com/stretchr/testify/require"
)

func TestRepair_124_LatestExportComposite(t *testing.T) {
	cfg := config.DefaultTestConfig(t)
	cfg.SnapshotInterval = 0
	src, err := flatkv.NewCommitStore(t.Context(), cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, src.Close()) }()
	_, err = src.LoadVersion(0, false)
	require.NoError(t, err)

	addr, slot := ktype.Address{1}, ktype.Slot{2}
	storage := make([]byte, 32)
	storage[31] = 3
	pairs := []*proto.KVPair{
		{Key: keys.BuildEVMKey(keys.EVMKeyNonce, addr[:]), Value: []byte{0, 0, 0, 0, 0, 0, 0, 4}},
		{Key: keys.BuildEVMKey(keys.EVMKeyCode, addr[:]), Value: []byte{0x60, 0x80}},
		{Key: keys.BuildEVMKey(keys.EVMKeyStorage, ktype.StorageKey(addr, slot)), Value: storage},
	}
	require.NoError(t, src.ApplyChangeSets([]*proto.NamedChangeSet{
		{Name: keys.EVMStoreKey, Changeset: proto.ChangeSet{Pairs: pairs}},
		{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("supply"), Value: []byte("100")}}}},
	}))
	_, err = src.Commit()
	require.NoError(t, err)
	height, err := src.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(2), height)
	root := src.CommittedRootHash()

	rawExporter, err := src.Exporter(0)
	require.NoError(t, err)
	exporter, err := composite.NewExporter(nil, rawExporter)
	require.NoError(t, err)
	defer func() { require.NoError(t, exporter.Close()) }()

	dstCfg := config.DefaultTestConfig(t)
	dst, err := flatkv.NewCommitStore(t.Context(), dstCfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, dst.Close()) }()
	rawImporter, err := dst.Importer(height)
	require.NoError(t, err)
	importer := composite.NewImporter(nil, rawImporter)
	count, headers := 0, 0
	for {
		item, err := exporter.Next()
		if errors.Is(err, errorutils.ErrorExportDone) {
			break
		}
		require.NoError(t, err)
		switch item := item.(type) {
		case string:
			require.Equal(t, keys.FlatKVStoreKey, item)
			headers++
			require.NoError(t, importer.AddModule(item))
		case *types.SnapshotNode:
			require.Equal(t, height, item.Version)
			require.Equal(t, int8(0), item.Height)
			importer.AddNode(item)
			count++
		default:
			t.Fatalf("unexpected exported item %T", item)
		}
	}
	require.Equal(t, 1, headers)
	require.Equal(t, 4, count)
	require.NoError(t, exporter.Close())
	require.NoError(t, importer.Close())
	require.NoError(t, dst.Close())

	reopened, err := flatkv.NewCommitStore(t.Context(), dstCfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, reopened.Close()) }()
	_, err = reopened.LoadVersion(0, false)
	require.NoError(t, err)
	require.Equal(t, height, reopened.Version())
	for _, pair := range pairs {
		value, found := reopened.Get(keys.EVMStoreKey, pair.Key)
		require.True(t, found)
		require.Equal(t, pair.Value, value)
	}
	value, found := reopened.Get("bank", []byte("supply"))
	require.True(t, found)
	require.Equal(t, []byte("100"), value)
	require.Equal(t, root, reopened.CommittedRootHash())
	require.Equal(t, root, reopened.RootHash())
	require.NoError(t, flatkv.VerifyLtHash(reopened))
}
