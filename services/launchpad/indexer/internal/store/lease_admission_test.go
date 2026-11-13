package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestLeaseAdmissionPreservesDurationBoundsAndFences(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	for _, tc := range []struct {
		chain         int32
		stream, owner string
		ttl           time.Duration
	}{
		{0, "stream", "owner", time.Second}, {125, "", "owner", time.Second}, {125, "stream", "", time.Second}, {125, "stream", "owner", 0}, {125, "stream", "owner", 31 * time.Second},
	} {
		if _, err := st.AcquireLease(ctx, tc.chain, tc.stream, tc.owner, tc.ttl); err == nil {
			t.Fatal("invalid lease admitted")
		}
	}
	for _, ttl := range []time.Duration{0, 31 * time.Second} {
		if err := st.RenewLease(ctx, 125, "admission", "owner", 1, ttl); err == nil {
			t.Fatal("invalid renewal admitted")
		}
	}
	fence, err := st.AcquireLease(ctx, 125, "admission", "owner", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.ReleaseLease(ctx, 125, "admission", "owner", fence+1); !errors.Is(err, ErrFenced) {
		t.Fatalf("foreign fence released: %v", err)
	}
	pool.Close()
	for _, call := range []func() error{
		func() error { return st.ReleaseLease(ctx, 125, "admission", "owner", fence) },
		func() error { return st.RenewLease(ctx, 125, "admission", "owner", fence, time.Second) },
	} {
		if err = call(); err == nil {
			t.Fatal("unavailable lease storage admitted")
		}
	}
}

func TestOutboxReconciliationRejectsDuplicateAndUndeclaredIntents(t *testing.T) {
	first := OutboxIntent{TransactionHash: "transaction", Kind: "market", Payload: json.RawMessage(`{"amount":"1"}`)}
	other := first
	other.LogIndex = 1
	inventory := first
	inventory.Kind = accounting.InventorySubject
	for _, tc := range []struct {
		name             string
		expected, actual []OutboxIntent
		count            int
		want             bool
	}{
		{"duplicate expected", []OutboxIntent{first, first}, []OutboxIntent{first, other}, 2, false},
		{"undeclared actual", []OutboxIntent{first}, []OutboxIntent{other}, 1, false},
		{"wrong count", []OutboxIntent{first}, []OutboxIntent{first}, 0, false},
		{"deferred count", []OutboxIntent{first}, []OutboxIntent{first, inventory}, 3, false},
		{"undeclared market", []OutboxIntent{inventory}, []OutboxIntent{first}, 1, false},
		{"deferred inventory", []OutboxIntent{first}, []OutboxIntent{first, inventory}, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reconciledWithDeferredInventory(tc.expected, tc.actual, tc.count); got != tc.want {
				t.Fatalf("reconciled %v want %v", got, tc.want)
			}
		})
	}
	if sameIntents([]OutboxIntent{first, first}, []OutboxIntent{first, other}) {
		t.Fatal("duplicate expected intent admitted")
	}
	if sameIntents([]OutboxIntent{first}, []OutboxIntent{other}) {
		t.Fatal("undeclared actual intent admitted")
	}
}
