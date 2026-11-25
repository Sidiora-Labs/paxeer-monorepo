package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

func runtimeRetriesFixture(t *testing.T) (*HTTPRuntime, *notificationDeliveryPeers) {
	t.Helper()
	ctx := context.Background()
	db := notificationIdentityOpen(t, ctx, notificationIdentityDatabase(t, ctx))
	notificationDeliverySchema(t, db)
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA indexer; CREATE TABLE indexer.indexer_canonical_creations_v1(pool text,token text,creator text,height bigint,block_hash text,transaction_hash text,log_index bigint,block_timestamp bigint)`); err != nil {
		t.Fatal(err)
	}
	peer := notificationDeliveryAuthorities(t)
	config := RuntimeConfig{
		Principal: peer.principal, Secrets: notificationDeliveryCredentials(t), Client: peer.client,
		D11Base: peer.server.URL, D12Base: peer.server.URL,
		D11TokenRef: "REPAIR958_D11", D12TokenRef: "REPAIR958_D12", CursorKeyRef: "REPAIR958_CURSOR", InternalTokenRef: "REPAIR958_INTERNAL",
		DeletePeerDNS: "delete.example", ChatPeerDNS: "chat.example",
		Pools: CanonicalPools{DB: db}, Intervals: discovery.SQLIntervalAuthority{DB: db},
	}
	runtime, err := NewHTTPRuntime(ctx, db, func(r *http.Request) (string, error) { p, err := peer.principal(r); return p.AccountID, err }, config)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, peer
}

func runtimeRetriesWait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("runtime did not reach the expected pass state")
		}
		time.Sleep(time.Millisecond)
	}
}

func runtimeRetriesStart(t *testing.T, runtime *HTTPRuntime) (context.Context, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("runtime cancellation: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("runtime jobs did not stop")
		}
		if runtime.Ready(context.Background()) == nil {
			t.Error("stopped runtime ready")
		}
	})
	return ctx, done
}

func TestRuntimeRetriesHTTPRecoversReadinessAndFeed(t *testing.T) {
	for _, name := range []string{"discovery_events", "maintenance", "credentials", "push_delivery"} {
		t.Run(name, func(t *testing.T) {
			runtime, peer := runtimeRetriesFixture(t)
			var restore func()
			if name == "credentials" {
				t.Setenv("REPAIR958_D11", "")
				restore = func() { t.Setenv("REPAIR958_D11", "repair958-owner-token") }
			} else {
				table := "narrative_discovery.evaluation_cursor"
				if name == "maintenance" {
					table = "narrative_discovery.snapshots"
				}
				if name == "push_delivery" {
					table = "discovery_notifications.outbox"
					push, err := NewPushRuntime(context.Background(), runtime.DB, runtime.Accounts, RuntimeOptions{Secrets: runtime.Config.Secrets, Owner: "push", DeepLinkOrigin: "https://example.test"})
					if err != nil {
						t.Fatal(err)
					}
					runtime.Push = push
				}
				parts := strings.Split(table, ".")
				if _, err := runtime.DB.Exec("ALTER TABLE " + table + " RENAME TO retry_unavailable"); err != nil {
					t.Fatal(err)
				}
				restore = func() {
					if _, err := runtime.DB.Exec("ALTER TABLE " + parts[0] + ".retry_unavailable RENAME TO " + parts[1]); err != nil {
						t.Fatal(err)
					}
				}
			}
			ctx, done := runtimeRetriesStart(t, runtime)
			runtimeRetriesWait(t, func() bool {
				runtime.mu.RLock()
				defer runtime.mu.RUnlock()
				return runtime.passes[name].err != nil
			})
			if !errors.Is(runtime.Ready(ctx), ErrUnavailable) {
				t.Fatal("failed pass remained ready")
			}
			select {
			case err := <-done:
				t.Fatalf("runtime stopped after a recoverable error: %v", err)
			default:
			}
			restore()
			runtimeRetriesWait(t, func() bool { return runtime.Ready(ctx) == nil })
			runtime.mu.RLock()
			pass := runtime.passes[name]
			runtime.mu.RUnlock()
			if pass.at.IsZero() || pass.err != nil {
				t.Fatalf("last pass did not recover: %+v", pass)
			}
			server := httptest.NewServer(runtime.Handler())
			defer server.Close()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v2/discovery/feed", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer repair958-session")
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK || peer.calls.Load() == 0 {
				t.Fatalf("recovered authenticated feed: %d", response.StatusCode)
			}
		})
	}
}

func TestRuntimeRetriesCredentialsKeepLastGoodAndRefreshAgain(t *testing.T) {
	t.Setenv("RUNTIME_REFRESH_VALUE", "current")
	credentials, err := NewCredentials(context.Background(), secrets.Options{Provider: secrets.ProviderEnv}, "RUNTIME_REFRESH_VALUE")
	if err != nil {
		t.Fatal(err)
	}
	ticks, advance := make(chan time.Duration), make(chan struct{})
	var waiting atomic.Int32
	credentials.wait = func(ctx context.Context, delay time.Duration) error {
		waiting.Add(1)
		defer waiting.Add(-1)
		select {
		case ticks <- delay:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-advance:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- credentials.Run(ctx) }()
	defer cancel()
	nextTick := func() {
		t.Helper()
		select {
		case delay := <-ticks:
			if delay != 30*time.Second {
				t.Fatalf("refresh interval: %v", delay)
			}
		case <-time.After(time.Second):
			t.Fatal("credential loop did not reach the next tick")
		}
	}
	nextTick()
	t.Setenv("RUNTIME_REFRESH_VALUE", "")
	advance <- struct{}{}
	nextTick()
	if got, err := credentials.Get("RUNTIME_REFRESH_VALUE"); err != nil || got != "current" || credentials.Ready(ctx) != nil {
		t.Fatalf("last good credentials lost: %q %v", got, err)
	}
	credentials.mu.Lock()
	credentials.refreshed = time.Now().Add(-61 * time.Second)
	credentials.mu.Unlock()
	if credentials.Ready(ctx) == nil {
		t.Fatal("stale credentials ready")
	}
	if _, err := credentials.Get("RUNTIME_REFRESH_VALUE"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("stale credential served: %v", err)
	}
	t.Setenv("RUNTIME_REFRESH_VALUE", "rotated")
	advance <- struct{}{}
	nextTick()
	if got, err := credentials.Get("RUNTIME_REFRESH_VALUE"); err != nil || got != "rotated" || credentials.Ready(ctx) != nil {
		t.Fatalf("credentials did not recover: %q %v", got, err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || waiting.Load() != 0 {
			t.Fatalf("credential shutdown: %v, waiting=%d", err, waiting.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("credential loop did not stop")
	}
}

func TestRuntimeRetriesBackoffIsBoundedAndResets(t *testing.T) {
	ctx := context.Background()
	db := notificationIdentityOpen(t, ctx, notificationIdentityDatabase(t, ctx))
	store := Store{DB: db}
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	// A missing relation makes the production maintenance operation fail.
	if _, err := db.Exec(`ALTER TABLE discovery_notifications.inbox RENAME TO retry_inbox`); err != nil {
		t.Fatal(err)
	}
	clock := make(chan time.Duration)
	advance := make(chan struct{})
	var waiting atomic.Int32
	runtime := runtimeJobs{wait: func(ctx context.Context, delay time.Duration) error {
		waiting.Add(1)
		defer waiting.Add(-1)
		select {
		case clock <- delay:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-advance:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runtime.run(run, []runtimeJob{{name: "retention", interval: 30 * time.Second, timeout: time.Second, pass: func(ctx context.Context) error {
			_, err := store.Retain(ctx, time.Now(), 90, 1000)
			return err
		}}})
	}()
	started := time.Now()
	for index, want := range []time.Duration{1, 2, 4, 8, 16, 30, 30, 30, 1} {
		select {
		case got := <-clock:
			if got != want*time.Second {
				t.Fatalf("pass %d delay = %v, want %vs", index, got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("runtime did not reach its backoff")
		}
		if index < 7 && runtime.ready() == nil {
			t.Fatal("failing job ready")
		}
		if index == 6 {
			if _, err := db.Exec(`ALTER TABLE discovery_notifications.retry_inbox RENAME TO inbox`); err != nil {
				t.Fatal(err)
			}
		}
		if index == 7 {
			if err := runtime.ready(); err != nil {
				t.Fatal("successful pass not ready:", err)
			}
			if _, err := db.Exec(`ALTER TABLE discovery_notifications.inbox RENAME TO retry_inbox`); err != nil {
				t.Fatal(err)
			}
		}
		if index < 8 {
			advance <- struct{}{}
		}
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("injected clock exceeded two seconds")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || waiting.Load() != 0 || runtime.ready() == nil {
			t.Fatalf("runtime shutdown: %v, waiting=%d", err, waiting.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("runtime did not stop during backoff")
	}
}

func TestRuntimeRetriesPushAndFinalErrors(t *testing.T) {
	runtime, _ := runtimeRetriesFixture(t)
	push, err := NewPushRuntime(context.Background(), runtime.DB, runtime.Accounts, RuntimeOptions{Secrets: runtime.Config.Secrets, Owner: "push", DeepLinkOrigin: "https://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DB.Exec(`ALTER TABLE discovery_notifications.outbox RENAME TO retry_outbox`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- push.Run(ctx) }()
	defer cancel()
	runtimeRetriesWait(t, func() bool {
		push.mu.RLock()
		defer push.mu.RUnlock()
		return push.passes["push_delivery"].err != nil
	})
	if push.ready() == nil {
		t.Fatal("failed delivery pass ready")
	}
	if _, err := runtime.DB.Exec(`ALTER TABLE discovery_notifications.retry_outbox RENAME TO outbox`); err != nil {
		t.Fatal(err)
	}
	runtimeRetriesWait(t, func() bool { return push.ready() == nil })
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("push shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("push jobs did not stop")
	}
	push.mu.RLock()
	passes := make(map[string]runtimePass, len(push.passes))
	for name, pass := range push.passes {
		passes[name] = pass
	}
	push.mu.RUnlock()
	push.Worker.Owner = ""
	if err := push.Run(context.Background()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid worker retried: %v", err)
	}
	if push.running {
		t.Fatal("final error left runtime jobs running")
	}
	if reflect.DeepEqual(passes, push.passes) {
		t.Fatal("runtime restart retained old pass results")
	}
	runtime.Events.Publisher = nil
	if err := runtime.Run(context.Background()); !errors.Is(err, discovery.ErrInvalid) {
		t.Fatalf("invalid discovery publisher retried: %v", err)
	}
	if runtime.running {
		t.Fatal("final discovery error left runtime jobs running")
	}
	if err := (*PushRuntime)(nil).Run(context.Background()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil push runtime: %v", err)
	}
}
