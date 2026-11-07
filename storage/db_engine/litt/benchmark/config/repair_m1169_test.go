package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/unit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func repairM1169Load(t *testing.T, document string) (*BenchmarkConfig, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "benchmark config.json")
	require.NoError(t, os.WriteFile(path, []byte(document), 0600))
	return LoadConfig(path)
}

func TestRepairM_1169_LoadConfigRejectsInvalid(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "unopened-storage")
	paths, err := json.Marshal([]string{dbPath})
	require.NoError(t, err)
	maxInt := uint64(^uint(0) >> 1)
	number := func(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }
	cases := []struct {
		name, field, value, message string
	}{
		{"root-null", "", "null", "configuration must not be null"},
		{"nested-null", "LittConfig", "null", "LittConfig"},
		{"nested-no-paths", "LittConfig", `{}`, "LittConfig"},
		{"nested-null-paths", "LittConfig", `{"Paths":null}`, "LittConfig"},
		{"nested-empty-paths", "LittConfig", `{"Paths":[]}`, "LittConfig"},
		{"nested-blank-path", "LittConfig", `{"Paths":[" "]}`, "LittConfig.Paths"},
		{"zero-writers", "WriterParallelism", "0", "WriterParallelism"},
		{"negative-writers", "WriterParallelism", "-1", "WriterParallelism"},
		{"overflow-writers", "WriterParallelism", "1e100", "WriterParallelism"},
		{"zero-readers", "ReaderParallelism", "0", "ReaderParallelism"},
		{"negative-readers", "ReaderParallelism", "-1", "ReaderParallelism"},
		{"overflow-readers", "ReaderParallelism", "1e100", "ReaderParallelism"},
		{"zero-write-channel", "WriteInfoChanelSize", "0", "WriteInfoChanelSize"},
		{"negative-write-channel", "WriteInfoChanelSize", "-1", "WriteInfoChanelSize"},
		{"int-overflow-write-channel", "WriteInfoChanelSize", strconv.FormatUint(maxInt+1, 10), "WriteInfoChanelSize"},
		{"allocation-overflow-write-channel", "WriteInfoChanelSize", strconv.FormatUint(maxInt/uint64(strconv.IntSize/8)+1, 10), "WriteInfoChanelSize"},
		{"uint-overflow-write-channel", "WriteInfoChanelSize", "18446744073709551616", "WriteInfoChanelSize"},
		{"zero-read-channel", "ReadInfoChanelSize", "0", "ReadInfoChanelSize"},
		{"negative-read-channel", "ReadInfoChanelSize", "-1", "ReadInfoChanelSize"},
		{"allocation-overflow-read-channel", "ReadInfoChanelSize", strconv.FormatUint(maxInt/uint64(strconv.IntSize/8)+1, 10), "ReadInfoChanelSize"},
		{"uint-overflow-read-channel", "ReadInfoChanelSize", "18446744073709551616", "ReadInfoChanelSize"},
		{"zero-cohort", "CohortSize", "0", "CohortSize"},
		{"negative-cohort", "CohortSize", "-1", "CohortSize"},
		{"signed-overflow-cohort", "CohortSize", "9223372036854775808", "CohortSize"},
		{"zero-pool", "RandomPoolSize", "0", "RandomPoolSize"},
		{"negative-pool", "RandomPoolSize", "-1", "RandomPoolSize"},
		{"overflow-pool", "RandomPoolSize", strconv.FormatUint(maxInt+1, 10), "RandomPoolSize"},
		{"value-format-overflow", "ValueSizeMB", number((float64(math.MaxUint32) + 1) / unit.MB), "ValueSizeMB"},
		{"batch-overflow", "BatchSizeMB", number(float64(maxInt) / unit.MB), "BatchSizeMB"},
		{"write-rate-overflow", "MaximumWriteThroughputMB", number(math.Exp2(64) / unit.MB), "MaximumWriteThroughputMB"},
		{"read-rate-overflow", "MaximumReadThroughputMB", number(math.Exp2(64) / unit.MB), "MaximumReadThroughputMB"},
		{"write-rate-rounds-to-zero-per-worker", "MaximumWriteThroughputMB", number(1.0 / unit.MB), "MaximumWriteThroughputMB"},
		{"read-rate-rounds-to-zero-per-worker", "MaximumReadThroughputMB", number(1.0 / unit.MB), "MaximumReadThroughputMB"},
		{"zero-gc-period", "CohortGCPeriodSeconds", "0", "CohortGCPeriodSeconds"},
		{"zero-ttl", "TTLHours", "0", "TTLHours"},
		{"duration-int64-boundary", "TimeLimitSeconds", number(math.Exp2(63) / float64(time.Second)), "TimeLimitSeconds"},
		{"doubled-startup-overflow", "StartupSleepFactorSeconds", number(1.1 * math.Exp2(62) / float64(time.Second)), "StartupSleepFactorSeconds"},
		{"margin-equals-ttl", "ReadSafetyMarginMinutes", "60", "ReadSafetyMarginMinutes"},
		{"margin-exceeds-ttl", "ReadSafetyMarginMinutes", "61", "ReadSafetyMarginMinutes"},
		{"unknown-field", "NoSuchField", "1", "unknown field"},
	}
	for _, field := range []string{"ValueSizeMB", "BatchSizeMB", "MaximumWriteThroughputMB", "MaximumReadThroughputMB"} {
		for _, value := range []string{"0", "-1", "1e-300", "1e300", "1e309"} {
			cases = append(cases, struct{ name, field, value, message string }{field + "-" + value, field, value, field})
		}
	}
	for _, field := range []string{"CohortGCPeriodSeconds", "TTLHours", "ReadSafetyMarginMinutes", "StartupSleepFactorSeconds", "MetricsLoggingPeriodSeconds", "TimeLimitSeconds"} {
		for _, value := range []string{"-1", "1e-300", "1e300"} {
			cases = append(cases, struct{ name, field, value, message string }{field + "-" + value, field, value, field})
		}
	}
	for _, pair := range []struct{ field, value string }{
		{"GCBatchSize", "0"}, {"ControlChannelSize", "0"}, {"ControlChannelSize", "-1"},
		{"TargetSegmentFileSize", "0"}, {"MaxSegmentKeyCount", "0"}, {"TargetSegmentKeyFileSize", "0"},
		{"GCPeriod", "0"}, {"GCPeriod", "-1"}, {"MetricsUpdateInterval", "0"}, {"MetricsUpdateInterval", "-1"},
		{"MetricsPort", "-1"}, {"MetricsPort", "65536"}, {"MinimumFlushInterval", "-1"}, {"KeymapType", `"unsupported"`},
	} {
		value := fmt.Sprintf(`{"Paths":%s,%q:%s}`, paths, pair.field, pair.value)
		cases = append(cases, struct{ name, field, value, message string }{"nested-" + pair.field + "-" + pair.value, "LittConfig", value, "LittConfig"})
	}
	tooManyPaths := make([]string, litt.MaxShardingFactor+1)
	for i := range tooManyPaths {
		tooManyPaths[i] = filepath.Join(dbPath, strconv.Itoa(i))
	}
	encodedPaths, err := json.Marshal(tooManyPaths)
	require.NoError(t, err)
	cases = append(cases, struct{ name, field, value, message string }{"too-many-shards", "LittConfig", fmt.Sprintf(`{"Paths":%s}`, encodedPaths), "LittConfig.Paths"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := tc.value
			if tc.field != "" {
				if tc.field == "LittConfig" {
					document = fmt.Sprintf(`{"LittConfig":%s}`, tc.value)
				} else {
					document = fmt.Sprintf(`{"LittConfig":{"Paths":%s},%q:%s}`, paths, tc.field, tc.value)
				}
			}
			loaded, err := repairM1169Load(t, document)
			require.Error(t, err)
			require.Nil(t, loaded, "invalid config must not escape to consumers")
			require.Contains(t, err.Error(), tc.message)
			_, statErr := os.Stat(dbPath)
			require.True(t, os.IsNotExist(statErr), "validation must precede storage creation")
		})
	}
}

func TestRepairM_1169_LoadConfigValidBoundaries(t *testing.T) {
	t.Run("shipped-config", func(t *testing.T) {
		loaded, err := LoadConfig("basic-config.json")
		require.NoError(t, err)
		require.NoError(t, loaded.LittConfig.Validate())
		require.Len(t, loaded.LittConfig.Paths, 3)
		require.Equal(t, 1024.0, loaded.MaximumWriteThroughputMB)
	})
	t.Run("one-byte-and-disabled-options", func(t *testing.T) {
		document := fmt.Sprintf(`{
			"LittConfig":{"Paths":["/unused/litt"],"MetricsEnabled":false,"MetricsUpdateInterval":0},
			"MetadataDirectory":%q,
			"WriterParallelism":1,"ReaderParallelism":1,
			"WriteInfoChanelSize":1,"ReadInfoChanelSize":1,"CohortSize":1,"RandomPoolSize":2,
			"ValueSizeMB":%g,"BatchSizeMB":%g,
			"MaximumWriteThroughputMB":%g,"MaximumReadThroughputMB":%g,
			"CohortGCPeriodSeconds":1e-9,"TTLHours":1,"ReadSafetyMarginMinutes":0,
			"StartupSleepFactorSeconds":0,"MetricsLoggingPeriodSeconds":0,"TimeLimitSeconds":0
		}`, t.TempDir(), 1.0/unit.MB, 1.0/unit.MB, 1.0/unit.MB, 1.0/unit.MB)
		loaded, err := repairM1169Load(t, document)
		require.NoError(t, err)
		require.True(t, filepath.IsAbs(loaded.MetadataDirectory))
		require.Equal(t, uint64(1), uint64(loaded.ValueSizeMB*unit.MB))
		require.Equal(t, uint64(1), uint64(loaded.BatchSizeMB*unit.MB))
		require.Equal(t, uint64(1), loaded.CohortSize)
		require.Equal(t, 1, cap(make(chan *int, loaded.WriteInfoChanelSize)))
		require.Equal(t, 1, cap(make(chan *int, loaded.ReadInfoChanelSize)))
		for _, throughput := range []float64{loaded.MaximumWriteThroughputMB, loaded.MaximumReadThroughputMB} {
			limiter := rate.NewLimiter(rate.Limit(uint64(throughput*unit.MB)), int(loaded.ValueSizeMB*unit.MB))
			require.True(t, limiter.ReserveN(time.Now(), 1).OK())
		}
		period := time.Duration(loaded.CohortGCPeriodSeconds * float64(time.Second))
		require.Equal(t, time.Nanosecond, period)
		ticker := time.NewTicker(period)
		ticker.Stop()
		require.Zero(t, loaded.StartupSleepFactorSeconds)
		require.Zero(t, loaded.MetricsLoggingPeriodSeconds)
		require.Zero(t, loaded.TimeLimitSeconds)
	})
	t.Run("representable-upper-durations", func(t *testing.T) {
		seconds := math.Exp2(62) / float64(time.Second)
		document := fmt.Sprintf(`{"LittConfig":{"Paths":["/unused/litt"]},"TimeLimitSeconds":%g,"StartupSleepFactorSeconds":%g}`, seconds, seconds/2)
		loaded, err := repairM1169Load(t, document)
		require.NoError(t, err)
		require.Positive(t, time.Duration(loaded.TimeLimitSeconds*float64(time.Second)))
		require.Positive(t, time.Duration(loaded.StartupSleepFactorSeconds*float64(time.Second)*2))
	})
}

func TestRepairM_1169_ValidateNonFinite(t *testing.T) {
	fields := []struct {
		name string
		set  func(*BenchmarkConfig, float64)
	}{
		{"ValueSizeMB", func(c *BenchmarkConfig, v float64) { c.ValueSizeMB = v }},
		{"BatchSizeMB", func(c *BenchmarkConfig, v float64) { c.BatchSizeMB = v }},
		{"MaximumWriteThroughputMB", func(c *BenchmarkConfig, v float64) { c.MaximumWriteThroughputMB = v }},
		{"MaximumReadThroughputMB", func(c *BenchmarkConfig, v float64) { c.MaximumReadThroughputMB = v }},
		{"CohortGCPeriodSeconds", func(c *BenchmarkConfig, v float64) { c.CohortGCPeriodSeconds = v }},
		{"TTLHours", func(c *BenchmarkConfig, v float64) { c.TTLHours = v }},
		{"ReadSafetyMarginMinutes", func(c *BenchmarkConfig, v float64) { c.ReadSafetyMarginMinutes = v }},
		{"StartupSleepFactorSeconds", func(c *BenchmarkConfig, v float64) { c.StartupSleepFactorSeconds = v }},
		{"MetricsLoggingPeriodSeconds", func(c *BenchmarkConfig, v float64) { c.MetricsLoggingPeriodSeconds = v }},
		{"TimeLimitSeconds", func(c *BenchmarkConfig, v float64) { c.TimeLimitSeconds = v }},
	}
	for _, field := range fields {
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			t.Run(field.name+"-"+fmt.Sprint(value), func(t *testing.T) {
				cfg := DefaultBenchmarkConfig()
				cfg.LittConfig.Paths = []string{"/unused/litt"}
				field.set(cfg, value)
				err := cfg.Validate()
				require.Error(t, err)
				require.Contains(t, err.Error(), field.name)
			})
		}
	}
	for _, literal := range []string{"NaN", "Infinity", "-Infinity"} {
		t.Run("json-"+literal, func(t *testing.T) {
			cfg, err := repairM1169Load(t, fmt.Sprintf(`{"LittConfig":{"Paths":["/unused/litt"]},"ValueSizeMB":%s}`, literal))
			require.Error(t, err)
			require.Nil(t, cfg)
		})
	}
}

func TestRepairM_1169_DashboardDatasourceImport(t *testing.T) {
	data, err := os.ReadFile("benchmark-grafana-dashboard.json")
	require.NoError(t, err)
	type datasource struct{ Type, UID string }
	type target struct {
		Datasource datasource
		Expr       string
	}
	type panel struct {
		ID         int
		Datasource datasource
		Targets    []target
	}
	type dashboard struct {
		Inputs      []struct{ Name, Label, Type, PluginID, PluginName string } `json:"__inputs"`
		Panels      []panel
		Annotations struct {
			List []struct{ Datasource datasource }
		}
	}
	var exported dashboard
	require.NoError(t, json.Unmarshal(data, &exported))
	require.Len(t, exported.Inputs, 1)
	input := exported.Inputs[0]
	require.Equal(t, "datasource", input.Type)
	require.Equal(t, "prometheus", input.PluginID)
	require.Equal(t, "Prometheus", input.PluginName)
	require.NotEmpty(t, input.Label)
	require.Equal(t, "DS_PROMETHEUS", input.Name)
	placeholder := "${" + input.Name + "}"
	require.NotContains(t, string(data), "denye6lsft2bka")
	require.NotEmpty(t, exported.Panels)
	for _, p := range exported.Panels {
		require.Equal(t, datasource{"prometheus", placeholder}, p.Datasource, "panel %d", p.ID)
		require.NotEmpty(t, p.Targets)
		for _, query := range p.Targets {
			require.Equal(t, datasource{"prometheus", placeholder}, query.Datasource, "panel %d target", p.ID)
		}
	}
	for _, selectedUID := range []string{"operator-prometheus-a", "different-instance-prometheus-b"} {
		t.Run(selectedUID, func(t *testing.T) {
			// Grafana's import input contract replaces ${name} with the selected datasource UID.
			importJSON := bytes.ReplaceAll(data, []byte(placeholder), []byte(selectedUID))
			var imported dashboard
			require.NoError(t, json.Unmarshal(importJSON, &imported))
			roundTrip, err := json.Marshal(imported)
			require.NoError(t, err)
			require.NotContains(t, string(roundTrip), placeholder)
			require.Len(t, imported.Panels, len(exported.Panels))
			var queries int
			for i, p := range imported.Panels {
				require.Equal(t, datasource{"prometheus", selectedUID}, p.Datasource)
				require.Len(t, p.Targets, len(exported.Panels[i].Targets))
				for j, query := range p.Targets {
					require.Equal(t, datasource{"prometheus", selectedUID}, query.Datasource)
					require.Equal(t, exported.Panels[i].Targets[j].Expr, query.Expr, "import must preserve PromQL and Grafana interval macros")
					if strings.TrimSpace(query.Expr) != "" {
						queries++
					}
				}
			}
			require.Positive(t, queries)
			require.Len(t, imported.Annotations.List, 1)
			require.Equal(t, datasource{"grafana", "-- Grafana --"}, imported.Annotations.List[0].Datasource)
		})
	}
}
