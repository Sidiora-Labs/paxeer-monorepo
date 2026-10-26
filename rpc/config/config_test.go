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

type opts struct {
	httpAddress                  interface{}
	httpEnabled                  interface{}
	httpPort                     interface{}
	wsEnabled                    interface{}
	wsPort                       interface{}
	readTimeout                  interface{}
	readHeaderTimeout            interface{}
	writeTimeout                 interface{}
	idleTimeout                  interface{}
	simulationGasLimit           interface{}
	simulationEVMTimeout         interface{}
	corsOrigins                  interface{}
	wsOrigins                    interface{}
	filterTimeout                interface{}
	checkTxTimeout               interface{}
	maxTxPoolTxs                 interface{}
	slow                         interface{}
	disableWatermark             interface{}
	denyList                     interface{}
	maxLogNoBlock                interface{}
	maxBlocksForLog              interface{}
	maxSubscriptionsNewHead      interface{}
	enableTestAPI                interface{}
	enableUnsafeKeyringRPC       interface{}
	maxConcurrentTraceCalls      interface{}
	maxConcurrentSimulationCalls interface{}
	maxTraceLookbackBlocks       interface{}
	traceTimeout                 interface{}
	enableParallelizedBlockTrace interface{}
	rpcStatsInterval             interface{}
	workerPoolSize               interface{}
	workerQueueSize              interface{}
	ipRateLimitRPS               interface{}
	ipRateLimitBurst             interface{}
}

func (o *opts) Get(k string) interface{} {
	if k == "evm.batch_request_limit" ||
		k == "evm.batch_response_max_size" ||
		k == "evm.send_raw_tx_sync_timeout" ||
		k == "evm.max_subscriptions_logs" {
		return nil
	}
	if k == "evm.http_address" {
		return o.httpAddress
	}
	if k == "evm.http_enabled" {
		return o.httpEnabled
	}
	if k == "evm.http_port" {
		return o.httpPort
	}
	if k == "evm.ws_enabled" {
		return o.wsEnabled
	}
	if k == "evm.ws_port" {
		return o.wsPort
	}
	if k == "evm.read_timeout" {
		return o.readTimeout
	}
	if k == "evm.read_header_timeout" {
		return o.readHeaderTimeout
	}
	if k == "evm.write_timeout" {
		return o.writeTimeout
	}
	if k == "evm.idle_timeout" {
		return o.idleTimeout
	}
	if k == "evm.simulation_gas_limit" {
		return o.simulationGasLimit
	}
	if k == "evm.simulation_evm_timeout" {
		return o.simulationEVMTimeout
	}
	if k == "evm.cors_origins" {
		return o.corsOrigins
	}
	if k == "evm.ws_origins" {
		return o.wsOrigins
	}
	if k == "evm.filter_timeout" {
		return o.filterTimeout
	}
	if k == "evm.checktx_timeout" {
		return o.checkTxTimeout
	}
	if k == "evm.max_tx_pool_txs" {
		return o.maxTxPoolTxs
	}
	if k == "evm.slow" {
		return o.slow
	}
	if k == "evm.disable_watermark" {
		return o.disableWatermark
	}
	if k == "evm.deny_list" {
		return o.denyList
	}
	if k == "evm.max_log_no_block" {
		return o.maxLogNoBlock
	}
	if k == "evm.max_blocks_for_log" {
		return o.maxBlocksForLog
	}
	if k == "evm.max_subscriptions_new_head" {
		return o.maxSubscriptionsNewHead
	}
	if k == "evm.enable_test_api" {
		return o.enableTestAPI
	}
	if k == "evm.enable_unsafe_keyring_rpc" {
		return o.enableUnsafeKeyringRPC
	}
	if k == "evm.max_concurrent_trace_calls" {
		return o.maxConcurrentTraceCalls
	}
	if k == "evm.max_concurrent_simulation_calls" {
		return o.maxConcurrentSimulationCalls
	}
	if k == "evm.max_trace_lookback_blocks" {
		return o.maxTraceLookbackBlocks
	}
	if k == "evm.trace_timeout" {
		return o.traceTimeout
	}
	if k == "evm.enable_parallelized_block_trace" {
		return o.enableParallelizedBlockTrace
	}
	if k == "evm.rpc_stats_interval" {
		return o.rpcStatsInterval
	}
	if k == "evm.worker_pool_size" {
		return o.workerPoolSize
	}
	if k == "evm.worker_queue_size" {
		return o.workerQueueSize
	}
	if k == "evm.enabled_legacy_pax_apis" {
		return nil
	}
	if k == "evm.trace_bake_enabled" ||
		k == "evm.trace_bake_workers" ||
		k == "evm.trace_bake_queue_size" ||
		k == "evm.trace_bake_tracers" ||
		k == "evm.trace_bake_window_blocks" ||
		k == "evm.trace_bake_use_snapshot" ||
		k == "evm.trace_bake_snapshot_window" {
		return nil
	}
	if k == "evm.ip_rate_limit_rps" {
		return o.ipRateLimitRPS
	}
	if k == "evm.ip_rate_limit_burst" {
		return o.ipRateLimitBurst
	}
	panic("unknown key")
}

// getDefaultOpts returns a valid opts struct with all required fields set
func getDefaultOpts() opts {
	return opts{
		"0.0.0.0",
		true,
		1,
		true,
		2,
		time.Duration(5),
		time.Duration(5),
		time.Duration(5),
		time.Duration(5),
		uint64(10),
		time.Duration(60),
		"",
		"",
		time.Duration(5),
		time.Duration(5),
		1000,
		false,
		false,
		make([]string, 0),
		20000,
		1000,
		10000,
		false,
		false,
		uint64(10),
		uint64(10),
		int64(100),
		30 * time.Second,
		false,
		10 * time.Second,
		32,
		1000,
		200.0,
		400,
	}
}

func readConfigTemplate(t *testing.T, cfg config.Config) config.Config {
	t.Helper()
	tmpl, err := template.New("evm").Parse(config.ConfigTemplate)
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, tmpl.Execute(&rendered, struct{ EVM config.Config }{EVM: cfg}))
	options := viper.New()
	options.SetConfigType("toml")
	require.NoError(t, options.ReadConfig(&rendered))
	result, err := config.ReadConfig(options)
	require.NoError(t, err)
	return result
}

func TestReadConfigBatchLimits(t *testing.T) {
	tests := []struct {
		name         string
		request      interface{}
		response     interface{}
		wantRequest  int
		wantResponse int
		wantError    bool
	}{
		{name: "defaults", wantRequest: 1000, wantResponse: 25000000},
		{name: "integers", request: 250, response: 5000000, wantRequest: 250, wantResponse: 5000000},
		{name: "strings", request: "500", response: "10000000", wantRequest: 500, wantResponse: 10000000},
		{name: "request only", request: 25, wantRequest: 25, wantResponse: 25000000},
		{name: "response only", response: 5000, wantRequest: 1000, wantResponse: 5000},
		{name: "zero", request: 0, response: 0},
		{name: "invalid request", request: "bad", wantError: true},
		{name: "invalid response", response: "bad", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := viper.New()
			if tt.request != nil {
				options.Set("evm.batch_request_limit", tt.request)
			}
			if tt.response != nil {
				options.Set("evm.batch_response_max_size", tt.response)
			}
			cfg, err := config.ReadConfig(options)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantRequest, cfg.BatchRequestLimit)
			require.Equal(t, tt.wantResponse, cfg.BatchResponseMaxSize)
			roundTrip := readConfigTemplate(t, cfg)
			require.Equal(t, tt.wantRequest, roundTrip.BatchRequestLimit)
			require.Equal(t, tt.wantResponse, roundTrip.BatchResponseMaxSize)
		})
	}
}

func TestReadConfigSendRawTxSyncTimeout(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		want      time.Duration
		wantError bool
	}{
		{name: "default", want: 2 * time.Second},
		{name: "duration string", value: "750ms", want: 750 * time.Millisecond},
		{name: "duration value", value: 3 * time.Second, want: 3 * time.Second},
		{name: "zero", value: "0s"},
		{name: "invalid duration", value: "bad", wantError: true},
		{name: "invalid type", value: []string{"2s"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := viper.New()
			if tt.value != nil {
				options.Set("evm.send_raw_tx_sync_timeout", tt.value)
			}
			cfg, err := config.ReadConfig(options)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, cfg.SendRawTxSyncTimeout)
			require.Equal(t, tt.want, readConfigTemplate(t, cfg).SendRawTxSyncTimeout)
		})
	}
}

func TestReadConfigMaxSubscriptionsLogs(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		want      uint64
		wantError bool
	}{
		{name: "default", want: 10000},
		{name: "integer", value: uint64(500), want: 500},
		{name: "string", value: "25000", want: 25000},
		{name: "zero", value: uint64(0)},
		{name: "negative", value: -1, wantError: true},
		{name: "invalid", value: "bad", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := viper.New()
			if tt.value != nil {
				options.Set("evm.max_subscriptions_logs", tt.value)
			}
			cfg, err := config.ReadConfig(options)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, cfg.MaxSubscriptionsLogs)
			require.Equal(t, uint64(10000), cfg.MaxSubscriptionsNewHead)
			require.Equal(t, tt.want, readConfigTemplate(t, cfg).MaxSubscriptionsLogs)
		})
	}
}

func TestReadConfig(t *testing.T) {
	goodOpts := getDefaultOpts()
	cfg, err := config.ReadConfig(&goodOpts)
	require.Nil(t, err)
	require.False(t, cfg.EnableParallelizedBlockTrace)
	badOpts := goodOpts
	badOpts.httpEnabled = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.httpPort = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.wsEnabled = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.wsPort = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.readTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.readHeaderTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.writeTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.idleTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.filterTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.simulationGasLimit = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.simulationEVMTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.corsOrigins = map[string]interface{}{}
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.wsOrigins = map[string]interface{}{}
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.checkTxTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.maxTxPoolTxs = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.slow = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.denyList = map[string]interface{}{}
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)
	badOpts = goodOpts
	badOpts.enableUnsafeKeyringRPC = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	// Test bad types for new trace config options
	badOpts = goodOpts
	badOpts.maxConcurrentTraceCalls = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	// Test bad types for new trace config options
	badOpts = goodOpts
	badOpts.maxConcurrentSimulationCalls = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	badOpts = goodOpts
	badOpts.maxTraceLookbackBlocks = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	badOpts = goodOpts
	badOpts.traceTimeout = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	badOpts = goodOpts
	badOpts.enableParallelizedBlockTrace = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	// Test bad types for worker pool config
	badOpts = goodOpts
	badOpts.workerPoolSize = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	badOpts = goodOpts
	badOpts.workerQueueSize = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	// Test bad types for rate limit config
	badOpts = goodOpts
	badOpts.ipRateLimitRPS = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

	badOpts = goodOpts
	badOpts.ipRateLimitBurst = "bad"
	_, err = config.ReadConfig(&badOpts)
	require.NotNil(t, err)

}

// Test worker pool configuration values
func TestReadConfigWorkerPool(t *testing.T) {
	tests := []struct {
		name            string
		workerPoolSize  interface{}
		workerQueueSize interface{}
		expectedWorkers int
		expectedQueue   int
	}{
		{
			name:            "custom values",
			workerPoolSize:  32,
			workerQueueSize: 1000,
			expectedWorkers: 32,
			expectedQueue:   1000,
		},
		{
			name:            "zero values (use defaults)",
			workerPoolSize:  0,
			workerQueueSize: 0,
			expectedWorkers: 0,
			expectedQueue:   0,
		},
		{
			name:            "only worker size",
			workerPoolSize:  48,
			workerQueueSize: 0,
			expectedWorkers: 48,
			expectedQueue:   0,
		},
		{
			name:            "only queue size",
			workerPoolSize:  0,
			workerQueueSize: 2000,
			expectedWorkers: 0,
			expectedQueue:   2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := getDefaultOpts()
			// Only set the fields we're testing
			opts.workerPoolSize = tt.workerPoolSize
			opts.workerQueueSize = tt.workerQueueSize

			cfg, err := config.ReadConfig(&opts)
			require.Nil(t, err)

			require.Equal(t, tt.expectedWorkers, cfg.WorkerPoolSize,
				"WorkerPoolSize mismatch")
			require.Equal(t, tt.expectedQueue, cfg.WorkerQueueSize,
				"WorkerQueueSize mismatch")
		})
	}
}

func TestReadConfigEnableParallelizedBlockTrace(t *testing.T) {
	opts := getDefaultOpts()
	opts.enableParallelizedBlockTrace = true

	cfg, err := config.ReadConfig(&opts)
	require.NoError(t, err)
	require.True(t, cfg.EnableParallelizedBlockTrace)
}
