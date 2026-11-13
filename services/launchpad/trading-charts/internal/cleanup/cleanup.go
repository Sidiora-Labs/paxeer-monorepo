package cleanup

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Close reports a resource cleanup error.
func Close(resource io.Closer) {
	if err := resource.Close(); err != nil {
		slog.Error("resource close failed", slog.Any("err", err))
	}
}

// CloseContext reports a context-bound resource cleanup error.
func CloseContext(ctx context.Context, resource interface{ Close(context.Context) error }) {
	if err := resource.Close(ctx); err != nil {
		slog.Error("resource close failed", slog.Any("err", err))
	}
}

// Rollback reports an unsuccessful transaction cleanup.
func Rollback(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("transaction rollback failed", slog.Any("err", err))
	}
}
