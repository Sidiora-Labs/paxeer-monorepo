package ranker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestPolicyAcceptsEveryFrozenRankingCategory(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate ranking policy test source")
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
	if !slices.Equal(Categories[:], frozen) {
		t.Fatalf("Categories = %v, frozen IF_RANKINGS enum = %v", Categories, frozen)
	}
	for _, category := range frozen {
		t.Run(category, func(t *testing.T) {
			policy, err := Policy(category)
			if err != nil {
				t.Fatalf("Policy(%q): %v", category, err)
			}
			if policy.Category != category || policy.Version != AlgorithmVersion+"/"+category {
				t.Fatalf("policy identity does not match %q: %+v", category, policy)
			}
			freshness, traders := int64(90), 1
			if category == "new" {
				freshness, traders = 30, 0
			}
			if policy.FreshnessSeconds != freshness || policy.MinimumTraders != traders {
				t.Fatalf("Policy(%q) freshness/trader floor = %d/%d, want %d/%d", category, policy.FreshnessSeconds, policy.MinimumTraders, freshness, traders)
			}
		})
	}
	for _, category := range []string{"", "unknown", "top-volume", "gain", "loss", "Gainers", "Losers", " gainers", "losers "} {
		if policy, err := Policy(category); err == nil {
			t.Errorf("Policy(%q) accepted category outside frozen enum: %+v", category, policy)
		}
	}
}
