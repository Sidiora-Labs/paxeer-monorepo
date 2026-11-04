// Package engine implements the OHLCV candle builder and gap-fill logic, porting
// candles/src/engine. All money math uses math/big via shared/db helpers — never
// float (invariant i1). Candle updates are published to Redis channel
// constants.ChannelCandleUpdate for downstream WS consumers.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

// largeTradeThreshold is $500 USDL in raw 6-decimal units — swaps above this
// count as "large" (parity with candle-builder.ts).
var largeTradeThreshold = new(big.Int).SetInt64(500_000_000)

// minValidTimestamp is Jan 1 2024 — any timestamp before this is clearly bad data.
const minValidTimestamp int64 = 1704067200

// dedupCacheTTL bounds how long a folded swap key is remembered in-process and
// dedupCacheMax bounds its size. The durable candles.processed_swaps table is
// the correctness floor, so the in-memory cache is a pure hot-path optimisation
// and these values are not correctness-critical.
const (
	dedupCacheTTL = 2 * time.Minute
	dedupCacheMax = 50_000
)

// SwapEvent is the input swap event from the indexer webhook or Redis pub/sub.
type SwapEvent struct {
	PoolID         string      `json:"poolId"`
	PoolAddress    string      `json:"poolAddress"`
	Sender         string      `json:"sender"`
	IsBuy          bool        `json:"isBuy"`
	AmountIn       string      `json:"amountIn"`
	AmountOut      string      `json:"amountOut"`
	Fee            string      `json:"fee"`
	Price          string      `json:"price"`
	BlockTimestamp int64       `json:"blockTimestamp"`
	TxHash         string      `json:"txHash"`
	LogIndex       int         `json:"logIndex"`
	Position       *ChainOrder `json:"position"`
}

// CandleUpdateEvent is published to Redis on every candle change.
type CandleUpdateEvent struct {
	PoolAddress     string `json:"poolAddress"`
	Timeframe       string `json:"timeframe"`
	CandleStart     int64  `json:"candleStart"`
	Open            string `json:"open"`
	High            string `json:"high"`
	Low             string `json:"low"`
	Close           string `json:"close"`
	VolumeUsdl      string `json:"volumeUsdl"`
	VolumeToken     string `json:"volumeToken"`
	BuyVolumeUsdl   string `json:"buyVolumeUsdl"`
	SellVolumeUsdl  string `json:"sellVolumeUsdl"`
	TradeCount      int    `json:"tradeCount"`
	UniqueTraders   int    `json:"uniqueTraders"`
	LargeTradeCount int    `json:"largeTradeCount"`
	McapOpen        string `json:"mcapOpen"`
	McapHigh        string `json:"mcapHigh"`
	McapLow         string `json:"mcapLow"`
	McapClose       string `json:"mcapClose"`
	EventID         string `json:"eventId"`
	Revision        int    `json:"revision"`
}

// Builder commits all timeframe folds, trader membership and delivery records together.
type Builder struct {
	storage developmentStore
	redis   *goredis.Client
	logger  *slog.Logger
}

func New(pool *pgxpool.Pool, rdb *goredis.Client, st *store.Store, logger *slog.Logger) *Builder {
	return &Builder{storage: &developmentSQLStore{pool: pool}, redis: rdb, logger: logger}
}

func (b *Builder) ProcessSwap(ctx context.Context, swap SwapEvent) error {
	if swap.BlockTimestamp < minValidTimestamp {
		return nil
	}
	if swap.PoolAddress == "" || swap.TxHash == "" || swap.Position == nil || swap.Position.Height == 0 || swap.LogIndex < 0 || swap.Position.LogIndex != uint64(swap.LogIndex) {
		return fmt.Errorf("engine: source chain position required")
	}
	if err := b.storage.Transaction(ctx, swap.PoolAddress, swap.TxHash+":"+strconv.Itoa(swap.LogIndex), func(tx developmentKV) error {
		return b.foldSwap(tx, swap)
	}); err != nil {
		return err
	}
	// A delivery failure is visible to ingestion; retry drains the committed
	// outbox even when the swap's durable identity is already claimed.
	return b.RelayCommitted(ctx, swap.PoolAddress)
}

// dedupCache is a small, bounded, TTL'd set of recently-folded (txHash:logIndex)
// keys. It short-circuits the common near-simultaneous dual-delivery duplicate
// before a DB round-trip; correctness never depends on it (the
// candles.processed_swaps table is the durable floor). Concurrency-safe.
type dedupCache struct {
	mu      sync.Mutex
	entries map[string]int64 // key -> expiry (unix nanos)
	ttl     time.Duration
	max     int
	now     func() time.Time
}

func newDedupCache(ttl time.Duration, maxEntries int) *dedupCache {
	return &dedupCache{entries: make(map[string]int64), ttl: ttl, max: maxEntries, now: time.Now}
}

// has reports whether key is present and unexpired, pruning it if expired.
func (c *dedupCache) has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp, ok := c.entries[key]
	if !ok {
		return false
	}
	if c.now().UnixNano() > exp {
		delete(c.entries, key)
		return false
	}
	return true
}

// add records key with a fresh TTL, pruning expired entries (and, if still at
// capacity, evicting arbitrary entries) to keep memory bounded.
func (c *dedupCache) add(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now().UnixNano()
	if len(c.entries) >= c.max {
		for k, exp := range c.entries {
			if now > exp {
				delete(c.entries, k)
			}
		}
		for k := range c.entries {
			if len(c.entries) < c.max {
				break
			}
			delete(c.entries, k)
		}
	}
	c.entries[key] = now + c.ttl.Nanoseconds()
}
