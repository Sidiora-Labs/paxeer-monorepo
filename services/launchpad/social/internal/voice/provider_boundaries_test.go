package voice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLocalProviderRejectsUnboundGrantsAndMissingSessions(t *testing.T) {
	provider := localLiveKit(t)
	ctx := context.Background()
	if _, err := provider.Start(ctx, ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty room key = %v", err)
	}
	if _, err := provider.Start(ctx, strings.Repeat("r", 161)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("oversized room key = %v", err)
	}
	if _, err := provider.Ticket(ctx, "", host, false, time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ticket without room = %v", err)
	}
	if _, err := provider.Ticket(ctx, "kindle-room", "", false, time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ticket without identity = %v", err)
	}
	if _, err := provider.Ticket(ctx, "kindle-room", host, false, 2*time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ticket beyond admission lifetime = %v", err)
	}
	if _, err := provider.token(host, nil, 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("zero-lifetime token = %v", err)
	}
	if _, err := provider.Record(ctx, "kindle-missing", "recording-key"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("recording without restricted output = %v", err)
	}
	if _, found, err := provider.Find(ctx, "missing-room-key"); err != nil || found {
		t.Fatalf("missing provider room = %t: %v", found, err)
	}
	if err := provider.Revoke(ctx, "kindle-missing", host); err != nil {
		t.Fatalf("idempotent absent grant revocation = %v", err)
	}
	if err := provider.End(ctx, "kindle-missing"); err != nil {
		t.Fatalf("idempotent absent room end = %v", err)
	}
	if err := provider.Grant(ctx, "kindle-missing", host, true); err == nil {
		t.Fatal("missing room accepted a publisher grant")
	}
}
