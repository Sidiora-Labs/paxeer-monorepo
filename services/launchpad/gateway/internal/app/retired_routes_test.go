package app_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestRetiredConnectedTemplatesAnswerNotServed(t *testing.T) {
	g := bootDevIdentity(t, 1000)
	routes := proxy.RetiredConnectedRoutes()
	if len(routes) == 0 {
		t.Fatal("retired connected route catalog is empty")
	}
	for _, route := range routes {
		t.Run(route.Method+" "+route.Template, func(t *testing.T) {
			path := strings.NewReplacer("{assetId}", "0f6b6c52-3f0e-4c38-9a59-2b2b3f6f8c11", "{roomId}", "0f6b6c52-3f0e-4c38-9a59-2b2b3f6f8c11").Replace(route.Template)
			resp, raw := send(t, route.Method, g.base+path, nil, map[string]string{"Origin": devOrigin})
			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503: %s", resp.StatusCode, raw)
			}
			if resp.Header.Get("Content-Type") != "application/json" || resp.Header.Get("Cache-Control") != "private, no-store" || resp.Header.Get("Retry-After") != "" {
				t.Fatalf("unexpected not-served headers: %v", resp.Header)
			}
			if err := interfaces.ValidateJSON("Error", raw); err != nil {
				t.Fatalf("not a frozen Error: %v\n%s", err, raw)
			}
			var envelope auth.PublicError
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Code != "DEPENDENCY_UNAVAILABLE" || envelope.Retryable || envelope.RetryAfterMS != nil {
				t.Fatalf("unexpected not-served envelope: %s", raw)
			}
		})
	}
}
