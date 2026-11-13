package fanout_test

import (
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketControlFramesValidateAndUpdateSubscriptions(t *testing.T) {
	h := newWSHarness(t)
	client := h.dial(t, addr(0xaa))
	if err := client.WriteMessage(websocket.TextMessage, []byte(`{"type":`)); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, client, "error"); got["message"] != "Invalid JSON" {
		t.Fatalf("malformed frame = %+v", got)
	}
	send(t, client, map[string]any{"type": "unknown"})
	if got := readUntil(t, client, "error"); got["message"] != "Unknown message type: unknown" {
		t.Fatalf("unknown frame = %+v", got)
	}
	send(t, client, map[string]any{"type": "join_pool", "pool_address": "not-a-pool"})
	if got := readUntil(t, client, "error"); got["message"] != "invalid pool address" {
		t.Fatalf("invalid pool join = %+v", got)
	}
	send(t, client, map[string]any{"type": "leave_pool", "pool_address": "not-a-pool"})
	if got := readUntil(t, client, "error"); got["message"] != "invalid pool address" {
		t.Fatalf("invalid pool leave = %+v", got)
	}
	pool := addr(1)
	send(t, client, map[string]any{"type": "join_pool", "pool_address": pool})
	if got := readUntil(t, client, "joined_pool"); got["pool_address"] != pool {
		t.Fatalf("joined pool = %+v", got)
	}
	send(t, client, map[string]any{"type": "leave_pool", "pool_address": pool})
	if got := readUntil(t, client, "left_pool"); got["pool_address"] != pool {
		t.Fatalf("left pool = %+v", got)
	}
	send(t, client, map[string]any{"type": "subscribe_dms"})
	if got := readUntil(t, client, "subscribed_dms"); got["type"] != "subscribed_dms" {
		t.Fatalf("direct subscription = %+v", got)
	}
	send(t, client, map[string]any{"type": "dm", "to": "bad-wallet", "content": "private message"})
	if got := readUntil(t, client, "error"); got["client_message_id"] == "" || got["message"] == "" {
		t.Fatalf("invalid direct send = %+v", got)
	}
}
