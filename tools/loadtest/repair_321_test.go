package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const repair321SettingsMarker = "repair_321_effective_settings="

// Run the real main/Cobra handler in an isolated process. The wrapper only
// observes the settings consumed by that handler, preserving its returned error.
func repair321CLIChild(t *testing.T) bool {
	t.Helper()
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator == -1 {
		return false
	}
	rootCmd.SetArgs(os.Args[separator+1:])
	handler := rootCmd.RunE
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := handler(cmd, args)
		encoded, encodeErr := json.Marshal(config.ResolveSettings())
		require.NoError(t, encodeErr)
		fmt.Printf("%s%s\n", repair321SettingsMarker, encoded)
		return err
	}
	main()
	os.Exit(0)
	return true
}

func repair321Profile() config.LoadConfig {
	return config.LoadConfig{
		ChainID:   125,
		Endpoints: []string{"http://127.0.0.1:1"},
		Scenarios: []config.Scenario{{
			Name: scenarios.EVMTransfer, Weight: 1,
			Accounts: &config.AccountConfig{Accounts: 2},
		}},
	}
}

type repair321Result struct {
	stats    *stats.FinalStats
	settings config.Settings
	output   string
	exit     int
}

// The producer is LoadConfig's real JSON codec; the consumer is the real CLI,
// generator, dry-run sender, collector, JSON reporter and main's exit handling.
func repair321RunCLI(t *testing.T, profile config.LoadConfig, args ...string) repair321Result {
	t.Helper()
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.json")
	reportPath := filepath.Join(dir, "report.json")
	encoded, err := json.Marshal(profile)
	require.NoError(t, err)
	if profile.Settings == nil {
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &fields))
		require.NotContains(t, fields, "settings", "producer must actually omit optional settings")
	}
	require.NoError(t, os.WriteFile(profilePath, encoded, 0o600))
	binary, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	commandArgs := []string{
		"-test.run=^" + strings.Split(t.Name(), "/")[0] + "$", "--",
		"--config", profilePath, "--dry-run", "--stats-interval", "50ms",
		"--metricsListenAddr", "127.0.0.1:0", "--report-path", reportPath,
	}
	commandArgs = append(commandArgs, args...)
	cmd := exec.CommandContext(ctx, binary, commandArgs...)
	pipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = cmd.Stdout
	require.NoError(t, cmd.Start())

	// Stop only after signal handling is installed and the real collector has
	// logged completed transactions. No RPC endpoint or synthetic stats are used.
	scanner := bufio.NewScanner(pipe)
	var output bytes.Buffer
	ready, stopped := false, false
	completedTxs := regexp.MustCompile(`throughput tps=[^,]+, txs=[1-9][0-9]*,`)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(&output, line)
		if strings.Contains(line, "Dry-run mode: Simulating requests") {
			ready = true
		}
		if ready && !stopped && completedTxs.MatchString(line) {
			require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
			stopped = true
		}
	}
	waitErr := cmd.Wait()
	require.NoError(t, scanner.Err(), "%s", output.String())
	require.NoError(t, ctx.Err(), "CLI timed out: %s", output.String())
	require.True(t, ready, "CLI did not reach execution: %s", output.String())
	require.True(t, stopped, "CLI did not collect transactions: %s", output.String())
	if waitErr != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, waitErr, &exitErr, "%s", output.String())
	}

	data, err := os.ReadFile(reportPath)
	require.NoError(t, err, "real CLI must write its JSON report before exiting: %s", output.String())
	var finalStats stats.FinalStats
	require.NoError(t, json.Unmarshal(data, &finalStats))
	require.Positive(t, finalStats.OverallPerformance.TotalTransactions)
	require.Positive(t, finalStats.OverallPerformance.AverageTPS)
	require.Equal(t, finalStats.LoadTestStatistics.TotalTxs, finalStats.OverallPerformance.TotalTransactions)
	require.Equal(t, finalStats.ScenarioDistribution[scenarios.EVMTransfer], finalStats.OverallPerformance.TotalTransactions)
	endpoint := finalStats.EndpointPerformance[profile.Endpoints[0]]
	require.Positive(t, endpoint.SampleCount)
	require.Positive(t, endpoint.LatencyP99)
	require.Contains(t, output.String(), "Received shutdown signal, stopping gracefully")
	require.NotContains(t, output.String(), "Failed to send transaction")
	require.NotContains(t, output.String(), "dispatcher: context canceled")

	var effective config.Settings
	settingsFound := false
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.HasPrefix(line, repair321SettingsMarker) {
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, repair321SettingsMarker)), &effective))
			settingsFound = true
		}
	}
	require.True(t, settingsFound, "real CLI did not finish its handler: %s", output.String())
	return repair321Result{stats: &finalStats, settings: effective, output: output.String(), exit: cmd.ProcessState.ExitCode()}
}

func TestRepair_321_ProfileBounds(t *testing.T) {
	if repair321CLIChild(t) {
		return
	}
	for _, tc := range []struct {
		name   string
		bounds config.BoundsConfig
	}{
		{name: "average TPS", bounds: config.BoundsConfig{MinAverageTPS: 1e12}},
		{name: "exact uint64 total", bounds: config.BoundsConfig{MinTotalTxs: math.MaxUint64}},
		// A negative nonzero maximum is supported by the existing bounds API
		// and guarantees a violation independently of scheduler timing.
		{name: "p99 latency", bounds: config.BoundsConfig{MaxP99LatencyMs: -1}},
		{name: "all bounds pass", bounds: config.BoundsConfig{MinAverageTPS: 0.000001, MinTotalTxs: 1, MaxP99LatencyMs: 60000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := repair321Profile()
			profile.Settings = &config.Settings{TPS: 20}
			profile.Bounds = &tc.bounds
			got := repair321RunCLI(t, profile)
			require.Equal(t, tc.bounds.MinAverageTPS, got.settings.MinTPS)
			require.Equal(t, tc.bounds.MinTotalTxs, got.settings.MinTotalTxs)
			require.Equal(t, config.Duration(time.Duration(tc.bounds.MaxP99LatencyMs)*time.Millisecond), got.settings.MaxP99Latency)
			bounds := stats.Bounds{MinAverageTPS: tc.bounds.MinAverageTPS, MinTotalTxs: tc.bounds.MinTotalTxs, MaxP99Latency: time.Duration(tc.bounds.MaxP99LatencyMs) * time.Millisecond}
			violations := bounds.Check(got.stats)
			if tc.name == "all bounds pass" {
				require.Empty(t, violations)
				require.NotContains(t, got.output, ErrPerformanceBounds.Error())
				require.Equal(t, 0, got.exit, "%s", got.output)
			} else {
				require.Len(t, violations, 1)
				require.Contains(t, got.output, ErrPerformanceBounds.Error())
				v := violations[0]
				require.Contains(t, got.output, fmt.Sprintf("%s: limit=%s observed=%s", v.Bound, v.Limit, v.Observed))
				require.Equal(t, 1, got.exit, "%s", got.output)
			}
		})
	}
}

func TestRepair_321_Precedence(t *testing.T) {
	if repair321CLIChild(t) {
		return
	}
	passing := config.Settings{MinTPS: 0.000001, MaxP99Latency: config.Duration(time.Minute), MinTotalTxs: 1}
	for _, tc := range []struct {
		name        string
		settings    config.Settings
		args        []string
		want        config.Settings
		wantFailure bool
	}{
		{name: "nonzero settings override profile", settings: passing, want: passing},
		{name: "nonzero flags override settings and profile", settings: config.Settings{MinTPS: 2e12, MaxP99Latency: -1, MinTotalTxs: math.MaxUint64 - 1},
			args: []string{"--min-tps=0.000001", "--max-p99-latency=1m", "--min-total-txs=1"}, want: passing},
		{name: "explicit zero flags disable profile and settings", settings: config.Settings{MinTPS: 2e12, MaxP99Latency: -1, MinTotalTxs: math.MaxUint64 - 1},
			args: []string{"--min-tps=0", "--max-p99-latency=0s", "--min-total-txs=0"}},
		{name: "one flag leaves other profile bounds enforced", args: []string{"--min-tps=0"},
			want: config.Settings{MaxP99Latency: config.Duration(-time.Millisecond), MinTotalTxs: math.MaxUint64}, wantFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := repair321Profile()
			profile.Settings = &tc.settings
			profile.Bounds = &config.BoundsConfig{MinAverageTPS: 1e12, MaxP99LatencyMs: -1, MinTotalTxs: math.MaxUint64}
			got := repair321RunCLI(t, profile, tc.args...)
			require.Equal(t, tc.want.MinTPS, got.settings.MinTPS)
			require.Equal(t, tc.want.MaxP99Latency, got.settings.MaxP99Latency)
			require.Equal(t, tc.want.MinTotalTxs, got.settings.MinTotalTxs)
			if tc.wantFailure {
				require.NotContains(t, got.output, "min_average_tps:")
				require.Contains(t, got.output, "min_total_txs: limit="+strconv.FormatUint(math.MaxUint64, 10))
				require.Contains(t, got.output, "max_p99_latency:"+profile.Endpoints[0])
				require.Equal(t, 1, got.exit, "%s", got.output)
			} else {
				require.NotContains(t, got.output, ErrPerformanceBounds.Error())
				require.Equal(t, 0, got.exit, "%s", got.output)
			}
		})
	}
}

func TestRepair_321_OptionalSettings(t *testing.T) {
	if repair321CLIChild(t) {
		return
	}
	for _, tc := range []struct {
		name     string
		settings *config.Settings
		bounds   *config.BoundsConfig
	}{
		{name: "omitted settings"},
		{name: "empty settings", settings: &config.Settings{}},
		{name: "omitted settings with profile bound", bounds: &config.BoundsConfig{MinTotalTxs: math.MaxUint64}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := repair321Profile()
			profile.Settings, profile.Bounds = tc.settings, tc.bounds
			got := repair321RunCLI(t, profile)
			defaults := config.DefaultSettings()
			require.Equal(t, defaults.Workers, got.settings.Workers)
			require.Equal(t, defaults.BufferSize, got.settings.BufferSize)
			require.Equal(t, defaults.TPS, got.settings.TPS)
			require.Equal(t, defaults.TargetGas, got.settings.TargetGas)
			require.Equal(t, defaults.NumBlocksToWrite, got.settings.NumBlocksToWrite)
			require.True(t, got.settings.DryRun)
			require.Equal(t, config.Duration(50*time.Millisecond), got.settings.StatsInterval)
			require.NotContains(t, got.output, "config settings are required")
			if tc.bounds != nil {
				require.Equal(t, tc.bounds.MinTotalTxs, got.settings.MinTotalTxs)
				require.Contains(t, got.output, "min_total_txs: limit="+strconv.FormatUint(math.MaxUint64, 10))
				require.Equal(t, 1, got.exit, "%s", got.output)
			} else {
				require.NotContains(t, got.output, ErrPerformanceBounds.Error())
				require.Equal(t, 0, got.exit, "%s", got.output)
			}
		})
	}
}
