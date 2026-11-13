package sse_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/sse"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/migrations"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestAuthenticatedEventStreamAdmission(t *testing.T) {
	const origin = "https://web.example"
	topic := "market.price.0x" + strings.Repeat("1", 40)
	t.Setenv(internaltest.ExternalPostgresEnv, os.Getenv("API_TEST_DATABASE_URL"))
	pool := internaltest.NewPostgres(t)
	migration, err := migrations.Migrations().ReadFile("00001_api_stream.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, up, found := strings.Cut(string(migration), "-- +goose Up")
	if !found {
		t.Fatal("stream migration missing Up")
	}
	up, _, found = strings.Cut(up, "-- +goose Down")
	if !found {
		t.Fatal("stream migration missing Down")
	}
	if _, err = pool.Exec(context.Background(), up); err != nil {
		t.Fatal(err)
	}
	stream, err := broker.NewPostgresStream(pool, "api_stream", bytes.Repeat([]byte{9}, 32), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	hub := sse.NewHub(sse.Options{PrivateStream: stream, AllowedOrigins: []string{origin}, Fanout: broker.NewTopicFanout(), MaxConns: 1})
	for _, tc := range []struct {
		name, query, requestOrigin, authorization, wantBody string
		status                                              int
	}{
		{"untrusted origin", "topics=" + topic, "https://other.example", "", "FORBIDDEN", 403},
		{"missing topics", "", origin, "", "FIELD_INVALID", 422},
		{"malformed query", "topics=%GG", origin, "", "FIELD_INVALID", 422},
		{"oversized query", "topics=" + strings.Repeat("x", 8193), origin, "", "FORBIDDEN", 403},
		{"duplicate topics", "topics=" + topic + "&topics=" + topic, origin, "", "FIELD_INVALID", 422},
		{"invalid topic", "topics=market.price.*", origin, "", "FIELD_INVALID", 422},
		{"unauthenticated private topic", "topics=private.notifications.0x" + strings.Repeat("1", 40), origin, "Bearer bad", "AUTH_REQUIRED", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v2/stream/sse?"+tc.query, nil)
			req.Header.Set("Origin", tc.requestOrigin)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			res := httptest.NewRecorder()
			hub.HandleV2(res, req)
			if res.Code != tc.status || !strings.Contains(res.Body.String(), tc.wantBody) {
				t.Fatalf("status %d body %q, want %d %q", res.Code, res.Body.String(), tc.status, tc.wantBody)
			}
		})
	}
	unavailable := sse.NewHub(sse.Options{})
	for _, empty := range []*sse.Hub{unavailable, sse.NewHub(sse.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})} {
		empty.Drain()
		res := httptest.NewRecorder()
		empty.HandleV2(res, httptest.NewRequest(http.MethodGet, "/v2/stream/sse?topics="+topic, nil))
		if res.Code != 503 || !strings.Contains(res.Body.String(), "DEPENDENCY_UNAVAILABLE") {
			t.Fatalf("unavailable stream: %d %s", res.Code, res.Body.String())
		}
	}
	if count := hub.Connections(); count != 0 {
		t.Fatalf("initial connections: %d", count)
	}
	server, _ := newServer(t, sse.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/stream/sse?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != 200 || !strings.Contains(string(body), "SNAPSHOT_REQUIRED") {
		t.Fatalf("stream status %d body %q error %v", res.StatusCode, body, err)
	}

	invalidRequest, err := http.NewRequest(http.MethodGet, server.URL+"/v2/stream/sse?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	invalidRequest.Header.Set("Origin", origin)
	invalidRequest.Header.Set("Last-Event-ID", "invalid")
	invalidResponse, err := http.DefaultClient.Do(invalidRequest)
	if err != nil {
		t.Fatal(err)
	}
	invalidBody, readErr := io.ReadAll(invalidResponse.Body)
	if err := invalidResponse.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || invalidResponse.StatusCode != 200 || !strings.Contains(string(invalidBody), "resnapshot_required") {
		t.Fatalf("invalid resume: %d %s %v", invalidResponse.StatusCode, invalidBody, readErr)
	}
	asset := map[string]any{"kind": "erc20", "chainId": "125", "address": "0x" + strings.Repeat("1", 40), "currencyCode": nil, "decimals": 18}
	quote := map[string]any{"kind": "erc20", "chainId": "125", "address": "0x" + strings.Repeat("2", 40), "currencyCode": nil, "decimals": 6}
	payload, err := json.Marshal(map[string]any{"numerator": "1", "denominator": "1", "baseAsset": asset, "quoteAsset": quote, "source": map[string]any{"kind": "post_swap_spot", "amountBasis": "effective_reserves", "sourceEventId": nil, "position": nil, "providerId": nil, "observedAt": "2026-10-12T00:00:00.000Z", "sourceReference": nil}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	frame := broker.Frame{EventID: uuid.NewString(), Topic: topic, Sequence: "1", Revision: "1", OccurredAt: "2026-10-12T00:00:00.000Z", Watermark: json.RawMessage("null"), Payload: payload}
	if err = stream.AppendProjectionTx(context.Background(), tx, frame); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := stream.PublicSnapshot(context.Background(), []string{topic})
	if err != nil {
		t.Fatal(err)
	}
	var cursor struct {
		ResumeCursor string `json:"resumeCursor"`
	}
	if err = json.Unmarshal(snapshot, &cursor); err != nil || cursor.ResumeCursor == "" {
		t.Fatalf("resume cursor: %s, %v", snapshot, err)
	}
	replayServer := httptest.NewServer(http.HandlerFunc(hub.HandleV2))
	defer replayServer.Close()
	replayCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	replayRequest, err := http.NewRequestWithContext(replayCtx, http.MethodGet, replayServer.URL+"/v2/stream/sse?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	replayRequest.Header.Set("Origin", origin)
	replayRequest.Header.Set("Last-Event-ID", cursor.ResumeCursor)
	replayResponse, err := replayServer.Client().Do(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer replayResponse.Body.Close()
	if replayResponse.StatusCode != 200 || replayResponse.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("replay headers: %d %v", replayResponse.StatusCode, replayResponse.Header)
	}
	reader := bufio.NewReader(replayResponse.Body)
	if subscribed := readNextData(t, reader); subscribed["op"] != "subscribed" {
		t.Fatalf("subscription: %v", subscribed)
	}
	if checkpoint := readNextData(t, reader); checkpoint["op"] != "checkpoint" || checkpoint["code"] != "REPLAY_COMPLETE" {
		t.Fatalf("replay checkpoint: %v", checkpoint)
	}
	liveTx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = liveTx.Rollback(context.Background()) }()
	live := frame
	live.EventID = uuid.NewString()
	live.Sequence, live.Revision = "2", "2"
	if err := stream.AppendProjectionTx(context.Background(), liveTx, live); err != nil {
		t.Fatal(err)
	}
	if err := liveTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if event := readNextData(t, reader); event["topic"] != topic || event["sequence"] != "2" || event["eventId"] != live.EventID {
		t.Fatalf("committed live event: %v", event)
	}
	if count := hub.Connections(); count != 1 {
		t.Fatalf("live connections: %d", count)
	}
	limitedRequest, err := http.NewRequest(http.MethodGet, replayServer.URL+"/v2/stream/sse?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	limitedRequest.Header.Set("Origin", origin)
	limitedResponse, err := http.DefaultClient.Do(limitedRequest)
	if err != nil {
		t.Fatal(err)
	}
	limitedBody, readErr := io.ReadAll(limitedResponse.Body)
	if err := limitedResponse.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || limitedResponse.StatusCode != 503 || !strings.Contains(string(limitedBody), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("connection capacity: %d %s %v", limitedResponse.StatusCode, limitedBody, readErr)
	}
	hub.Drain()
	if hub.Ready() {
		t.Fatal("draining hub admitted new streams")
	}
	if heartbeat := readNextData(t, reader); heartbeat["op"] != "heartbeat" || heartbeat["code"] != "SERVER_DRAINING" {
		t.Fatalf("drain heartbeat: %v", heartbeat)
	}
}
