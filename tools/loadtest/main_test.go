package main

import (
	"errors"
	"testing"
	"time"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/stretchr/testify/require"
)

func TestEnforceBoundsReturnsErrPerformanceBounds(t *testing.T) {
	finalStats := &stats.FinalStats{
		OverallPerformance: stats.OverallPerformance{
			AverageTPS:        12.5,
			TotalTransactions: 50,
		},
		EndpointPerformance: map[string]stats.EndpointPerformance{
			"endpoint-b": {LatencyP99: 3 * time.Second},
			"endpoint-a": {LatencyP99: 2 * time.Second},
		},
	}
	t.Run("every violation is included", func(t *testing.T) {
		err := enforceBounds(config.Settings{
			MinTPS:        20,
			MaxP99Latency: config.Duration(time.Second),
			MinTotalTxs:   100,
		}, finalStats)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrPerformanceBounds))
		require.EqualError(t, err, "performance bounds violated: "+
			"min_average_tps: limit=20 observed=12.5; "+
			"min_total_txs: limit=100 observed=50; "+
			"max_p99_latency:endpoint-a: limit=1s observed=2s; "+
			"max_p99_latency:endpoint-b: limit=1s observed=3s")
	})
	t.Run("each bound is enforced independently", func(t *testing.T) {
		for _, settings := range []config.Settings{
			{MinTPS: 20},
			{MaxP99Latency: config.Duration(time.Second)},
			{MinTotalTxs: 100},
		} {
			require.ErrorIs(t, enforceBounds(settings, finalStats), ErrPerformanceBounds)
		}
	})
	t.Run("zero defaults disable bounds", func(t *testing.T) {
		require.NoError(t, enforceBounds(config.DefaultSettings(), finalStats))
	})
	t.Run("passing bounds", func(t *testing.T) {
		require.NoError(t, enforceBounds(config.Settings{
			MinTPS:        10,
			MaxP99Latency: config.Duration(4 * time.Second),
			MinTotalTxs:   40,
		}, finalStats))
	})
	t.Run("equality passes", func(t *testing.T) {
		require.NoError(t, enforceBounds(config.Settings{
			MinTPS:        12.5,
			MaxP99Latency: config.Duration(3 * time.Second),
			MinTotalTxs:   50,
		}, finalStats))
	})
}

func TestProofPaxloadBoundsShippedProfileMainExit(t *testing.T) {
	if repair321CLIChild(t) {
		return
	}
	profile, err := cli.LoadConfig("profiles/evm_transfer.json")
	require.NoError(t, err)
	require.NotNil(t, profile.Bounds)
	for _, disable := range []bool{false, true} {
		name := "profile regression exits one"
		if disable {
			name = "explicit zero flags exit zero"
		}
		t.Run(name, func(t *testing.T) {
			args := []string{"--tps=0.1", "--stats-interval=1s"}
			if disable {
				args = append(args, "--min-tps=0", "--max-p99-latency=0s", "--min-total-txs=0")
			}
			got := repair321RunCLI(t, *profile, args...)
			require.Less(t, got.stats.OverallPerformance.AverageTPS, profile.Bounds.MinAverageTPS)
			if disable {
				require.Equal(t, 0, got.exit, "%s", got.output)
				require.Zero(t, got.settings.MinTPS)
				require.Zero(t, got.settings.MaxP99Latency)
				require.Zero(t, got.settings.MinTotalTxs)
				require.NotContains(t, got.output, ErrPerformanceBounds.Error())
			} else {
				require.Equal(t, 1, got.exit, "%s", got.output)
				require.Equal(t, profile.Bounds.MinAverageTPS, got.settings.MinTPS)
				require.Equal(t, config.Duration(time.Duration(profile.Bounds.MaxP99LatencyMs)*time.Millisecond), got.settings.MaxP99Latency)
				require.Equal(t, profile.Bounds.MinTotalTxs, got.settings.MinTotalTxs)
				require.Contains(t, got.output, ErrPerformanceBounds.Error())
				require.ErrorIs(t, enforceBounds(got.settings, got.stats), ErrPerformanceBounds)
			}
		})
	}
}
