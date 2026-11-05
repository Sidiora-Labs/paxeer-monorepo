package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestReplayGatewayMountsFrozenTopicSnapshotRead(t *testing.T) {
	catalogPath := filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "packages", "sdk", "src", "operations.gen.ts")
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read frozen operation catalog: %v", err)
	}
	_, definitions, ok := bytes.Cut(source, []byte("export const operationDefinitions = "))
	if !ok {
		t.Fatal("frozen catalog has no operationDefinitions export")
	}
	definitions, _, ok = bytes.Cut(definitions, []byte(" as const;"))
	if !ok {
		t.Fatal("frozen operationDefinitions has no as const terminator")
	}
	var operations map[string]struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(definitions, &operations); err != nil {
		t.Fatalf("decode frozen operationDefinitions: %v", err)
	}
	snapshot, ok := operations["IF_TOPIC_SNAPSHOT_READ"]
	if !ok {
		t.Fatal("frozen catalog is missing IF_TOPIC_SNAPSHOT_READ")
	}
	if snapshot.Method != http.MethodGet || snapshot.Path != "/v2/snapshots" {
		t.Fatalf("IF_TOPIC_SNAPSHOT_READ = %s %s, want GET /v2/snapshots", snapshot.Method, snapshot.Path)
	}

	router := chi.NewRouter()
	if err := RegisterReplayGateway(router, "http://127.0.0.1:18080"); err != nil {
		t.Fatalf("register replay gateway: %v", err)
	}
	var snapshotMethods []string
	if err := chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if route == snapshot.Path {
			snapshotMethods = append(snapshotMethods, method)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk replay gateway routes: %v", err)
	}
	if len(snapshotMethods) != 1 || snapshotMethods[0] != snapshot.Method {
		t.Fatalf("replay gateway methods for %s = %v, want only %s from IF_TOPIC_SNAPSHOT_READ", snapshot.Path, snapshotMethods, snapshot.Method)
	}
}
