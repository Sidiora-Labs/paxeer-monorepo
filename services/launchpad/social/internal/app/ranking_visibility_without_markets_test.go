package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestRankingVisibilityWithoutMarkets(t *testing.T) {
	h := newPlatformHarness(t)
	if _, err := h.dbConn.Exec(context.Background(), `DELETE FROM indexer.indexer_canonical_creations WHERE pool=$1`, h.pool); err != nil {
		t.Fatal(err)
	}
	wantRevision := fmt.Sprintf("%x", sha256.Sum256(nil))
	for name, read := range map[string]func(context.Context) (any, error){
		"connected": func(ctx context.Context) (any, error) {
			return (&socialConnectedRuntime{pool: h.app.pool}).rankingVisibility(ctx)
		},
		"platform": func(ctx context.Context) (any, error) {
			return platformRankingVisibility(ctx, h.app.pool)
		},
	} {
		t.Run(name, func(t *testing.T) {
			value, err := read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Revision   string          `json:"revision"`
				ObservedAt string          `json:"observedAt"`
				Complete   bool            `json:"complete"`
				Pools      map[string]bool `json:"pools"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !got.Complete || got.Pools == nil || len(got.Pools) != 0 || got.ObservedAt == "" || got.Revision != wantRevision {
				t.Fatalf("empty ranking visibility = %+v", got)
			}
		})
	}
}
