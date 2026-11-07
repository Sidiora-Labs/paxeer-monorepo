package evm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"

	commonevm "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
)

func repairM1095Snapshot(t *testing.T, count int) []types.SnapshotNode {
	t.Helper()
	cfg := testConfig()
	cfg.ImportNumWorkers = 1
	cfg.UseDefaultComparer = true
	source, err := mvcc.OpenDB(t.TempDir(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, source.Close()) })
	var evmPairs, bankPairs []*proto.KVPair
	for i := 0; i < count; i++ {
		address := make([]byte, commonevm.AddressLen)
		binary.BigEndian.PutUint64(address[len(address)-8:], uint64(i+1))
		key := commonevm.BuildEVMKey(StoreNonce, address)
		evmPairs = append(evmPairs, &proto.KVPair{Key: key, Value: []byte(fmt.Sprintf("evm-%d", i))})
		bankPairs = append(bankPairs, &proto.KVPair{Key: key, Value: []byte(fmt.Sprintf("bank-%d", i))})
	}
	require.NoError(t, source.ApplyChangesetSync(7, []*proto.NamedChangeSet{
		{Name: EVMStoreKey, Changeset: proto.ChangeSet{Pairs: evmPairs}},
		{Name: "bank", Changeset: proto.ChangeSet{Pairs: bankPairs}},
	}))
	var nodes []types.SnapshotNode
	for _, module := range []string{EVMStoreKey, "bank"} {
		stopped, err := source.RawIterate(module, func(key, value []byte, version int64) bool {
			require.Equal(t, int64(7), version)
			nodes = append(nodes, types.SnapshotNode{
				StoreKey: module, Key: bytes.Clone(key), Value: bytes.Clone(value),
			})
			return false
		})
		require.NoError(t, err)
		require.False(t, stopped)
	}
	require.Len(t, nodes, 2*count)
	require.NoError(t, source.Close())
	return nodes
}

func repairM1095Import(t *testing.T, store types.StateStore, version int64, nodes []types.SnapshotNode, capacity int) error {
	t.Helper()
	input := make(chan types.SnapshotNode, capacity)
	produced := make(chan int, 1)
	result := make(chan error, 1)
	abort := make(chan struct{})
	defer close(abort)
	go func() {
		defer close(input)
		sent := 0
		for _, node := range nodes {
			select {
			case input <- node:
				sent++
			case <-abort:
				return
			}
		}
		produced <- sent
	}()
	go func() { result <- store.Import(version, input) }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	var err error
	select {
	case err = <-result:
	case <-timer.C:
		t.Fatal("import did not return within five seconds")
	}
	select {
	case sent := <-produced:
		require.Equal(t, len(nodes), sent, "the complete snapshot producer must finish")
	case <-timer.C:
		t.Fatal("import returned with its snapshot producer blocked")
	}
	return err
}

func TestRepairM_1095_BackendFailureDrainsAndJoins(t *testing.T) {
	// The separate importer flushes at 10,000 nodes; leave substantially more
	// than the filter's 100-node buffer outstanding when that flush fails.
	nodes := repairM1095Snapshot(t, 12000)
	for _, capacity := range []int{0, 100} {
		for _, failure := range []string{"closed", "invalid-version"} {
			t.Run(fmt.Sprintf("%s/input-buffer=%d", failure, capacity), func(t *testing.T) {
				t.Cleanup(leaktest.CheckTimeout(t, 5*time.Second))
				cfg := testConfig()
				cfg.ImportNumWorkers = 1
				cfg.SeparateEVMSubDBs = failure == "invalid-version"
				backend, err := NewEVMStateStore(t.TempDir(), cfg)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, backend.Close()) })
				version := int64(-1)
				wantError := "version must be non-negative"
				if failure == "closed" {
					require.NoError(t, backend.Close())
					version = 7
					wantError = "EVMStateStore: database is closed"
				}
				// Compose two production StateStore adapters, without an error
				// double: the consumer is a real closed store or a real separate
				// importer whose Pebble batch rejects the negative version.
				store := &EVMStateStore{managedDBs: []types.StateStore{backend}}
				err = repairM1095Import(t, store, version, nodes, capacity)
				require.EqualError(t, err, wantError, "preserve the backend's exact failure")
			})
		}
	}
}

func TestRepairM_1095_BackendEarlyReturnDrainsAndJoins(t *testing.T) {
	t.Cleanup(leaktest.CheckTimeout(t, 5*time.Second))
	nodes := repairM1095Snapshot(t, 320)
	cfg := testConfig()
	// The real MVCC importer with zero workers returns before reading input.
	// Exercise cleanup for early nil returns as well as the failures above.
	cfg.ImportNumWorkers = 0
	store, err := NewEVMStateStore(t.TempDir(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, repairM1095Import(t, store, 7, nodes, 0))
}

func TestRepairM_1095_UnifiedImportFiltersAndReopens(t *testing.T) {
	nodes := repairM1095Snapshot(t, 320)
	for _, workers := range []int{1, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Cleanup(leaktest.CheckTimeout(t, 5*time.Second))
			cfg := testConfig()
			cfg.ImportNumWorkers = workers
			dir := t.TempDir()
			store, err := NewEVMStateStore(dir, cfg)
			require.NoError(t, err)
			t.Cleanup(func() {
				if store != nil {
					require.NoError(t, store.Close())
				}
			})
			require.NoError(t, repairM1095Import(t, store, 7, nodes, 100))
			check := func() {
				for _, node := range nodes {
					if node.StoreKey != EVMStoreKey {
						continue
					}
					value, err := store.Get(EVMStoreKey, 7, node.Key)
					require.NoError(t, err)
					require.Equal(t, node.Value, value)
				}
				evmCount, bankCount := 0, 0
				stopped, err := store.primaryDB().RawIterate(EVMStoreKey, func(_, _ []byte, version int64) bool {
					require.Equal(t, int64(7), version)
					evmCount++
					return false
				})
				require.NoError(t, err)
				require.False(t, stopped)
				stopped, err = store.primaryDB().RawIterate("bank", func(_, _ []byte, _ int64) bool {
					bankCount++
					return false
				})
				require.NoError(t, err)
				require.False(t, stopped)
				require.Equal(t, 320, evmCount)
				require.Zero(t, bankCount, "non-EVM namespaces must never reach the backend")
				require.Equal(t, int64(7), store.GetLatestVersion())
			}
			check()
			require.NoError(t, store.Close())
			store = nil
			store, err = NewEVMStateStore(dir, cfg)
			require.NoError(t, err)
			check()
		})
	}
}
