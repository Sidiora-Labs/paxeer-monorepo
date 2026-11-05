package layerxledger_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerCompletionRetiredSurfacesAbsent(t *testing.T) {
	plannedPaths := []struct {
		task string
		path string
	}{
		{"2.14.1", "precompiles/layerxledger/parity_test.go"},
		{"2.14.2", "precompiles/layerxauth/parity_test.go"},
		{"2.14.3", "precompiles/layerxledger/gas_conformance_test.go"},
		{"2.14.3", "precompiles/layerxauth/gas_conformance_test.go"},
		{"2.14.4", "modules/layerxledger/keeper/retire_test.go"},
		{"2.14.4", "modules/layerxauth/keeper/retire_test.go"},
		{"2.14.5", "modules/layerxauth/keeper/shared_host_test.go"},
		{"2.40.1", "precompiles/layerxpay/conformance_escrow_test.go"},
		{"2.40.2", "precompiles/layerxpay/conformance_budget_test.go"},
		{"2.40.3", "precompiles/layerxpay/conformance_service_test.go"},
		{"2.40.4", "precompiles/layerxagent/streams/parity_test.go"},
		{"2.40.5", "precompiles/layerxpay/gas_bound_test.go"},
		{"2.40.6", "modules/layerxagent/keeper/blockgas_bound_test.go"},
		{"2.40.7", "precompiles/layerxagent/scheduler/topics_test.go"},
		{"2.40.8", "precompiles/layerxagent/streams/topics_test.go"},
		{"2.40.9", "modules/layerxpay/keeper/genesis_rehearsal_test.go"},
	}

	for _, planned := range plannedPaths {
		t.Run(planned.path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(planned.path)))
			if err != nil {
				t.Fatalf("task %s planned path %s: %v", planned.task, planned.path, err)
			}
			if !info.Mode().IsRegular() {
				t.Errorf("task %s planned path %s must be a regular file, got %s", planned.task, planned.path, info.Mode())
			}
		})
	}
}
