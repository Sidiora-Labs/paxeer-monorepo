package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
)

func TestLeaseStandbyPostgresTakeover(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, pool := canonicaltest.New(t)
	st := New(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'standby',100,100)`); err != nil {
		t.Fatal(err)
	}
	holder, err := st.AcquireLease(ctx, 125, "standby", "holder", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.AcquireLease(ctx, 125, "standby", "standby", 30*time.Second); !errors.Is(err, ErrFenced) {
		t.Fatalf("standby acquisition while held = %v", err)
	}
	if err = st.ReleaseLease(ctx, 125, "standby", "holder", holder); err != nil {
		t.Fatal(err)
	}
	standby, err := st.AcquireLease(ctx, 125, "standby", "standby", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if standby <= holder {
		t.Fatalf("takeover fence = %d, holder fence = %d", standby, holder)
	}
	height, err := st.GetCanonicalWatermark(ctx, 125, "standby")
	if err != nil || height != 100 {
		t.Fatalf("durable watermark = %d, err = %v", height, err)
	}
	if err = st.ReleaseLease(ctx, 125, "standby", "standby", standby); err != nil {
		t.Fatal(err)
	}
}
