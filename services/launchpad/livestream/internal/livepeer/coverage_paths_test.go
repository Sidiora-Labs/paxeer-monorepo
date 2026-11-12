package livepeer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProviderStreamLifecycleAndPlaybackPolicy(t *testing.T) {
	const operation = "11111111-1111-4111-8111-111111111111"
	const webhook = "playback_hook"
	var callsMu sync.Mutex
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callsMu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		callsMu.Unlock()
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("provider authorization missing")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /stream":
			var body struct {
				Name           string          `json:"name"`
				Profiles       []streamProfile `json:"profiles"`
				PlaybackPolicy PlaybackPolicy  `json:"playbackPolicy"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name != CorrelationName(operation) || len(body.Profiles) != 3 || body.PlaybackPolicy.Context["operationId"] != operation || body.PlaybackPolicy.WebhookID != webhook {
				t.Errorf("provision request = %+v, %v", body, err)
			}
			_, _ = w.Write([]byte(`{"id":"provider_1","playbackId":"playback_1","streamKey":"key_1","name":"KindleLaunch:` + operation + `"}`))
		case "GET /stream":
			if r.URL.RawQuery != "streamsonly=true" {
				t.Error("stream inventory query missing")
			}
			_, _ = w.Write([]byte(`[{"id":"provider_1","playbackId":"playback_1","streamKey":"key_1","name":"KindleLaunch:` + operation + `"}]`))
		case "GET /stream/provider_1":
			_, _ = w.Write([]byte(`{"id":"provider_1","playbackId":"playback_1","streamKey":"key_1","playbackPolicy":{"type":"webhook","webhookId":"` + webhook + `","webhookContext":{"operationId":"` + operation + `"},"refreshInterval":1}}`))
		case "PATCH /stream/provider_1":
			var body struct {
				PlaybackPolicy PlaybackPolicy `json:"playbackPolicy"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlaybackPolicy.Type != webhookPolicy || body.PlaybackPolicy.WebhookID != webhook || body.PlaybackPolicy.Context["operationId"] != operation {
				t.Errorf("playback policy = %+v, %v", body, err)
			}
			w.WriteHeader(http.StatusNoContent)
		case "DELETE /stream/provider_1", "DELETE /stream/provider_1/terminate":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected provider call: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := New("fixture-key", server.URL, nil, WithWebhookPlayback(webhook))
	ctx := context.Background()
	created, err := c.CreateCorrelated(ctx, operation)
	if err != nil || created.ID != "provider_1" || created.StreamKey != "key_1" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	if err := c.Available(ctx); err != nil {
		t.Fatal(err)
	}
	found, err := c.FindCorrelated(ctx, operation)
	if err != nil || found.ID != created.ID {
		t.Fatalf("found = %+v, %v", found, err)
	}
	if err := c.EnsurePlaybackPolicy(ctx, created.ID, operation); err != nil {
		t.Fatal(err)
	}
	if err := c.TerminateStream(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteStream(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if c.RtmpURL(created.StreamKey) != "rtmp://rtmp.livepeer.studio/live/key_1" || c.RtmpURL("../key") != "" {
		t.Fatal("ingest URL was not bound to the stream key")
	}
	callsMu.Lock()
	count := len(calls)
	callsMu.Unlock()
	if count != 7 {
		t.Fatalf("provider calls = %d", count)
	}
	if _, err := c.FindCorrelated(ctx, "invalid"); !errors.Is(err, ErrCorrelationMissing) {
		t.Fatal(err)
	}
	if err := c.DeleteStream(ctx, "../private"); err == nil {
		t.Fatal("unsafe stream id accepted")
	}
}

func TestProviderPolicyRequiresTheRegisteredOperationAndCallback(t *testing.T) {
	const operation = "11111111-1111-4111-8111-111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "PATCH /stream/provider_1":
			w.WriteHeader(http.StatusNoContent)
		case "GET /stream/provider_1":
			_, _ = w.Write([]byte(`{"id":"provider_1","playbackId":"playback_1","playbackPolicy":{"type":"webhook","webhookId":"other_hook","webhookContext":{"operationId":"` + operation + `"},"refreshInterval":1}}`))
		case "GET /webhook/hook_1":
			_, _ = w.Write([]byte(`{"id":"hook_1","url":"https://livepeer.studio/access","sharedSecret":"fixture-secret-0123456789-0123456789","events":["stream.started"]}`))
		case "POST /webhook":
			_, _ = w.Write([]byte(`{"id":"../unsafe"}`))
		default:
			t.Errorf("provider authority path = %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := New("fixture-key", server.URL, nil, WithWebhookPlayback("hook_1"))
	ctx := context.Background()
	if err := c.EnsurePlaybackPolicy(ctx, "provider_1", operation); err == nil || !strings.Contains(err.Error(), "unconfirmed") {
		t.Fatalf("changed provider playback policy = %v", err)
	}
	if err := c.EnsurePlaybackPolicy(ctx, "../unsafe", operation); err == nil {
		t.Fatal("unsafe stream identity accepted for playback policy")
	}
	if err := c.ValidatePlaybackWebhook(ctx, "hook_1", "https://livepeer.studio/access", "fixture-secret-0123456789-0123456789"); err == nil || !strings.Contains(err.Error(), "event missing") {
		t.Fatalf("callback without access event = %v", err)
	}
	if _, err := c.RegisterPlaybackWebhook(ctx, "playback", "https://livepeer.studio/access", "fixture-secret-0123456789-0123456789", "project_1"); err == nil || !strings.Contains(err.Error(), "response invalid") {
		t.Fatalf("unsafe registered callback = %v", err)
	}
}

func TestProviderClientRejectsUnboundRequestsAndResponses(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "PATCH /stream/provider_1":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "GET /webhook/hook_1":
			w.WriteHeader(http.StatusNotFound)
		case "GET /data/views/query":
			_, _ = w.Write([]byte(`null`))
		default:
			t.Errorf("unbound provider request = %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := New("fixture-key", server.URL, nil, WithHTTPClient(nil), WithWebhookPlayback("hook_1"))
	if c.http == nil {
		t.Fatal("provider HTTP client missing")
	}
	if _, err := c.CreateStream(ctx, "unbound-operation"); err == nil || !strings.Contains(err.Error(), "playback operation invalid") {
		t.Fatalf("unbound playback operation = %v", err)
	}
	if err := c.EnsurePlaybackPolicy(ctx, "provider_1", "11111111-1111-4111-8111-111111111111"); err == nil {
		t.Fatal("provider policy outage accepted")
	}
	if err := c.ValidatePlaybackWebhook(ctx, "hook_1", "https://livepeer.studio/access", "fixture-secret-0123456789-0123456789"); err == nil {
		t.Fatal("missing playback registration accepted")
	}
	if err := c.ValidatePlaybackWebhook(ctx, "../hook", "https://livepeer.studio/access", "fixture-secret-0123456789-0123456789"); err == nil {
		t.Fatal("unsafe callback identity accepted")
	}
	if err := c.ValidatePlaybackWebhook(ctx, "hook_1", "http://livepeer.studio/access", "fixture-secret-0123456789-0123456789"); err == nil {
		t.Fatal("plaintext callback accepted")
	}
	if _, err := c.RegisterPlaybackWebhook(ctx, "../unsafe", "https://livepeer.studio/access", "fixture-secret-0123456789-0123456789", "project_1"); err == nil {
		t.Fatal("unsafe callback registration accepted")
	}
	if _, err := c.RegisterPlaybackWebhook(ctx, "playback", "http://livepeer.studio/access", "fixture-secret-0123456789-0123456789", "project_1"); err == nil {
		t.Fatal("plaintext callback registration accepted")
	}
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.GetViewerMinutes(ctx, from, from.Add(time.Hour)); err == nil {
		t.Fatal("empty viewership response accepted")
	}
}

func TestProviderUsageViewershipAndWebhookRegistration(t *testing.T) {
	const secret = "fixture-secret-0123456789-0123456789"
	const endpoint = "https://livepeer.studio/playback-access"
	var callsMu sync.Mutex
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callsMu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		callsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /data/usage/query":
			if r.URL.Query().Get("from") == "" || r.URL.Query().Get("to") == "" {
				t.Error("usage interval missing")
			}
			_, _ = w.Write([]byte(`{"UserID":"account_1","DeliveryUsageMins":1.5,"TotalUsageMins":2,"StorageUsageMins":0.25}`))
		case "GET /user/me":
			_, _ = w.Write([]byte(`{"id":"account_1"}`))
		case "GET /data/views/query":
			_, _ = w.Write([]byte(`[{"playtimeMins":1.25},{"playtimeMins":2.5}]`))
		case "POST /webhook":
			var body PlaybackWebhook
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name != "playback" || body.URL != endpoint || body.SharedSecret != secret || body.ProjectID != "project_1" || len(body.Events) != 1 || body.Events[0] != "playback.accessControl" {
				t.Errorf("webhook registration = %+v, %v", body, err)
			}
			_, _ = w.Write([]byte(`{"id":"hook_1"}`))
		case "GET /webhook/hook_1":
			_, _ = w.Write([]byte(`{"id":"hook_1","url":"` + endpoint + `","sharedSecret":"` + secret + `","events":["playback.accessControl"]}`))
		default:
			t.Errorf("unexpected provider call: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := New("fixture-key", server.URL, nil)
	ctx := context.Background()
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	usage, err := c.GetUsage(ctx, from, to)
	if err != nil || usage.UserID != "account_1" || usage.DeliveryMinutes.String() != "1.5" || usage.StorageMinutes.String() != "0.25" {
		t.Fatalf("usage = %+v, %v", usage, err)
	}
	minutes, err := c.GetViewerMinutes(ctx, from, to)
	if err != nil || minutes != "3.750000000000000000" {
		t.Fatalf("viewership = %q, %v", minutes, err)
	}
	registered, err := c.RegisterPlaybackWebhook(ctx, "playback", endpoint, secret, "project_1")
	if err != nil || registered.ID != "hook_1" {
		t.Fatalf("registered = %+v, %v", registered, err)
	}
	callsMu.Lock()
	count := len(calls)
	callsMu.Unlock()
	if count != 5 {
		t.Fatalf("provider calls = %d", count)
	}
	if _, err := c.GetUsage(ctx, to, from); err == nil {
		t.Fatal("reversed usage interval accepted")
	}
	if _, err := c.GetViewerMinutes(ctx, to, from); err == nil {
		t.Fatal("reversed viewership interval accepted")
	}
	if _, err := c.RegisterPlaybackWebhook(ctx, "playback", "http://livepeer.studio/access", secret, "project_1"); err == nil {
		t.Fatal("plaintext playback callback accepted")
	}
	if err := c.ValidatePlaybackWebhook(ctx, "hook_1", endpoint, strings.Repeat("x", 32)); err == nil {
		t.Fatal("mismatched callback secret accepted")
	}
}

func TestProviderRejectsChangedAccountStreamAndPlaybackAuthority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/stream":
			_, _ = w.Write([]byte(`null`))
		case "/stream/provider_1":
			_, _ = w.Write([]byte(`{"id":"other_provider","playbackId":"playback_1","playbackPolicy":{"type":"webhook","webhookId":"wrong_hook"}}`))
		case "/data/usage/query":
			_, _ = w.Write([]byte(`{"UserID":"other_account","DeliveryUsageMins":1,"TotalUsageMins":1,"StorageUsageMins":1}`))
		case "/user/me":
			_, _ = w.Write([]byte(`{"id":"account_1"}`))
		case "/data/views/query":
			_, _ = w.Write([]byte(`[{"playtimeMins":-1}]`))
		case "/webhook/hook_1":
			_, _ = w.Write([]byte(`{"id":"hook_1","url":"https://livepeer.studio/access","sharedSecret":"fixture-secret-0123456789-0123456789","events":[]}`))
		default:
			t.Errorf("unexpected provider request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := New("fixture-key", server.URL, nil, WithHTTPClient(server.Client()))
	ctx := context.Background()
	if err := c.Available(ctx); !errors.Is(err, ErrAmbiguousCorrelation) {
		t.Fatalf("empty stream inventory = %v", err)
	}
	if _, err := c.GetStream(ctx, "provider_1"); err == nil {
		t.Fatal("substituted provider identity accepted")
	}
	if _, err := c.GetStream(ctx, "../private"); err == nil {
		t.Fatal("unsafe provider identity accepted")
	}
	if err := c.TerminateStream(ctx, "../private"); err == nil {
		t.Fatal("unsafe termination identity accepted")
	}
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.GetUsage(ctx, from, from.Add(time.Hour)); err == nil {
		t.Fatal("substituted usage account accepted")
	}
	if _, err := c.GetViewerMinutes(ctx, from, from.Add(time.Hour)); err == nil {
		t.Fatal("negative viewership accepted")
	}
	if err := c.ValidatePlaybackWebhook(ctx, "hook_1", "https://livepeer.studio/access", "fixture-secret-0123456789-0123456789"); err == nil {
		t.Fatal("playback webhook without access event accepted")
	}
}
