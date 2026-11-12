package rest_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestDevelopmentMarketKeepsPartialPoolStateExplicit(t *testing.T) {
	f := newMktMarketFixture(t, true)
	f.createPool(t, mktPoolA, mktTokenA, mktPoolOptical, 1760000000)
	f.setCursor(t, f.chain.head(t).Number.Uint64())

	for _, path := range []string{
		"/v2/pools/" + mktPoolA,
		"/v2/pools/" + mktPoolA + "/stats",
	} {
		t.Run(path, func(t *testing.T) {
			assertV2Error(t, getV2(t, f.srv.URL+path), 503, "DEPENDENCY_UNAVAILABLE")
		})
	}
	response := assertV2Success(t, mktPost(t, f.srv.URL+"/v2/pools/stats:batch", `{"poolAddresses":["`+mktPoolA+`"]}`), "StatsBatchResponse")
	data := mktData(response)
	if got := data["items"].([]any); len(got) != 0 {
		t.Fatalf("pool without committed statistics appeared in batch: %v", got)
	}
	if got := data["missing"].([]any); len(got) != 1 || got[0] != mktPoolA {
		t.Fatalf("missing pool was not reported: %v", got)
	}

	f.rank(t, "new", redis.Z{Score: 1, Member: mktPoolA})
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools?sort=new"), 503, "DEPENDENCY_UNAVAILABLE")
	f.insertPoolStats(t, mktPoolA, mktTokenA, "100", "0", 1, 0, 0, 1760000000, 1760000000)
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools/"+mktPoolA+"/stats"), 503, "DEPENDENCY_UNAVAILABLE")
	response = assertV2Success(t, mktPost(t, f.srv.URL+"/v2/pools/stats:batch", `{"poolAddresses":["`+mktPoolA+`"]}`), "StatsBatchResponse")
	if got := mktData(response)["missing"].([]any); len(got) != 1 || got[0] != mktPoolA {
		t.Fatalf("pool without receipt-verified reserves entered batch: %v", got)
	}
	if _, err := f.pool.Exec(context.Background(), `DELETE FROM indexer.cursors WHERE chain_id=125`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v2/pools/" + mktPoolA + "/stats",
		"/v2/rankings/trending",
	} {
		t.Run("no head "+path, func(t *testing.T) {
			assertV2Error(t, getV2(t, f.srv.URL+path), 503, "DEPENDENCY_UNAVAILABLE")
		})
	}
	assertV2Error(t, mktPost(t, f.srv.URL+"/v2/pools/stats:batch", `{"poolAddresses":["`+mktPoolA+`"]}`), 503, "DEPENDENCY_UNAVAILABLE")
	if _, err := f.pool.Exec(context.Background(), `DROP TABLE stats.pool_stats CASCADE`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v2/pools/" + mktPoolA,
		"/v2/pools/" + mktPoolA + "/stats",
	} {
		t.Run("unavailable "+path, func(t *testing.T) {
			assertV2Error(t, getV2(t, f.srv.URL+path), 503, "DEPENDENCY_UNAVAILABLE")
		})
	}
}
