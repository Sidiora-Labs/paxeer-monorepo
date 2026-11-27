package sse_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/sse"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/migrations"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestSnapshotStartedTopicsEventStream(t *testing.T) {
	for _, waitingRow := range []bool{false, true} {
		name := "missing projection"
		if waitingRow {
			name = "empty projection"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv(internaltest.ExternalPostgresEnv, os.Getenv("API_TEST_DATABASE_URL"))
			pool := internaltest.NewPostgres(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
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
			if _, err := pool.Exec(ctx, up); err != nil {
				t.Fatal(err)
			}
			stream, err := broker.NewPostgresStream(pool, "api_stream", bytes.Repeat([]byte{9}, 32), sharedauth.HTTPStreamAuthority{})
			if err != nil {
				t.Fatal(err)
			}
			topic := "market.price.0x1111111111111111111111111111111111111111"
			empty := "market.price.0x3333333333333333333333333333333333333333"
			missing := "market.price.0x4444444444444444444444444444444444444444"
			topics := []string{topic, empty, missing}
			if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_projection(topic,sequence) VALUES($1,0)", empty); err != nil {
				t.Fatal(err)
			}
			asset := map[string]any{"kind": "erc20", "chainId": "125", "address": strings.TrimPrefix(topic, "market.price."), "currencyCode": nil, "decimals": 18}
			quote := map[string]any{"kind": "erc20", "chainId": "125", "address": "0x2222222222222222222222222222222222222222", "currencyCode": nil, "decimals": 6}
			payload := map[string]any{"numerator": "1", "denominator": "1", "baseAsset": asset, "quoteAsset": quote, "source": map[string]any{"kind": "post_swap_spot", "amountBasis": "effective_reserves", "sourceEventId": nil, "position": nil, "providerId": nil, "observedAt": "2026-10-12T00:00:00.000Z", "sourceReference": nil}}
			frame := broker.Frame{EventID: uuid.NewString(), Topic: topic, Sequence: "1", Revision: "1", OccurredAt: "2026-10-12T00:00:00.000Z", Watermark: json.RawMessage("null")}
			frame.Payload, err = json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			commit := func(frame broker.Frame) {
				t.Helper()
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				if err := stream.AppendProjectionTx(ctx, tx, frame); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			}
			commit(frame)
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
			server, _ := newServer(t, sse.Options{PrivateStream: stream, AllowedOrigins: []string{"https://web.example"}})
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v2/stream/sse?"+url.Values{"topics": {strings.Join(topics, ",")}}.Encode(), nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Origin", "https://web.example")
			request.Header.Set("Last-Event-ID", snapshot.ResumeCursor)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
				t.Fatalf("stream headers: %d %v", response.StatusCode, response.Header)
			}
			reader := bufio.NewReader(response.Body)
			if subscribed := readNextData(t, reader); subscribed["op"] != "subscribed" {
				t.Fatalf("subscription: %v", subscribed)
			}
			checkpoint := readNextData(t, reader)
			if checkpoint["op"] != "checkpoint" || checkpoint["code"] != "REPLAY_COMPLETE" {
				t.Fatalf("checkpoint: %v", checkpoint)
			}
			frame.EventID, frame.Topic = uuid.NewString(), missing
			if waitingRow {
				frame.Topic = empty
			}
			asset["address"] = strings.TrimPrefix(frame.Topic, "market.price.")
			frame.Payload, err = json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			commit(frame)
			type result struct {
				body []byte
				err  error
			}
			ended := make(chan result, 1)
			go func() { raw, err := io.ReadAll(reader); ended <- result{raw, err} }()
			timer := time.NewTimer(time.Until(started.Add(3 * time.Second)))
			defer timer.Stop()
			select {
			case <-timer.C:
				t.Fatal("started topic did not require a snapshot within three seconds")
			case terminal := <-ended:
				if terminal.err != nil {
					t.Fatal(terminal.err)
				}
				event := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(terminal.body)), "data: "))
				if interfaces.ValidateJSON("StreamControl", []byte(event)) != nil {
					t.Fatalf("terminal control: %s", terminal.body)
				}
				var control map[string]any
				if err := json.Unmarshal([]byte(event), &control); err != nil {
					t.Fatal(err)
				}
				if time.Since(started) > 3*time.Second || control["op"] != "resnapshot_required" || control["code"] != "RESUME_INVALID" || control["snapshotUrl"] != broker.SnapshotRoute(topics) || control["cursor"] != checkpoint["cursor"] {
					t.Fatalf("resnapshot control: %v", control)
				}
			}
		})
	}
}
