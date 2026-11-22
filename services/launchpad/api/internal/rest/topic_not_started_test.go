package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestTopicNotStartedProductionSnapshots(t *testing.T) {
	pool, model, stream, address, payload := productionReadsFixture(t)
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err := rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: model, Stream: stream, Admission: admission, RegisterSnapshots: true}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	started := "market.stats." + address
	otherAddress := "0x3333333333333333333333333333333333333333"
	missing := "market.stats." + otherAddress
	read := func(topics []string, status int, code string) {
		t.Helper()
		response, err := server.Client().Get(server.URL + "/v2/snapshots?" + url.Values{"topics": {strings.Join(topics, ",")}}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != status {
			t.Fatalf("snapshot: %d %s %v", response.StatusCode, raw, err)
		}
		if status == 200 {
			if err := interfaces.ValidateJSON("TopicSnapshotReadResponse", raw); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Data struct {
					Snapshot struct {
						Topics []struct{ Topic string } `json:"topics"`
					} `json:"snapshot"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &result); err != nil || len(result.Data.Snapshot.Topics) != len(topics) {
				t.Fatalf("snapshot topics: %s %v", raw, err)
			}
			return
		}
		if err := interfaces.ValidateJSON("Error", raw); err != nil {
			t.Fatal(err)
		}
		failure, err := interfaces.DecodeError(raw)
		hint, retry := interfaces.SafeInteger(1000), "1"
		if code == "TOPIC_NOT_STARTED" {
			hint, retry = 5000, "5"
		}
		if err != nil || failure.Code != code || !failure.Retryable || failure.RetryAfterMs == nil || *failure.RetryAfterMs != hint || response.Header.Get("Retry-After") != retry || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("snapshot error: %s %v %v", raw, response.Header, err)
		}
	}
	read([]string{missing}, 404, "TOPIC_NOT_STARTED")
	read([]string{started, missing}, 404, "TOPIC_NOT_STARTED")
	var schema string
	if err := pool.QueryRow(context.Background(), `SELECT nspname FROM pg_namespace WHERE nspname LIKE 'l22_stream_%'`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	projection := pgx.Identifier{schema, "stream_projection"}.Sanitize()
	if _, err := pool.Exec(context.Background(), "INSERT INTO "+projection+"(topic,sequence) VALUES($1,0)", missing); err != nil {
		t.Fatal(err)
	}
	read([]string{missing}, 404, "TOPIC_NOT_STARTED")
	read([]string{started}, 200, "")
	var stats map[string]json.RawMessage
	if err := json.Unmarshal(payload, &stats); err != nil {
		t.Fatal(err)
	}
	stats["poolAddress"], err = json.Marshal(otherAddress)
	if err != nil {
		t.Fatal(err)
	}
	nextPayload, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	frame := broker.Frame{EventID: "stats_first_frame", Topic: missing, Sequence: "1", Revision: "1", OccurredAt: "2026-10-12T00:00:00.000Z", Watermark: stats["watermark"], Payload: nextPayload}
	if err := stream.AppendProjectionTx(context.Background(), tx, frame); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	read([]string{started, missing}, 200, "")
	if logs.Len() != 0 {
		t.Fatalf("snapshot errors logged: %s", logs.String())
	}
	if _, err := pool.Exec(context.Background(), "DELETE FROM "+pgx.Identifier{schema, "stream_generation"}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	read([]string{started}, 503, "DEPENDENCY_UNAVAILABLE")
	read([]string{"market.rankings.trending"}, 503, "DEPENDENCY_UNAVAILABLE")
}
