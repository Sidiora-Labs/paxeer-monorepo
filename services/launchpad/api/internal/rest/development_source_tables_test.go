package rest_test

import (
	"context"
	"testing"
)

func TestDevelopmentMarketReportsUnavailableReadSources(t *testing.T) {
	for _, tc := range []struct {
		name, drop, path string
	}{
		{"pool statistics", "DROP TABLE stats.pool_stats CASCADE", "/v2/pools/" + mktPoolA + "/stats"},
		{"batched statistics", "DROP TABLE stats.pool_stats CASCADE", "/v2/pools/stats:batch"},
		{"pool detail", "DROP TABLE indexer.pool_state_snapshots CASCADE", "/v2/pools/" + mktPoolA},
		{"holder balances", "DROP TABLE stats.pool_holders CASCADE", "/v2/pools/" + mktPoolA + "/holders"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMktMarketFixture(t, true)
			f.createPool(t, mktPoolA, mktTokenA, mktPoolOptical, 1760000000)
			f.setCursor(t, f.chain.head(t).Number.Uint64())
			if _, err := f.pool.Exec(context.Background(), tc.drop); err != nil {
				t.Fatal(err)
			}
			if tc.name == "batched statistics" {
				assertV2Error(t, mktPost(t, f.srv.URL+tc.path, `{"poolAddresses":["`+mktPoolA+`"]}`), 503, "DEPENDENCY_UNAVAILABLE")
			} else {
				assertV2Error(t, getV2(t, f.srv.URL+tc.path), 503, "DEPENDENCY_UNAVAILABLE")
			}
		})
	}
}
