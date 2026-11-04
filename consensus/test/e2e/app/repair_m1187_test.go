package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1187_SnapshotPruneReopen(t *testing.T) {
	for _, keep := range []int{0, 2, 4, 5} {
		t.Run(fmt.Sprintf("keep_%d", keep), func(t *testing.T) {
			dir := t.TempDir()
			store, err := app.NewSnapshotStore(dir)
			require.NoError(t, err)
			state, err := app.NewState(t.TempDir(), 1)
			require.NoError(t, err)
			state.Set("large", strings.Repeat("x", 1_000_001))
			var snapshots []abci.Snapshot
			var payloads [][]byte
			for height := 1; height <= 4; height++ {
				state.Set("height", fmt.Sprint(height))
				state.Finalize()
				snapshot, err := store.Create(state)
				require.NoError(t, err)
				require.Greater(t, snapshot.Chunks, uint32(1))
				snapshots = append(snapshots, snapshot)
				payload, err := state.Export()
				require.NoError(t, err)
				payloads = append(payloads, payload)
			}
			require.Error(t, store.Prune(-1))
			require.NoError(t, store.Prune(keep))
			// Stores hold no persistent handles; discard the instance before reopening.
			store = nil
			store, err = app.NewSnapshotStore(dir)
			require.NoError(t, err)
			first := max(0, len(snapshots)-keep)
			listed, err := store.List()
			require.NoError(t, err)
			require.Len(t, listed, len(snapshots)-first)
			for i, snapshot := range snapshots {
				if i < first {
					require.NoFileExists(t, filepath.Join(dir, fmt.Sprintf("%d.json", snapshot.Height)))
					chunk, err := store.LoadChunk(snapshot.Height, snapshot.Format, 0)
					require.NoError(t, err)
					require.Nil(t, chunk)
					continue
				}
				require.Equal(t, snapshot, *listed[i-first])
				var payload []byte
				for chunk := uint32(0); chunk < snapshot.Chunks; chunk++ {
					data, err := store.LoadChunk(snapshot.Height, snapshot.Format, chunk)
					require.NoError(t, err)
					require.NotEmpty(t, data)
					payload = append(payload, data...)
				}
				require.Equal(t, payloads[i], payload)
				restoredDir := t.TempDir()
				restored, err := app.NewState(restoredDir, 1)
				require.NoError(t, err)
				require.NoError(t, restored.Import(snapshot.Height, payload))
				restored, err = app.NewState(restoredDir, 1)
				require.NoError(t, err)
				require.Equal(t, snapshot.Hash, restored.Hash)
				require.Equal(t, fmt.Sprint(snapshot.Height), restored.Get("height"))
			}
			require.NoError(t, store.Prune(keep))
		})
	}
}

func TestRepairM_1187_SnapshotInterruptedPrune(t *testing.T) {
	for _, restart := range []bool{true, false} {
		t.Run(fmt.Sprintf("restart_%t", restart), func(t *testing.T) {
			dir := t.TempDir()
			store, err := app.NewSnapshotStore(dir)
			require.NoError(t, err)
			state, err := app.NewState(t.TempDir(), 1)
			require.NoError(t, err)
			for height := 1; height <= 3; height++ {
				state.Set("height", fmt.Sprint(height))
				state.Finalize()
				_, err := store.Create(state)
				require.NoError(t, err)
			}
			before, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
			require.NoError(t, err)
			// A real filesystem obstruction interrupts the prune after file removal.
			obstruction := filepath.Join(dir, "metadata.json.new")
			require.NoError(t, os.Mkdir(obstruction, 0755))
			require.Error(t, store.Prune(1))
			require.NoFileExists(t, filepath.Join(dir, "1.json"))
			require.NoFileExists(t, filepath.Join(dir, "2.json"))
			require.FileExists(t, filepath.Join(dir, "3.json"))
			after, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
			require.NoError(t, err)
			require.Equal(t, before, after)
			if restart {
				store = nil
				_, err = app.NewSnapshotStore(dir)
				require.Error(t, err)
			}
			require.NoError(t, os.Remove(obstruction))
			if restart {
				store, err = app.NewSnapshotStore(dir)
				require.NoError(t, err)
			} else {
				require.NoError(t, store.Prune(1))
			}
			listed, err := store.List()
			require.NoError(t, err)
			require.Len(t, listed, 1)
			require.Equal(t, uint64(3), listed[0].Height)
			data, err := store.LoadChunk(3, 1, 0)
			require.NoError(t, err)
			want, err := state.Export()
			require.NoError(t, err)
			require.Equal(t, want, data)
			for height := uint64(1); height < 3; height++ {
				data, err := store.LoadChunk(height, 1, 0)
				require.NoError(t, err)
				require.Nil(t, data)
			}
			state.Set("height", "4")
			state.Finalize()
			_, err = store.Create(state)
			require.NoError(t, err)
			require.NoError(t, store.Prune(1))
			store, err = app.NewSnapshotStore(dir)
			require.NoError(t, err)
			listed, err = store.List()
			require.NoError(t, err)
			require.Len(t, listed, 1)
			require.Equal(t, uint64(4), listed[0].Height)
		})
	}
}

func TestRepairM_1187_AppStateRollback(t *testing.T) {
	dir := t.TempDir()
	state, err := app.NewState(dir, 1)
	require.NoError(t, err)
	state.Set("stable", "older")
	oldHash := bytes.Clone(state.Finalize())
	_, err = state.Commit()
	require.NoError(t, err)
	oldValues, err := state.Export()
	require.NoError(t, err)
	state.Set("stable", "newer")
	state.Set("added", "must disappear")
	state.Finalize()
	_, err = state.Commit()
	require.NoError(t, err)
	require.True(t, state.CanRollback())
	require.NoError(t, state.Rollback())
	assertRollback := func(state *app.State) {
		t.Helper()
		require.Equal(t, uint64(1), state.Height)
		require.Equal(t, oldHash, state.Hash)
		require.Equal(t, "older", state.Get("stable"))
		require.Empty(t, state.Get("added"))
		values, err := state.Export()
		require.NoError(t, err)
		require.Equal(t, oldValues, values)
		// Recompute through the real import path instead of trusting the saved hash.
		check, err := app.NewState(t.TempDir(), 1)
		require.NoError(t, err)
		require.NoError(t, check.Import(state.Height, values))
		require.Equal(t, check.Hash, state.Hash)
	}
	assertRollback(state)
	state = nil
	state, err = app.NewState(dir, 1)
	require.NoError(t, err)
	assertRollback(state)
	require.NoError(t, state.Rollback())
	assertRollback(state)
	state.Set("next", "after rollback")
	state.Finalize()
	_, err = state.Commit()
	require.NoError(t, err)
	state, err = app.NewState(dir, 1)
	require.NoError(t, err)
	require.Equal(t, "after rollback", state.Get("next"))
	require.Empty(t, state.Get("added"))
	require.NoError(t, state.Rollback())
	assertRollback(state)
}

func TestRepairM_1187_RollbackRefusesInvalidTarget(t *testing.T) {
	for _, corruption := range []string{"json", "hash"} {
		t.Run(corruption, func(t *testing.T) {
			dir := t.TempDir()
			state, err := app.NewState(dir, 1)
			require.NoError(t, err)
			for _, value := range []string{"older", "newer"} {
				state.Set("key", value)
				state.Finalize()
				_, err := state.Commit()
				require.NoError(t, err)
			}
			currentFile := filepath.Join(dir, "app_state.json")
			before, err := os.ReadFile(currentFile)
			require.NoError(t, err)
			previousFile := filepath.Join(dir, "prev_app_state.json")
			data, err := os.ReadFile(previousFile)
			require.NoError(t, err)
			if corruption == "json" {
				data = data[:len(data)-1]
			} else {
				var saved map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(data, &saved))
				saved["Hash"] = json.RawMessage(`"AA=="`)
				data, err = json.Marshal(saved)
				require.NoError(t, err)
			}
			require.NoError(t, os.WriteFile(previousFile, data, 0644))
			hash := bytes.Clone(state.Hash)
			require.Error(t, state.Rollback())
			require.Equal(t, "newer", state.Get("key"))
			require.Equal(t, uint64(2), state.Height)
			require.Equal(t, hash, state.Hash)
			after, err := os.ReadFile(currentFile)
			require.NoError(t, err)
			require.Equal(t, before, after)
			state, err = app.NewState(dir, 1)
			require.NoError(t, err)
			require.Equal(t, "newer", state.Get("key"))
		})
	}
}

func TestRepairM_1187_RollbackInstallFailure(t *testing.T) {
	dir := t.TempDir()
	state, err := app.NewState(dir, 1)
	require.NoError(t, err)
	for _, value := range []string{"older", "newer"} {
		state.Set("key", value)
		state.Finalize()
		_, err := state.Commit()
		require.NoError(t, err)
	}
	obstruction := filepath.Join(dir, "app_state.json.new")
	require.NoError(t, os.Mkdir(obstruction, 0755))
	hash := bytes.Clone(state.Hash)
	require.Error(t, state.Rollback())
	require.Equal(t, "newer", state.Get("key"))
	require.Equal(t, uint64(2), state.Height)
	require.Equal(t, hash, state.Hash)
	state, err = app.NewState(dir, 1)
	require.NoError(t, err)
	require.Equal(t, "newer", state.Get("key"))
	require.NoError(t, os.Remove(obstruction))
	require.NoError(t, state.Rollback())
	state, err = app.NewState(dir, 1)
	require.NoError(t, err)
	require.Equal(t, "older", state.Get("key"))
	require.Equal(t, uint64(1), state.Height)
}
