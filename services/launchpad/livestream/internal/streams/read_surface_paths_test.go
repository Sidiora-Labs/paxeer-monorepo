package streams

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicReadWiringServesStoreAndRefusesBroadcast(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if err := ReadSchemaReady(ctx, f.database, "livestream"); err != nil {
		t.Fatal(err)
	}
	if err := ReadSchemaReady(ctx, f.database, "not-a-schema"); err == nil {
		t.Fatal("invalid schema accepted")
	}
	key := []byte(strings.Repeat("v", 32))
	deps, err := NewPublicReads(PublicReadOptions{Pool: f.database, Redis: f.redis, WalletLimit: 3, GlobalLimit: 1, ViewerTokenKey: key})
	if err != nil || deps.Lifecycle == nil || deps.Viewers == nil || deps.ViewerTokens == nil || !deps.BroadcastUnprovisioned {
		t.Fatalf("public read dependencies = %+v, %v", deps, err)
	}
	key[0] = 'x'
	if deps.ViewerTokens.Key[0] != 'v' {
		t.Fatal("viewer token signing key aliases caller memory")
	}
	if _, err := NewPublicReads(PublicReadOptions{Pool: f.database, Redis: f.redis, WalletLimit: 3, ViewerTokenKey: []byte("short")}); err == nil {
		t.Fatal("short production viewer key accepted")
	}
	development, err := NewDevelopmentReads(DevelopmentReadOptions{Pool: f.database, Redis: f.redis, WalletLimit: 3})
	if err != nil || len(development.ViewerTokens.Key) != 32 {
		t.Fatalf("development viewer key = %+v, %v", development.ViewerTokens, err)
	}
	router := http.NewServeMux()
	router.HandleFunc("POST /v2/streams", New(deps).provisioned(func(w http.ResponseWriter, r *http.Request) {
		t.Error("broadcast handler ran in read-only mode")
	}))
	server := httptest.NewServer(router)
	defer server.Close()
	response, err := http.Post(server.URL+"/v2/streams", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("broadcast refusal = %d, %v", response.StatusCode, response.Header)
	}
}

func TestPublicReadsRequireModerationAuthorityForStoredStreams(t *testing.T) {
	f := repair57New(t)
	o, intent := f.create("read-restriction-authority")
	f.handlers.restrictions = nil
	for _, path := range []string{
		"/v2/streams/" + o.ID,
		"/v2/streams/pool/" + intent.Pool,
		"/v2/streams?poolAddress=" + intent.Pool,
	} {
		response, err := http.Get(f.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if response.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("read without moderation authority %s = %d", path, response.StatusCode)
		}
	}
}
