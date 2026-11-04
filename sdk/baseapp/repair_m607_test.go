package baseapp

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/snapshots"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_607_SealedRetentionContext(t *testing.T) {
	TestCommitSealedRetention(t)
}

func TestRepairM_607_ConcurrentCheckDeliver(t *testing.T) {
	TestConcurrentCheckDeliver(t)
}

func TestRepairM_607_MaxBlockGasLimits(t *testing.T) {
	TestMaxBlockGasLimits(t)
}

func TestRepairM_607_SnapshotRollbackRestoreReopen(t *testing.T) {
	source := setupBaseAppWithSnapshots(t, 2, 1)
	defer func() { require.NoError(t, source.Close()) }()
	sourceCMS := source.cms.(*storev2rootmulti.Store)
	require.NotNil(t, sourceCMS.GetStateStore())
	require.Equal(t, int64(2), sourceCMS.GetStateStore().GetLatestVersion())

	// Independently reproduce the committed fixture values, rather than taking the source store as an oracle.
	rng := rand.New(rand.NewSource(3920758213583))
	expected := make([][]byte, 200)
	for i := range expected {
		expected[i] = make([]byte, 10000)
		_, err := rng.Read(expected[i])
		require.NoError(t, err)
		require.Equal(t, expected[i], source.cms.GetKVStore(capKey2).Get([]byte(fmt.Sprint(i))))
	}

	directory := t.TempDir()
	scConfig := paxdbconfig.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.SnapshotInterval = 1
	scConfig.MemIAVLConfig.SnapshotMinTimeInterval = 0
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	ssConfig := paxdbconfig.DefaultStateStoreConfig()
	ssConfig.AsyncWriteBuffer = 0
	open := func() (*BaseApp, *storev2rootmulti.Store) {
		cms := storev2rootmulti.NewStore(directory, scConfig, ssConfig, nil)
		snapshotStore, err := snapshots.NewStore(dbm.NewMemDB(), t.TempDir())
		require.NoError(t, err)
		app := setupBaseApp(t, func(app *BaseApp) { app.SetCMS(cms) }, SetSnapshotStore(snapshotStore))
		return app, cms
	}
	target, cms := open()
	targetClosed := false
	defer func() {
		if !targetClosed {
			require.NoError(t, target.Close())
		}
	}()
	listed, err := source.ListSnapshots(context.Background(), &abci.RequestListSnapshots{})
	require.NoError(t, err)
	require.Len(t, listed.Snapshots, 1)
	snapshot := listed.Snapshots[0]
	require.Equal(t, uint64(2), snapshot.Height)
	offered, err := target.OfferSnapshot(context.Background(), &abci.RequestOfferSnapshot{Snapshot: snapshot})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseOfferSnapshot_ACCEPT, offered.Result)
	for i := uint32(0); i < snapshot.Chunks; i++ {
		chunk, err := source.LoadSnapshotChunk(context.Background(), &abci.RequestLoadSnapshotChunk{
			Height: snapshot.Height, Format: snapshot.Format, Chunk: i,
		})
		require.NoError(t, err)
		require.NotEmpty(t, chunk.Chunk)
		applied, err := target.ApplySnapshotChunk(context.Background(), &abci.RequestApplySnapshotChunk{Index: i, Chunk: chunk.Chunk})
		require.NoError(t, err)
		require.Equal(t, abci.ResponseApplySnapshotChunk_ACCEPT, applied.Result)
	}
	assertRecovered := func(app *BaseApp, cms *storev2rootmulti.Store) {
		require.Equal(t, source.LastCommitID(), app.LastCommitID())
		require.Equal(t, int64(2), app.LastBlockHeight())
		ss := cms.GetStateStore()
		require.NotNil(t, ss)
		require.Equal(t, int64(2), ss.GetLatestVersion())
		require.Equal(t, int64(2), ss.GetEarliestVersion())
		for i, value := range expected {
			key := []byte(fmt.Sprint(i))
			require.Equal(t, value, app.cms.GetKVStore(capKey2).Get(key))
			got, err := ss.Get(capKey2.Name(), 2, key)
			require.NoError(t, err)
			require.Equal(t, value, got)
		}
		for _, key := range []string{"rollback-1", "rollback-2"} {
			require.Nil(t, app.cms.GetKVStore(capKey2).Get([]byte(key)))
			got, err := ss.Get(capKey2.Name(), 2, []byte(key))
			require.NoError(t, err)
			require.Empty(t, got)
		}
	}
	assertRecovered(target, cms)
	require.NoError(t, target.Close())
	targetClosed = true
	reopened, reopenedCMS := open()
	defer func() { require.NoError(t, reopened.Close()) }()
	assertRecovered(reopened, reopenedCMS)
}
