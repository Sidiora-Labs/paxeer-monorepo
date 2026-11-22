package internaltest

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	rankingStreamTimeout         = 3 * time.Minute
	rankingStreamDeadlineReserve = 30 * time.Second
)

func rankingStreamDeadline(now, binaryDeadline time.Time, hasDeadline bool) time.Time {
	deadline := now.Add(rankingStreamTimeout)
	if end := binaryDeadline.Add(-rankingStreamDeadlineReserve); hasDeadline && end.Before(deadline) {
		deadline = end
	}
	return deadline
}

func RankingBroker(t *testing.T) string {
	t.Helper()
	brokerURL := os.Getenv("RANKING_TEST_NATS_URL")
	admin := os.Getenv(PostgresDSNEnv)
	if brokerURL == "" || admin == "" {
		t.Fatal("RANKING_TEST_NATS_URL and RANKING_TEST_POSTGRES_DSN are required")
	}
	end, ok := t.Deadline()
	deadline := rankingStreamDeadline(time.Now(), end, ok)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	lock, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := lock.Close(ctx); err != nil {
			t.Errorf("release ranking broker: %v", err)
		}
	})
	digest := sha256.Sum256([]byte("ranking-test:market.rankings.*:" + brokerURL))
	bits := binary.BigEndian.Uint64(digest[:8])
	lockID := int64(bits & math.MaxInt64)
	if bits > math.MaxInt64 {
		lockID += math.MinInt64
	}
	if _, err := lock.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		t.Fatalf("acquire ranking broker within deadline: %v", err)
	}
	conn, err := nats.Connect(brokerURL, nats.NoReconnect(), nats.Timeout(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatal(err)
	}
	initial := map[string]bool{}
	clean := func(ctx context.Context, reset bool) error {
		streams := js.ListStreams(ctx)
		var remove []string
		for info := range streams.Info() {
			ranking := false
			for _, subject := range info.Config.Subjects {
				if rankingSubject(subject) {
					ranking = true
				}
			}
			if ranking || (!reset && !initial[info.Config.Name]) {
				remove = append(remove, info.Config.Name)
			} else if reset {
				initial[info.Config.Name] = true
			}
		}
		if err := streams.Err(); err != nil {
			return err
		}
		for _, name := range remove {
			if err := js.DeleteStream(ctx, name); err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
				return err
			}
			for {
				_, err := js.Stream(ctx, name)
				if errors.Is(err, jetstream.ErrStreamNotFound) {
					break
				}
				if err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(20 * time.Millisecond):
				}
			}
		}
		return nil
	}
	if err := clean(ctx, true); err != nil {
		t.Fatalf("reset ranking broker streams: %v", err)
	}
	t.Cleanup(func() {
		end, ok := t.Deadline()
		ctx, cancel := context.WithDeadline(context.Background(), rankingStreamDeadline(time.Now(), end, ok))
		defer cancel()
		if err := clean(ctx, false); err != nil {
			t.Errorf("clean ranking broker streams: %v", err)
		}
	})
	return brokerURL
}

func rankingSubject(subject string) bool {
	parts := strings.Split(subject, ".")
	prefix := []string{"market", "rankings"}
	for i, part := range parts {
		if i >= 3 {
			return false
		}
		if part == ">" {
			return true
		}
		if part != "*" && i < len(prefix) && part != prefix[i] {
			return false
		}
	}
	return len(parts) == 3
}
