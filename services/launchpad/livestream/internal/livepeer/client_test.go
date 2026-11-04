package livepeer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderFailureRedactsSecretsAndBoundsBody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"error", 500, "SECRET-stream-key", "livepeer: provider returned status 500"},
		{"oversize", 200, strings.Repeat("x", (1<<20)+1), "livepeer: response too large"},
		{"redirect", 302, "", "livepeer: provider returned status 302"},
		{"malformed", 200, "invalid-json SECRET-stream-key", "livepeer: response JSON invalid"},
		{"empty_object", 200, "{}", "livepeer: invalid provider stream response"},
		{"null", 200, "null", "livepeer: invalid provider stream response"},
		{"missing_key", 200, `{"id":"id","playbackId":"playback"}`, "livepeer: invalid provider stream response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Error("missing configured authorization")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c := New("fixture-key", server.URL, nil)
			stream, e := c.CreateStream(context.Background(), "fixture-correlation")
			if stream != nil || e == nil || e.Error() != tc.wantErr {
				t.Fatalf("CreateStream = (%v, %v), want (nil, %q)", stream, e, tc.wantErr)
			}
			if strings.Contains(e.Error(), "SECRET") || strings.Contains(e.Error(), "fixture-key") {
				t.Fatalf("provider error leaked a secret: %v", e)
			}
			if tc.status < 200 || tc.status >= 300 {
				var providerErr *ProviderError
				if !errors.As(e, &providerErr) || providerErr.Status != tc.status {
					t.Fatalf("provider status classification = %v, want %d", e, tc.status)
				}
			}
		})
	}
}
func TestProviderRejectsInvalidResponseMediaType(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mediaType string
	}{
		{"missing", ""},
		{"text", "text/plain"},
		{"html", "text/html"},
		{"malformed", "application/json; charset"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Suppress net/http's automatic Content-Type for the missing-header case.
				w.Header()["Content-Type"] = nil
				if tc.mediaType != "" {
					w.Header().Set("Content-Type", tc.mediaType)
				}
				_, _ = w.Write([]byte(`{"id":"id","playbackId":"playback","streamKey":"SECRET-stream-key"}`))
			}))
			defer server.Close()
			c := New("fixture-key", server.URL, nil)
			stream, err := c.CreateStream(context.Background(), "fixture-correlation")
			if stream != nil || err == nil || err.Error() != "livepeer: response content type invalid" {
				t.Fatalf("CreateStream = (%v, %v), want sanitized media-type refusal", stream, err)
			}
		})
	}
}
func TestProviderRedirectDoesNotForwardAuthorization(t *testing.T) {
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer origin.Close()
	c := New("fixture-key", origin.URL, nil)
	if stream, e := c.GetStream(context.Background(), "one"); stream != nil || e == nil || e.Error() != "livepeer: provider returned status 302" {
		t.Fatalf("redirect result = (%v, %v), want sanitized status 302", stream, e)
	}
	if requests.Load() != 0 {
		t.Fatal("followed redirect")
	}
}
func TestProviderUnavailableAndCancellation(t *testing.T) {
	c := New("", "https://unused.invalid", nil)
	if _, e := c.CreateStream(context.Background(), "one"); e == nil {
		t.Fatal("missing key accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	c = New("fixture", server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if stream, e := c.GetStream(ctx, "one"); stream != nil || e == nil || e.Error() != "livepeer: request unavailable" {
		t.Fatalf("cancellation result = (%v, %v), want sanitized transport refusal", stream, e)
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("request ended without the caller deadline: %v", ctx.Err())
	}
}
func TestCDNPlaybackIdentityCannotEscape(t *testing.T) {
	c := New("fixture", "https://livepeer.studio/api", nil)
	for _, id := range []string{"", "../private", "x?redirect=https://evil.invalid", "x/y"} {
		if c.PlaybackURL(id) != "" {
			t.Fatal(id)
		}
	}
	if c.PlaybackURL("valid_1") != "https://livepeercdn.studio/hls/valid_1/index.m3u8" {
		t.Fatal("bad canonical CDN")
	}
}
