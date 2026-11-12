package devsocial

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDirectRoutesRejectMalformedPositionsAndResources(t *testing.T) {
	h := newDirectRouteHarness(t)
	actor := directAddr(0xa11ce)
	comment := map[string]any{"body": "community message", "clientCommentId": uuid.NewString()}
	for _, tc := range []struct {
		method, path string
		body         any
		status       int
	}{
		{http.MethodGet, "/v2/pools/invalid/comments", nil, 422},
		{http.MethodPost, "/v2/pools/invalid/comments", comment, 422},
		{http.MethodGet, "/v2/pools/invalid/callouts", nil, 422},
		{http.MethodGet, "/v2/pools/" + h.pool + "/comments?cursor=invalid", nil, 400},
		{http.MethodGet, "/v2/pools/" + h.pool + "/comments?limit=0", nil, 422},
		{http.MethodGet, "/v2/pools/" + h.pool + "/comments?limit=101", nil, 422},
		{http.MethodGet, "/v2/comments/invalid", nil, 405},
		{http.MethodPatch, "/v2/comments/invalid", map[string]any{"body": "edited", "expectedRevision": "1"}, 422},
		{http.MethodDelete, "/v2/comments/invalid", nil, 422},
		{http.MethodPut, "/v2/comments/invalid/likes/me", nil, 422},
		{http.MethodDelete, "/v2/comments/invalid/likes/me", nil, 422},
		{http.MethodGet, "/v2/bounties?limit=0", nil, 422},
		{http.MethodGet, "/v2/bounties?limit=101", nil, 422},
		{http.MethodGet, "/v2/bounties?cursor=invalid", nil, 400},
		{http.MethodGet, "/v2/bounties/invalid", nil, 422},
		{http.MethodGet, "/v2/bounties/invalid/submissions", nil, 422},
		{http.MethodGet, "/v2/bounties/" + uuid.NewString() + "/submissions?cursor=invalid", nil, 400},
		{http.MethodPost, "/v2/bounties/invalid/submissions", map[string]any{"body": "entry", "clientSubmissionId": uuid.NewString()}, 422},
		{http.MethodPost, "/v2/bounties/invalid/submissions/invalid:accept", map[string]any{"awardRaw": "1", "expectedBountyRevision": "1"}, 422},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := h.do(tc.method, tc.path, actor, tc.body, nil)
			if response.status != tc.status {
				t.Fatalf("invalid resource response = %d %s", response.status, response.body)
			}
		})
	}
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/v2/bounties", map[string]any{"title": "Community artwork", "description": "Draw art", "assetAddress": h.token, "budgetRaw": "100", "poolAddress": h.pool}},
		{http.MethodPatch, "/v2/comments/" + uuid.NewString(), map[string]any{"body": "edited", "expectedRevision": "1"}},
		{http.MethodDelete, "/v2/comments/" + uuid.NewString(), nil},
		{http.MethodPut, "/v2/comments/" + uuid.NewString() + "/likes/me", nil},
	} {
		response := h.do(tc.method, tc.path, actor, tc.body, map[string]string{"Idempotency-Key": strings.Repeat("x", 129)})
		if response.status != http.StatusUnprocessableEntity {
			t.Fatalf("unbounded idempotency key = %d %s", response.status, response.body)
		}
	}
}
