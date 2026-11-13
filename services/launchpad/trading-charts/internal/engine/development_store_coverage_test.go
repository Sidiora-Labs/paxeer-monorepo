package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestDevelopmentStoreReadsLegacyProjectionAndConflictingIdentity(t *testing.T) {
	ctx := context.Background()
	builder, pool := chartDevelopmentBuilder(t)
	first := repair1051Swap(0, 1704067260, "1000000000000000000")
	if err := builder.ProcessSwap(ctx, first); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, key, want string
		deleteState     bool
	}{
		{"legacy candle", developmentKey("candle", first.PoolAddress, "1m", "1704067260"), "source reconstruction", true},
		{"legacy cursor", developmentKey("cursor", first.PoolAddress, "1m"), "", true},
		{"absent outbox", developmentKey("outbox", "absent"), "", false},
		{"invalid key", "unrecognised/[]", "invalid development key arity", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.deleteState {
				if _, err := pool.Exec(ctx, `DELETE FROM candles.development_state WHERE key=$1`, tc.key); err != nil {
					t.Fatal(err)
				}
			}
			err := builder.storage.Transaction(ctx, first.PoolAddress, first.TxHash, func(kv developmentKV) error {
				raw, readErr := kv.Get(tc.key)
				if tc.want != "" {
					if readErr == nil || !strings.Contains(readErr.Error(), tc.want) {
						t.Fatalf("legacy read: %v", readErr)
					}
					return nil
				}
				if readErr != nil {
					t.Fatal(readErr)
				}
				if tc.name == "legacy cursor" {
					var cursor store.CursorRow
					if err := json.Unmarshal(raw, &cursor); err != nil || cursor.PoolAddress != first.PoolAddress || cursor.Timeframe != "1m" || cursor.LastCandleStart != 1704067260 {
						t.Fatalf("legacy cursor: %+v, %v", cursor, err)
					}
				} else if len(raw) != 0 {
					t.Fatalf("absent development document: %s", raw)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	conflict := first
	conflict.Price = "2000000000000000000"
	if err := builder.storage.Transaction(ctx, first.PoolAddress, first.TxHash, func(kv developmentKV) error {
		claimed, claimErr := kv.Claim(conflict)
		if claimed || claimErr == nil || !strings.Contains(claimErr.Error(), "conflicting swap identity") {
			t.Fatalf("conflicting source identity: %v, %v", claimed, claimErr)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key  string
		body []byte
	}{
		{key: "bad", body: []byte(`{}`)},
		{key: developmentKey("cursor", first.PoolAddress, "1m"), body: []byte("invalid")},
		{key: developmentKey("candle", first.PoolAddress, "1m", "1704067260"), body: []byte("invalid")},
	} {
		err := builder.storage.Transaction(ctx, first.PoolAddress, "", func(kv developmentKV) error { return kv.Put(tc.key, tc.body) })
		if err == nil {
			t.Fatalf("invalid development document admitted: %s", tc.key)
		}
	}
	rollback := errors.New("rollback requested")
	err := builder.storage.Transaction(ctx, first.PoolAddress, "", func(kv developmentKV) error {
		if err := kv.Put(developmentKey("outbox", "rolled-back"), []byte(`[]`)); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction rollback: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.development_state WHERE key=$1`, developmentKey("outbox", "rolled-back")).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled back document: %d, %v", count, err)
	}
}
