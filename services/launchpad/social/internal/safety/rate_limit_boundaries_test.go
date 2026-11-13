package safety

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestAbuseLimiterEnforcesActionBudgetsAndSlowMode(t *testing.T) {
	opt, err := redis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opt)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	for _, tc := range []struct {
		kind  string
		limit int
	}{
		{"pool", 10},
		{"dm", 20},
		{"dm_new", 5},
		{"comment", 10},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			limiter := &AbuseLimiter{Redis: client, Namespace: "community-" + uuid.NewString()}
			for i := 0; i < tc.limit; i++ {
				if err := limiter.Take(ctx, walletA, tc.kind, "pool-a", 0); err != nil {
					t.Fatalf("admission %d: %v", i, err)
				}
			}
			err := limiter.Take(ctx, walletA, tc.kind, "pool-a", 0)
			var limited RateLimited
			if !errors.As(err, &limited) || limited.RetryAfterSeconds < 1 {
				t.Fatalf("exhausted action budget = %v", err)
			}
			if err := limiter.Take(ctx, walletB, tc.kind, "pool-a", 0); err != nil {
				t.Fatalf("independent wallet budget = %v", err)
			}
		})
	}
	limiter := &AbuseLimiter{Redis: client, Namespace: "community-" + uuid.NewString()}
	if err := limiter.Take(ctx, walletA, "comment", "pool-a", 30); err != nil {
		t.Fatal(err)
	}
	var limited RateLimited
	if err := limiter.Take(ctx, walletA, "comment", "pool-a", 30); !errors.As(err, &limited) || limited.RetryAfterSeconds < 1 || limited.RetryAfterSeconds > 30 {
		t.Fatalf("slow mode admission = %v", err)
	}
	if err := limiter.Take(ctx, walletA, "comment", "pool-b", 30); err != nil {
		t.Fatalf("independent resource slow mode = %v", err)
	}
	if err := limiter.Take(ctx, walletA, "unknown", "pool-a", 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown action = %v", err)
	}
	if err := limiter.Take(ctx, "invalid", "comment", "pool-a", 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid wallet = %v", err)
	}
	closed := redis.NewClient(opt)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	limiter.Redis = closed
	if err := limiter.Take(ctx, walletB, "comment", "pool-a", 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable rate authority = %v", err)
	}
	if status, retry := RateLimitStatus(ErrUnavailable); status != 503 || retry != "" {
		t.Fatalf("unavailable rate status = %d/%q", status, retry)
	}
}

func TestAbuseLimiterBoundsLocalDenialsAndReclaimsExpiredWallets(t *testing.T) {
	limiter := &AbuseLimiter{Namespace: "community-rate"}
	expires := time.Now().Add(time.Hour)
	limiter.local = make(map[string]time.Time, 10000)
	for i := 0; i < 10000; i++ {
		limiter.local[fmt.Sprintf("0x%040x", i+100)] = expires
	}
	if err := limiter.Take(context.Background(), walletA, "comment", "pool-a", 0); !errors.Is(err, ErrUnavailable) || len(limiter.local) != 10000 {
		t.Fatalf("full local denial ledger = %d: %v", len(limiter.local), err)
	}
	for wallet := range limiter.local {
		limiter.local[wallet] = time.Now().Add(-time.Minute)
	}
	if err := limiter.Take(context.Background(), walletA, "comment", "pool-a", 0); !errors.Is(err, ErrUnavailable) || len(limiter.local) != 1 {
		t.Fatalf("reclaimed local denial ledger = %d: %v", len(limiter.local), err)
	}
	var limited RateLimited
	if err := limiter.Take(context.Background(), walletA, "comment", "pool-a", 0); !errors.As(err, &limited) || limited.RetryAfterSeconds < 1 || limited.RetryAfterSeconds > 60 {
		t.Fatalf("repeated local denial = %v", err)
	}
}
