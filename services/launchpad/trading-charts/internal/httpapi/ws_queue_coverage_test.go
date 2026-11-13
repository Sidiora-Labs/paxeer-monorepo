package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketQueueAndConnectionBounds(t *testing.T) {
	h := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 1, maxPerIP: 1}
	closed := &clientSub{send: make(chan []byte, 1), quit: make(chan struct{})}
	closed.stop()
	h.enqueue(closed, []byte("ignored"))
	if len(closed.send) != 0 || closed.queued.Load() != 0 {
		t.Fatalf("closed client accepted queued data: %d %d", len(closed.send), closed.queued.Load())
	}
	full := &clientSub{send: make(chan []byte, 1), quit: make(chan struct{})}
	h.enqueue(full, []byte("first"))
	h.enqueue(full, []byte("second"))
	select {
	case <-full.quit:
	default:
		t.Fatal("full client queue stayed connected")
	}
	if full.queued.Load() != int64(len("first")) || string(<-full.send) != "first" {
		t.Fatalf("bounded queue retained wrong message: %d", full.queued.Load())
	}
	large := &clientSub{send: make(chan []byte, 1), quit: make(chan struct{})}
	h.enqueue(large, make([]byte, 256*1024+1))
	select {
	case <-large.quit:
	default:
		t.Fatal("oversized client message stayed connected")
	}
	if large.queued.Load() != 0 || len(large.send) != 0 {
		t.Fatalf("oversized message entered queue: %d %d", large.queued.Load(), len(large.send))
	}
	address := fixture().PoolAddress
	client := &clientSub{send: make(chan []byte, 2), quit: make(chan struct{}), topics: map[string]struct{}{address + ":1m": {}}}
	h.indexClient(client)
	h.broadcastCandleUpdate("invalid")
	invalid := fixture()
	invalid.Open = "1.0"
	wire, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	h.broadcastCandleUpdate(string(wire))
	if len(client.send) != 0 {
		t.Fatalf("invalid candle entered queue: %d", len(client.send))
	}
	valid, err := json.Marshal(fixture())
	if err != nil {
		t.Fatal(err)
	}
	h.broadcastCandleUpdate(string(valid))
	if len(client.send) != 1 {
		t.Fatalf("valid candle missed subscriber: %d", len(client.send))
	}
	var delivery struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(<-client.send, &delivery); err != nil || delivery.Type != "candle.updated" {
		t.Fatalf("queued candle: %+v, %v", delivery, err)
	}
	h.removeFromIndex(client)
	if len(h.subscriptionIndex) != 0 {
		t.Fatalf("removed client retained an index: %d", len(h.subscriptionIndex))
	}
	for _, tc := range []struct {
		name, message      string
		maxConns, maxPerIP int
	}{
		{"server capacity", "Server at capacity", 0, 1},
		{"client capacity", "Too many connections from this IP", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quota := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: tc.maxConns, maxPerIP: tc.maxPerIP}
			server := httptest.NewServer(http.HandlerFunc(quota.handleWS))
			defer server.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_, raw, err := conn.ReadMessage()
			if err != nil || !strings.Contains(string(raw), tc.message) {
				t.Fatalf("connection quota: %s, %v", raw, err)
			}
			if len(quota.clients) != 0 {
				t.Fatalf("quota admitted a client: %d", len(quota.clients))
			}
		})
	}
}
