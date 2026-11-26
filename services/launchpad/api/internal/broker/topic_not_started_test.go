package broker

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestTopicNotStartedSnapshotAndResume(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	missing := "market.stats.0x1111111111111111111111111111111111111111"
	for _, topics := range [][]string{{first.Topic}, {missing, first.Topic}} {
		if raw, err := stream.PublicSnapshot(ctx, topics); !errors.Is(err, ErrTopicNotStarted) || !errors.Is(err, ErrStreamUnavailable) || raw != nil {
			t.Fatalf("unstarted snapshot: %s %v", raw, err)
		}
	}
	commitStreamFrame(t, ctx, stream, first)
	if raw, err := stream.PublicSnapshot(ctx, []string{first.Topic, missing}); err != nil || interfaces.ValidateJSON("TopicSnapshot", raw) != nil {
		t.Fatalf("mixed snapshot: %s %v", raw, err)
	} else {
		var snapshot struct{ Topics []struct{ Topic string } }
		if err := json.Unmarshal(raw, &snapshot); err != nil || len(snapshot.Topics) != 1 || snapshot.Topics[0].Topic != first.Topic {
			t.Fatalf("mixed snapshot topics: %s %v", raw, err)
		}
	}
	if _, err := stream.pool.Exec(ctx, "INSERT INTO "+stream.table("stream_projection")+"(topic,sequence) VALUES($1,0)", missing); err != nil {
		t.Fatal(err)
	}
	if raw, err := stream.PublicSnapshot(ctx, []string{missing}); !errors.Is(err, ErrTopicNotStarted) || raw != nil {
		t.Fatalf("empty projection: %s %v", raw, err)
	}
	raw, err := stream.PublicSnapshot(ctx, []string{first.Topic})
	if err != nil || interfaces.ValidateJSON("TopicSnapshot", raw) != nil {
		t.Fatalf("started snapshot: %s %v", raw, err)
	}
	cursor := snapshotCursor(t, ctx, stream, []string{first.Topic})
	replay, err := stream.PublicResume(ctx, []string{first.Topic}, cursor)
	if err != nil {
		t.Fatal(err)
	}
	replay.Close()
	if _, err := stream.pool.Exec(ctx, "UPDATE "+stream.table("stream_projection")+" SET sequence=0,frame=NULL WHERE topic=$1", first.Topic); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.PublicResume(ctx, []string{first.Topic}, cursor); !errors.Is(err, ErrFrame) || errors.Is(err, ErrTopicNotStarted) {
		t.Fatalf("resume with empty projection: %v", err)
	}
	if _, err := stream.pool.Exec(ctx, "DELETE FROM "+stream.table("stream_projection")+" WHERE topic=$1", missing); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.pool.Exec(ctx, "DELETE FROM "+stream.table("stream_delivery_outbox")+" WHERE partition_id IN (SELECT partition_id FROM "+stream.table("stream_projection")+" WHERE topic=$1)", first.Topic); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.pool.Exec(ctx, "DELETE FROM "+stream.table("stream_projection")+" WHERE topic=$1", first.Topic); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.PublicResume(ctx, []string{first.Topic}, cursor); err != ErrStreamUnavailable {
		t.Fatalf("resume with missing projection: %v", err)
	}
}

func TestTopicNotStartedAvailabilityAndIntegrity(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, first)
	if _, err := stream.pool.Exec(ctx, "UPDATE "+stream.table("stream_projection")+" SET frame=$1 WHERE topic=$2", []byte(`{}`), first.Topic); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.PublicSnapshot(ctx, []string{first.Topic}); !errors.Is(err, ErrFrame) || errors.Is(err, ErrTopicNotStarted) {
		t.Fatalf("invalid frame: %v", err)
	}
	if _, err := stream.pool.Exec(ctx, "DELETE FROM "+stream.table("stream_generation")); err != nil {
		t.Fatal(err)
	}
	for _, topic := range []string{first.Topic, "market.rankings.trending"} {
		if _, err := stream.PublicSnapshot(ctx, []string{topic}); err != ErrStreamUnavailable {
			t.Fatalf("missing generation: %v", err)
		}
	}
	for _, unconfigured := range []*PostgresStream{nil, {}} {
		if _, err := unconfigured.PublicSnapshot(ctx, []string{first.Topic}); err != ErrStreamUnavailable {
			t.Fatalf("unconfigured stream: %v", err)
		}
	}
}

func TestTopicNotStartedHTTPErrorEnvelope(t *testing.T) {
	for _, credential := range []string{"", "Authorization", "Cookie"} {
		request := httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil)
		if credential != "" {
			request.Header.Set(credential, "credential")
		}
		response := httptest.NewRecorder()
		response.Header().Set("Retry-After", "1")
		WriteStreamHTTPError(response, request, 404, "TOPIC_NOT_STARTED")
		if err := interfaces.ValidateJSON("Error", response.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		failure, err := interfaces.DecodeError(response.Body.Bytes())
		if err != nil || response.Code != 404 || failure.Code != "TOPIC_NOT_STARTED" || !failure.Retryable || failure.RetryAfterMs == nil || *failure.RetryAfterMs != 5000 || response.Header().Get("Retry-After") != "5" {
			t.Fatalf("unstarted error: %d %s %v %v", response.Code, response.Body, response.Header(), err)
		}
		cache := "no-store"
		if credential != "" {
			cache = "private, no-store"
		}
		if response.Header().Get("Cache-Control") != cache {
			t.Fatalf("cache control: %v", response.Header())
		}
	}
}
