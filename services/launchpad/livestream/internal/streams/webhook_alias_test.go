package streams

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestPostgresProviderWebhookAliasVerifiesSignature(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	request, decision := intent("webhook_alias")
	o, err := s.Create(ctx, request, decision, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginProvision(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	o, err = s.BindProvider(ctx, o.ID, "streamid")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	secret := strings.Repeat("k", 32)
	var lastOutcome string
	h := New(Deps{Lifecycle: s, StudioWebhook: true, WebhookMilliseconds: true, WebhookSecret: secret, Clock: func() time.Time { return now }, ObserveOutcome: func(outcome string) { lastOutcome = outcome }})
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	repairM1001AssertAliases(t, router, secret, now, &lastOutcome)
	var count int
	if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_inbox").Scan(&count); err != nil || count != 1 {
		t.Fatalf("signed event must be persisted once: count=%d err=%v", count, err)
	}
	current, err := s.Get(ctx, o.ID)
	if err != nil || current.State != "LIVE" || current.Revision != o.Revision+1 || current.ProviderTime == nil || !current.ProviderTime.Equal(now.Add(-time.Minute)) {
		t.Fatalf("signed event must be applied once: %+v err=%v", current, err)
	}
}
