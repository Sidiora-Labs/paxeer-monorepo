package reconcile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/reconcile"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReconcilerRequiresRefreshTransportAndPropagatesReadCancellation(t *testing.T) {
	st := seed(t)
	ctx := context.Background()
	if n, err := reconcile.New(st, nil, discardLogger(), 0).RunOnce(ctx); err == nil || n != 0 {
		t.Fatal(n, err)
	}
	cfg := st.Pool().Config()
	tracer := new(internaltest.QueryCancellation)
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	worker := reconcile.New(store.New(pool), internaltest.NewRedis(t), discardLogger(), 0)
	for _, target := range []int{1, 2} {
		tracer.Start(target)
		n, err := worker.RunOnce(ctx)
		queries := tracer.Stop()
		if n != 0 || !errors.Is(err, context.Canceled) || len(queries) != target {
			t.Fatal(n, queries, err)
		}
	}
	if _, err := reconcile.NewInventoryRebuilder(nil); err == nil {
		t.Fatal("missing inventory accepted")
	}
	rebuild, err := reconcile.NewInventoryRebuilder(&l16.Service{})
	if err != nil || rebuild == nil {
		t.Fatal(rebuild, err)
	}
	if _, err = rebuild.RebuildPartition(ctx, "invalid", "invalid", 0); !errors.Is(err, l16.ErrBaselineUnavailable) {
		t.Fatal(err)
	}
}
