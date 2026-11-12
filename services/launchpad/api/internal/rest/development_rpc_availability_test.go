package rest_test

import (
	"testing"
)

func TestDevelopmentComputedReadsReportChainUnavailable(t *testing.T) {
	f := newMktMarketFixture(t, true)
	f.setCursor(t, f.chain.head(t).Number.Uint64())
	if err := f.chain.backend.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v2/rankings/trending",
		"/v2/leaderboards/trader?window=24h",
		"/v2/holder-rewards/stats",
		"/v2/holder-rewards/pools",
		"/v2/platform/metrics?window=24h",
	} {
		t.Run(path, func(t *testing.T) {
			response := getV2(t, f.srv.URL+path)
			assertV2Error(t, response, 503, "DEPENDENCY_UNAVAILABLE")
			assertRetryable(t, response, true)
		})
	}
}
