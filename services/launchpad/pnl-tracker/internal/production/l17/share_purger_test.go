package l17

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestCloudflareSharePurgeChecksOrigin(t *testing.T) {
	const id = "7a6b5c4d-3e2f-4a1b-8c9d-0e1f2a3b4c5d"
	const zone = "0123456789abcdef0123456789abcdef"
	var mu sync.Mutex
	providerStatus := http.StatusOK
	originStatus := http.StatusGone
	var requests []string
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			if r.Host != "api.cloudflare.com" || r.Header.Get("Authorization") != "Bearer fixture-token" || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("provider request headers: %s %s", r.Host, r.Header)
			}
			var body struct {
				Files []string `json:"files"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Files) != 2 || body.Files[0] != server.URL+"/v3/portfolio/shares/"+id || body.Files[1] != body.Files[0]+"/image.png" {
				t.Errorf("provider purge body: %+v, %v", body, err)
			}
			w.WriteHeader(providerStatus)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}
		w.WriteHeader(originStatus)
	}))
	defer server.Close()
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}
	transport.ForceAttemptHTTP2 = false
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()
	purger := CloudflareSharePurgerV3{ZoneID: zone, Token: "fixture-token", PublicOrigin: server.URL, Client: client}
	if err := purger.PurgeShare(context.Background(), id); err != nil {
		t.Fatalf("purge and origin disappearance: %v", err)
	}
	want := []string{"POST /client/v4/zones/" + zone + "/purge_cache", "GET /v3/portfolio/shares/" + id, "GET /v3/portfolio/shares/" + id + "/image.png"}
	mu.Lock()
	observed := append([]string(nil), requests...)
	mu.Unlock()
	if len(observed) != len(want) {
		t.Fatalf("requests = %v", observed)
	}
	for i := range want {
		if observed[i] != want[i] {
			t.Fatalf("request %d = %q, want %q", i, observed[i], want[i])
		}
	}
	mu.Lock()
	providerStatus = http.StatusBadGateway
	mu.Unlock()
	if err := purger.PurgeShare(context.Background(), id); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("provider rejection = %v", err)
	}
	mu.Lock()
	providerStatus = http.StatusOK
	originStatus = http.StatusOK
	mu.Unlock()
	if err := purger.PurgeShare(context.Background(), id); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("still visible at origin = %v", err)
	}
}
