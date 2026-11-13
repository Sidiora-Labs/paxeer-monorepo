package delivery

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBudgetedDeliveryRejectsUnavailableWorkCapacity(t *testing.T) {
	ctx := context.Background()
	for name, run := range map[string]func(context.Context, *WorkBudget) error{
		"relay":   (Relay{}).RunOnceWithBudget,
		"webhook": (&WebhookBridge{}).RunOnceWithBudget,
		"replay":  (&ReplayEngine{}).RunBlockWithBudget,
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(ctx, nil); err == nil {
				t.Fatal("work admitted without a budget")
			}
			budget, err := NewWorkBudget(1, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			release, err := budget.acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if err := run(cancelled, budget); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled capacity wait: %v", err)
			}
			if len(budget.slots) != 1 {
				t.Fatal("cancelled waiter changed the active work slot")
			}
		})
	}
}

func TestMarketProjectionHeadRequiresAdmittedTransaction(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	writer := StreamWriter{Schema: os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"}
	topic := "market.stats.0x" + strings.Repeat("a", 40)
	if frame, err := writer.MarketProjectionHeadTx(ctx, nil, topic); err == nil || frame != nil {
		t.Fatalf("head without transaction: %q %v", frame, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := writer.MarketProjectionHeadTx(ctx, tx, topic)
	var databaseError *pgconn.PgError
	if frame != nil || !errors.As(err, &databaseError) || databaseError.Code != "42883" {
		t.Fatalf("head without installed stream schema: %q %v", frame, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if frame, err := writer.MarketProjectionHeadTx(ctx, tx, topic); frame != nil || !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("head after rollback: %q %v", frame, err)
	}
}
