package cmd

import (
	"bytes"
	"reflect"
	"testing"
	"text/template"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/blocktest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/replay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func readAppToml(t *testing.T, appTemplate string, config CustomAppConfig) *viper.Viper {
	t.Helper()
	tmpl, err := template.New("app").Parse(appTemplate)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tmpl.Execute(&buf, config))
	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(&buf))
	return v
}

func TestAppTomlWasmKeysMatchReadWasmConfig(t *testing.T) {
	appTemplate, appConfig := initAppConfig()
	config := appConfig.(CustomAppConfig)
	require.Equal(t, wasmtypes.DefaultWasmConfig().MemoryCacheSize, config.WASM.MemoryCacheSize)
	require.Empty(t, config.WASM.SimulationGasLimit)

	for _, tc := range []struct {
		name               string
		memoryCacheSize    uint32
		simulationGasLimit string
	}{
		{name: "defaults", memoryCacheSize: config.WASM.MemoryCacheSize},
		{name: "custom", memoryCacheSize: config.WASM.MemoryCacheSize + 1, simulationGasLimit: "987654"},
		{name: "cache disabled", memoryCacheSize: 0, simulationGasLimit: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config.WASM.MemoryCacheSize = tc.memoryCacheSize
			config.WASM.SimulationGasLimit = tc.simulationGasLimit
			v := readAppToml(t, appTemplate, config)
			require.True(t, v.IsSet("wasm.memory_cache_size"))
			require.True(t, v.IsSet("wasm.simulation_gas_limit"))
			require.False(t, v.IsSet("wasm.lru_size"))
			require.Equal(t, tc.simulationGasLimit, v.Get("wasm.simulation_gas_limit"))
			got, err := wasm.ReadWasmConfig(v)
			require.NoError(t, err)
			require.Equal(t, tc.memoryCacheSize, got.MemoryCacheSize)
			require.Equal(t, uint64(300000), got.SmartQueryGasLimit)
			if tc.simulationGasLimit == "" {
				require.Nil(t, got.SimulationGasLimit)
			} else {
				require.NotNil(t, got.SimulationGasLimit)
				require.Equal(t, v.GetUint64("wasm.simulation_gas_limit"), *got.SimulationGasLimit)
			}
		})
	}
}

func TestAppTomlEthReplayContractStateChecksKey(t *testing.T) {
	appTemplate, appConfig := initAppConfig()
	config := appConfig.(CustomAppConfig)
	for _, enabled := range []bool{false, true} {
		config.ETHReplay.ContractStateChecks = enabled
		v := readAppToml(t, appTemplate, config)
		require.True(t, v.IsSet("eth_replay.contract_state_checks"))
		require.False(t, v.IsSet("eth_replay.eth_replay_contract_state_checks"))
		got, err := replay.ReadConfig(v)
		require.NoError(t, err)
		require.Equal(t, enabled, got.ContractStateChecks)
	}
}

func TestCustomAppConfigEthBlockTestTag(t *testing.T) {
	field, ok := reflect.TypeOf(CustomAppConfig{}).FieldByName("ETHBlockTest")
	require.True(t, ok)
	require.Equal(t, "eth_blocktest", field.Tag.Get("mapstructure"))

	appTemplate, appConfig := initAppConfig()
	config := appConfig.(CustomAppConfig)
	config.ETHBlockTest = blocktest.Config{Enabled: true, TestDataPath: "/tmp/paxd-blocktest-data"}
	v := readAppToml(t, appTemplate, config)
	require.True(t, v.IsSet("eth_blocktest.eth_blocktest_enabled"))
	require.False(t, v.IsSet("eth_block_test"))
	var decoded CustomAppConfig
	require.NoError(t, v.Unmarshal(&decoded))
	require.Equal(t, config.ETHBlockTest, decoded.ETHBlockTest)
	got, err := blocktest.ReadConfig(v)
	require.NoError(t, err)
	require.Equal(t, config.ETHBlockTest, got)
}
