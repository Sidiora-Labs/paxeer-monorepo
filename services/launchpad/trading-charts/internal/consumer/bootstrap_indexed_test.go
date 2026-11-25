package consumer

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap/indexedtest"
)

const indexedChartsSource = "services/launchpad/trading-charts/internal/consumer/source_bootstrap.go"

func indexedChartArgs(from, to int64) []any {
	return []any{indexedtest.Stream, from, to, int64(0), int64(48), int64(0), int64(0), "tx-48", "block-48", "emitter", "event", "pool"}
}

func indexedChartCountArgs(from, to int64) []any {
	return []any{indexedtest.Stream, from, to, int64(0), int64(48), int64(0), int64(0), "event", "pool"}
}

func TestBootstrapIndexedCommitsReference(t *testing.T) {
	f := indexedtest.Open(t, "CHARTS_TEST_POSTGRES_URL", "candles_runtime")
	refs := indexedtest.References(t)
	step := indexedtest.Statement(t, indexedChartsSource, "advance")
	count := indexedtest.Statement(t, indexedChartsSource, "recheck")
	indexedtest.Histories(t, f, func(t *testing.T, from, to int64) {
		indexedtest.Equal(t, f.Runtime, step, refs["bootstrapSliceSQL"], indexedChartArgs(from, to)...)
		indexedtest.Equal(t, f.Runtime, count, refs["recheck"], indexedChartCountArgs(from, to)...)
	})
}

func indexedChartPlanArgs(from, to int64) []any {
	return []any{indexedtest.Stream, from, to, from, to, int64(0), int64(0), "tx", "block", "emitter", "no-swap", "pool"}
}

func TestBootstrapIndexedCommitsPlans(t *testing.T) {
	f := indexedtest.Open(t, "CHARTS_TEST_POSTGRES_URL", "candles_runtime")
	f.Million(t)
	refs := indexedtest.References(t)
	step := indexedtest.Statement(t, indexedChartsSource, "advance")
	count := indexedtest.Statement(t, indexedChartsSource, "recheck")
	f.WithoutHeightIndex(t, func(t *testing.T) {
		for _, size := range []int64{128, 2000} {
			for _, from := range []int64{0, 498000, 1000000 - size} {
				t.Run(fmt.Sprintf("step-%d/%d", size, from), func(t *testing.T) {
					indexedtest.Explain(t, f.Runtime, refs["bootstrapSliceSQL"], indexedChartPlanArgs(from, from+size)...)
				})
			}
		}
	})
	for _, size := range []int64{128, 2000} {
		for _, from := range []int64{0, 498000, 1000000 - size} {
			t.Run(fmt.Sprintf("step-%d/%d", size, from), func(t *testing.T) {
				plan := indexedtest.Explain(t, f.Runtime, step, indexedChartPlanArgs(from, from+size)...)
				indexedtest.AssertIndexed(t, plan.Plan, float64(size+4))
			})
			t.Run(fmt.Sprintf("count-%d/%d", size, from), func(t *testing.T) {
				args := []any{indexedtest.Stream, from, from + size, from, from + size, int64(0), int64(0), "no-swap", "pool"}
				plan := indexedtest.Explain(t, f.Runtime, count, args...)
				indexedtest.AssertIndexed(t, plan.Plan, float64(size+4))
			})
		}
	}
}
