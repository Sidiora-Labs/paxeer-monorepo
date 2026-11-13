package social

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types"
)

func TestModeratorBanRoutesUseVerifiedPrincipalAndRevision(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db}
	moderator := "0x1111111111111111111111111111111111111111"
	target := "0x2222222222222222222222222222222222222222"
	pool := "0x3333333333333333333333333333333333333333"
	reason := "spam"
	principal := safety.Principal{Wallet: moderator, SessionID: "moderation-route", Role: "room_moderator", Pools: []string{pool}, Expires: time.Now().Add(time.Hour)}
	ctx = safety.WithPrincipal(WithModeration(ctx, store), principal)
	h := New(Deps{})
	request := func(method, path string, body any) *http.Request {
		t.Helper()
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		return httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(ctx)
	}
	invalid := httptest.NewRecorder()
	h.adminCreateBan(invalid, request(http.MethodPost, "/admin/bans", map[string]any{"wallet": target, "pool_address": pool, "duration_seconds": 0}))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid duration = %d: %s", invalid.Code, invalid.Body.String())
	}
	created := httptest.NewRecorder()
	h.adminCreateBan(created, request(http.MethodPost, "/admin/bans", types.BanRequest{Wallet: target, PoolAddress: &pool, Reason: &reason, BannedBy: target}))
	if created.Code != http.StatusOK {
		t.Fatalf("create ban = %d: %s", created.Code, created.Body.String())
	}
	var answer types.BanResponse
	if err := json.Unmarshal(created.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if !answer.OK || answer.Ban.ID == "" || answer.Ban.Wallet != target || answer.Ban.BannedBy != moderator {
		t.Fatalf("ban response = %+v", answer)
	}
	listed := httptest.NewRecorder()
	h.adminListBans(listed, request(http.MethodGet, "/admin/bans?limit=10", nil))
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte(answer.Ban.ID)) {
		t.Fatalf("ban list = %d: %s", listed.Code, listed.Body.String())
	}
	path := "/admin/bans/" + answer.Ban.ID
	revoke := request(http.MethodDelete, path, map[string]any{"expected_revision": 1, "reason": "reviewed"})
	route := chi.NewRouteContext()
	route.URLParams.Add("banId", answer.Ban.ID)
	revoke = revoke.WithContext(context.WithValue(revoke.Context(), chi.RouteCtxKey, route))
	revoked := httptest.NewRecorder()
	h.adminDeleteBan(revoked, revoke)
	if revoked.Code != http.StatusOK {
		t.Fatalf("revoke ban = %d: %s", revoked.Code, revoked.Body.String())
	}
	var revision int64
	var isRevoked bool
	if err := db.QueryRow(ctx, `SELECT revision,revoked_at IS NOT NULL FROM safety_bans WHERE id=$1`, answer.Ban.ID).Scan(&revision, &isRevoked); err != nil {
		t.Fatal(err)
	}
	if revision != 2 || !isRevoked {
		t.Fatalf("revoked ban = revision %d, revoked %t", revision, isRevoked)
	}
	staleRequest := request(http.MethodDelete, path, map[string]any{"expected_revision": 1, "reason": "reviewed"})
	staleRequest = staleRequest.WithContext(context.WithValue(staleRequest.Context(), chi.RouteCtxKey, route))
	stale := httptest.NewRecorder()
	h.adminDeleteBan(stale, staleRequest)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale revocation = %d: %s", stale.Code, stale.Body.String())
	}
	caseID := uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'message','reviewed-message',$2,$3,$4,'spam','reported content')`, caseID, pool, target, moderator); err != nil {
		t.Fatal(err)
	}
	reports := httptest.NewRecorder()
	h.adminListReports(reports, request(http.MethodGet, "/admin/reports?status=pending&limit=10", nil))
	if reports.Code != http.StatusOK || !bytes.Contains(reports.Body.Bytes(), []byte(caseID)) {
		t.Fatalf("review queue = %d: %s", reports.Code, reports.Body.String())
	}
	resolve := request(http.MethodPatch, "/admin/reports/"+caseID, map[string]any{"status": "resolved", "expected_revision": 1, "reason": "reviewed", "policy_version": "review-v2"})
	caseRoute := chi.NewRouteContext()
	caseRoute.URLParams.Add("reportId", caseID)
	resolve = resolve.WithContext(context.WithValue(resolve.Context(), chi.RouteCtxKey, caseRoute))
	resolved := httptest.NewRecorder()
	h.adminResolveReport(resolved, resolve)
	if resolved.Code != http.StatusOK || !bytes.Contains(resolved.Body.Bytes(), []byte(`"revision":2`)) {
		t.Fatalf("resolved report = %d: %s", resolved.Code, resolved.Body.String())
	}
	var caseState string
	if err := db.QueryRow(ctx, `SELECT state FROM safety_cases WHERE id=$1`, caseID).Scan(&caseState); err != nil || caseState != "actioned" {
		t.Fatalf("committed review = %q: %v", caseState, err)
	}
	messageDelete := httptest.NewRecorder()
	h.adminDeleteMessage(messageDelete, request(http.MethodDelete, "/admin/messages/reviewed-message", nil))
	if messageDelete.Code != http.StatusServiceUnavailable {
		t.Fatalf("unversioned message removal = %d", messageDelete.Code)
	}
}

func TestModeratorRoutesValidateBodiesAndReportStoreAvailability(t *testing.T) {
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db}
	h := New(Deps{})
	moderator := "0x1111111111111111111111111111111111111111"
	target := "0x2222222222222222222222222222222222222222"
	principal := safety.Principal{Wallet: moderator, SessionID: "moderation-route", Role: "global_administrator", Expires: time.Now().Add(time.Hour), StepUp: time.Now()}
	ctx := safety.WithPrincipal(WithModeration(context.Background(), store), principal)
	request := func(method, path, body string, available bool) *http.Request {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if available {
			r = r.WithContext(ctx)
		}
		return r
	}
	assertStatus := func(handler func(http.ResponseWriter, *http.Request), r *http.Request, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", r.Method, r.URL.Path, w.Code, want, w.Body.String())
		}
	}
	assertStatus(h.report, request(http.MethodPost, "/reports", `{}`, false), http.StatusServiceUnavailable)
	assertStatus(h.adminCreateBan, request(http.MethodPost, "/admin/bans", `{}`, false), http.StatusServiceUnavailable)
	assertStatus(h.adminResolveReport, request(http.MethodPatch, "/admin/reports/unused", `{}`, false), http.StatusServiceUnavailable)
	assertStatus(h.adminDeleteBan, request(http.MethodDelete, "/admin/bans/unused", `{}`, false), http.StatusServiceUnavailable)
	assertStatus(h.adminListBans, request(http.MethodGet, "/admin/bans", "", false), http.StatusServiceUnavailable)
	assertStatus(h.adminListReports, request(http.MethodGet, "/admin/reports", "", false), http.StatusServiceUnavailable)
	assertStatus(h.report, request(http.MethodPost, "/reports", `{} {}`, true), http.StatusBadRequest)
	assertStatus(h.adminCreateBan, request(http.MethodPost, "/admin/bans", `{"wallet":"`+target+`","duration_seconds":31536001}`, true), http.StatusBadRequest)
	assertStatus(h.adminCreateBan, request(http.MethodPost, "/admin/bans", `{"wallet":"`+target+`","pool_address":"invalid"}`, true), http.StatusBadRequest)
	assertStatus(h.adminCreateBan, request(http.MethodPost, "/admin/bans", `{"wallet":"`+target+`"} {}`, true), http.StatusBadRequest)
	assertStatus(h.adminDeleteBan, request(http.MethodDelete, "/admin/bans/unused", `{ "unknown": true }`, true), http.StatusBadRequest)
	db.Close()
	assertStatus(h.report, request(http.MethodPost, "/reports", `{"message_id":"message","reason":"spam"}`, true), http.StatusServiceUnavailable)
	assertStatus(h.adminCreateBan, request(http.MethodPost, "/admin/bans", `{"wallet":"`+target+`","reason":"reviewed conduct"}`, true), http.StatusServiceUnavailable)
	assertStatus(h.adminListBans, request(http.MethodGet, "/admin/bans", "", true), http.StatusServiceUnavailable)
	assertStatus(h.adminListReports, request(http.MethodGet, "/admin/reports", "", true), http.StatusServiceUnavailable)
}
