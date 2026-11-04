package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func repairM1310ReadConfig(t *testing.T, document string) *viper.Viper {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.toml")
	require.NoError(t, os.WriteFile(path, []byte(document), 0600))
	v := viper.New()
	v.SetConfigFile(path)
	require.NoError(t, v.ReadInConfig())
	return v
}

func TestRepairM_1310_LabelValidation(t *testing.T) {
	cases := []struct {
		name  string
		label string
	}{
		{"numeric_name", `[125, "paxeer"]`},
		{"numeric_value", `["chain_id", 125]`},
		{"numeric_both", `[125, 126]`},
		{"boolean_name", `[true, "paxeer"]`},
		{"boolean_value", `["chain_id", false]`},
		{"nested_name", `[["chain_id"], "paxeer"]`},
		{"nested_value", `["chain_id", ["paxeer"]]`},
		{"empty_pair", `[]`},
		{"short_pair", `["chain_id"]`},
		{"long_pair", `["chain_id", "paxeer", "extra"]`},
		{"not_a_pair", `"chain_id"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := repairM1310ReadConfig(t, fmt.Sprintf(`[telemetry]
global-labels = [["network", "paxeer"], %s, ["node", "validator"]]
`, tc.label))
			cfg, err := GetConfig(v)
			require.ErrorContains(t, err, "failed to parse global label number 1 from config")
			require.Equal(t, Config{}, cfg, "invalid labels must not return a partial config")
		})
	}
}

func TestRepairM_1310_ValidLabelsRoundtrip(t *testing.T) {
	for _, labels := range [][][]string{
		{},
		{{"chain_id", "125"}, {"network", "paxeer"}, {"empty", ""}},
	} {
		cfg := DefaultConfig()
		cfg.Telemetry.GlobalLabels = labels
		path := filepath.Join(t.TempDir(), "app.toml")
		WriteConfigFile(path, cfg)
		v := viper.New()
		v.SetConfigFile(path)
		require.NoError(t, v.ReadInConfig())
		loaded, err := GetConfig(v)
		require.NoError(t, err)
		parsed, err := ParseConfig(v)
		require.NoError(t, err)
		require.Equal(t, labels, loaded.Telemetry.GlobalLabels)
		require.Equal(t, cfg.Telemetry, loaded.Telemetry)
		require.Equal(t, parsed.Telemetry, loaded.Telemetry)
	}
}

func TestRepairM_1310_PruningRoundtrip(t *testing.T) {
	cases := []struct {
		name             string
		strategy         string
		keepEvery        string
		interval         string
		snapshotInterval uint64
		pruningError     bool
		configError      bool
	}{
		{"custom_periodic", "custom", "30", "10", 60, false, false},
		{"custom_recent_only", "custom", "0", "10", 0, false, false},
		{"custom_keep_all", "custom", "1", "0", 0, false, false},
		{"invalid_periodic_interval", "custom", "30", "0", 0, true, false},
		{"invalid_recent_interval", "custom", "0", "0", 0, true, false},
		{"invalid_keep_all_interval", "custom", "1", "10", 0, true, false},
		{"everything_with_snapshots", "everything", "0", "10", 60, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := repairM1310ReadConfig(t, fmt.Sprintf(`minimum-gas-prices = "0.025uhpx"
inter-block-cache = true
concurrency-workers = 17
occ-enabled = true
index-events = ["message.sender"]
pruning = %q
pruning-keep-recent = "120"
pruning-keep-every = %q
pruning-interval = %q
[telemetry]
global-labels = []
[state-sync]
snapshot-interval = %d
`, tc.strategy, tc.keepEvery, tc.interval, tc.snapshotInterval))
			loaded, err := GetConfig(v)
			require.NoError(t, err)
			parsed, err := ParseConfig(v)
			require.NoError(t, err)
			expected := BaseConfig{
				MinGasPrices:       "0.025uhpx",
				InterBlockCache:    true,
				ConcurrencyWorkers: 17,
				OccEnabled:         true,
				IndexEvents:        []string{"message.sender"},
				Pruning:            tc.strategy,
				PruningKeepRecent:  "120",
				PruningKeepEvery:   tc.keepEvery,
				PruningInterval:    tc.interval,
			}
			require.Equal(t, expected, loaded.BaseConfig)
			require.Equal(t, expected, parsed.BaseConfig)
			require.Equal(t, parsed.StateSync.SnapshotInterval, loaded.StateSync.SnapshotInterval)
			for _, cfg := range []Config{loaded, *parsed} {
				values := make([]uint64, 3)
				for i, value := range []string{cfg.PruningKeepRecent, cfg.PruningKeepEvery, cfg.PruningInterval} {
					values[i], err = strconv.ParseUint(value, 10, 64)
					require.NoError(t, err)
				}
				opts := storetypes.NewPruningOptions(values[0], values[1], values[2])
				if tc.pruningError {
					require.ErrorContains(t, opts.Validate(), "invalid 'Interval'")
				} else {
					require.NoError(t, opts.Validate())
				}
				if tc.configError {
					require.ErrorContains(t, cfg.ValidateBasic(tmcfg.DefaultConfig()), "cannot enable state sync snapshots")
				} else {
					require.NoError(t, cfg.ValidateBasic(tmcfg.DefaultConfig()))
				}
			}
		})
	}
}
