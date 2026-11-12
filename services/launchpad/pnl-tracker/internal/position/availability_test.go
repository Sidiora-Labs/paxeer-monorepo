package position_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
)

func TestCanonicalConsumerRequiresInventoryAndBrokerMessage(t *testing.T) {
	if _, err := position.NewCanonicalConsumer(nil); err == nil {
		t.Fatal("missing inventory accepted")
	}
	consumer, err := position.NewCanonicalConsumer(&l16.AccountingConsumer{})
	if err != nil || consumer == nil {
		t.Fatal(consumer, err)
	}
	if err = consumer.ProcessDelivery(context.Background(), nil); err == nil {
		t.Fatal("missing broker delivery accepted")
	}
	var message *natsdriver.Message
	if err = consumer.ProcessDelivery(context.Background(), message); err == nil {
		t.Fatal("nil broker message accepted")
	}
}

func TestLegacyConsumerKeepsCommittedTradeWhenCacheIsUnavailable(t *testing.T) {
	consumer, st, rdb := setup(t)
	ctx := context.Background()
	event := position.SwapEvent{PoolAddress: "0xpool", Sender: "0xcache-user", IsBuy: true, AmountIn: "100", AmountOut: "10", Price: "0", Fee: "0", BlockNumber: 1, BlockTimestamp: 1, TxHash: "0xcache-buy"}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := consumer.ProcessEvent(canceled, event); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	invalid := event
	invalid.AmountOut = "invalid"
	if err := consumer.ProcessEvent(ctx, invalid); err == nil {
		t.Fatal("invalid trade accepted")
	}
	if err := rdb.Close(); err != nil {
		t.Fatal(err)
	}
	if err := consumer.ProcessEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetPosition(ctx, event.Sender, event.PoolAddress)
	if err != nil || got == nil || got.CurrentHoldings != "10" || got.TradeCount != 1 {
		t.Fatal(got, err)
	}
	if err := consumer.ProcessEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
}
