package internaltest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestRankingSubjectOverlap(t *testing.T) {
	for subject, want := range map[string]bool{
		"market.rankings.*":              true,
		"market.rankings.trending":       true,
		"market.rankings.>":              true,
		"market.*.*":                     true,
		"market.>":                       true,
		"*.rankings.*":                   true,
		">":                              true,
		"market.stats.*":                 false,
		"market":                         false,
		"":                               false,
		"*":                              false,
		"*.*.*":                          true,
		"*.*.>":                          true,
		"market.*.trending":              true,
		"market.rankings.trending.extra": false,
		"market.rankings":                false,
		"market.rankings.*.extra":        false,
		"social.>":                       false,
	} {
		if got := rankingSubject(subject); got != want {
			t.Errorf("overlap for %s = %v, want %v", subject, got, want)
		}
	}
}

func TestRankingBrokerCleansStreamsOnRelease(t *testing.T) {
	var name string
	t.Run("holder", func(t *testing.T) {
		brokerURL := RankingBroker(t)
		conn, err := nats.Connect(brokerURL)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		js, err := jetstream.New(conn)
		if err != nil {
			t.Fatal(err)
		}
		name = "RANKING_CLEANUP_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{"market.rankings.*"}, Storage: jetstream.MemoryStorage}); err != nil {
			t.Fatal(err)
		}
	})
	conn, err := nats.Connect(os.Getenv("RANKING_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := js.Stream(ctx, name); !errors.Is(err, jetstream.ErrStreamNotFound) {
		t.Fatalf("released stream remains: %v", err)
	}
}

func TestRankingBrokerRecoversAfterHolderExits(t *testing.T) {
	if os.Getenv("RANKING_BROKER_TEST_WORKER") == "1" {
		conn, err := nats.Connect(RankingBroker(t))
		if err != nil {
			t.Fatal(err)
		}
		js, err := jetstream.New(conn)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		name := "RANKING_HOLDER_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{"market.rankings.*"}, Storage: jetstream.MemoryStorage}); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	worker := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRankingBrokerRecoversAfterHolderExits$", "-test.timeout=4m")
	worker.Env = append(os.Environ(), "RANKING_BROKER_TEST_WORKER=1")
	if output, err := worker.CombinedOutput(); err != nil {
		t.Fatalf("broker holder: %v: %s", err, output)
	}
	conn, err := nats.Connect(RankingBroker(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatal(err)
	}
	streams := js.ListStreams(ctx)
	for info := range streams.Info() {
		for _, subject := range info.Config.Subjects {
			if rankingSubject(subject) {
				t.Errorf("ranking subjects remain on stream %s", info.Config.Name)
			}
		}
	}
	if err := streams.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRankingStreamDeadline(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, tc := range []struct {
		name           string
		binaryDeadline time.Time
		hasDeadline    bool
		want           time.Time
	}{
		{
			name: "without binary deadline",
			want: now.Add(3 * time.Minute),
		},
		{
			name:           "distant binary deadline",
			binaryDeadline: now.Add(4 * time.Minute),
			hasDeadline:    true,
			want:           now.Add(3 * time.Minute),
		},
		{
			name:           "near binary deadline",
			binaryDeadline: now.Add(2 * time.Minute),
			hasDeadline:    true,
			want:           now.Add(90 * time.Second),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rankingStreamDeadline(now, tc.binaryDeadline, tc.hasDeadline); !got.Equal(tc.want) {
				t.Fatalf("stream deadline = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRankingStreamRequestRetriesUnansweredRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var requests []context.Context
	var starts []time.Time
	start := time.Now()
	result, err := rankingStreamRequest(ctx, func(request context.Context) (string, error) {
		starts = append(starts, time.Now())
		requests = append(requests, request)
		deadline, ok := request.Deadline()
		if request == ctx || !ok || time.Until(deadline) > 500*time.Millisecond {
			t.Fatal("request must have its own 500 millisecond deadline")
		}
		if len(requests) <= 2 {
			<-request.Done()
			return "", request.Err()
		}
		return "ready", nil
	})
	if err != nil || result != "ready" || len(requests) != 3 {
		t.Fatalf("request result: got %q, %v after %d sends", result, err, len(requests))
	}
	if time.Since(start) >= 2*time.Second || ctx.Err() != nil {
		t.Fatal("request must answer well before the caller deadline")
	}
	for i, request := range requests {
		if request.Err() == nil {
			t.Fatal("request context remains active")
		}
		if i > 0 {
			previousDeadline, _ := requests[i-1].Deadline()
			if request == requests[i-1] || starts[i].Before(previousDeadline.Add(20*time.Millisecond)) {
				t.Fatal("retry must wait 20 milliseconds and use a new request context")
			}
		}
	}
}

func TestRankingStreamRequestReturnsErrorReply(t *testing.T) {
	for _, reply := range []error{jetstream.ErrStreamNotFound, jetstream.ErrStreamNameAlreadyInUse, context.DeadlineExceeded} {
		t.Run(reply.Error(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sends := 0
			var sent context.Context
			start := time.Now()
			result, err := rankingStreamRequest(ctx, func(request context.Context) (string, error) {
				sends++
				sent = request
				return "reply", reply
			})
			if err != reply || result != "reply" || sends != 1 {
				t.Fatalf("error reply: got %q, %v after %d sends; want %v after one send", result, err, sends, reply)
			}
			if time.Since(start) >= 500*time.Millisecond || ctx.Err() != nil || sent.Err() != context.Canceled {
				t.Fatal("error reply must return immediately and cancel its request context")
			}
		})
	}
}

func TestRankingStreamRequestDeadlineRetainsLastError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var lastErr error
	var requests []context.Context
	start := time.Now()
	result, err := rankingStreamRequest(ctx, func(request context.Context) (string, error) {
		requests = append(requests, request)
		deadline, ok := request.Deadline()
		callerDeadline, _ := ctx.Deadline()
		if request == ctx || !ok || deadline.After(callerDeadline) || time.Until(deadline) > 500*time.Millisecond {
			t.Fatal("request deadline must be within 500 milliseconds and the caller deadline")
		}
		<-request.Done()
		lastErr = fmt.Errorf("unanswered request %d: %w", len(requests), request.Err())
		return "", lastErr
	})
	if result != "" || !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("caller deadline: result=%q err=%v caller=%v", result, err, ctx.Err())
	}
	if len(requests) < 2 || lastErr == nil || !errors.Is(err, lastErr) || !strings.Contains(err.Error(), lastErr.Error()) {
		t.Fatalf("last request error: sends=%d err=%v last=%v", len(requests), err, lastErr)
	}
	if time.Since(start) >= 3*time.Second {
		t.Fatal("request must stop at the caller deadline")
	}
	for i, request := range requests {
		if request.Err() == nil || (i > 0 && request == requests[i-1]) {
			t.Fatal("each request must have its own canceled context")
		}
	}
}
