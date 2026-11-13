package migration_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/composite"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv"
	flatcfg "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"
	m "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/migration"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func migrationRecoveryConfig(t *testing.T) config.StateCommitConfig {
	t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	cfg.FlatKVConfig = *flatcfg.DefaultTestConfig(t)
	cfg.FlatKVConfig.ReaderThreadsPerCore = 0.01
	cfg.FlatKVConfig.MiscPoolThreadsPerCore = 0
	cfg.KeysToMigratePerBlock = 1
	return cfg
}

// These handles are the production stores, using only private temporary disk.
type migrationRecoveryStores struct {
	mem    *memiavl.CommitStore
	flat   *flatkv.CommitStore
	closed bool
}

func migrationRecoveryOpen(t *testing.T, home string, cfg config.StateCommitConfig) *migrationRecoveryStores {
	t.Helper()
	mem := memiavl.NewCommitStore(home, cfg.MemIAVLConfig)
	require.NoError(t, mem.Initialize([]string{keys.EVMStoreKey, keys.BankStoreKey}))
	_, err := mem.LoadVersion(0, false)
	require.NoError(t, err)
	fcfg := cfg.FlatKVConfig
	fcfg.DataDir = utils.GetFlatKVPath(home)
	flat, err := flatkv.NewCommitStore(t.Context(), &fcfg)
	require.NoError(t, err)
	_, err = flat.LoadVersion(0, false)
	require.NoError(t, err)
	stores := &migrationRecoveryStores{mem: mem, flat: flat}
	t.Cleanup(func() { stores.close(t) })
	return stores
}

func (s *migrationRecoveryStores) close(t *testing.T) {
	t.Helper()
	if s.closed {
		return
	}
	s.closed = true
	require.NoError(t, s.mem.Close())
	require.NoError(t, s.flat.Close())
}

func migrationRecoveryChanges(store string, pairs ...*proto.KVPair) []*proto.NamedChangeSet {
	return []*proto.NamedChangeSet{{Name: store, Changeset: proto.ChangeSet{Pairs: pairs}}}
}

func migrationRecoveryPut(key, value string) *proto.KVPair {
	return &proto.KVPair{Key: []byte(key), Value: []byte(value)}
}

func migrationRecoveryCommit(t *testing.T, s *migrationRecoveryStores, height int64) {
	t.Helper()
	v, err := s.mem.Commit()
	require.NoError(t, err)
	require.Equal(t, height, v)
	v, err = s.flat.Commit()
	require.NoError(t, err)
	require.Equal(t, height, v)
}

func migrationRecoveryBoundary(t *testing.T, flat *flatkv.CommitStore) m.MigrationBoundary {
	t.Helper()
	raw, found := flat.Get(m.MigrationStore, []byte(m.MigrationBoundaryKey))
	require.True(t, found)
	boundary, err := m.DeserializeMigrationBoundary(raw)
	require.NoError(t, err)
	return boundary
}

func migrationRecoveryRead(t *testing.T, router m.Router, key, want string) {
	t.Helper()
	got, found, err := router.Read(keys.EVMStoreKey, []byte(key))
	require.NoError(t, err)
	require.True(t, found, "missing %q", key)
	require.Equal(t, []byte(want), got)
}

func migrationRecoveryUnion(t *testing.T, s *migrationRecoveryStores, want map[string]string) {
	t.Helper()
	union := make(map[string]string)
	// Iterate each backend's logical EVM view, not physical flatkv bucket counts.
	old, err := s.mem.Iterator(keys.EVMStoreKey, nil, nil, true)
	require.NoError(t, err)
	for ; old.Valid(); old.Next() {
		union[string(old.Key())] = string(old.Value())
	}
	require.NoError(t, old.Error())
	require.NoError(t, old.Close())
	next, err := s.flat.Iterator(keys.EVMStoreKey, nil, nil, true)
	require.NoError(t, err)
	for ; next.Valid(); next.Next() {
		_, duplicate := union[string(next.Key())]
		require.False(t, duplicate, "normal migration must not leave duplicate source rows")
		union[string(next.Key())] = string(next.Value())
	}
	require.NoError(t, next.Error())
	require.NoError(t, next.Close())
	require.Equal(t, want, union, "compare values and the complete union, including keys beyond the cursor")
}

func TestMigrationRecoveryPartition(t *testing.T) {
	cfg := migrationRecoveryConfig(t)
	s := migrationRecoveryOpen(t, t.TempDir(), cfg)
	require.NoError(t, s.mem.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey,
		migrationRecoveryPut("a", "old-a"), migrationRecoveryPut("b", "old-b"), migrationRecoveryPut("c", "old-c"))))
	migrationRecoveryCommit(t, s, 1)
	router, err := m.BuildRouter(t.Context(), config.MigrateEVM, s.mem, s.flat, 1)
	require.NoError(t, err)
	require.NoError(t, router.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey,
		migrationRecoveryPut("a", "overlay-a"), migrationRecoveryPut("c", "updated-c"), migrationRecoveryPut("z-new-1", "new-1")), true))
	boundary := migrationRecoveryBoundary(t, s.flat)
	require.Equal(t, []byte("a"), boundary.Key())
	require.False(t, boundary.IsMigrated(keys.EVMStoreKey, []byte("z-new-1")))
	require.Nil(t, s.mem.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("z-new-1")))
	value, found := s.flat.Get(keys.EVMStoreKey, []byte("z-new-1"))
	require.True(t, found)
	require.Equal(t, []byte("new-1"), value)
	for key, want := range map[string]string{"a": "overlay-a", "b": "old-b", "c": "updated-c", "z-new-1": "new-1"} {
		migrationRecoveryRead(t, router, key, want)
	}
	migrationRecoveryUnion(t, s, map[string]string{"a": "overlay-a", "b": "old-b", "c": "updated-c", "z-new-1": "new-1"})
	require.NoError(t, router.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey,
		&proto.KVPair{Key: []byte("z-new-1"), Delete: true}, migrationRecoveryPut("z-new-2", "new-2")), false))
	unchanged := migrationRecoveryBoundary(t, s.flat)
	require.True(t, boundary.Equals(unchanged), "later flushes must preserve the cursor")
	_, found, err = router.Read(keys.EVMStoreKey, []byte("z-new-1"))
	require.NoError(t, err)
	require.False(t, found)
	for range 2 {
		require.NoError(t, router.ApplyChangeSets(nil, true))
	}
	raw, found := s.flat.Get(m.MigrationStore, []byte(m.MigrationVersionKey))
	require.True(t, found)
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(raw))
	_, found = s.flat.Get(m.MigrationStore, []byte(m.MigrationBoundaryKey))
	require.False(t, found, "fresh keys must not extend the migration tail")
	migrationRecoveryUnion(t, s, map[string]string{"a": "overlay-a", "b": "old-b", "c": "updated-c", "z-new-2": "new-2"})
	doc, err := os.ReadFile("OPERATIONS.md")
	require.NoError(t, err)
	require.Contains(t, string(doc), "TestApplyChangeSets_BrandNewKeysDoNotExtendMigrationTail")
	require.Contains(t, string(doc), "logical union and authoritative values")
}

func TestMigrationRecoveryTerminalRouting(t *testing.T) {
	for _, mode := range []config.WriteMode{config.MemiavlOnly, config.FlatKVOnly} {
		t.Run(string(mode), func(t *testing.T) {
			cfg := migrationRecoveryConfig(t)
			cfg.WriteMode = mode
			home := t.TempDir()
			cs, err := composite.NewCompositeCommitStore(t.Context(), home, cfg)
			require.NoError(t, err)
			require.NoError(t, cs.Initialize([]string{keys.EVMStoreKey, keys.BankStoreKey}))
			_, err = cs.LoadVersion(0, false)
			require.NoError(t, err)
			require.NoError(t, cs.ApplyChangeSets(migrationRecoveryChanges(keys.BankStoreKey, migrationRecoveryPut("balance", "terminal-value"))))
			height, err := cs.Commit()
			require.NoError(t, err)
			require.Equal(t, int64(1), height)
			require.Equal(t, []byte("terminal-value"), cs.GetChildStoreByName(keys.BankStoreKey).Get([]byte("balance")))
			require.NoError(t, cs.Close())

			// Drive the same builder directly with only the required backend present.
			if mode == config.MemiavlOnly {
				mem := memiavl.NewCommitStore(home, cfg.MemIAVLConfig)
				_, err = mem.LoadVersion(0, false)
				require.NoError(t, err)
				defer mem.Close()
				router, err := m.BuildRouter(t.Context(), mode, mem, nil, 0)
				require.NoError(t, err)
				require.IsType(t, &m.PassthroughRouter{}, router)
				got, found, err := router.Read(keys.BankStoreKey, []byte("balance"))
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, []byte("terminal-value"), got)
				require.NoError(t, router.ApplyChangeSets(migrationRecoveryChanges(keys.BankStoreKey, migrationRecoveryPut("balance", "updated")), false))
				require.Equal(t, []byte("updated"), mem.GetChildStoreByName(keys.BankStoreKey).Get([]byte("balance")))
			} else {
				fcfg := cfg.FlatKVConfig
				fcfg.DataDir = utils.GetFlatKVPath(home)
				flat, err := flatkv.NewCommitStore(t.Context(), &fcfg)
				require.NoError(t, err)
				_, err = flat.LoadVersion(0, false)
				require.NoError(t, err)
				defer flat.Close()
				router, err := m.BuildRouter(t.Context(), mode, nil, flat, 0)
				require.NoError(t, err)
				require.IsType(t, &m.PassthroughRouter{}, router)
				got, found, err := router.Read(keys.BankStoreKey, []byte("balance"))
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, []byte("terminal-value"), got)
				require.NoError(t, router.ApplyChangeSets(migrationRecoveryChanges(keys.BankStoreKey, migrationRecoveryPut("balance", "updated")), false))
				got, found = flat.Get(keys.BankStoreKey, []byte("balance"))
				require.True(t, found)
				require.Equal(t, []byte("updated"), got)
			}
		})
	}
	doc, err := os.ReadFile("README.md")
	require.NoError(t, err)
	require.Contains(t, string(doc), "routerCtx, cs.config.WriteMode, cs.memIAVL, cs.flatKV")
	require.Contains(t, string(doc), "Neither terminal mode constructs a `MigrationManager`")
}

func TestMigrationRecoveryCrashReopen(t *testing.T) {
	cfg := migrationRecoveryConfig(t)
	cfg.WriteMode = config.MigrateEVM
	home := t.TempDir()
	s := migrationRecoveryOpen(t, home, cfg)
	require.NoError(t, s.mem.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey,
		migrationRecoveryPut("a", "original-a"), migrationRecoveryPut("b", "original-b"), migrationRecoveryPut("c", "original-c"))))
	migrationRecoveryCommit(t, s, 1)
	router, err := m.BuildRouter(t.Context(), cfg.WriteMode, s.mem, s.flat, 1)
	require.NoError(t, err)
	require.NoError(t, router.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey, migrationRecoveryPut("z", "uncommitted-z")), true))
	require.Nil(t, s.mem.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("a")))
	boundary := migrationRecoveryBoundary(t, s.flat)
	require.Equal(t, []byte("a"), boundary.Key())
	// Interrupt the actual ordered commit path after the old commit: do not
	// commit flatkv. Closing loses its pending batch, preserving version 1.
	height, err := s.mem.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(2), height)
	require.Equal(t, int64(1), s.flat.Version())
	s.close(t)

	cs, err := composite.NewCompositeCommitStore(t.Context(), home, cfg)
	require.NoError(t, err)
	require.NoError(t, cs.Initialize([]string{keys.EVMStoreKey, keys.BankStoreKey}))
	_, err = cs.LoadVersion(0, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), cs.Version(), "latest reopen must roll the ahead backend back")
	for key, want := range map[string]string{"a": "original-a", "b": "original-b", "c": "original-c"} {
		require.Equal(t, []byte(want), cs.GetChildStoreByName(keys.EVMStoreKey).Get([]byte(key)))
	}
	require.Nil(t, cs.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("z")))
	require.NoError(t, cs.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey, migrationRecoveryPut("z", "replayed-z"))))
	height, err = cs.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(2), height)
	require.NoError(t, cs.Close())

	// Inspect recovered durable metadata and the complete logical union, then
	// reopen once more to prove rollback and resumed progress survive restart.
	s = migrationRecoveryOpen(t, home, cfg)
	require.Equal(t, int64(2), s.mem.Version())
	require.Equal(t, int64(2), s.flat.Version())
	boundary = migrationRecoveryBoundary(t, s.flat)
	require.Equal(t, []byte("a"), boundary.Key())
	migrationRecoveryUnion(t, s, map[string]string{"a": "original-a", "b": "original-b", "c": "original-c", "z": "replayed-z"})
	s.close(t)
	cs, err = composite.NewCompositeCommitStore(t.Context(), home, cfg)
	require.NoError(t, err)
	require.NoError(t, cs.Initialize([]string{keys.EVMStoreKey, keys.BankStoreKey}))
	_, err = cs.LoadVersion(0, false)
	require.NoError(t, err)
	defer cs.Close()
	require.NoError(t, cs.ApplyChangeSets(nil))
	height, err = cs.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(3), height)
	require.Equal(t, []byte("original-b"), cs.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("b")))
	doc, err := os.ReadFile("OPERATIONS.md")
	require.NoError(t, err)
	require.Contains(t, string(doc), "first-ever interrupted commit")
	require.Contains(t, string(doc), "interruption/reopen")
}

func TestMigrationRecoveryOldWriterFailureStopsDispatch(t *testing.T) {
	cfg := migrationRecoveryConfig(t)
	s := migrationRecoveryOpen(t, t.TempDir(), cfg)
	require.NoError(t, s.mem.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey,
		migrationRecoveryPut("a", "original-a"), migrationRecoveryPut("b", "original-b"))))
	migrationRecoveryCommit(t, s, 1)
	handle, err := s.mem.LoadVersion(1, true)
	require.NoError(t, err)
	old := handle.(*memiavl.CommitStore)
	defer old.Close()
	manager, err := m.NewMigrationManager(1, 0, 1,
		old.Get,
		func(changes []*proto.NamedChangeSet, _ bool) error { return old.ApplyChangeSets(changes) },
		func(store string, key []byte) ([]byte, bool, error) {
			value, found := s.flat.Get(store, key)
			return value, found, nil
		},
		func(changes []*proto.NamedChangeSet, _ bool) error { return s.flat.ApplyChangeSets(changes) },
		m.NewMemiavlMigrationIterator(old.GetDB(), []string{keys.EVMStoreKey}), nil)
	require.NoError(t, err)
	err = manager.ApplyChangeSets(nil, true)
	require.ErrorContains(t, err, "failed to apply changes to old database")
	// The error comes from the real read-only source store, not an injected
	// failure. The real destination must not receive either values or metadata.
	for _, key := range []string{"a", "b"} {
		_, found := s.flat.Get(keys.EVMStoreKey, []byte(key))
		require.False(t, found)
	}
	for _, key := range []string{m.MigrationBoundaryKey, m.MigrationVersionKey} {
		_, found := s.flat.Get(m.MigrationStore, []byte(key))
		require.False(t, found)
	}
	require.Equal(t, []byte("original-a"), old.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("a")))
}

func TestMigrationRecoveryVersionZeroMigrationEntry(t *testing.T) {
	cfg := migrationRecoveryConfig(t)
	home := t.TempDir()
	s := migrationRecoveryOpen(t, home, cfg)
	require.NoError(t, s.mem.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey,
		migrationRecoveryPut("a", "original-a"), migrationRecoveryPut("b", "original-b"))))
	height, err := s.mem.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(1), height)
	require.Equal(t, int64(0), s.flat.Version())
	s.close(t)
	cfg.WriteMode = config.MigrateEVM
	cs, err := composite.NewCompositeCommitStore(t.Context(), home, cfg)
	require.NoError(t, err)
	require.NoError(t, cs.Initialize([]string{keys.EVMStoreKey, keys.BankStoreKey}))
	_, err = cs.LoadVersion(0, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), cs.Version(), "version-zero entry must preserve the existing source height")
	require.Equal(t, []byte("original-a"), cs.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("a")))
	require.NoError(t, cs.ApplyChangeSets(nil))
	height, err = cs.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(2), height, "fresh flatkv must be seeded for the next commit, not rolled back")
	require.NoError(t, cs.Close())
	s = migrationRecoveryOpen(t, home, cfg)
	require.Equal(t, int64(2), s.mem.Version())
	require.Equal(t, int64(2), s.flat.Version())
	migrationRecoveryUnion(t, s, map[string]string{"a": "original-a", "b": "original-b"})
}

func migrationRecoveryGauges(t *testing.T, reader *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	gauges := make(map[string]int64)
	for _, scope := range data.ScopeMetrics {
		if scope.Scope.Name != "paxdb_migration" {
			continue
		}
		for _, instrument := range scope.Metrics {
			gauge, ok := instrument.Data.(metricdata.Gauge[int64])
			if !ok {
				continue
			}
			require.Len(t, gauge.DataPoints, 1, "gauge %s must have one series regardless of cursor history", instrument.Name)
			require.Zero(t, gauge.DataPoints[0].Attributes.Len(), "no cursor, module, or version labels on %s", instrument.Name)
			gauges[instrument.Name] = gauge.DataPoints[0].Value
		}
	}
	require.Len(t, gauges, 4)
	return gauges
}

func TestMigrationRecoveryBoundaryMetricCardinality(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	cfg := migrationRecoveryConfig(t)
	s := migrationRecoveryOpen(t, t.TempDir(), cfg)
	const count = 4096
	pairs := make([]*proto.KVPair, count+1)
	for i := range pairs {
		pairs[i] = migrationRecoveryPut(fmt.Sprintf("key-%05d", i), "value")
	}
	require.NoError(t, s.mem.ApplyChangeSets(migrationRecoveryChanges(keys.EVMStoreKey, pairs...)))
	migrationRecoveryCommit(t, s, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	router, err := m.BuildRouter(ctx, config.MigrateEVM, s.mem, s.flat, 1)
	require.NoError(t, err)
	initial := migrationRecoveryGauges(t, reader)
	require.Equal(t, int64(0), initial["paxdb_migration_boundary_snapshot"])
	for i := range count {
		require.NoError(t, router.ApplyChangeSets(nil, true))
		boundary := migrationRecoveryBoundary(t, s.flat)
		require.Equal(t, pairs[i].Key, boundary.Key(), "each sample must represent a distinct real migrated boundary")
		got := migrationRecoveryGauges(t, reader)
		require.Equal(t, int64(1), got["paxdb_migration_boundary_snapshot"])
		require.Equal(t, int64(0), got["paxdb_migration_version"])
		require.Equal(t, int64(i+1), got["paxdb_migration_keys_migrated"])
		require.Equal(t, int64((i+1)*14), got["paxdb_migration_bytes_migrated"])
	}
	require.NoError(t, router.ApplyChangeSets(nil, true))
	complete := migrationRecoveryGauges(t, reader)
	require.Equal(t, int64(2), complete["paxdb_migration_boundary_snapshot"])
	require.Equal(t, int64(1), complete["paxdb_migration_version"])
	require.Equal(t, int64(count+1), complete["paxdb_migration_keys_migrated"])
	require.Equal(t, int64((count+1)*14), complete["paxdb_migration_bytes_migrated"])
	// Exercise the production sampling loop as well as the immediate apply path.
	metrics := m.NewMigrationMetrics(ctx, 3, time.Millisecond)
	metrics.SetVersion(2)
	metrics.SetBoundary(m.NewMigrationBoundary(keys.BankStoreKey, []byte{0, 255, 0}))
	time.Sleep(5 * time.Millisecond)
	sampled := migrationRecoveryGauges(t, reader)
	require.Equal(t, int64(1), sampled["paxdb_migration_boundary_snapshot"])
	require.Equal(t, int64(2), sampled["paxdb_migration_version"])
	metrics.SetVersion(3)
	metrics.SetBoundary(m.MigrationBoundaryComplete)
	metrics.Close()
	final := migrationRecoveryGauges(t, reader)
	require.Equal(t, int64(2), final["paxdb_migration_boundary_snapshot"])
	require.Equal(t, int64(3), final["paxdb_migration_version"])
}
