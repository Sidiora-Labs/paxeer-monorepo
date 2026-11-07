package social

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/go-chi/chi/v5"
)

func TestSafetyRoutesCoverFrozenVoiceCatalog(t *testing.T) {
	source, err := os.ReadFile("../../../../../sdk/launchpad/packages/sdk/src/operations.gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	_, definitions, ok := strings.Cut(string(source), "export const operationDefinitions = ")
	if !ok {
		t.Fatal("frozen operationDefinitions export not found")
	}
	definitions = strings.TrimSpace(definitions)
	if !strings.HasSuffix(definitions, " as const;") {
		t.Fatal("frozen operationDefinitions terminator not found")
	}
	type route struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	var catalog map[string]route
	if err := json.Unmarshal([]byte(strings.TrimSuffix(definitions, " as const;")), &catalog); err != nil {
		t.Fatalf("decode frozen operationDefinitions: %v", err)
	}

	router := chi.NewRouter()
	var control voice.Control
	RegisterSafetyRoutes(router, nil, &control)
	mounted := make(map[route]bool)
	if err := chi.Walk(router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		mounted[route{Method: method, Path: path}] = true
		return nil
	}); err != nil {
		t.Fatalf("walk safety routes: %v", err)
	}

	var operations []string
	for _, prefix := range []string{"IF_VOICE_", "IF_CONNECTED_VOICE_"} {
		count := 0
		for name := range catalog {
			if strings.HasPrefix(name, prefix) {
				operations = append(operations, name)
				count++
			}
		}
		if count == 0 {
			t.Fatalf("frozen catalog contains no %s operations", prefix)
		}
	}
	sort.Strings(operations)
	for _, name := range operations {
		t.Run(name, func(t *testing.T) {
			want := catalog[name]
			if want.Method == "" || !strings.HasPrefix(want.Path, "/") {
				t.Fatalf("invalid frozen route: %+v", want)
			}
			if !mounted[want] {
				t.Errorf("RegisterSafetyRoutes does not mount %s %s", want.Method, want.Path)
			}
		})
	}
}
