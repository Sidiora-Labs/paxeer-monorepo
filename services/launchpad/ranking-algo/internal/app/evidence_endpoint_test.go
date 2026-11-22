package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/google/uuid"
)

func rankingEvidencePeer(t *testing.T, pools map[string]bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cutoff, err := strconv.ParseInt(r.URL.Query().Get("cutoffTime"), 10, 64)
		if r.Method != http.MethodGet || r.URL.Path != "/internal/ranking-evidence" || r.URL.Query().Get("cutoffHeight") != "200" || r.URL.Query().Get("blockHash") != "0x"+strings.Repeat("c8", 32) || err != nil || cutoff < 86400 {
			http.Error(w, "cutoff unavailable", http.StatusBadRequest)
			return
		}
		observations := make(map[string]store.CanonicalPoolEvidence, len(pools))
		for pool, paused := range pools {
			value := paused
			observations[pool] = store.CanonicalPoolEvidence{Paused: &value, PauseEvidence: strings.Repeat("a", 64)}
		}
		if err := json.NewEncoder(w).Encode(store.CanonicalRankingEvidence{Revision: strings.Repeat("d", 64), CutoffHeight: 200, CutoffTime: cutoff, BlockHash: "0x" + strings.Repeat("c8", 32), Pools: observations}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestEvidenceEndpointRejectsInvalidBinding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint any
	}{
		{"missing", nil},
		{"empty", ""},
		{"scheme", "ftp://indexer.example.com/internal/ranking-evidence"},
		{"host", "http:///internal/ranking-evidence"},
		{"query", "http://indexer.example.com/internal/ranking-evidence?height=200"},
		{"empty query", "http://indexer.example.com/internal/ranking-evidence?"},
		{"path", "http://indexer.example.com/internal/other"},
		{"escaped path", "http://indexer.example.com/internal/%72anking-evidence"},
		{"user", "http://user@indexer.example.com/internal/ranking-evidence"},
		{"fragment", "http://indexer.example.com/internal/ranking-evidence#pool"},
		{"empty fragment", "http://indexer.example.com/internal/ranking-evidence#"},
		{"port", "http://indexer.example.com:65536/internal/ranking-evidence"},
		{"empty port", "http://indexer.example.com:/internal/ranking-evidence"},
		{"whitespace", "http://indexer.example.com /internal/ranking-evidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := map[string]any{"version": 2, "gatewayDNS": gatewayDNS, "gatewayCertSHA256": strings.Repeat("a", 64), "visibilityDNS": visibilityDNS, "visibilityCertSHA256": strings.Repeat("b", 64), "maxAttempts": 10, "maximumReads": 8}
			if tc.endpoint != nil {
				binding["evidenceEndpoint"] = tc.endpoint
			}
			raw, err := json.Marshal(binding)
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Config{RankingRuntimeFile: filepath.Join(t.TempDir(), "runtime.json"), RankingTickIntervalMS: 30000, RankingNewTickIntervalMS: 10000}
			cfg.RankingRuntimeSHA256 = writeBound(t, cfg.RankingRuntimeFile, raw)
			instance, err := newNativeApp(context.Background(), cfg, quietLogger(), nil, nil, nil, nil, nil, nil)
			if instance != nil || err == nil || !strings.Contains(err.Error(), "evidenceEndpoint") {
				t.Fatalf("invalid evidenceEndpoint startup = %v, %v", instance, err)
			}
		})
	}
}

func TestEvidenceEndpointBuildsReaderAndReadsUnpausedPool(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	runtime := f.app.native
	if runtime.binding.Version != 2 || runtime.reader.Evidence == nil {
		t.Fatal("version-two binding must build the evidence reader")
	}
	if runtime.evidenceClient == runtime.client || runtime.evidenceClient.Timeout <= 0 || runtime.evidenceClient.Timeout > 3*time.Second {
		t.Fatal("evidence reader requires its own bounded client")
	}
	source, err := store.NewFrozenInputs(f.app.pool, runtime.reader)
	if err != nil {
		t.Fatal(err)
	}
	input, err := source.Freeze(context.Background(), uuid.NewString(), f.cutoff)
	if err != nil || !input.Complete || len(input.Candidates) != 61 {
		t.Fatalf("bound evidence freeze = %+v, %v", input, err)
	}
	for _, candidate := range input.Candidates {
		if candidate.Paused || candidate.NullableMetrics == nil || candidate.NullableMetrics.Paused == nil || *candidate.NullableMetrics.Paused {
			t.Fatalf("pool lacks an unpaused observation: %+v", candidate)
		}
	}
	f.close()
	f.open(t)
	if f.app.native.reader.Evidence == nil {
		t.Fatal("reopened application lacks its bound evidence reader")
	}
	publicationBoundProduce(t, f)
}

func TestEvidenceEndpointReadFailureRefusesInputs(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	publicationBoundProduce(t, f)
	f.evidence.Close()
	ctx := context.Background()
	snapshots, err := f.app.native.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, false)
	if err == nil || len(snapshots) != 0 {
		t.Fatalf("unavailable evidence refresh = %v, %v", snapshots, err)
	}
	var retained int
	if err := f.app.pool.QueryRow(ctx, "SELECT count(*) FROM ranking.snapshots").Scan(&retained); err != nil || retained != len(ranker.Categories) {
		t.Fatalf("input refusal changed retained snapshots: %d, %v", retained, err)
	}
	code, body, _ := f.get(t, "/internal/v2/rankings/inputs")
	if code != http.StatusServiceUnavailable || !strings.Contains(string(body), "RANKING_INPUT_UNAVAILABLE") {
		t.Fatalf("unavailable evidence inputs = %d %s", code, body)
	}
}

func TestEvidenceEndpointRefusesRedirects(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	var followed atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	var binding config.RankingRuntimeBinding
	if err := config.ReadBoundDocument(f.cfg.RankingRuntimeFile, f.cfg.RankingRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	binding.EvidenceEndpoint = redirect.URL + "/internal/ranking-evidence"
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	f.close()
	f.cfg.RankingRuntimeSHA256 = writeBound(t, f.cfg.RankingRuntimeFile, raw)
	f.open(t)
	_, err = f.app.native.reader.Evidence.Read(context.Background(), 200, f.cutoff.Unix(), "0x"+strings.Repeat("c8", 32))
	if err != ranker.ErrUnavailable || followed.Load() != 0 {
		t.Fatalf("redirect evidence read = %v, followed %d", err, followed.Load())
	}
}
