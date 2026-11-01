package app

import (
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func TestProductionUnservedOperationsAreFrozenApiOperations(t *testing.T) {
	if len(productionUnservedOperations) != 21 {
		t.Fatalf("production unserved operations: got %d, want 21", len(productionUnservedOperations))
	}

	apiOperations := make(map[string]struct{})
	for _, op := range internaltest.FrozenV2Operations(t) {
		if op.Service == "api" {
			apiOperations[op.Method+" "+op.Path] = struct{}{}
		}
	}
	developmentOperations := make(map[string]struct{})
	for _, op := range developmentUnservedOperations {
		developmentOperations[op.method+" "+op.path] = struct{}{}
	}

	seen := make(map[string]struct{})
	for _, op := range productionUnservedOperations {
		key := op.method + " " + op.path
		if _, ok := apiOperations[key]; !ok {
			t.Errorf("production unserved operation absent from frozen api catalog: %s", key)
		}
		if _, ok := developmentOperations[key]; ok {
			t.Errorf("production unserved operation also listed for development: %s", key)
		}
		if _, ok := seen[key]; ok {
			t.Errorf("duplicate production unserved operation: %s", key)
		}
		seen[key] = struct{}{}
	}
}
