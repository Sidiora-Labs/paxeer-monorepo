package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSnapshotRoutesAcceptEveryFrozenRankingCategory(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate snapshot route test source")
	}
	path := filepath.Join(filepath.Dir(source), "../../../../../sdk/launchpad/packages/sdk/src/operations.gen.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, definitions, ok := strings.Cut(string(data), "export const operationDefinitions = ")
	if !ok {
		t.Fatal("frozen SDK operationDefinitions declaration missing")
	}
	definitions, ok = strings.CutSuffix(strings.TrimSpace(definitions), " as const;")
	if !ok {
		t.Fatal("frozen SDK operationDefinitions terminator missing")
	}
	var operations map[string]json.RawMessage
	if err := json.Unmarshal([]byte(definitions), &operations); err != nil {
		t.Fatalf("decode frozen SDK operations: %v", err)
	}
	rawRankings, ok := operations["IF_RANKINGS"]
	if !ok {
		t.Fatal("frozen SDK IF_RANKINGS operation missing")
	}
	var rankings struct {
		Parameters []struct {
			In     string `json:"in"`
			Name   string `json:"name"`
			Schema struct {
				Enum []string `json:"enum"`
			} `json:"schema"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(rawRankings, &rankings); err != nil {
		t.Fatalf("decode frozen SDK IF_RANKINGS: %v", err)
	}
	var frozen []string
	categoryParameters := 0
	for _, parameter := range rankings.Parameters {
		if parameter.In == "path" && parameter.Name == "category" {
			categoryParameters++
			frozen = parameter.Schema.Enum
		}
	}
	if categoryParameters != 1 || len(frozen) == 0 {
		t.Fatalf("expected one nonempty IF_RANKINGS category enum, got %d parameters and %v", categoryParameters, frozen)
	}
	for _, category := range frozen {
		t.Run(category, func(t *testing.T) {
			got, err := snapshotCategory(category)
			if err != nil || got != category {
				t.Fatalf("snapshotCategory(%q) = %q, %v; want %q, nil", category, got, err, category)
			}
		})
	}
	t.Run("top_volume", func(t *testing.T) {
		got, err := snapshotCategory("top_volume")
		if err != nil || got != "volume" {
			t.Fatalf("snapshotCategory(%q) = %q, %v; want %q, nil", "top_volume", got, err, "volume")
		}
	})
}
