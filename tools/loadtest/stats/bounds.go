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
	if fs == nil {
		fs = &FinalStats{}
	}
	enabledSuccess := b.MinAverageTPS != 0 || b.MinTotalTxs != 0
	if (enabledSuccess || b.MaxP99Latency != 0) && fs.Incomplete {
		violations = append(violations, Violation{Bound: "incomplete_run", Limit: "complete evidence", Observed: "incomplete"})
	}
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
	if enabledSuccess && fs.OverallPerformance.SampleCount <= 0 {
		violations = append(violations, Violation{Bound: "insufficient_samples:accepted_submissions", Limit: "at least 1", Observed: strconv.Itoa(fs.OverallPerformance.SampleCount)})
	}
	if b.MaxP99Latency != 0 {
		required := make(map[string]bool)
		for _, endpoint := range fs.RequiredEndpoints {
			required[endpoint] = true
		}
		for endpoint := range fs.EndpointPerformance {
			required[endpoint] = true
		}
		endpoints := make([]string, 0, len(required))
		for endpoint := range required {
			endpoints = append(endpoints, endpoint)
		}
		sort.Strings(endpoints)
		if len(endpoints) == 0 {
			violations = append(violations, Violation{Bound: "insufficient_samples:max_p99_latency", Limit: "at least 1 endpoint with samples", Observed: "0 endpoints"})
		}
		for _, endpoint := range endpoints {
			performance := fs.EndpointPerformance[endpoint]
			if performance.SampleCount <= 0 {
				violations = append(violations, Violation{Bound: "insufficient_samples:" + endpoint, Limit: "at least 1", Observed: strconv.Itoa(performance.SampleCount)})
			}
			latency := performance.LatencyP99
			if latency > b.MaxP99Latency {
				violations = append(violations, Violation{Bound: "max_p99_latency:" + endpoint, Limit: b.MaxP99Latency.String(), Observed: latency.String()})
			}
		}
	}
	return violations
}
