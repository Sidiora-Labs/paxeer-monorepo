package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/unit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/benchmark/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1171_LauncherHandoff(t *testing.T) {
	script, err := os.ReadFile("run.sh")
	require.NoError(t, err)
	// Inspect the production shell boundary without requiring a shell or a built
	// benchmark executable. Both expansions must remain single literal words;
	// exec must replace the launcher rather than leave it between signals and CLI.
	lines := strings.Split(strings.TrimSpace(string(script)), "\n")
	require.Equal(t, `exec "$BINARY_PATH" "$CONFIG_PATH"`, lines[len(lines)-1])
}

func TestRepairM_1171_ConfigPaths(t *testing.T) {
	launchDir := filepath.Join(t.TempDir(), "benchmark launch [1171] * ?")
	require.NoError(t, os.MkdirAll(launchDir, 0700))
	t.Chdir(launchDir)
	for _, name := range []string{"config with spaces.json", "config [1171] * ?.json"} {
		t.Run(name, func(t *testing.T) {
			cfg := config.DefaultBenchmarkConfig()
			cfg.LittConfig.Paths = []string{filepath.Join(t.TempDir(), "database with spaces")}
			cfg.LittConfig.KeymapType = keymap.MemKeymapType
			cfg.LittConfig.MetricsEnabled = false
			cfg.MetadataDirectory = filepath.Join(t.TempDir(), "metadata with spaces")
			cfg.RandomPoolSize = 256
			cfg.ValueSizeMB = 32.0 / unit.MB
			cfg.BatchSizeMB = 128.0 / unit.MB
			cfg.CohortSize = 2
			cfg.WriteInfoChanelSize = 2
			cfg.ReadInfoChanelSize = 2
			cfg.WriterParallelism = 1
			cfg.ReaderParallelism = 1
			cfg.MaximumWriteThroughputMB = 0.01
			cfg.MaximumReadThroughputMB = 0.01
			cfg.StartupSleepFactorSeconds = 0
			cfg.MetricsLoggingPeriodSeconds = 0
			cfg.TimeLimitSeconds = 0.05
			data, err := json.Marshal(cfg)
			require.NoError(t, err)
			relative := filepath.Join("configs with spaces", name)
			require.NoError(t, os.MkdirAll(filepath.Dir(relative), 0700))
			require.NoError(t, os.WriteFile(relative, data, 0600))
			absolute, err := filepath.Abs(relative)
			require.NoError(t, err)
			for _, path := range []string{relative, absolute} {
				var stderr bytes.Buffer
				require.Equal(t, 0, RunCLI([]string{path}, &stderr), "path %q: %s", path, stderr.String())
				require.Empty(t, stderr.String())
			}
			// A real persisted cohort shows the CLI loaded the intended JSON and
			// ran its engine through storage and orderly shutdown.
			cohort, err := LoadCohort(filepath.Join(cfg.MetadataDirectory, "cohorts", "0.cohort"))
			require.NoError(t, err)
			require.Equal(t, uint64(0), cohort.LowKeyIndex())
			require.Equal(t, cfg.CohortSize-1, cohort.HighKeyIndex())
		})
	}
}

func TestRepairM_1171_ArgumentContract(t *testing.T) {
	for _, args := range [][]string{nil, {"first.json", "second.json"}} {
		var stderr bytes.Buffer
		require.Equal(t, 1, RunCLI(args, &stderr))
		require.Equal(t, "Usage: run.sh <config-file-path>\n", stderr.String())
	}
	var stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing config [1171] * ?.json")
	require.Equal(t, 1, RunCLI([]string{missing}, &stderr))
	require.Contains(t, stderr.String(), "Failed to create benchmark engine")
	require.Contains(t, stderr.String(), missing)
}
