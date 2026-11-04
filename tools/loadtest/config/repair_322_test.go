package config_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

var repair322Completed = regexp.MustCompile(`throughput tps=[^,]+, txs=[1-9][0-9]*,`)

// Observe completed work from the real collector before cancelling the command.
// Cancellation exercises the same final-report writer as signal shutdown.
type repair322Output struct {
	mu        sync.Mutex
	data      bytes.Buffer
	cancel    context.CancelFunc
	completed bool
}

func (o *repair322Output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n, err := o.data.Write(p)
	if repair322Completed.Match(p) {
		o.completed = true
		o.cancel()
	}
	return n, err
}

func (o *repair322Output) snapshot() (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.data.String(), o.completed
}

func repair322RunCLI(t *testing.T, profile config.LoadConfig, flags ...string) error {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	// Real, isolated metric registries let successive commands execute in process.
	registerer, gatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() {
		prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registerer, gatherer
	})

	var requests atomic.Int64
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "dry-run must not send RPC requests", http.StatusBadRequest)
	}))
	defer endpoint.Close()
	profile.Endpoints = []string{endpoint.URL}
	profile.ChainID = 125
	profile.Scenarios = []config.Scenario{{
		Name: scenarios.EVMTransfer, Weight: 1,
		Accounts: &config.AccountConfig{Accounts: 2},
	}}
	// Produce the profile with the production JSON codec, then let Cobra's real
	// handler read the file, resolve settings and run the generator and reporter.
	data, err := json.Marshal(profile)
	require.NoError(t, err)
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	require.NoError(t, os.WriteFile(profilePath, data, 0o600))
	cmd, err := cli.NewCommand()
	require.NoError(t, err)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	args := []string{"--config", profilePath, "--dry-run", "--tps=20", "--stats-interval=10ms", "--metricsListenAddr=127.0.0.1:0"}
	cmd.SetArgs(append(args, flags...))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output := &repair322Output{cancel: cancel}
	previousOutput := log.Writer()
	log.SetOutput(output)
	defer log.SetOutput(previousOutput)
	err = cmd.ExecuteContext(ctx)
	text, completed := output.snapshot()
	require.True(t, completed, "CLI never collected a transaction: %s", text)
	require.ErrorIs(t, ctx.Err(), context.Canceled, "CLI timed out: %s", text)
	require.Zero(t, requests.Load(), "the CLI's dry-run must need no RPC service")
	return err
}

func TestRepair_322_ReportPathPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		top       bool
		settings  bool
		nested    bool
		flag      bool
		flagEmpty bool
		want      string
	}{
		{name: "top level with omitted settings", top: true, want: "top"},
		{name: "top level with empty settings", top: true, settings: true, want: "top"},
		{name: "settings only", settings: true, nested: true, want: "nested"},
		{name: "settings overrides top level", top: true, settings: true, nested: true, want: "nested"},
		{name: "flag overrides both profile paths", top: true, settings: true, nested: true, flag: true, want: "flag"},
		{name: "flag overrides top level with omitted settings", top: true, flag: true, want: "flag"},
		{name: "flag only", flag: true, want: "flag"},
		{name: "empty flag disables both profile paths", top: true, settings: true, nested: true, flag: true, flagEmpty: true},
		{name: "empty flag disables top level with omitted settings", top: true, flag: true, flagEmpty: true},
		{name: "no report requested"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			paths := map[string]string{
				"top":    filepath.Join(dir, "top.json"),
				"nested": filepath.Join(dir, "nested.json"),
				"flag":   filepath.Join(dir, "flag.json"),
			}
			sentinel := []byte("existing report must remain untouched")
			for key, path := range paths {
				if key != tc.want {
					require.NoError(t, os.WriteFile(path, sentinel, 0o600))
				}
			}
			var profile config.LoadConfig
			if tc.top {
				profile.ReportPath = paths["top"]
			}
			if tc.settings {
				profile.Settings = &config.Settings{}
			}
			if tc.nested {
				profile.Settings.ReportPath = paths["nested"]
			}
			var flags []string
			if tc.flag {
				value := paths["flag"]
				if tc.flagEmpty {
					value = ""
				}
				flags = []string{"--report-path=" + value}
			}
			err := repair322RunCLI(t, profile, flags...)
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, paths[tc.want], config.ResolveSettings().ReportPath)
			for key, path := range paths {
				data, err := os.ReadFile(path)
				require.NoError(t, err, "report path %s", key)
				if key != tc.want {
					require.Equal(t, sentinel, data, "lower-priority report path %s was written", key)
					continue
				}
				var report stats.FinalStats
				require.NoError(t, json.Unmarshal(data, &report))
				require.Positive(t, report.LoadTestStatistics.TotalTxs)
				require.Equal(t, report.LoadTestStatistics.TotalTxs, report.OverallPerformance.TotalTransactions)
				require.Equal(t, report.LoadTestStatistics.TotalTxs, report.ScenarioDistribution[scenarios.EVMTransfer])
				require.Len(t, report.EndpointPerformance, 1)
				for _, endpoint := range report.EndpointPerformance {
					require.Positive(t, endpoint.SampleCount)
				}
				expected, err := json.MarshalIndent(&report, "", "  ")
				require.NoError(t, err)
				require.Equal(t, expected, data, "CLI must write the real indented JSON report")
			}
		})
	}
}

func TestRepair_322_ReportWriteError(t *testing.T) {
	for _, source := range []string{"top", "nested", "flag"} {
		t.Run(source, func(t *testing.T) {
			dir := t.TempDir()
			missing := filepath.Join(dir, "missing", "report.json")
			lower := filepath.Join(dir, "lower.json")
			profile := config.LoadConfig{ReportPath: missing}
			var flags []string
			if source == "nested" {
				profile.ReportPath = lower
				profile.Settings = &config.Settings{ReportPath: missing}
			}
			if source == "flag" {
				profile.ReportPath = lower
				profile.Settings = &config.Settings{ReportPath: lower}
				flags = []string{"--report-path=" + missing}
			}
			err := repair322RunCLI(t, profile, flags...)
			require.ErrorIs(t, err, os.ErrNotExist)
			require.Contains(t, err.Error(), "write report "+missing+": ")
			var pathErr *os.PathError
			require.True(t, errors.As(err, &pathErr), "%v", err)
			require.Equal(t, missing, pathErr.Path)
			_, err = os.Stat(lower)
			require.ErrorIs(t, err, os.ErrNotExist, "a failed report write must not fall back to a lower-priority path")
		})
	}
}
