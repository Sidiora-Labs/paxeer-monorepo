package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"
)

// stats-tracker-lifecycle is proved through the production recording queue,
// reporting loop and owned Stop handle, including replacement and reopening.
type repairM1294Log struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (l *repairM1294Log) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.data.Write(p)
}

func repairM1294Capture(t *testing.T) *repairM1294Log {
	t.Helper()
	InitRPCTracker(context.Background(), 0)
	InitWSTracker(context.Background(), 0)
	previous := logger
	output := &repairM1294Log{}
	logger = slog.New(slog.NewJSONHandler(output, nil))
	t.Cleanup(func() {
		InitRPCTracker(context.Background(), 0)
		InitWSTracker(context.Background(), 0)
		logger = previous
	})
	return output
}

func (l *repairM1294Log) methodTotals(t *testing.T, method string) (count, success, fails int) {
	t.Helper()
	l.mu.Lock()
	data := append([]byte(nil), l.data.Bytes()...)
	l.mu.Unlock()
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var record struct {
			Message string `json:"msg"`
			Method  string `json:"method"`
			Count   int    `json:"count"`
			Success int    `json:"success"`
			Fails   int    `json:"fails"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode production stats report: %v", err)
		}
		if record.Message == "method stats" && record.Method == method {
			count += record.Count
			success += record.Success
			fails += record.Fails
		}
	}
	return
}

func repairM1294AssertTotals(t *testing.T, output *repairM1294Log, method string, count, success, fails int) {
	t.Helper()
	gotCount, gotSuccess, gotFails := output.methodTotals(t, method)
	if gotCount != count || gotSuccess != success || gotFails != fails {
		t.Fatalf("%s reports count/success/fails = %d/%d/%d, want %d/%d/%d", method, gotCount, gotSuccess, gotFails, count, success, fails)
	}
}

func repairM1294Wait(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tracker lifecycle did not join")
	}
}

func repairM1294AssertStopped(t *testing.T, handle *tracker) {
	t.Helper()
	if handle == nil || handle.ctx.Err() != context.Canceled {
		t.Fatal("previous tracker context was not canceled")
	}
	repairM1294Wait(t, &handle.wg)
	if pending := len(handle.ch); pending != 0 {
		t.Fatalf("stopped tracker retained %d unreported events", pending)
	}
}

func TestRepairM_1294_ReplacementAndRestart(t *testing.T) {
	for _, transport := range []struct {
		name string
		init func(context.Context, time.Duration) *tracker
	}{
		{"http", InitRPCTracker},
		{"websocket", InitWSTracker},
	} {
		t.Run(transport.name, func(t *testing.T) {
			output := repairM1294Capture(t)
			ctx := context.Background()
			first := transport.init(ctx, 24*time.Hour)
			for i := 0; i < 64; i++ {
				RecordAPIInvocation("before_restart", transport.name, time.Now().Add(-time.Millisecond), i%2 == 0)
			}
			replacement := transport.init(ctx, 12*time.Hour)
			if replacement == nil || replacement == first || replacement.interval != 12*time.Hour {
				t.Fatal("replacement did not install the configured tracker")
			}
			repairM1294AssertStopped(t, first)
			repairM1294AssertTotals(t, output, "before_restart", 64, 32, 32)

			// A delayed Stop from the old server must leave its successor active.
			first.Stop()
			first.TrackMessage("stale_handle", transport.name, time.Now(), true)
			RecordAPIInvocation("replacement", transport.name, time.Now(), true)
			replacement.Stop()
			repairM1294AssertStopped(t, replacement)
			repairM1294AssertTotals(t, output, "replacement", 1, 1, 0)
			repairM1294AssertTotals(t, output, "stale_handle", 0, 0, 0)
			RecordAPIInvocation("after_stop", transport.name, time.Now(), true)

			reopened := transport.init(ctx, 6*time.Hour)
			if reopened == nil || reopened == replacement || reopened.ctx.Err() != nil || reopened.interval != 6*time.Hour {
				t.Fatal("reopening did not restore an active tracker with the new interval")
			}
			RecordAPIInvocation("after_restart", transport.name, time.Now(), false)
			reopened.Stop()
			reopened.Stop()
			repairM1294AssertStopped(t, reopened)
			repairM1294AssertTotals(t, output, "after_restart", 1, 0, 1)
			repairM1294AssertTotals(t, output, "before_restart", 64, 32, 32)
			repairM1294AssertTotals(t, output, "after_stop", 0, 0, 0)
		})
	}
}

func TestRepairM_1294_ZeroIntervalDisables(t *testing.T) {
	for _, transport := range []struct {
		name      string
		other     string
		init      func(context.Context, time.Duration) *tracker
		initOther func(context.Context, time.Duration) *tracker
	}{
		{"http", "websocket", InitRPCTracker, InitWSTracker},
		{"websocket", "http", InitWSTracker, InitRPCTracker},
	} {
		t.Run(transport.name, func(t *testing.T) {
			output := repairM1294Capture(t)
			ctx := context.Background()
			active := transport.init(ctx, time.Hour)
			other := transport.initOther(ctx, time.Hour)
			RecordAPIInvocation("before_disable", transport.name, time.Now(), true)
			disabled := transport.init(ctx, 0)
			if disabled != nil {
				t.Fatal("zero interval retained a tracker")
			}
			disabled.Stop()
			repairM1294AssertStopped(t, active)
			repairM1294AssertTotals(t, output, "before_disable", 1, 1, 0)
			RecordAPIInvocation("disabled", transport.name, time.Now(), true)
			active.TrackMessage("disabled_handle", transport.name, time.Now(), true)
			if transport.init(ctx, -time.Second) != nil {
				t.Fatal("negative interval re-enabled tracking")
			}
			if other.ctx.Err() != nil {
				t.Fatal("disabling one transport stopped the other")
			}
			RecordAPIInvocation("other_transport", transport.other, time.Now(), true)
			other.Stop()
			repairM1294AssertStopped(t, other)
			repairM1294AssertTotals(t, output, "other_transport", 1, 1, 0)

			reopened := transport.init(ctx, time.Hour)
			RecordAPIInvocation("reenabled", transport.name, time.Now(), true)
			reopened.Stop()
			repairM1294AssertStopped(t, reopened)
			repairM1294AssertTotals(t, output, "reenabled", 1, 1, 0)
			repairM1294AssertTotals(t, output, "disabled", 0, 0, 0)
			repairM1294AssertTotals(t, output, "disabled_handle", 0, 0, 0)
			if len(active.ch) != 0 {
				t.Fatal("disabled tracker continued accepting events")
			}
		})
	}
}

func TestRepairM_1294_ConcurrentRecordingAndReplacement(t *testing.T) {
	output := repairM1294Capture(t)
	ctx := context.Background()
	start := make(chan struct{})
	handles := make(chan *tracker, 128)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for n := 0; n < 500; n++ {
				RecordAPIInvocation("concurrent_http", "http", time.Now(), true)
				RecordAPIInvocation("concurrent_ws", "websocket", time.Now(), false)
				runtime.Gosched()
			}
		}()
	}
	for _, init := range []func(context.Context, time.Duration) *tracker{InitRPCTracker, InitWSTracker, InitRPCTracker, InitWSTracker} {
		workers.Add(1)
		go func(init func(context.Context, time.Duration) *tracker) {
			defer workers.Done()
			<-start
			var previous *tracker
			for n := 0; n < 24; n++ {
				interval := time.Hour
				if n%4 == 0 {
					interval = 0
				}
				handle := init(ctx, interval)
				if handle != nil {
					handles <- handle
				}
				previous.Stop()
				previous = handle
				runtime.Gosched()
			}
			previous.Stop()
		}(init)
	}
	close(start)
	repairM1294Wait(t, &workers)
	InitRPCTracker(ctx, 0)
	InitWSTracker(ctx, 0)
	close(handles)
	for handle := range handles {
		repairM1294AssertStopped(t, handle)
	}

	http := InitRPCTracker(ctx, time.Hour)
	ws := InitWSTracker(ctx, time.Hour)
	RecordAPIInvocation("recovered_http", "http", time.Now(), true)
	RecordAPIInvocation("recovered_ws", "websocket", time.Now(), false)
	http.Stop()
	ws.Stop()
	repairM1294AssertStopped(t, http)
	repairM1294AssertStopped(t, ws)
	repairM1294AssertTotals(t, output, "recovered_http", 1, 1, 0)
	repairM1294AssertTotals(t, output, "recovered_ws", 1, 0, 1)
}

func TestRepairM_1294_ContextCancellation(t *testing.T) {
	output := repairM1294Capture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	http := InitRPCTracker(ctx, time.Hour)
	ws := InitWSTracker(ctx, time.Hour)
	RecordAPIInvocation("canceled_http", "http", time.Now(), true)
	RecordAPIInvocation("canceled_ws", "websocket", time.Now(), false)
	cancel()
	repairM1294Wait(t, &http.wg)
	repairM1294Wait(t, &ws.wg)
	repairM1294AssertStopped(t, http)
	repairM1294AssertStopped(t, ws)
	RecordAPIInvocation("canceled_late", "http", time.Now(), true)
	RecordAPIInvocation("canceled_late", "websocket", time.Now(), true)
	http.Stop()
	ws.Stop()
	repairM1294AssertTotals(t, output, "canceled_http", 1, 1, 0)
	repairM1294AssertTotals(t, output, "canceled_ws", 1, 0, 1)
	repairM1294AssertTotals(t, output, "canceled_late", 0, 0, 0)
}
