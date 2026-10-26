package stats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBoundsCheckReportsMinAverageTPS(t *testing.T) {
	fs := FinalStats{
		LoadTestStatistics: LoadTestStatistics{AvgTPS: 200, TotalTxs: 10000},
		OverallPerformance: OverallPerformance{
			AverageTPS:        90,
			TotalTransactions: 9000,
		},
	}
	require.Equal(t, []Violation{
		{Bound: "min_average_tps", Limit: "100", Observed: "90"},
	}, (Bounds{MinAverageTPS: 100}).Check(&fs))

	t.Run("fractional throughput", func(t *testing.T) {
		fs := FinalStats{OverallPerformance: OverallPerformance{AverageTPS: 99.999}}
		require.Equal(t, []Violation{
			{Bound: "min_average_tps", Limit: "100.001", Observed: "99.999"},
		}, (Bounds{MinAverageTPS: 100.001}).Check(&fs))
	})

	t.Run("transaction minimum is also reported", func(t *testing.T) {
		require.Equal(t, []Violation{
			{Bound: "min_average_tps", Limit: "100", Observed: "90"},
			{Bound: "min_total_txs", Limit: "9500", Observed: "9000"},
		}, (Bounds{MinAverageTPS: 100, MinTotalTxs: 9500}).Check(&fs))
	})

	t.Run("transaction counts retain uint64 precision", func(t *testing.T) {
		fs := FinalStats{OverallPerformance: OverallPerformance{TotalTransactions: 18446744073709551614}}
		require.Equal(t, []Violation{
			{Bound: "min_total_txs", Limit: "18446744073709551615", Observed: "18446744073709551614"},
		}, (Bounds{MinTotalTxs: 18446744073709551615}).Check(&fs))
	})
}

func TestBoundsCheckReportsP99PerEndpoint(t *testing.T) {
	fs := FinalStats{EndpointPerformance: map[string]EndpointPerformance{
		"a": {LatencyP99: 80 * time.Millisecond},
		"b": {LatencyP99: 250 * time.Millisecond},
	}}
	bounds := Bounds{MaxP99Latency: 200 * time.Millisecond}
	require.Equal(t, []Violation{
		{Bound: "max_p99_latency:b", Limit: "200ms", Observed: "250ms"},
	}, bounds.Check(&fs))

	t.Run("every failing endpoint in sorted order", func(t *testing.T) {
		fs := FinalStats{
			OverallPerformance: OverallPerformance{AverageTPS: 90, TotalTransactions: 9},
			EndpointPerformance: map[string]EndpointPerformance{
				"z":     {LatencyP99: 400 * time.Millisecond},
				"b":     {LatencyP99: 250 * time.Millisecond},
				"a":     {LatencyP99: 80 * time.Millisecond},
				"equal": {LatencyP99: 200 * time.Millisecond},
			},
		}
		require.Equal(t, []Violation{
			{Bound: "min_average_tps", Limit: "100", Observed: "90"},
			{Bound: "min_total_txs", Limit: "10", Observed: "9"},
			{Bound: "max_p99_latency:b", Limit: "200ms", Observed: "250ms"},
			{Bound: "max_p99_latency:z", Limit: "200ms", Observed: "400ms"},
		}, (Bounds{MinAverageTPS: 100, MinTotalTxs: 10, MaxP99Latency: bounds.MaxP99Latency}).Check(&fs))
	})
}

func TestBoundsCheckPassesWithinBounds(t *testing.T) {
	fs := FinalStats{
		OverallPerformance: OverallPerformance{AverageTPS: 150, TotalTransactions: 9000},
		EndpointPerformance: map[string]EndpointPerformance{
			"a": {LatencyP99: 50 * time.Millisecond},
		},
	}
	for _, tc := range []struct {
		name   string
		bounds Bounds
		stats  FinalStats
	}{
		{
			name:   "latency unset",
			bounds: Bounds{MinAverageTPS: 100, MinTotalTxs: 5000},
			stats:  fs,
		},
		{
			name:   "all bounds hold",
			bounds: Bounds{MinAverageTPS: 100, MinTotalTxs: 5000, MaxP99Latency: 200 * time.Millisecond},
			stats:  fs,
		},
		{
			name:   "equality holds",
			bounds: Bounds{MinAverageTPS: 150, MinTotalTxs: 9000, MaxP99Latency: 50 * time.Millisecond},
			stats:  fs,
		},
		{
			name:  "all bounds unset",
			stats: fs,
		},
		{
			name:   "throughput and transaction bounds unset",
			bounds: Bounds{MaxP99Latency: time.Millisecond},
			stats: FinalStats{EndpointPerformance: map[string]EndpointPerformance{
				"a": {LatencyP99: time.Millisecond},
			}},
		},
		{
			name:  "empty stats with bounds unset",
			stats: FinalStats{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Empty(t, tc.bounds.Check(&tc.stats))
		})
	}
}
