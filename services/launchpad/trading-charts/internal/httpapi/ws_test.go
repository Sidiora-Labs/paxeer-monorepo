package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture() CandleSnapshot {
	return CandleSnapshot{Version: 1, Chain: 125, PoolAddress: "0x" + strings.Repeat("a", 40), Timeframe: "1m", Start: 60, Revision: "1", AlgorithmVersion: "v1", SourceWatermark: "125:4", Cursor: "opaque", Open: "1", High: "2", Low: "1", Close: "2", VolumeUsdl: "3", VolumeToken: "0", BuyVolumeUsdl: "3", SellVolumeUsdl: "0", McapOpen: "1", McapHigh: "2", McapLow: "1", McapClose: "2"}
}
func TestSnapshotExactAndCoalesced(t *testing.T) {
	c := NewCandleCoalescer(2)
	a := fixture()
	for _, r := range []string{"1", "3", "2", "3"} {
		a.Revision = r
		if e := c.Add(a); e != nil {
			t.Fatal(e)
		}
	}
	b := a
	b.PoolAddress = "0x" + strings.Repeat("b", 40)
	if e := c.Add(b); e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	out := c.Drain(now)
	if len(out) != 2 {
		t.Fatal(out)
	}
	for _, v := range out {
		if v.Revision != "3" {
			t.Fatal(v)
		}
	}
	a.Revision = "4"
	_ = c.Add(a)
	if len(c.Drain(now.Add(499*time.Millisecond))) != 0 {
		t.Fatal("over 2Hz")
	}
	if len(c.Drain(now.Add(500*time.Millisecond))) != 1 {
		t.Fatal("missing flush")
	}
	a.Open = "1.0"
	if _, e := EncodeCandle(a); e == nil {
		t.Fatal("noncanonical accepted")
	}
}
func TestConflictAndBound(t *testing.T) {
	c := NewCandleCoalescer(1)
	a := fixture()
	_ = c.Add(a)
	a.Open = "3"
	if c.Add(a) == nil {
		t.Fatal("conflict accepted")
	}
	a.PoolAddress = "0x" + strings.Repeat("b", 40)
	if c.Add(a) == nil {
		t.Fatal("capacity exceeded")
	}
}
func TestSlowConsumerAndHealthy(t *testing.T) {
	h := &wsHub{}
	slow := &clientSub{send: make(chan []byte, 256), quit: make(chan struct{})}
	fast := &clientSub{send: make(chan []byte, 1), quit: make(chan struct{})}
	for i := 0; i < 257; i++ {
		h.enqueue(slow, make([]byte, 1024))
	}
	select {
	case <-slow.quit:
	default:
		t.Fatal("not evicted")
	}
	if slow.queued.Load() > 256*1024 {
		t.Fatal("byte bound")
	}
	h.enqueue(fast, []byte("healthy"))
	if string(<-fast.send) != "healthy" {
		t.Fatal("blocked")
	}
}
func TestSocketValidationAndOrigins(t *testing.T) {
	h := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 10, maxPerIP: 10}
	srv := httptest.NewServer(http.HandlerFunc(h.handleWS))
	defer srv.Close()
	u := "ws" + strings.TrimPrefix(srv.URL, "http")
	header := http.Header{"Origin": []string{"https://evil.invalid"}}
	if c, _, e := websocket.DefaultDialer.Dial(u, header); e == nil {
		c.Close()
		t.Fatal("origin accepted")
	}
	conn, _, e := websocket.DefaultDialer.Dial(u, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	read := func() map[string]interface{} {
		t.Helper()
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, raw, e := conn.ReadMessage()
		if e != nil {
			t.Fatal(e)
		}
		var v map[string]interface{}
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	read()
	for _, pools := range [][]string{{}, make([]string, 21), {"invalid"}} {
		conn.WriteJSON(map[string]interface{}{"type": "subscribe", "pools": pools, "timeframes": []string{"1m"}})
		if read()["type"] != "error" {
			t.Fatal("invalid accepted")
		}
	}
	pool := fixture().PoolAddress
	conn.WriteJSON(map[string]interface{}{"type": "subscribe", "pools": []string{pool, strings.ToUpper(pool[:2]) + pool[2:]}, "timeframes": []string{"1m"}})
	if read()["type"] != "subscribed" {
		t.Fatal("alias rejected")
	}
	uppercase := fixture()
	uppercase.PoolAddress = strings.ToUpper(uppercase.PoolAddress)
	h.broadcastCandleUpdate(string(mustEncode(t, uppercase)))
	snapshot := read()
	if snapshot["type"] != "candle.updated" || snapshot["data"].(map[string]interface{})["poolAddress"] != pool {
		t.Fatal("uppercase snapshot missed canonical subscription", snapshot)
	}
	conn.WriteJSON(map[string]interface{}{"type": "unsubscribe", "pools": []string{pool}, "timeframes": []string{"1m"}})
	if read()["type"] != "unsubscribed" {
		t.Fatal("unsubscribe")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.subscriptionIndex) != 0 {
		t.Fatal("leaked index")
	}
}
func mustEncode(t *testing.T, c CandleSnapshot) []byte {
	t.Helper()
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestRedisPartitionToRealSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	channel := "kd-25af2ac44985146cf57fae0c:L12:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	url := "redis://:kindle_fixture_password@127.0.0.1:26379/0"
	rdb := goredis.NewClient(parseRedisOpts(url))
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	h := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 10, maxPerIP: 10, channel: channel, retentionCapacity: 2}
	go h.runRedisSubscriber(ctx, url)
	deadline := time.Now().Add(3 * time.Second)
	for {
		n, e := rdb.PubSubNumSub(ctx, channel).Result()
		if e != nil {
			t.Fatal(e)
		}
		if n[channel] == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("subscriber not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	srv := httptest.NewServer(http.HandlerFunc(h.handleWS))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	read := func() map[string]interface{} {
		t.Helper()
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, raw, e := conn.ReadMessage()
		if e != nil {
			t.Fatal(e)
		}
		var v map[string]interface{}
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	read()
	a := fixture()
	b := a
	b.PoolAddress = "0x" + strings.Repeat("b", 40)
	conn.WriteJSON(map[string]interface{}{"type": "subscribe", "pools": []string{a.PoolAddress, b.PoolAddress}, "timeframes": []string{"1m"}})
	if read()["type"] != "subscribed" {
		t.Fatal("subscribe")
	}
	for _, r := range []string{"1", "3", "2", "3"} {
		a.Revision = r
		if err = rdb.Publish(ctx, channel, mustEncode(t, a)).Err(); err != nil {
			t.Fatal(err)
		}
	}
	if err = rdb.Publish(ctx, channel, mustEncode(t, b)).Err(); err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for len(seen) < 2 {
		msg := read()
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal(msg)
		}
		seen[data["poolAddress"].(string)] = data["revision"].(string)
	}
	if seen[a.PoolAddress] != "3" || seen[b.PoolAddress] != "1" {
		t.Fatal(seen)
	}
	a.Revision = "2"
	b.Revision = "2"
	if err = rdb.Publish(ctx, channel, mustEncode(t, a)).Err(); err != nil {
		t.Fatal(err)
	}
	if err = rdb.Publish(ctx, channel, mustEncode(t, b)).Err(); err != nil {
		t.Fatal(err)
	}
	next := read()
	nextData := next["data"].(map[string]interface{})
	if nextData["poolAddress"] != b.PoolAddress || nextData["revision"] != "2" {
		t.Fatal("stale state reemitted after drain", next)
	}
	a.Revision = "3"
	a.Open = "3"
	if err = rdb.Publish(ctx, channel, mustEncode(t, a)).Err(); err != nil {
		t.Fatal(err)
	}
	if conflict := read(); conflict["type"] != "resync_required" {
		t.Fatal("cross-window integrity error did not trigger resync", conflict)
	}
	a.Open = "1"
	a.Revision = "1"
	a.Start = 120
	if err = rdb.Publish(ctx, channel, mustEncode(t, a)).Err(); err != nil {
		t.Fatal(err)
	}
	if saturated := read(); saturated["type"] != "resync_required" {
		t.Fatal("retention saturation silently lost live state", saturated)
	}

}
func TestQuotaAtomicAndForgedIdentity(t *testing.T) {
	h := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 10, maxPerIP: 10}
	srv := httptest.NewServer(http.HandlerFunc(h.handleWS))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	read := func() string {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, raw, e := conn.ReadMessage()
		if e != nil {
			t.Fatal(e)
		}
		var msg struct{ Type string }
		json.Unmarshal(raw, &msg)
		return msg.Type
	}
	read()
	for i := 0; i < 21; i++ {
		conn.WriteJSON(map[string]interface{}{"type": "subscribe", "pools": []string{fmt.Sprintf("0x%040x", i+1)}, "timeframes": []string{"1m"}, "requestId": "quota"})
		got := read()
		want := "subscribed"
		if i == 20 {
			want = "error"
		}
		if got != want {
			t.Fatalf("topic %d: %s", i, got)
		}
	}
	h.mu.RLock()
	n := len(h.subscriptionIndex)
	h.mu.RUnlock()
	if n != 20 {
		t.Fatal(n)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	if clientIPFromRequest(req) != "127.0.0.1" {
		t.Fatal("forged identity")
	}
	conn.WriteMessage(websocket.TextMessage, make([]byte, 16*1024+1))
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, e := conn.ReadMessage(); e == nil {
		t.Fatal("oversize accepted")
	}
}

func TestExplicitPairsDoNotCreateCrossProducts(t *testing.T) {
	a := fixture().PoolAddress
	b := "0x" + strings.Repeat("b", 40)
	topics, err := validateTopics([]seriesSubscription{{a, "1m"}, {b, "5m"}, {strings.ToUpper(a), "1m"}}, nil, nil)
	if err != nil || len(topics) != 2 {
		t.Fatalf("topics=%v err=%v", topics, err)
	}
	if _, ok := topics[a+":5m"]; ok {
		t.Fatal("cross product allocated")
	}
	if _, err = validateTopics([]seriesSubscription{{a, "1m"}}, []string{a}, []string{"1m"}); err == nil {
		t.Fatal("ambiguous format")
	}
}

func TestSlowSocketCloseIncludesCursor(t *testing.T) {
	h := &wsHub{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		sub := &clientSub{conn: conn, send: make(chan []byte, 256), quit: make(chan struct{}), cursor: "accepted-stream-cursor"}
		h.enqueue(sub, make([]byte, 256*1024))
		h.enqueue(sub, []byte("overflow"))
		h.writePump(sub)
	}))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, _, err = conn.ReadMessage()
		if err != nil {
			break
		}
	}
	closeErr, ok := err.(*websocket.CloseError)
	if !ok || closeErr.Code != 1013 || !strings.Contains(closeErr.Text, "accepted-stream-cursor") {
		t.Fatalf("close: %v", err)
	}
}

func TestRevisionKnowledgeSurvivesDrain(t *testing.T) {
	c := NewCandleCoalescer(1)
	v := fixture()
	v.Revision = "3"
	v.PoolAddress = strings.ToUpper(v.PoolAddress)
	if err := c.Add(v); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	first := c.Drain(now)
	if len(first) != 1 || first[0].PoolAddress != fixture().PoolAddress {
		t.Fatalf("not normalized: %v", first)
	}
	v.PoolAddress = strings.ToLower(v.PoolAddress)
	v.Revision = "2"
	if err := c.Add(v); err != nil {
		t.Fatal(err)
	}
	if got := c.Drain(now.Add(time.Second)); len(got) != 0 {
		t.Fatalf("stale revision reemitted: %v", got)
	}
	v.Revision = "3"
	v.Open = "3"
	if err := c.Add(v); err == nil {
		t.Fatal("cross-window conflict accepted")
	}
	v.Open = "1"
	v.Cursor = "new-transport-cursor"
	if err := c.Add(v); err != nil {
		t.Fatal(err)
	}
	if got := c.Drain(now.Add(2 * time.Second)); len(got) != 0 {
		t.Fatal("same revision reemitted")
	}
	v.Revision = "4"
	if err := c.Add(v); err != nil {
		t.Fatal(err)
	}
	if got := c.Drain(now.Add(3 * time.Second)); len(got) != 1 || got[0].Revision != "4" {
		t.Fatal(got)
	}
	v.PoolAddress = "0x" + strings.Repeat("b", 40)
	if err := c.Add(v); err == nil {
		t.Fatal("retention capacity evicted revision knowledge")
	}
}

func TestSocketAdversarialFloodAndRelease(t *testing.T) {
	h := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 8, maxPerIP: 1}
	srv := httptest.NewServer(http.HandlerFunc(h.handleWS))
	defer srv.Close()
	endpoint := "ws" + strings.TrimPrefix(srv.URL, "http")
	headers := http.Header{"X-Forwarded-For": {"8.8.8.8"}, "Forwarded": {"for=8.8.8.8"}, "X-Real-IP": {"8.8.8.8"}}
	first, _, err := websocket.DefaultDialer.Dial(endpoint, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	read := func(conn *websocket.Conn) string {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var frame struct{ Type string }
		if json.Unmarshal(raw, &frame) != nil {
			t.Fatal("invalid server JSON")
		}
		return frame.Type
	}
	if read(first) != "connected" {
		t.Fatal("welcome missing")
	}
	headers.Set("X-Forwarded-For", "9.9.9.9")
	second, _, err := websocket.DefaultDialer.Dial(endpoint, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if read(second) != "error" {
		t.Fatal("forged forwarding headers bypassed socket quota")
	}
	_ = first.WriteJSON(map[string]any{"type": "subscribe", "series": []seriesSubscription{{fixture().PoolAddress, "1m"}}})
	if read(first) != "subscribed" {
		t.Fatal("subscription missing")
	}
	for i := 0; i < 100; i++ {
		if first.WriteJSON(map[string]any{"type": "subscribe", "series": []seriesSubscription{{fixture().PoolAddress, "1m"}}, "requestId": fmt.Sprint(i)}) != nil {
			break
		}
	}
	_ = first.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		if _, _, err := first.ReadMessage(); err != nil {
			break
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		h.mu.RLock()
		released := len(h.clients) == 0 && len(h.subscriptionIndex) == 0 && len(h.ipCounts) == 0
		h.mu.RUnlock()
		if released {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("flood teardown retained subscriptions or IP quota")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestSocketRejectsMalformedOriginAndBinaryFrames(t *testing.T) {
	h := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 4, maxPerIP: 4}
	srv := httptest.NewServer(http.HandlerFunc(h.handleWS))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	for _, origin := range []string{"http://user@" + host, "http://" + host + "/path", "http://" + host + "?forged=1", "http://" + host + "#fragment", "https://evil.invalid"} {
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+host, http.Header{"Origin": {origin}})
		if err == nil {
			_ = conn.Close()
			t.Fatalf("malformed origin admitted: %s", origin)
		}
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+host, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err = conn.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	if err = conn.WriteMessage(websocket.BinaryMessage, []byte(`{"type":"subscribe"}`)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = conn.ReadMessage(); err == nil {
		t.Fatal("binary subscription frame admitted")
	}
}

// Exercise the route registration used by the application, with the admitted
// canonical producer and a loopback replay peer. The peer owns retained frames
// and cursors; the charts proxy must preserve them across its own restart.
func testAdmittedCandleApplicationStream(t *testing.T, frames [][]byte, retainedPath string) {
	t.Helper()
	if len(frames) != 2 {
		t.Fatal("two real producer revisions required")
	}
	retained, err := json.Marshal(frames)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(retainedPath, retained, 0600); err != nil {
		t.Fatal(err)
	}
	load := func() [][]byte {
		t.Helper()
		raw, err := os.ReadFile(retainedPath)
		if err != nil {
			t.Error(err)
			return nil
		}
		var recovered [][]byte
		if err = json.Unmarshal(raw, &recovered); err != nil {
			t.Error(err)
			return nil
		}
		return recovered
	}
	latest, err := interfaces.DecodeCandleEvent(frames[1])
	if err != nil {
		t.Fatal(err)
	}
	cursors := []interfaces.OpaqueResumeCursor{"snapshot_0", "retained_1", "retained_2"}
	subscriptionID := "repair-m1050"
	snapshotURL := "/v2/snapshots?topics=" + latest.Topic
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recovered := load()
		if len(recovered) != 2 {
			http.Error(w, "retained source unavailable", 503)
			return
		}
		if r.URL.Path == "/v2/snapshots" {
			if r.URL.Query().Get("topics") != latest.Topic {
				t.Error("snapshot topic lost")
				w.WriteHeader(422)
				return
			}
			envelope := interfaces.TopicSnapshotReadResponse{RequestId: "10500000-0000-4000-8000-000000000001", Quality: "complete", GeneratedAt: latest.OccurredAt, Data: interfaces.TopicSnapshotReadData{Snapshot: interfaces.TopicSnapshot{SnapshotId: latest.Payload.SnapshotId, GeneratedAt: latest.OccurredAt, ResumeCursor: cursors[2], Topics: []interfaces.TopicSnapshotEntry{{Topic: latest.Topic, Sequence: latest.Sequence, Revision: latest.Revision, Payload: interfaces.TopicSnapshotEntryPayload{Variant2: &latest.Payload}}}}}}
			raw, err := json.Marshal(envelope)
			if err != nil {
				t.Error(err)
				return
			}
			if _, err = interfaces.DecodeTopicSnapshotReadResponse(raw); err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		}
		if r.URL.Path != "/v2/stream" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Origin") != "https://charts.example" {
			t.Error("origin lost")
			w.WriteHeader(403)
			return
		}
		upgrade := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }, Subprotocols: []string{"sidiora.v2"}}
		conn, err := upgrade.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		subscribe, err := interfaces.DecodeSubscribe(raw)
		if err != nil || subscribe.AfterCursor == nil || len(subscribe.Topics) != 1 || string(subscribe.Topics[0]) != latest.Topic {
			t.Error("subscribe interface changed", err)
			return
		}
		index := -1
		if *subscribe.AfterCursor == cursors[0] {
			index = 0
		}
		if *subscribe.AfterCursor == cursors[1] {
			index = 1
		}
		if index >= 0 {
			if err = conn.WriteMessage(websocket.TextMessage, recovered[index]); err != nil {
				t.Error(err)
				return
			}
			control := interfaces.StreamControl{Op: "checkpoint", SubscriptionId: &subscriptionID, Cursor: &cursors[index+1]}
			if err = conn.WriteJSON(control); err != nil {
				t.Error(err)
			}
		} else {
			code := "SNAPSHOT_REQUIRED"
			control := interfaces.StreamControl{Op: "resnapshot_required", Code: &code, SubscriptionId: &subscriptionID, SnapshotUrl: &snapshotURL}
			if err = conn.WriteJSON(control); err != nil {
				t.Error(err)
			}
		}
	}))
	defer upstream.Close()
	start := func() (*httptest.Server, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		router := chi.NewRouter()
		RegisterWS(ctx, router, WSDeps{ReplayGatewayOrigin: upstream.URL})
		return httptest.NewServer(router), cancel
	}
	open := func(endpoint string, cursor interfaces.OpaqueResumeCursor) *websocket.Conn {
		t.Helper()
		dialer := websocket.Dialer{Subprotocols: []string{"sidiora.v2"}, HandshakeTimeout: 2 * time.Second}
		conn, response, err := dialer.Dial("ws"+strings.TrimPrefix(endpoint, "http")+"/v2/stream", http.Header{"Origin": {"https://charts.example"}})
		if err != nil {
			if response != nil {
				_ = response.Body.Close()
			}
			t.Fatal(err)
		}
		if conn.Subprotocol() != "sidiora.v2" {
			t.Fatal("subprotocol lost")
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err = conn.WriteJSON(interfaces.Subscribe{Op: "subscribe", SubscriptionId: subscriptionID, Topics: []interfaces.TopicName{interfaces.TopicName(latest.Topic)}, AfterCursor: &cursor}); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	read := func(conn *websocket.Conn) []byte {
		t.Helper()
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	first, cancel := start()
	conn := open(first.URL, cursors[0])
	if got := read(conn); !bytes.Equal(got, frames[0]) {
		t.Fatalf("producer frame changed: %s", got)
	}
	checkpoint, err := interfaces.DecodeStreamControl(read(conn))
	if err != nil || checkpoint.Op != "checkpoint" || checkpoint.Cursor == nil || *checkpoint.Cursor != cursors[1] {
		t.Fatal("checkpoint lost", checkpoint, err)
	}
	_ = conn.Close()
	cancel()
	first.Close()
	// Drop the original byte slices and reopen the charts transport. Its volatile
	// state is gone; only the peer's retained producer bytes and client cursor remain.
	frames = nil
	reopened, cancelReopened := start()
	defer reopened.Close()
	defer cancelReopened()
	conn = open(reopened.URL, *checkpoint.Cursor)
	resumed, err := interfaces.DecodeCandleEvent(read(conn))
	if err != nil || resumed.Revision != "2" || resumed.Sequence != "2" || resumed.Payload.Trades != "2" || resumed.Payload.VolumeUsdlRaw != "2000003" || resumed.Payload.Close.Numerator != "500001" || resumed.Payload.Close.Denominator != "500000" || resumed.Payload.Close.Source.SourceEventId == nil || *resumed.Payload.Close.Source.SourceEventId != *latest.Payload.Close.Source.SourceEventId {
		t.Fatal("canonical producer state did not resume", resumed, err)
	}
	next, err := interfaces.DecodeStreamControl(read(conn))
	if err != nil || next.Cursor == nil || *next.Cursor != cursors[2] {
		t.Fatal("resume checkpoint", next, err)
	}
	_ = conn.Close()
	conn = open(reopened.URL, "expired_cursor")
	control, err := interfaces.DecodeStreamControl(read(conn))
	if err != nil || control.Op != "resnapshot_required" || control.Code == nil || *control.Code != "SNAPSHOT_REQUIRED" || control.SnapshotUrl == nil || *control.SnapshotUrl != snapshotURL {
		t.Fatal("resync refusal lost", control, err)
	}
	_ = conn.Close()
	response, err := reopened.Client().Get(reopened.URL + *control.SnapshotUrl)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := interfaces.DecodeTopicSnapshotReadResponse(raw)
	if err != nil || response.StatusCode != 200 || snapshot.Data.Snapshot.ResumeCursor != cursors[2] || len(snapshot.Data.Snapshot.Topics) != 1 {
		t.Fatalf("resnapshot failed: %d %s %v", response.StatusCode, raw, err)
	}
	recovered := snapshot.Data.Snapshot.Topics[0].Payload.Variant2
	if recovered == nil || recovered.Revision != resumed.Payload.Revision || recovered.VolumeUsdlRaw != resumed.Payload.VolumeUsdlRaw || recovered.Close.Numerator != resumed.Payload.Close.Numerator {
		t.Fatal("snapshot and socket disagree", recovered)
	}
	upstream.Close()
	response, err = reopened.Client().Get(reopened.URL + snapshotURL)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	refusal, err := interfaces.DecodeError(raw)
	if err != nil || response.StatusCode != 503 || refusal.Code != "DEPENDENCY_UNAVAILABLE" || !refusal.Retryable {
		t.Fatalf("source outage acknowledged: %d %s %v", response.StatusCode, raw, err)
	}
	_, response, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(reopened.URL, "http")+"/v2/stream", nil)
	if err == nil {
		t.Fatal("unavailable socket source accepted")
	}
	if response == nil || response.StatusCode != 503 {
		t.Fatal("socket outage status", response, err)
	}
	_ = response.Body.Close()
	// The application's unavailable legacy source is an explicit migration refusal.
	router := chi.NewRouter()
	RegisterWS(context.Background(), router, WSDeps{})
	legacy := httptest.NewRecorder()
	router.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/ws", nil))
	if legacy.Code != 410 || !strings.Contains(legacy.Body.String(), "LEGACY_STREAM_RETIRED") || !strings.Contains(legacy.Header().Get("Link"), "/v2/stream") {
		t.Fatal("legacy source accepted", legacy.Code, legacy.Body.String())
	}
}
