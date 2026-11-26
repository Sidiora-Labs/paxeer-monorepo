package cache

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAccountingUnavailableRecorded(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, tc := range []struct {
		name   string
		cause  error
		query  string
		status int
		code   string
	}{
		{"store failure", fmt.Errorf("read accounting page: %w", sql.ErrConnDone), "", 503, "DEPENDENCY_UNAVAILABLE"},
		{"store integrity", store.ErrDataIntegrity, "", 503, "INTEGRITY_QUARANTINED"},
		{"fields", rest.ErrAccountingFields, "", 422, "FIELD_INVALID"},
		{"cursor", store.ErrCursorInvalid, "", 400, "CURSOR_INVALID"},
		{"continuation", store.ErrAccountingNotReady, "?cursor=private-cursor", 400, "CURSOR_INVALID"},
		{"rate limit", &LimitError{RetryAfter: time.Second}, "", 429, "RATE_LIMITED"},
	} {
		for _, resource := range []string{"candles", "trades"} {
			t.Run(tc.name+"/"+resource, func(t *testing.T) {
				output.Reset()
				pattern := "/v2/pools/{poolAddress}/" + resource
				router := chi.NewRouter()
				router.Get(pattern, func(w http.ResponseWriter, r *http.Request) {
					accountingRuntimeError(w, r, tc.cause)
				})
				request := httptest.NewRequest(http.MethodGet, "/v2/pools/"+runtimeAdmissionPool+"/"+resource+tc.query, nil)
				request.Header.Set("Authorization", "Bearer private-authorization")
				request.Header.Set("Cookie", "private-cookie")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				var body rest.APIError
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != tc.status || body.Code != tc.code {
					t.Fatalf("response = %d %s, decode = %v", response.Code, response.Body, err)
				}
				if tc.status != http.StatusServiceUnavailable {
					if output.Len() != 0 {
						t.Fatalf("client error logged: %s", &output)
					}
					return
				}
				if !body.Retryable || body.RetryAfterMS == nil || *body.RetryAfterMS != 1000 || response.Header().Get("Retry-After") != "1" {
					t.Fatalf("retry metadata = %+v %v", body, response.Header())
				}
				if _, err := uuid.Parse(body.RequestID); err != nil || response.Header().Get("X-Request-ID") != body.RequestID {
					t.Fatalf("response request ID = %q, %v", body.RequestID, err)
				}
				var record map[string]any
				if err := json.Unmarshal(output.Bytes(), &record); err != nil {
					t.Fatalf("want one log record: %s, %v", &output, err)
				}
				if len(record) != 6 || record["level"] != "ERROR" || record["msg"] != "accounting request unavailable" || record["route"] != pattern || record["request_id"] != body.RequestID || record["error"] != tc.cause.Error() {
					t.Fatalf("log record = %s", &output)
				}
				for _, private := range []string{runtimeAdmissionPool, "private-cursor", "private-authorization", "private-cookie"} {
					if strings.Contains(output.String(), private) {
						t.Fatalf("request value logged: %s", &output)
					}
				}
				if strings.Contains(response.Body.String(), tc.cause.Error()) {
					t.Fatalf("store cause exposed: %s", response.Body)
				}
			})
		}
	}
}

func TestAccountingUnavailableRecordedRoutes(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	config := runtimeAdmissionConfig(t, 0)
	runtime := runtimeAdmissionRuntime(t, config)
	for _, cached := range []bool{true, false} {
		for _, resource := range []string{"candles", "trades"} {
			t.Run(fmt.Sprintf("runtime=%t/%s", cached, resource), func(t *testing.T) {
				output.Reset()
				router := chi.NewRouter()
				if cached {
					if err := runtime.Mount(router); err != nil {
						t.Fatal(err)
					}
				} else {
					rest.RegisterAccountingV2(router, runtime.bindings.config.Model, config.SigningKey)
				}
				pattern := "/v2/pools/{poolAddress}/" + resource
				path := "/v2/pools/" + runtimeAdmissionPool + "/" + resource
				if resource == "candles" {
					path += "?interval=1m&from=2026-10-12T00%3A00%3A00.000Z&to=2026-10-12T01%3A00%3A00.000Z"
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				var body rest.APIError
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusServiceUnavailable || body.Code != "DEPENDENCY_UNAVAILABLE" {
					t.Fatalf("response = %d %s, decode = %v", response.Code, response.Body, err)
				}
				var record struct {
					Level     string `json:"level"`
					Message   string `json:"msg"`
					Route     string `json:"route"`
					RequestID string `json:"request_id"`
					Error     string `json:"error"`
				}
				if err := json.Unmarshal(output.Bytes(), &record); err != nil || record.Level != "ERROR" || record.Message != "accounting request unavailable" || record.Route != pattern || record.RequestID != body.RequestID {
					t.Fatalf("log = %s, decode = %v", &output, err)
				}
				if cached && record.Error != store.ErrAccountingNotReady.Error() || !cached && record.Error != "closed pool" {
					t.Fatalf("cause = %q", record.Error)
				}
				if strings.Contains(output.String(), runtimeAdmissionPool) || strings.Contains(output.String(), "2026-10-12") || strings.Contains(response.Body.String(), record.Error) {
					t.Fatalf("request or cause exposed: log = %s, response = %s", &output, response.Body)
				}
			})
		}
	}
}
