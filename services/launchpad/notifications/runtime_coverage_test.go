package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

func TestCredentialRefreshReplacesValuesAndReportsUnavailability(t *testing.T) {
	ctx := context.Background()
	t.Setenv("RELEASE_REFRESH_TOKEN", "first")
	credentials, err := NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, "RELEASE_REFRESH_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := credentials.Get("RELEASE_REFRESH_TOKEN"); err != nil || got != "first" {
		t.Fatalf("initial token: %s %v", got, err)
	}
	t.Setenv("RELEASE_REFRESH_TOKEN", "second")
	if err := credentials.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := credentials.Get("RELEASE_REFRESH_TOKEN"); err != nil || got != "second" {
		t.Fatalf("refreshed token: %s %v", got, err)
	}
	t.Setenv("RELEASE_REFRESH_TOKEN", "")
	if err := credentials.Refresh(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty secret accepted: %v", err)
	}
	if _, err := credentials.Get("RELEASE_REFRESH_TOKEN"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("stale token served: %v", err)
	}
	if err := credentials.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unready credentials: %v", err)
	}
	t.Setenv("RELEASE_REFRESH_TOKEN", "third")
	if err := credentials.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := credentials.Get("RELEASE_REFRESH_TOKEN"); err != nil || got != "third" {
		t.Fatalf("recovered token: %s %v", got, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := credentials.Run(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled refresh loop: %v", err)
	}
}

func TestPushRuntimeRunsWorkerAndCredentialLifecycle(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []RuntimeOptions{
		{Owner: "worker", DeepLinkOrigin: "http://example.test"},
		{Owner: "worker", DeepLinkOrigin: "https://example.test?query=1"},
		{Owner: "worker", DeepLinkOrigin: "https://example.test", APNSKey: "unused"},
		{Owner: "worker", DeepLinkOrigin: "https://example.test", FCMServiceAccount: "unused"},
		{Owner: "worker", DeepLinkOrigin: "https://example.test", VAPIDKey: "unused"},
	} {
		invalid.Secrets = options
		if _, err := NewPushRuntime(ctx, db, authority, invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid push options: %+v %v", invalid, err)
		}
	}
	runtime, err := NewPushRuntime(ctx, db, authority, RuntimeOptions{Secrets: options, Owner: "worker", DeepLinkOrigin: "https://example.test/"})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Worker.DeepLinkOrigin != "https://example.test" || len(runtime.Worker.Adapters) != 0 {
		t.Fatalf("push runtime: %+v", runtime.Worker)
	}
	run, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if err := runtime.Run(run); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("push runtime cancellation: %v", err)
	}
}

func TestRuntimeRejectsMissingIdentityAndCursorAuthority(t *testing.T) {
	ctx := context.Background()
	if _, err := NewCredentials(ctx, secrets.Options{Provider: "unknown"}, "RELEASE_CURSOR"); err == nil {
		t.Fatal("unknown secret provider accepted")
	}
	if _, err := NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, "MISSING_RELEASE_TOKEN"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing secret admitted: %v", err)
	}
	if _, err := NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty credential reference: %v", err)
	}
	t.Setenv("RELEASE_CURSOR", "short")
	credentials, err := NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, "RELEASE_CURSOR")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := credentials.Get("missing"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown credential: %v", err)
	}
	credentials.mu.Lock()
	credentials.refreshed = time.Now().Add(-time.Minute)
	credentials.mu.Unlock()
	if err := credentials.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expired credential ready: %v", err)
	}
	runtime := &HTTPRuntime{Credentials: credentials, Config: RuntimeConfig{CursorKeyRef: "RELEASE_CURSOR"}}
	response := httptest.NewRecorder()
	runtime.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("feed without source authority: %d", response.Code)
	}
	internal := httptest.NewRecorder()
	runtime.InternalHandler().ServeHTTP(internal, httptest.NewRequest(http.MethodPost, "/internal/v2/notifications/chat", nil))
	if internal.Code != http.StatusBadRequest {
		t.Fatalf("empty internal chat request: %d", internal.Code)
	}
	if err := runtime.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("runtime without database ready: %v", err)
	}
	if err := (*HTTPRuntime)(nil).Run(ctx); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil runtime started: %v", err)
	}
}
