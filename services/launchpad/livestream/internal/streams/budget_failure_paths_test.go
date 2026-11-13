package streams

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUsageRecordsRejectChangedProviderEventsAndInvalidUnits(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("usage-event-binding")
	record := UsageRecord{ProviderEvent: "usage_event_1", OperationID: o.ID, ViewerMinutes: "1", TranscodeMinutes: "2", StorageBytes: "3", DeliveryBytes: "4", CostUnits: "5"}
	if err := f.store.RecordUsage(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordUsage(ctx, record); err != nil {
		t.Fatalf("idempotent provider event = %v", err)
	}
	changed := record
	changed.CostUnits = "6"
	if err := f.store.RecordUsage(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed provider cost = %v", err)
	}
	other, _ := f.create("usage-event-other")
	changed = record
	changed.OperationID = other.ID
	if err := f.store.RecordUsage(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("reassigned provider event = %v", err)
	}
	changed = record
	changed.ViewerMinutes = "-1"
	if err := f.store.RecordUsage(ctx, changed); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative usage = %v", err)
	}
}

func TestVendorBudgetRequiresCurrentUsageAndAlertAuthority(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.BudgetSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	budget := repairM1001Budget()
	budget.VendorUserID = "account_1"
	f.store.Budget = &budget
	if err := f.store.EmitBudgetAlerts(ctx); err != nil {
		t.Fatalf("empty vendor alert set = %v", err)
	}
	worker := &VendorUsageWorker{Store: f.store, AlertEndpoint: "http://livestream.test/alerts", AlertKey: []byte(strings.Repeat("a", 32))}
	if err := worker.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing current vendor usage = %v", err)
	}
	if err := worker.DeliverAlert(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("plaintext alert endpoint = %v", err)
	}
	worker.AlertEndpoint = "https://livestream.test/alerts"
	worker.AlertKey = []byte("short")
	if err := worker.DeliverAlert(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("short alert key = %v", err)
	}
	if _, err := BudgetThresholds("invalid", "100"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid spent units = %v", err)
	}
	if _, err := BudgetThresholds("1", "0"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero budget = %v", err)
	}
}
