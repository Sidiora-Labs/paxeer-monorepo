package ws_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ws"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func topicNotStartedRead(t *testing.T, server *httptest.Server, topics []string, status int, code string, expected ...string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/v2/snapshots?"+url.Values{"topics": {strings.Join(topics, ",")}}.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "https://web.example")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != status {
		t.Fatalf("snapshot: %d %s %v", response.StatusCode, raw, err)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("snapshot cache: %v", response.Header)
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
		if len(expected) == 0 {
			expected = topics
		}
		if err := json.Unmarshal(raw, &result); err != nil || len(result.Data.Snapshot.Topics) != len(expected) {
			t.Fatalf("snapshot topics: %s %v", raw, err)
		}
		for i, entry := range result.Data.Snapshot.Topics {
			if entry.Topic != expected[i] {
				t.Fatalf("snapshot topic: %s, want %s", entry.Topic, expected[i])
			}
		}
		return
	}
	if err := interfaces.ValidateJSON("Error", raw); err != nil {
		t.Fatal(err)
	}
	failure, err := interfaces.DecodeError(raw)
	if err != nil || failure.Code != code {
		t.Fatalf("snapshot error: %s %v", raw, err)
	}
	if code == "NOT_FOUND" {
		if failure.Retryable || failure.RetryAfterMs != nil || response.Header.Get("Retry-After") != "" {
			t.Fatalf("unknown topic retry: %s %v", raw, response.Header)
		}
		return
	}
	hint, retry := interfaces.SafeInteger(1000), "1"
	if code == "TOPIC_NOT_STARTED" {
		hint, retry = 5000, "5"
	}
	if !failure.Retryable || failure.RetryAfterMs == nil || *failure.RetryAfterMs != hint || response.Header.Get("Retry-After") != retry {
		t.Fatalf("snapshot retry: %s %v", raw, response.Header)
	}
}

func TestTopicNotStartedHubSnapshots(t *testing.T) {
	stream, pool, topic := wsStreamFixture(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	server, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{"https://web.example"}, Logger: logger})
	missing := "market.price.0x3333333333333333333333333333333333333333"
	topicNotStartedRead(t, server, []string{missing}, 404, "TOPIC_NOT_STARTED")
	topicNotStartedRead(t, server, []string{topic, missing}, 200, "", topic)
	if _, err := pool.Exec(context.Background(), `INSERT INTO api_stream.stream_projection(topic,sequence) VALUES($1,0)`, missing); err != nil {
		t.Fatal(err)
	}
	topicNotStartedRead(t, server, []string{missing}, 404, "TOPIC_NOT_STARTED")
	topicNotStartedRead(t, server, []string{topic}, 200, "")
	var prior []byte
	if err := pool.QueryRow(context.Background(), `SELECT frame FROM api_stream.stream_projection WHERE topic=$1`, topic).Scan(&prior); err != nil {
		t.Fatal(err)
	}
	var frame broker.Frame
	if err := json.Unmarshal(prior, &frame); err != nil {
		t.Fatal(err)
	}
	frame.Topic, frame.EventID = missing, uuid.NewString()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err := stream.AppendProjectionTx(context.Background(), tx, frame); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	topicNotStartedRead(t, server, []string{topic, missing}, 200, "")
	if _, err := pool.Exec(context.Background(), `DELETE FROM api_stream.stream_generation`); err != nil {
		t.Fatal(err)
	}
	topicNotStartedRead(t, server, []string{topic}, 503, "DEPENDENCY_UNAVAILABLE")
	topicNotStartedRead(t, server, []string{"market.rankings.trending"}, 503, "DEPENDENCY_UNAVAILABLE")
	if logs.Len() != 0 {
		t.Fatalf("snapshot errors logged: %s", logs.String())
	}
}

func TestTopicNotStartedUnknownTopicPrecedence(t *testing.T) {
	stream, pool, topic := wsStreamFixture(t)
	registry, err := rest.NewDevelopmentStream(context.Background(), rest.DevelopmentStreamOptions{
		Pool: pool, Store: store.New(pool, nil), SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{},
	})
	if err != nil {
		t.Fatal(err)
	}
	server, _ := newServer(t, ws.Options{PrivateStream: stream, TopicRegistry: registry, AllowedOrigins: []string{"https://web.example"}})
	missing := "market.price.0x3333333333333333333333333333333333333333"
	topicNotStartedRead(t, server, []string{missing}, 404, "NOT_FOUND")
	topicNotStartedRead(t, server, []string{topic, missing}, 404, "NOT_FOUND")
	topicNotStartedRead(t, server, []string{"market.rankings.trending"}, 404, "TOPIC_NOT_STARTED")
}

func TestTopicNotStartedRegistryFailure(t *testing.T) {
	stream, pool, _ := wsStreamFixture(t)
	ctx := context.Background()
	registry, err := rest.NewDevelopmentStream(ctx, rest.DevelopmentStreamOptions{
		Pool: pool, Store: store.New(pool, nil), SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{},
	})
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("3", 40)
	topics := []string{"market.price." + address}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$1,$1,$1,$2,1,1,$2)`, address, "0x"+strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://web.example", Endpoint: "/v2/stream"}
	if _, err := stream.Snapshot(ctx, sharedauth.StreamPrincipal{}, binding); !errors.Is(err, broker.ErrTopicNotStarted) {
		t.Fatalf("snapshot without first frame: %v", err)
	}
	server, _ := newServer(t, ws.Options{PrivateStream: stream, TopicRegistry: registry, AllowedOrigins: []string{"https://web.example"}})
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.pools RENAME TO pools_unavailable`); err != nil {
		t.Fatal(err)
	}
	topicNotStartedRead(t, server, topics, 503, "DEPENDENCY_UNAVAILABLE")
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.pools_unavailable RENAME TO pools`); err != nil {
		t.Fatal(err)
	}
	topicNotStartedRead(t, server, topics, 404, "TOPIC_NOT_STARTED")
}
