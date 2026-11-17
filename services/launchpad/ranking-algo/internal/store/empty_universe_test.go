package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func TestEmptyUniverseVisibilityContract(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		change   func(map[string]any)
		accepted bool
	}{
		{"complete empty pools", 200, nil, true},
		{"non 200", 503, nil, false},
		{"incomplete", 200, func(v map[string]any) { v["complete"] = false }, false},
		{"missing pools", 200, func(v map[string]any) { delete(v, "pools") }, false},
		{"null pools", 200, func(v map[string]any) { v["pools"] = nil }, false},
		{"future observation", 200, func(v map[string]any) { v["observedAt"] = time.Now().Add(time.Minute) }, false},
		{"stale observation", 200, func(v map[string]any) { v["observedAt"] = time.Now().Add(-61 * time.Second) }, false},
		{"missing observation", 200, func(v map[string]any) { delete(v, "observedAt") }, false},
		{"empty revision", 200, func(v map[string]any) { v["revision"] = "" }, false},
		{"short revision", 200, func(v map[string]any) { v["revision"] = strings.Repeat("a", 63) }, false},
		{"long revision", 200, func(v map[string]any) { v["revision"] = strings.Repeat("a", 65) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			revision := fmt.Sprintf("%x", sha256.Sum256(nil))
			observed := time.Now().UTC().Truncate(time.Second)
			body := map[string]any{"complete": true, "pools": map[string]bool{}, "revision": revision, "observedAt": observed}
			if tc.change != nil {
				tc.change(body)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/internal/v2/social/ranking-visibility" {
					t.Errorf("unexpected visibility request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				if err := json.NewEncoder(w).Encode(body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewHTTPRankingVisibility(server.URL+"/internal/v2/social/ranking-visibility", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.ReadRankingVisibility(context.Background())
			if !tc.accepted {
				if err != ranker.ErrUnavailable {
					t.Fatalf("visibility error = %v, want ErrUnavailable", err)
				}
				if _, _, err = client.CurrentRankingRevision(context.Background()); err != ranker.ErrUnavailable {
					t.Fatalf("visibility fence error = %v, want ErrUnavailable", err)
				}
				return
			}
			if err != nil || !out.Complete || out.Pools == nil || len(out.Pools) != 0 || out.Revision != revision || !out.ObservedAt.Equal(observed) {
				t.Fatalf("empty visibility = %+v, %v", out, err)
			}
			gotRevision, gotObserved, err := client.CurrentRankingRevision(context.Background())
			if err != nil || gotRevision != revision || !gotObserved.Equal(observed) {
				t.Fatalf("empty visibility fence = %s, %s, %v", gotRevision, gotObserved, err)
			}
		})
	}
}
