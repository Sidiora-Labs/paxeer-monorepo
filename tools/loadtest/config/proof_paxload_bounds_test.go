// Proves: config.LoadConfig.Bounds and config.Settings -> encoding/json and Viper keys -> config.MergeBounds, config.LoadSettings and config.ResolveSettings.
//
// The consumer runs in this built test binary as a subprocess, using the exported
// configuration API and the real JSON/Viper transport. Successive loads in the
// same process exercise fresh decoding, replacement and flag precedence.
// Repairs: config.go::BoundsConfig.UnmarshalJSON refuses millisecond latency
// overflow before MergeBounds can wrap it; settings.go::LoadSettings preserves
// uint64 transaction bounds that Viper's float64 JSON decoding used to round.
// The CLI now calls MergeBounds before LoadSettings/ResolveSettings.
// TestProofPaxloadBoundsCallerGap retains the historical missing-merge
// characterization and also drives the shipped profile through the real CLI.
package config_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const proofBoundsChild = "PAXLOAD_BOUNDS_PROOF_CHILD"

func proofBoundsCommand(t *testing.T, args []string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "paxload"}
	cmd.Flags().Duration("stats-interval", 0, "Stats interval")
	cmd.Flags().Int("buffer-size", 0, "Buffer size")
	cmd.Flags().Float64("tps", 0, "TPS")
	cmd.Flags().Bool("dry-run", false, "Dry run")
	cmd.Flags().Bool("debug", false, "Debug")
	cmd.Flags().Bool("track-receipts", false, "Track receipts")
	cmd.Flags().Bool("track-blocks", false, "Track blocks")
	cmd.Flags().Bool("prewarm", false, "Prewarm")
	cmd.Flags().Bool("track-user-latency", false, "Track user latency")
	cmd.Flags().Int("workers", 0, "Workers")
	cmd.Flags().Bool("ramp-up", false, "Ramp up")
	cmd.Flags().String("report-path", "", "Report path")
	cmd.Flags().Float64("min-tps", 0, "Minimum average TPS")
	cmd.Flags().Duration("max-p99-latency", 0, "Maximum p99 latency")
	cmd.Flags().Uint64("min-total-txs", 0, "Minimum total transactions")
	cmd.Flags().String("txs-dir", "", "Transactions directory")
	cmd.Flags().Uint64("target-gas", 10_000_000, "Target gas")
	cmd.Flags().Int("num-blocks-to-write", 100, "Blocks to write")
	require.NoError(t, cmd.ParseFlags(args))
	require.NoError(t, config.InitializeViper(cmd))
	return cmd
}

func proofBoundsConsume(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	proofBoundsCommand(t, args)
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	count := 0
	for {
		var cfg config.LoadConfig
		err := decoder.Decode(&cfg)
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "decode producer config")
		require.NotNil(t, cfg.Settings)
		config.MergeBounds(&cfg, cfg.Settings)
		require.NoError(t, config.LoadSettings(cfg.Settings))
		require.NoError(t, encoder.Encode(config.ResolveSettings()))
		count++
	}
	require.Positive(t, count)
}

func proofBoundsRun(t *testing.T, input []byte, args []string) ([]byte, error) {
	t.Helper()
	binary, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	commandArgs := append([]string{"-test.run=^TestProofPaxloadBounds$", "--"}, args...)
	cmd := exec.CommandContext(ctx, binary, commandArgs...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, proofBoundsChild+"=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, proofBoundsChild+"=1")
	cmd.Stdin = bytes.NewReader(input)
	return cmd.CombinedOutput()
}

func TestProofPaxloadBounds(t *testing.T) {
	if os.Getenv(proofBoundsChild) == "1" {
		proofBoundsConsume(t)
		return
	}

	profile := config.BoundsConfig{MinAverageTPS: 400.25, MaxP99LatencyMs: 250, MinTotalTxs: 1<<53 + 1}
	configured := config.Settings{MinTPS: 123.75, MaxP99Latency: config.Duration(1500 * time.Millisecond), MinTotalTxs: math.MaxUint64}
	changed := config.BoundsConfig{MinAverageTPS: 800.5, MaxP99LatencyMs: 125, MinTotalTxs: math.MaxUint64 - 1}
	fromProfile := config.Settings{MinTPS: profile.MinAverageTPS, MaxP99Latency: config.Duration(250 * time.Millisecond), MinTotalTxs: profile.MinTotalTxs}
	fromChanged := config.Settings{MinTPS: changed.MinAverageTPS, MaxP99Latency: config.Duration(125 * time.Millisecond), MinTotalTxs: changed.MinTotalTxs}

	for _, tc := range []struct {
		name        string
		settings    config.Settings
		args        []string
		want        config.Settings
		wantChanged config.Settings
		wantCleared config.Settings
	}{
		{name: "profile bounds", want: fromProfile, wantChanged: fromChanged},
		{name: "settings override profile", settings: configured, want: configured, wantChanged: configured},
		{name: "flags override settings and profile", settings: configured,
			args:        []string{"--min-tps", "901.25", "--max-p99-latency", "75ms", "--min-total-txs", "18446744073709551614"},
			want:        config.Settings{MinTPS: 901.25, MaxP99Latency: config.Duration(75 * time.Millisecond), MinTotalTxs: math.MaxUint64 - 1},
			wantChanged: config.Settings{MinTPS: 901.25, MaxP99Latency: config.Duration(75 * time.Millisecond), MinTotalTxs: math.MaxUint64 - 1},
			wantCleared: config.Settings{MinTPS: 901.25, MaxP99Latency: config.Duration(75 * time.Millisecond), MinTotalTxs: math.MaxUint64 - 1}},
		{name: "explicit zero flags disable bounds", settings: configured,
			args: []string{"--min-tps", "0", "--max-p99-latency", "0s", "--min-total-txs", "0"}},
		{name: "one flag overrides independently", args: []string{"--min-tps", "999.5"},
			want:        config.Settings{MinTPS: 999.5, MaxP99Latency: fromProfile.MaxP99Latency, MinTotalTxs: fromProfile.MinTotalTxs},
			wantChanged: config.Settings{MinTPS: 999.5, MaxP99Latency: fromChanged.MaxP99Latency, MinTotalTxs: fromChanged.MinTotalTxs},
			wantCleared: config.Settings{MinTPS: 999.5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := config.LoadConfig{Bounds: &profile, Settings: &tc.settings}
			var input bytes.Buffer
			encoder := json.NewEncoder(&input)
			// The same result must be observed twice, then a changed result and
			// an empty result must replace it without resetting Viper.
			require.NoError(t, encoder.Encode(producer))
			require.NoError(t, encoder.Encode(producer))
			producer.Bounds = &changed
			require.NoError(t, encoder.Encode(producer))
			producer.Bounds = nil
			producer.Settings = &config.Settings{}
			require.NoError(t, encoder.Encode(producer))
			output, err := proofBoundsRun(t, input.Bytes(), tc.args)
			require.NoError(t, err, "%s", output)
			decoder := json.NewDecoder(bytes.NewReader(output))
			for i, want := range []config.Settings{tc.want, tc.want, tc.wantChanged, tc.wantCleared} {
				var observed config.Settings
				require.NoError(t, decoder.Decode(&observed), "observation %d: %s", i, output)
				require.Equal(t, want.MinTPS, observed.MinTPS, "observation %d", i)
				require.Equal(t, want.MaxP99Latency, observed.MaxP99Latency, "observation %d", i)
				require.Equal(t, want.MinTotalTxs, observed.MinTotalTxs, "observation %d", i)
				require.Equal(t, config.DefaultSettings().Workers, observed.Workers)
			}
		})
	}
}

func TestProofPaxloadBoundsJSONKeys(t *testing.T) {
	producer := config.LoadConfig{
		Bounds:   &config.BoundsConfig{MinAverageTPS: 400.25, MaxP99LatencyMs: 250, MinTotalTxs: math.MaxUint64},
		Settings: &config.Settings{MinTPS: 123.75, MaxP99Latency: config.Duration(1500 * time.Millisecond), MinTotalTxs: math.MaxUint64},
	}
	encoded, err := json.Marshal(producer)
	require.NoError(t, err)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	for key, want := range map[string]map[string]string{
		"bounds":   {"minAverageTps": "400.25", "maxP99LatencyMs": "250", "minTotalTxs": "18446744073709551615"},
		"settings": {"minTps": "123.75", "maxP99Latency": `"1.5s"`, "minTotalTxs": "18446744073709551615"},
	} {
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(envelope[key], &fields))
		for field, value := range want {
			require.JSONEq(t, value, string(fields[field]), "%s.%s", key, field)
		}
	}
}

func TestProofPaxloadBoundsRefusesMalformedProducer(t *testing.T) {
	for _, millis := range []int64{math.MaxInt64/int64(time.Millisecond) + 1, math.MinInt64/int64(time.Millisecond) - 1} {
		producer := config.LoadConfig{Bounds: &config.BoundsConfig{MaxP99LatencyMs: millis}, Settings: &config.Settings{}}
		encoded, err := json.Marshal(producer)
		require.NoError(t, err)
		output, err := proofBoundsRun(t, encoded, nil)
		require.Error(t, err)
		require.Contains(t, string(output), "decode producer config")
		require.Contains(t, string(output), "maxP99LatencyMs is outside the time.Duration range")
	}

	producer := config.LoadConfig{Settings: &config.Settings{MaxP99Latency: config.Duration(time.Second), MinTotalTxs: 1}}
	encoded, err := json.Marshal(producer)
	require.NoError(t, err)
	for _, malformed := range [][]byte{
		bytes.Replace(encoded, []byte(`"1s"`), []byte(`"not-a-duration"`), 1),
		bytes.Replace(encoded, []byte(`"minTotalTxs":1`), []byte(`"minTotalTxs":-1`), 1),
	} {
		require.NotEqual(t, encoded, malformed)
		output, err := proofBoundsRun(t, malformed, nil)
		require.Error(t, err)
		require.Contains(t, string(output), "decode producer config")
	}
}

func TestProofPaxloadBoundsLatencyBoundary(t *testing.T) {
	for _, millis := range []int64{math.MaxInt64 / int64(time.Millisecond), math.MinInt64 / int64(time.Millisecond)} {
		producer := config.LoadConfig{Bounds: &config.BoundsConfig{MaxP99LatencyMs: millis}, Settings: &config.Settings{}}
		encoded, err := json.Marshal(producer)
		require.NoError(t, err)
		output, err := proofBoundsRun(t, encoded, nil)
		require.NoError(t, err, "%s", output)
		var observed config.Settings
		require.NoError(t, json.NewDecoder(bytes.NewReader(output)).Decode(&observed))
		require.Equal(t, time.Duration(millis)*time.Millisecond, observed.MaxP99Latency.ToDuration())
	}
}

func TestProofPaxloadBoundsCallerGap(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	proofBoundsCommand(t, nil)
	producer := config.LoadConfig{Bounds: &config.BoundsConfig{MinAverageTPS: 400, MaxP99LatencyMs: 250, MinTotalTxs: 5000}, Settings: &config.Settings{}}
	encoded, err := json.Marshal(producer)
	require.NoError(t, err)
	var decoded config.LoadConfig
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	// Omitting the merge reproduces the historical runLoadTest caller gap.
	require.NoError(t, config.LoadSettings(decoded.Settings))
	observed := config.ResolveSettings()
	require.Zero(t, observed.MinTPS)
	require.Zero(t, observed.MaxP99Latency)
	require.Zero(t, observed.MinTotalTxs)
	// Calling the available merger delivers the profile through the same keys.
	config.MergeBounds(&decoded, decoded.Settings)
	require.NoError(t, config.LoadSettings(decoded.Settings))
	observed = config.ResolveSettings()
	require.Equal(t, float64(400), observed.MinTPS)
	require.Equal(t, 250*time.Millisecond, observed.MaxP99Latency.ToDuration())
	require.Equal(t, uint64(5000), observed.MinTotalTxs)
	t.Run("shipped profile fails after real collection", func(t *testing.T) { proofBoundsRunProfileCLI(t, false) })
	t.Run("explicit zero flags exit successfully", func(t *testing.T) { proofBoundsRunProfileCLI(t, true) })
}

const proofBoundsCLIMarker = "profile_cli_result="

type proofBoundsCLIResult struct {
	Settings          config.Settings `json:"settings"`
	PerformanceBounds bool            `json:"performanceBounds"`
}

// The existing test executable supplies process isolation without a Go subprocess
// toolchain. The handler is the same production Cobra command used by main.
func proofBoundsCLIChild(t *testing.T) bool {
	t.Helper()
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		cmd, err := cli.NewCommand()
		require.NoError(t, err)
		cmd.SetArgs(os.Args[i+1:])
		handler := cmd.RunE
		cmd.RunE = func(command *cobra.Command, args []string) error {
			err := handler(command, args)
			encoded, encodeErr := json.Marshal(proofBoundsCLIResult{config.ResolveSettings(), errors.Is(err, cli.ErrPerformanceBounds)})
			require.NoError(t, encodeErr)
			fmt.Printf("%s%s\n", proofBoundsCLIMarker, encoded)
			return err
		}
		if err := cmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	return false
}

func proofBoundsRunProfileCLI(t *testing.T, disable bool) {
	t.Helper()
	profilePath := filepath.Join("..", "profiles", "evm_transfer.json")
	profile, err := cli.LoadConfig(profilePath)
	require.NoError(t, err)
	require.NotNil(t, profile.Bounds)
	require.Positive(t, profile.Bounds.MinAverageTPS)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	binary, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := []string{"-test.run=^TestRepairM_816_ProfileCLI$", "--", "--config", profilePath,
		"--dry-run", "--tps=0.1", "--stats-interval=1s", "--metricsListenAddr=127.0.0.1:0", "--report-path", reportPath}
	if disable {
		args = append(args, "--min-tps=0", "--max-p99-latency=0s", "--min-total-txs=0")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	pipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = cmd.Stdout
	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	var output bytes.Buffer
	ready, stopped, observed := false, false, false
	var result proofBoundsCLIResult
	completed := regexp.MustCompile(`throughput tps=[^,]+, txs=[1-9][0-9]*,`)
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(&output, line)
		if strings.Contains(line, "Dry-run mode: Simulating requests") {
			ready = true
		}
		if ready && !stopped && completed.MatchString(line) {
			require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
			stopped = true
		}
		if strings.HasPrefix(line, proofBoundsCLIMarker) {
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, proofBoundsCLIMarker)), &result))
			observed = true
		}
	}
	waitErr := cmd.Wait()
	require.NoError(t, scanner.Err(), "%s", output.String())
	require.NoError(t, ctx.Err(), "CLI timed out: %s", output.String())
	require.True(t, ready, "%s", output.String())
	require.True(t, stopped, "no completed transactions: %s", output.String())
	require.True(t, observed, "production handler did not return: %s", output.String())
	data, err := os.ReadFile(reportPath)
	require.NoError(t, err, "%s", output.String())
	var report stats.FinalStats
	require.NoError(t, json.Unmarshal(data, &report))
	require.Positive(t, report.OverallPerformance.TotalTransactions)
	require.Positive(t, report.OverallPerformance.AverageTPS)
	require.Equal(t, report.LoadTestStatistics.TotalTxs, report.OverallPerformance.TotalTransactions)
	require.Equal(t, report.OverallPerformance.TotalTransactions, report.ScenarioDistribution[scenarios.EVMTransfer])
	require.Positive(t, report.EndpointPerformance[profile.Endpoints[0]].SampleCount)
	require.Less(t, report.OverallPerformance.AverageTPS, profile.Bounds.MinAverageTPS)
	require.NotContains(t, output.String(), "Failed to send transaction")
	require.Contains(t, output.String(), "Received shutdown signal, stopping gracefully")
	if disable {
		require.NoError(t, waitErr, "%s", output.String())
		require.Equal(t, 0, cmd.ProcessState.ExitCode())
		require.False(t, result.PerformanceBounds)
		require.Zero(t, result.Settings.MinTPS)
		require.Zero(t, result.Settings.MaxP99Latency)
		require.Zero(t, result.Settings.MinTotalTxs)
		require.NotContains(t, output.String(), cli.ErrPerformanceBounds.Error())
	} else {
		var exitErr *exec.ExitError
		require.ErrorAs(t, waitErr, &exitErr, "%s", output.String())
		require.Equal(t, 1, exitErr.ExitCode())
		require.True(t, result.PerformanceBounds, "handler must return ErrPerformanceBounds")
		require.Equal(t, profile.Bounds.MinAverageTPS, result.Settings.MinTPS)
		require.Equal(t, config.Duration(time.Duration(profile.Bounds.MaxP99LatencyMs)*time.Millisecond), result.Settings.MaxP99Latency)
		require.Equal(t, profile.Bounds.MinTotalTxs, result.Settings.MinTotalTxs)
		require.Contains(t, output.String(), cli.ErrPerformanceBounds.Error())
		require.Contains(t, output.String(), "min_average_tps: limit=5 observed=")
	}
}
