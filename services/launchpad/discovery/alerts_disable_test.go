package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestPostgresDisableRuleIsOwnerScoped(t *testing.T) {
	s, id := realStore(t)
	ctx := context.Background()
	owner, other := id+"owner", id+"other"
	t.Cleanup(func() {
		if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.rules WHERE account_id=$1`, owner); err != nil {
			t.Error(err)
		}
	})
	rule, err := NewAlertRule(AlertRule{ID: id, OwnerAccountID: owner, PoolAddress: pool, Metric: "price", Threshold: "10", Direction: "crosses_above", Channels: []string{"inapp"}, Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRule(ctx, owner, rule, 0); err != nil {
		t.Fatal(err)
	}
	rule.Revision++
	rule.Threshold = "20"
	if err := s.SaveRule(ctx, owner, rule, 1); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 100; i++ {
		next := rule
		next.ID = fmt.Sprintf("%s:%d", id, i)
		next.Revision = 1
		if err := s.SaveRule(ctx, owner, next, 0); err != nil {
			t.Fatalf("create rule %d: %v", i, err)
		}
	}
	assertStored := func(want AlertRule) {
		t.Helper()
		var b []byte
		var got AlertRule
		if err := s.DB.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.rules WHERE id=$1 AND account_id=$2`, id, owner).Scan(&b); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("stored rule = %+v, want %+v", got, want)
		}
	}
	assertActiveCount := func(want int) {
		t.Helper()
		var got int
		if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM narrative_discovery.rules WHERE account_id=$1 AND document->>'Disabled'='false'`, owner).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("active rules = %d, want %d", got, want)
		}
	}
	replacement := rule
	replacement.ID = id + ":replacement"
	replacement.Revision = 1
	if err := s.SaveRule(ctx, owner, replacement, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("create above active-rule limit: got %v, want ErrInvalid", err)
	}
	for _, attempt := range []struct{ actor, ruleID string }{
		{other, id},
		{owner, id + ":missing"},
		{"", id},
	} {
		if err := s.DisableRule(ctx, attempt.actor, attempt.ruleID); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("disable as %q for %q: got %v, want sql.ErrNoRows", attempt.actor, attempt.ruleID, err)
		}
		assertStored(rule)
		assertActiveCount(100)
	}
	if err := s.SaveRule(ctx, owner, replacement, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign disable freed a slot: got %v, want ErrInvalid", err)
	}
	if err := s.DisableRule(ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	rule.Disabled = true
	rule.Revision++
	assertStored(rule)
	assertActiveCount(99)
	if err := s.SaveRule(ctx, owner, replacement, 0); err != nil {
		t.Fatalf("create in freed active-rule slot: %v", err)
	}
	assertActiveCount(100)
	if err := s.DisableRule(ctx, owner, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("disable already-disabled rule: got %v, want sql.ErrNoRows", err)
	}
	assertStored(rule)
	assertActiveCount(100)
}
