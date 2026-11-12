package ws_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ws"
	"github.com/gorilla/websocket"
)

func TestAuthenticatedWebSocketLiveActions(t *testing.T) {
	stream, _, topic := wsStreamFixture(t)
	const origin = "https://web.example"
	server, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	snapshotRequest, err := http.NewRequest(http.MethodGet, server.URL+"/v2/snapshots?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshotRequest.Header.Set("Origin", origin)
	response, err := server.Client().Do(snapshotRequest)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Data struct {
			Snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
			} `json:"snapshot"`
		} `json:"data"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&snapshot)
	response.Body.Close()
	if decodeErr != nil || response.StatusCode != http.StatusOK || snapshot.Data.Snapshot.ResumeCursor == "" {
		t.Fatalf("snapshot status=%d cursor=%q error=%v", response.StatusCode, snapshot.Data.Snapshot.ResumeCursor, decodeErr)
	}
	for _, tc := range []struct {
		name, action, operation, code string
	}{
		{"heartbeat", `{"op":"ping"}`, "heartbeat", ""},
		{"invalid JSON", `{`, "error", "FIELD_INVALID"},
		{"unknown operation", `{"op":"unknown"}`, "error", "FIELD_INVALID"},
		{"subscribe change", `{"op":"subscribe"}`, "resnapshot_required", "SCOPE_CHANGE_REQUIRED"},
		{"unsubscribe change", `{"op":"unsubscribe"}`, "resnapshot_required", "SCOPE_CHANGE_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/v2/stream"
			conn, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(endpoint, http.Header{"Origin": []string{origin}})
			if err != nil {
				t.Fatalf("connect: %v %v", err, response)
			}
			defer conn.Close()
			request, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": snapshot.Data.Snapshot.ResumeCursor})
			if err != nil {
				t.Fatal(err)
			}
			if err = conn.WriteMessage(websocket.TextMessage, request); err != nil {
				t.Fatal(err)
			}
			if frame := readFrame(t, conn); frame["op"] != "subscribed" {
				t.Fatalf("subscription: %v", frame)
			}
			if frame := readFrame(t, conn); frame["op"] != "checkpoint" || frame["code"] != "REPLAY_COMPLETE" {
				t.Fatalf("replay: %v", frame)
			}
			if err = conn.WriteMessage(websocket.TextMessage, []byte(tc.action)); err != nil {
				t.Fatal(err)
			}
			frame := readFrame(t, conn)
			if frame["op"] != tc.operation || tc.code != "" && frame["code"] != tc.code {
				t.Fatalf("%s control: %v", tc.name, frame)
			}
		})
	}
}
