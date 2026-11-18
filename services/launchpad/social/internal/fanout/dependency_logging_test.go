package fanout

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDependencyResponsesLogDatabaseCause(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://social:unused@127.0.0.1:1/social")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	cause := pool.Ping(context.Background())
	if cause == nil {
		t.Fatal("closed database accepted a probe")
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	for _, tc := range []struct {
		name    string
		respond func(http.ResponseWriter)
	}{
		{"chat", func(w http.ResponseWriter) { connectedError(w, cause) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			response := httptest.NewRecorder()
			tc.respond(response)
			var logged struct {
				Error string `json:"error"`
			}
			if response.Code != http.StatusServiceUnavailable || json.Unmarshal(logs.Bytes(), &logged) != nil || logged.Error != cause.Error() || strings.Contains(logs.String(), "private-session") {
				t.Fatalf("dependency response = %d, log = %s", response.Code, logs.String())
			}
		})
	}
}

func TestRoomJoinFailureLogsDatabaseCause(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://social:unused@127.0.0.1:1/social")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	var logs bytes.Buffer
	hub := &Hub{log: slog.New(slog.NewJSONHandler(&logs, nil)), sendBuffer: 8, protocolVersion: 2}
	hub.durable = &DurableStore{Pool: pool, Policy: hub.admitSend}
	actor, target := "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"
	connection := newConn(hub, nil, actor)
	connection.principal.Topics = []string{"chat.pool." + target}
	hub.onJoinPool(connection, types.IncomingWS{PoolAddress: target})
	var logged struct {
		Op  string `json:"op"`
		Err string `json:"err"`
	}
	if err := json.Unmarshal(logs.Bytes(), &logged); err != nil || logged.Op != "durable room join" || !strings.Contains(logged.Err, "closed pool") {
		t.Fatalf("room join cause = %s: %v", logs.String(), err)
	}
	select {
	case raw := <-connection.send:
		var ack types.WSAck
		if err := json.Unmarshal(raw, &ack); err != nil || ack.Type != "error" || ack.Message != "room unavailable" || strings.Contains(string(raw), "closed pool") {
			t.Fatalf("room join response = %s: %v", raw, err)
		}
	default:
		t.Fatal("room join did not answer")
	}
	if len(connection.joinedPools()) != 0 {
		t.Fatal("failed room join subscribed the connection")
	}
}
