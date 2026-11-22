package broker

import (
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestStreamHTTPErrorConsumerEnvelope(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{{401, "AUTH_REQUIRED"}, {403, "FORBIDDEN"}, {422, "FIELD_INVALID"}, {503, "DEPENDENCY_UNAVAILABLE"}} {
		for _, credential := range []string{"", "Authorization", "Cookie"} {
			req := httptest.NewRequest(http.MethodGet, "/v2/stream", nil)
			if credential != "" {
				req.Header.Set(credential, "credential")
			}
			res := httptest.NewRecorder()
			res.Header().Set("X-Request-ID", "invalid")
			WriteStreamHTTPError(res, req, tc.status, tc.code)
			failure, err := v2.DecodeError(res.Body.Bytes())
			if err != nil || res.Code != tc.status || failure.Code != tc.code || failure.RequestId != res.Header().Get("X-Request-ID") || failure.Message == "" || failure.Details != nil || failure.FieldErrors == nil || len(failure.FieldErrors) != 0 {
				t.Fatalf("stream failure: %d %s %v: %v", res.Code, res.Body, res.Header(), err)
			}
			if _, err := uuid.Parse(failure.RequestId); err != nil {
				t.Fatal(err)
			}
			if failure.Retryable != (tc.status == 503) || (failure.RetryAfterMs != nil) != failure.Retryable {
				t.Fatalf("retry metadata: %+v", failure)
			}
			if failure.Retryable {
				if *failure.RetryAfterMs != 1000 || res.Header().Get("Retry-After") != "1" {
					t.Fatalf("dependency retry: %+v %v", failure, res.Header())
				}
			} else if res.Header().Get("Retry-After") != "" {
				t.Fatal("nonretryable stream failure carries Retry-After")
			}
			cache := "no-store"
			if credential != "" {
				cache = "private, no-store"
			}
			if res.Header().Get("Cache-Control") != cache || res.Header().Get("X-Schema-Version") != "2" || res.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("stream response metadata: %v", res.Header())
			}
		}
	}
}
