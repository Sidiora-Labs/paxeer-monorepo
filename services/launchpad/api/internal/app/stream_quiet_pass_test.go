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

func TestStreamRelayQuietPassEmpty(t *testing.T) {
	logs, removed := runStreamRelayQuietPasses(t, 0, 21)
	if len(removed) < 20 {
		t.Fatalf("observed %d retention passes, want at least 20", len(removed))
	}
	for pass, count := range removed {
		if count != 0 {
			t.Fatalf("quiet pass %d removed %d frames", pass+1, count)
		}
	}
	if strings.Contains(logs, "stream relay retention pass") || strings.Contains(logs, `"level":"ERROR"`) || strings.Contains(logs, `"level":"WARN"`) {
		t.Fatalf("quiet passes wrote retention or failure lines: %s", logs)
	}
}

func TestStreamRelayQuietPassRemovedFrames(t *testing.T) {
	logs, removed := runStreamRelayQuietPasses(t, 2, 3)
	var wantLines int
	var wantRemoved int64
	for _, count := range removed {
		if count > 0 {
			wantLines++
			wantRemoved += count
		}
	}
	if wantLines != 1 || wantRemoved != 2 {
		t.Fatalf("retention removed %d frames in %d passes, want 2 in 1", wantRemoved, wantLines)
	}
	var gotLines int
	var gotRemoved int64
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var record struct {
			Level   string `json:"level"`
			Message string `json:"msg"`
			Removed int64  `json:"removed_frames"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("relay log record: %q: %v", line, err)
		}
		if record.Level == "ERROR" || record.Level == "WARN" {
			t.Fatalf("successful retention wrote a failure line: %s", line)
		}
		if record.Message != "stream relay retention pass" {
			continue
		}
		if record.Level != "INFO" || record.Removed <= 0 {
			t.Fatalf("retention line must report a positive count at info: %s", line)
		}
		gotLines++
		gotRemoved += record.Removed
	}
	if gotLines != wantLines || gotRemoved != wantRemoved {
		t.Fatalf("logged %d removals in %d lines, want %d in %d: %s", gotRemoved, gotLines, wantRemoved, wantLines, logs)
	}
}

func runStreamRelayQuietPasses(t *testing.T, oldFrames, passes int) (string, []int64) {
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
	const stream = "QUIET_RELAY"
	if _, err = js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{"market.>", "private.portfolio.*"}, Storage: nats.FileStorage, MaxAge: 72 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(stream); err != nil {
			t.Error(err)
		}
	}()
	document := streamRelayDocument{Routes: []delivery.StreamRoute{{Stream: stream, Subjects: []string{"market.>", "private.portfolio.*"}}}, Epoch: db.Epoch, Region: db.Region, Batch: 50, MaxAttempts: 3, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, IdleInterval: 100 * time.Millisecond, MaxLag: time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 50}
	relay, err := newStreamRelay(ctx, runtimePool, nc, "quiet_relay", document)
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
	if _, err = admin.Exec(ctx, "ALTER TABLE api_stream.stream_delivery_outbox ALTER COLUMN created_at SET DEFAULT (clock_timestamp()-interval '96 hours')"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= oldFrames+1; i++ {
		if i == oldFrames+1 {
			if _, err = admin.Exec(ctx, "ALTER TABLE api_stream.stream_delivery_outbox ALTER COLUMN created_at SET DEFAULT clock_timestamp()"); err != nil {
				t.Fatal(err)
			}
		}
		frame.EventID, frame.Sequence, frame.Revision = fmt.Sprintf("quiet_price_%d", i), fmt.Sprint(i), fmt.Sprint(i)
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
	}
	if err = relay.relay.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var published int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM api_stream.stream_delivery_publications WHERE state='published'").Scan(&published); err != nil || published != oldFrames+1 {
		t.Fatalf("published %d frames, want %d: %v", published, oldFrames+1, err)
	}
	if err = relay.Ready(ctx); err != nil {
		t.Fatalf("drained relay readiness: %v", err)
	}
	var logs bytes.Buffer
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		relay.Run(runCtx, slog.New(slog.NewJSONHandler(&logs, nil)))
		close(done)
	}()
	defer func() { stop(); <-done }()
	removed := make([]int64, 0, passes)
	for len(removed) < passes {
		var epoch, count int64
		var released bool
		if err = admin.QueryRow(ctx, "SELECT retention_epoch,retention_removed,COALESCE(retention_expires_at<=clock_timestamp(),false) FROM api_stream.stream_delivery_offsets ORDER BY partition_id LIMIT 1").Scan(&epoch, &count, &released); err != nil {
			t.Fatal(err)
		}
		if released && epoch > int64(len(removed)) {
			if epoch != int64(len(removed)+1) {
				t.Fatalf("retention epoch %d skipped an observed pass after %d", epoch, len(removed))
			}
			removed = append(removed, count)
			if len(removed) < passes {
				if _, err = admin.Exec(ctx, "UPDATE api_stream.stream_delivery_offsets SET retention_next_at=clock_timestamp() WHERE retention_epoch=$1", epoch); err != nil {
					t.Fatal(err)
				}
			}
		}
		if len(removed) < passes {
			select {
			case <-ctx.Done():
				t.Fatalf("observed %d of %d retention passes: %v", len(removed), passes, ctx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	stop()
	<-done
	var retained int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM api_stream.stream_delivery_outbox").Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("retained %d frames, want the recent frame: %v", retained, err)
	}
	if err = relay.Ready(ctx); err != nil {
		t.Fatalf("relay readiness after retention: %v", err)
	}
	return logs.String(), removed
}
