package config

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

func TestConfigTemplateCoversEveryFlag(t *testing.T) {
	flags := []string{
		flagBatchRequestLimit,
		flagBatchResponseMaxSize,
		flagSendRawTxSyncTimeout,
		flagMaxSubscriptionsLogs,
		flagHTTPAddress,
		flagHTTPEnabled,
		flagHTTPPort,
		flagWSEnabled,
		flagWSPort,
		flagReadTimeout,
		flagReadHeaderTimeout,
		flagWriteTimeout,
		flagIdleTimeout,
		flagSimulationGasLimit,
		flagSimulationEVMTimeout,
		flagCORSOrigins,
		flagWSOrigins,
		flagFilterTimeout,
		flagMaxTxPoolTxs,
		flagSlow,
		flagDenyList,
		flagMaxLogNoBlock,
		flagMaxBlocksForLog,
		flagMaxSubscriptionsNewHead,
		flagEnableTestAPI,
		flagEnableUnsafeKeyringRPC,
		flagMaxConcurrentTraceCalls,
		flagMaxConcurrentSimulationCalls,
		flagMaxTraceLookbackBlocks,
		flagTraceTimeout,
		flagEnableParallelizedBlockTrace,
		flagRPCStatsInterval,
		flagWorkerPoolSize,
		flagWorkerQueueSize,
		flagEVMLegacyPaxApis,
		flagTraceBakeEnabled,
		flagTraceBakeWorkers,
		flagTraceBakeQueueSize,
		flagTraceBakeTracers,
		flagTraceBakeWindowBlocks,
		flagTraceBakeUseSnapshot,
		flagTraceBakeSnapshotWindow,
		flagIPRateLimitRPS,
		flagIPRateLimitBurst,
	}

	tmpl, err := template.New("evm").Parse(ConfigTemplate)
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, tmpl.Execute(&rendered, struct{ EVM Config }{EVM: DefaultConfig}))

	var decoded map[string]map[string]interface{}
	_, err = toml.NewDecoder(&rendered).Decode(&decoded)
	require.NoError(t, err)
	require.Contains(t, decoded, "evm")
	keys := decoded["evm"]

	for _, flag := range flags {
		t.Run(flag, func(t *testing.T) {
			key := strings.TrimPrefix(flag, "evm.")
			if _, ok := keys[key]; !ok {
				t.Errorf("ConfigTemplate does not render a key for flag %q", flag)
			}
		})
	}
}
