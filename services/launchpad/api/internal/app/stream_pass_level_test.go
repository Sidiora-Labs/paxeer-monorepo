package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type streamRelayPassLog struct {
	bytes.Buffer
	stop context.CancelFunc
}

func (w *streamRelayPassLog) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.stop()
	return n, err
}

func TestStreamRelayPassLevelRetry(t *testing.T) {
	runStreamRelayPassLevel(t, "retry", 0, "WARN", "stream relay publication retried")
}

func runStreamRelayPassLevel(t *testing.T, mode string, attempts int, level, message string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := internaltest.NewAPIRuntimeDatabase(t, "api_rest_runtime", "api_stream", "recovery")
	admin, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	runtimePool, err := pgxpool.New(ctx, db.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimePool.Close()
	nc, err := nats.Connect(streamRelayNATSURL(t), nats.Timeout(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	stream := fmt.Sprintf("PASS_LEVEL_%d", time.Now().UnixNano())
	if _, err = js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{"market.>", "private.portfolio.*"}, Storage: nats.FileStorage, MaxAge: 72 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(stream); err != nil {
			t.Error(err)
		}
	}()
	document := streamRelayDocument{Routes: []delivery.StreamRoute{{Stream: stream, Subjects: []string{"market.>", "private.portfolio.*"}}}, Epoch: db.Epoch, Region: db.Region, Batch: 1, MaxAttempts: 3, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, IdleInterval: 100 * time.Millisecond, MaxLag: time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 50}
	relay, err := newStreamRelay(ctx, runtimePool, nc, "pass_level", document)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := broker.NewPostgresStream(admin, "api_stream", []byte(strings.Repeat("k", 32)), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	var frame broker.Frame
	for _, committed := range committedFrames(t) {
		if strings.HasPrefix(committed.topic, "market.price.") {
			if err = json.Unmarshal(committed.wire, &frame); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if frame.Topic == "" {
		t.Fatal("committed price frame required")
	}
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = reader.AppendProjectionTx(ctx, tx, frame); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var partition int64
	if err = admin.QueryRow(ctx, "SELECT partition_id FROM api_stream.stream_projection WHERE topic=$1", frame.Topic).Scan(&partition); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, "INSERT INTO api_stream.stream_delivery_offsets(partition_id) VALUES($1)", partition); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, "INSERT INTO api_stream.stream_delivery_publications(partition_id,sequence,event_id,frame_sha256,attempts) SELECT partition_id,sequence,event_id,encode(sha256(frame),'hex'),$2 FROM api_stream.stream_delivery_outbox WHERE partition_id=$1", partition, attempts); err != nil {
		t.Fatal(err)
	}
	if mode == "quarantine" {
		if _, err = admin.Exec(ctx, "UPDATE api_stream.stream_projection SET topic=$1 WHERE partition_id=$2", strings.Replace(frame.Topic, "market.price.", "market.trade.", 1), partition); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err = js.UpdateStream(&nats.StreamConfig{Name: stream, Subjects: []string{"unavailable.>"}, Storage: nats.FileStorage, MaxAge: 72 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	logs := &streamRelayPassLog{stop: stop}
	done := make(chan struct{})
	go func() {
		relay.Run(runCtx, slog.New(slog.NewJSONHandler(logs, nil)))
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		stop()
		<-done
		t.Fatal("relay pass did not report its result")
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one failed pass line, got %s", logs.String())
	}
	var record struct {
		Level   string `json:"level"`
		Message string `json:"msg"`
		Error   string `json:"err"`
	}
	if err = json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatal(err)
	}
	if record.Level != level || record.Message != message {
		t.Fatalf("pass log: %s", logs.String())
	}
	if mode != "quarantine" {
		for _, detail := range []string{"stream: persistence publication unavailable", "no response from stream", frame.Topic, fmt.Sprintf("partition=%d", partition), "sequence=1", fmt.Sprintf("attempts=%d", attempts+1)} {
			if !strings.Contains(record.Error, detail) {
				t.Fatalf("pass error lacks %q: %s", detail, record.Error)
			}
		}
	}
	if err = relay.Ready(ctx); err == nil || !strings.Contains(err.Error(), "pass failing") {
		t.Fatalf("failed relay pass readiness: %v", err)
	}
	var state, class string
	var recordedAttempts, next int
	if err = admin.QueryRow(ctx, "SELECT p.state,p.failure_class,p.attempts,c.next_sequence FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_offsets c USING(partition_id) WHERE p.partition_id=$1", partition).Scan(&state, &class, &recordedAttempts, &next); err != nil {
		t.Fatal(err)
	}
	wantState, wantClass := "pending", string(delivery.Transient)
	if mode == "quarantine" {
		wantState, wantClass = "quarantined", string(delivery.IntegrityConflict)
	}
	if state != wantState || class != wantClass || recordedAttempts != attempts+1 || next != 1 {
		t.Fatalf("pass state=%s class=%s attempts=%d next=%d", state, class, recordedAttempts, next)
	}
}

func TestStreamRelayPassLevelAboveAttemptBound(t *testing.T) {
	runStreamRelayPassLevel(t, "retry", 3, "ERROR", "stream relay pass failed")
}

func TestStreamRelayPassLevelAtAttemptBound(t *testing.T) {
	runStreamRelayPassLevel(t, "retry", 2, "WARN", "stream relay publication retried")
}

func TestStreamRelayPassLevelQuarantine(t *testing.T) {
	runStreamRelayPassLevel(t, "quarantine", 0, "ERROR", "stream relay pass failed")
}
