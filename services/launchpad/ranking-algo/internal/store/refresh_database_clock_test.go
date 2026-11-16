package store

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func TestRefreshCutoffRequiresTransactionClock(t *testing.T) {
	reader, err := NewCanonicalInputReader("indexer", "stats", "stream", "projection", visibilityStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = reader.ReadRankingUniverseTxDetailed(context.Background(), nil, "run")
	step, class, ok := ranker.RefreshRefusalDetails(err)
	if !ok || step != "cutoff" || class != "validation" {
		t.Fatalf("cutoff refusal = %q/%q/%t", step, class, ok)
	}
}

type visibilityStub struct{}

func (visibilityStub) ReadRankingVisibility(context.Context) (RankingVisibility, error) {
	return RankingVisibility{}, nil
}
