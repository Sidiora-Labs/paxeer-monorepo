package http

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// sharedTestRedisURL returns the already running Redis logical database named
// by SHARED_TEST_REDIS_URL, flushed before and after the test; the test fails
// immediately when it is unset or names no database (containers are never
// started).
func sharedTestRedisURL(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("SHARED_TEST_REDIS_URL")
	if uri == "" {
		t.Fatal("SHARED_TEST_REDIS_URL is not set: point it at a running Redis logical database reserved for these tests (redis://:password@host:port/N); tests never start containers")
	}
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse SHARED_TEST_REDIS_URL: %v", err)
	}
	if db, convErr := strconv.Atoi(strings.Trim(u.Path, "/")); convErr != nil || db < 0 || db > 15 {
		t.Fatal("SHARED_TEST_REDIS_URL must name a logical database 0..15 reserved for these tests (redis://:password@host:port/N)")
	}
	opt, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatalf("parse SHARED_TEST_REDIS_URL: %v", err)
	}
	flush := func() {
		client := goredis.NewClient(opt)
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.FlushDB(ctx).Err(); err != nil {
			t.Errorf("flush SHARED_TEST_REDIS_URL: %v", err)
		}
	}
	flush()
	t.Cleanup(flush)
	return uri
}

func TestNewRouterHealth(t *testing.T) {
	t.Parallel()
	r := NewRouter(ServerOptions{})
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Errorf("/health body = %s", body)
	}
}

func TestCORSHeader(t *testing.T) {
	t.Parallel()
	r := NewRouter(ServerOptions{CORSOrigins: "https://sidiora.fun"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://sidiora.fun")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://sidiora.fun" {
		t.Errorf("ACAO = %q", got)
	}
}

func TestRouterExposesAnswerHeadersAcrossOrigins(t *testing.T) {
	t.Parallel()
	const origin = "https://sidiora.fun"
	const exposed = "X-Schema-Version, Retry-After, X-Request-Id, Etag"
	router := NewRouter(ServerOptions{CORSOrigins: origin})
	router.Get("/unavailable", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		name, method, path, origin, exposed, retryAfter string
		status                                          int
	}{
		{"success", http.MethodGet, "/health", origin, exposed, "", http.StatusOK},
		{"unavailable", http.MethodGet, "/unavailable", origin, exposed, "7", http.StatusServiceUnavailable},
		{"success without origin", http.MethodGet, "/health", "", "", "", http.StatusOK},
		{"unavailable without origin", http.MethodGet, "/unavailable", "", "", "7", http.StatusServiceUnavailable},
		{"disallowed origin", http.MethodGet, "/health", "https://other.example", "", "", http.StatusOK},
		{"preflight", http.MethodOptions, "/unavailable", origin, "", "", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequestWithContext(context.Background(), tc.method, server.URL+tc.path, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			if tc.method == http.MethodOptions {
				request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
				request.Header.Set("Access-Control-Request-Headers", "Authorization, X-Client-Header")
			}
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.status)
			}
			values := response.Header.Values("Access-Control-Expose-Headers")
			if tc.exposed == "" {
				if len(values) != 0 {
					t.Errorf("exposed headers = %v, want none", values)
				}
			} else if len(values) != 1 || values[0] != tc.exposed {
				t.Errorf("exposed headers = %v, want [%s]", values, tc.exposed)
			}
			if got := response.Header.Get("Retry-After"); got != tc.retryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tc.retryAfter)
			}
			wantOrigin, wantCredentials := "", ""
			if tc.origin == origin {
				wantOrigin, wantCredentials = origin, "true"
			}
			if got := response.Header.Get("Access-Control-Allow-Origin"); got != wantOrigin {
				t.Errorf("allow origin = %q, want %q", got, wantOrigin)
			}
			if got := response.Header.Get("Access-Control-Allow-Credentials"); got != wantCredentials {
				t.Errorf("allow credentials = %q, want %q", got, wantCredentials)
			}
			if tc.method == http.MethodOptions {
				for header, want := range map[string]string{
					"Access-Control-Allow-Methods": http.MethodPatch,
					"Access-Control-Allow-Headers": "Authorization, X-Client-Header",
					"Access-Control-Max-Age":       "300",
				} {
					if got := response.Header.Get(header); got != want {
						t.Errorf("%s = %q, want %q", header, got, want)
					}
				}
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				if len(body) != 0 {
					t.Errorf("preflight body = %q, want empty", body)
				}
			}
		})
	}
}

func TestResolveOrigins(t *testing.T) {
	t.Parallel()
	if got := resolveOrigins(""); len(got) != 1 || got[0] != "*" {
		t.Errorf("empty -> %v", got)
	}
	if got := resolveOrigins("*"); got[0] != "*" {
		t.Errorf("star -> %v", got)
	}
	if got := resolveOrigins(" a.com , b.com "); len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
		t.Errorf("list -> %v", got)
	}
	if got := resolveOrigins(" , "); len(got) != 1 || got[0] != "*" {
		t.Errorf("blank list -> %v", got)
	}
}

func TestReadyHealthy(t *testing.T) {
	t.Parallel()
	r := NewRouter(ServerOptions{Health: HealthDeps{
		DB:    func(context.Context) error { return nil },
		Redis: func(context.Context) error { return nil },
		Custom: map[string]Check{
			"upstream": func(context.Context) error { return nil },
		},
	}})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"database":"ok"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestReadyDegraded(t *testing.T) {
	t.Parallel()
	r := NewRouter(ServerOptions{Health: HealthDeps{
		DB: func(context.Context) error { return errors.New("down") },
	}})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"degraded"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestReadyCustomCheckFails(t *testing.T) {
	t.Parallel()
	r := NewRouter(ServerOptions{Health: HealthDeps{
		DB: func(context.Context) error { return nil },
		Custom: map[string]Check{
			"upstream": func(context.Context) error { return errors.New("upstream down") },
		},
	}})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("custom-fail ready = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"upstream":"failed"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestWriteErrorMasks5xx(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusInternalServerError, "secret detail", "leaky message")
	if strings.Contains(rec.Body.String(), "leaky") || strings.Contains(rec.Body.String(), "secret detail") {
		t.Errorf("5xx leaked internals: %s", rec.Body.String())
	}
	rec2 := httptest.NewRecorder()
	WriteError(rec2, http.StatusBadRequest, "Bad Request", "field x invalid")
	if !strings.Contains(rec2.Body.String(), "field x invalid") {
		t.Errorf("4xx should keep message: %s", rec2.Body.String())
	}
}

func TestNotFoundAndMethodHandlers(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	NotFoundHandler()(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("notfound = %d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	MethodNotAllowedHandler()(rec2, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec2.Code != http.StatusMethodNotAllowed {
		t.Errorf("methodnotallowed = %d", rec2.Code)
	}
}

func TestRateLimitInMemory(t *testing.T) {
	t.Parallel()
	mw := RateLimit(RateLimitOptions{Max: 2, Window: time.Minute})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	codes := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
		if i == 0 && rec.Header().Get("X-RateLimit-Limit") != "2" {
			t.Errorf("limit header = %q", rec.Header().Get("X-RateLimit-Limit"))
		}
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes = %v, want [200 200 429]", codes)
	}
}

func TestRateLimitRoutePrefixBypass(t *testing.T) {
	t.Parallel()
	mw := RateLimit(RateLimitOptions{Max: 1, Window: time.Minute, RoutePrefix: "/api/"})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.RemoteAddr = "9.9.9.9:1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("non-prefixed route should bypass limit, got %d", rec.Code)
		}
	}
}

func TestAPIKeyAuth(t *testing.T) {
	t.Parallel()
	if _, err := APIKeyAuth(APIKeyOptions{}); err == nil {
		t.Fatal("empty key should error")
	}
	mw, err := APIKeyAuth(APIKeyOptions{APIKey: "s3cr3t"})
	if err != nil {
		t.Fatal(err)
	}
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	// health bypass
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/health bypass = %d", rec.Code)
	}
	// missing
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing key = %d, want 401", rec.Code)
	}
	// wrong
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("X-API-Key", "wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong key = %d, want 403", rec.Code)
	}
	// correct
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("X-API-Key", "s3cr3t")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("correct key = %d, want 200", rec.Code)
	}
}

func TestScanBufferNotConfigured(t *testing.T) {
	t.Setenv("CLAMAV_HOST", "")
	res := ScanBuffer(context.Background(), []byte("data"), ScanOptions{})
	if res.Clean || res.Verdict != ScanUnavailable || res.Reason != "clamav-not-configured" || res.ContentHash != "" || res.Size != 0 {
		t.Errorf("unconfigured scan = %+v", res)
	}
}

func TestScanBufferUnreachable(t *testing.T) {
	t.Parallel()
	// Port 1 is not listening, so the scan is unavailable.
	res := ScanBuffer(context.Background(), []byte("data"), ScanOptions{Host: "127.0.0.1", Port: 1, Timeout: 500 * time.Millisecond})
	if res.Clean || res.Verdict != ScanUnavailable || res.Reason != "clamav-unreachable" || res.ContentHash != "" || res.Size != 0 {
		t.Errorf("unreachable scan must remain unavailable: %+v", res)
	}
}

// fakeClamd starts a TCP server speaking enough of the clamd INSTREAM protocol
// to reply with a fixed response, then returns its host and port.
func fakeClamd(t *testing.T, reply string) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Drain the INSTREAM frames until the zero-length terminator.
		hdr := []byte("zINSTREAM\x00")
		buf := make([]byte, len(hdr))
		_, _ = io.ReadFull(conn, buf)
		for {
			var sz [4]byte
			if _, err := io.ReadFull(conn, sz[:]); err != nil {
				break
			}
			n := binary.BigEndian.Uint32(sz[:])
			if n == 0 {
				break
			}
			chunk := make([]byte, n)
			if _, err := io.ReadFull(conn, chunk); err != nil {
				break
			}
		}
		_, _ = conn.Write([]byte(reply))
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

func TestScanBufferClean(t *testing.T) {
	t.Parallel()
	host, port := fakeClamd(t, "stream: OK\x00")
	res := ScanBuffer(context.Background(), []byte("safe data"), ScanOptions{Host: host, Port: port, Timeout: 2 * time.Second})
	if !res.Clean {
		t.Errorf("expected clean, got %+v", res)
	}
}

func TestScanBufferInfected(t *testing.T) {
	t.Parallel()
	host, port := fakeClamd(t, "stream: Win.Test.EICAR_HDB-1 FOUND\x00")
	res := ScanBuffer(context.Background(), []byte("eicar"), ScanOptions{Host: host, Port: port, Timeout: 2 * time.Second})
	if res.Clean {
		t.Errorf("expected infected, got %+v", res)
	}
	if res.Verdict != ScanInfected || res.Reason != "malware-detected" || res.ContentHash != "" || res.Size != 0 {
		t.Errorf("infected scan = %+v", res)
	}
}

func TestRateLimitRedis(t *testing.T) {
	opt, err := goredis.ParseURL(sharedTestRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(opt)
	defer rdb.Close()

	mw := RateLimit(RateLimitOptions{Max: 2, Window: time.Minute, Redis: rdb})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	var codes []int
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.6.7.8:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("redis-backed codes = %v, want [200 200 429]", codes)
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:443"
	if got := clientIP(req); got != "8.8.8.8" {
		t.Errorf("clientIP = %q", got)
	}
	req.RemoteAddr = "no-port"
	if got := clientIP(req); got != "no-port" {
		t.Errorf("clientIP fallback = %q", got)
	}
}

func TestResolveClientIP(t *testing.T) {
	t.Parallel()
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		xRealIP    string
		trusted    []netip.Prefix
		want       string
	}{
		{"no trusted proxies ignores headers", "10.0.0.1:9000", "1.2.3.4", "1.2.3.4", nil, ""},
		{"untrusted peer ignores headers", "8.8.8.8:443", "1.2.3.4", "1.2.3.4", trusted, ""},
		{"trusted peer uses rightmost untrusted XFF", "10.0.0.1:9000", "1.2.3.4, 192.168.1.5, 10.0.0.1", "", trusted, "1.2.3.4"},
		{"trusted peer falls back to X-Real-IP", "192.168.1.1:80", "", "203.0.113.9", trusted, "203.0.113.9"},
		{"trusted peer, all hops trusted -> empty", "10.0.0.1:9000", "10.1.1.1, 192.168.0.9", "", trusted, ""},
		{"trusted peer, no headers -> empty", "10.0.0.1:9000", "", "", trusted, ""},
		{"bare host (no port) trusted peer", "10.0.0.2", "1.2.3.4", "", trusted, "1.2.3.4"},
		{"invalid chain does not use X-Real-IP", "10.0.0.1:9000", "invalid, 1.2.3.4", "203.0.113.9", trusted, ""},
		{"empty hop rejects chain", "10.0.0.1:9000", "1.2.3.4,,10.1.1.1", "", trusted, ""},
		{"all trusted chain does not use X-Real-IP", "10.0.0.1:9000", "10.1.1.1", "203.0.113.9", trusted, ""},
		{"invalid X-Real-IP", "10.0.0.1:9000", "", "not-an-ip", trusted, ""},
		{"canonical IPv6", "10.0.0.1:9000", "2001:0DB8:0:0:0:0:0:1", "", trusted, "2001:db8::1"},
		{"mapped trusted hop", "10.0.0.1:9000", "::ffff:1.2.3.4, ::ffff:10.1.1.1", "", trusted, "1.2.3.4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := http.Header{}
			if c.xff != "" {
				h.Set("X-Forwarded-For", c.xff)
			}
			if c.xRealIP != "" {
				h.Set("X-Real-IP", c.xRealIP)
			}
			if got := resolveClientIP(c.remoteAddr, h, c.trusted); got != c.want {
				t.Errorf("resolveClientIP(%q) = %q, want %q", c.remoteAddr, got, c.want)
			}
		})
	}
}

func TestRealIPMiddleware(t *testing.T) {
	t.Parallel()
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	var seen string
	h := realIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.RemoteAddr
	}))

	// Trusted peer: forwarded client IP is adopted.
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:5000"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "203.0.113.7" {
		t.Errorf("trusted peer RemoteAddr = %q, want 203.0.113.7", seen)
	}

	// Untrusted peer: RemoteAddr is left intact (no spoofing).
	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "8.8.8.8:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "8.8.8.8:443" {
		t.Errorf("untrusted peer RemoteAddr = %q, want 8.8.8.8:443 (unchanged)", seen)
	}
}
