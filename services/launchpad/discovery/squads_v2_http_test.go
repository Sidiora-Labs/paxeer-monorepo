package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestPostgresV2SquadCreateReadAndMembership(t *testing.T) {
	s, _ := realStore(t)
	owner, applicant, outsider, invited := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	h := HTTP{Store: s, Authenticate: func(r *http.Request) (string, error) {
		return r.Header.Get("X-Test-Account"), nil
	}}
	request := func(actor, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Test-Account", actor)
		r.Header.Set("X-Request-Id", discoveryErrorRequestID)
		w := httptest.NewRecorder()
		h.Handler().ServeHTTP(w, r)
		return w
	}
	assertResponse := func(w *httptest.ResponseRecorder, status int, schema string, dst any) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("%s: status = %d, want %d: %s", schema, w.Code, status, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s headers: %v", schema, w.Header())
		}
		if err := v2.ValidateJSON(schema, w.Body.Bytes()); err != nil {
			t.Fatalf("%s: %v: %s", schema, err, w.Body.String())
		}
		var envelope struct {
			RequestID   string  `json:"requestId"`
			GeneratedAt string  `json:"generatedAt"`
			Quality     string  `json:"quality"`
			Revision    *string `json:"revision"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		at, err := time.Parse("2006-01-02T15:04:05.000Z", envelope.GeneratedAt)
		if err != nil || time.Since(at) < 0 || time.Since(at) > time.Minute || envelope.RequestID != discoveryErrorRequestID || envelope.Quality != "complete" || envelope.Revision == nil {
			t.Fatalf("invalid envelope: %+v, %v", envelope, err)
		}
		if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
			t.Fatal(err)
		}
	}
	create := func(visibility, policy string) v2.Squad {
		t.Helper()
		body := fmt.Sprintf(`{"name":"Research","description":"Shared research","visibility":%q,"joinPolicy":%q}`, visibility, policy)
		var response v2.SquadCreateResponse
		assertResponse(request(owner, http.MethodPost, "/v2/squads", body), http.StatusCreated, "SquadCreateResponse", &response)
		q := response.Data.Squad
		id, err := uuid.Parse(q.SquadId)
		if err != nil || id.Version() != 4 || id.Variant() != uuid.RFC4122 {
			t.Fatalf("server squad ID = %q: %v", q.SquadId, err)
		}
		t.Cleanup(func() {
			for _, table := range []string{"transfers", "invitations", "members"} {
				if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.`+table+` WHERE squad_id=$1`, q.SquadId); err != nil {
					t.Error(err)
				}
			}
			if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.squads WHERE id=$1`, q.SquadId); err != nil {
				t.Error(err)
			}
		})
		want := v2.Squad{SquadId: q.SquadId, Name: "Research", Description: "Shared research", Visibility: visibility, JoinPolicy: policy, Revision: "1", MemberCount: "1"}
		if q != want || response.Revision == nil || *response.Revision != "1" {
			t.Fatalf("create = %+v, want %+v", response, want)
		}
		stored, err := (Store{s.DB}).Squad(context.Background(), q.SquadId)
		if err != nil || stored.OwnerAccountID != owner || stored.Description != q.Description || stored.Visibility != q.Visibility || stored.Policy != policy || stored.Revision != 1 {
			t.Fatalf("persisted squad = %+v: %v", stored, err)
		}
		return q
	}
	read := func(actor string, q v2.Squad, count string) {
		t.Helper()
		var response v2.SquadReadResponse
		assertResponse(request(actor, http.MethodGet, "/v2/squads/"+q.SquadId, ""), http.StatusOK, "SquadReadResponse", &response)
		q.MemberCount = v2.UInt256(count)
		if response.Data.Squad != q || response.Data.AllowedActions == nil || response.Revision == nil || *response.Revision != "1" {
			t.Fatalf("read = %+v, want %+v", response, q)
		}
	}
	q := create("private", "request")
	read(owner, q, "1")
	assertDiscoveryFrozenError(t, request(outsider, http.MethodGet, "/v2/squads/"+q.SquadId, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
	assertDiscoveryFrozenError(t, request(owner, http.MethodGet, "/v2/squads/"+uuid.NewString(), ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
	membership := func(actor, id, account, action, expected, state, role, revision string) {
		t.Helper()
		var response v2.SquadMembershipResponse
		path := "/v2/squads/" + id + "/members/" + account
		body := fmt.Sprintf(`{"action":%q,"expectedRevision":%q}`, action, expected)
		assertResponse(request(actor, http.MethodPut, path, body), http.StatusOK, "SquadMembershipResponse", &response)
		want := v2.SquadMembership{SquadId: id, AccountId: account, State: state, Role: role, Revision: v2.PositiveUInt256(revision)}
		if response.Data.Membership != want || response.Revision == nil || string(*response.Revision) != revision {
			t.Fatalf("%s response = %+v, want %+v", action, response, want)
		}
		m, err := h.squadMember(context.Background(), id, account)
		if err != nil || m.Status != state || fmt.Sprint(m.Revision) != revision || (action != "invite" && m.Role != role) {
			t.Fatalf("%s persisted membership = %+v: %v", action, m, err)
		}
	}
	path := "/v2/squads/" + q.SquadId + "/members/" + applicant
	assertDiscoveryFrozenError(t, request(outsider, http.MethodPut, path, `{"action":"request","expectedRevision":"0"}`), http.StatusForbidden, "SCOPE_DENIED", discoveryErrorRequestID)
	membership(applicant, q.SquadId, applicant, "request", "0", "requested", "member", "1")
	assertDiscoveryFrozenError(t, request(applicant, http.MethodGet, "/v2/squads/"+q.SquadId, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
	assertDiscoveryFrozenError(t, request(applicant, http.MethodPut, path, `{"action":"approve","expectedRevision":"1"}`), http.StatusForbidden, "SCOPE_DENIED", discoveryErrorRequestID)
	membership(owner, q.SquadId, applicant, "approve", "1", "active", "member", "2")
	read(applicant, q, "2")
	membership(owner, q.SquadId, applicant, "promote", "2", "active", "moderator", "3")
	assertDiscoveryFrozenError(t, request(owner, http.MethodPut, path, `{"action":"demote","expectedRevision":"2"}`), http.StatusConflict, "CONFLICT", discoveryErrorRequestID)
	m, err := h.squadMember(context.Background(), q.SquadId, applicant)
	if err != nil || m.Role != "moderator" || m.Revision != 3 {
		t.Fatalf("stale demotion changed membership: %+v, %v", m, err)
	}
	membership(owner, q.SquadId, applicant, "demote", "3", "active", "member", "4")
	membership(owner, q.SquadId, applicant, "remove", "4", "removed", "member", "5")
	read(owner, q, "1")
	assertDiscoveryFrozenError(t, request(applicant, http.MethodGet, "/v2/squads/"+q.SquadId, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
	membership(applicant, q.SquadId, applicant, "request", "5", "requested", "member", "6")
	membership(owner, q.SquadId, applicant, "approve", "6", "active", "member", "7")
	membership(owner, q.SquadId, applicant, "ban", "7", "banned", "member", "8")
	assertDiscoveryFrozenError(t, request(applicant, http.MethodGet, "/v2/squads/"+q.SquadId, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
	assertDiscoveryFrozenError(t, request(applicant, http.MethodPut, path, `{"action":"request","expectedRevision":"8"}`), http.StatusForbidden, "SCOPE_DENIED", discoveryErrorRequestID)
	public := create("public", "invite")
	if public.SquadId == q.SquadId {
		t.Fatal("server reused squad ID")
	}
	read(outsider, public, "1")
	membership(owner, public.SquadId, invited, "invite", "0", "invited", "member", "1")
	var body []byte
	var invitation Invitation
	if err := s.DB.QueryRow(`SELECT document FROM narrative_discovery.invitations WHERE squad_id=$1 AND account_id=$2`, public.SquadId, invited).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &invitation); err != nil {
		t.Fatal(err)
	}
	if invitation.AccountID != invited || invitation.SquadID != public.SquadId || invitation.Consumed || !invitation.ExpiresAt.After(time.Now()) || invitation.ExpiresAt.After(time.Now().Add(7*24*time.Hour)) || invitation.TokenHash == sha256.Sum256(nil) || invitation.TokenHash == [32]byte{} {
		t.Fatalf("invitation not persisted correctly: %+v", invitation)
	}
	read(outsider, public, "1")
	for _, body := range []string{
		`{"action":"invalid","expectedRevision":"8"}`,
		`{"action":"ban","expectedRevision":"8","role":"owner"}`,
		`{"action":"ban","expectedRevision":8}`,
		`{"action":"ban","expectedRevision":"18446744073709551616"}`,
		`{"action":"ban"}`,
	} {
		assertDiscoveryFrozenError(t, request(owner, http.MethodPut, path, body), http.StatusBadRequest, "INVALID_REQUEST", discoveryErrorRequestID)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v2/squads"},
		{http.MethodGet, "/v2/squads/" + q.SquadId},
		{http.MethodPut, path},
	} {
		assertDiscoveryFrozenError(t, request("", route.method, route.path, `{}`), http.StatusUnauthorized, "AUTH_REQUIRED", discoveryErrorRequestID)
	}
}

func TestV2SquadCreateRejectsUnknownFields(t *testing.T) {
	handler := (HTTP{Authenticate: func(*http.Request) (string, error) {
		return "ed084842-de8e-4909-88d9-a10237835afb", nil
	}}).Handler()
	for _, body := range []string{
		`{"name":"Research","description":"","visibility":"private","joinPolicy":"request","id":"fb496a71-90cd-4784-9b49-be00ddd67a71"}`,
		`{"name":"Research","description":"","visibility":"private","joinPolicy":"request","ownerAccountId":"ed084842-de8e-4909-88d9-a10237835afb"}`,
		`{"name":"Research","description":"","visibility":"private","joinPolicy":"request","unexpected":true}`,
		`{"name":"Research","visibility":"private","joinPolicy":"request"}`,
		`{"name":"Research","description":null,"visibility":"private","joinPolicy":"request"}`,
		`{"name":"Research","description":"","visibility":"private","joinPolicy":"request"} {}`,
		`{"name":`,
	} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v2/squads", strings.NewReader(body))
			r.Header.Set("X-Request-Id", discoveryErrorRequestID)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertDiscoveryFrozenError(t, w, http.StatusBadRequest, "INVALID_REQUEST", discoveryErrorRequestID)
		})
	}
}
