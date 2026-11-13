package devsocial

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/google/uuid"
)

func TestBountyAndSubmissionPagesPreserveCommittedOrder(t *testing.T) {
	h := newDirectRouteHarness(t)
	creator, firstAuthor, secondAuthor := directAddr(0xa11ce), directAddr(0xb0b), directAddr(0xc0b)
	create := func(title string) string {
		t.Helper()
		body := map[string]any{"title": title, "description": "Community artwork", "assetAddress": strings.ToLower(deployment.SettlementAddress), "budgetRaw": "100", "poolAddress": h.pool}
		data := directData(h.expect(h.do(http.MethodPost, "/v2/bounties", creator, body, map[string]string{"Idempotency-Key": uuid.NewString()}), http.StatusCreated, "BountyCreateResponse"))
		return data["bounty"].(map[string]any)["bountyId"].(string)
	}
	first, second := create("First artwork"), create("Second artwork")
	firstPage := h.expect(h.do(http.MethodGet, "/v2/bounties?limit=1", creator, nil, nil), http.StatusOK, "BountyListResponse")
	if len(directItems(firstPage)) != 1 || firstPage["nextCursor"] == nil || directItems(firstPage)[0].(map[string]any)["bountyId"] != second {
		t.Fatalf("first bounty page = %v", firstPage)
	}
	secondPage := h.expect(h.do(http.MethodGet, "/v2/bounties?limit=1&cursor="+firstPage["nextCursor"].(string), creator, nil, nil), http.StatusOK, "BountyListResponse")
	if len(directItems(secondPage)) != 1 || secondPage["nextCursor"] != nil || directItems(secondPage)[0].(map[string]any)["bountyId"] != first {
		t.Fatalf("second bounty page = %v", secondPage)
	}
	if _, err := h.db.Exec(context.Background(), `UPDATE chat.v2_bounties SET state='open' WHERE bounty_id=$1`, first); err != nil {
		t.Fatal(err)
	}
	submit := func(actor, body string) string {
		t.Helper()
		data := directData(h.expect(h.do(http.MethodPost, "/v2/bounties/"+first+"/submissions", actor, map[string]any{"body": body, "clientSubmissionId": uuid.NewString(), "evidenceUrls": []string{"https://example.org/artwork"}}, nil), http.StatusCreated, "BountySubmitResponse"))
		return data["submission"].(map[string]any)["submissionId"].(string)
	}
	firstSubmission, secondSubmission := submit(firstAuthor, "First entry"), submit(secondAuthor, "Second entry")
	firstPage = h.expect(h.do(http.MethodGet, "/v2/bounties/"+first+"/submissions?limit=1", "", nil, nil), http.StatusOK, "BountySubmissionsResponse")
	if len(directItems(firstPage)) != 1 || firstPage["nextCursor"] == nil || directItems(firstPage)[0].(map[string]any)["submissionId"] != secondSubmission {
		t.Fatalf("first submission page = %v", firstPage)
	}
	secondPage = h.expect(h.do(http.MethodGet, "/v2/bounties/"+first+"/submissions?limit=1&cursor="+firstPage["nextCursor"].(string), "", nil, nil), http.StatusOK, "BountySubmissionsResponse")
	if len(directItems(secondPage)) != 1 || secondPage["nextCursor"] != nil || directItems(secondPage)[0].(map[string]any)["submissionId"] != firstSubmission {
		t.Fatalf("second submission page = %v", secondPage)
	}
}
