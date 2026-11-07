package cmd

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/stretchr/testify/require"
)

func TestStartCommandRegistersWasmFlags(t *testing.T) {
	rootCmd, _ := NewRootCmd()
	startCmd, args, err := rootCmd.Find([]string{"start"})
	require.NoError(t, err)
	require.Equal(t, "start", startCmd.Name())
	require.Empty(t, args)

	for _, name := range []string{
		"wasm.memory_cache_size",
		"wasm.query_gas_limit",
		"wasm.simulation_gas_limit",
	} {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, startCmd.Flags().Lookup(name), "paxd start must define --%s", name)
		})
	}
}

func TestStartCommandRegistersSequencerFlags(t *testing.T) {
	rootCmd, _ := NewRootCmd()
	startCmd, args, err := rootCmd.Find([]string{"start"})
	require.NoError(t, err)
	require.Equal(t, "start", startCmd.Name())
	require.Empty(t, args)

	for _, name := range []string{
		sequencer.FlagEnable,
		sequencer.FlagMode,
		sequencer.FlagWALDir,
		sequencer.FlagReceiptKeyFile,
		"sequencer.receipt-forward-url",
		"sequencer.role",
		sequencer.FlagStandbyAddress,
		sequencer.FlagReplicationTimeout,
		sequencer.FlagMicroBatchWindow,
		sequencer.FlagMicroBatchMaxCount,
		sequencer.FlagWALRetainBlocks,
		sequencer.FlagNonceQueueMaxPerSender,
		sequencer.FlagNonceQueueMaxTotal,
		sequencer.FlagLeaseDuration,
	} {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, startCmd.Flags().Lookup(name), "paxd start must define --%s", name)
		})
	}
}
