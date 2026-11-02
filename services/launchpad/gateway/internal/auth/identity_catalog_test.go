package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func frozenIdentityRouter(t *testing.T) *chi.Mux {
	t.Helper()
	t.Setenv("IDENTITY_CATALOG_TEST_KEY", strings.Repeat("s", 48))
	now := time.Now()
	tokens, err := NewIdentityTokens("https://kindle.test", "kindle-owner-v2", "catalog", []SigningKey{{
		ID: "catalog", SecretReference: "IDENTITY_CATALOG_TEST_KEY",
		ActiveAt: now.Add(-time.Minute), RetireAt: now.Add(time.Hour),
	}}, EnvironmentIdentitySecrets{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	identity := &Identity{tokens: tokens, now: time.Now, origins: map[string]bool{"https://kindle.test": true}}
	router := chi.NewRouter()
	if err := identity.RegisterV2Routes(router, IdentityHTTPConfig{ServiceToken: strings.Repeat("t", 32)}); err != nil {
		t.Fatal(err)
	}
	return router
}

func TestIdentityV2RoutesCoverFrozenIdentityCatalog(t *testing.T) {
	router := frozenIdentityRouter(t)
	routes := make(map[string]bool)
	if err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	catalogPath := filepath.Join("../../../../..", "sdk/launchpad/packages/sdk/src/operations.gen.ts")
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	_, definitions, ok := strings.Cut(string(source), "export const operationDefinitions = ")
	if !ok {
		t.Fatal("frozen SDK operationDefinitions declaration is missing")
	}
	var operations map[string]struct {
		Service string `json:"service"`
		Method  string `json:"method"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(strings.NewReader(definitions)).Decode(&operations); err != nil {
		t.Fatalf("decode frozen SDK operationDefinitions: %v", err)
	}

	identityOperations := 0
	for name, operation := range operations {
		if operation.Service != "identity" {
			continue
		}
		identityOperations++
		t.Run(name, func(t *testing.T) {
			route := operation.Method + " " + operation.Path
			if !routes[route] {
				t.Errorf("frozen identity operation %s has no registered route: %s", name, route)
			}
		})
	}
	if identityOperations != 6 {
		t.Fatalf("frozen catalog has %d identity operations, want 6", identityOperations)
	}
}

func TestIdentityChallengeErrorMatchesFrozenErrorSchema(t *testing.T) {
	router := frozenIdentityRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/v2/auth/challenges", nil)
	request.Header.Set("Origin", "https://kindle.test")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty challenge body: status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
		t.Fatalf("challenge response violates frozen v2 Error schema: %v; body = %s", err, response.Body.String())
	}
	var envelope v2.Error
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != "FIELD_INVALID" {
		t.Errorf("empty challenge body: error code = %q, want FIELD_INVALID", envelope.Code)
	}
	if envelope.RequestId != response.Header().Get("X-Request-Id") {
		t.Errorf("error request ID %q differs from response header %q", envelope.RequestId, response.Header().Get("X-Request-Id"))
	}
}
