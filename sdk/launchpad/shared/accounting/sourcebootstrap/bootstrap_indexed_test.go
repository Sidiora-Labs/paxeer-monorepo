package sourcebootstrap_test

import (
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap/indexedtest"
)

const indexedSourceFile = "sdk/launchpad/shared/accounting/sourcebootstrap/sourcebootstrap.go"

func TestBootstrapIndexedCommitsReference(t *testing.T) {
	f := indexedtest.Open(t, "COORDINATOR_TEST_POSTGRES_DSN", "api_rest_runtime")
	refs := indexedtest.References(t)
	indexedtest.Histories(t, f, func(t *testing.T, from, to int64) {
		for _, name := range []string{"CommitContaining", "PreviousHeightContaining"} {
			sql := indexedtest.Statement(t, indexedSourceFile, name)
			for height := from; height <= to; height++ {
				indexedtest.Equal(t, f.Runtime, sql, refs[name], indexedtest.Stream, height)
			}
		}
	})
}

func TestBootstrapIndexedCommitsPlans(t *testing.T) {
	f := indexedtest.Open(t, "COORDINATOR_TEST_POSTGRES_DSN", "api_rest_runtime")
	f.Million(t)
	for _, name := range []string{"CommitContaining", "PreviousHeightContaining"} {
		sql := indexedtest.Statement(t, indexedSourceFile, name)
		for _, height := range []int64{1, 500000, 1000000} {
			t.Run(name+"/"+strconv.FormatInt(height, 10), func(t *testing.T) {
				plan := indexedtest.Explain(t, f.Runtime, sql, indexedtest.Stream, height)
				indexedtest.AssertIndexed(t, plan.Plan, 5)
				commit, err := sourcebootstrap.CommitContaining(t.Context(), f.Runtime, indexedtest.Stream, height)
				if err != nil || commit.LastCompleteHeight != height || commit.BlockHash != "block-"+strconv.FormatInt(height, 10) {
					t.Fatalf("containing commit: %+v %v", commit, err)
				}
				previous, err := sourcebootstrap.PreviousHeightContaining(t.Context(), f.Runtime, indexedtest.Stream, height)
				if err != nil || previous != height-1 {
					t.Fatalf("previous height: %d %v", previous, err)
				}
			})
		}
	}
}
