package stats

import (
	"sort"
	"strconv"
	"time"
)

type Bounds struct {
	MinAverageTPS float64
	MaxP99Latency time.Duration
	MinTotalTxs   uint64
}

type Violation struct {
	Bound    string
	Limit    string
	Observed string
}

func (b Bounds) Check(fs *FinalStats) []Violation {
	var violations []Violation
	if b.MinAverageTPS != 0 && fs.OverallPerformance.AverageTPS < b.MinAverageTPS {
		violations = append(violations, Violation{
			Bound:    "min_average_tps",
			Limit:    strconv.FormatFloat(b.MinAverageTPS, 'f', -1, 64),
			Observed: strconv.FormatFloat(fs.OverallPerformance.AverageTPS, 'f', -1, 64),
		})
	}
	if b.MinTotalTxs != 0 && fs.OverallPerformance.TotalTransactions < b.MinTotalTxs {
		violations = append(violations, Violation{
			Bound:    "min_total_txs",
			Limit:    strconv.FormatUint(b.MinTotalTxs, 10),
			Observed: strconv.FormatUint(fs.OverallPerformance.TotalTransactions, 10),
		})
	}
	if b.MaxP99Latency != 0 {
		endpoints := make([]string, 0, len(fs.EndpointPerformance))
		for endpoint := range fs.EndpointPerformance {
			endpoints = append(endpoints, endpoint)
		}
		sort.Strings(endpoints)
		for _, endpoint := range endpoints {
			latency := fs.EndpointPerformance[endpoint].LatencyP99
			if latency > b.MaxP99Latency {
				violations = append(violations, Violation{
					Bound:    "max_p99_latency:" + endpoint,
					Limit:    b.MaxP99Latency.String(),
					Observed: latency.String(),
				})
			}
		}
	}
	return violations
}
