package store

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func TestCanonicalEvidenceRejectsChangedCutoffData(t *testing.T) {
	const cutoff int64 = 100000
	const height uint64 = 200
	blockHash := "0x" + strings.Repeat("a", 64)
	base := CanonicalRankingEvidence{Revision: strings.Repeat("b", 64), CutoffHeight: height, CutoffTime: cutoff, BlockHash: blockHash, Pools: map[string]CanonicalPoolEvidence{poolA: {Paused: boolPointer(false), PauseEvidence: strings.Repeat("c", 64)}}}
	for _, tc := range []struct {
		name   string
		status int
		change func(*CanonicalRankingEvidence)
	}{
		{"status", http.StatusServiceUnavailable, nil},
		{"revision", http.StatusOK, func(e *CanonicalRankingEvidence) { e.Revision = "unbound" }},
		{"cutoff", http.StatusOK, func(e *CanonicalRankingEvidence) { e.CutoffTime++ }},
		{"pool address", http.StatusOK, func(e *CanonicalRankingEvidence) { e.Pools = map[string]CanonicalPoolEvidence{"bad": {}} }},
		{"pool encoding", http.StatusOK, func(e *CanonicalRankingEvidence) {
			e.Pools = map[string]CanonicalPoolEvidence{"0x" + strings.Repeat("z", 40): {}}
		}},
		{"pause proof", http.StatusOK, func(e *CanonicalRankingEvidence) {
			e.Pools[poolA] = CanonicalPoolEvidence{Paused: boolPointer(false), PauseEvidence: "bad"}
		}},
		{"self volume", http.StatusOK, func(e *CanonicalRankingEvidence) {
			e.Pools[poolA] = CanonicalPoolEvidence{SelfVolume: &CanonicalSelfVolume{Raw: "1", WindowStart: 0, WindowEnd: cutoff, Evidence: strings.Repeat("d", 64)}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := base
			data.Pools = map[string]CanonicalPoolEvidence{poolA: base.Pools[poolA]}
			if tc.change != nil {
				tc.change(&data)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("cutoffHeight") != "200" || r.URL.Query().Get("cutoffTime") != "100000" || r.URL.Query().Get("blockHash") != blockHash {
					t.Errorf("evidence request lost its frozen cutoff")
				}
				w.WriteHeader(tc.status)
				if err := json.NewEncoder(w).Encode(data); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			reader, err := NewHTTPCanonicalEvidence(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reader.Read(context.Background(), height, cutoff, blockHash); err == nil {
				t.Fatal("changed canonical evidence accepted")
			}
		})
	}
	if _, err := NewHTTPCanonicalEvidence("file:///tmp/evidence", http.DefaultClient); err != ranker.ErrUnavailable {
		t.Fatalf("non-HTTP evidence endpoint accepted: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(server.Close)
	reader, err := NewHTTPCanonicalEvidence(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(context.Background(), 0, cutoff, blockHash); err != ranker.ErrUnavailable {
		t.Fatalf("zero cutoff height accepted: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Read(ctx, height, cutoff, blockHash); err == nil {
		t.Fatal("canceled canonical evidence read accepted")
	}
}
