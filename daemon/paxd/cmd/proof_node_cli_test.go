// Proves: NewRootCmd/SequencerCmd/NewCustomAppConfig -> paxd subprocess, JSON, seed file and app.toml -> sequencer.LoadReceiptKey/ReadConfig and wasm.ReadWasmConfig.
// Repairs root.go:initAppConfig discarding WASM.QueryGasLimit and
// addModuleInitFlags advertising a different query gas default from app_config.go.
// Start now returns decoder errors before constructing the application, rather
// than panicking in newApp or reaching genesis loading with malformed options.
// node/sequencer/config.go:AddStartFlags calls the receipt option
// sequencer.receipt-key-file; that existing spelling is asserted here.
package cmd

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	srvconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestProofNodeCli(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "paxd")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "..")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build the real paxd executable: %s", output)

	run := func(t *testing.T, home string, args ...string) (string, string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		args = append([]string{"--home", home}, args...)
		command := exec.CommandContext(ctx, binary, args...)
		// Keep caller configuration out of the real process's options.
		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "PAX_") && !strings.HasPrefix(env, "PAXD_") {
				command.Env = append(command.Env, env)
			}
		}
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		require.NoError(t, ctx.Err(), "paxd must terminate within its deadline")
		return stdout.String(), stderr.String(), err
	}

	t.Run("producer subprocess and receipt key consumer", func(t *testing.T) {
		home := t.TempDir()
		consensusPath := filepath.Join(home, "consensus-key.json")
		validator, err := privval.GenFilePV(consensusPath, filepath.Join(home, "consensus-state.json"), "")
		require.NoError(t, err)
		require.NoError(t, validator.Key.Save())
		var previous [32]byte
		for round := 0; round < 2; round++ {
			keyPath := filepath.Join(home, fmt.Sprintf("receipt-%d.key", round))
			stdout, stderr, err := run(t, home, "sequencer", "gen-receipt-key", "--output", keyPath)
			require.NoError(t, err, "%s", stderr)
			var result struct {
				PublicKey string `json:"public_key"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &result))
			public, err := hex.DecodeString(result.PublicKey)
			require.NoError(t, err)
			require.Len(t, public, ed25519.PublicKeySize)
			publicKey := [32]byte(public)
			require.NotEqual(t, previous, publicKey, "a fresh subprocess must generate a fresh key")
			previous = publicKey
			info, err := os.Stat(keyPath)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			seed, err := os.ReadFile(keyPath)
			require.NoError(t, err)
			require.NotContains(t, stdout+stderr, strings.TrimSpace(string(seed)))
			pins := []verify.SequencerAuthorization{{
				SequencerID: [32]byte{1}, PublicKey: publicKey,
				FirstBatchNumber: 1, LastBatchNumber: 10,
			}}
			// Both loads open the producer's file anew after the process has exited.
			for read := 0; read < 2; read++ {
				key, err := sequencer.LoadReceiptKey(keyPath, consensusPath, pins, 1)
				require.NoError(t, err)
				require.Equal(t, publicKey, key.PublicKey())
				id, err := key.KeyID(1)
				require.NoError(t, err)
				require.Equal(t, pins[0].SequencerID, id)
			}
			_, _, err = run(t, home, "sequencer", "gen-receipt-key", "--output", keyPath)
			require.Error(t, err, "the real CLI must refuse replacing an existing key")
			after, err := os.ReadFile(keyPath)
			require.NoError(t, err)
			require.Equal(t, seed, after)
			_, err = sequencer.LoadReceiptKey(keyPath, consensusPath, pins, 11)
			require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
			// Corrupt actual producer output; the consumer must re-read and refuse it.
			require.NoError(t, os.WriteFile(keyPath, seed[:len(seed)-2], 0600))
			_, err = sequencer.LoadReceiptKey(keyPath, consensusPath, pins, 1)
			require.ErrorIs(t, err, sequencer.ErrReceiptKeyFile)
		}
		stdout, stderr, err := run(t, home, "sequencer", "gen-receipt-key")
		require.Error(t, err)
		require.Empty(t, stdout)
		require.Contains(t, stderr, "--output")
	})

	t.Run("root and server registration through executable", func(t *testing.T) {
		home := t.TempDir()
		for round := 0; round < 2; round++ {
			stdout, stderr, err := run(t, home, "--help")
			require.NoError(t, err, "%s", stderr)
			for _, command := range []string{"sequencer", "start", "add-genesis-receipt-key", "add-genesis-sequencer-leader"} {
				require.Contains(t, stdout, command)
			}
			stdout, stderr, err = run(t, home, "sequencer", "--help")
			require.NoError(t, err, "%s", stderr)
			require.Contains(t, stdout, "gen-receipt-key")
			stdout, stderr, err = run(t, home, "start", "--help")
			require.NoError(t, err, "%s", stderr)
			for _, name := range []string{
				sequencer.FlagEnable, sequencer.FlagMode, sequencer.FlagWALDir,
				sequencer.FlagReceiptKeyFile, "wasm.memory_cache_size",
				"wasm.query_gas_limit", "wasm.simulation_gas_limit",
			} {
				require.Contains(t, stdout, "--"+name)
			}
		}
	})

	t.Run("start flags cross into real config consumers", func(t *testing.T) {
		home := t.TempDir()
		args := []string{
			"--sequencer.enable", "--sequencer.mode", "microbatch",
			"--sequencer.wal-dir", filepath.Join(home, "wal"),
			"--sequencer.receipt-key-file", filepath.Join(home, "receipt.key"),
			"--sequencer.replication-timeout", "1s", "--sequencer.lease-duration", "3s",
			"--sequencer.microbatch-window", "5ms", "--sequencer.microbatch-max-count", "8",
			"--sequencer.nonce-queue-max-per-sender", "4", "--sequencer.nonce-queue-max-total", "16",
			"--wasm.memory_cache_size", "17", "--wasm.query_gas_limit", "456789",
			"--wasm.simulation_gas_limit", "987654",
		}
		for round := 0; round < 2; round++ {
			root, _ := NewRootCmd()
			start, remaining, err := root.Find([]string{"start"})
			require.NoError(t, err)
			require.Empty(t, remaining)
			require.Equal(t, "300000", start.Flags().Lookup("wasm.query_gas_limit").DefValue)
			require.NoError(t, start.ParseFlags(args))
			options := viper.New()
			require.NoError(t, options.BindPFlags(start.Flags()))
			seq, err := sequencer.ReadConfig(options)
			require.NoError(t, err)
			require.True(t, seq.Enable)
			require.Equal(t, sequencer.ModeMicroBatch, seq.Mode)
			require.Equal(t, filepath.Join(home, "wal"), seq.WALDir)
			require.Equal(t, filepath.Join(home, "receipt.key"), seq.ReceiptKeyFile)
			require.Equal(t, 3*time.Second, seq.LeaseDuration)
			require.Equal(t, 5*time.Millisecond, seq.MicroBatchWindow)
			require.Equal(t, 8, seq.MicroBatchMaxCount)
			wasmConfig, err := wasm.ReadWasmConfig(options)
			require.NoError(t, err)
			require.Equal(t, uint32(17), wasmConfig.MemoryCacheSize)
			require.Equal(t, uint64(456789), wasmConfig.SmartQueryGasLimit)
			require.NotNil(t, wasmConfig.SimulationGasLimit)
			require.Equal(t, uint64(987654), *wasmConfig.SimulationGasLimit)
		}
		// Exercise the same decoders through the real start handler. These errors
		// must be returned before startup attempts to load an uninitialised genesis.
		for _, tc := range []struct {
			args    []string
			refusal string
		}{
			{[]string{"--sequencer.mode", "malformed"}, sequencer.ErrConfigMode.Error()},
			{[]string{"--sequencer.wal-dir", ""}, sequencer.ErrConfigWALDir.Error()},
			{[]string{"--sequencer.receipt-key-file", ""}, sequencer.ErrConfigReceiptKeyFile.Error()},
			{[]string{"--wasm.simulation_gas_limit", "malformed"}, "invalid wasm configuration"},
		} {
			commandArgs := append([]string{"start"}, args...)
			commandArgs = append(commandArgs, tc.args...)
			stdout, stderr, err := run(t, home, commandArgs...)
			require.Error(t, err)
			require.Contains(t, stdout+stderr, tc.refusal)
			require.NotContains(t, stdout+stderr, "panic:")
		}
		// The real server pre-run rendered these CLI values into app.toml.
		// Reopen that subprocess output independently on each observation.
		for read := 0; read < 2; read++ {
			emitted := viper.New()
			emitted.SetConfigFile(filepath.Join(home, "config", "app.toml"))
			require.NoError(t, emitted.ReadInConfig())
			observed, err := wasm.ReadWasmConfig(emitted)
			require.NoError(t, err)
			require.Equal(t, uint32(17), observed.MemoryCacheSize)
			require.Equal(t, uint64(456789), observed.SmartQueryGasLimit)
			require.NotNil(t, observed.SimulationGasLimit)
			require.Equal(t, uint64(987654), *observed.SimulationGasLimit)
		}
	})

	t.Run("custom app config crosses TOML and executable", func(t *testing.T) {
		base := srvconfig.DefaultConfig()
		base.MinGasPrices = "0.25uhpx"
		evm := evmrpcconfig.DefaultConfig
		config := NewCustomAppConfig(base, evm)
		require.Equal(t, *base, config.Config)
		require.Equal(t, evm, config.EVM)
		require.Equal(t, uint64(300000), config.WASM.QueryGasLimit)
		config.WASM = WASMConfig{MemoryCacheSize: 23, QueryGasLimit: 654321, SimulationGasLimit: "765432"}
		appTemplate, _ := initAppConfig()
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0700))
		path := filepath.Join(home, "config", "app.toml")
		srvconfig.SetConfigTemplate(appTemplate)
		srvconfig.WriteConfigFile(path, config)
		for round := 0; round < 2; round++ {
			options := viper.New()
			options.SetConfigFile(path)
			require.NoError(t, options.ReadInConfig())
			var decoded CustomAppConfig
			require.NoError(t, options.Unmarshal(&decoded))
			require.Equal(t, config.WASM, decoded.WASM)
			observed, err := wasm.ReadWasmConfig(options)
			require.NoError(t, err)
			require.Equal(t, config.WASM.MemoryCacheSize, observed.MemoryCacheSize)
			require.Equal(t, config.WASM.QueryGasLimit, observed.SmartQueryGasLimit)
			require.NotNil(t, observed.SimulationGasLimit)
			require.Equal(t, uint64(765432), *observed.SimulationGasLimit)
			// The subprocess reads the producer's app.toml in its inherited pre-run.
			_, stderr, err := run(t, home, "sequencer", "gen-receipt-key", "--output", filepath.Join(home, fmt.Sprintf("config-read-%d.key", round)))
			require.NoError(t, err, "%s", stderr)
		}
		config.WASM.SimulationGasLimit = "malformed"
		srvconfig.WriteConfigFile(path, config)
		options := viper.New()
		options.SetConfigFile(path)
		require.NoError(t, options.ReadInConfig())
		_, err := wasm.ReadWasmConfig(options)
		require.Error(t, err)
		stdout, stderr, err := run(t, home, "start")
		require.Error(t, err)
		require.Contains(t, stdout+stderr, "invalid wasm configuration")
		require.NotContains(t, stdout+stderr, "panic:")
	})
}
