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
// characterization; repair_321_test.go proves the actual CLI wiring.
package config_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/paxeer-network/pax-load/config"
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
}
