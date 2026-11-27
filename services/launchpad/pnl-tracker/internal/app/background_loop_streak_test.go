package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type backgroundLoopStreakRecord struct {
	Level        string `json:"level"`
	Message      string `json:"msg"`
	Job          string `json:"job"`
	Err          string `json:"err"`
	FailedPasses int    `json:"failed_passes"`
}

func assertBackgroundLoopStreakLogs(t *testing.T, output *bytes.Buffer, want ...backgroundLoopStreakRecord) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	var got []backgroundLoopStreakRecord
	for {
		var record backgroundLoopStreakRecord
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, record)
	}
	if len(got) != len(want) {
		t.Fatalf("log records = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("log record %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBackgroundLoopStreakTransientFailure(t *testing.T) {
	var output bytes.Buffer
	a := &App{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	a.loop(ctx, time.Millisecond, "reconcile", func(context.Context) error {
		calls++
		if calls == 2 {
			return errors.New("dependency unavailable")
		}
		if calls == 5 {
			cancel()
		}
		return nil
	})
	if calls != 5 {
		t.Fatalf("passes = %d, want 5", calls)
	}
	assertBackgroundLoopStreakLogs(t, &output,
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 1},
		backgroundLoopStreakRecord{"INFO", "background job recovered", "reconcile", "", 1},
	)
}

func TestBackgroundLoopStreakErrorThrottle(t *testing.T) {
	var output bytes.Buffer
	now := time.Time{}
	a := &App{
		logger:           slog.New(slog.NewJSONHandler(&output, nil)),
		backgroundJobNow: func() time.Time { return now },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	want := []backgroundLoopStreakRecord{
		{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 1},
		{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 2},
		{"ERROR", "background job failed", "reconcile", "dependency unavailable", 3},
	}
	calls := 0
	a.loop(ctx, time.Millisecond, "reconcile", func(context.Context) error {
		calls++
		switch calls {
		case 11:
			assertBackgroundLoopStreakLogs(t, &output, want...)
			now = now.Add(30*time.Second - time.Nanosecond)
		case 12:
			assertBackgroundLoopStreakLogs(t, &output, want...)
			now = now.Add(time.Nanosecond)
		case 13:
			want = append(want, backgroundLoopStreakRecord{"ERROR", "background job failed", "reconcile", "dependency unavailable", 12})
			assertBackgroundLoopStreakLogs(t, &output, want...)
			cancel()
		}
		return errors.New("dependency unavailable")
	})
	if calls != 13 {
		t.Fatalf("passes = %d, want 13", calls)
	}
	assertBackgroundLoopStreakLogs(t, &output, want...)
}

func TestBackgroundLoopStreakRecoveryResetsFailures(t *testing.T) {
	var output bytes.Buffer
	a := &App{
		logger:           slog.New(slog.NewJSONHandler(&output, nil)),
		backgroundJobNow: func() time.Time { return time.Time{} },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	a.loop(ctx, time.Millisecond, "reconcile", func(context.Context) error {
		calls++
		if calls == 6 || calls == 7 {
			return nil
		}
		if calls == 10 {
			cancel()
		}
		return errors.New("dependency unavailable")
	})
	if calls != 10 {
		t.Fatalf("passes = %d, want 10", calls)
	}
	assertBackgroundLoopStreakLogs(t, &output,
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 1},
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 2},
		backgroundLoopStreakRecord{"ERROR", "background job failed", "reconcile", "dependency unavailable", 3},
		backgroundLoopStreakRecord{"INFO", "background job recovered", "reconcile", "", 5},
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 1},
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 2},
		backgroundLoopStreakRecord{"ERROR", "background job failed", "reconcile", "dependency unavailable", 3},
	)
}

func TestBackgroundLoopStreakCancellation(t *testing.T) {
	var output bytes.Buffer
	now := time.Time{}
	a := &App{
		logger:           slog.New(slog.NewJSONHandler(&output, nil)),
		backgroundJobNow: func() time.Time { return now },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	a.loop(ctx, time.Millisecond, "reconcile", func(context.Context) error {
		calls++
		if calls == 4 {
			now = now.Add(30 * time.Second)
			cancel()
		}
		return errors.New("dependency unavailable")
	})
	if calls != 4 {
		t.Fatalf("passes = %d, want 4", calls)
	}
	assertBackgroundLoopStreakLogs(t, &output,
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 1},
		backgroundLoopStreakRecord{"WARN", "background job pass failed", "reconcile", "dependency unavailable", 2},
		backgroundLoopStreakRecord{"ERROR", "background job failed", "reconcile", "dependency unavailable", 3},
		backgroundLoopStreakRecord{"ERROR", "background job failed", "reconcile", "dependency unavailable", 4},
	)
}
