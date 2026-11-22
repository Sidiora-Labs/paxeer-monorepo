package httpapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"
)

func logOperationalFailure(ctx context.Context, logger *slog.Logger, operation string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	cause := "unavailable"
	switch {
	case errors.Is(err, context.Canceled):
		cause = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		cause = "timeout"
	case errors.Is(err, puddle.ErrClosedPool):
		cause = "pool_closed"
	case errors.Is(err, pgx.ErrNoRows):
		cause = "record_missing"
	case errors.Is(err, store.ErrWatermark):
		cause = "invalid_watermark"
	}
	attributes := []any{slog.String("operation", operation), slog.String("cause", cause)}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		attributes[1] = slog.String("cause", "database_error")
		attributes = append(attributes, slog.String("sqlstate", databaseError.Code))
	}
	logger.ErrorContext(ctx, "indexer operational read unavailable", attributes...)
}
