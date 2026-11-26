package indexedtest_test

import (
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap/indexedtest"
)

func TestRegisteredSourceHistories(t *testing.T) {
	t.Chdir("..")
	f := indexedtest.Open(t, "COORDINATOR_TEST_POSTGRES_DSN", "api_rest_runtime")
	references := indexedtest.References(t)
	statements := map[string]string{}
	for _, name := range []string{"CommitContaining", "PreviousHeightContaining"} {
		statements[name] = indexedtest.Statement(t, "sdk/launchpad/shared/accounting/sourcebootstrap/sourcebootstrap.go", name)
		if references[name] == "" || statements[name] == "" {
			t.Fatalf("source statement required for %s", name)
		}
	}
	comparisons := 0
	indexedtest.Histories(t, f, func(t *testing.T, from, to int64) {
		for height := from; height <= to; height++ {
			for name, statement := range statements {
				indexedtest.Equal(t, f.Runtime, statement, references[name], indexedtest.Stream, height)
				comparisons++
			}
		}
	})
	if comparisons != 2706 {
		t.Fatalf("source history comparisons: got %d want 2706", comparisons)
	}
}

func TestRegisteredSourceIndexAccounting(t *testing.T) {
	f := indexedtest.Open(t, "COORDINATOR_TEST_POSTGRES_DSN", "api_rest_runtime")
	f.Seed(t, 20000, 4)
	f.Blocks(t, 0, 64)
	f.Analyze(t)
	sql := indexedtest.Statement(t, "sdk/launchpad/shared/accounting/sourcebootstrap/sourcebootstrap.go", "CommitContaining")
	for _, height := range []int64{1, 40000, 80000} {
		t.Run(strconv.FormatInt(height, 10), func(t *testing.T) {
			plan := indexedtest.Explain(t, f.Runtime, sql, indexedtest.Stream, height)
			indexedtest.AssertIndexed(t, plan.Plan, 5)
			if plan.Milliseconds < 0 || indexedtest.Rows(plan.Plan) < 1 {
				t.Fatalf("source index accounting: %+v", plan)
			}
			commit, err := sourcebootstrap.CommitContaining(t.Context(), f.Runtime, indexedtest.Stream, height)
			last := ((height-1)/4 + 1) * 4
			if err != nil || commit.LastCompleteHeight != last || commit.BlockHash != "block-"+strconv.FormatInt(last, 10) {
				t.Fatalf("source commit: %+v %v", commit, err)
			}
		})
	}
}
