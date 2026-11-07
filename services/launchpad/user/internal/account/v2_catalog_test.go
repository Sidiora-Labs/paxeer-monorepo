package account

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestUserHandlerServesFrozenUserCatalog(t *testing.T) {
	source, err := os.ReadFile("../../../../../sdk/launchpad/packages/sdk/src/operations.gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	_, catalog, ok := strings.Cut(string(source), "export const operationDefinitions = ")
	if !ok {
		t.Fatal("operationDefinitions declaration missing from frozen catalog")
	}
	var operations map[string]struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Public *bool  `json:"public"`
	}
	if err := json.NewDecoder(strings.NewReader(catalog)).Decode(&operations); err != nil {
		t.Fatalf("decode frozen operationDefinitions: %v", err)
	}
	routes := []struct {
		path  string
		exact bool
		count int
	}{
		{path: "/v2/profiles/{wallet}", exact: true, count: 2},
		{path: "/v2/profiles:batch", exact: true, count: 1},
		{path: "/v2/me/watchlist", count: 3},
		{path: "/v2/me/preferences", count: 2},
		{path: "/v2/me/devices", count: 2},
		{path: "/v2/me/exports", count: 2},
		{path: "/v2/me/deletion-requests", count: 2},
	}
	coverage := make(map[string]int)
	var selected []string
	for name, operation := range operations {
		for _, route := range routes {
			if operation.Path == route.path || (!route.exact && strings.HasPrefix(operation.Path, route.path)) {
				selected = append(selected, name)
				coverage[route.path]++
				break
			}
		}
	}
	for _, route := range routes {
		if coverage[route.path] != route.count {
			t.Errorf("frozen catalog contains %d operations for %s, want %d", coverage[route.path], route.path, route.count)
		}
	}
	if len(selected) != 14 {
		t.Errorf("frozen catalog selected %d user operations, want 14", len(selected))
	}
	if t.Failed() {
		t.FailNow()
	}
	sort.Strings(selected)

	s := fixture(t)
	owner(t, s, 42561)
	wallet := fmt.Sprintf("0x%040x", 42561)
	parameters := strings.NewReplacer(
		"{wallet}", wallet,
		"{poolAddress}", "0x0000000000000000000000000000000000000001",
		"{deviceId}", "00000000-0000-4000-8000-000000000001",
		"{jobId}", "00000000-0000-4000-8000-000000000002",
	)
	h := HTTP{Store: s}
	handler := h.Handler()
	for _, name := range selected {
		operation := operations[name]
		t.Run(name, func(t *testing.T) {
			if operation.Method == "" || operation.Public == nil {
				t.Fatal("catalog operation is missing method or public metadata")
			}
			path := parameters.Replace(operation.Path)
			if strings.ContainsAny(path, "{}") {
				t.Fatalf("unresolved catalog path parameter: %s", path)
			}
			if operation.Path == "/v2/profiles:batch" {
				path += "?" + url.Values{"wallets": {wallet}}.Encode()
			}
			request := httptest.NewRequest(operation.Method, path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
				t.Errorf("%s %s is not routed: status %d, body %s", operation.Method, path, response.Code, response.Body.String())
			}
			if !*operation.Public {
				if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
					t.Errorf("unauthenticated %s %s: status %d, want 401 or 403; body %s", operation.Method, path, response.Code, response.Body.String())
				}
				if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
					t.Errorf("unauthenticated %s %s returned an invalid frozen Error: %v; body %s", operation.Method, path, err, response.Body.String())
				}
			}
		})
	}
}
