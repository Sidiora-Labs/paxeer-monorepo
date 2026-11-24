package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func TestEmptyUniverseVisibilityContract(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 123456789, time.UTC)
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
		{"future observation", 200, func(v map[string]any) { v["observedAt"] = now.Add(time.Minute) }, false},
		{"stale observation", 200, func(v map[string]any) { v["observedAt"] = now.Add(-61 * time.Second) }, false},
		{"observation at clock allowance", 200, func(v map[string]any) { v["observedAt"] = now.Add(store.RankingVisibilityClockAllowance) }, true},
		{"observation beyond clock allowance", 200, func(v map[string]any) {
			v["observedAt"] = now.Add(store.RankingVisibilityClockAllowance + time.Nanosecond)
		}, false},
		{"observation at older bound", 200, func(v map[string]any) { v["observedAt"] = now.Add(-60 * time.Second) }, true},
		{"observation beyond older bound", 200, func(v map[string]any) { v["observedAt"] = now.Add(-60*time.Second - time.Nanosecond) }, false},
		{"missing observation", 200, func(v map[string]any) { delete(v, "observedAt") }, false},
		{"empty revision", 200, func(v map[string]any) { v["revision"] = "" }, false},
		{"short revision", 200, func(v map[string]any) { v["revision"] = strings.Repeat("a", 63) }, false},
		{"long revision", 200, func(v map[string]any) { v["revision"] = strings.Repeat("a", 65) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			revision := fmt.Sprintf("%x", sha256.Sum256(nil))
			observed := now
			body := map[string]any{"complete": true, "pools": map[string]bool{}, "revision": revision, "observedAt": observed}
			if tc.change != nil {
				tc.change(body)
			}
			if value, ok := body["observedAt"].(time.Time); ok {
				observed = value
			}
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/internal/v2/social/ranking-visibility" {
					t.Errorf("unexpected visibility request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				if err := json.NewEncoder(w).Encode(body); err != nil {
					t.Error(err)
				}
			}))
			defer peer.Close()
			client := &rankingVisibilityClient{endpoint: peer.URL + "/internal/v2/social/ranking-visibility", client: peer.Client(), now: func() time.Time { return now }}
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

func emptyUniverseRuntime(t *testing.T, pool *pgxpool.Pool, visibility *rankingVisibilityClient, logger *slog.Logger) *nativeRankingRuntime {
	t.Helper()
	guard, err := queue.NewEpochGuard("recovery", 1, internaltest.RuntimeRegion)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.NewCanonicalInputReader("indexer", "stats", sourceStream, projection, visibility)
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.NewFrozenInputs(pool, reader)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := ranker.NewCoordinator(source, publication)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err = rdb.Close(); err != nil {
		t.Fatal(err)
	}
	cache, err := store.NewSnapshotCache(rdb, "empty-universe")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	return &nativeRankingRuntime{binding: config.RankingRuntimeBinding{MaxAttempts: 10, EvidenceEndpoint: "http://indexer.example.com/internal/ranking-evidence"}, pool: pool, visibility: visibility, reader: reader, coordinator: coordinator, publication: publication, cache: cache, stream: delivery.StreamWriter{Schema: "api_stream"}, guard: guard, ctx: ctx, stop: stop, logger: logger}
}

func TestEmptyUniversePublicationReadinessAndFirstMarket(t *testing.T) {
	db := internaltest.NewRankingRuntimeDatabase(t)
	pool := publicationRecoveryOpenPool(t, db.AdminDSN)
	defer func() { pool.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cutoff := time.Now().UTC().Truncate(time.Second)
	blockHash := "0x" + strings.Repeat("c8", 32)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer-1',1,clock_timestamp()+interval '10 minutes')`, sourceStream)
	exec(`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,200,200)`, sourceStream)
	exec(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,$1,200,$2,0,true,true,$3)`, sourceStream, blockHash, cutoff)
	pki := newPKI(t)
	social := pki.issue(t, visibilityDNS, x509.ExtKeyUsageServerAuth)
	clientLeaf := pki.issue(t, rankingDNS, x509.ExtKeyUsageClientAuth)
	peer, _ := publicationRecoveryVisibilityPeer(t, pki, social, clientLeaf, map[string]bool{}, cutoff)
	httpClient := peer.Client()
	transport := httpClient.Transport.(*http.Transport)
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = visibilityDNS
	transport.TLSClientConfig.Certificates = []tls.Certificate{clientLeaf.pair(t)}
	defer httpClient.CloseIdleConnections()
	visibility := &rankingVisibilityClient{endpoint: peer.URL + "/internal/v2/social/ranking-visibility", client: httpClient, now: time.Now}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	runtime := emptyUniverseRuntime(t, pool, visibility, logger)
	defer func() { runtime.close() }()
	if err := runtime.ready(ctx); err == nil {
		t.Fatal("ranking ready before category heads exist")
	}
	raw, err := runtime.reader.MarshalUniverse(ctx, pool, uuid.NewString(), cutoff)
	if err != nil {
		t.Fatalf("freeze empty universe: %v", err)
	}
	var input ranker.InputSnapshot
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	emptyRevision := fmt.Sprintf("%x", sha256.Sum256(nil))
	if !input.Complete || input.Candidates == nil || len(input.Candidates) != 0 || input.Provenance == nil || input.Provenance.UniverseDigest != ranker.CanonicalUniverse(nil) || input.Provenance.ModerationRevision != emptyRevision {
		t.Fatalf("empty frozen universe = %s", raw)
	}
	for _, tc := range []struct {
		name   string
		cutoff time.Time
	}{
		{"visibility after frozen cutoff", cutoff.Add(-time.Second)},
		{"visibility older than frozen bound", cutoff.Add(61 * time.Second)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runtime.reader.MarshalUniverse(ctx, pool, uuid.NewString(), tc.cutoff); err != ranker.ErrUnavailable {
				t.Fatalf("empty frozen observation error = %v, want ErrUnavailable", err)
			}
		})
	}
	runtime.run(ctx)
	for {
		var published int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NOT NULL`).Scan(&published); err != nil {
			t.Fatal(err)
		}
		if published == len(ranker.Categories) && runtime.ready(ctx) == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("empty refresh did not become ready: %v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	runtime.close()
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Fatalf("empty refresh logged a failure: %s", logs.String())
	}
	heads := map[string]ranker.Header{}
	latestCutoff := cutoff.Unix()
	for _, category := range ranker.Categories {
		head, err := runtime.publication.Current(ctx, category)
		if err != nil {
			t.Fatalf("empty %s head: %v", category, err)
		}
		if head.Total != 0 || head.Revision != 1 || head.Stale(time.Now()) || head.Provenance.ModerationRevision != emptyRevision || head.Provenance.UniverseDigest != input.Provenance.UniverseDigest {
			t.Fatalf("empty %s head = %+v", category, head)
		}
		heads[category] = head
		if head.CutoffTime > latestCutoff {
			latestCutoff = head.CutoffTime
		}
		var frame []byte
		if err = pool.QueryRow(ctx, `SELECT frame FROM api_stream.stream_projection WHERE topic=$1`, "market.rankings."+category).Scan(&frame); err != nil {
			t.Fatal(err)
		}
		if _, err = delivery.ParseCommittedStreamFrame(frame); err != nil {
			t.Fatalf("empty %s stream frame: %v", category, err)
		}
		var event v1.RankingEvent
		if err = json.Unmarshal(frame, &event); err != nil || event.Payload.Items == nil || len(event.Payload.Items) != 0 || event.Payload.TotalEligible != "0" || event.Payload.SnapshotId != head.RunID {
			t.Fatalf("empty %s stream payload = %s, %v", category, frame, err)
		}
	}
	assertPages := func(r *nativeRankingRuntime, firstMarket string) {
		t.Helper()
		codec, err := ranker.NewCursorCodec([]byte(strings.Repeat("c", 32)))
		if err != nil {
			t.Fatal(err)
		}
		handler, err := httpapi.NewSnapshots(r.publication.SnapshotStore, r.cache, codec, visibility, 8)
		if err != nil {
			t.Fatal(err)
		}
		router := chi.NewRouter()
		httpapi.RegisterSnapshots(router, handler)
		for _, category := range ranker.Categories {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/rankings/"+category, nil))
			var page v1.RankingsResponse
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
				t.Fatalf("%s page status=%d body=%s", category, response.Code, response.Body.String())
			}
			snapshot := page.Data.Snapshot
			expected := 0
			if category == "new" && firstMarket != "" {
				expected = 1
			}
			if snapshot.Items == nil || len(snapshot.Items) != expected || string(snapshot.TotalEligible) != fmt.Sprint(expected) || snapshot.Truncated || page.NextCursor != nil || page.Quality != "complete" {
				t.Fatalf("%s page = %s", category, response.Body.String())
			}
			if expected == 1 && snapshot.Items[0].PoolAddress != firstMarket {
				t.Fatalf("first market absent: %s", response.Body.String())
			}
			inspection := httptest.NewRecorder()
			router.ServeHTTP(inspection, httptest.NewRequest(http.MethodGet, "/rankings/runs/"+snapshot.SnapshotId+"/"+category, nil))
			if inspection.Code != 200 {
				t.Fatalf("%s inspection status=%d body=%s", category, inspection.Code, inspection.Body.String())
			}
		}
	}
	assertPages(runtime, "")
	pool.Close()
	pool = publicationRecoveryOpenPool(t, db.AdminDSN)
	runtime = emptyUniverseRuntime(t, pool, visibility, logger)
	if err = runtime.ready(ctx); err != nil {
		t.Fatalf("retained empty heads not ready: %v", err)
	}
	for category, previous := range heads {
		head, err := runtime.publication.Current(ctx, category)
		if err != nil || head.RunID != previous.RunID || head.Revision != previous.Revision {
			t.Fatalf("retained %s head changed: %+v, %v", category, head, err)
		}
	}
	for time.Now().Unix() <= latestCutoff {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	nextCutoff := time.Now().UTC().Truncate(time.Second)
	nextBlockHash := fmt.Sprintf("0x%064x", 201)
	exec(`UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=201,typed_complete_height=201 WHERE chain_id=125 AND stream_id=$1`, sourceStream)
	exec(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,$1,201,$2,1,true,true,$3)`, sourceStream, nextBlockHash, nextCutoff)
	firstMarket := fmt.Sprintf("0x%040x", 1)
	owner := fmt.Sprintf("0x%040x", 2)
	token := fmt.Sprintf("0x%040x", 1001)
	exec(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$3,$3,1,201,$4,$4,0,0,$5,'{}','{}')`, token, firstMarket, owner, nextBlockHash, nextCutoff.Unix())
	nextPeer, _ := publicationRecoveryVisibilityPeer(t, pki, social, clientLeaf, map[string]bool{firstMarket: true}, nextCutoff)
	visibility.endpoint = nextPeer.URL + "/internal/v2/social/ranking-visibility"
	evidencePeer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cutoffHeight") != "201" || r.URL.Query().Get("cutoffTime") != fmt.Sprint(nextCutoff.Unix()) || r.URL.Query().Get("blockHash") != nextBlockHash {
			t.Error("canonical evidence request lost cutoff binding")
			w.WriteHeader(400)
			return
		}
		paused := false
		evidence := store.CanonicalRankingEvidence{Revision: strings.Repeat("d", 64), CutoffHeight: 201, CutoffTime: nextCutoff.Unix(), BlockHash: nextBlockHash, Pools: map[string]store.CanonicalPoolEvidence{firstMarket: {Paused: &paused, PauseEvidence: strings.Repeat("a", 64)}}}
		if err := json.NewEncoder(w).Encode(evidence); err != nil {
			t.Error(err)
		}
	}))
	defer evidencePeer.Close()
	runtime.reader.Evidence, err = store.NewHTTPCanonicalEvidence(evidencePeer.URL, evidencePeer.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.ready(ctx); err != ranker.ErrUnavailable {
		t.Fatalf("heads ready before visibility revision refresh: %v", err)
	}
	for _, newOnly := range []bool{false, true} {
		if _, err = runtime.coordinator.Refresh(ctx, uuid.NewString(), nextCutoff, newOnly); err != nil {
			t.Fatalf("first market refresh newOnly=%v: %v", newOnly, err)
		}
	}
	nextVisibility, err := visibility.ReadRankingVisibility(ctx)
	if err != nil || nextVisibility.Revision == emptyRevision {
		t.Fatalf("first market visibility revision: %+v, %v", nextVisibility, err)
	}
	for _, category := range ranker.Categories {
		head, err := runtime.publication.Current(ctx, category)
		expected := int64(0)
		if category == "new" {
			expected = 1
		}
		if err != nil || head.Total != expected || head.Provenance.Watermark.Height != "201" || head.Revision != 2 || head.RunID == heads[category].RunID || head.Provenance.ModerationRevision != nextVisibility.Revision || head.Provenance.UniverseDigest == input.Provenance.UniverseDigest {
			t.Fatalf("first market %s head = %+v, %v", category, head, err)
		}
	}
	for i := 0; i < len(ranker.Categories); i++ {
		worked, err := runtime.publication.CommitStreamOnce(ctx, runtime.stream, 10)
		if err != nil || !worked {
			t.Fatalf("first market stream publication %d: worked=%v, %v", i, worked, err)
		}
	}
	if err = runtime.ready(ctx); err != nil {
		t.Fatalf("first market heads not ready: %v", err)
	}
	assertPages(runtime, firstMarket)
}
