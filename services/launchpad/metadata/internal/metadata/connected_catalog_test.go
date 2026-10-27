package metadata

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestConnectedMediaRoutesCoverFrozenMetadataCatalog(t *testing.T) {
	catalogPath := filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "packages", "sdk", "src", "operations.gen.ts")
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read frozen operation catalog: %v", err)
	}
	_, definitions, found := bytes.Cut(source, []byte("export const operationDefinitions = "))
	if !found {
		t.Fatal("frozen catalog has no operationDefinitions declaration")
	}
	var operations map[string]struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := json.NewDecoder(bytes.NewReader(definitions)).Decode(&operations); err != nil {
		t.Fatalf("decode frozen operation definitions: %v", err)
	}
	type route struct {
		method string
		path   string
	}
	catalog := make(map[route]string)
	for name, operation := range operations {
		if !strings.HasPrefix(operation.Path, "/v2/tokens/{tokenAddress}/") && !strings.HasPrefix(operation.Path, "/v2/accounts/{wallet}/assets/") {
			continue
		}
		if operation.Method == "" {
			t.Fatalf("catalog operation %s has no method", name)
		}
		key := route{method: operation.Method, path: operation.Path}
		if previous, exists := catalog[key]; exists {
			t.Fatalf("catalog operations %s and %s duplicate %s %s", previous, name, key.method, key.path)
		}
		catalog[key] = name
	}
	if len(catalog) == 0 {
		t.Fatal("frozen catalog contains no connected token or account asset operations")
	}

	router := chi.NewRouter()
	var connected ConnectedMedia
	connected.RegisterRoutes(router)
	mounted := make(map[route]bool)
	if err := chi.Walk(router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(path, "/v2/") {
			key := route{method: method, path: path}
			mounted[key] = true
			if _, exists := catalog[key]; !exists {
				t.Errorf("mounted route %s %s has no connected catalog operation", method, path)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("walk connected media routes: %v", err)
	}
	for key, name := range catalog {
		if !mounted[key] {
			t.Errorf("catalog operation %s is not mounted: %s %s", name, key.method, key.path)
		}
	}
}
