package fanout_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
)

func TestDurableHistoryAndCurrentEventRejectStaleIdentity(t *testing.T) {
	store := durableFixture(t)
	ctx := context.Background()
	message := sendDM(t, store, "boundary-current")
	if err := store.Authorized(ctx, addr(0xcc), message.ConversationID); !errors.Is(err, fanout.ErrNotFound) {
		t.Fatalf("unjoined history = %v", err)
	}
	if _, err := store.List(ctx, "invalid", 10); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("invalid conversation owner = %v", err)
	}
	if _, err := store.List(ctx, addr(0xaa), 101); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("unbounded conversation page = %v", err)
	}
	if _, err := store.History(ctx, addr(0xaa), message.ConversationID, "", 101); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("unbounded history page = %v", err)
	}
	if _, err := store.History(ctx, addr(0xcc), message.ConversationID, "", 10); !errors.Is(err, fanout.ErrNotFound) {
		t.Fatalf("unjoined message history = %v", err)
	}
	for _, cursor := range []string{"invalid", "a.b", "not-base64.!", "a.", "x.y.z"} {
		if _, err := store.History(ctx, addr(0xaa), message.ConversationID, cursor, 10); !errors.Is(err, fanout.ErrCursor) {
			t.Fatalf("cursor %q = %v", cursor, err)
		}
	}
	if err := store.MarkRead(ctx, addr(0xaa), message.ConversationID, -1); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("negative read marker = %v", err)
	}
	if err := store.MarkRead(ctx, addr(0xcc), message.ConversationID, 1); !errors.Is(err, fanout.ErrNotFound) {
		t.Fatalf("unjoined read marker = %v", err)
	}
	if err := store.JoinPool(ctx, "invalid", addr(1)); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("invalid pool actor = %v", err)
	}
	if err := store.JoinPool(ctx, addr(0xaa), addr(1)); err != nil {
		t.Fatalf("joined indexed pool = %v", err)
	}
	if err := store.JoinPool(ctx, addr(0xaa), addr(0xdead)); err == nil {
		t.Fatal("unknown pool admitted")
	}
	for _, tc := range []struct {
		payload string
		want    error
	}{
		{`{`, nil},
		{`{}`, fanout.ErrInvalid},
		{`{"type":"read_marker","conversation_id":"` + message.ConversationID + `","through_sequence":"1"}`, nil},
	} {
		value, err := store.CurrentEvent(ctx, []byte(tc.payload))
		if tc.payload == "{" {
			if err == nil {
				t.Fatal("malformed current event accepted")
			}
			continue
		}
		if !errors.Is(err, tc.want) || (tc.want == nil && len(value) == 0) {
			t.Fatalf("current event %q = %s: %v", tc.payload, value, err)
		}
	}
	for _, change := range []func(*fanout.DurableMessage){
		func(v *fanout.DurableMessage) { v.ConversationID = "other" },
		func(v *fanout.DurableMessage) { v.Sequence++ },
		func(v *fanout.DurableMessage) { v.Type = "pool_message" },
	} {
		candidate := message
		change(&candidate)
		wire, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.CurrentEvent(ctx, wire); !errors.Is(err, fanout.ErrConflict) {
			t.Fatalf("changed event identity = %v", err)
		}
	}
	wire, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if current, err := store.CurrentEvent(ctx, wire); err != nil || len(current) == 0 {
		t.Fatalf("current committed event = %s: %v", current, err)
	}
}
