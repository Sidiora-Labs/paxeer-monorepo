package l17

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
)

func TestRevisionCacheBoundsAndPortfolioReuse(t *testing.T) {
	ctx := context.Background()
	client := internaltest.NewRedis(t)
	now := time.Now().UTC()
	id, err := opaqueID()
	if err != nil {
		t.Fatal(err)
	}
	key := "portfolio-cache-test:" + id
	t.Cleanup(func() {
		if err := client.Del(ctx, key).Err(); err != nil {
			t.Error(err)
		}
	})
	cache := RevisionCache{Client: client, TTL: 10 * time.Second}
	if _, found, err := cache.Get(ctx, key, now); err != nil || found {
		t.Fatal(found, err)
	}
	value := PortfolioResponse{Algorithm: Algorithm, QuoteAsset: "USDL", Decimals: 6}
	if err = cache.SetUntil(ctx, key, value, now, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if got, found, err := cache.Get(ctx, key, now); err != nil || !found || got.Algorithm != Algorithm {
		t.Fatal(got, found, err)
	}
	for _, at := range []time.Time{now.Add(-time.Nanosecond), now.Add(time.Second)} {
		if _, found, err := cache.Get(ctx, key, at); err != nil || found {
			t.Fatal(found, err)
		}
	}
	for _, ttl := range []time.Duration{0, 11 * time.Second} {
		bad := cache
		bad.TTL = ttl
		if err := bad.Set(ctx, key, value, now); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if err = cache.SetUntil(ctx, key, value, now, now); !errors.Is(err, ErrRestart) {
		t.Fatal(err)
	}
	if err = (RevisionCache{}).Set(ctx, key, value, now); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, _, err = (RevisionCache{}).Get(ctx, key, now); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, raw := range []string{"invalid", strings.Repeat("x", (1<<20)+1)} {
		if err = client.Set(ctx, key, raw, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		if _, _, err = cache.Get(ctx, key, now); err == nil {
			t.Fatal("malformed cache accepted")
		}
	}
	repository := phase2Repository(t)
	summary := phase2Commit(t, repository, "cached-source", "1", 1)
	service := Service{Repository: repository, Cache: &cache, CursorKey: []byte(strings.Repeat("k", 32))}
	first, err := service.Portfolio(ctx, wallet, "", 10, now)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Portfolio(ctx, wallet, "", 10, now)
	if err != nil || again.Page.Summary.SnapshotID != summary.SnapshotID || len(again.Page.Positions) != len(first.Page.Positions) {
		t.Fatal(again, err)
	}
	base, err := CacheKey(wallet, "", "1", false)
	if err != nil {
		t.Fatal(err)
	}
	portfolioKey := base + ":" + summary.SnapshotID + ":-1:10"
	t.Cleanup(func() {
		if err := client.Del(ctx, portfolioKey).Err(); err != nil {
			t.Error(err)
		}
	})
	stale := again
	stale.Display = &USDDisplay{UsdlRaw: "1", UsdMicro: ptr("1"), Quote: &DisplayQuote{Numerator: "1", Denominator: "1", Provider: "display", ObservedAt: now.Add(-time.Minute)}}
	if err = cache.Set(ctx, portfolioKey, stale, now); err != nil {
		t.Fatal(err)
	}
	service.Display = &DisplayProvider{MaxAge: time.Second}
	final, err := service.Portfolio(ctx, wallet, "", 10, now)
	if err != nil || final.Display == nil || final.Display.UsdMicro != nil || !final.Display.Quote.Stale {
		t.Fatal(final, err)
	}
}

func TestDisplayProviderRequiresFreshBoundQuote(t *testing.T) {
	now := time.Now().UTC()
	ctx := context.Background()
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"fresh", `{"numerator":"3","denominator":"2","provider":"display","observedAt":"` + now.Format(time.RFC3339Nano) + `","stale":false}`, 200, true},
		{"provider", `{"numerator":"1","denominator":"1","provider":"other","observedAt":"` + now.Format(time.RFC3339Nano) + `"}`, 200, false},
		{"stale", `{"numerator":"1","denominator":"1","provider":"display","observedAt":"` + now.Add(-time.Hour).Format(time.RFC3339Nano) + `"}`, 200, false},
		{"future", `{"numerator":"1","denominator":"1","provider":"display","observedAt":"` + now.Add(time.Hour).Format(time.RFC3339Nano) + `"}`, 200, false},
		{"denominator", `{"numerator":"1","denominator":"0","provider":"display","observedAt":"` + now.Format(time.RFC3339Nano) + `"}`, 200, false},
		{"malformed", `{`, 200, false}, {"trailing", `{} {}`, 200, false}, {"unavailable", `{}`, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Error(r.Method)
				}
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			provider := DisplayProvider{Endpoint: server.URL, ProviderID: "display", Client: server.Client(), MaxAge: time.Minute}
			got := provider.Convert(ctx, "1000000", now)
			if got.UsdlRaw != "1000000" || (got.UsdMicro != nil) != tc.valid {
				t.Fatal(got)
			}
			if tc.valid && *got.UsdMicro != "1500000" {
				t.Fatal(got)
			}
		})
	}
	for _, endpoint := range []string{"http://display.example", "https://user@display.example", "https://display.example/#fragment", "%%%"} {
		if got := (DisplayProvider{Endpoint: endpoint, ProviderID: "display", MaxAge: time.Minute}).Convert(ctx, "1", now); got.UsdMicro != nil || got.Quote != nil {
			t.Fatal(got)
		}
	}
}

func TestCloudflareCardPurgeConfirmsGoneOrigin(t *testing.T) {
	const id = "7a6b5c4d-3e2f-4a1b-8c9d-0e1f2a3b4c5d"
	const zone = "0123456789abcdef0123456789abcdef"
	var mu sync.Mutex
	providerStatus := 200
	originStatus := 410
	success := true
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == "POST" {
			var body struct {
				Files []string `json:"files"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Files) != 1 || body.Files[0] != server.URL+"/v2/portfolio/cards/"+id+"/image.png" || r.Header.Get("Authorization") != "Bearer purge-token" {
				t.Error(body, err)
			}
			w.WriteHeader(providerStatus)
			if err := json.NewEncoder(w).Encode(map[string]bool{"success": success}); err != nil {
				t.Error(err)
			}
			return
		}
		w.WriteHeader(originStatus)
	}))
	defer server.Close()
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	transport.ForceAttemptHTTP2 = false
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}
	defer transport.CloseIdleConnections()
	purger := CloudflarePurger{ZoneID: zone, Token: "purge-token", PublicOrigin: server.URL, Client: &http.Client{Transport: transport}}
	if err := purger.PurgeCard(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		provider, origin int
		success          bool
	}{{503, 410, true}, {200, 200, true}, {200, 410, false}} {
		mu.Lock()
		providerStatus, originStatus, success = tc.provider, tc.origin, tc.success
		mu.Unlock()
		if err := purger.PurgeCard(context.Background(), id); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
	bad := purger
	bad.PublicOrigin = "http://public.example"
	if err := bad.PurgeCard(context.Background(), id); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := purger.PurgeCard(context.Background(), "invalid"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
