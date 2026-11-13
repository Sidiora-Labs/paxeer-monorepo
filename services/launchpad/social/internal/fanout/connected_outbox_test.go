package fanout_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	goredis "github.com/redis/go-redis/v9"
)

func TestConnectedControlAndLiveOutboxesDeliverOnce(t *testing.T) {
	ctx := context.Background()
	store := durableFixture(t)
	if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
		t.Fatal(err)
	}
	message := sendDM(t, store, "connected-outbox")
	if err := store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, message.Sequence); err != nil {
		t.Fatal(err)
	}
	var publishedID, publishedWallet string
	var publishedBody map[string]any
	count, err := store.DrainControlOutbox(ctx, 100, func(_ context.Context, id, wallet string, raw []byte) error {
		publishedID, publishedWallet = id, wallet
		return json.Unmarshal(raw, &publishedBody)
	})
	if err != nil || count != 1 || publishedID == "" || publishedWallet != addr(0xbb) || publishedBody["type"] != "read_marker" {
		t.Fatalf("control delivery = %d, %q, %q, %v: %v", count, publishedID, publishedWallet, publishedBody, err)
	}
	if count, err = store.DrainControlOutbox(ctx, 100, func(context.Context, string, string, []byte) error {
		t.Fatal("delivered control event repeated")
		return nil
	}); err != nil || count != 0 {
		t.Fatalf("control replay = %d: %v", count, err)
	}

	uri := internaltest.NewRedisURL(t)
	options, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatal(err)
	}
	redis := goredis.NewClient(options)
	t.Cleanup(func() { _ = redis.Close() })
	channel := "social:live:fixture"
	subscription := redis.Subscribe(ctx, channel)
	t.Cleanup(func() { _ = subscription.Close() })
	if _, err = subscription.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Pool.Exec(ctx, `INSERT INTO chat_broker_inbox(event_id,payload_hash,broker_stream,broker_sequence) VALUES('connected-live','digest','SOCIAL',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Pool.Exec(ctx, `INSERT INTO chat_live_outbox(event_id,channel,payload) VALUES('connected-live',$1,$2)`, channel, []byte(`{"type":"pool_message","content":"visible"}`)); err != nil {
		t.Fatal(err)
	}
	count, err = store.DrainLiveOutbox(ctx, 100, redis)
	if err != nil || count != 1 {
		t.Fatalf("live delivery = %d: %v", count, err)
	}
	deadline, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	got, err := subscription.ReceiveMessage(deadline)
	var livePayload map[string]any
	if err != nil || json.Unmarshal([]byte(got.Payload), &livePayload) != nil || livePayload["type"] != "pool_message" || livePayload["content"] != "visible" {
		t.Fatalf("live payload = %+v: %v", got, err)
	}
	count, err = store.DrainLiveOutbox(ctx, 100, redis)
	if err != nil || count != 0 {
		t.Fatalf("live replay = %d: %v", count, err)
	}
}
