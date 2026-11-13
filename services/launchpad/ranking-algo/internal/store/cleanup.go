package store

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

func rollbackTransaction(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Warn("ranking transaction rollback failed", "error", err)
	}
}
