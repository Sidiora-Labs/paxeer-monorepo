package consumer

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap/indexedtest"
)

const indexedStatsSource = "services/launchpad/stats-workers/internal/consumer/source_bootstrap.go"

func indexedStatsArgs(from, to int64) []any {
	return []any{indexedtest.Stream, from, to, "event", "pool", int64(48), int64(0), int64(0), int64(0), int64(64), int64(3)}
}

func indexedStatsCountArgs(from, to int64) []any {
	return []any{indexedtest.Stream, from, to, int64(48), int64(0), int64(0), "event", "pool"}
}

func TestBootstrapIndexedCommitsReference(t *testing.T) {
	f := indexedtest.Open(t, "STATS_TEST_POSTGRES_DSN", "stats_runtime")
	refs := indexedtest.References(t)
	step := indexedtest.Statement(t, indexedStatsSource, "slice")
	count := indexedtest.Statement(t, indexedStatsSource, "counts")
	indexedtest.Histories(t, f, func(t *testing.T, from, to int64) {
		indexedtest.Equal(t, f.Runtime, step, refs["slice"], indexedStatsArgs(from, to)...)
		indexedtest.Equal(t, f.Runtime, count, refs["counts"], indexedStatsCountArgs(from, to)...)
	})
}

func indexedStatsPlanArgs(from, to int64) []any {
	return []any{indexedtest.Stream, from, to, "no-swap", "pool", to, int64(0), int64(0), from, to, int64(0)}
}

func TestBootstrapIndexedCommitsPlans(t *testing.T) {
	f := indexedtest.Open(t, "STATS_TEST_POSTGRES_DSN", "stats_runtime")
	f.Million(t)
	refs := indexedtest.References(t)
	step := indexedtest.Statement(t, indexedStatsSource, "slice")
	count := indexedtest.Statement(t, indexedStatsSource, "counts")
	f.WithoutHeightIndex(t, func(t *testing.T) {
		for _, size := range []int64{128, 2000} {
			for _, from := range []int64{0, 498000, 1000000 - size} {
				t.Run(fmt.Sprintf("step-%d/%d", size, from), func(t *testing.T) {
					indexedtest.Explain(t, f.Runtime, refs["slice"], indexedStatsPlanArgs(from, from+size)...)
				})
			}
		}
	})
	for _, size := range []int64{128, 2000, 4096} {
		for _, from := range []int64{0, 498000, 1000000 - size} {
			t.Run(fmt.Sprintf("step-%d/%d", size, from), func(t *testing.T) {
				plan := indexedtest.Explain(t, f.Runtime, step, indexedStatsPlanArgs(from, from+size)...)
				indexedtest.AssertIndexed(t, plan.Plan, float64(size+4))
				if size == 4096 && indexedtest.Rows(plan.Plan) > 64*float64(size) {
					t.Fatalf("step read %.0f rows, bound %d", indexedtest.Rows(plan.Plan), 64*size)
				}
				s := statsBootstrapper{config: SourceVerifiedAccountingConfig{Pool: f.Runtime, SourceStream: indexedtest.Stream, StatementTimeout: time.Minute, MaxReplayFills: 10000}}
				p := statsBootstrapProgress{last: from, bootstrap: statsSourceBootstrap{Evidence: sourcebootstrap.StatsEvidence{ReplayFromHeight: from, FirstCommitLastHeight: from + size}}}
				p.bootstrap.Fill.Position.Height = strconv.FormatInt(from+size, 10)
				p.bootstrap.Fill.Position.TransactionIndex = "0"
				p.bootstrap.Fill.Position.LogIndex = "0"
				if err := s.slice(t.Context(), &p, from+size); err != nil {
					t.Fatal(err)
				}
				if p.last != from+size || p.commits != size || p.raw != p.records || p.openRaw != 0 {
					t.Fatalf("step progress: %+v", p)
				}
			})
			t.Run(fmt.Sprintf("count-%d/%d", size, from), func(t *testing.T) {
				args := []any{indexedtest.Stream, from, from + size, from + size, int64(0), int64(0), "no-swap", "pool"}
				plan := indexedtest.Explain(t, f.Runtime, count, args...)
				indexedtest.AssertIndexed(t, plan.Plan, float64(size+4))
			})
		}
	}
}
