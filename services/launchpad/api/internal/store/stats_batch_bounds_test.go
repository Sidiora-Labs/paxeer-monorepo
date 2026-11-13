package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestStatsBatchValidatesCachedAndDatabaseBounds(t *testing.T) {
	st, pool, redis := newStore(t)
	ctx := context.Background()
	if values, err := st.StatsBatch(ctx, nil); err != nil || len(values) != 0 {
		t.Fatalf("empty batch: %v %v", values, err)
	}
	if _, err := st.StatsBatch(ctx, make([]string, 101)); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("oversized batch: %v", err)
	}
	for _, body := range []string{"", "{", strings.Repeat("x", store.PublicJSONMaxBytes+1)} {
		if err := redis.Set(ctx, "stats:pool", body, 0).Err(); err != nil {
			t.Fatal(err)
		}
		if _, err := st.StatsBatch(ctx, []string{"pool"}); !errors.Is(err, store.ErrDataIntegrity) {
			t.Fatalf("invalid cache length %d: %v", len(body), err)
		}
	}
	if err := redis.Set(ctx, "stats:pool", `{"price":"1"}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if values, err := st.StatsBatch(ctx, []string{"pool"}); err != nil || string(values["pool"]) != `{"price":"1"}` {
		t.Fatalf("cached batch: %v %v", values, err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE stats.pool_stats`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.StatsBatch(ctx, []string{"missing"}); err == nil {
		t.Fatal("missing stats source accepted")
	}
}
