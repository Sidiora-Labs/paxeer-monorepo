package rest_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestDevelopmentMarketReportsUnavailableIndexedPools(t *testing.T) {
	f := newMktMarketFixture(t, true)
	f.setCursor(t, f.chain.head(t).Number.Uint64())
	f.rank(t, "trending", redis.Z{Score: 1, Member: mktPoolA})
	f.rank(t, "new", redis.Z{Score: 1, Member: mktPoolA})
	if _, err := f.pool.Exec(context.Background(), `DROP TABLE indexer.pools CASCADE`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v2/pools/" + mktPoolA,
		"/v2/pools/" + mktPoolA + "/stats",
		"/v2/pools/" + mktPoolA + "/holders",
		"/v2/pools?sort=new",
	} {
		t.Run(path, func(t *testing.T) {
			assertV2Error(t, getV2(t, f.srv.URL+path), 503, "DEPENDENCY_UNAVAILABLE")
		})
	}
	for _, path := range []string{"/v2/pools/" + mktPoolA + "/trades", "/v2/pools/" + mktPoolA + "/candles?interval=1m&from=2026-10-12T00:00:00.000Z&to=2026-10-12T00:01:00.000Z"} {
		response := getV2(t, f.srv.URL+path)
		if response.status != 404 {
			t.Fatalf("unregistered history route %s: %d", path, response.status)
		}
	}
	ranking := assertV2Success(t, getV2(t, f.srv.URL+"/v2/rankings/trending"), "RankingsResponse")
	items := mktData(ranking)["snapshot"].(map[string]any)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["poolAddress"] != mktPoolA {
		t.Fatalf("independent ranking projection: %v", ranking)
	}
	assertV2Error(t, mktPost(t, f.srv.URL+"/v2/pools/stats:batch", `{"poolAddresses":["`+mktPoolA+`"]}`), 503, "DEPENDENCY_UNAVAILABLE")
}
