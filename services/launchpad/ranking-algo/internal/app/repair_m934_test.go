package app

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// This peer serves the social ranking-visibility transcript shape and revision
// algorithm from social/internal/app/app.go over the production mTLS interface.
func repairM934VisibilityPeer(t *testing.T, pki *testPKI, social, client leaf, pools map[string]bool, observedAt ...time.Time) (*httptest.Server, *atomic.Bool) {
	t.Helper()
	authority := &atomic.Bool{}
	authority.Store(true)
	keys := make([]string, 0, len(pools))
	for address := range pools {
		keys = append(keys, address)
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, address := range keys {
		fmt.Fprintf(hash, "%s:%t:%d\n", address, pools[address], 1)
	}
	revision := fmt.Sprintf("%x", hash.Sum(nil))
	peer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v2/social/ranking-visibility" {
			http.NotFound(w, r)
			return
		}
		if !authority.Load() {
			http.Error(w, "visibility authority unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		observed := time.Now().UTC().Truncate(time.Second)
		if len(observedAt) > 0 {
			observed = observedAt[0]
		}
		if err := json.NewEncoder(w).Encode(store.RankingVisibility{Revision: revision, ObservedAt: observed, Complete: true, Pools: pools}); err != nil {
			t.Errorf("visibility response: %v", err)
		}
	}))
	roots := x509.NewCertPool()
	roots.AddCert(pki.caCert)
	peer.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{social.pair(t)}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, VerifyConnection: func(c tls.ConnectionState) error {
		if len(c.PeerCertificates) == 0 || c.PeerCertificates[0].VerifyHostname(rankingDNS) != nil || fmt.Sprintf("%x", sha256.Sum256(c.PeerCertificates[0].Raw)) != client.pin() {
			return ranker.ErrUnavailable
		}
		return nil
	}}
	peer.StartTLS()
	t.Cleanup(peer.Close)
	return peer, authority
}

type repairM934Fixture struct {
	cfg       config.Config
	path      string
	cutoff    time.Time
	gateway   leaf
	roots     *x509.CertPool
	authority *atomic.Bool
	app       *App
	server    *httptest.Server
	client    *http.Client
}

func repairM934NewFixture(t *testing.T) *repairM934Fixture {
	t.Helper()
	cutoff := time.Now().UTC().Truncate(time.Second)
	pki := newPKI(t)
	server := pki.issue(t, rankingDNS, x509.ExtKeyUsageServerAuth)
	client := pki.issue(t, rankingDNS, x509.ExtKeyUsageClientAuth)
	gateway := pki.issue(t, gatewayDNS, x509.ExtKeyUsageClientAuth)
	social := pki.issue(t, visibilityDNS, x509.ExtKeyUsageServerAuth)
	pools := map[string]bool{}
	for i := 1; i <= 61; i++ {
		pools[fmt.Sprintf("0x%040x", i)] = i != 61
	}
	peer, authority := repairM934VisibilityPeer(t, pki, social, client, pools, cutoff)
	dir := t.TempDir()
	binding := config.RankingRuntimeBinding{
		Version: 2, RankingSchema: "ranking", StatsSchema: "stats", SourceSchema: "indexer", SourceStream: sourceStream, ProjectionVersion: projection,
		CacheNamespace: "snapshots", CursorKeyReference: "REPAIR_M934_CURSOR_KEY", RecoverySchema: "recovery", RecoveryEpoch: 1, RecoveryRegion: "fixture-loopback",
		VisibilityEndpoint: peer.URL + "/internal/v2/social/ranking-visibility", VisibilityDNS: visibilityDNS, VisibilityCertSHA256: social.pin(), GatewayDNS: gatewayDNS, GatewayCertSHA256: gateway.pin(),
		CAFile: filepath.Join(dir, "ca.pem"), ClientCertFile: filepath.Join(dir, "client.pem"), ClientKeyReference: "REPAIR_M934_CLIENT_KEY", ServerCertFile: filepath.Join(dir, "server.pem"), ServerKeyReference: "REPAIR_M934_SERVER_KEY", MaxAttempts: 10, MaximumReads: 8,
	}
	binding.CASHA256 = writeBound(t, binding.CAFile, pki.caPEM)
	binding.ClientCertSHA256 = writeBound(t, binding.ClientCertFile, client.certPEM)
	binding.ServerCertSHA256 = writeBound(t, binding.ServerCertFile, server.certPEM)
	t.Setenv(binding.ClientKeyReference, string(client.keyPEM))
	t.Setenv(binding.ServerKeyReference, string(server.keyPEM))
	t.Setenv(binding.CursorKeyReference, base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("c", 32))))
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RankingRuntimeFile: filepath.Join(dir, "runtime.json"), RankingTickIntervalMS: 30000, RankingNewTickIntervalMS: 10000}
	cfg.SecretsProvider = "env"
	cfg.RankingRuntimeSHA256 = writeBound(t, cfg.RankingRuntimeFile, raw)
	roots := x509.NewCertPool()
	roots.AddCert(pki.caCert)
	f := &repairM934Fixture{cfg: cfg, path: filepath.Join(dir, "publication.json"), cutoff: cutoff, gateway: gateway, roots: roots, authority: authority}
	state := repairM934State{Snapshots: map[string]*repairM934Stored{}, Heads: map[string]string{}, Frames: map[string]json.RawMessage{}, Cutoff: f.cutoff.Unix()}
	raw, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.close)
	f.open(t)
	return f
}

func (f *repairM934Fixture) open(t *testing.T) {
	t.Helper()
	pool := repairM934OpenPool(t, f.path)
	// Cache loss must leave durable publication readable. A closed production
	// client exercises that existing path without requiring a Redis process.
	cache := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	a, err := newNativeApp(context.Background(), f.cfg, quietLogger(), pool, cache, sharedprocess.NewTracker(), nil, map[string]sharedhttp.Check{}, nil)
	if err != nil {
		pool.Close()
		t.Fatalf("native application prerequisites: %v", err)
	}
	f.app = a
	f.server = httptest.NewUnstartedServer(a.Router)
	f.server.TLS = a.native.serverTLS.Clone()
	f.server.StartTLS()
	f.client = &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: f.roots, ServerName: rankingDNS, Certificates: []tls.Certificate{f.gateway.pair(t)}}}}
}

func (f *repairM934Fixture) close() {
	if f.client != nil {
		f.client.CloseIdleConnections()
		f.client = nil
	}
	if f.server != nil {
		f.server.Close()
		f.server = nil
	}
	if f.app != nil {
		f.app.Close()
		f.app = nil
	}
}

func (f *repairM934Fixture) get(t *testing.T, path string) (int, []byte, http.Header) {
	t.Helper()
	response, err := f.client.Get(f.server.URL + path)
	if err != nil {
		t.Fatalf("consume %s: %v", path, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, raw, response.Header
}

func TestRepairM_934_PublicationEveryCategoryAndRestartRecovery(t *testing.T) {
	f := repairM934NewFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	input, err := f.app.native.reader.MarshalUniverse(ctx, f.app.pool, uuid.NewString(), f.cutoff)
	if err != nil {
		t.Fatalf("canonical input transport: %v", err)
	}
	var frozen ranker.InputSnapshot
	if err = json.Unmarshal(input, &frozen); err != nil {
		t.Fatal(err)
	}
	if !frozen.Complete || len(frozen.Candidates) != 61 || frozen.Provenance == nil || frozen.Provenance.UniverseDigest != ranker.CanonicalUniverse(frozen.Candidates) || len(frozen.Provenance.ModerationRevision) != 64 {
		t.Fatalf("incomplete canonical input: %+v", frozen)
	}
	hidden := fmt.Sprintf("0x%040x", 61)
	if !frozen.Candidates[60].Suppressed {
		t.Fatal("social moderation verdict was not frozen")
	}
	var produced []ranker.Snapshot
	for _, newOnly := range []bool{false, true} {
		snapshots, e := f.app.native.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, newOnly)
		if e != nil {
			t.Fatalf("native publication newOnly=%v: %v", newOnly, e)
		}
		expected := len(ranker.HeavyCategoryNames())
		if newOnly {
			expected = 1
		}
		if len(snapshots) != expected {
			t.Fatalf("newOnly=%v produced %d categories, want %d", newOnly, len(snapshots), expected)
		}
		produced = append(produced, snapshots...)
	}
	if len(produced) != len(ranker.Categories) || len(produced) != 8 {
		t.Fatalf("frozen category set changed: %d", len(produced))
	}
	want := map[string]v1.RankingSnapshot{}
	headers := map[string]ranker.Header{}
	for _, snapshot := range produced {
		category := snapshot.Category()
		if _, duplicate := want[category]; duplicate {
			t.Fatalf("duplicate category %s", category)
		}
		entries := snapshot.Entries()
		if len(entries) == 0 {
			t.Fatalf("%s publication is vacuous", category)
		}
		h, e := f.app.native.publication.Current(ctx, category)
		if e != nil {
			t.Fatal(e)
		}
		if h.RunID != snapshot.RunID() || h.InputDigest != snapshot.InputDigest() || h.OutputDigest != snapshot.OutputDigest() || h.Revision != 1 || h.Total != int64(len(entries)) || h.Provenance.ModerationRevision != frozen.Provenance.ModerationRevision {
			t.Fatalf("%s head differs from producer: %+v", category, h)
		}
		items := make([]ranker.RankedItem, len(entries))
		for i, entry := range entries {
			if entry.Address == hidden {
				t.Fatalf("hidden pool published in %s", category)
			}
			items[i] = ranker.RankedItem{Address: entry.Address, Score: strconv.FormatFloat(entry.Score, 'f', -1, 64), Rank: int64(i + 1)}
		}
		page, e := f.app.native.publication.Page(ctx, h, 0, 100)
		if e != nil || !reflect.DeepEqual(page.Items, items) {
			t.Fatalf("%s stored entries differ from producer: %v", category, e)
		}
		wire, e := store.WireSnapshot(h, items)
		if e != nil {
			t.Fatal(e)
		}
		want[category], headers[category] = wire, h
	}
	assertRecovered := func() {
		t.Helper()
		if e := f.app.native.ready(ctx); e != nil {
			t.Fatalf("native readiness: %v", e)
		}
		for _, category := range ranker.Categories {
			h, e := f.app.native.publication.Current(ctx, category)
			if e != nil || !reflect.DeepEqual(h, headers[category]) {
				t.Fatalf("%s durable head changed across restart: %v", category, e)
			}
			code, raw, responseHeaders := f.get(t, "/rankings/"+category+"?limit=100")
			var response v1.RankingsResponse
			if code != http.StatusOK || json.Unmarshal(raw, &response) != nil {
				t.Fatalf("%s consumption=%d %s", category, code, raw)
			}
			if response.Quality != "complete" || response.Revision == nil || string(*response.Revision) != "1" || responseHeaders.Get("X-Ranking-Snapshot") != headers[category].RunID || !reflect.DeepEqual(response.Data.Snapshot, want[category]) {
				t.Fatalf("%s consumed response differs from producer: %s", category, raw)
			}
		}
	}
	assertCount := func(table string, want int) {
		t.Helper()
		var got int
		if e := f.app.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&got); e != nil || got != want {
			t.Fatalf("%s count=%d, want %d: %v", table, got, want, e)
		}
	}
	assertRecovered()
	assertCount("ranking.publication_outbox WHERE published_at IS NULL", len(ranker.Categories))
	// Reopen every application component and the persisted storage peer before
	// publication, proving that durable intents and heads survive shutdown.
	f.close()
	f.open(t)
	assertRecovered()
	writer := delivery.StreamWriter{Schema: "api_stream"}
	for i := 0; i < 3; i++ {
		worked, e := f.app.native.publication.CommitStreamOnce(ctx, writer, 10)
		if e != nil || !worked {
			t.Fatalf("initial stream commit %d: %v", i, e)
		}
	}
	f.close()
	f.open(t)
	assertCount("ranking.publication_outbox WHERE published_at IS NULL", len(ranker.Categories)-3)
	for i := 3; i < len(ranker.Categories); i++ {
		worked, e := f.app.native.publication.CommitStreamOnce(ctx, writer, 10)
		if e != nil || !worked {
			t.Fatalf("recovered stream commit %d: %v", i, e)
		}
	}
	if worked, e := f.app.native.publication.CommitStreamOnce(ctx, writer, 10); e != nil || worked {
		t.Fatalf("stream drain duplicated publication: worked=%v err=%v", worked, e)
	}
	assertCount("ranking.publication_outbox WHERE published_at IS NULL", 0)
	assertCount("api_stream.stream_delivery_outbox", len(ranker.Categories))
	for _, category := range ranker.Categories {
		var frame []byte
		var sequence int64
		if e := f.app.pool.QueryRow(ctx, "SELECT sequence,frame FROM api_stream.stream_projection WHERE topic=$1", "market.rankings."+category).Scan(&sequence, &frame); e != nil {
			t.Fatal(e)
		}
		if _, e := delivery.ParseCommittedStreamFrame(frame); e != nil {
			t.Fatalf("%s committed stream consumer: %v", category, e)
		}
		var event v1.RankingEvent
		if e := json.Unmarshal(frame, &event); e != nil {
			t.Fatal(e)
		}
		if sequence != 1 || event.Sequence != "1" || event.Topic != "market.rankings."+category || !reflect.DeepEqual(event.Payload, want[category]) {
			t.Fatalf("%s committed consumer differs from producer", category)
		}
	}
	assertRecovered()
	// Losing the authority must also refuse reads of already recovered heads.
	f.authority.Store(false)
	if e := f.app.native.ready(ctx); e == nil {
		t.Fatal("recovered heads bypassed unavailable visibility authority")
	}
	for _, category := range ranker.Categories {
		code, raw, _ := f.get(t, "/rankings/"+category)
		if code != http.StatusServiceUnavailable || !strings.Contains(string(raw), "RANKING_UNAVAILABLE") {
			t.Fatalf("recovered %s bypassed visibility authority: %d %s", category, code, raw)
		}
	}
}

func TestRepairM_934_NoAuthorityRefusesPublicationAndReads(t *testing.T) {
	f := repairM934NewFixture(t)
	f.authority.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, newOnly := range []bool{false, true} {
		if _, err := f.app.native.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, newOnly); err == nil {
			t.Fatalf("publication without authority admitted newOnly=%v", newOnly)
		}
	}
	if err := f.app.native.ready(ctx); err == nil {
		t.Fatal("readiness without authority admitted")
	}
	for _, category := range ranker.Categories {
		code, raw, _ := f.get(t, "/rankings/"+category)
		if code != http.StatusServiceUnavailable || !strings.Contains(string(raw), "RANKING_UNAVAILABLE") {
			t.Fatalf("%s without authority=%d %s", category, code, raw)
		}
	}
	for _, table := range []string{"ranking.category_heads", "ranking.snapshots", "ranking.publication_outbox", "api_stream.stream_delivery_outbox"} {
		var count int
		if err := f.app.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("publication without authority left %s=%d: %v", table, count, err)
		}
	}
}

func TestRepairM_934_VisibilityRequiresMutualTLSAndPinnedIdentity(t *testing.T) {
	f := repairM934NewFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := f.app.native.visibility.ReadRankingVisibility(ctx); err != nil {
		t.Fatalf("declared mTLS peer prerequisite: %v", err)
	}
	transport := f.app.native.client.Transport.(*http.Transport)
	noCertificate := transport.Clone()
	noCertificate.TLSClientConfig.Certificates = nil
	c := &rankingVisibilityClient{endpoint: f.app.native.visibility.endpoint, client: &http.Client{Transport: noCertificate, Timeout: time.Second}}
	defer c.client.CloseIdleConnections()
	if _, err := c.ReadRankingVisibility(ctx); err == nil {
		t.Fatal("visibility accepted without a client certificate")
	}
	wrongPin := transport.Clone()
	wrongPin.TLSClientConfig.ServerName = "other.railway.internal"
	c = &rankingVisibilityClient{endpoint: f.app.native.visibility.endpoint, client: &http.Client{Transport: wrongPin, Timeout: time.Second}}
	defer c.client.CloseIdleConnections()
	if _, err := c.ReadRankingVisibility(ctx); err == nil {
		t.Fatal("visibility accepted a different server identity")
	}
	var binding config.RankingRuntimeBinding
	if err := config.ReadBoundDocument(f.cfg.RankingRuntimeFile, f.cfg.RankingRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	binding.VisibilityCertSHA256 = strings.Repeat("0", 64)
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	f.close()
	f.cfg.RankingRuntimeSHA256 = writeBound(t, f.cfg.RankingRuntimeFile, raw)
	f.open(t)
	if _, err := f.app.native.visibility.ReadRankingVisibility(ctx); err == nil {
		t.Fatal("visibility accepted a mismatched server certificate pin")
	}
}
