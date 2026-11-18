package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/google/uuid"
)

func publicationBoundProduce(t *testing.T, f *publicationRecoveryFixture) {
	t.Helper()
	for _, newOnly := range []bool{false, true} {
		if _, err := f.app.native.coordinator.Refresh(context.Background(), uuid.NewString(), f.cutoff, newOnly); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplicationPublicationReadinessRecoversOnBoundedStartup(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	ctx := context.Background()
	publicationBoundProduce(t, f)
	runtime := f.app.native
	if err := runtime.publication.SetRetryInterval(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := runtime.ready(ctx); err != nil {
		t.Fatalf("initial readiness: %v", err)
	}
	exec := func(sql string) {
		t.Helper()
		if _, err := f.app.pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE ranking.publication_outbox SET created_at=clock_timestamp()-interval '61 seconds' WHERE category='breakout'`)
	if err := runtime.ready(ctx); err == nil {
		t.Fatal("oldest pending row did not refuse readiness")
	}
	exec(`UPDATE ranking.publication_outbox SET created_at=clock_timestamp()`)
	if err := runtime.ready(ctx); err != nil {
		t.Fatalf("pending age recovery: %v", err)
	}
	if _, err := f.app.pool.Exec(ctx, `UPDATE ranking.publication_outbox SET attempts=$1 WHERE category IN ('breakout','gainers')`, runtime.binding.MaxAttempts); err != nil {
		t.Fatal(err)
	}
	if err := runtime.ready(ctx); err == nil {
		t.Fatal("bound rows did not refuse readiness")
	}
	var output bytes.Buffer
	runtime.logger = slog.New(slog.NewJSONHandler(&output, nil))
	runtime.workers.Add(1)
	go func() { defer runtime.workers.Done(); runtime.runPublication() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var pending int
		if err := f.app.pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("startup did not publish bounded rows immediately")
		}
		time.Sleep(20 * time.Millisecond)
	}
	runtime.stop()
	runtime.workers.Wait()
	if f.app.native != runtime {
		t.Fatal("runtime changed during recovery")
	}
	if err := runtime.ready(ctx); err != nil {
		t.Fatalf("readiness did not recover without restart: %v", err)
	}
	var invalid, frames int
	if err := f.app.pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE attempts>$1 OR (category IN ('breakout','gainers') AND attempts<>$1)`, runtime.binding.MaxAttempts).Scan(&invalid); err != nil || invalid != 0 {
		t.Fatalf("bounded startup attempts=%d: %v", invalid, err)
	}
	if err := f.app.pool.QueryRow(ctx, `SELECT count(*) FROM api_stream.stream_delivery_outbox`).Scan(&frames); err != nil || frames != len(ranker.Categories) {
		t.Fatalf("startup frames=%d: %v", frames, err)
	}
	if strings.Count(output.String(), `"msg":"ranking publication recovered"`) != 1 {
		t.Fatalf("recovery transition: %s", output.String())
	}
}

func TestApplicationPublicationBoundLogsOnlyTransitions(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	ctx := context.Background()
	publicationBoundProduce(t, f)
	runtime := f.app.native
	pace := 30 * time.Second
	if err := runtime.publication.SetRetryInterval(pace); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	runtime.logger = slog.New(slog.NewJSONHandler(&output, nil))
	if _, err := f.app.pool.Exec(ctx, `ALTER TABLE api_stream.stream_delivery_outbox ADD CONSTRAINT publication_available CHECK(convert_from(frame,'UTF8')::jsonb->>'topic'<>'market.rankings.breakout')`); err != nil {
		t.Fatal(err)
	}
	var boundStarted, boundAt time.Time
	for i := 0; i < runtime.binding.MaxAttempts; i++ {
		boundStarted = time.Now()
		runtime.publishRound()
		boundAt = time.Now()
	}
	if strings.Count(output.String(), `"msg":"ranking publication attempt bound reached"`) != 1 {
		t.Fatalf("bound transition: %s", output.String())
	}
	runtime.publishRound()
	runtime.publishRound()
	if time.Since(boundStarted) >= pace {
		t.Fatal("initial rounds exceeded the retry interval")
	}
	var pending int
	if err := f.app.pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NULL AND category='breakout'`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("bounded retry ran early: pending=%d %v", pending, err)
	}
	before := output.String()
	time.Sleep(time.Until(boundAt.Add(pace)))
	repeatedStarted := time.Now()
	runtime.publishRound()
	if output.String() != before {
		t.Fatalf("repeated bounded failure wrote another line: %s", output.String())
	}
	repeatedAt := time.Now()
	if _, err := f.app.pool.Exec(ctx, `ALTER TABLE api_stream.stream_delivery_outbox DROP CONSTRAINT publication_available`); err != nil {
		t.Fatal(err)
	}
	runtime.publishRound()
	if time.Since(repeatedStarted) >= pace {
		t.Fatal("availability update and early round exceeded the retry interval")
	}
	if err := f.app.pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("bounded retry ran early: pending=%d %v", pending, err)
	}
	time.Sleep(time.Until(repeatedAt.Add(pace)))
	runtime.publishRound()
	runtime.publishRound()
	if err := f.app.pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("publication recovery pending=%d: %v", pending, err)
	}
	if strings.Count(output.String(), `"msg":"ranking publication recovered"`) != 1 {
		t.Fatalf("recovery transition: %s", output.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["category"] != "breakout" || record["errorClass"] != "database" || record["error"] != nil {
			t.Fatalf("publication log fields: %s", line)
		}
		for key := range record {
			if key != "time" && key != "level" && key != "msg" && key != "category" && key != "errorClass" && key != "code" {
				t.Fatalf("unexpected publication log field %s", key)
			}
		}
	}
}
