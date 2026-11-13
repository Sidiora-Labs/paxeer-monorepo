package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestAccountingRoutesValidatePublicRequests(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	router := chi.NewRouter()
	h := AccountingV2Handler{SigningKey: []byte(strings.Repeat("k", 32))}
	RegisterAccountingV2(router, nil, h.SigningKey)
	for _, tc := range []struct {
		path   string
		status int
		code   string
	}{
		{"/v2/pools/" + pool + "/trades", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/v2/pools/" + pool + "/trades?side=invalid", 422, "FIELD_INVALID"},
		{"/v2/pools/" + pool + "/trades?cursor=invalid", 400, "CURSOR_INVALID"},
		{"/v2/pools/invalid/trades", 422, "FIELD_INVALID"},
		{"/v2/pools/" + pool + "/candles?interval=1m&from=2026-01-01T00:00:00.000Z&to=2026-01-01T01:00:00.000Z", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/v2/pools/" + pool + "/candles?interval=2m&from=2026-01-01T00:00:00.000Z&to=2026-01-01T01:00:00.000Z", 422, "FIELD_INVALID"},
	} {
		record := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		router.ServeHTTP(record, req)
		var response APIError
		if err := json.Unmarshal(record.Body.Bytes(), &response); err != nil || record.Code != tc.status || response.Code != tc.code {
			t.Errorf("%s: status=%d error=%+v parse=%v", tc.path, record.Code, response, err)
		}
	}
	if err := RegisterAccountingV2WithAdmission(nil, nil, h.SigningKey, nil, nil); err != store.ErrAccountingNotReady {
		t.Fatalf("missing admission accepted: %v", err)
	}
	record := httptest.NewRecorder()
	accountingError(record, 503, "DEPENDENCY_UNAVAILABLE", true)
	var response APIError
	if err := json.Unmarshal(record.Body.Bytes(), &response); err != nil || record.Code != 503 || !response.Retryable || record.Header().Get("Retry-After") != "1" {
		t.Fatalf("accounting error: %d %+v %v", record.Code, response, err)
	}
}
