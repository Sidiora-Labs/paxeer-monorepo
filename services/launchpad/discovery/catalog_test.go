package discovery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestDiscoveryHandlerServesFrozenDiscoveryCatalog(t *testing.T) {
	catalogPath := filepath.Join("..", "..", "..", "sdk", "launchpad", "packages", "sdk", "src", "operations.gen.ts")
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read frozen operation catalog: %v", err)
	}
	_, definitions, found := bytes.Cut(source, []byte("export const operationDefinitions = "))
	if !found {
		t.Fatal("frozen catalog has no operationDefinitions declaration")
	}
	var operations map[string]struct {
		Method        string `json:"method"`
		Path          string `json:"path"`
		Authenticated bool   `json:"authenticated"`
		Parameters    []struct {
			In   string `json:"in"`
			Name string `json:"name"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(bytes.NewReader(definitions)).Decode(&operations); err != nil {
		t.Fatalf("decode frozen operation definitions: %v", err)
	}
	var names []string
	authenticated := 0
	for name, operation := range operations {
		if operation.Path == "/v2/discovery/notifications" || strings.HasPrefix(operation.Path, "/v2/discovery/notifications/") {
			continue
		}
		if !strings.HasPrefix(operation.Path, "/v2/discovery/") && !strings.HasPrefix(operation.Path, "/v2/squads") {
			continue
		}
		names = append(names, name)
		if operation.Authenticated {
			authenticated++
		}
	}
	if len(names) != 15 || authenticated != 12 {
		t.Fatalf("frozen discovery catalog has %d operations (%d authenticated), want 15 (12 authenticated)", len(names), authenticated)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			operation := operations[name]
			if operation.Method == "" {
				t.Fatal("catalog operation has no method")
			}
			path := operation.Path
			for _, parameter := range operation.Parameters {
				if parameter.In == "path" {
					path = strings.ReplaceAll(path, "{"+parameter.Name+"}", "c9e42506-f847-4ec1-9c54-e320c98c1884")
				}
			}
			if strings.ContainsAny(path, "{}") {
				t.Fatalf("unresolved catalog path: %s", path)
			}
			authCalls := 0
			handler := (HTTP{Authenticate: func(*http.Request) (string, error) {
				authCalls++
				return "", ErrForbidden
			}}).Handler()
			request := httptest.NewRequest(operation.Method, path, nil)
			if !operation.Authenticated {
				request.Header.Set("Authorization", "Bearer invalid")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
				t.Fatalf("catalog route %s %s returned %d: %s", operation.Method, path, response.Code, response.Body.String())
			}
			if operation.Authenticated {
				if authCalls != 1 {
					t.Errorf("Authenticate called %d times, want 1", authCalls)
				}
				if response.Code != http.StatusUnauthorized {
					t.Errorf("unauthenticated %s %s returned %d, want 401: %s", operation.Method, path, response.Code, response.Body.String())
				}
				if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
					t.Errorf("response is not a frozen Error: %v: %s", err, response.Body.String())
				}
			}
		})
	}
}
