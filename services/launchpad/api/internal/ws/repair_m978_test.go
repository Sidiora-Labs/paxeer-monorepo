package ws_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ws"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func repairM978Action(t *testing.T, c *websocket.Conn, op string, channels, pools []string, want string) map[string]any {
	t.Helper()
	if err := c.WriteJSON(map[string]any{"type": op, "channels": channels, "pools": pools, "timeframes": []string{"1m"}}); err != nil {
		t.Fatal(err)
	}
	got := readFrame(t, c)
	if got["type"] != want {
		t.Fatalf("%s: got %v, want %s", op, got, want)
	}
	return got
}

func repairM978Set(t *testing.T, frame map[string]any, key string, want []string) {
	t.Helper()
	values, ok := frame[key].([]any)
	if !ok || len(values) != len(want) {
		t.Fatalf("%s = %v, want %v", key, frame[key], want)
	}
	set := make(map[string]bool, len(want))
	for _, value := range want {
		set[value] = true
	}
	for _, value := range values {
		s, ok := value.(string)
		if !ok || !set[s] {
			t.Fatalf("unexpected or duplicate %s value %v in %v", key, value, frame)
		}
		delete(set, s)
	}
	if len(set) != 0 {
		t.Fatalf("missing %s values: %v", key, set)
	}
}

func repairM978Error(t *testing.T, frame map[string]any, want string) {
	t.Helper()
	if frame["message"] != want {
		t.Fatalf("error = %v, want %q", frame, want)
	}
}

func repairM978Pool(n int) string { return fmt.Sprintf("0x%040x", 0xabc000+n) }

func repairM978Dispatch(t *testing.T, b *broker.Broker, path, pool string) {
	t.Helper()
	if path == "/ws" {
		b.Dispatch(constants.ChannelSwap, swapPayload(pool))
		return
	}
	payload, err := json.Marshal(map[string]any{
		"poolAddress": pool, "timeframe": "1m", "candleStart": 60,
		"open": "1000000", "high": "2000000", "low": "500000", "close": "1500000",
		"volumeUsdl": "0", "volumeToken": "0", "buyVolumeUsdl": "0", "sellVolumeUsdl": "0",
		"tradeCount": 1, "uniqueTraders": 1, "largeTradeCount": 0,
		"mcapOpen": "0", "mcapHigh": "0", "mcapLow": "0", "mcapClose": "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Dispatch(constants.ChannelCandleUpdate, payload)
}

func repairM978Delivery(t *testing.T, c *websocket.Conn, path, pool string) {
	t.Helper()
	got := readFrame(t, c)
	if path == "/ws" {
		if got["type"] != "swap" || got["pool"] != strings.ToLower(pool) {
			t.Fatalf("unexpected routed frame: %v, want swap for %s", got, pool)
		}
		return
	}
	data, ok := got["data"].(map[string]any)
	if got["type"] != "candle_update" || !ok || data["poolAddress"] != pool {
		t.Fatalf("unexpected routed frame: %v, want candle for %s", got, pool)
	}
}

func TestRepairM_978_FrameLimit(t *testing.T) {
	for _, path := range []string{"/ws", "/ws/candles"} {
		for _, fragmented := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fragmented=%t", path, fragmented), func(t *testing.T) {
				b := broker.New(broker.Options{Logger: discard()})
				hub := ws.NewHub(ws.Options{Broker: b, Logger: discard(), Flush: time.Millisecond})
				r := chi.NewRouter()
				hub.Register(r)
				srv := httptest.NewServer(r)
				t.Cleanup(srv.Close)
				url := "ws" + strings.TrimPrefix(srv.URL, "http") + path
				bufferSize := 32 * 1024
				if fragmented {
					bufferSize = 256
				}
				dialer := websocket.Dialer{WriteBufferSize: bufferSize, HandshakeTimeout: 3 * time.Second}
				c, resp, err := dialer.Dial(url, nil)
				if resp != nil {
					_ = resp.Body.Close()
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = c.Close() })
				readUntil(t, c, "connected")
				const limit = 16 * 1024
				ping := `{"type":"ping"}`
				atLimit := ping + strings.Repeat(" ", limit-len(ping))
				if fragmented {
					if err := c.WriteMessage(websocket.TextMessage, []byte(atLimit)); err != nil {
						t.Fatal(err)
					}
					if got := readFrame(t, c); got["type"] != "pong" {
						t.Fatalf("frame at byte limit rejected: %v", got)
					}
				}
				oversized := []byte(atLimit + " ")
				if fragmented {
					// Each fragment stays below the limit; their total exceeds it.
					w, err := c.NextWriter(websocket.TextMessage)
					if err != nil {
						t.Fatal(err)
					}
					for start := 0; start < len(oversized); start += 128 {
						end := start + 128
						if end > len(oversized) {
							end = len(oversized)
						}
						if _, err := w.Write(oversized[start:end]); err != nil {
							t.Fatal(err)
						}
					}
					if err := w.Close(); err != nil {
						t.Fatal(err)
					}
				} else if err := c.WriteMessage(websocket.TextMessage, oversized); err != nil {
					t.Fatal(err)
				}
				if err := c.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
					t.Fatal(err)
				}
				_, _, err = c.ReadMessage()
				if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
					t.Fatalf("oversized message: got %v, want close 1009", err)
				}
				deadline := time.Now().Add(3 * time.Second)
				for hub.Connections() != 0 || b.Subscribers() != 0 {
					if time.Now().After(deadline) {
						t.Fatalf("oversized connection leaked: connections=%d subscribers=%d", hub.Connections(), b.Subscribers())
					}
					time.Sleep(time.Millisecond)
				}
			})
		}
	}
}

func TestRepairM_978_CanonicalPoolAddresses(t *testing.T) {
	for _, path := range []string{"/ws", "/ws/candles"} {
		t.Run(path, func(t *testing.T) {
			srv, b := newServer(t, ws.Options{})
			c := dial(t, srv, path)
			readUntil(t, c, "connected")
			pool := repairM978Pool(1)
			mixed := "0x" + strings.ToUpper(pool[2:])
			channels := []string{constants.ChannelSwap}
			ack := repairM978Action(t, c, "subscribe", channels, []string{mixed, pool, " " + mixed + " "}, "subscribed")
			repairM978Set(t, ack, "pools", []string{pool})
			if path == "/ws/candles" {
				repairM978Set(t, ack, "timeframes", []string{"1m"})
			}
			for _, invalid := range []string{"", " ", "0xAAA", "0x" + strings.Repeat("a", 39), "0x" + strings.Repeat("a", 41), strings.Repeat("a", 40), "0x" + strings.Repeat("g", 40), "0x" + strings.Repeat("0", 40)} {
				for _, op := range []string{"subscribe", "unsubscribe"} {
					bad := repairM978Action(t, c, op, []string{constants.ChannelMarketCreated}, []string{pool, repairM978Pool(2), invalid}, "error")
					repairM978Error(t, bad, "Invalid pool address")
					ack = repairM978Action(t, c, "subscribe", nil, nil, "subscribed")
					repairM978Set(t, ack, "pools", []string{pool})
					if path == "/ws" {
						repairM978Set(t, ack, "channels", channels)
					}
				}
			}
			if path == "/ws" {
				b.Dispatch(constants.ChannelMarketCreated, swapPayload(pool))
			}
			repairM978Dispatch(t, b, path, repairM978Pool(2))
			repairM978Dispatch(t, b, path, mixed)
			repairM978Delivery(t, c, path, mixed)
			ack = repairM978Action(t, c, "unsubscribe", nil, []string{mixed}, "unsubscribed")
			repairM978Set(t, ack, "pools", nil)
		})
	}
}

func TestRepairM_978_IncrementalPoolBudget(t *testing.T) {
	for _, path := range []string{"/ws", "/ws/candles"} {
		t.Run(path, func(t *testing.T) {
			srv, b := newServer(t, ws.Options{})
			c := dial(t, srv, path)
			readUntil(t, c, "connected")
			channels := []string{constants.ChannelSwap}
			pools := make([]string, 128)
			for i := range pools {
				pools[i] = repairM978Pool(i + 1)
				ack := repairM978Action(t, c, "subscribe", channels, []string{pools[i]}, "subscribed")
				repairM978Set(t, ack, "pools", pools[:i+1])
			}
			extra := repairM978Pool(129)
			bad := repairM978Action(t, c, "subscribe", []string{constants.ChannelMarketCreated}, []string{extra}, "error")
			repairM978Error(t, bad, "Pool subscription limit exceeded")
			ack := repairM978Action(t, c, "subscribe", nil, []string{"0x" + strings.ToUpper(pools[0][2:]), pools[0]}, "subscribed")
			repairM978Set(t, ack, "pools", pools)
			if path == "/ws" {
				repairM978Set(t, ack, "channels", channels)
				b.Dispatch(constants.ChannelMarketCreated, swapPayload(pools[0]))
			}
			repairM978Dispatch(t, b, path, extra)
			repairM978Dispatch(t, b, path, pools[0])
			repairM978Delivery(t, c, path, pools[0])

			// Another connection has its own budget.
			other := dial(t, srv, path)
			readUntil(t, other, "connected")
			ack = repairM978Action(t, other, "subscribe", channels, []string{extra}, "subscribed")
			repairM978Set(t, ack, "pools", []string{extra})
			if err := other.Close(); err != nil {
				t.Fatal(err)
			}

			ack = repairM978Action(t, c, "unsubscribe", nil, []string{pools[0]}, "unsubscribed")
			repairM978Set(t, ack, "pools", pools[1:])
			bad = repairM978Action(t, c, "subscribe", nil, []string{extra, repairM978Pool(130)}, "error")
			repairM978Error(t, bad, "Pool subscription limit exceeded")
			ack = repairM978Action(t, c, "subscribe", nil, nil, "subscribed")
			repairM978Set(t, ack, "pools", pools[1:])
			ack = repairM978Action(t, c, "subscribe", nil, []string{extra}, "subscribed")
			want := append(append([]string(nil), pools[1:]...), extra)
			repairM978Set(t, ack, "pools", want)
			repairM978Dispatch(t, b, path, pools[0])
			repairM978Dispatch(t, b, path, repairM978Pool(130))
			repairM978Dispatch(t, b, path, extra)
			repairM978Delivery(t, c, path, extra)
		})
	}
}

func TestRepairM_978_InitialPoolBudget(t *testing.T) {
	for _, path := range []string{"/ws", "/ws/candles"} {
		t.Run(path, func(t *testing.T) {
			srv, _ := newServer(t, ws.Options{})
			c := dial(t, srv, path)
			readUntil(t, c, "connected")
			pools := make([]string, 129)
			for i := range pools {
				pools[i] = repairM978Pool(i + 1)
			}
			bad := repairM978Action(t, c, "subscribe", []string{constants.ChannelSwap}, pools, "error")
			repairM978Error(t, bad, "Pool subscription limit exceeded")
			ack := repairM978Action(t, c, "subscribe", nil, nil, "subscribed")
			repairM978Set(t, ack, "pools", nil)
			if path == "/ws" {
				repairM978Set(t, ack, "channels", nil)
			}
		})
	}
}
