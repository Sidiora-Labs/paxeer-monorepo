package rootmulti

import (
	"bytes"
	"context"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	protoio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/require"
)

func TestStorev2EmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	sink := oteltest.InstallLegacySink(t)
	telemetry.MeasureSince(time.Now(), "rootmulti_test", "control")
	require.Contains(t, sink.Keys(), "oteltest.rootmulti_test.control")

	assertNoLegacy := func(t *testing.T) {
		t.Helper()
		keys := sink.Keys()
		for _, key := range []string{
			"storeV2.sc.commit.latency",
			"storeV2.ss.version",
			"historical.abci.query",
			"iavl.store.total_key_bytes",
			"iavl.store.total_value_bytes",
			"iavl.store.total_num_keys",
			"state_sync.num_keys_exported",
		} {
			require.NotContains(t, keys, "oteltest."+key)
		}
	}

	t.Run("commits with state store", func(t *testing.T) {
		ssCfg := config.DefaultStateStoreConfig()
		ssCfg.Enable = true
		ssCfg.AsyncWriteBuffer = 0
		store, keys := newTestRootMultiWithSS(t, t.TempDir(), memiavlOnlyConfig(), ssCfg)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		require.NotNil(t, store.GetStateStore())

		for i, value := range []string{"old", "", "latest"} {
			if value != "" {
				cms := store.CacheMultiStore()
				cms.GetKVStore(keys["bank"]).Set([]byte("balance"), []byte(value))
				cms.Write()
			}
			before := scCommitLatency(reader)
			commit := store.Commit(true)
			require.Equal(t, int64(i+1), commit.Version)
			require.Equal(t, uint64(1), scCommitLatency(reader)-before)
			version, found := ssVersion(reader)
			require.True(t, found)
			require.Equal(t, commit.Version, version)
			require.Equal(t, commit.Version, store.GetStateStore().GetLatestVersion())
			if value != "" {
				require.Equal(t, []byte(value), store.GetKVStore(keys["bank"]).Get([]byte("balance")))
			}
			assertNoLegacy(t)
		}
	})

	t.Run("historical queries", func(t *testing.T) {
		cfg := memiavlOnlyConfig()
		cfg.MemIAVLConfig.SnapshotInterval = 10000
		cfg.HistoricalProofMaxInFlight = 1
		store, keys := newTestRootMulti(t, t.TempDir(), cfg)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		for _, value := range []string{"old", "latest"} {
			cms := store.CacheMultiStore()
			cms.GetKVStore(keys["bank"]).Set([]byte("balance"), []byte(value))
			cms.Write()
			store.Commit(true)
		}
		require.Equal(t, int64(2), store.LastCommitID().Version)
		assertNoLegacy(t)

		for _, prove := range []bool{false, true} {
			req := abci.RequestQuery{Path: "/bank/key", Data: []byte("balance"), Height: 1, Prove: prove}
			before := historicalAbciQuery(reader)
			resp := store.Query(context.Background(), req)
			require.EqualValues(t, 0, resp.Code, resp.Log)
			require.Equal(t, int64(1), resp.Height)
			require.Equal(t, []byte("old"), resp.Value)
			if prove {
				require.NotNil(t, resp.ProofOps)
				require.NotEmpty(t, resp.ProofOps.Ops)
			}
			require.Equal(t, int64(1), historicalAbciQuery(reader)-before)
			assertNoLegacy(t)

			require.NoError(t, store.tryAcquireHistProofPermit())
			before = historicalAbciQuery(reader)
			resp = store.Query(context.Background(), req)
			store.releaseHistProofPermit()
			require.NotEqualValues(t, 0, resp.Code)
			require.Contains(t, resp.Log, "historical proof busy")
			require.Equal(t, int64(1), historicalAbciQuery(reader)-before)
			assertNoLegacy(t)
		}
	})

	t.Run("snapshot export", func(t *testing.T) {
		store, keys := newTestRootMulti(t, t.TempDir(), memiavlOnlyConfig())
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		cms := store.CacheMultiStore()
		for name, pair := range map[string]struct{ key, value string }{
			"acc":  {key: "a", value: "one"},
			"bank": {key: "bb", value: "four"},
			"evm":  {key: "ccc", value: "seven"},
		} {
			cms.GetKVStore(keys[name]).Set([]byte(pair.key), []byte(pair.value))
		}
		cms.Write()
		commit := store.Commit(true)
		require.Equal(t, int64(1), commit.Version)
		before := stateSyncKeysExported(reader)
		var buf bytes.Buffer
		require.NoError(t, store.Snapshot(uint64(commit.Version), protoio.NewDelimitedWriter(&buf)))
		require.NotEmpty(t, buf.Bytes())
		keyBytes, found := iavlTotalKeyBytes(reader)
		require.True(t, found)
		require.Equal(t, int64(6), keyBytes)
		valueBytes, found := iavlTotalValueBytes(reader)
		require.True(t, found)
		require.Equal(t, int64(12), valueBytes)
		numKeys, found := iavlTotalNumKeys(reader)
		require.True(t, found)
		require.Equal(t, int64(3), numKeys)
		require.Equal(t, int64(3), stateSyncKeysExported(reader)-before)
		assertNoLegacy(t)
	})
}
