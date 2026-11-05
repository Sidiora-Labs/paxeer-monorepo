package notifications

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestHTTPRuntimeRoutesFrozenNotificationPaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/v2/me/notifications"},
		{name: "read", method: http.MethodPut, path: "/v2/me/notifications/00000000-0000-7000-8000-000000000001/read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authenticated := false
			runtime := HTTPRuntime{Notifications: HTTP{
				Authenticate: func(*http.Request) (string, error) {
					authenticated = true
					return "", errors.New("authentication rejected")
				},
			}}
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()
			runtime.Handler().ServeHTTP(response, request)

			if !authenticated {
				t.Error("request did not reach notifications authentication")
			}
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
			}
			if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
				t.Fatalf("invalid frozen Error: %v: %s", err, response.Body.String())
			}
		})
	}
}
