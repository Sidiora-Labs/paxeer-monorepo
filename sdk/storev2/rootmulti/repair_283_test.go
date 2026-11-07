package rootmulti_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	snapshottypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/snapshots/types"
	proofmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss"
	protoio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair283Configs(mode config.WriteMode, split bool) (config.StateCommitConfig, config.StateStoreConfig) {
	sc := config.DefaultStateCommitConfig()
	sc.WriteMode = mode
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotInterval = 1
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	sc.HistoricalProofRateLimit = 0
	state := config.DefaultStateStoreConfig()
	state.AsyncWriteBuffer = 0
	state.ImportNumWorkers = 4
	state.EVMSplit = split
	return sc, state
}

func repair283Open(t *testing.T, home string, sc config.StateCommitConfig, state config.StateStoreConfig) *rootmulti.Store {
	t.Helper()
	store := rootmulti.NewStore(home, sc, state, nil)
	for _, name := range []string{"bank", "evm"} {
		store.MountStoreWithDB(types.NewKVStoreKey(name), types.StoreTypeIAVL, nil)
	}
	require.NoError(t, store.LoadLatestVersion())
	return store
}

func repair283App(t *testing.T, store *rootmulti.Store) *baseapp.BaseApp {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	app := baseapp.NewBaseApp("repair-283", db, nil, nil, &testutil.TestAppOpts{})
	app.SetCMS(store)
	return app
}

func repair283Query(t *testing.T, app *baseapp.BaseApp, height int64, prove bool, key []byte) *abci.ResponseQuery {
	t.Helper()
	res, err := app.Query(context.Background(), &abci.RequestQuery{
		Path: "/store/bank/key", Data: key, Height: height, Prove: prove,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}

func repair283WaitForCommit(t *testing.T, store *rootmulti.Store, height int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		return store.GetStateStore().GetLatestVersion() == height
	}, 10*time.Second, time.Millisecond, "ordinary asynchronous commit did not reach SS")
}

func repair283AssertValue(t *testing.T, res *abci.ResponseQuery, height int64, key, value, hash []byte, prove bool) {
	t.Helper()
	require.Zero(t, res.Code, res.Log)
	require.Empty(t, res.Codespace)
	require.Equal(t, height, res.Height)
	require.Equal(t, key, res.Key)
	require.Equal(t, value, res.Value)
	if prove {
		require.NotNil(t, res.ProofOps)
		require.Len(t, res.ProofOps.Ops, 2)
		runtime := proofmulti.DefaultProofRuntime()
		path := fmt.Sprintf("/bank/x:%X", key)
		require.NoError(t, runtime.VerifyValue(res.ProofOps, hash, path, value))
		require.Error(t, runtime.VerifyValue(res.ProofOps, hash, path, []byte("wrong value")))
	} else {
		require.Nil(t, res.ProofOps)
	}
}

func TestRepair_283_FutureQueryHeight(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("SS=%t", enabled), func(t *testing.T) {
			sc, state := repair283Configs(config.MemiavlOnly, false)
			state.Enable = enabled
			store := repair283Open(t, t.TempDir(), sc, state)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			key := []byte("balance")
			commits := make(map[int64]types.CommitID)
			for height := int64(1); height <= 3; height++ {
				store.GetStoreByName("bank").(types.KVStore).Set(key, []byte(fmt.Sprint(height)))
				commits[height] = store.Commit(true)
				require.Equal(t, height, commits[height].Version)
			}
			if enabled {
				repair283WaitForCommit(t, store, 3)
			}
			app := repair283App(t, store)
			for _, prove := range []bool{false, true} {
				for _, height := range []int64{0, 2, 3, 4, 1 << 40} {
					t.Run(fmt.Sprintf("height=%d/prove=%t", height, prove), func(t *testing.T) {
						direct := store.Query(context.Background(), abci.RequestQuery{
							Path: "/bank/key", Data: key, Height: height, Prove: prove,
						})
						response := repair283Query(t, app, height, prove, key)
						if height > 3 {
							for _, res := range []*abci.ResponseQuery{&direct, response} {
								require.Equal(t, sdkerrors.ErrInvalidHeight.ABCICode(), res.Code)
								require.Equal(t, sdkerrors.ErrInvalidHeight.Codespace(), res.Codespace)
								require.Contains(t, res.Log, fmt.Sprintf("cannot query height %d above committed height 3", height))
								require.Zero(t, res.Height)
								require.Nil(t, res.Key)
								require.Nil(t, res.Value)
								require.Nil(t, res.ProofOps)
							}
							return
						}
						resolved := height
						if resolved == 0 {
							resolved = 3
						}
						for _, res := range []*abci.ResponseQuery{&direct, response} {
							repair283AssertValue(t, res, resolved, key, []byte(fmt.Sprint(resolved)), commits[resolved].Hash, prove)
						}
					})
				}
			}
			// CMS also resolves negative heights to the tip. BaseApp must preserve
			// that resolved height rather than relabel the response with -1.
			repair283AssertValue(t, repair283Query(t, app, -1, false, key), 3, key, []byte("3"), commits[3].Hash, false)
		})
	}
}

func repair283Snapshot(t *testing.T, store *rootmulti.Store, height uint64) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, store.Snapshot(height, protoio.NewDelimitedWriter(&buf)))
	require.NotEmpty(t, buf.Bytes())
	return buf.Bytes()
}

func repair283Restore(t *testing.T, store *rootmulti.Store, height uint64, snapshot []byte) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := store.Restore(height, 1, protoio.NewDelimitedReader(bytes.NewReader(snapshot), 1<<28))
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("restore did not drain and join its importers")
		return nil
	}
}

func TestRepair_283_RestoreImmediateHistoryAndRestart(t *testing.T) {
	for _, split := range []bool{false, true} {
		t.Run(fmt.Sprintf("split=%t", split), func(t *testing.T) {
			sc, state := repair283Configs(config.MemiavlOnly, split)
			source := repair283Open(t, t.TempDir(), sc, state)
			const count = 257
			evmKey := append([]byte{0x0a}, bytes.Repeat([]byte{0x12}, 20)...)
			commits := make(map[int64]types.CommitID)
			for height := int64(1); height <= 3; height++ {
				for i := 0; i < count; i++ {
					source.GetStoreByName("bank").(types.KVStore).Set([]byte(fmt.Sprintf("balance-%04d", i)), []byte(fmt.Sprintf("%d/%d", height, i)))
				}
				source.GetStoreByName("evm").(types.KVStore).Set(evmKey, bytes.Repeat([]byte{byte(height)}, 8))
				commits[height] = source.Commit(true)
			}
			snapshot := repair283Snapshot(t, source, 2)
			require.NoError(t, source.Close())
			home := t.TempDir()
			destination := repair283Open(t, home, sc, state)
			require.NoError(t, repair283Restore(t, destination, 2, snapshot))
			require.Equal(t, commits[2], destination.LastCommitID())
			check := func(store *rootmulti.Store) {
				t.Helper()
				require.Equal(t, int64(2), store.GetStateStore().GetEarliestVersion())
				require.Equal(t, store.LastCommitID().Version, store.GetStateStore().GetLatestVersion())
				app := repair283App(t, store)
				for i := 0; i < count; i++ {
					key := []byte(fmt.Sprintf("balance-%04d", i))
					repair283AssertValue(t, repair283Query(t, app, 2, false, key), 2, key, []byte(fmt.Sprintf("2/%d", i)), commits[2].Hash, false)
				}
				key := []byte("balance-0000")
				repair283AssertValue(t, repair283Query(t, app, 2, true, key), 2, key, []byte("2/0"), commits[2].Hash, true)
				evm := store.Query(context.Background(), abci.RequestQuery{Path: "/evm/key", Data: evmKey, Height: 2})
				require.Zero(t, evm.Code, evm.Log)
				require.Equal(t, int64(2), evm.Height)
				require.Equal(t, bytes.Repeat([]byte{2}, 8), evm.Value)
				require.Nil(t, evm.ProofOps)
			}
			// No polling or post-restore block is permitted before the first read.
			check(destination)
			destination.GetStoreByName("bank").(types.KVStore).Set([]byte("balance-0000"), []byte("3/0"))
			require.Equal(t, int64(3), destination.Commit(true).Version)
			repair283WaitForCommit(t, destination, 3)
			check(destination)
			require.NoError(t, destination.Close())
			reopened := repair283Open(t, home, sc, state)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			require.Equal(t, int64(3), reopened.LastCommitID().Version)
			check(reopened)
		})
	}
}

func repair283RewriteSnapshot(t *testing.T, snapshot []byte, rewrite func(*snapshottypes.SnapshotItem)) []byte {
	t.Helper()
	reader := protoio.NewDelimitedReader(bytes.NewReader(snapshot), 1<<28)
	var buf bytes.Buffer
	writer := protoio.NewDelimitedWriter(&buf)
	for {
		var item snapshottypes.SnapshotItem
		err := reader.ReadMsg(&item)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		rewrite(&item)
		require.NoError(t, writer.WriteMsg(&item))
	}
	return buf.Bytes()
}

func TestRepair_283_RestoreImportErrorAndRetry(t *testing.T) {
	sc, state := repair283Configs(config.FlatKVOnly, false)
	source := repair283Open(t, t.TempDir(), sc, state)
	key, value := []byte("balance"), []byte("snapshot-value")
	source.GetStoreByName("bank").(types.KVStore).Set(key, value)
	source.Commit(true)
	commit := source.Commit(true)
	snapshot := repair283Snapshot(t, source, 2)
	require.NoError(t, source.Close())
	corrupted := 0
	broken := repair283RewriteSnapshot(t, snapshot, func(item *snapshottypes.SnapshotItem) {
		if node := item.GetIAVL(); node != nil {
			// Damage only the SS serialization version; physical keys remain
			// valid for the real SC importer, isolating the SS conversion error.
			node.Value = []byte{0xff}
			corrupted++
		}
	})
	require.Positive(t, corrupted)
	home := t.TempDir()
	destination := repair283Open(t, home, sc, state)
	err := repair283Restore(t, destination, 2, broken)
	require.ErrorContains(t, err, "SS import failure")
	require.ErrorContains(t, err, "unsupported serialization version")
	require.Zero(t, destination.GetStateStore().GetEarliestVersion())
	require.Zero(t, destination.GetStateStore().GetLatestVersion())
	require.Zero(t, destination.LastCommitID().Version)
	// The failed attempt has joined its workers; retry the real snapshot,
	// then stop and reopen both stores before reading its history.
	require.NoError(t, repair283Restore(t, destination, 2, snapshot))
	require.Equal(t, commit, destination.LastCommitID())
	repair283AssertValue(t, repair283Query(t, repair283App(t, destination), 2, false, key), 2, key, value, commit.Hash, false)
	require.NoError(t, destination.Close())
	reopened := repair283Open(t, home, sc, state)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	require.Equal(t, commit, reopened.LastCommitID())
	require.Equal(t, int64(2), reopened.GetStateStore().GetEarliestVersion())
	require.Equal(t, int64(2), reopened.GetStateStore().GetLatestVersion())
	repair283AssertValue(t, repair283Query(t, repair283App(t, reopened), 2, false, key), 2, key, value, commit.Hash, false)
}

func TestRepair_283_RestoreReadAndSCFailures(t *testing.T) {
	sc, state := repair283Configs(config.MemiavlOnly, false)
	source := repair283Open(t, t.TempDir(), sc, state)
	source.GetStoreByName("bank").(types.KVStore).Set([]byte("balance"), []byte("original"))
	source.Commit(true)
	source.Commit(true)
	snapshot := repair283Snapshot(t, source, 2)
	require.NoError(t, source.Close())
	for _, kind := range []string{"read", "SC"} {
		t.Run(kind, func(t *testing.T) {
			broken := append([]byte(nil), snapshot...)
			want := "invalid protobuf message"
			if kind == "read" {
				// An unterminated protobuf length after the real snapshot.
				broken = append(broken, 0x80)
			} else {
				want = "node version under/overflows"
				corrupted := 0
				broken = repair283RewriteSnapshot(t, snapshot, func(item *snapshottypes.SnapshotItem) {
					if node := item.GetIAVL(); node != nil {
						node.Version = -1
						corrupted++
					}
				})
				require.Positive(t, corrupted)
			}
			home := t.TempDir()
			destination := repair283Open(t, home, sc, state)
			err := repair283Restore(t, destination, 2, broken)
			require.ErrorContains(t, err, want)
			require.Zero(t, destination.LastCommitID().Version)
			require.Zero(t, destination.GetStateStore().GetEarliestVersion())
			require.NoError(t, destination.Close())
			// Reopening the actual history backend also proves no importer was
			// left running against a closed database after the failure returned.
			history, err := ss.NewStateStore(home, state)
			require.NoError(t, err)
			require.Zero(t, history.GetEarliestVersion())
			require.NoError(t, history.Close())
		})
	}
}
