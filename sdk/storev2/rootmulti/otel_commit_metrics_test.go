package rootmulti

import (
	"context"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/stretchr/testify/require"
)

const rootmultiMeterName = "paxcosmos_storev2_rootmulti"

func scCommitLatency(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(rootmultiMeterName, "sc_commit_latency")
}

func ssVersion(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(rootmultiMeterName, "ss_version")
}

func historicalAbciQuery(reader *oteltest.Reader) int64 {
	return reader.CounterSum(rootmultiMeterName, "historical_abci_query")
}

func TestCommitAndFlushRecordLatencyAndSSVersion(t *testing.T) {
	reader := oteltest.Install()
	ssCfg := config.DefaultStateStoreConfig()
	ssCfg.Enable = true
	ssCfg.AsyncWriteBuffer = 0
	store, keys := newTestRootMultiWithSS(t, t.TempDir(), memiavlOnlyConfig(), ssCfg)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NotNil(t, store.GetStateStore())

	beforeLatency := scCommitLatency(reader)
	first := store.Commit(true)
	require.Equal(t, int64(1), first.Version)
	version, found := ssVersion(reader)
	require.True(t, found)
	require.Equal(t, first.Version, version)
	require.Equal(t, uint64(1), scCommitLatency(reader)-beforeLatency)

	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "changed block", value: "v2"},
		{name: "empty block"},
		{name: "changed block after empty block", value: "v4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			beforeLatency := scCommitLatency(reader)
			beforeVersion, found := ssVersion(reader)
			require.True(t, found)
			if tc.value != "" {
				cms := store.CacheMultiStore()
				cms.GetKVStore(keys["bank"]).Set([]byte("balance"), []byte(tc.value))
				cms.Write()
			}

			commit := store.Commit(true)

			version, found := ssVersion(reader)
			require.True(t, found)
			require.Equal(t, beforeVersion+1, commit.Version)
			require.Equal(t, commit.Version, version)
			require.Equal(t, int64(1), version-beforeVersion)
			require.Equal(t, uint64(1), scCommitLatency(reader)-beforeLatency)
			require.Equal(t, commit.Version, store.GetStateStore().GetLatestVersion())
			if tc.value != "" {
				require.Equal(t, []byte(tc.value), store.GetKVStore(keys["bank"]).Get([]byte("balance")))
			}
		})
	}
}

func TestHistoricalQueryRecordsHistoricalAbciQuery(t *testing.T) {
	reader := oteltest.Install()
	for _, prove := range []bool{false, true} {
		name := "without proof"
		if prove {
			name = "with proof"
		}
		t.Run(name, func(t *testing.T) {
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

			require.NoError(t, store.tryAcquireHistProofPermit())
			before = historicalAbciQuery(reader)
			resp = store.Query(context.Background(), req)
			store.releaseHistProofPermit()
			require.NotEqualValues(t, 0, resp.Code)
			require.Contains(t, resp.Log, "historical proof busy")
			require.Equal(t, int64(1), historicalAbciQuery(reader)-before)

			before = historicalAbciQuery(reader)
			resp = store.Query(context.Background(), req)
			require.EqualValues(t, 0, resp.Code, resp.Log)
			require.Equal(t, []byte("old"), resp.Value)
			require.Equal(t, int64(1), historicalAbciQuery(reader)-before)

			req.Height = 2
			before = historicalAbciQuery(reader)
			resp = store.Query(context.Background(), req)
			require.EqualValues(t, 0, resp.Code, resp.Log)
			require.Equal(t, []byte("latest"), resp.Value)
			require.Equal(t, before, historicalAbciQuery(reader))
		})
	}
}
