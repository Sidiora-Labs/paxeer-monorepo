package devsocial

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/google/uuid"
)

func TestDevelopmentSocialRoutesReportUnavailableStore(t *testing.T) {
	h := newDirectRouteHarness(t)
	actor := directAddr(0xa11ce)
	id := uuid.NewString()
	h.runtime.pool.Close()
	for _, tc := range []struct {
		method, path string
		body         any
		headers      map[string]string
	}{
		{http.MethodGet, "/v2/bounties", nil, nil},
		{http.MethodGet, "/v2/bounties/stats", nil, nil},
		{http.MethodGet, "/v2/bounties/" + id, nil, nil},
		{http.MethodGet, "/v2/bounties/" + id + "/submissions", nil, nil},
		{http.MethodPost, "/v2/bounties", map[string]any{"title": "Community artwork", "description": "Art for the pool", "assetAddress": strings.ToLower(deployment.SettlementAddress), "budgetRaw": "100", "poolAddress": h.pool}, map[string]string{"Idempotency-Key": "bounty-unavailable"}},
		{http.MethodPost, "/v2/bounties/" + id + "/submissions", map[string]any{"body": "Community entry", "clientSubmissionId": uuid.NewString(), "evidenceUrls": []string{"https://example.org/entry"}}, nil},
		{http.MethodPost, "/v2/bounties/" + id + "/submissions/" + uuid.NewString() + ":accept", map[string]any{"awardRaw": "1", "expectedBountyRevision": "1"}, map[string]string{"Idempotency-Key": "accept-unavailable"}},
		{http.MethodGet, "/v2/pools/" + h.pool + "/callouts", nil, nil},
		{http.MethodGet, "/v2/pools/" + h.pool + "/comments", nil, nil},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := h.do(tc.method, tc.path, actor, tc.body, tc.headers)
			if response.status != http.StatusServiceUnavailable || !strings.Contains(string(response.body), "DEPENDENCY_UNAVAILABLE") {
				t.Fatalf("unavailable social route = %d %s", response.status, response.body)
			}
		})
	}
}
