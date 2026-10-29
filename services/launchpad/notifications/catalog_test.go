package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestNotificationsHandlerServesFrozenNotificationCatalog(t *testing.T) {
	source, err := os.ReadFile("../../../sdk/launchpad/packages/sdk/src/operations.gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	_, definitions, ok := strings.Cut(string(source), "export const operationDefinitions = ")
	if !ok {
		t.Fatal("frozen operationDefinitions catalog not found")
	}
	var operations map[string]struct {
		Method string  `json:"method"`
		Path   string  `json:"path"`
		Body   *string `json:"body"`
	}
	if err := json.NewDecoder(strings.NewReader(definitions)).Decode(&operations); err != nil {
		t.Fatalf("decode frozen operation catalog: %v", err)
	}
	var names []string
	selected := make(map[string]bool)
	for name, operation := range operations {
		if strings.Contains(operation.Path, "/notifications") {
			names = append(names, name)
			selected[name] = true
		}
	}
	wantNames := []string{
		"IF_NOTIFICATIONS",
		"IF_NOTIFICATION_READ",
		"IF_CONNECTED_NOTIFICATION_LIST",
		"IF_CONNECTED_NOTIFICATION_READ",
		"IF_CONNECTED_NOTIFICATION_DELETE",
	}
	if len(names) != len(wantNames) {
		t.Errorf("catalog has %d notification operations, want %d", len(names), len(wantNames))
	}
	for _, name := range wantNames {
		if !selected[name] {
			t.Errorf("frozen notification operation %s is missing", name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			operation := operations[name]
			if operation.Method == "" {
				t.Fatal("catalog operation has no HTTP method")
			}
			path := strings.NewReplacer(
				"{notificationId}", "c9e42506-f847-4ec1-9c54-e320c98c1884",
				"{id}", strings.Repeat("a", 64),
			).Replace(operation.Path)
			if strings.ContainsAny(path, "{}") {
				t.Fatalf("unresolved catalog path parameter: %s", path)
			}
			request := httptest.NewRequest(operation.Method, path, nil)
			if operation.Body != nil {
				request = httptest.NewRequest(operation.Method, path, strings.NewReader(`{"read":true}`))
				request.Header.Set("Content-Type", "application/json")
			}
			authCalls := 0
			handler := HTTP{Authenticate: func(*http.Request) (string, error) {
				authCalls++
				return "", ErrForbidden
			}}.Handler()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
				t.Errorf("catalog route %s %s is not served: status %d", operation.Method, path, response.Code)
			}
			if response.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401: %s", response.Code, response.Body.String())
			}
			if authCalls != 1 {
				t.Errorf("Authenticate called %d times, want 1", authCalls)
			}
			if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
				t.Errorf("invalid frozen Error: %v: %s", err, response.Body.String())
			}
		})
	}
}
