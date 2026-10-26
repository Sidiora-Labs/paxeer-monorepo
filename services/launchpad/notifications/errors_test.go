package notifications

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

const notificationErrorRequestID = "c9e42506-f847-4ec1-9c54-e320c98c1884"

func assertNotificationFrozenError(t *testing.T, w *httptest.ResponseRecorder, status int, code, requestID string) string {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want private, no-store", got)
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

func TestNotificationErrorsUseFrozenEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"missing", sql.ErrNoRows, http.StatusNotFound, "NOT_FOUND"},
		{"invalid", ErrInvalid, http.StatusBadRequest, "INVALID_REQUEST"},
		{"forbidden", ErrForbidden, http.StatusForbidden, "SCOPE_DENIED"},
		{"unavailable", ErrUnavailable, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"},
		{"unknown", errors.New("private dependency failure"), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, wrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("wrapped=%t", wrapped), func(t *testing.T) {
					err := tc.err
					if wrapped {
						err = fmt.Errorf("private operation: %w", err)
					}
					w := httptest.NewRecorder()
					respond(w, nil, err)
					assertNotificationFrozenError(t, w, tc.status, tc.code, "")
				})
			}
		})
	}

	t.Run("request IDs", func(t *testing.T) {
		for _, header := range []string{notificationErrorRequestID, "", "not-a-uuid"} {
			t.Run(fmt.Sprintf("header=%q", header), func(t *testing.T) {
				r := httptest.NewRequest(http.MethodGet, "/v2/discovery/notifications", nil)
				r.Header.Set("X-Request-Id", header)
				w := httptest.NewRecorder()
				writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
				wantID := ""
				if header == notificationErrorRequestID {
					wantID = header
				}
				assertNotificationFrozenError(t, w, http.StatusUnauthorized, "AUTH_REQUIRED", wantID)
			})
		}
		first, second := httptest.NewRecorder(), httptest.NewRecorder()
		respond(first, nil, ErrUnavailable)
		respond(second, nil, ErrUnavailable)
		firstID := assertNotificationFrozenError(t, first, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "")
		secondID := assertNotificationFrozenError(t, second, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "")
		if firstID == secondID {
			t.Error("separate errors reused a generated requestId")
		}
	})

	t.Run("unauthenticated routes", func(t *testing.T) {
		for _, auth := range []Authenticate{nil, (HTTP{}).actor} {
			for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
				path := "/v2/discovery/notifications"
				if method != http.MethodGet {
					path += "/notification-id"
				}
				r := httptest.NewRequest(method, path, nil)
				r.Header.Set("X-Request-Id", notificationErrorRequestID)
				w := httptest.NewRecorder()
				HTTP{Authenticate: auth}.Handler().ServeHTTP(w, r)
				assertNotificationFrozenError(t, w, http.StatusUnauthorized, "AUTH_REQUIRED", notificationErrorRequestID)
			}
		}
	})

	t.Run("internal failures", func(t *testing.T) {
		accountPath := "/internal/v2/accounts/" + notificationErrorRequestID + "/notification-data?deletionId=" + notificationErrorRequestID
		for _, tc := range []struct {
			name, method, path, body string
			h                        HTTP
			status                   int
			code                     string
		}{
			{"invalid account", http.MethodDelete, "/internal/v2/accounts/invalid/notification-data", "", HTTP{}, http.StatusBadRequest, "INVALID_REQUEST"},
			{"invalid deletion", http.MethodDelete, "/internal/v2/accounts/" + notificationErrorRequestID + "/notification-data?deletionId=invalid", "", HTTP{}, http.StatusBadRequest, "INVALID_REQUEST"},
			{"delete unavailable", http.MethodDelete, accountPath, "", HTTP{}, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"},
			{"delete forbidden", http.MethodDelete, accountPath, "", HTTP{InternalAuthorize: (InternalPolicy{}).Authorize}, http.StatusForbidden, "SCOPE_DENIED"},
			{"invalid chat JSON", http.MethodPost, "/internal/v2/notifications/chat", "{", HTTP{}, http.StatusBadRequest, "INVALID_REQUEST"},
			{"unknown chat field", http.MethodPost, "/internal/v2/notifications/chat", `{"unexpected":true}`, HTTP{}, http.StatusBadRequest, "INVALID_REQUEST"},
			{"trailing chat JSON", http.MethodPost, "/internal/v2/notifications/chat", "{} {}", HTTP{}, http.StatusBadRequest, "INVALID_REQUEST"},
			{"oversized chat JSON", http.MethodPost, "/internal/v2/notifications/chat", `{"EventID":"` + strings.Repeat("x", 4096) + `"}`, HTTP{}, http.StatusBadRequest, "INVALID_REQUEST"},
			{"chat unavailable", http.MethodPost, "/internal/v2/notifications/chat", "{}", HTTP{}, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"},
			{"chat forbidden", http.MethodPost, "/internal/v2/notifications/chat", "{}", HTTP{InternalAuthorize: (InternalPolicy{}).Authorize}, http.StatusForbidden, "SCOPE_DENIED"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				w := httptest.NewRecorder()
				tc.h.InternalHandler().ServeHTTP(w, r)
				assertNotificationFrozenError(t, w, tc.status, tc.code, "")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		respond(w, map[string]bool{"read": true}, nil)
		if w.Code != http.StatusOK || w.Body.String() != "{\"read\":true}\n" {
			t.Fatalf("success response = %d %s", w.Code, w.Body.String())
		}
	})
}
