package config_test

import (
	"bytes"
	"testing"
	"text/template"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func repairM1299Options(t *testing.T, producer config.Config) *viper.Viper {
	t.Helper()
	tmpl, err := template.New("evm").Parse(config.ConfigTemplate)
	require.NoError(t, err)
	var emitted bytes.Buffer
	require.NoError(t, tmpl.Execute(&emitted, struct{ EVM config.Config }{EVM: producer}))
	options := viper.New()
	options.SetConfigType("toml")
	require.NoError(t, options.ReadConfig(&emitted))
	require.True(t, options.IsSet("evm.filter_timeout"))
	require.True(t, options.IsSet("evm.rpc_stats_interval"))
	return options
}

func TestRepairM_1299_InvalidDurations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filter  time.Duration
		stats   time.Duration
		key     string
		minimum string
	}{
		{"negative filter", -time.Second, time.Second, "evm.filter_timeout", "2ns"},
		{"negative nanosecond filter", -time.Nanosecond, time.Second, "evm.filter_timeout", "2ns"},
		{"zero filter", 0, time.Second, "evm.filter_timeout", "2ns"},
		{"one nanosecond filter", time.Nanosecond, time.Second, "evm.filter_timeout", "2ns"},
		{"negative stats", time.Second, -time.Second, "evm.rpc_stats_interval", "1ns"},
		{"negative nanosecond stats", time.Second, -time.Nanosecond, "evm.rpc_stats_interval", "1ns"},
		{"zero stats", time.Second, 0, "evm.rpc_stats_interval", "1ns"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, enabled := range []bool{true, false} {
				producer := config.DefaultConfig
				producer.HTTPEnabled = enabled
				producer.WSEnabled = enabled
				producer.FilterTimeout = tc.filter
				producer.RPCStatsInterval = tc.stats
				_, err := config.ReadConfig(repairM1299Options(t, producer))
				require.ErrorContains(t, err, tc.key)
				require.ErrorContains(t, err, "at least "+tc.minimum)
			}
		})
	}
}

func TestRepairM_1299_DurationRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter time.Duration
		stats  time.Duration
	}{
		{"defaults", config.DefaultConfig.FilterTimeout, config.DefaultConfig.RPCStatsInterval},
		{"minimums", 2 * time.Nanosecond, time.Nanosecond},
		{"odd filter", 3 * time.Nanosecond, 2 * time.Nanosecond},
		{"operator values", 1500 * time.Millisecond, 250 * time.Millisecond},
		{"maximums", time.Duration(1<<63 - 1), time.Duration(1<<63 - 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := config.DefaultConfig
			producer.FilterTimeout = tc.filter
			producer.RPCStatsInterval = tc.stats
			decoded, err := config.ReadConfig(repairM1299Options(t, producer))
			require.NoError(t, err)
			require.Equal(t, producer, decoded)
			// Exercise the actual ticker operations used by cleanupLoop and
			// StartMetricsPrinter with the decoded configuration values.
			cleanup := time.NewTicker(decoded.FilterTimeout / 2)
			defer cleanup.Stop()
			stats := time.NewTicker(decoded.RPCStatsInterval)
			defer stats.Stop()
		})
	}
}

func TestRepairM_1299_DefaultsAndDocumentation(t *testing.T) {
	decoded, err := config.ReadConfig(viper.New())
	require.NoError(t, err)
	require.Equal(t, config.DefaultConfig, decoded)
	require.GreaterOrEqual(t, decoded.FilterTimeout, 2*time.Nanosecond)
	require.GreaterOrEqual(t, decoded.RPCStatsInterval, time.Nanosecond)
	require.Contains(t, config.ConfigTemplate, "# Timeout for filters. Minimum: 2ns; cleanup runs every half-timeout.\nfilter_timeout =")
	require.Contains(t, config.ConfigTemplate, "# Interval for reporting RPC stats. Minimum: 1ns (must be positive).\nrpc_stats_interval =")
	// Keep the ordinary package regression cases in the focused repair gate.
	assertTickerDurationErrors(t)
}
