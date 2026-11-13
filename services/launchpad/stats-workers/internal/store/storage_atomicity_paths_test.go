package store

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
)

func TestStoreTransactionsRejectUnavailableOrInvalidStorage(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	base := New(pool)
	if inserted, err := base.InsertInitialPoolStats(ctx, InitialPoolStats{PoolAddress: "pool-a", TokenAddress: "token-a", CreatorAddress: nil, Price: "100", MarketCap: "200", High24h: "100", Low24h: "100", CreatedAt: 100, UpdatedAt: 100}); err != nil || !inserted {
		t.Fatalf("pool: %t %v", inserted, err)
	}
	if applied, err := base.ApplyHolderDelta(ctx, "pool-a", "wallet-a", true, "0", "10", 100); err != nil || !applied {
		t.Fatalf("holder: %t %v", applied, err)
	}
	if err := base.ApplyVolumeBucket(ctx, "pool-a", 60, "10", "100", "wallet-a", true); err != nil {
		t.Fatal(err)
	}
	if err := base.UpsertPriceSnapshot(ctx, "pool-a", 60, "90"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, setup, want string
		run               func(*Store) error
	}{
		{"holder arithmetic", "UPDATE stats.pool_holders SET balance='invalid'", "holder balance math", func(s *Store) error {
			_, e := s.ApplyHolderDelta(ctx, "pool-a", "wallet-a", true, "0", "1", 200)
			return e
		}},
		{"holder percentage", "", "holder pct", func(s *Store) error {
			_, e := s.ApplyHolderDelta(ctx, "pool-a", "wallet-b", true, "0", "invalid", 200)
			return e
		}},
		{"holder read", "ALTER TABLE stats.pool_holders RENAME TO saved_holders", "read holder", func(s *Store) error {
			_, e := s.ApplyHolderDelta(ctx, "pool-a", "wallet-a", true, "0", "1", 200)
			return e
		}},
		{"holder write", "ALTER TABLE stats.pool_holders ADD CHECK(balance='10') NOT VALID", "upsert holder", func(s *Store) error {
			_, e := s.ApplyHolderDelta(ctx, "pool-a", "wallet-a", true, "0", "1", 200)
			return e
		}},
		{"holder count", "ALTER TABLE stats.pool_holders RENAME TO saved_holders", "count holders", func(s *Store) error { return s.RefreshPoolHolderStats(ctx, "pool-a", 200) }},
		{"holder rank", "UPDATE stats.pool_holders SET balance='invalid'", "", func(s *Store) error { return s.RefreshPoolHolderStats(ctx, "pool-a", 200) }},
		{"holder pool", "ALTER TABLE stats.pool_stats RENAME TO saved_stats", "read pool stats", func(s *Store) error { return s.RefreshPoolHolderStats(ctx, "pool-a", 200) }},
		{"holder stats write", "ALTER TABLE stats.pool_stats ADD CHECK(updated_at=100) NOT VALID", "update holder stats", func(s *Store) error { return s.RefreshPoolHolderStats(ctx, "pool-a", 200) }},
		{"bucket arithmetic", "UPDATE stats.volume_buckets SET volume_usdl='invalid'", "bucket volume add", func(s *Store) error { return s.ApplyVolumeBucket(ctx, "pool-a", 60, "1", "100", "wallet-a", true) }},
		{"bucket update", "ALTER TABLE stats.volume_buckets ADD CHECK(volume_usdl='10') NOT VALID", "update volume bucket", func(s *Store) error { return s.ApplyVolumeBucket(ctx, "pool-a", 60, "1", "100", "wallet-a", true) }},
		{"bucket insert", "ALTER TABLE stats.volume_buckets ADD CHECK(false) NOT VALID", "insert volume bucket", func(s *Store) error { return s.ApplyVolumeBucket(ctx, "pool-a", 120, "1", "100", "wallet-a", true) }},
		{"bucket read", "ALTER TABLE stats.volume_buckets RENAME TO saved_buckets", "read volume bucket", func(s *Store) error { return s.ApplyVolumeBucket(ctx, "pool-a", 60, "1", "100", "wallet-a", true) }},
		{"bucket trader", "ALTER TABLE stats.bucket_traders ADD CHECK(false) NOT VALID", "insert bucket trader", func(s *Store) error { return s.ApplyVolumeBucket(ctx, "pool-a", 60, "1", "100", "wallet-b", true) }},
		{"rolling read", "ALTER TABLE stats.volume_buckets RENAME TO saved_buckets", "read rolling buckets", func(s *Store) error { return s.RecalculateRollingStats(ctx, "pool-a", 200) }},
		{"rolling amount", "UPDATE stats.volume_buckets SET volume_usdl='invalid'", "rolling volume add", func(s *Store) error { return s.RecalculateRollingStats(ctx, "pool-a", 200) }},
		{"rolling traders", "ALTER TABLE stats.bucket_traders RENAME TO saved_traders", "count unique traders", func(s *Store) error { return s.RecalculateRollingStats(ctx, "pool-a", 200) }},
		{"rolling write", "ALTER TABLE stats.pool_stats ADD CHECK(updated_at=100) NOT VALID", "update rolling stats", func(s *Store) error { return s.RecalculateRollingStats(ctx, "pool-a", 200) }},
		{"snapshot write", "ALTER TABLE stats.price_snapshots ADD CHECK(false) NOT VALID", "upsert price snapshot", func(s *Store) error { return s.UpsertPriceSnapshot(ctx, "pool-a", 120, "100") }},
		{"snapshot read", "ALTER TABLE stats.price_snapshots RENAME TO saved_snapshots", "read snapshot", func(s *Store) error { _, e := s.ComputePriceChanges(ctx, "pool-a", "100", 200); return e }},
		{"price pool read", "ALTER TABLE stats.pool_stats RENAME TO saved_stats", "read pool stats high/low", func(s *Store) error { return s.UpdatePoolStatsPriceAndChanges(ctx, "pool-a", "101", "202", 200, nil) }},
		{"price high integer", "UPDATE stats.pool_stats SET high_24h='invalid'", "high max", func(s *Store) error { return s.UpdatePoolStatsPriceAndChanges(ctx, "pool-a", "101", "202", 200, nil) }},
		{"price low integer", "UPDATE stats.pool_stats SET low_24h='invalid'", "low min", func(s *Store) error { return s.UpdatePoolStatsPriceAndChanges(ctx, "pool-a", "101", "202", 200, nil) }},
		{"price update", "ALTER TABLE stats.pool_stats ADD CHECK(updated_at=100) NOT VALID", "update pool stats price/changes", func(s *Store) error { return s.UpdatePoolStatsPriceAndChanges(ctx, "pool-a", "101", "202", 200, nil) }},
		{"idle invalid price", "UPDATE stats.pool_stats SET price='invalid'", "invalid current price", func(s *Store) error { return s.refreshIdlePool(ctx, "pool-a", 200) }},
		{"idle invalid snapshot", "UPDATE stats.price_snapshots SET price='invalid'", "invalid idle snapshot price", func(s *Store) error { return s.refreshIdlePool(ctx, "pool-a", 200) }},
		{"idle unavailable transactions", "ALTER TABLE stats.pool_transactions RENAME TO saved_transactions", "does not exist", func(s *Store) error { return s.refreshIdlePool(ctx, "pool-a", 200) }},
		{"idle unavailable price history", "ALTER TABLE stats.price_snapshots RENAME TO saved_snapshots", "read snapshot", func(s *Store) error { return s.refreshIdlePool(ctx, "pool-a", 200) }},
		{"idle invalid amount", "INSERT INTO stats.pool_transactions(id,pool_address,sender,is_buy,amount_in,amount_out,price,fee,tx_hash,block_timestamp) VALUES('invalid','pool-a','wallet-a',true,'invalid','1','1','0','hash',150)", "invalid rolling settlement amount", func(s *Store) error { return s.refreshIdlePool(ctx, "pool-a", 200) }},
		{"idle missing pool", "", "no rows", func(s *Store) error { return s.refreshIdlePool(ctx, "absent", 200) }},
		{"idle write rejected", "ALTER TABLE stats.pool_stats ADD CHECK(high_24h='100') NOT VALID; UPDATE stats.price_snapshots SET price='110'", "check constraint", func(s *Store) error { return s.refreshIdlePool(ctx, "pool-a", 200) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.setup != "" {
				if _, err = tx.Exec(ctx, tc.setup); err != nil {
					t.Fatal(err)
				}
			}
			if err = tc.run(base.WithTx(tx)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("storage rejection: %v, want %s", err, tc.want)
			}
		})
	}
	holder, found, err := base.GetHolderBalance(ctx, "pool-a", "wallet-a")
	if err != nil || !found || holder != "10" {
		t.Fatalf("committed holder changed: %s %t %v", holder, found, err)
	}
	stats, err := base.FindPoolStats(ctx, "POOL-A")
	if err != nil || stats == nil || stats.Price != "100" || stats.UpdatedAt != 100 {
		t.Fatalf("committed stats changed: %+v %v", stats, err)
	}
	if row, err := base.FindPoolStats(ctx, "absent"); err != nil || row != nil {
		t.Fatalf("absent pool: %+v %v", row, err)
	}
}

func TestStoreClosedDatabaseReturnsErrors(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	st := New(pool)
	pool.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"find pool", func() error { _, e := st.FindPoolStats(ctx, "pool"); return e }},
		{"batch pools", func() error { _, e := st.GetPoolStatsBatch(ctx, []string{"pool"}); return e }},
		{"holder page", func() error { _, e := st.ListHolders(ctx, "pool", 10, 0); return e }},
		{"all holders", func() error { _, e := st.ListHoldersByBalance(ctx, "pool"); return e }},
		{"holder balance", func() error { _, _, e := st.GetHolderBalance(ctx, "pool", "wallet"); return e }},
		{"holder balances", func() error { _, e := st.ListHolderBalances(ctx, "pool", 10, 0); return e }},
		{"holder count", func() error { _, e := st.CountHolders(ctx, "pool"); return e }},
		{"holder apply", func() error { _, e := st.ApplyHolderDelta(ctx, "pool", "wallet", true, "0", "1", 200); return e }},
		{"idle windows", func() error { _, _, e := st.RefreshIdleWindows(ctx, 200, "", 10); return e }},
		{"idle pool", func() error { return st.refreshIdlePool(ctx, "pool", 200) }},
		{"platform", func() error { _, e := st.PlatformMetrics(ctx, 200); return e }},
		{"pressure", func() error { _, _, e := st.PressureStats(ctx, "pool", 100); return e }},
		{"search", func() error { _, e := st.Search(ctx, false, "%token%", 10); return e }},
		{"prune traders", func() error { _, e := st.PruneBucketTraders(ctx, 100); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "closed") {
				t.Fatalf("closed store: %v", err)
			}
		})
	}
}
