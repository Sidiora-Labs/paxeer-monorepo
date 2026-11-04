package evmrpc_test

// Task 22.33.1298 repair record (spec/ is outside scope):
// metrics-ticker-panic configuration refusal -> TestRepairM_1298_ZeroInterval,
// TestRepairM_1298_NegativeIntervals; valid startup -> TestRepairM_1298_ValidStartup.
// Wiring request, domain 7: rpc/worker_pool_metrics.go must reject nonpositive
// intervals synchronously in StartMetricsPrinter before publishing its stop
// channel or launching a goroutine, using config.ErrInvalidRPCStatsInterval;
// rpc/rpcstack.go must propagate that error to direct startup callers.
// HTTPServer.Start currently uses its fixed metricsPrinterInterval, independently
// of config.RPCStatsInterval; these tests also drive the configured stats tracker.
// Baseline verify was not run: the explicit gate order requires implementation
// before hf-build and hf-done and prohibits a preliminary verification run.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func repairM1298Options(t *testing.T, producer rpcconfig.Config) *viper.Viper {
	t.Helper()
	tmpl, err := template.New("evm").Parse(rpcconfig.ConfigTemplate)
	require.NoError(t, err)
	var emitted bytes.Buffer
	require.NoError(t, tmpl.Execute(&emitted, struct{ EVM rpcconfig.Config }{producer}))
	path := filepath.Join(t.TempDir(), "app.toml")
	require.NoError(t, os.WriteFile(path, emitted.Bytes(), 0600))
	options := viper.New()
	options.SetConfigFile(path)
	require.NoError(t, options.ReadInConfig())
	require.True(t, options.IsSet("evm.rpc_stats_interval"))
	return options
}

func repairM1298Reject(t *testing.T, interval time.Duration) {
	t.Helper()
	for _, enabled := range []bool{false, true} {
		producer := rpcconfig.DefaultConfig
		producer.HTTPEnabled, producer.WSEnabled = enabled, enabled
		producer.RPCStatsInterval = interval
		options := repairM1298Options(t, producer)
		for _, decode := range []func() (rpcconfig.Config, error){
			func() (rpcconfig.Config, error) { return rpcconfig.ReadConfig(options) },
			func() (rpcconfig.Config, error) { return app.ReadEVMRPCConfig(options) },
		} {
			_, err := decode()
			require.ErrorIs(t, err, rpcconfig.ErrInvalidRPCStatsInterval)
			require.ErrorContains(t, err, "evm.rpc_stats_interval")
			require.ErrorContains(t, err, "at least 1ns")
		}
	}
}

func TestRepairM_1298_ZeroInterval(t *testing.T) {
	repairM1298Reject(t, 0)
}

func TestRepairM_1298_NegativeIntervals(t *testing.T) {
	for _, interval := range []time.Duration{-time.Nanosecond, -time.Second, time.Duration(-1 << 63)} {
		t.Run(interval.String(), func(t *testing.T) { repairM1298Reject(t, interval) })
	}
}

func TestRepairM_1298_DefaultAndMalformedIntervals(t *testing.T) {
	defaults, err := app.ReadEVMRPCConfig(viper.New())
	require.NoError(t, err)
	require.Equal(t, rpcconfig.DefaultConfig.RPCStatsInterval, defaults.RPCStatsInterval)
	require.Positive(t, defaults.RPCStatsInterval)
	for _, value := range []interface{}{"not-a-duration", "9223372036854775808ns"} {
		options := viper.New()
		options.Set("evm.rpc_stats_interval", value)
		_, err := app.ReadEVMRPCConfig(options)
		require.Error(t, err)
		require.ErrorContains(t, err, "evm.rpc_stats_interval")
	}
	for _, value := range []interface{}{0, int64(-1), -time.Second, "0s", "-1ns"} {
		options := viper.New()
		options.Set("evm.rpc_stats_interval", value)
		_, err := app.ReadEVMRPCConfig(options)
		require.ErrorIs(t, err, rpcconfig.ErrInvalidRPCStatsInterval)
	}
}

func TestRepairM_1298_ValidStartup(t *testing.T) {
	// This committed fixture uses a real in-memory app/keeper and local consensus
	// client, with no mock RPC dependencies or external node.
	for _, interval := range []time.Duration{time.Nanosecond, 250 * time.Millisecond, rpcconfig.DefaultConfig.RPCStatsInterval} {
		for _, ws := range []bool{false, true} {
			name := "http/"
			if ws {
				name = "websocket/"
			}
			t.Run(name+interval.String(), func(t *testing.T) {
				// Each startup owns its consensus subscription namespace.
				f := newRepair1291Fixture(t, "repair-1298", false)
				provider := func(int64) sdk.Context { return f.ctx }
				txProvider := func(int64) client.TxConfig { return f.app.GetTxConfig() }
				producer := f.config
				producer.RPCStatsInterval = interval
				cfg, err := app.ReadEVMRPCConfig(repairM1298Options(t, producer))
				require.NoError(t, err)
				require.Equal(t, producer, cfg)
				var service evmrpc.EVMServer
				if ws {
					service, err = evmrpc.NewEVMWebSocketServer(cfg, f.consensus, &f.app.EvmKeeper,
						f.app.BeginBlockKeepers, f.app.BaseApp, f.app.TracerAnteHandler, provider, txProvider, "", nil, nil)
				} else {
					service, err = evmrpc.NewEVMHTTPServer(cfg, f.consensus, &f.app.EvmKeeper,
						f.app.BeginBlockKeepers, f.app.BaseApp, f.app.TracerAnteHandler, provider, txProvider, "", nil)
				}
				require.NoError(t, err)
				t.Cleanup(service.Stop)
				server := service.(*evmrpc.HTTPServer)
				require.NoError(t, server.SetListenAddr("127.0.0.1", 0))
				require.NoError(t, server.Start())
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				var consumer *gethrpc.Client
				if ws {
					consumer, err = gethrpc.DialWebsocket(ctx, "ws://"+server.ListenAddr(), "http://localhost")
				} else {
					consumer, err = gethrpc.DialHTTP("http://" + server.ListenAddr())
				}
				require.NoError(t, err)
				t.Cleanup(consumer.Close)
				var chainID hexutil.Uint64
				require.NoError(t, consumer.CallContext(ctx, &chainID, "eth_chainId"))
				require.Equal(t, hexutil.Uint64(125), chainID)
			})
		}
	}
}
