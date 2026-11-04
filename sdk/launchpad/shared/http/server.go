// Package http is the shared HTTP layer built on chi v5 (D1/L10), porting
// shared/src/http: server bootstrap + CORS, health/readiness, JSON error
// handling, rate limiting, API-key auth, and clamd virus scanning.
//
// Consumers that also import net/http should alias this package, e.g.
// `sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"`.
package http

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// ServerOptions configures NewRouter.
type ServerOptions struct {
	// CORSOrigins is a comma-separated allowlist, or "*"/empty for all (dev).
	// Defaults to the value passed; callers typically pass CORS_ALLOWED_ORIGINS.
	CORSOrigins string
	// Health wires readiness dependency checks.
	Health HealthDeps
	// TrustedProxies lists CIDR ranges of L7 proxies in front of this service.
	// Forwarded headers (X-Forwarded-For / X-Real-IP) are honoured ONLY when the
	// direct peer falls inside one of these ranges; otherwise the raw connection
	// IP is used. Empty (the default) trusts no proxy, which is spoofing-safe.
	TrustedProxies []netip.Prefix
}

// NewRouter builds a chi router with the standard middleware stack (request id,
// real IP, panic recovery, CORS) and the health endpoints registered.
func NewRouter(opts ServerOptions) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(realIP(opts.TrustedProxies))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   resolveOrigins(opts.CORSOrigins),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	RegisterHealth(r, opts.Health)
	return r
}

// resolveOrigins mirrors the TS resolveOrigins: empty or "*" allows all,
// otherwise a trimmed comma-split allowlist.
func resolveOrigins(in string) []string {
	raw := strings.TrimSpace(in)
	if raw == "" || raw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

// realIP sets r.RemoteAddr to the real client IP, replacing chi's deprecated and
// spoofable middleware.RealIP. Forwarded headers are honoured only when the
// direct peer is a trusted proxy (see ServerOptions.TrustedProxies); for any
// other peer the headers are ignored so a client cannot forge its own IP.
func realIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := resolveClientIP(r.RemoteAddr, r.Header, trusted); ip != "" {
				r.RemoteAddr = ip
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveClientIP returns the trustworthy client IP, or "" to leave RemoteAddr
// unchanged. Forwarded addresses must be bare IP literals, without ports or
// zones; accepted addresses are canonicalized and IPv4-mapped IPv6 is unmapped.
// For a trusted peer, every hop in every X-Forwarded-For field is validated
// before selecting the rightmost untrusted hop. Any invalid or empty hop, or an
// all-trusted chain, keeps the connection peer and never falls back to X-Real-IP.
// Only when X-Forwarded-For is absent may a single valid X-Real-IP be used.
// Untrusted peers never have their forwarded headers honoured.
func resolveClientIP(remoteAddr string, h http.Header, trusted []netip.Prefix) string {
	if len(trusted) == 0 || !ipInPrefixes(hostOnly(remoteAddr), trusted) {
		return ""
	}
	if fields := h.Values("X-Forwarded-For"); len(fields) != 0 {
		client := ""
		for _, field := range fields {
			for _, hop := range strings.Split(field, ",") {
				addr, ok := forwardedAddr(hop)
				if !ok {
					return ""
				}
				if cand := addr.String(); !ipInPrefixes(cand, trusted) {
					client = cand
				}
			}
		}
		return client
	}
	if fields := h.Values("X-Real-IP"); len(fields) == 1 {
		if addr, ok := forwardedAddr(fields[0]); ok {
			return addr.String()
		}
	}
	return ""
}

func forwardedAddr(raw string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || addr.Zone() != "" {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// hostOnly strips an optional :port from an address (host:port -> host).
func hostOnly(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// ipInPrefixes reports whether ip parses and falls within any of the prefixes.
func ipInPrefixes(ip string, prefixes []netip.Prefix) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.Zone() != "" {
		return false
	}
	addr = addr.Unmap()
	for _, p := range prefixes {
		// Apply the same IPv4 identity to mapped proxy ranges as to hops.
		if p.Addr().Is4In6() && p.Bits() >= 96 {
			p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
		}
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
