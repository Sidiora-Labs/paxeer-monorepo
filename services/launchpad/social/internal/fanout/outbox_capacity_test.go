package fanout_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
)

func TestConnectedReadMarkerReservesDurableOutboxCapacity(t *testing.T) {
	ctx := context.Background()
	store := durableFixture(t)
	if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
		t.Fatal(err)
	}
	message := sendDM(t, store, "reserved-read-marker")
	store.PendingOutboxLimit = 2
	store.MaxOutboxAge = time.Hour
	if err := store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, message.Sequence); !errors.Is(err, fanout.ErrUnavailable) {
		t.Fatalf("full outbox accepted read marker: %v", err)
	}
	var through, markers int
	if err := store.Pool.QueryRow(ctx, `SELECT read_sequence FROM chat_members WHERE conversation_id=$1 AND wallet=$2`, message.ConversationID, addr(0xbb)).Scan(&through); err != nil || through != 0 {
		t.Fatalf("read position after full outbox = %d: %v", through, err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chat_control_outbox`).Scan(&markers); err != nil || markers != 0 {
		t.Fatalf("control intents after full outbox = %d: %v", markers, err)
	}
	store.PendingOutboxLimit = 3
	if err := store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, message.Sequence); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT read_sequence FROM chat_members WHERE conversation_id=$1 AND wallet=$2`, message.ConversationID, addr(0xbb)).Scan(&through); err != nil || through != int(message.Sequence) {
		t.Fatalf("committed read position = %d: %v", through, err)
	}
	store.MaxOutboxAge = time.Nanosecond
	if err := store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, 0); !errors.Is(err, fanout.ErrUnavailable) {
		t.Fatalf("aged outbox accepted read marker: %v", err)
	}
}
