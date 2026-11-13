package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestStoreReadsHonorCanceledRequestContext(t *testing.T) {
	st, _, _ := newStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := st.DevelopmentIndexerCursor(ctx, 125); !errors.Is(err, store.ErrIndexerCursorUnavailable) {
		t.Fatalf("canceled indexer cursor: %v", err)
	}
	const pool = "0x1111111111111111111111111111111111111111"
	request := store.AccountingPageRequest{Pool: pool, Interval: "1m", From: 1000, To: 61000, Limit: 10}
	for _, check := range []struct {
		name string
		read func(context.Context) error
	}{
		{"indexer head", func(ctx context.Context) error { _, err := st.DevelopmentIndexerHead(ctx); return err }},
		{"pool registry", func(ctx context.Context) error { _, err := st.PoolRegistry(ctx, []string{pool}); return err }},
		{"pool reserves", func(ctx context.Context) error { _, err := st.PoolReserves(ctx, []string{pool}); return err }},
		{"pool home", func(ctx context.Context) error { _, err := st.PoolHomeChains(ctx, []string{pool}); return err }},
		{"pool indexed", func(ctx context.Context) error { _, err := st.DevelopmentPoolIndexed(ctx, pool); return err }},
		{"pools indexed", func(ctx context.Context) error { _, err := st.DevelopmentPoolsIndexed(ctx, []string{pool}); return err }},
		{"swaps recorded", func(ctx context.Context) error { _, err := st.DevelopmentSwapsRecorded(ctx, pool, ""); return err }},
		{"candles recorded", func(ctx context.Context) error {
			_, err := st.DevelopmentCandlesRecorded(ctx, pool, "1m", 1, 61)
			return err
		}},
		{"development pools", func(ctx context.Context) error { _, err := st.DevelopmentPools(ctx, []string{pool}); return err }},
		{"development candles", func(ctx context.Context) error { _, err := st.DevelopmentCandlePage(ctx, request); return err }},
		{"development trades", func(ctx context.Context) error { _, err := st.DevelopmentTrades(ctx, request); return err }},
		{"development holders", func(ctx context.Context) error {
			_, err := st.DevelopmentHolders(ctx, pool, nil, "", "", 10)
			return err
		}},
		{"development movers", func(ctx context.Context) error { _, _, err := st.DevelopmentPriceMovers(ctx, true, 10); return err }},
		{"development leaderboard", func(ctx context.Context) error {
			_, err := st.DevelopmentLeaderboard(ctx, "volume", "24h", 1100, store.ChainWatermark{})
			return err
		}},
		{"development discovery sources", func(ctx context.Context) error { _, _, err := st.DevelopmentDiscoverySources(ctx, nil, 10); return err }},
		{"development discovery source", func(ctx context.Context) error {
			_, _, err := st.DevelopmentDiscoverySource(ctx, "source-1")
			return err
		}},
		{"development callouts", func(ctx context.Context) error {
			_, _, err := st.DevelopmentPoolCallouts(ctx, pool, nil, 10)
			return err
		}},
		{"development search", func(ctx context.Context) error {
			_, err := st.DevelopmentSearch(ctx, "pool", "exact_address", pool, "", 10)
			return err
		}},
		{"launched chains", func(ctx context.Context) error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"launched chain", func(ctx context.Context) error { _, err := st.LaunchedChain(ctx, 7); return err }},
		{"chain revenue", func(ctx context.Context) error { _, err := st.ChainRevenue(ctx, 7, pool, 10, 0); return err }},
		{"holder rewards", func(ctx context.Context) error { _, err := st.HolderRewardsStats(ctx); return err }},
		{"holder reward pools", func(ctx context.Context) error { _, err := st.HolderRewardPools(ctx, "rewards"); return err }},
		{"stats batch", func(ctx context.Context) error { _, err := st.StatsBatch(ctx, []string{pool}); return err }},
		{"cached document", func(ctx context.Context) error { _, _, err := st.CachedJSON(ctx, "platform:metrics"); return err }},
	} {
		t.Run(check.name, func(t *testing.T) {
			if err := check.read(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled read: %v", err)
			}
		})
	}
}
