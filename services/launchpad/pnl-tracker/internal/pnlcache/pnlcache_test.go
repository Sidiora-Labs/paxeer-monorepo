package pnlcache_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
)

func TestInvalidateUserKeepsOtherCachedProducts(t *testing.T) {
	ctx := context.Background()
	rdb := internaltest.NewRedis(t)
	user := "0xAbCd"
	positions := pnlcache.KeyPositions(user)
	portfolio := pnlcache.KeyPortfolio(user)
	card := pnlcache.KeyCard("CardA")
	og := pnlcache.KeyOG("CardA")
	if positions != "pnl:positions:0xabcd" || portfolio != "pnl:portfolio:0xabcd" || card != "pnl:card:CardA" || og != "pnl:og:CardA" {
		t.Fatalf("cache keys: %q %q %q %q", positions, portfolio, card, og)
	}
	for _, key := range []string{positions, portfolio, card, og} {
		if err := rdb.Set(ctx, key, "value", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	if err := pnlcache.InvalidateUser(ctx, rdb, user); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{positions, portfolio} {
		if exists, err := rdb.Exists(ctx, key).Result(); err != nil || exists != 0 {
			t.Fatalf("invalidated key %s exists = %d, error = %v", key, exists, err)
		}
	}
	for _, key := range []string{card, og} {
		if value, err := rdb.Get(ctx, key).Result(); err != nil || value != "value" {
			t.Fatalf("retained key %s = %q, error = %v", key, value, err)
		}
	}
	if err := pnlcache.InvalidateUser(ctx, nil, user); err != nil {
		t.Fatalf("absent cache: %v", err)
	}
}
