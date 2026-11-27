package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

func TestOutboxBoundedReaderReplay(t *testing.T) {
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
	if err = js.DeleteStream("BOUNDED_REPLAY"); err != nil && !errors.Is(err, nats.ErrStreamNotFound) {
		t.Fatal(err)
	}
	if _, err = js.AddStream(&nats.StreamConfig{Name: "BOUNDED_REPLAY", Subjects: []string{"market.>", "private.portfolio.*"}, Storage: nats.FileStorage, MaxAge: 72 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream("BOUNDED_REPLAY"); err != nil {
			t.Error(err)
		}
	}()
	document := streamRelayDocument{Routes: []delivery.StreamRoute{{Stream: "BOUNDED_REPLAY", Subjects: []string{"market.>", "private.portfolio.*"}}}, Epoch: db.Epoch, Region: db.Region, Batch: 50, MaxAttempts: 3, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, IdleInterval: 100 * time.Millisecond, MaxLag: time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 50}
	relay, err := newStreamRelay(ctx, runtimePool, nc, "bounded_relay", document)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := broker.NewPostgresStream(admin, "api_stream", []byte(strings.Repeat("k", 32)), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	var frame broker.Frame
	for _, f := range committedFrames(t) {
		if strings.HasPrefix(f.topic, "market.price.") {
			if err = json.Unmarshal(f.wire, &frame); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if frame.Topic == "" {
		t.Fatal("committed price frame required")
	}
	topics := []string{frame.Topic}
	cursors := map[int]string{}
	wires := map[int][]byte{}
	if _, err = admin.Exec(ctx, "ALTER TABLE api_stream.stream_delivery_outbox ALTER COLUMN created_at SET DEFAULT (clock_timestamp()-interval '96 hours')"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 6; i++ {
		if i == 3 {
			if _, err = admin.Exec(ctx, "ALTER TABLE api_stream.stream_delivery_outbox ALTER COLUMN created_at SET DEFAULT clock_timestamp()"); err != nil {
				t.Fatal(err)
			}
		}
		frame.EventID, frame.Sequence, frame.Revision = "bounded_price_"+strconv.Itoa(i), strconv.Itoa(i), strconv.Itoa(i)
		wire, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		wires[i] = wire
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
		if i == 1 || i == 3 {
			raw, err := reader.PublicSnapshot(ctx, topics)
			if err != nil {
				t.Fatal(err)
			}
			var snapshot struct {
				Cursor string `json:"resumeCursor"`
			}
			if err = json.Unmarshal(raw, &snapshot); err != nil || snapshot.Cursor == "" {
				t.Fatalf("snapshot cursor: %s %v", raw, err)
			}
			cursors[i] = snapshot.Cursor
		}
	}
	var directDelete bool
	if err = runtimePool.QueryRow(ctx, "SELECT has_table_privilege(current_user,'api_stream.stream_delivery_outbox','DELETE')").Scan(&directDelete); err != nil || directDelete {
		t.Fatalf("retention authority: direct delete=%t %v", directDelete, err)
	}
	var logs bytes.Buffer
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		relay.Run(runCtx, slog.New(slog.NewTextHandler(&logs, nil)))
		close(done)
	}()
	defer func() { stop(); <-done }()
	for {
		var first int64
		var released bool
		err = admin.QueryRow(ctx, "SELECT (SELECT min(sequence) FROM api_stream.stream_delivery_outbox),COALESCE((SELECT retention_expires_at<=clock_timestamp() FROM api_stream.stream_delivery_offsets ORDER BY partition_id LIMIT 1),false)").Scan(&first, &released)
		if err != nil {
			t.Fatal(err)
		}
		if first == 3 && released {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("relay retention did not keep the reader history")
		case <-time.After(20 * time.Millisecond):
		}
	}
	stop()
	<-done
	if !strings.Contains(logs.String(), "removed_frames=2") {
		t.Fatalf("retention count missing: %s", logs.String())
	}
	if _, err = reader.PublicResume(ctx, topics, cursors[1]); !errors.Is(err, broker.ErrExpired) || err.Error() != "RESUME_EXPIRED" {
		t.Fatalf("cursor below retained history: %v", err)
	}
	replay, err := reader.PublicResume(ctx, topics, cursors[3])
	if err != nil {
		t.Fatal(err)
	}
	batch, err := replay.Batch(ctx)
	if err != nil || len(batch) != 3 {
		t.Fatalf("retained replay: %d %v", len(batch), err)
	}
	for i, wire := range batch {
		if !bytes.Equal(wire, wires[i+4]) {
			t.Fatalf("replayed frame %d differs from the producer", i+4)
		}
		message, err := js.GetMsg("BOUNDED_REPLAY", uint64(i+4))
		if err != nil || !bytes.Equal(message.Data, wire) {
			t.Fatalf("published frame %d: %v", i+4, err)
		}
	}
	restarted, err := newStreamRelay(ctx, runtimePool, nc, "bounded_restart", document)
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := restarted.relay.Prune(ctx); err != nil || removed != 0 {
		t.Fatalf("retention cadence across restart: %d %v", removed, err)
	}
	if err = restarted.Ready(ctx); err != nil {
		t.Fatalf("retained stream readiness: %v", err)
	}
}

func TestOutboxBoundedRetentionWindow(t *testing.T) {
	base := streamRelayDocument{Routes: []delivery.StreamRoute{{Stream: "LIVE_STREAM", Subjects: []string{"market.>", "private.portfolio.*"}}}, Epoch: 1, Region: "fixture-loopback", Batch: 50, MaxAttempts: 3, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, IdleInterval: 100 * time.Millisecond, MaxLag: time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 50}
	if delivery.DefaultStreamRetention != 72*time.Hour {
		t.Fatal("default retention must preserve the 72h reader replay window")
	}
	for _, window := range []time.Duration{-time.Second, time.Nanosecond, 24 * time.Hour, delivery.StreamReplayWindow - time.Nanosecond} {
		doc := base
		doc.RetentionWindow = window
		if _, err := newStreamRelay(context.Background(), nil, nil, "relay", doc); err == nil || !strings.Contains(err.Error(), "retention window") {
			t.Fatalf("startup admitted retention %s: %v", window, err)
		}
		if _, err := delivery.NewStreamOutboxRelay(nil, nil, delivery.StreamOutboxConfig{RetentionWindow: window}, nil, nil); err == nil || !strings.Contains(err.Error(), "retention window") {
			t.Fatalf("shared relay admitted retention %s: %v", window, err)
		}
	}
	for _, window := range []time.Duration{0, 72 * time.Hour, 96 * time.Hour} {
		doc := base
		doc.RetentionWindow = window
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "relay.json")
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		loaded, err := readStreamRelayDocument(&config.Config{StreamRelayFile: path, StreamRelaySHA256: fmt.Sprintf("%x", sha256.Sum256(raw))})
		if err != nil || loaded == nil || loaded.RetentionWindow != window {
			t.Fatalf("bound retention window %s: %+v %v", window, loaded, err)
		}
	}
}
