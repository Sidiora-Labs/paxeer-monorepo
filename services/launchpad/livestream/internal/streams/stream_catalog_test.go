package streams

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestLivestreamRoutesCoverFrozenStreamCatalog(t *testing.T) {
	catalogPath := filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "packages", "sdk", "src", "operations.gen.ts")
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read frozen operation catalog: %v", err)
	}
	_, definitions, found := strings.Cut(string(source), "export const operationDefinitions = ")
	if !found {
		t.Fatal("frozen operation catalog has no operationDefinitions export")
	}
	var operations map[string]struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimSpace(definitions), " as const;")), &operations); err != nil {
		t.Fatalf("decode frozen operation catalog: %v", err)
	}

	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	mounted := make(map[string]bool)
	if err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		mounted[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("walk livestream routes: %v", err)
	}

	normalize := strings.NewReplacer("{streamId}", "{id}", "{operationId}", "{id}")
	selected := 0
	for name, operation := range operations {
		if name == "IF_STREAM_TICKET_ISSUE" {
			continue
		}
		if !strings.HasPrefix(operation.Path, "/v2/streams") && !strings.HasPrefix(operation.Path, "/v2/provider-webhooks/") {
			continue
		}
		selected++
		t.Run(name, func(t *testing.T) {
			route := operation.Method + " " + normalize.Replace(operation.Path)
			if !mounted[route] {
				t.Errorf("frozen operation %s is not mounted by RegisterRoutes: %s", name, route)
			}
		})
	}
	if selected != 8 {
		t.Errorf("selected %d frozen stream operations, want 8 excluding IF_STREAM_TICKET_ISSUE", selected)
	}
}
