// Package development exposes the development chart producer and its committed
// state contract to development projectors. Production uses accounting instead.
package development

import (
	"context"
	"log/slog"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Candle = engine.DevelopmentCandle
type Swap = engine.SwapEvent
type Order = engine.ChainOrder
type Source = consumer.DevelopmentSource

func NewSource(pool *pgxpool.Pool, redis *redis.Client, rpc *rpc.Client, logger *slog.Logger) *Source {
	return consumer.NewDevelopmentSource(pool, rpc, engine.New(pool, redis, store.New(pool), logger), logger)
}

func Migrate(ctx context.Context, dsn string) error { return migrate.Up(ctx, dsn) }
