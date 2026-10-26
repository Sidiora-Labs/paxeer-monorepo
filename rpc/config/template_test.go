package config_test

import (
	"bytes"
	"testing"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/stretchr/testify/require"
)

func TestConfigTemplateRendersReadKeys(t *testing.T) {
	custom := config.DefaultConfig
	custom.MaxTxPoolTxs = 2345
	custom.EnableTestAPI = true
	custom.MaxConcurrentSimulationCalls = 7
	custom.RPCStatsInterval = 1250 * time.Millisecond

	for _, tc := range []struct {
		name string
		cfg  config.Config
	}{
		{name: "defaults", cfg: config.DefaultConfig},
		{name: "overrides", cfg: custom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := template.New("evm").Parse(config.ConfigTemplate)
			require.NoError(t, err)
			var rendered bytes.Buffer
			require.NoError(t, tmpl.Execute(&rendered, struct{ EVM config.Config }{EVM: tc.cfg}))

			var decoded map[string]map[string]interface{}
			_, err = toml.NewDecoder(&rendered).Decode(&decoded)
			require.NoError(t, err)
			require.Contains(t, decoded, "evm")
			for key, want := range map[string]interface{}{
				"max_tx_pool_txs":                 int64(tc.cfg.MaxTxPoolTxs),
				"enable_test_api":                 tc.cfg.EnableTestAPI,
				"max_concurrent_simulation_calls": int64(tc.cfg.MaxConcurrentSimulationCalls),
				"rpc_stats_interval":              tc.cfg.RPCStatsInterval.String(),
			} {
				require.Contains(t, decoded["evm"], key)
				require.Equal(t, want, decoded["evm"][key], key)
			}
		})
	}
}
