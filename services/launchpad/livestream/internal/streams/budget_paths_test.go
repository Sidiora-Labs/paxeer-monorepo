package streams

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBudgetAdmissionReservesEveryActiveBroadcast(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.BudgetSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	f.store.Budget = &BudgetPolicy{MonthlyUnits: "2", BroadcastReserveUnits: "1", DeliveryPricePerTB: "1", Currency: "USD", ApprovalReference: "creator_approved", PeakDurationSeconds: 60}
	first, intent := f.create("budget-first")
	second, _ := f.create("budget-second")
	if first.ID == second.ID {
		t.Fatal("distinct reservations shared an operation")
	}
	intent.IdempotencyKey = "budget-third"
	_, err := f.store.Create(ctx, intent, CreatorDecision{Wallet: intent.Wallet, Pool: intent.Pool, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}, true)
	if !errors.Is(err, ErrQuota) {
		t.Fatalf("third broadcast reservation = %v", err)
	}
	usage := UsageRecord{ProviderEvent: "usage_1", OperationID: first.ID, ViewerMinutes: "1", TranscodeMinutes: "1", StorageBytes: "1", DeliveryBytes: "1", CostUnits: "1"}
	if err := f.store.RecordUsage(ctx, usage); err != nil {
		t.Fatal(err)
	}
	if err := f.store.EmitBudgetAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	var thresholds []int32
	rows, err := f.database.Query(ctx, "SELECT threshold FROM livestream.stream_budget_alerts ORDER BY threshold")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var threshold int32
		if err := rows.Scan(&threshold); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		thresholds = append(thresholds, threshold)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	if len(thresholds) != 1 || thresholds[0] != 50 {
		t.Fatalf("budget threshold rows = %v", thresholds)
	}
	metrics := &LifecycleMetrics{}
	metrics.ObserveWebhook("applied")
	metrics.ObserveWebhook("duplicate")
	metrics.ObserveWebhook("quarantined_unknown")
	metrics.ObserveWebhook("ignored_stale_or_terminal")
	metrics.DisagreementAlert("provider_state_disagreement", 120*time.Second)
	var output bytes.Buffer
	if err := metrics.WritePrometheus(&output); err != nil {
		t.Fatal(err)
	}
	for _, metric := range []string{
		"livestream_webhook_applied_total 1", "livestream_webhook_duplicate_total 1",
		"livestream_webhook_quarantined_total 1", "livestream_provider_disagreement_total 1",
		"livestream_provider_disagreement_alerts_total 1",
	} {
		if !strings.Contains(output.String(), metric) {
			t.Fatalf("missing %s in %s", metric, output.String())
		}
	}
}
