package social

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestSafetyRoutesValidateReviewAuthorityAndResourceIdentity(t *testing.T) {
	store := &safety.Store{DB: internaltest.NewPostgres(t)}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, nil)
	caseID, appealID := uuid.NewString(), uuid.NewString()
	wallet := "0x1111111111111111111111111111111111111111"
	principal := safety.Principal{Wallet: wallet, SessionID: "safety-route-boundary", Role: "room_moderator", Pools: []string{"pool-a"}, Expires: time.Now().Add(time.Hour), StepUp: time.Now()}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/v2/safety/reports", `{"targetType":"unknown","targetId":"target","category":"spam"}`, 400},
		{http.MethodPatch, "/v2/safety/cases/" + caseID, `{"status":"triaged","expected_revision":1,"reason":"reviewed","policy_version":"v2"}`, 404},
		{http.MethodPost, "/v2/safety/cases/" + caseID + "/bans", `{"wallet":"invalid","scope":"pool","resource":"pool-a","expectedRevision":1,"reason":"reviewed","policyVersion":"v2"}`, 400},
		{http.MethodPost, "/v2/safety/cases/" + caseID + "/restore", `{"caseId":"` + uuid.NewString() + `","expectedRevision":1,"reason":"reviewed","policyVersion":"v2"}`, 400},
		{http.MethodPost, "/v2/safety/appeals", `{"actionId":"invalid","reason":"reviewed"}`, 400},
		{http.MethodGet, "/v2/safety/appeals/" + appealID, ``, 404},
		{http.MethodPost, "/v2/safety/appeals/" + appealID + "/reverse-ban", `{"appealId":"` + uuid.NewString() + `","banId":"` + uuid.NewString() + `","expectedRevision":1,"expectedBanRevision":1,"reason":"reviewed","policyVersion":"v2"}`, 400},
		{http.MethodGet, "/internal/v2/safety/decision?wallet=" + wallet + "&scope=global", ``, 200},
		{http.MethodGet, "/internal/v2/safety/decision?wallet=" + wallet + "&scope=unknown", ``, 503},
		{http.MethodGet, "/internal/v2/safety/visibility?targetType=message&targetId=missing", ``, 404},
		{http.MethodGet, "/v2/voice-rooms/invalid", ``, 400},
		{http.MethodGet, "/v2/voice-rooms/invalid/prompt", ``, 400},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			request = request.WithContext(safety.WithPrincipal(request.Context(), principal))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("safety route = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
