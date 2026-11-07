package discovery

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

const discoveryErrorRequestID = "c9e42506-f847-4ec1-9c54-e320c98c1884"

func assertDiscoveryFrozenError(t *testing.T, w *httptest.ResponseRecorder, status int, code, requestID string) string {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if err := v2.ValidateJSON("Error", w.Body.Bytes()); err != nil {
		t.Fatalf("invalid frozen Error: %v: %s", err, w.Body.String())
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 {
		t.Errorf("envelope has %d fields, want 7", len(fields))
	}
	for name, want := range map[string]string{"retryAfterMs": "null", "fieldErrors": "[]", "details": "null"} {
		if got := string(fields[name]); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	var envelope v2.Error
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != code {
		t.Errorf("code = %q, want %q", envelope.Code, code)
	}
	if envelope.Message != http.StatusText(status) {
		t.Errorf("message = %q, want %q", envelope.Message, http.StatusText(status))
	}
	if envelope.Retryable != (status == http.StatusServiceUnavailable) {
		t.Errorf("retryable = %t for status %d", envelope.Retryable, status)
	}
	if requestID != "" {
		if envelope.RequestId != requestID {
			t.Errorf("requestId = %q, want %q", envelope.RequestId, requestID)
		}
	} else {
		id, err := uuid.Parse(envelope.RequestId)
		if err != nil {
			t.Fatalf("generated requestId is not a UUID: %v", err)
		}
		if id.Version() != 4 || id.Variant() != uuid.RFC4122 {
			t.Errorf("generated requestId = %q, want RFC 4122 UUID v4", envelope.RequestId)
		}
	}
	return envelope.RequestId
}

func TestDiscoveryErrorsUseFrozenEnvelope(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid", ErrInvalid, http.StatusBadRequest, "INVALID_REQUEST"},
		{"forbidden", ErrForbidden, http.StatusForbidden, "SCOPE_DENIED"},
		{"conflict", ErrConflict, http.StatusConflict, "CONFLICT"},
		{"missing", sql.ErrNoRows, http.StatusNotFound, "NOT_FOUND"},
		{"unknown", errors.New("private dependency failure"), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, wrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("wrapped=%t", wrapped), func(t *testing.T) {
					err := tc.err
					if wrapped {
						err = fmt.Errorf("discovery operation: %w", err)
					}
					for _, requestID := range []string{discoveryErrorRequestID, ""} {
						r := httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil)
						r.Header.Set("X-Request-Id", requestID)
						w := httptest.NewRecorder()
						reply(w, r, nil, err)
						assertDiscoveryFrozenError(t, w, tc.status, tc.code, requestID)
					}
				})
			}
		})
	}
	t.Run("fresh request IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for range 2 {
			w := httptest.NewRecorder()
			reply(w, httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil), nil, ErrUnavailable)
			id := assertDiscoveryFrozenError(t, w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "")
			if ids[id] {
				t.Fatal("request ID reused across requests")
			}
			ids[id] = true
		}
	})
	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		reply(w, httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil), map[string]bool{"ok": true}, nil)
		if w.Code != http.StatusOK || w.Body.String() != "{\"ok\":true}\n" {
			t.Fatalf("success response changed: %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("handler validation", func(t *testing.T) {
		handler := (HTTP{}).Handler()
		for _, path := range []string{
			"/v2/discovery/feed?limit=invalid",
			"/v2/discovery/leaderboards?start=invalid",
			"/v2/discovery/leaderboards?start=2026-01-01T00:00:00Z&end=invalid",
		} {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.Header.Set("X-Request-Id", discoveryErrorRequestID)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertDiscoveryFrozenError(t, w, http.StatusBadRequest, "INVALID_REQUEST", discoveryErrorRequestID)
		}
	})
}

func TestDiscoveryUnauthenticatedWritesAnswerFrozenError(t *testing.T) {
	handler := (HTTP{}).Handler()
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v2/discovery/callouts"},
		{http.MethodPatch, "/v2/discovery/callouts/callout"},
		{http.MethodPut, "/v2/discovery/alerts/alert"},
		{http.MethodPost, "/v2/discovery/squads"},
		{http.MethodPatch, "/v2/discovery/squads/squad"},
		{http.MethodPatch, "/v2/discovery/squads/squad/members/account"},
		{http.MethodPost, "/v2/discovery/squads/squad/invitations"},
		{http.MethodPost, "/v2/discovery/squads/squad/accept"},
		{http.MethodPost, "/v2/discovery/squads/squad/transfer"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			for _, requestID := range []string{discoveryErrorRequestID, ""} {
				r := httptest.NewRequest(route.method, route.path, nil)
				r.Header.Set("X-Request-Id", requestID)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				assertDiscoveryFrozenError(t, w, http.StatusUnauthorized, "AUTH_REQUIRED", requestID)
			}
		})
	}
	t.Run("authenticated read rejected", func(t *testing.T) {
		for _, path := range []string{"/v2/discovery/feed", "/v2/discovery/feed/entry"} {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.Header.Set("Authorization", "Bearer invalid")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertDiscoveryFrozenError(t, w, http.StatusUnauthorized, "AUTH_REQUIRED", "")
		}
	})
}
