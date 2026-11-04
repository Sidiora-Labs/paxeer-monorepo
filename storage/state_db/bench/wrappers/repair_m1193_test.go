package wrappers_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonevm "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/bench"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/bench/wrappers"
	flatkvConfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
)

func TestRepairM_1193_HelperLocalBackends(t *testing.T) {
	for _, backend := range []wrappers.DBType{
		wrappers.NoOp,
		wrappers.MemIAVL,
		wrappers.FlatKV,
		wrappers.CompositeDual,
		wrappers.CompositeSplit,
		wrappers.CompositeCosmos,
		wrappers.SSComposite,
		wrappers.CompositeDual_SSComposite,
	} {
		t.Run(string(backend), func(t *testing.T) {
			scenario := bench.TestScenario{Backend: backend}
			db, err := scenario.OpenDB(t.Context(), t.TempDir())
			require.NoError(t, err)
			require.NotNil(t, db)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			require.Zero(t, db.Version())

			key := commonevm.BuildEVMKey(commonevm.EVMKeyStorage, bytes.Repeat([]byte{0x19}, 52))
			for version := int64(1); version <= 2; version++ {
				value := bytes.Repeat([]byte{byte(version)}, bench.ValueSize)
				entry := &proto.ChangelogEntry{
					Version: version,
					Changesets: []*proto.NamedChangeSet{{
						Name: wrappers.EVMStoreName,
						Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{
							Key: key, Value: value,
						}}},
					}},
				}
				require.NoError(t, db.ApplyChangeSets(entry))
				committed, err := db.Commit()
				require.NoError(t, err)
				require.Equal(t, version, committed)
				require.Equal(t, version, db.Version())
				if backend == wrappers.NoOp {
					data, found, err := db.Read(key)
					require.NoError(t, err)
					require.False(t, found)
					require.Nil(t, data)
				} else {
					require.EventuallyWithT(t, func(c *assert.CollectT) {
						data, found, err := db.Read(key)
						require.NoError(c, err)
						require.True(c, found)
						require.Equal(c, value, data)
					}, 5*time.Second, 10*time.Millisecond)
				}
			}
		})
	}
}

func TestRepairM_1193_FactoryRejectsConfigShapes(t *testing.T) {
	for _, tc := range []struct {
		backend  wrappers.DBType
		typedNil any
		value    any
		wantType string
	}{
		{wrappers.FlatKV, (*flatkvConfig.Config)(nil), *flatkvConfig.DefaultConfig(), "*flatkvConfig.Config"},
		{wrappers.SSComposite, (*config.StateStoreConfig)(nil), config.DefaultStateStoreConfig(), "*config.StateStoreConfig"},
		{wrappers.CompositeDual_SSComposite, (*config.StateStoreConfig)(nil), config.DefaultStateStoreConfig(), "*config.StateStoreConfig"},
		{wrappers.SSHistoricalOffload, (*wrappers.HistoricalOffloadConfig)(nil), wrappers.HistoricalOffloadConfig{}, "*wrappers.HistoricalOffloadConfig"},
	} {
		t.Run(string(tc.backend), func(t *testing.T) {
			for _, shape := range []struct {
				name  string
				value any
			}{
				{"nil", nil},
				{"typed_nil", tc.typedNil},
				{"value_instead_of_pointer", tc.value},
				{"wrong_pointer", new(string)},
				{"scalar", 17},
			} {
				t.Run(shape.name, func(t *testing.T) {
					dir := filepath.Join(t.TempDir(), "must-not-open")
					db, err := wrappers.NewDBImpl(t.Context(), tc.backend, dir, shape.value)
					require.Nil(t, db)
					require.ErrorContains(t, err, string(tc.backend))
					require.ErrorContains(t, err, "configuration must be a non-nil "+tc.wantType)
					require.NoDirExists(t, dir)
				})
			}
		})
	}

	for _, backend := range []wrappers.DBType{
		wrappers.NoOp, wrappers.MemIAVL, wrappers.CompositeDual, wrappers.CompositeSplit, wrappers.CompositeCosmos,
	} {
		t.Run(string(backend), func(t *testing.T) {
			for _, cfg := range []any{new(string), (*flatkvConfig.Config)(nil), flatkvConfig.DefaultConfig()} {
				dir := filepath.Join(t.TempDir(), "must-not-open")
				db, err := wrappers.NewDBImpl(t.Context(), backend, dir, cfg)
				require.Nil(t, db)
				require.ErrorContains(t, err, fmt.Sprintf("%s configuration must be nil", backend))
				require.NoDirExists(t, dir)
			}
		})
	}
}

func TestRepairM_1193_HelperOffloadConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *wrappers.HistoricalOffloadConfig
		want string
	}{
		{"missing_destination", nil, "TestScenario.HistoricalOffloadConfig with explicit Kafka brokers and topic"},
		{"unsupported_provider", &wrappers.HistoricalOffloadConfig{Provider: "local"}, "unsupported historical offload provider"},
		{"missing_kafka_config", &wrappers.HistoricalOffloadConfig{Provider: "kafka"}, "historical offload kafka config is required"},
		{"missing_brokers", &wrappers.HistoricalOffloadConfig{
			Provider: "kafka", Kafka: &wrappers.KafkaHistoricalOffloadConfig{Topic: "repair-m1193"},
		}, "kafka brokers are required"},
		{"missing_topic", &wrappers.HistoricalOffloadConfig{
			Provider: "kafka", Kafka: &wrappers.KafkaHistoricalOffloadConfig{Brokers: []string{"127.0.0.1:1"}},
		}, "kafka topic is required"},
		{"invalid_acks", &wrappers.HistoricalOffloadConfig{
			Provider: "kafka", Kafka: &wrappers.KafkaHistoricalOffloadConfig{
				Brokers: []string{"127.0.0.1:1"}, Topic: "repair-m1193", RequiredAcks: "invalid",
			},
		}, "unsupported kafka required acks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "must-not-open")
			scenario := bench.TestScenario{Backend: wrappers.SSHistoricalOffload, HistoricalOffloadConfig: tc.cfg}
			db, err := scenario.OpenDB(t.Context(), dir)
			require.Nil(t, db)
			require.ErrorContains(t, err, tc.want)
			require.NoDirExists(t, dir)
		})
	}

	t.Run("explicit_destination", func(t *testing.T) {
		cfg := &wrappers.HistoricalOffloadConfig{
			Provider: "kafka",
			Kafka: &wrappers.KafkaHistoricalOffloadConfig{
				Brokers: []string{"127.0.0.1:1"}, Topic: "repair-m1193",
			},
		}
		scenario := bench.TestScenario{Backend: wrappers.SSHistoricalOffload, HistoricalOffloadConfig: cfg}
		db, err := scenario.OpenDB(t.Context(), t.TempDir())
		require.NoError(t, err)
		require.NotNil(t, db)
		// Construction validates the real Kafka writer without dialing a broker.
		require.Equal(t, "snappy", cfg.Kafka.Compression)
		require.Equal(t, "none", cfg.Kafka.RequiredAcks)
		require.Zero(t, db.Version())
		require.NoError(t, db.Close())
	})
}

func TestRepairM_1193_UnknownBackend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "must-not-open")
	scenario := bench.TestScenario{Backend: wrappers.DBType("unknown")}
	db, err := scenario.OpenDB(t.Context(), dir)
	require.Nil(t, db)
	require.ErrorContains(t, err, "unsupported DB type: unknown")
	require.NoDirExists(t, dir)
}
