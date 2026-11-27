package ws_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ws"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestSnapshotStartedTopicsRouteAndWebSocket(t *testing.T) {
	for _, waitingRow := range []bool{false, true} {
		name := "missing projection"
		if waitingRow {
			name = "empty projection"
		}
		t.Run(name, func(t *testing.T) {
			stream, pool, topic := wsStreamFixture(t)
			ctx := context.Background()
			empty := "market.price.0x3333333333333333333333333333333333333333"
			missing := "market.price.0x4444444444444444444444444444444444444444"
			topics := []string{topic, empty, missing}
			if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_projection(topic,sequence) VALUES($1,0)", empty); err != nil {
				t.Fatal(err)
			}
			server, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{"https://web.example"}})
			topicNotStartedRead(t, server, topics, 200, "", topic)
			topicNotStartedRead(t, server, []string{empty, missing}, 404, "TOPIC_NOT_STARTED")
			raw, err := stream.PublicSnapshot(ctx, topics)
			if err != nil {
				t.Fatal(err)
			}
			var snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
			}
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
			conn, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v2/stream", http.Header{"Origin": {"https://web.example"}})
			if response != nil {
				defer response.Body.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.WriteJSON(map[string]any{"op": "subscribe", "subscriptionId": "started", "topics": topics, "afterCursor": snapshot.ResumeCursor}); err != nil {
				t.Fatal(err)
			}
			if control := readFrame(t, conn); control["op"] != "subscribed" {
				t.Fatalf("subscription: %v", control)
			}
			checkpoint := readFrame(t, conn)
			if checkpoint["op"] != "checkpoint" || checkpoint["code"] != "REPLAY_COMPLETE" {
				t.Fatalf("checkpoint: %v", checkpoint)
			}
			var prior []byte
			if err := pool.QueryRow(ctx, "SELECT frame FROM api_stream.stream_projection WHERE topic=$1", topic).Scan(&prior); err != nil {
				t.Fatal(err)
			}
			var frame broker.Frame
			if err := json.Unmarshal(prior, &frame); err != nil {
				t.Fatal(err)
			}
			frame.EventID, frame.Topic = uuid.NewString(), missing
			if waitingRow {
				frame.Topic = empty
			}
			var payload map[string]any
			if err := json.Unmarshal(frame.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			payload["baseAsset"].(map[string]any)["address"] = strings.TrimPrefix(frame.Topic, "market.price.")
			frame.Payload, err = json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if err := stream.AppendProjectionTx(ctx, tx, frame); err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := conn.SetReadDeadline(started.Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			_, raw, err = conn.ReadMessage()
			if err != nil || interfaces.ValidateJSON("StreamControl", raw) != nil {
				t.Fatalf("terminal control: %s %v", raw, err)
			}
			var control map[string]any
			if err := json.Unmarshal(raw, &control); err != nil {
				t.Fatal(err)
			}
			if time.Since(started) > 3*time.Second || control["op"] != "resnapshot_required" || control["code"] != "RESUME_INVALID" || control["snapshotUrl"] != broker.SnapshotRoute(topics) || control["cursor"] != checkpoint["cursor"] {
				t.Fatalf("resnapshot control: %v", control)
			}
			_, _, err = conn.ReadMessage()
			var closed *websocket.CloseError
			if !errors.As(err, &closed) || closed.Code != websocket.CloseNormalClosure {
				t.Fatalf("subscription close: %v", err)
			}
		})
	}
}
