package main

import (
	"errors"
	"testing"
	"time"

	"github.com/paxeer-network/pax-load/config"
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
