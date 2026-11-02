package benchmark_test

import (
	"path/filepath"
	"runtime"
	"testing"

	evmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/benchmark"
)

func TestEveryBenchmarkScenarioLoads(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate scenario test file")
	}
	pattern := filepath.Join(filepath.Dir(testFile), "scenarios", "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob scenario files: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no scenario files match %s", pattern)
	}

	const evmChainID int64 = 31337
	const paxChainID = "benchmark-scenarios-test"
	if evmconfig.IsLiveEVMChainID(evmChainID) {
		t.Fatalf("test EVM chain ID %d is reserved for a live chain", evmChainID)
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			cfg, err := benchmark.LoadConfig(file, evmChainID, paxChainID)
			if err != nil {
				t.Fatalf("load scenario config %s: %v", file, err)
			}
			if cfg == nil {
				t.Fatal("loader returned a nil config")
			}
			if cfg.ChainID != evmChainID || cfg.PaxChainID != paxChainID {
				t.Errorf("chain IDs = (%d, %q), want (%d, %q)", cfg.ChainID, cfg.PaxChainID, evmChainID, paxChainID)
			}
			if len(cfg.Scenarios) == 0 {
				t.Fatal("config declares no scenarios")
			}
			for i, scenario := range cfg.Scenarios {
				if scenario.Weight <= 0 {
					t.Errorf("scenario %d (%s) has non-positive weight %v", i, scenario.Name, scenario.Weight)
				}
			}
		})
	}
}
