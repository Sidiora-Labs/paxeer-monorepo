package app

import (
	"context"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/go-chi/chi/v5"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestNetworkReadRejectsInvalidDocuments(t *testing.T) {
	for _, data := range []any{make(chan int), map[string]any{"invalid": true}} {
		w := httptest.NewRecorder()
		writeNetworkEnvelope(w, httptest.NewRequest(http.MethodGet, "/v2/network/deployment", nil), "DeploymentResponse", data, "partial", "no-store")
		if w.Code != 503 || !strings.Contains(w.Body.String(), "INTEGRITY_QUARANTINED") {
			t.Fatalf("document response: %d %s", w.Code, w.Body.String())
		}
	}
	d := networkReads{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), manifest: deployment.Manifest{}}
	w := httptest.NewRecorder()
	d.deployment(w, httptest.NewRequest(http.MethodGet, "/v2/network/deployment", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "INTEGRITY_QUARANTINED") {
		t.Fatalf("manifest response: %d %s", w.Code, w.Body.String())
	}
	for _, endpoint := range []string{"", "://bad", "http://example.com", "https://user:pass@example.com", "https://example.com/#fragment", "ftp://example.com"} {
		client, err := dialNetworkRPC(context.Background(), endpoint, false)
		if client != nil {
			client.Close()
		}
		if err == nil {
			t.Fatalf("invalid endpoint accepted: %q", endpoint)
		}
	}
	for _, remote := range []string{"invalid", "host:1", "127.0.0.1:12"} {
		if trustedPeer(remote, []string{"invalid"}) {
			t.Fatalf("untrusted peer accepted: %s", remote)
		}
	}
	for _, cfg := range []config.Config{{}, {NATSURL: "tls://127.0.0.1:1"}, {NATSURL: "nats://127.0.0.1:1", NATSCredentials: "missing"}, {NATSURL: "tls://127.0.0.1:1", NATSCredentialsCreds: "invalid"}} {
		nc, err := connectRuntimeNATS(&cfg)
		if nc != nil {
			nc.Close()
		}
		if err == nil {
			t.Fatal("invalid broker configuration admitted")
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/v2/cups/current?extra=1", nil)
	w = httptest.NewRecorder()
	currentCupEmpty(w, req)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "QUERY_INVALID") {
		t.Fatalf("cup query: %d %s", w.Code, w.Body.String())
	}
}

func TestCommittedNetworkHeadValidatesSourceIdentity(t *testing.T) {
	db := internaltest.NewPostgres(t)
	ctx := context.Background()
	engine := contractAuthority(t).engine
	a := &networkAuthority{pool: db, engine: engine, config: networkAuthorityConfig{SourceSchema: "indexer"}, origins: map[string]bool{"https://example.com": true}}
	if _, err := a.head(ctx); err == nil {
		t.Fatal("missing head source accepted")
	}
	if _, err := db.Exec(ctx, `CREATE TABLE indexer.indexer_committed_heads_v2(chain_id integer,height text,block_hash text,integrity text)`); err != nil {
		t.Fatal(err)
	}
	const hash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		height, hash, integrity string
		valid                   bool
	}{
		{"42", hash, "healthy", true}, {"-1", hash, "healthy", false}, {"9007199254740992", hash, "healthy", false}, {"42", "invalid", "healthy", false}, {"42", hash, "quarantined", false},
	} {
		if _, err := db.Exec(ctx, `DELETE FROM indexer.indexer_committed_heads_v2;`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads_v2 VALUES(125,$1,$2,$3)`, tc.height, tc.hash, tc.integrity); err != nil {
			t.Fatal(err)
		}
		head, err := a.head(ctx)
		if tc.valid {
			if err != nil || head.Number != "42" || head.Hash != hash {
				t.Fatalf("head: %+v %v", head, err)
			}
		} else if err == nil {
			t.Fatalf("invalid head accepted: %+v", tc)
		}
	}
	admission, err := rest.NewAdmission(20, 20, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	a.Register(router, admission)
	for _, body := range []string{strings.Repeat("x", 16<<10+1), `{"family":"creation","extra":1}`, `{"family":"creation"}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "https://example.com/v2/preflight/create", strings.NewReader(body))
		r.Header.Set("Origin", "https://example.com")
		router.ServeHTTP(w, r)
		want := 422
		if body == `{"family":"creation"}` {
			want = 503
		}
		if w.Code != want {
			t.Fatalf("preflight status: %d %s", w.Code, w.Body.String())
		}
	}
}
