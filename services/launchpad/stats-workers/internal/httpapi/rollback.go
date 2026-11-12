package httpapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

func rollbackTransaction(ctx context.Context, tx interface{ Rollback(context.Context) error }) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("transaction rollback failed", "error", err)
	}
}
