package http

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func repairM975Trusted() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("fd00::/8"),
	}
}

func repairM975Headers(xff, real []string) http.Header {
	h := make(http.Header)
	if xff != nil {
		h["X-Forwarded-For"] = xff
	}
	if real != nil {
		h["X-Real-Ip"] = real
	}
	return h
}

type repairM975Identity struct {
	Remote string
	Client string
}

// Both entry points consume requests transported over loopback, through the
// production router and (when requested) the production in-memory rate limiter.
func repairM975Server(t *testing.T, trusted []netip.Prefix, limited bool) *httptest.Server {
	t.Helper()
	router := NewRouter(ServerOptions{TrustedProxies: trusted})
	router.Get("/identity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(repairM975Identity{
			Remote: r.RemoteAddr,
			Client: TrustedClientIP(r, trusted),
		}); err != nil {
			t.Errorf("encode identity: %v", err)
		}
	})
	var handler http.Handler = router
	if limited {
		handler = RateLimit(RateLimitOptions{
			Max: 1, Window: time.Minute,
			KeyFunc: func(r *http.Request) string { return TrustedClientIP(r, trusted) },
		})(router)
	}
	server := httptest.NewServer(handler)
	server.Client().Timeout = 5 * time.Second
	t.Cleanup(server.Close)
	return server
}

func repairM975Request(t *testing.T, server *httptest.Server, headers http.Header, wantStatus int, wantClient string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, server.URL+"/identity", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = headers
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		t.Fatalf("headers %v: status %d, want %d", headers, response.StatusCode, wantStatus)
	}
	if wantStatus == http.StatusTooManyRequests {
		var envelope struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != CodeRateLimited || response.Header.Get("X-RateLimit-Remaining") != "0" {
			t.Fatalf("missing exhausted-budget response: %+v, %v", envelope, response.Header)
		}
		return
	}
	var identity repairM975Identity
	if err := json.NewDecoder(response.Body).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	if identity.Client != wantClient {
		t.Fatalf("headers %v: client %q, want %q", headers, identity.Client, wantClient)
	}
	if wantClient == "127.0.0.1" {
		// Fail-closed forwarding leaves the socket peer intact in the router.
		host, _, err := net.SplitHostPort(identity.Remote)
		if err != nil || host != wantClient {
			t.Fatalf("fallback remote %q, want loopback socket peer", identity.Remote)
		}
	} else if identity.Remote != wantClient {
		t.Fatalf("router remote %q, want canonical %q", identity.Remote, wantClient)
	}
}

func TestRepairM_975_RouterForwardedValidation(t *testing.T) {
	server := repairM975Server(t, repairM975Trusted(), false)
	cases := []struct {
		name string
		xff  []string
		real []string
		want string
	}{
		{"no headers", nil, nil, "127.0.0.1"},
		{"trusted chain", []string{"198.51.100.8, 10.1.2.3, 192.168.1.2"}, nil, "198.51.100.8"},
		{"rightmost untrusted", []string{"203.0.113.99, 198.51.100.8, 10.1.2.3"}, nil, "198.51.100.8"},
		{"multiple chain fields", []string{"203.0.113.99, 198.51.100.8", "10.1.2.3, fd00::1"}, nil, "198.51.100.8"},
		{"mapped trusted hops", []string{"198.51.100.8, ::ffff:10.1.2.3, fd00:0:0:0:0:0:0:1"}, nil, "198.51.100.8"},
		{"canonical IPv6", []string{" 2001:0DB8:0000:0000:0000:0000:0000:0008 , 10.1.2.3"}, nil, "2001:db8::8"},
		{"mapped IPv4 client", []string{"::ffff:c633:6408, 10.1.2.3"}, nil, "198.51.100.8"},
		{"real IPv6", nil, []string{" 2001:0DB8:0:0:0:0:0:8 "}, "2001:db8::8"},
		{"real mapped IPv4", nil, []string{"::ffff:198.51.100.8"}, "198.51.100.8"},
		{"valid chain takes precedence", []string{"198.51.100.8, 10.1.2.3"}, []string{"invalid"}, "198.51.100.8"},
		{"invalid right hop", []string{"198.51.100.8, invalid"}, []string{"203.0.113.1"}, "127.0.0.1"},
		{"invalid left hop", []string{"invalid, 198.51.100.8, 10.1.2.3"}, []string{"203.0.113.1"}, "127.0.0.1"},
		{"invalid additional field", []string{"198.51.100.8", "invalid"}, nil, "127.0.0.1"},
		{"invalid first field", []string{"invalid", "198.51.100.8"}, nil, "127.0.0.1"},
		{"empty chain", []string{""}, []string{"203.0.113.1"}, "127.0.0.1"},
		{"blank chain", []string{" \t "}, []string{"203.0.113.1"}, "127.0.0.1"},
		{"empty middle hop", []string{"198.51.100.8,,10.1.2.3"}, nil, "127.0.0.1"},
		{"empty leading hop", []string{",198.51.100.8"}, nil, "127.0.0.1"},
		{"empty trailing hop", []string{"198.51.100.8,"}, nil, "127.0.0.1"},
		{"all trusted ignores real", []string{"10.1.2.3, fd00::1"}, []string{"203.0.113.1"}, "127.0.0.1"},
		{"IPv4 port", []string{"198.51.100.8:443"}, nil, "127.0.0.1"},
		{"IPv6 port", []string{"[2001:db8::8]:443"}, nil, "127.0.0.1"},
		{"IPv6 brackets", []string{"[2001:db8::8]"}, nil, "127.0.0.1"},
		{"scoped IPv6", []string{"fe80::8%eth0"}, nil, "127.0.0.1"},
		{"mapped scoped IPv4", []string{"::ffff:198.51.100.8%eth0"}, nil, "127.0.0.1"},
		{"hostname", []string{"client.example"}, nil, "127.0.0.1"},
		{"unknown", []string{"unknown"}, nil, "127.0.0.1"},
		{"IPv4 leading zeros", []string{"198.051.100.8"}, nil, "127.0.0.1"},
		{"bad real", nil, []string{"arbitrary-identity"}, "127.0.0.1"},
		{"empty real", nil, []string{""}, "127.0.0.1"},
		{"real port", nil, []string{"198.51.100.8:443"}, "127.0.0.1"},
		{"real zone", nil, []string{"fe80::8%eth0"}, "127.0.0.1"},
		{"real list", nil, []string{"198.51.100.8,203.0.113.1"}, "127.0.0.1"},
		{"duplicate real", nil, []string{"198.51.100.8", "203.0.113.1"}, "127.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repairM975Request(t, server, repairM975Headers(tc.xff, tc.real), http.StatusOK, tc.want)
		})
	}
}

func TestRepairM_975_OuterLimiterIdentity(t *testing.T) {
	cases := []struct {
		name     string
		identity string
		headers  []http.Header
	}{
		{"malformed chains share peer budget", "127.0.0.1", []http.Header{
			repairM975Headers([]string{"invalid-a"}, []string{"203.0.113.1"}),
			repairM975Headers([]string{"invalid-b"}, []string{"203.0.113.2"}),
			repairM975Headers([]string{"invalid,198.51.100.8,10.1.2.3"}, nil),
			repairM975Headers([]string{"198.51.100.8,,10.1.2.3"}, nil),
			repairM975Headers([]string{"198.51.100.8", "invalid"}, nil),
			repairM975Headers([]string{""}, []string{"203.0.113.3"}),
			repairM975Headers([]string{"10.1.2.3"}, []string{"203.0.113.4"}),
			repairM975Headers(nil, nil),
		}},
		{"malformed real values share peer budget", "127.0.0.1", []http.Header{
			repairM975Headers(nil, []string{"invalid-a"}),
			repairM975Headers(nil, []string{"invalid-b"}),
			repairM975Headers(nil, []string{"198.51.100.8:443"}),
			repairM975Headers(nil, []string{"fe80::8%eth0"}),
			repairM975Headers(nil, []string{"198.51.100.8", "203.0.113.2"}),
		}},
		{"IPv6 spellings share client budget", "2001:db8::8", []http.Header{
			repairM975Headers([]string{"2001:db8::8,10.1.2.3"}, nil),
			repairM975Headers([]string{"2001:0DB8:0:0:0:0:0:8,192.168.1.2"}, nil),
			repairM975Headers(nil, []string{"2001:0db8:0000:0000:0000:0000:0000:0008"}),
		}},
		{"mapped IPv4 shares client budget", "198.51.100.8", []http.Header{
			repairM975Headers([]string{"198.51.100.8"}, nil),
			repairM975Headers([]string{"::ffff:198.51.100.8,::ffff:10.1.2.3"}, nil),
			repairM975Headers(nil, []string{"::ffff:c633:6408"}),
		}},
		{"trusted chains share client budget", "198.51.100.8", []http.Header{
			repairM975Headers([]string{"203.0.113.1,198.51.100.8,10.1.2.3,fd00::1"}, nil),
			repairM975Headers([]string{"203.0.113.2,198.51.100.8", "192.168.1.2,::ffff:10.1.2.3"}, nil),
			repairM975Headers(nil, []string{"198.51.100.8"}),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := repairM975Server(t, repairM975Trusted(), true)
			for i, headers := range tc.headers {
				status := http.StatusTooManyRequests
				if i == 0 {
					status = http.StatusOK
				}
				repairM975Request(t, server, headers, status, tc.identity)
			}
			// A different valid client has an independent budget, even after
			// malformed input or equivalent spellings exhaust the first one.
			repairM975Request(t, server, repairM975Headers([]string{"198.51.100.99,10.1.2.3"}, nil), http.StatusOK, "198.51.100.99")
		})
	}
}

func TestRepairM_975_UntrustedPeer(t *testing.T) {
	server := repairM975Server(t, nil, true)
	repairM975Request(t, server, repairM975Headers([]string{"198.51.100.8,10.1.2.3"}, nil), http.StatusOK, "127.0.0.1")
	repairM975Request(t, server, repairM975Headers(nil, []string{"2001:db8::8"}), http.StatusTooManyRequests, "127.0.0.1")
}

func TestRepairM_975_MappedConnectionPeer(t *testing.T) {
	for _, trusted := range [][]netip.Prefix{
		repairM975Trusted(),
		{
			netip.MustParsePrefix("::ffff:127.0.0.0/104"),
			netip.MustParsePrefix("::ffff:10.0.0.0/104"),
			netip.MustParsePrefix("fd00::/8"),
		},
	} {
		router := NewRouter(ServerOptions{TrustedProxies: trusted})
		router.Get("/identity", func(w http.ResponseWriter, r *http.Request) {
			if r.RemoteAddr != "198.51.100.8" {
				t.Errorf("mapped trusted peer did not resolve canonical client: %q", r.RemoteAddr)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		for _, peer := range []string{"127.0.0.1:41234", "[::ffff:127.0.0.1]:41234", "[fd00::1]:41234"} {
			req := httptest.NewRequest(http.MethodGet, "/identity", nil)
			req.RemoteAddr = peer
			req.Header.Set("X-Forwarded-For", "198.51.100.8, ::ffff:10.1.2.3, fd00:0:0:0:0:0:0:2")
			if got := TrustedClientIP(req, trusted); got != "198.51.100.8" {
				t.Fatalf("outer identity for peer %q = %q", peer, got)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("router status for peer %q = %d", peer, rec.Code)
			}
		}
	}
}
