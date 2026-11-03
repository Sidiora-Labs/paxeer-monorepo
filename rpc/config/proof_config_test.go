// Proves: config.Config/DefaultConfig/ConfigTemplate -> app.toml and Viper AppOptions -> app.ReadEVMRPCConfig/ReadConfig.
//
// The eight config interfaces are exercised by the actual paxd init command and
// the root command's template and CustomAppConfig producers, including the four
// batch, subscription and synchronous submission keys. Each consumer opens the
// emitted file afresh, twice; rewriting it then proves no previous decode is used.
// No chain state persists across this path: the persisted artifact is app.toml.
//
// Repairs rpc/config/config.go ReadConfig: negative send_raw_tx_sync_timeout
// values were accepted, and conversion errors did not identify the config key.
// Repairs daemon/paxd/cmd/init_test.go TestInitModeConfiguration: its simplified
// template omitted EVM settings and therefore could not prove their mode values.
package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	svrcmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	srvconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func proofConfigOptions(t *testing.T, path string) *viper.Viper {
	t.Helper()
	options := viper.New()
	options.SetConfigFile(path)
	require.NoError(t, options.ReadInConfig())
	return options
}

func proofConfigObserve(t *testing.T, path string, want config.Config) {
	t.Helper()
	for attempt := 0; attempt < 2; attempt++ {
		// A new reader and decoder each time, as on node startup after reopening.
		options := proofConfigOptions(t, path)
		for _, key := range []string{
			"evm.batch_request_limit",
			"evm.batch_response_max_size",
			"evm.max_subscriptions_logs",
			"evm.send_raw_tx_sync_timeout",
		} {
			require.True(t, options.IsSet(key), "producer omitted %s", key)
		}
		observed, err := app.ReadEVMRPCConfig(options)
		require.NoError(t, err)
		require.Equal(t, want, observed)
		decoded, err := config.ReadConfig(options)
		require.NoError(t, err)
		require.Equal(t, observed, decoded)
	}
}

func proofConfigWrite(path string, producer cmd.CustomAppConfig) {
	appTemplate, _ := cmd.InitAppConfig()
	srvconfig.SetConfigTemplate(appTemplate)
	srvconfig.WriteConfigFile(path, producer)
}

func TestProofConfigInit(t *testing.T) {
	// Assert the contract's defaults independently of the producer's constants.
	require.Equal(t, 1000, config.BatchRequestLimit)
	require.Equal(t, 25000000, config.BatchResponseMaxSize)
	require.Equal(t, uint64(10000), config.MaxSubscriptionsLogs)
	require.Equal(t, 2*time.Second, config.SendRawTxSyncTimeout)
	require.Equal(t, config.BatchRequestLimit, config.DefaultConfig.BatchRequestLimit)
	require.Equal(t, config.BatchResponseMaxSize, config.DefaultConfig.BatchResponseMaxSize)
	require.Equal(t, config.MaxSubscriptionsLogs, config.DefaultConfig.MaxSubscriptionsLogs)
	require.Equal(t, config.SendRawTxSyncTimeout, config.DefaultConfig.SendRawTxSyncTimeout)
	for _, mode := range []params.NodeMode{
		params.NodeModeValidator, params.NodeModeSeed,
		params.NodeModeFull, params.NodeModeArchive,
	} {
		t.Run(string(mode), func(t *testing.T) {
			home := t.TempDir()
			root, _ := cmd.NewRootCmd()
			root.SetArgs([]string{"init", "config-proof", "--home", home,
				"--chain-id", "config-proof-1", "--mode", string(mode)})
			require.NoError(t, svrcmd.Execute(root, home))

			want := config.DefaultConfig
			want.HTTPEnabled = mode == params.NodeModeFull || mode == params.NodeModeArchive
			want.WSEnabled = want.HTTPEnabled
			require.Equal(t, want, params.DefaultEVMConfigByMode(mode))
			path := filepath.Join(home, "config", "app.toml")
			proofConfigObserve(t, path, want)
		})
	}
}

func TestProofConfigProducerUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.toml")
	appTemplate, producer := cmd.InitAppConfig()
	require.Contains(t, appTemplate, config.ConfigTemplate)
	require.Equal(t, config.DefaultConfig, producer.EVM)
	proofConfigWrite(path, producer)
	proofConfigObserve(t, path, config.DefaultConfig)

	producer.EVM.BatchRequestLimit = 17
	producer.EVM.BatchResponseMaxSize = 1234567
	producer.EVM.MaxSubscriptionsLogs = 29
	producer.EVM.SendRawTxSyncTimeout = 1750 * time.Millisecond
	proofConfigWrite(path, producer)
	proofConfigObserve(t, path, producer.EVM)

	// Explicit zero values must survive transport, rather than restore defaults.
	producer.EVM.BatchRequestLimit = 0
	producer.EVM.BatchResponseMaxSize = 0
	producer.EVM.MaxSubscriptionsLogs = 0
	producer.EVM.SendRawTxSyncTimeout = 0
	proofConfigWrite(path, producer)
	proofConfigObserve(t, path, producer.EVM)

	// Reusing a real Viper is also uncached once it reopens the producer's file.
	options := proofConfigOptions(t, path)
	producer.EVM.BatchRequestLimit = 31
	proofConfigWrite(path, producer)
	require.NoError(t, options.ReadInConfig())
	observed, err := app.ReadEVMRPCConfig(options)
	require.NoError(t, err)
	require.Equal(t, producer.EVM, observed)

	// Missing keys still take the default path used when opening older configs.
	defaults, err := app.ReadEVMRPCConfig(viper.New())
	require.NoError(t, err)
	require.Equal(t, config.DefaultConfig, defaults)
}

func TestProofConfigMalformedProducer(t *testing.T) {
	for _, tc := range []struct {
		key, emitted, malformed string
	}{
		{"evm.batch_request_limit", "batch_request_limit = 1000", `batch_request_limit = "invalid"`},
		{"evm.batch_response_max_size", "batch_response_max_size = 25000000", `batch_response_max_size = "invalid"`},
		{"evm.max_subscriptions_logs", "max_subscriptions_logs = 10000", "max_subscriptions_logs = -1"},
		{"evm.send_raw_tx_sync_timeout", `send_raw_tx_sync_timeout = "2s"`, `send_raw_tx_sync_timeout = "invalid"`},
	} {
		t.Run(tc.key, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "app.toml")
			_, producer := cmd.InitAppConfig()
			proofConfigWrite(path, producer)
			emitted, err := os.ReadFile(path)
			require.NoError(t, err)
			before := []byte("\n" + tc.emitted + "\n")
			require.Equal(t, 1, bytes.Count(emitted, before))
			// Corrupt only the producer's value at the file boundary. TOML remains
			// valid, so refusal must come from the actual consumer's typed decoder.
			malformed := bytes.Replace(emitted, before, []byte("\n"+tc.malformed+"\n"), 1)
			require.NoError(t, os.WriteFile(path, malformed, 0600))
			for attempt := 0; attempt < 2; attempt++ {
				options := proofConfigOptions(t, path)
				_, err := app.ReadEVMRPCConfig(options)
				require.ErrorContains(t, err, tc.key)
				_, err = config.ReadConfig(options)
				require.ErrorContains(t, err, tc.key)
			}
			// Refusal must not poison a subsequent decode of repaired producer output.
			proofConfigWrite(path, producer)
			proofConfigObserve(t, path, producer.EVM)
		})
	}
}

func TestProofConfigNegativeTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.toml")
	producer := cmd.NewAppConfigForMode(srvconfig.DefaultConfig(), params.NodeModeFull)
	producer.EVM.SendRawTxSyncTimeout = -time.Second
	proofConfigWrite(path, producer)
	for attempt := 0; attempt < 2; attempt++ {
		_, err := app.ReadEVMRPCConfig(proofConfigOptions(t, path))
		require.ErrorContains(t, err, "evm.send_raw_tx_sync_timeout")
		require.ErrorContains(t, err, "non-negative")
	}
}
