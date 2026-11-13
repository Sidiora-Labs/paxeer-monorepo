package social

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestSafetyRoutesResolveAppealedPoolBan(t *testing.T) {
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, nil)
	const issuer = "0x1111111111111111111111111111111111111111"
	const affected = "0x2222222222222222222222222222222222222222"
	const reviewer = "0x3333333333333333333333333333333333333333"
	const pool = "0x4444444444444444444444444444444444444444"
	caseID := uuid.NewString()
	if _, err := db.Exec(context.Background(), `INSERT INTO safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'message','safety-route-message',$2,$3,$4,'abuse','reported message')`, caseID, pool, affected, issuer); err != nil {
		t.Fatal(err)
	}
	principal := func(wallet, role string) safety.Principal {
		return safety.Principal{Wallet: wallet, SessionID: "safety-route-session", Role: role, Pools: []string{pool}, Expires: time.Now().Add(time.Hour), StepUp: time.Now()}
	}
	request := func(actor safety.Principal, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req = req.WithContext(safety.WithPrincipal(req.Context(), actor))
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req)
		return rw
	}
	moderator := principal(issuer, "room_moderator")
	listed := request(moderator, http.MethodGet, "/v2/safety/cases?limit=10", nil)
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte(caseID)) {
		t.Fatalf("case listing = %d %s", listed.Code, listed.Body.String())
	}
	ban := request(moderator, http.MethodPost, "/v2/safety/cases/"+caseID+"/bans", map[string]any{"wallet": affected, "scope": "pool", "resource": pool, "expectedRevision": 1, "reason": "reviewed report", "policyVersion": "pool-v2"})
	if ban.Code != http.StatusOK {
		t.Fatalf("case ban = %d %s", ban.Code, ban.Body.String())
	}
	var result struct {
		BanID string `json:"banId"`
	}
	if err := json.Unmarshal(ban.Body.Bytes(), &result); err != nil || result.BanID == "" {
		t.Fatalf("ban response = %+v: %v", result, err)
	}
	decision := request(moderator, http.MethodGet, "/internal/v2/safety/decision?wallet="+affected+"&scope=pool&resource="+pool, nil)
	if decision.Code != http.StatusOK || !bytes.Contains(decision.Body.Bytes(), []byte(`"decision":"deny"`)) {
		t.Fatalf("active ban decision = %d %s", decision.Code, decision.Body.String())
	}
	var actionID string
	if err := db.QueryRow(context.Background(), `SELECT action_id FROM safety_action_bans WHERE ban_id=$1`, result.BanID).Scan(&actionID); err != nil {
		t.Fatal(err)
	}
	appeal := request(principal(affected, "participant"), http.MethodPost, "/v2/safety/appeals", map[string]string{"actionId": actionID, "reason": "review request"})
	if appeal.Code != http.StatusOK {
		t.Fatalf("appeal = %d %s", appeal.Code, appeal.Body.String())
	}
	var appealResult struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(appeal.Body.Bytes(), &appealResult); err != nil || appealResult.ID == "" {
		t.Fatalf("appeal response = %+v: %v", appealResult, err)
	}
	private := request(principal(affected, "participant"), http.MethodGet, "/v2/safety/appeals/"+appealResult.ID, nil)
	if private.Code != http.StatusOK || !bytes.Contains(private.Body.Bytes(), []byte(`"state":"submitted"`)) {
		t.Fatalf("private appeal = %d %s", private.Code, private.Body.String())
	}
	reverse := request(principal(reviewer, "room_moderator"), http.MethodPost, "/v2/safety/appeals/"+appealResult.ID+"/reverse-ban", safety.ReversalCommand{BanID: result.BanID, ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "appeal accepted", PolicyVersion: "pool-v2"})
	if reverse.Code != http.StatusOK {
		t.Fatalf("reversal = %d %s", reverse.Code, reverse.Body.String())
	}
	if got := store.Check(context.Background(), affected, "pool", pool); got != safety.Allow {
		t.Fatalf("reversed ban decision = %v", got)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO safety_visibility(target_type,target_id,revision,status,case_id) VALUES('message','safety-route-message',1,'removed',$1)`, caseID); err != nil {
		t.Fatal(err)
	}
	restored := request(principal(reviewer, "room_moderator"), http.MethodPost, "/v2/safety/cases/"+caseID+"/restore", safety.RestoreCommand{ExpectedRevision: 1, Reason: "appeal accepted", PolicyVersion: "pool-v2"})
	if restored.Code != http.StatusOK || !bytes.Contains(restored.Body.Bytes(), []byte(`"status":"restored"`)) {
		t.Fatalf("restored visibility = %d %s", restored.Code, restored.Body.String())
	}
	var visibilityStatus string
	var visibilityRevision int64
	if err := db.QueryRow(context.Background(), `SELECT status,revision FROM safety_visibility WHERE target_type='message' AND target_id='safety-route-message'`).Scan(&visibilityStatus, &visibilityRevision); err != nil || visibilityStatus != "restored" || visibilityRevision != 2 {
		t.Fatalf("stored visibility = %q revision %d: %v", visibilityStatus, visibilityRevision, err)
	}
}

func TestSafetyRoutesReportReviewAndVisibility(t *testing.T) {
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db, Targets: safety.VoiceTargets{DB: db}}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, nil)
	const reporter = "0x1111111111111111111111111111111111111111"
	const moderator = "0x2222222222222222222222222222222222222222"
	const pool = "0x3333333333333333333333333333333333333333"
	roomID := uuid.NewString()
	if _, err := db.Exec(context.Background(), `INSERT INTO voice_rooms(id,pool_id,host_wallet,start_key) VALUES($1,$2,$3,$4)`, roomID, pool, moderator, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO voice_participants(room_id,wallet) VALUES($1,$2)`, roomID, reporter); err != nil {
		t.Fatal(err)
	}
	targetID := roomID + ":" + reporter
	request := func(actor safety.Principal, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req.WithContext(safety.WithPrincipal(req.Context(), actor)))
		return rw
	}
	member := safety.Principal{Wallet: reporter, SessionID: "report-session", Role: "participant", Expires: time.Now().Add(time.Hour)}
	reviewer := safety.Principal{Wallet: moderator, SessionID: "review-session", Role: "room_moderator", Pools: []string{pool}, Expires: time.Now().Add(time.Hour), StepUp: time.Now()}
	report := request(member, http.MethodPost, "/v2/safety/reports", map[string]string{"targetType": "voice_participant", "targetId": targetID, "category": "spam", "description": "repeated spam"})
	if report.Code != http.StatusOK {
		t.Fatalf("report = %d %s", report.Code, report.Body.String())
	}
	var created safety.Case
	if err := json.Unmarshal(report.Body.Bytes(), &created); err != nil || created.ID == "" || created.Revision != 1 {
		t.Fatalf("created case = %+v: %v", created, err)
	}
	listed := request(reviewer, http.MethodGet, "/v2/safety/cases?limit=10", nil)
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte(created.ID)) {
		t.Fatalf("review queue = %d %s", listed.Code, listed.Body.String())
	}
	transition := request(reviewer, http.MethodPatch, "/v2/safety/cases/"+created.ID, map[string]any{"status": "dismissed", "expected_revision": 1, "reason": "reviewed", "policy_version": "review-v2"})
	if transition.Code != http.StatusOK || !bytes.Contains(transition.Body.Bytes(), []byte(`"revision":2`)) {
		t.Fatalf("review transition = %d %s", transition.Code, transition.Body.String())
	}
	var state string
	var revision int64
	if err := db.QueryRow(context.Background(), `SELECT state,revision FROM safety_cases WHERE id=$1`, created.ID).Scan(&state, &revision); err != nil || state != "dismissed" || revision != 2 {
		t.Fatalf("committed review = %q revision %d: %v", state, revision, err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO safety_visibility(target_type,target_id,revision,status,case_id) VALUES('voice_participant',$1,1,'restored',$2)`, targetID, created.ID); err != nil {
		t.Fatal(err)
	}
	visibility := request(member, http.MethodGet, "/internal/v2/safety/visibility?targetType=voice_participant&targetId="+targetID, nil)
	if visibility.Code != http.StatusOK || !bytes.Contains(visibility.Body.Bytes(), []byte(`"status":"restored"`)) {
		t.Fatalf("message visibility = %d %s", visibility.Code, visibility.Body.String())
	}
}
