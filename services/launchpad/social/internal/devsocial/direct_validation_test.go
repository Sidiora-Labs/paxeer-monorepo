package devsocial

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDirectContractRejectsInvalidContentAndMissingTargets(t *testing.T) {
	h := newDirectRouteHarness(t)
	actor := directAddr(0xa11ce)
	missing := uuid.NewString()
	for _, tc := range []struct {
		method, path string
		body         any
		status       int
	}{
		{http.MethodPost, "/v2/callouts", map[string]any{"poolAddress": h.pool, "body": " ", "direction": "neutral", "disclosure": "organic", "clientCalloutId": uuid.NewString()}, 422},
		{http.MethodPost, "/v2/callouts", map[string]any{"poolAddress": h.pool, "body": "line\x00break", "direction": "neutral", "disclosure": "organic", "clientCalloutId": uuid.NewString()}, 422},
		{http.MethodPost, "/v2/bounties", map[string]any{"title": " ", "description": "Community artwork", "assetAddress": h.token, "budgetRaw": "100", "poolAddress": h.pool}, 422},
		{http.MethodPost, "/v2/bounties", map[string]any{"title": "Community artwork", "description": "line\x00break", "assetAddress": h.token, "budgetRaw": "100", "poolAddress": h.pool}, 422},
		{http.MethodPost, "/v2/bounties", map[string]any{"title": "Community artwork", "description": "Community artwork", "assetAddress": directAddr(0xdead), "budgetRaw": "100", "poolAddress": h.pool}, 422},
		{http.MethodPost, "/v2/bounties/" + missing + "/submissions", map[string]any{"body": "Community entry", "clientSubmissionId": uuid.NewString(), "evidenceUrls": []string{"https://example.org/entry"}}, 404},
		{http.MethodPost, "/v2/bounties/" + missing + "/submissions", map[string]any{"body": " ", "clientSubmissionId": uuid.NewString(), "evidenceUrls": []string{}}, 422},
		{http.MethodPost, "/v2/bounties/" + missing + "/submissions/" + uuid.NewString() + ":accept", map[string]any{"awardRaw": "1", "expectedBountyRevision": "1"}, 404},
		{http.MethodPost, "/v2/pools/" + h.pool + "/comments", map[string]any{"body": " ", "clientCommentId": uuid.NewString()}, 422},
		{http.MethodPatch, "/v2/comments/" + missing, map[string]any{"body": "Updated", "expectedRevision": "1"}, 404},
		{http.MethodDelete, "/v2/comments/" + missing, nil, 404},
		{http.MethodPut, "/v2/comments/" + missing + "/likes/me", nil, 404},
		{http.MethodDelete, "/v2/comments/" + missing + "/likes/me", nil, 204},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "profile", "targetId": "invalid", "reason": "spam", "evidence": "reviewed report"}, 422},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "token", "targetId": "invalid", "reason": "spam", "evidence": "reviewed report"}, 422},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "stream", "targetId": "invalid", "reason": "spam", "evidence": "reviewed report"}, 422},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "message", "targetId": missing, "reason": "spam", "evidence": "reviewed report"}, 404},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "comment", "targetId": missing, "reason": "spam", "evidence": "reviewed report"}, 404},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "profile", "targetId": actor, "reason": " ", "evidence": "reviewed report"}, 422},
		{http.MethodPost, "/v2/reports", map[string]any{"targetType": "profile", "targetId": actor, "reason": "spam", "evidence": "line\x00break"}, 422},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := h.do(tc.method, tc.path, actor, tc.body, nil)
			if response.status != tc.status {
				t.Fatalf("contract response = %d %s", response.status, response.body)
			}
			if tc.status == http.StatusNoContent && len(response.body) != 0 {
				t.Fatalf("idempotent like removal returned a body: %s", response.body)
			}
		})
	}
	for _, tc := range []struct {
		kind, target string
	}{
		{"profile", actor},
		{"token", h.token},
		{"stream", uuid.NewString()},
	} {
		response := h.do(http.MethodPost, "/v2/reports", actor, map[string]any{"targetType": tc.kind, "targetId": tc.target, "reason": "spam", "evidence": "reported content"}, map[string]string{"Idempotency-Key": uuid.NewString()})
		if response.status != http.StatusCreated || !strings.Contains(string(response.body), tc.target) {
			t.Fatalf("accepted %s report = %d %s", tc.kind, response.status, response.body)
		}
	}
}
