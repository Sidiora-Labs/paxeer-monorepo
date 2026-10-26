package internaltest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type FrozenOperation struct {
	ID            string
	Method        string `json:"method"`
	Path          string `json:"path"`
	Service       string `json:"service"`
	Body          string `json:"body"`
	Response      string `json:"response"`
	Authenticated bool   `json:"authenticated"`
}

func FrozenV2Operations(t *testing.T) []FrozenOperation {
	t.Helper()
	path := filepath.Join(RepositoryRoot(t), "sdk", "launchpad", "packages", "sdk", "src", "operations.gen.ts")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const declaration = "export const operationDefinitions = "
	_, object, ok := strings.Cut(string(raw), declaration+"{")
	if !ok {
		t.Fatalf("%s: operationDefinitions declaration absent", path)
	}
	object, _, ok = strings.Cut(object, "\n} as const;")
	if !ok {
		t.Fatalf("%s: operationDefinitions terminator absent", path)
	}
	var catalog map[string]FrozenOperation
	if err := json.Unmarshal([]byte("{"+object+"\n}"), &catalog); err != nil {
		t.Fatalf("%s: decode operationDefinitions: %v", path, err)
	}
	ids := make([]string, 0, len(catalog))
	for id := range catalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	operations := make([]FrozenOperation, 0, len(catalog))
	for _, id := range ids {
		op := catalog[id]
		op.ID = id
		operations = append(operations, op)
	}
	return operations
}
